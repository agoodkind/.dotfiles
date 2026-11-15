package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/gitdir"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// UpdateAuthorizedKeys fetches SSH public keys from GitHub.
//
// It appends any new keys to ~/.ssh/authorized_keys.
//
// Example:
//
//	if err := UpdateAuthorizedKeys(ctx, false, logger); err != nil {
//		return err
//	}
func UpdateAuthorizedKeys(ctx context.Context, skipNetwork bool, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: UpdateAuthorizedKeys")
	if common.IsWorkLaptop() || skipNetwork {
		return nil
	}

	home := os.Getenv("HOME")
	httpTmp := filepath.Join(home, ".ssh", "authorized_keys.tmp")
	if err := cmdexec.RunWithLogger(
		ctx,
		logger,
		"curl",
		"-fsSL",
		"https://github.com/agoodkind.keys",
		"-o",
		httpTmp,
	); err != nil {
		slog.WarnContext(ctx, "workspace: fetching authorized keys", "err", err)

		return fmt.Errorf("fetching authorized keys: %w", err)
	}

	defer os.Remove(httpTmp)

	homeRoot, err := os.OpenRoot(home)
	if err != nil {
		slog.WarnContext(ctx, "workspace: opening home dir", "err", err)

		return fmt.Errorf("opening home dir: %w", err)
	}
	defer closeRootWithWarning(homeRoot, home)

	if err := homeRoot.MkdirAll(".ssh", 0o700); err != nil {
		slog.WarnContext(ctx, "workspace: creating ssh dir", "err", err)

		return fmt.Errorf("creating ssh dir: %w", err)
	}

	authorizedKeysEnsure, err := homeRoot.OpenFile(
		".ssh/authorized_keys",
		os.O_CREATE|os.O_RDWR,
		0o600,
	)
	if err != nil {
		slog.WarnContext(ctx, "workspace: ensuring authorized_keys exists", "err", err)

		return fmt.Errorf("ensuring authorized_keys exists: %w", err)
	}

	_ = authorizedKeysEnsure.Close()

	rawExisting, err := homeRoot.ReadFile(".ssh/authorized_keys")
	if err != nil {
		slog.WarnContext(ctx, "workspace: reading authorized_keys", "err", err)

		return fmt.Errorf("reading authorized_keys: %w", err)
	}

	existing := authorizedKeyLineSet(rawExisting)

	for line := range strings.SplitSeq(string(rawExisting), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			existing[line] = true
		}
	}

	downloaded, err := homeRoot.ReadFile(".ssh/authorized_keys.tmp")
	if err != nil {
		slog.WarnContext(ctx, "workspace: reading downloaded keys", "err", err)

		return fmt.Errorf("reading downloaded keys: %w", err)
	}

	added, err := appendMissingAuthorizedKeys(ctx, homeRoot, existing, downloaded)
	if err != nil {
		slog.WarnContext(ctx, "workspace: appending authorized keys", "err", err)

		return err
	}

	common.InfoContextf(ctx, logger, "  Added %s authorized key(s)", strconv.Itoa(added))

	return nil
}

func authorizedKeyLineSet(raw []byte) map[string]bool {
	existing := map[string]bool{}

	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			existing[line] = true
		}
	}

	return existing
}

func appendMissingAuthorizedKeys(
	ctx context.Context,
	homeRoot *os.Root,
	existing map[string]bool,
	downloaded []byte,
) (int, error) {
	added := 0

	for line := range strings.SplitSeq(string(downloaded), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if existing[line] {
			continue
		}

		existing[line] = true
		authorizedKeysFile, err := homeRoot.OpenFile(
			".ssh/authorized_keys",
			os.O_APPEND|os.O_WRONLY,
			0o600,
		)
		if err != nil {
			slog.WarnContext(ctx, "workspace: opening authorized_keys for append", "err", err)

			return added, fmt.Errorf("opening authorized_keys for append: %w", err)
		}

		_, _ = fmt.Fprintln(authorizedKeysFile, line)
		_ = authorizedKeysFile.Close()
		added++
	}

	return added, nil
}

// SyncCursorUserRules reports that Cursor cloud rule upload is disabled.
// Sync invokes SyncCursorUserRules when SkipCursorSync is false. It logs the
// disabled status through the sync logger and does not contact the Cursor API.
//
// Example:
//
//	if err := SyncCursorUserRules(ctx, dotfiles, logger); err != nil {
//		return err
//	}
func SyncCursorUserRules(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	_ = dotfiles
	common.InfoContext(ctx, logger, "  cursor cloud rule sync is disabled")

	return nil
}

// SyncGitHooks installs git hook symlinks from .githooks/ into .git/hooks/.
//
// Sync resolves the shared git directory for linked worktrees before it writes
// hook symlinks into the hooks folder.
//
// Example:
//
//	if err := SyncGitHooks(ctx, dotfiles, logger); err != nil {
//		return err
//	}
func SyncGitHooks(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: SyncGitHooks")
	hooksPath := filepath.Join(dotfiles, ".githooks")
	if _, err := os.Stat(hooksPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		slog.WarnContext(ctx, "workspace: checking git hooks dir", "err", err)

		return fmt.Errorf("checking git hooks dir: %w", err)
	}

	// Hooks live in the shared git directory, which every worktree uses. In a
	// linked worktree <root>/.git is a file. Joining a hooks path onto that
	// file path fails.
	gitCommonDir := filepath.Join(dotfiles, ".git")
	if layout, err := gitdir.Resolve(ctx, dotfiles, logger); err == nil {
		gitCommonDir = layout.CommonDir
	}

	destination := filepath.Join(gitCommonDir, "hooks")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		slog.WarnContext(ctx, "workspace: creating git hooks destination", "err", err)

		return fmt.Errorf("creating git hooks destination: %w", err)
	}

	entries, err := os.ReadDir(hooksPath)
	if err != nil {
		slog.WarnContext(ctx, "workspace: reading git hooks dir", "err", err)

		return fmt.Errorf("reading git hooks dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		target := filepath.Join(destination, entry.Name())
		_ = os.Remove(target)
		// An absolute link target stays correct wherever the shared git
		// directory sits relative to the working tree.
		if err := os.Symlink(filepath.Join(hooksPath, entry.Name()), target); err != nil {
			slog.WarnContext(ctx, "workspace: creating hook symlink", "err", err)

			return fmt.Errorf("creating hook symlink %s: %w", target, err)
		}
	}

	return nil
}

// SyncGlobalGitHooks configures git to use the global hooks directory.
//
// Example:
//
//	if err := SyncGlobalGitHooks(ctx, dotfiles, logger); err != nil {
//		return err
//	}
func SyncGlobalGitHooks(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	hooks := filepath.Join(dotfiles, "git-global-hooks")
	if _, err := os.Stat(hooks); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		slog.WarnContext(ctx, "workspace: checking global git hooks dir", "err", err)

		return fmt.Errorf("checking global git hooks dir: %w", err)
	}

	if err := cmdexec.RunWithLogger(
		ctx,
		logger,
		"git",
		"config",
		"--global",
		"core.hooksPath",
		hooks,
	); err != nil {
		slog.WarnContext(ctx, "workspace: setting git global hooks path", "err", err)

		return fmt.Errorf("setting git global hooks path: %w", err)
	}

	return nil
}

// CheckGitHooksPath verifies the local git hooks path configuration.
//
// Example:
//
//	if err := CheckGitHooksPath(ctx, dotfiles, logger); err != nil {
//		return err
//	}
func CheckGitHooksPath(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	if configured, err := cmdexec.OutputWithLoggerAndEnv(
		ctx,
		logger,
		nil,
		"git",
		"-C",
		dotfiles,
		"config",
		"--local",
		"--get",
		"core.hooksPath",
	); err == nil {
		_ = strings.TrimSpace(configured)
	}

	return nil
}
