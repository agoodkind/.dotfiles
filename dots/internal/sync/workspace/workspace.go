// Package workspace implements workspace-level sync operations for dotfiles.
package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goodkind.io/.dotfiles/internal/clock"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// xdgDataDir returns the XDG_DATA_HOME directory, falling back to ~/.local/share.
func xdgDataDir() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}

	return filepath.Join(os.Getenv("HOME"), ".local", "share")
}

// xdgStateDir returns the XDG_STATE_HOME directory, falling back to ~/.local/state.
func xdgStateDir() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}

	return filepath.Join(os.Getenv("HOME"), ".local", "state")
}

// LinkDotfiles creates symlinks in the home directory for each file in the dotfiles home/ directory.
func LinkDotfiles(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: LinkDotfiles")
	homeDir := os.Getenv("HOME")
	backupPath := filepath.Join(
		xdgDataDir(),
		"dots",
		"backups",
		clock.Now().Format("20060102_150405"),
	)
	source := filepath.Join(dotfiles, "home")
	linked, skipped, backed := 0, 0, 0

	walkErr := filepath.WalkDir(
		source,
		func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if entry.IsDir() {
				return nil
			}

			rel, err := filepath.Rel(source, path)
			if err != nil {
				slog.WarnContext(ctx, "workspace: computing relative path", "err", err)
				return fmt.Errorf("computing relative path: %w", err)
			}

			homeFile := filepath.Join(homeDir, rel)

			if rel == localProfileName {
				if err := ensureLocalProfile(ctx, homeFile, path); err != nil {
					return err
				}

				skipped++

				return nil
			}

			if common.IsSymlinkTo(homeFile, path) {
				skipped++
				return nil
			}

			backedUp, pathIsFree := clearExistingHomeFile(ctx, homeFile, backupPath, rel, logger)
			if backedUp {
				backed++
			}

			if !pathIsFree {
				return nil
			}

			if err := os.MkdirAll(filepath.Dir(filepath.Clean(homeFile)), 0o755); err != nil {
				slog.WarnContext(ctx, "workspace: creating home file directory", "err", err)
				return fmt.Errorf("creating home file directory: %w", err)
			}

			_ = os.Remove(filepath.Clean(homeFile))
			if err := os.Symlink(filepath.Join(source, rel), homeFile); err != nil {
				slog.WarnContext(ctx, "workspace: creating symlink", "err", err)
				return fmt.Errorf("creating symlink %s: %w", homeFile, err)
			}

			linked++

			return nil
		},
	)
	if walkErr != nil {
		slog.WarnContext(ctx, "workspace: walking dotfiles", "err", walkErr)
		return fmt.Errorf("walking dotfiles: %w", walkErr)
	}

	common.InfoContextf(
		ctx,
		logger,
		"  Linked: %s Skipped: %s Backed up: %s",
		strconv.Itoa(linked),
		strconv.Itoa(skipped),
		strconv.Itoa(backed),
	)

	return nil
}

const (
	localProfileName     = ".profile"
	localProfileSentinel = "# Local login profile managed by dots."
)

// Docker Desktop rewrites ~/.profile on launch. A symlink would dirty the
// tracked copy, so this path stays a regular file that sources the repo.
func ensureLocalProfile(ctx context.Context, homeFile string, repoProfile string) error {
	info, err := os.Lstat(filepath.Clean(homeFile))
	if os.IsNotExist(err) {
		return writeLocalProfile(ctx, homeFile, repoProfile, "", true)
	}

	if err != nil {
		slog.WarnContext(ctx, "workspace: stat profile", "err", err)
		return fmt.Errorf("stat profile: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return replaceProfileSymlink(ctx, homeFile, repoProfile)
	}

	data, err := os.ReadFile(filepath.Clean(homeFile))
	if err != nil {
		slog.WarnContext(ctx, "workspace: reading profile", "err", err)
		return fmt.Errorf("reading profile: %w", err)
	}

	return writeLocalProfile(ctx, homeFile, repoProfile, string(data), false)
}

func replaceProfileSymlink(ctx context.Context, homeFile string, repoProfile string) error {
	if common.IsSymlinkTo(homeFile, repoProfile) {
		if err := os.Remove(filepath.Clean(homeFile)); err != nil {
			slog.WarnContext(ctx, "workspace: removing profile symlink", "err", err)
			return fmt.Errorf("removing profile symlink: %w", err)
		}

		return writeLocalProfile(ctx, homeFile, repoProfile, "", true)
	}

	data, err := os.ReadFile(filepath.Clean(homeFile))
	if err != nil {
		slog.WarnContext(ctx, "workspace: reading profile symlink", "err", err)
		return fmt.Errorf("reading profile symlink: %w", err)
	}

	if err := os.Remove(filepath.Clean(homeFile)); err != nil {
		slog.WarnContext(ctx, "workspace: removing stale profile symlink", "err", err)
		return fmt.Errorf("removing stale profile symlink: %w", err)
	}

	return writeLocalProfile(ctx, homeFile, repoProfile, string(data), true)
}

func writeLocalProfile(
	ctx context.Context,
	homeFile string,
	repoProfile string,
	existing string,
	mustWrite bool,
) error {
	body := withLocalProfileSource(existing, repoProfile)
	if !mustWrite && existing != "" && body == existing {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(filepath.Clean(homeFile)), 0o755); err != nil {
		slog.WarnContext(ctx, "workspace: creating profile directory", "err", err)
		return fmt.Errorf("creating profile directory: %w", err)
	}

	if err := writeFileAtomically(ctx, homeFile, []byte(body), 0o600); err != nil {
		return fmt.Errorf("writing local profile: %w", err)
	}

	return nil
}

func writeFileAtomically(ctx context.Context, path string, body []byte, mode os.FileMode) error {
	cleanPath := filepath.Clean(path)
	file, err := os.CreateTemp(filepath.Dir(cleanPath), ".dots-profile-*")
	if err != nil {
		slog.WarnContext(ctx, "workspace: creating temp profile", "err", err)
		return fmt.Errorf("creating temp profile: %w", err)
	}

	tempName := file.Name()
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		_ = os.Remove(tempName)
		slog.WarnContext(ctx, "workspace: writing temp profile", "err", err)

		return fmt.Errorf("writing temp profile: %w", err)
	}

	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		_ = os.Remove(tempName)
		slog.WarnContext(ctx, "workspace: chmod temp profile", "err", err)

		return fmt.Errorf("chmod temp profile: %w", err)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tempName)
		slog.WarnContext(ctx, "workspace: closing temp profile", "err", err)

		return fmt.Errorf("closing temp profile: %w", err)
	}

	if err := os.Rename(tempName, cleanPath); err != nil {
		_ = os.Remove(tempName)
		slog.WarnContext(ctx, "workspace: replacing profile", "err", err)

		return fmt.Errorf("replacing profile: %w", err)
	}

	return nil
}

func withLocalProfileSource(existing string, repoProfile string) string {
	quotedRepo := shellSingleQuote(repoProfile)
	block := localProfileSentinel + " Docker Desktop may write a PATH block here.\n" +
		"if [ -f " + quotedRepo + " ]; then\n" +
		"    . " + quotedRepo + "\n" +
		"fi\n"
	if strings.Contains(existing, localProfileSentinel) && strings.Contains(existing, repoProfile) {
		return existing
	}

	prefix := existing
	suffix := ""
	if before, after, found := strings.Cut(existing, localProfileSentinel); found {
		prefix = strings.TrimRight(before, "\n")
		suffix = suffixAfterManagedBlock(after)
	}

	if prefix == "" {
		return block + suffix
	}

	if !strings.HasSuffix(prefix, "\n") {
		prefix += "\n"
	}

	return prefix + block + suffix
}

func suffixAfterManagedBlock(afterSentinel string) string {
	_, afterFi, found := strings.Cut(afterSentinel, "\nfi\n")
	if !found {
		return ""
	}

	return afterFi
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// isWithinDir reports whether target is root itself or a descendant of root.
//
// A plain [strings.HasPrefix] on the cleaned paths is not enough: it treats
// /tmp/backup-evil as contained within /tmp/backup, since the check compares
// raw characters rather than path segments.
func isWithinDir(root string, target string) bool {
	cleanRoot := filepath.Clean(root)
	cleanTarget := filepath.Clean(target)
	if cleanTarget == cleanRoot {
		return true
	}

	return strings.HasPrefix(cleanTarget, cleanRoot+string(filepath.Separator))
}

// clearExistingHomeFile backs up whatever already occupies homeFile and then
// removes it, so the caller can put a symlink there.
//
// It reports whether a backup was taken, and whether the path is now free. A
// real file whose backup failed is left in place and reported as not free,
// since removing it would destroy the only copy. A symlink carries no content
// of its own, so it is always safe to replace.
func clearExistingHomeFile(
	ctx context.Context,
	homeFile string,
	backupPath string,
	rel string,
	logger *telemetry.Logger,
) (bool, bool) {
	existing, err := os.Lstat(filepath.Clean(homeFile))
	if err != nil {
		return false, true
	}

	backedUp := false
	backupDest := filepath.Join(backupPath, rel+".bak")
	switch {
	case !isWithinDir(backupPath, backupDest):
		slog.WarnContext(ctx, "workspace: skipping backup, path traversal detected", "rel", rel)
	default:
		if mkErr := os.MkdirAll(filepath.Dir(backupDest), 0o755); mkErr != nil {
			slog.WarnContext(
				ctx,
				"workspace: creating backup directory, skipping backup",
				"err",
				mkErr,
			)
		} else if cpErr := cmdexec.RunWithLogger(
			ctx,
			logger,
			"cp",
			"-HpR",
			homeFile,
			backupDest,
		); cpErr == nil {
			backedUp = true
		}
	}

	isSymlink := existing.Mode()&os.ModeSymlink != 0
	if !backedUp && !isSymlink {
		slog.WarnContext(
			ctx,
			"workspace: backup failed, leaving existing file in place",
			"path",
			homeFile,
		)

		return false, false
	}

	_ = os.RemoveAll(filepath.Clean(homeFile))

	return backedUp, true
}
