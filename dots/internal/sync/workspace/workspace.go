package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/cursor/logging"
	"goodkind.io/.dotfiles/internal/cursor/syncer"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/sync/compilation"
	"goodkind.io/.dotfiles/internal/telemetry"
)

func CleanupZinitCompletions(ctx context.Context, logger *telemetry.Logger) error {
	completionsDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "zinit", "completions")
	entries, err := os.ReadDir(completionsDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		path := filepath.Join(completionsDir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if _, err := os.Stat(path); err != nil {
				_ = os.Remove(path)
			}
		}
	}
	return nil
}

func LinkDotfiles(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	homeDir := os.Getenv("HOME")
	backupPath := filepath.Join(dotfiles, "backups", time.Now().Format("20060102_150405"))
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
			return err
		}
		homeFile := filepath.Join(homeDir, rel)

		if common.IsSymlinkTo(homeFile, path) {
			skipped++
			return nil
		}

		if _, err := os.Lstat(homeFile); err == nil {
			if err := os.MkdirAll(filepath.Dir(filepath.Join(backupPath, rel)), 0o755); err != nil {
				return err
			}
			if err := cmdexec.RunWithLogger(context.Background(), logger, "cp", "-HpR", homeFile, filepath.Join(backupPath, rel+".bak")); err == nil {
				backed++
			}
			_ = os.RemoveAll(homeFile)
		}

		if err := os.MkdirAll(filepath.Dir(homeFile), 0o755); err != nil {
			return err
		}
		_ = os.Remove(homeFile)
		if err := os.Symlink(path, homeFile); err != nil {
			return err
		}
		linked++
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	common.Infof(logger, "  Linked: %d Skipped: %d Backed up: %d", linked, skipped, backed)
	return nil
}

func SyncSSHConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	if common.IsWorkLaptop() {
		return nil
	}
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(sshDir, 0o700); err != nil {
		return err
	}

	src := filepath.Join(dotfiles, "lib", "ssh", "config")
	dst := filepath.Join(sshDir, "config")
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	_ = os.Remove(dst)
	if err := os.Symlink(src, dst); err != nil {
		return err
	}
	return os.Chmod(src, 0o600)
}

func UpdateAuthorizedKeys(ctx context.Context, skipNetwork bool, logger *telemetry.Logger) error {
	if common.IsWorkLaptop() || skipNetwork {
		return nil
	}

	home := os.Getenv("HOME")
	httpTmp := filepath.Join(home, ".ssh", "authorized_keys.tmp")
	if err := cmdexec.RunWithLogger(context.Background(), logger, "curl", "-fsSL", "https://github.com/agoodkind.keys", "-o", httpTmp); err != nil {
		return err
	}
	defer os.Remove(httpTmp)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		return err
	}
	if err := common.Touch(filepath.Join(home, ".ssh", "authorized_keys")); err != nil {
		return err
	}

	authorizedPath := filepath.Join(home, ".ssh", "authorized_keys")
	rawExisting, err := os.ReadFile(authorizedPath)
	if err != nil {
		return err
	}
	existing := map[string]struct{}{}
	for _, line := range strings.Split(string(rawExisting), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			existing[line] = struct{}{}
		}
	}

	downloaded, err := os.ReadFile(httpTmp)
	if err != nil {
		return err
	}
	added := 0
	for _, line := range strings.Split(string(downloaded), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := existing[line]; ok {
			continue
		}
		existing[line] = struct{}{}
		f, err := os.OpenFile(authorizedPath, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(f, line)
		_ = f.Close()
		added++
	}
	common.Infof(logger, "  Added %d authorized key(s)", added)
	return nil
}

func SyncCursorConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	cursorDir := filepath.Join(os.Getenv("HOME"), ".cursor")
	source := compilation.ResolveAgentSource(dotfiles)

	if err := compilation.EnsureCursorCompatibilityLink(dotfiles); err != nil {
		return err
	}

	if err := compilation.SyncFilesToDir(source.Commands, filepath.Join(cursorDir, "commands")); err != nil {
		return err
	}
	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(cursorDir, "skills")); err != nil {
		return err
	}
	if err := compilation.SyncRulesFromDir(source.Rules, filepath.Join(cursorDir, "rules")); err != nil {
		return err
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
				return err
			}
		}
	}
	return nil
}

func SyncCursorUserRules(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "Cursor", "User", "globalStorage", "state.vscdb")); err != nil {
		return nil
	}
	logging.ConfigureWithLogger(logger)
	if err := syncer.Run(); err != nil {
		return err
	}
	return nil
}

func SyncClaudeConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	claudeDir := filepath.Join(os.Getenv("HOME"), ".claude")
	source := compilation.ResolveAgentSource(dotfiles)

	if err := compilation.SyncFilesToDir(source.Commands, filepath.Join(claudeDir, "commands")); err != nil {
		return err
	}
	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(claudeDir, "skills")); err != nil {
		return err
	}
	if err := compilation.SyncRulesFromDirAsMd(source.Rules, filepath.Join(claudeDir, "rules")); err != nil {
		return err
	}
	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(claudeDir, "CLAUDE.md"), "Claude Memory"); err != nil {
		return err
	}
	return nil
}

func SyncCodexConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	_ = ctx
	_ = logger

	codexDir := filepath.Join(os.Getenv("HOME"), ".codex")
	agentsDir := filepath.Join(os.Getenv("HOME"), ".agents")
	source := compilation.ResolveAgentSource(dotfiles)

	if err := compilation.SyncFilesToDir(source.Commands, filepath.Join(agentsDir, "commands")); err != nil {
		return err
	}
	if err := compilation.SyncRulesFromDir(source.Rules, filepath.Join(agentsDir, "rules")); err != nil {
		return err
	}
	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(agentsDir, "skills")); err != nil {
		return err
	}
	if err := compilation.SyncCommandFilesAsSkillDirs(source.Commands, filepath.Join(agentsDir, "skills"), "cursor-command-"); err != nil {
		return err
	}
	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(codexDir, "AGENTS.md"), "Codex Instructions"); err != nil {
		return err
	}

	return nil
}

func SyncCopilotConfig(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	_ = ctx
	_ = logger

	source := compilation.ResolveAgentSource(dotfiles)
	githubDir := filepath.Join(dotfiles, ".github")

	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(dotfiles, "AGENTS.md"), "Agent Instructions"); err != nil {
		return err
	}
	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(githubDir, "copilot-instructions.md"), "Copilot Instructions"); err != nil {
		return err
	}
	if err := compilation.RenderCopilotInstructionFiles(source.Rules, filepath.Join(githubDir, "instructions")); err != nil {
		return err
	}
	if err := compilation.RenderCopilotPromptFiles(source.Commands, filepath.Join(githubDir, "prompts")); err != nil {
		return err
	}
	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(githubDir, "skills")); err != nil {
		return err
	}
	return compilation.SyncSkillDirs(source.Skills, filepath.Join(os.Getenv("HOME"), ".copilot", "skills"))
}

func SyncGitHooks(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	hooksPath := filepath.Join(dotfiles, ".githooks")
	destination := filepath.Join(dotfiles, ".git", "hooks")
	if _, err := os.Stat(hooksPath); err != nil {
		return nil
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(hooksPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		target := filepath.Join(destination, entry.Name())
		_ = os.Remove(target)
		if err := os.Symlink(filepath.Join("..", "..", ".githooks", entry.Name()), target); err != nil {
			return err
		}
	}
	return nil
}

func SyncGlobalGitHooks(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	hooks := filepath.Join(dotfiles, "git-global-hooks")
	if _, err := os.Stat(hooks); err != nil {
		return nil
	}
	return cmdexec.RunWithLogger(context.Background(), logger, "git", "config", "--global", "core.hooksPath", hooks)
}

func CheckGitHooksPath(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	configured, err := cmdexec.OutputWithLoggerAndEnv(context.Background(), logger, nil, "git", "-C", dotfiles, "config", "--local", "--get", "core.hooksPath")
	if err != nil {
		return nil
	}
	value := strings.TrimSpace(string(configured))
	if value == "" || value == ".githooks" {
		return nil
	}
	return nil
}

func UpdateZinitPlugins(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	if !runner.HasCommand("zsh") {
		return fmt.Errorf("zsh is not available")
	}
	zinitPath := filepath.Join(dotfiles, "lib", "zinit", "zinit.zsh")
	if _, err := os.Stat(zinitPath); err != nil {
		return nil
	}
	return cmdexec.RunWithLogger(
		ctx,
		logger,
		"zsh",
		"-c",
		"source '$DOTDOTFILES/lib/zinit/zinit.zsh'; zinit self-update; zinit update --all --quiet",
	)
}

func CleanupHomebrewRepair(ctx context.Context, logger *telemetry.Logger) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	if !runner.HasCommand("brew") {
		return nil
	}
	cacheDir := filepath.Join(os.Getenv("HOME"), "Library", "Caches", "Homebrew", "downloads")
	if matches, err := filepath.Glob(filepath.Join(cacheDir, "*.incomplete")); err == nil {
		for _, match := range matches {
			_ = os.RemoveAll(match)
		}
	}
	return cmdexec.RunWithLogger(context.Background(), logger, "brew", "cleanup", "--prune=all")
}

func CleanupNeovimRepair(ctx context.Context, logger *telemetry.Logger) error {
	nvimData := filepath.Join(os.Getenv("XDG_DATA_HOME"), "nvim")
	if os.Getenv("XDG_DATA_HOME") == "" {
		nvimData = filepath.Join(os.Getenv("HOME"), ".local", "share", "nvim")
	}
	lazyDir := filepath.Join(nvimData, "lazy")
	if entries, err := os.ReadDir(lazyDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasSuffix(name, ".cloning") {
				_ = os.RemoveAll(filepath.Join(lazyDir, name))
			}
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			entries2, err := os.ReadDir(filepath.Join(lazyDir, entry.Name()))
			if err != nil || len(entries2) == 0 {
				_ = os.RemoveAll(filepath.Join(lazyDir, entry.Name()))
			}
		}
	}

	swapDirs := []string{
		filepath.Join(os.Getenv("XDG_STATE_HOME"), "nvim", "swap"),
		filepath.Join(os.Getenv("HOME"), ".local", "share", "nvim", "swap"),
	}
	for _, dir := range swapDirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.swp"))
		if err == nil {
			for _, file := range files {
				_ = os.Remove(file)
			}
		}
	}
	return nil
}

func UpdateNeovimPlugins(ctx context.Context, logger *telemetry.Logger) error {
	if !runner.HasCommand("nvim") {
		return nil
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "nvim", "--headless", "-c", "lua require('lazy').sync()", "-c", "qa"); err != nil {
		return err
	}
	if !runner.HasCommand("tree-sitter") {
		return nil
	}
	versionOutput, err := cmdexec.OutputWithLoggerAndEnv(context.Background(), logger, nil, "tree-sitter", "--version")
	if err != nil {
		return nil
	}
	parts := strings.Fields(string(versionOutput))
	if len(parts) < 2 {
		return nil
	}
	version := strings.TrimPrefix(parts[1], "v")
	if common.VersionAtLeast(version, "0.21.0") {
		common.Info(logger, "  treesitter parsers")
		_ = cmdexec.RunWithLogger(context.Background(), logger, "nvim", "--headless", "+lua require('nvim-treesitter').install({'bash','lua','vim','vimdoc','python','javascript','typescript','json','yaml'})", "+sleep 10", "+qa")
	}
	return nil
}
