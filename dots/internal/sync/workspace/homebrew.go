package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/telemetry"
)

var errHomebrewOwnershipNeedsSudo = errors.New(
	"homebrew path(s) are not owned by the current user and passwordless sudo is unavailable",
)

// CleanupHomebrewRepair removes incomplete brew downloads and runs cleanup.
//
// When cleanup fails because a Homebrew-managed path was left owned by another
// user, it restores ownership of those paths and retries brew cleanup.
//
// Example:
//
//	if err := CleanupHomebrewRepair(ctx, logger); err != nil {
//		return err
//	}
func CleanupHomebrewRepair(ctx context.Context, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: CleanupHomebrewRepair")
	if runtime.GOOS != "darwin" {
		return nil
	}

	if !runner.HasCommand("brew") {
		return nil
	}

	cacheDir := filepath.Join(os.Getenv("HOME"), "Library", "Caches", "Homebrew", "downloads")
	if matches, err := filepath.Glob(filepath.Join(cacheDir, "*.incomplete")); err == nil &&
		len(matches) > 0 {
		relPaths := make([]string, 0, len(matches))
		for _, match := range matches {
			rel, relErr := filepath.Rel(cacheDir, match)
			if relErr != nil || !filepath.IsLocal(rel) {
				continue
			}

			relPaths = append(relPaths, rel)
		}

		if len(relPaths) > 0 {
			cacheRoot, openErr := os.OpenRoot(cacheDir)
			if openErr == nil {
				for _, rel := range relPaths {
					_ = cacheRoot.RemoveAll(rel)
				}

				_ = cacheRoot.Close()
			}
		}
	}

	out, err := cmdexec.OutputWithLogger(ctx, logger, "brew", "cleanup", "--prune=all")
	if err == nil {
		return nil
	}

	blocked := parseBrewPermissionDeniedPaths(out)
	if len(blocked) == 0 {
		slog.WarnContext(ctx, "workspace: running brew cleanup", "err", err)

		return fmt.Errorf("running brew cleanup: %w", err)
	}

	repaired, repairErr := restoreOwnershipChains(ctx, logger, blocked)
	if repairErr != nil {
		slog.WarnContext(ctx, "workspace: brew cleanup self-heal unavailable", "err", repairErr)

		return fmt.Errorf("running brew cleanup: %w", errors.Join(err, repairErr))
	}

	if repaired == 0 {
		slog.WarnContext(ctx, "workspace: running brew cleanup", "err", err)

		return fmt.Errorf("running brew cleanup: %w", err)
	}

	logger.InfoContext(
		ctx,
		fmt.Sprintf("  restored ownership of %d Homebrew path(s); retrying cleanup", repaired),
	)
	if _, err := cmdexec.OutputWithLogger(
		ctx,
		logger,
		"brew",
		"cleanup",
		"--prune=all",
	); err != nil {
		slog.WarnContext(ctx, "workspace: brew cleanup after repair", "err", err)

		return fmt.Errorf("running brew cleanup after restoring %d path(s): %w", repaired, err)
	}

	return nil
}

// permissionDeniedPattern captures the path in a Homebrew/Ruby permission error
// such as "Permission denied @ apply2files - /opt/homebrew/.../file".
var permissionDeniedPattern = regexp.MustCompile(`Permission denied @ \S+ - (.+)`)

// parseBrewPermissionDeniedPaths extracts the distinct paths that brew cleanup
// reported it could not remove, preserving first-seen order.
func parseBrewPermissionDeniedPaths(output string) []string {
	seen := make(map[string]struct{})
	paths := make([]string, 0)

	for line := range strings.SplitSeq(output, "\n") {
		match := permissionDeniedPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		path := strings.TrimSpace(match[1])
		if path == "" {
			continue
		}

		if _, ok := seen[path]; ok {
			continue
		}

		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	return paths
}

// restoreOwnershipChains restores ownership of each blocked path, plus any
// ancestor directory up to the first one the current user already owns, so brew
// can prune the path on retry. It touches only the broken ownership chain,
// never sibling files, which keeps it safe on an Intel prefix (/usr/local) that
// is shared with non-Homebrew software. When passwordless sudo is unavailable
// it returns an error naming the remediation rather than masking the failure.
func restoreOwnershipChains(
	ctx context.Context,
	logger *telemetry.Logger,
	blocked []string,
) (int, error) {
	current, err := user.Current()
	if err != nil {
		slog.WarnContext(ctx, "workspace: resolving current user", "err", err)

		return 0, fmt.Errorf("resolving current user: %w", err)
	}

	uid, err := strconv.Atoi(current.Uid)
	if err != nil {
		slog.WarnContext(ctx, "workspace: parsing current uid", "err", err)

		return 0, fmt.Errorf("parsing current uid %q: %w", current.Uid, err)
	}

	targets := foreignOwnedChain(blocked, uid)
	if len(targets) == 0 {
		return 0, nil
	}

	if !common.HasSudoAccess(ctx, logger) {
		ownershipErr := fmt.Errorf(
			"%w; run: sudo chown %s %s",
			errHomebrewOwnershipNeedsSudo,
			current.Username,
			strings.Join(targets, " "),
		)
		slog.WarnContext(
			ctx,
			"workspace: cannot restore ownership without sudo",
			"err",
			ownershipErr,
		)

		return 0, ownershipErr
	}

	args := append([]string{"-n", "chown", current.Username}, targets...)
	if err := cmdexec.RunWithLogger(ctx, logger, "sudo", args...); err != nil {
		slog.WarnContext(ctx, "workspace: restoring ownership", "err", err)

		return 0, fmt.Errorf("restoring ownership of Homebrew paths: %w", err)
	}

	return len(targets), nil
}

// foreignOwnedChain returns each blocked path together with its ancestor
// directories that are not owned by uid, stopping at the first owned ancestor.
// Paths it cannot stat are skipped.
func foreignOwnedChain(blocked []string, uid int) []string {
	seen := make(map[string]struct{})
	targets := make([]string, 0)

	for _, path := range blocked {
		for dir := path; ; {
			info, err := os.Lstat(dir)
			if err != nil {
				break
			}

			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok || int(stat.Uid) == uid {
				break
			}

			if _, ok := seen[dir]; !ok {
				seen[dir] = struct{}{}
				targets = append(targets, dir)
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}

			dir = parent
		}
	}

	return targets
}
