// Package workspace implements workspace-level sync operations for dotfiles.
package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"goodkind.io/.dotfiles/internal/clock"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/cursor/logging"
	"goodkind.io/.dotfiles/internal/cursor/syncer"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/sync/compilation"
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

// CleanupZinitCompletions removes dead symlinks from the zinit completions directory.
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
	for _, entry := range entries {
		path := filepath.Join(completionsDir, entry.Name())
		info, err := os.Lstat(filepath.Clean(path))
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if _, err := os.Stat(filepath.Clean(path)); err != nil {
				_ = os.Remove(filepath.Clean(path))
			}
		}
	}
	return nil
}

// LinkDotfiles creates symlinks in the home directory for each file in the dotfiles home/ directory.
func LinkDotfiles(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: LinkDotfiles")
	homeDir := os.Getenv("HOME")
	backupPath := filepath.Join(xdgDataDir(), "dots", "backups", clock.Now().Format("20060102_150405"))
	source := filepath.Join(dotfiles, "home")
	linked, skipped, backed := 0, 0, 0

	walkErr := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
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

		if common.IsSymlinkTo(homeFile, path) {
			skipped++
			return nil
		}

		if _, err := os.Lstat(filepath.Clean(homeFile)); err == nil {
			backupDest := filepath.Join(backupPath, rel+".bak")
			if !strings.HasPrefix(filepath.Clean(backupDest), filepath.Clean(backupPath)) {
				slog.WarnContext(ctx, "workspace: skipping backup, path traversal detected", "rel", rel)
			} else if mkErr := os.MkdirAll(filepath.Dir(backupDest), 0o755); mkErr != nil {
				slog.WarnContext(ctx, "workspace: creating backup directory, skipping backup", "err", mkErr)
			} else if err := cmdexec.RunWithLogger(ctx, logger, "cp", "-HpR", homeFile, backupDest); err == nil {
				backed++
			}
			_ = os.RemoveAll(filepath.Clean(homeFile))
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
	})
	if walkErr != nil {
		slog.WarnContext(ctx, "workspace: walking dotfiles", "err", walkErr)
		return fmt.Errorf("walking dotfiles: %w", walkErr)
	}
	common.InfoContextf(ctx, logger, "  Linked: %s Skipped: %s Backed up: %s", strconv.Itoa(linked), strconv.Itoa(skipped), strconv.Itoa(backed))
	return nil
}

// SyncSSHConfig installs the SSH config symlink into ~/.ssh/config.
func SyncSSHConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: SyncSSHConfig")
	if common.IsWorkLaptop() {
		return nil
	}
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if err := os.MkdirAll(filepath.Clean(sshDir), 0o700); err != nil {
		slog.WarnContext(ctx, "workspace: creating ssh dir", "err", err)
		return fmt.Errorf("creating ssh dir: %w", err)
	}
	if err := os.Chmod(filepath.Clean(sshDir), 0o700); err != nil {
		slog.WarnContext(ctx, "workspace: setting ssh dir permissions", "err", err)
		return fmt.Errorf("setting ssh dir permissions: %w", err)
	}

	src := filepath.Join(dotfiles, "lib", "ssh", "config")
	dst := filepath.Join(sshDir, "config")
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.WarnContext(ctx, "workspace: checking ssh config source", "err", err)
		return fmt.Errorf("checking ssh config source: %w", err)
	}
	_ = os.Remove(filepath.Clean(dst))
	if err := os.Symlink(src, dst); err != nil {
		slog.WarnContext(ctx, "workspace: creating ssh config symlink", "err", err)
		return fmt.Errorf("creating ssh config symlink: %w", err)
	}
	if err := os.Chmod(src, 0o600); err != nil {
		slog.WarnContext(ctx, "workspace: setting ssh config permissions", "err", err)
		return fmt.Errorf("setting ssh config permissions: %w", err)
	}
	return nil
}

// UpdateAuthorizedKeys fetches SSH public keys from GitHub and appends any new ones to ~/.ssh/authorized_keys.
func UpdateAuthorizedKeys(ctx context.Context, skipNetwork bool, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: UpdateAuthorizedKeys")
	if common.IsWorkLaptop() || skipNetwork {
		return nil
	}

	home := os.Getenv("HOME")
	httpTmp := filepath.Join(home, ".ssh", "authorized_keys.tmp")
	if err := cmdexec.RunWithLogger(ctx, logger, "curl", "-fsSL", "https://github.com/agoodkind.keys", "-o", httpTmp); err != nil {
		slog.WarnContext(ctx, "workspace: fetching authorized keys", "err", err)
		return fmt.Errorf("fetching authorized keys: %w", err)
	}
	defer os.Remove(httpTmp)
	if err := os.MkdirAll(filepath.Clean(filepath.Join(home, ".ssh")), 0o700); err != nil {
		slog.WarnContext(ctx, "workspace: creating ssh dir", "err", err)
		return fmt.Errorf("creating ssh dir: %w", err)
	}
	if err := common.Touch(filepath.Join(home, ".ssh", "authorized_keys")); err != nil {
		slog.WarnContext(ctx, "workspace: touching authorized_keys", "err", err)
		return fmt.Errorf("touching authorized_keys: %w", err)
	}

	authorizedPath := filepath.Join(home, ".ssh", "authorized_keys")
	rawExisting, err := os.ReadFile(filepath.Clean(authorizedPath))
	if err != nil {
		slog.WarnContext(ctx, "workspace: reading authorized_keys", "err", err)
		return fmt.Errorf("reading authorized_keys: %w", err)
	}
	existing := map[string]struct{}{}
	for line := range strings.SplitSeq(string(rawExisting), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			existing[line] = struct{}{}
		}
	}

	downloaded, err := os.ReadFile(filepath.Clean(httpTmp))
	if err != nil {
		slog.WarnContext(ctx, "workspace: reading downloaded keys", "err", err)
		return fmt.Errorf("reading downloaded keys: %w", err)
	}
	added := 0
	for line := range strings.SplitSeq(string(downloaded), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := existing[line]; ok {
			continue
		}
		existing[line] = struct{}{}
		f, err := os.OpenFile(filepath.Clean(authorizedPath), os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			slog.WarnContext(ctx, "workspace: opening authorized_keys for append", "err", err)
			return fmt.Errorf("opening authorized_keys for append: %w", err)
		}
		_, _ = fmt.Fprintln(f, line)
		_ = f.Close()
		added++
	}
	common.InfoContextf(ctx, logger, "  Added %s authorized key(s)", strconv.Itoa(added))
	return nil
}

// SyncCursorConfig compiles and syncs Cursor editor configuration files.
func SyncCursorConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	cursorDir := filepath.Join(os.Getenv("HOME"), ".cursor")
	source := compilation.ResolveAgentSource(dotfiles)

	if err := compilation.EnsureCursorCompatibilityLink(dotfiles); err != nil {
		slog.WarnContext(ctx, "workspace: ensuring cursor compatibility link", "err", err)
		return fmt.Errorf("ensuring cursor compatibility link: %w", err)
	}

	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(cursorDir, "skills")); err != nil {
		slog.WarnContext(ctx, "workspace: syncing cursor skills", "err", err)
		return fmt.Errorf("syncing cursor skills: %w", err)
	}
	if err := compilation.SyncRulesFromDir(source.Rules, filepath.Join(cursorDir, "rules")); err != nil {
		slog.WarnContext(ctx, "workspace: syncing cursor rules", "err", err)
		return fmt.Errorf("syncing cursor rules: %w", err)
	}

	if extra := os.Getenv("CURSOR_EXTRA_RULE_DIRS"); extra != "" {
		extraDirs := strings.Split(extra, ":")
		sort.Strings(extraDirs)
		for _, dir := range extraDirs {
			dir = strings.TrimSpace(dir)
			if dir == "" {
				continue
			}
			if err := compilation.SyncRulesFromDir(dir, filepath.Join(cursorDir, "rules")); err != nil {
				slog.WarnContext(ctx, "workspace: syncing extra cursor rules from", "err", err)
				return fmt.Errorf("syncing extra cursor rules from %s: %w", dir, err)
			}
		}
	}
	return nil
}

// SyncCursorUserRules syncs user rules to the Cursor editor on macOS.
func SyncCursorUserRules(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	if _, err := os.Stat(filepath.Clean(filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Cursor", "User", "globalStorage", "state.vscdb"))); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.WarnContext(ctx, "workspace: checking cursor state.vscdb", "err", err)
		return fmt.Errorf("checking cursor state.vscdb: %w", err)
	}
	logging.ConfigureWithLogger(logger)
	if err := syncer.Run(); err != nil {
		slog.WarnContext(ctx, "workspace: running cursor syncer", "err", err)
		return fmt.Errorf("running cursor syncer: %w", err)
	}
	return nil
}

// SyncClaudeConfig compiles and syncs Claude AI configuration files.
func SyncClaudeConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	claudeDir := filepath.Join(os.Getenv("HOME"), ".claude")
	source := compilation.ResolveAgentSource(dotfiles)

	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(claudeDir, "skills")); err != nil {
		slog.WarnContext(ctx, "workspace: syncing claude skills", "err", err)
		return fmt.Errorf("syncing claude skills: %w", err)
	}
	if err := compilation.SyncRulesFromDirAsMd(source.Rules, filepath.Join(claudeDir, "rules")); err != nil {
		slog.WarnContext(ctx, "workspace: syncing claude rules", "err", err)
		return fmt.Errorf("syncing claude rules: %w", err)
	}
	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(claudeDir, "CLAUDE.md"), "Claude Memory"); err != nil {
		slog.WarnContext(ctx, "workspace: rendering claude instruction doc", "err", err)
		return fmt.Errorf("rendering claude instruction doc: %w", err)
	}
	return nil
}

// SyncCodexConfig compiles and syncs Codex configuration files.
func SyncCodexConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	_ = ctx
	_ = logger

	homeDir := os.Getenv("HOME")
	codexDir := filepath.Join(homeDir, ".codex")
	agentsDir := filepath.Join(homeDir, ".agents")
	source := compilation.ResolveAgentSource(dotfiles)

	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(agentsDir, "skills")); err != nil {
		slog.WarnContext(ctx, "workspace: syncing agents skills", "err", err)
		return fmt.Errorf("syncing agents skills: %w", err)
	}
	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(codexDir, "skills")); err != nil {
		slog.WarnContext(ctx, "workspace: syncing codex skills", "err", err)
		return fmt.Errorf("syncing codex skills: %w", err)
	}
	if err := compilation.RenderCodexRules(source.Rules, filepath.Join(codexDir, "rules", "dotfiles.rules")); err != nil {
		slog.WarnContext(ctx, "workspace: rendering codex rules", "err", err)
		return fmt.Errorf("rendering codex rules: %w", err)
	}
	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(codexDir, "AGENTS.md"), "Codex Instructions"); err != nil {
		slog.WarnContext(ctx, "workspace: rendering codex instruction doc", "err", err)
		return fmt.Errorf("rendering codex instruction doc: %w", err)
	}

	return nil
}

// SyncGeminiConfig compiles and syncs Gemini configuration files.
func SyncGeminiConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	_ = ctx
	_ = logger

	source := compilation.ResolveAgentSource(dotfiles)
	geminiDir := filepath.Join(os.Getenv("HOME"), ".gemini")

	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(geminiDir, "GEMINI.md"), "Gemini Instructions"); err != nil {
		slog.WarnContext(ctx, "workspace: rendering gemini instruction doc", "err", err)
		return fmt.Errorf("rendering gemini instruction doc: %w", err)
	}

	return nil
}

// SyncCopilotConfig compiles and syncs GitHub Copilot configuration files.
func SyncCopilotConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	_ = ctx
	_ = logger

	source := compilation.ResolveAgentSource(dotfiles)
	githubDir := filepath.Join(dotfiles, ".github")

	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(dotfiles, "AGENTS.md"), "Agent Instructions"); err != nil {
		slog.WarnContext(ctx, "workspace: rendering AGENTS.md", "err", err)
		return fmt.Errorf("rendering AGENTS.md: %w", err)
	}
	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(githubDir, "copilot-instructions.md"), "Copilot Instructions"); err != nil {
		slog.WarnContext(ctx, "workspace: rendering copilot instructions", "err", err)
		return fmt.Errorf("rendering copilot instructions: %w", err)
	}
	if err := compilation.RenderCopilotInstructionFiles(source.Rules, filepath.Join(githubDir, "instructions")); err != nil {
		slog.WarnContext(ctx, "workspace: rendering copilot instruction files", "err", err)
		return fmt.Errorf("rendering copilot instruction files: %w", err)
	}
	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(githubDir, "skills")); err != nil {
		slog.WarnContext(ctx, "workspace: syncing github skills", "err", err)
		return fmt.Errorf("syncing github skills: %w", err)
	}
	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(os.Getenv("HOME"), ".copilot", "skills")); err != nil {
		slog.WarnContext(ctx, "workspace: syncing copilot skills", "err", err)
		return fmt.Errorf("syncing copilot skills: %w", err)
	}
	return nil
}

// SyncGitHooks installs git hook symlinks from .githooks/ into .git/hooks/.
func SyncGitHooks(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: SyncGitHooks")
	hooksPath := filepath.Join(dotfiles, ".githooks")
	destination := filepath.Join(dotfiles, ".git", "hooks")
	if _, err := os.Stat(hooksPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.WarnContext(ctx, "workspace: checking git hooks dir", "err", err)
		return fmt.Errorf("checking git hooks dir: %w", err)
	}
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
		if err := os.Symlink(filepath.Join("..", "..", ".githooks", entry.Name()), target); err != nil {
			slog.WarnContext(ctx, "workspace: creating hook symlink", "err", err)
			return fmt.Errorf("creating hook symlink %s: %w", target, err)
		}
	}
	return nil
}

// SyncGlobalGitHooks configures git to use the global hooks directory.
func SyncGlobalGitHooks(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	hooks := filepath.Join(dotfiles, "git-global-hooks")
	if _, err := os.Stat(hooks); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.WarnContext(ctx, "workspace: checking global git hooks dir", "err", err)
		return fmt.Errorf("checking global git hooks dir: %w", err)
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "git", "config", "--global", "core.hooksPath", hooks); err != nil {
		slog.WarnContext(ctx, "workspace: setting git global hooks path", "err", err)
		return fmt.Errorf("setting git global hooks path: %w", err)
	}
	return nil
}

// CheckGitHooksPath verifies the local git hooks path configuration.
func CheckGitHooksPath(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	if configured, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "config", "--local", "--get", "core.hooksPath"); err == nil {
		_ = strings.TrimSpace(configured)
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
	for _, line := range strings.Split(output, "\n") {
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
		return fmt.Errorf("one or more zinit plugins are detached")
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
		return true, false, fmt.Errorf("reading git HEAD: %w", err)
	}
	headRef := strings.TrimSpace(string(headContent))
	return true, !strings.HasPrefix(headRef, "ref: "), nil
}

func zinitPluginHeadPath(pluginDir string) (string, bool, error) {
	gitPath := filepath.Join(pluginDir, ".git")
	gitInfo, err := os.Stat(gitPath)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if gitInfo.IsDir() {
		return filepath.Join(gitPath, "HEAD"), true, nil
	}

	gitFileContent, err := os.ReadFile(gitPath)
	if err != nil {
		return "", true, fmt.Errorf("reading .git file: %w", err)
	}
	const gitDirPrefix = "gitdir: "
	gitDirLine := strings.TrimSpace(string(gitFileContent))
	if !strings.HasPrefix(gitDirLine, gitDirPrefix) {
		return "", true, fmt.Errorf("invalid .git file")
	}

	gitDir := strings.TrimSpace(strings.TrimPrefix(gitDirLine, gitDirPrefix))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(pluginDir, gitDir)
	}
	return filepath.Join(filepath.Clean(gitDir), "HEAD"), true, nil
}

// UpdateZinitPlugins runs zinit self-update and updates all zinit plugins.
func UpdateZinitPlugins(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	if !runner.HasCommand("zsh") {
		return fmt.Errorf("zsh is not available")
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
		common.InfoContext(ctx, logger, "  [zinit-update-status: ignored nonzero update exit after compile and verify]")
	}
	return nil
}

// CleanupHomebrewRepair removes incomplete Homebrew downloads and runs brew cleanup.
func CleanupHomebrewRepair(ctx context.Context, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: CleanupHomebrewRepair")
	if runtime.GOOS != "darwin" {
		return nil
	}
	if !runner.HasCommand("brew") {
		return nil
	}
	cacheDir := filepath.Join(os.Getenv("HOME"), "Library", "Caches", "Homebrew", "downloads")
	if matches, err := filepath.Glob(filepath.Join(cacheDir, "*.incomplete")); err == nil {
		for _, match := range matches {
			_ = os.RemoveAll(filepath.Clean(match))
		}
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "brew", "cleanup", "--prune=all"); err != nil {
		slog.WarnContext(ctx, "workspace: running brew cleanup", "err", err)
		return fmt.Errorf("running brew cleanup: %w", err)
	}
	return nil
}

// CleanupNeovimRepair removes stale Neovim plugin and swap file directories.
func CleanupNeovimRepair(ctx context.Context, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: CleanupNeovimRepair")
	nvimData := filepath.Join(xdgDataDir(), "nvim")
	lazyDir := filepath.Join(nvimData, "lazy")
	if entries, err := os.ReadDir(lazyDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasSuffix(name, ".cloning") {
				_ = os.RemoveAll(filepath.Clean(filepath.Join(lazyDir, name)))
			}
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			entries2, err := os.ReadDir(filepath.Join(lazyDir, entry.Name()))
			if err != nil || len(entries2) == 0 {
				_ = os.RemoveAll(filepath.Clean(filepath.Join(lazyDir, entry.Name())))
			}
		}
	}

	swapDirs := []string{
		filepath.Join(xdgStateDir(), "nvim", "swap"),
		filepath.Join(xdgDataDir(), "nvim", "swap"),
	}
	for _, dir := range swapDirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.swp"))
		if err == nil {
			for _, file := range files {
				_ = os.Remove(filepath.Clean(file))
			}
		}
	}
	return nil
}

// UpdateNeovimPlugins runs the Neovim lazy plugin sync and tree-sitter parser installs.
func UpdateNeovimPlugins(ctx context.Context, logger *telemetry.Logger) error {
	if !runner.HasCommand("nvim") {
		return nil
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "nvim", "--headless", "-c", "lua require('lazy').sync()", "-c", "qa"); err != nil {
		slog.WarnContext(ctx, "workspace: running neovim lazy sync", "err", err)
		return fmt.Errorf("running neovim lazy sync: %w", err)
	}
	if !runner.HasCommand("tree-sitter") {
		return nil
	}
	if versionOutput, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "tree-sitter", "--version"); err == nil {
		parts := strings.Fields(versionOutput)
		if len(parts) >= 2 {
			version := strings.TrimPrefix(parts[1], "v")
			if common.VersionAtLeast(version, "0.21.0") {
				common.InfoContext(ctx, logger, "  treesitter parsers")
				_ = cmdexec.RunWithLogger(ctx, logger, "nvim", "--headless", "+lua require('nvim-treesitter').install({'bash','lua','vim','vimdoc','python','javascript','typescript','json','yaml'})", "+sleep 10", "+qa")
			}
		}
	}
	return nil
}
