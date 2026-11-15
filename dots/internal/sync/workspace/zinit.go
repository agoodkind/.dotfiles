package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/telemetry"
)

var (
	errInvalidGitFile       = errors.New("invalid .git file")
	errUnsafeGitFilePath    = errors.New("unsafe .git file path")
	errZinitPluginsDetached = errors.New("one or more zinit plugins are detached")
	errZshUnavailable       = errors.New("zsh is not available")
)

// CleanupZinitCompletions removes dead symlinks from the zinit completions dir.
//
// The repair step invokes CleanupZinitCompletions to delete broken completion
// symlinks under ~/.local/share/zinit/completions.
//
// Example:
//
//	if err := CleanupZinitCompletions(ctx, logger); err != nil {
//		return err
//	}
func CleanupZinitCompletions(ctx context.Context, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: CleanupZinitCompletions")
	completionsDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "zinit", "completions")
	entries, err := os.ReadDir(completionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		slog.WarnContext(ctx, "workspace: reading zinit completions dir", "err", err)

		return fmt.Errorf("reading zinit completions dir: %w", err)
	}

	completionsRoot, err := os.OpenRoot(completionsDir)
	if err != nil {
		slog.WarnContext(ctx, "workspace: opening zinit completions dir", "err", err)

		return fmt.Errorf("opening zinit completions dir: %w", err)
	}
	defer closeRootWithWarning(completionsRoot, completionsDir)

	for _, entry := range entries {
		info, err := completionsRoot.Lstat(entry.Name())
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			if _, err := completionsRoot.Stat(entry.Name()); err != nil {
				_ = completionsRoot.Remove(entry.Name())
			}
		}
	}

	return nil
}

const zinitUpdateScript = `
source "$1"

zinit self-update
self_update_rc=$?
printf "[zinit-self-update-exit: %d]\n" "$self_update_rc"

zsh -c 'source "$1"; zinit update --all --quiet' zsh "$1"
update_rc=$?
printf "[zinit-update-exit: %d]\n" "$update_rc"

zinit compile --all
compile_rc=$?
printf "[zinit-compile-exit: %d]\n" "$compile_rc"

plugins_dir="${ZINIT[PLUGINS_DIR]:-$HOME/.local/share/zinit/plugins}"
printf "[zinit-plugins-dir: %s]\n" "$plugins_dir"

(( self_update_rc == 0 && compile_rc == 0 ))
`

func zinitMarkerValue(output string, marker string) (string, bool) {
	prefix := "[" + marker + ": "

	for line := range strings.SplitSeq(output, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, prefix) && strings.HasSuffix(trimmedLine, "]") {
			value := strings.TrimSuffix(strings.TrimPrefix(trimmedLine, prefix), "]")

			return value, true
		}
	}

	return "", false
}

func zinitUpdateExitFromOutput(output string) (int, bool) {
	value, ok := zinitMarkerValue(output, "zinit-update-exit")
	if !ok {
		return 0, false
	}

	exitCode, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}

	return exitCode, true
}

func defaultZinitPluginsDir() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "zinit", "plugins")
}

func verifyZinitPlugins(ctx context.Context, pluginsDir string, logger *telemetry.Logger) error {
	if pluginsDir == "" {
		pluginsDir = defaultZinitPluginsDir()
	}

	entries, err := os.ReadDir(pluginsDir)
	if os.IsNotExist(err) {
		common.InfoContext(ctx, logger, "  [zinit-verify: ok]")

		return nil
	}

	if err != nil {
		slog.WarnContext(ctx, "workspace: read zinit plugins dir", "dir", pluginsDir, "err", err)

		return fmt.Errorf("reading zinit plugins dir %s: %w", pluginsDir, err)
	}

	verificationFailed := false

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginName := entry.Name()
		if pluginName == "custom" || pluginName == "_local---zinit" {
			continue
		}

		pluginDir := filepath.Join(pluginsDir, pluginName)
		isGitRepo, isDetached, err := zinitPluginGitState(pluginDir)
		if err != nil {
			slog.WarnContext(
				ctx,
				"workspace: check zinit plugin",
				"plugin",
				pluginName,
				"dir",
				pluginDir,
				"err",
				err,
			)

			return fmt.Errorf("checking zinit plugin %s: %w", pluginName, err)
		}

		if !isGitRepo {
			common.InfoContextf(ctx, logger, "  [zinit-verify] %s: not a git repo", pluginName)

			continue
		}

		if isDetached {
			common.WarnContextf(ctx, logger, "  [zinit-verify] %s: detached HEAD", pluginName)
			verificationFailed = true
		}
	}

	if verificationFailed {
		return errZinitPluginsDetached
	}

	common.InfoContext(ctx, logger, "  [zinit-verify: ok]")

	return nil
}

func zinitPluginGitState(pluginDir string) (bool, bool, error) {
	headPath, ok, err := zinitPluginHeadPath(pluginDir)
	if err != nil || !ok {
		return ok, false, err
	}

	headContent, err := os.ReadFile(headPath)
	if err != nil {
		slog.Warn("workspace: read zinit HEAD", "path", headPath, "err", err)

		return true, false, fmt.Errorf("reading git HEAD: %w", err)
	}

	headRef := strings.TrimSpace(string(headContent))

	return true, !strings.HasPrefix(headRef, "ref: "), nil
}

func zinitPluginHeadPath(pluginDir string) (string, bool, error) {
	pluginRoot, err := os.OpenRoot(pluginDir)
	if err != nil {
		slog.Warn("workspace: open zinit plugin root", "dir", pluginDir, "err", err)

		return "", false, fmt.Errorf("opening zinit plugin root %s: %w", pluginDir, err)
	}
	defer closeRootWithWarning(pluginRoot, pluginDir)

	gitInfo, err := pluginRoot.Stat(".git")
	if os.IsNotExist(err) {
		return "", false, nil
	}

	if err != nil {
		slog.Warn("workspace: stat zinit .git", "dir", pluginDir, "err", err)

		return "", false, fmt.Errorf("stat .git in %s: %w", pluginDir, err)
	}

	if gitInfo.IsDir() {
		return filepath.Join(pluginDir, ".git", "HEAD"), true, nil
	}

	gitFileContent, err := pluginRoot.ReadFile(".git")
	if err != nil {
		slog.Warn("workspace: read zinit .git file", "dir", pluginDir, "err", err)

		return "", true, fmt.Errorf("reading .git file: %w", err)
	}

	const gitDirPrefix = "gitdir: "
	gitDirLine := strings.TrimSpace(string(gitFileContent))
	if !strings.HasPrefix(gitDirLine, gitDirPrefix) {
		return "", true, errInvalidGitFile
	}

	gitDir := strings.TrimSpace(strings.TrimPrefix(gitDirLine, gitDirPrefix))
	if filepath.IsAbs(gitDir) {
		relativeGitDir, err := filepath.Rel(pluginDir, gitDir)
		if err != nil {
			slog.Warn(
				"workspace: relativize gitdir",
				"dir",
				pluginDir,
				"gitdir",
				gitDir,
				"err",
				err,
			)

			return "", true, fmt.Errorf("relativizing .git file path: %w", err)
		}

		gitDir = relativeGitDir
	}

	if !filepath.IsLocal(gitDir) {
		slog.Warn("workspace: unsafe gitdir", "dir", pluginDir, "gitdir", gitDir)

		return "", true, errUnsafeGitFilePath
	}

	return filepath.Join(pluginDir, filepath.Clean(gitDir), "HEAD"), true, nil
}

func closeRootWithWarning(root *os.Root, pluginDir string) {
	if err := root.Close(); err != nil {
		slog.Warn("workspace: close zinit plugin root", "dir", pluginDir, "err", err)
	}
}

// UpdateZinitPlugins runs zinit self-update and updates all zinit plugins.
//
// Sync runs the embedded update script in zsh, then verifies plugin git state.
//
// Example:
//
//	if err := UpdateZinitPlugins(ctx, dotfiles, logger); err != nil {
//		return err
//	}
func UpdateZinitPlugins(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	if !runner.HasCommand("zsh") {
		return errZshUnavailable
	}

	zinitPath := filepath.Join(dotfiles, "lib", "zinit", "zinit.zsh")
	if _, err := os.Stat(zinitPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		slog.WarnContext(ctx, "workspace: checking zinit.zsh", "err", err)

		return fmt.Errorf("checking zinit.zsh: %w", err)
	}

	output, err := cmdexec.OutputWithLogger(
		ctx,
		logger,
		"zsh",
		"-c",
		zinitUpdateScript,
		"zsh",
		zinitPath,
	)
	if err != nil {
		slog.WarnContext(ctx, "workspace: updating zinit plugins", "err", err)

		return fmt.Errorf("updating zinit plugins: %w", err)
	}

	pluginsDir, _ := zinitMarkerValue(output, "zinit-plugins-dir")
	if err := verifyZinitPlugins(ctx, pluginsDir, logger); err != nil {
		slog.WarnContext(ctx, "workspace: verifying zinit plugins", "err", err)

		return fmt.Errorf("verifying zinit plugins: %w", err)
	}

	if updateExitCode, ok := zinitUpdateExitFromOutput(output); ok && updateExitCode != 0 {
		common.InfoContext(
			ctx,
			logger,
			"  [zinit-update-status: ignored nonzero update exit after compile and verify]",
		)
	}

	return nil
}
