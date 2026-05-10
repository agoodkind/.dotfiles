// Package uninstall implements dotfiles uninstallation routines.
package uninstall

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/compilation"
	"goodkind.io/.dotfiles/internal/telemetry"
)

var uninstallLogger *telemetry.Logger

type uninstallFlag string

const (
	flagPurgePackages uninstallFlag = "--purge-packages"
	flagHelp          uninstallFlag = "--help"
	flagHelpShort     uninstallFlag = "-h"
)

// Run executes the dotfiles uninstall workflow with the given arguments.
func Run(ctx context.Context, args ...string) error {
	logPath := filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "uninstall.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		return fmt.Errorf("creating uninstall logger: %w", err)
	}
	uninstallLogger = logger
	defer logger.Close()
	defer func() {
		uninstallLogger = nil
		runner.SetLogger(nil)
	}()
	runner.SetLogger(logger)
	_ = os.Setenv("DOTFILES_LOG", logPath)
	done := logger.SectionContext(ctx, "Uninstall")
	defer done()

	purgePackages := false
	for _, arg := range args {
		switch uninstallFlag(arg) {
		case flagPurgePackages:
			purgePackages = true
		case flagHelp, flagHelpShort:
			printUninstallUsage(ctx)
			return nil
		default:
			return fmt.Errorf("unsupported uninstall flag: %s", arg)
		}
	}

	if err := runUninstall(ctx, purgePackages); err != nil {
		slog.WarnContext(ctx, "Uninstall flow failed", "err", err)
		uninstallLogger.WarnContextWithErr(ctx, "Uninstall flow failed", err)
		return fmt.Errorf("uninstall flow failed: %w", err)
	}
	return nil
}

func printUninstallUsage(ctx context.Context) {
	logInfo(ctx, "Usage: dots uninstall [--purge-packages]")
}

type packageLists struct {
	common    []string
	brew      []string
	apt       []string
	snap      []string
	brewCasks []string
}

func runUninstall(ctx context.Context, purgePackages bool) error {
	if err := printUninstallBanner(ctx, purgePackages); err != nil {
		return err
	}
	if !promptYesNo(ctx, "Continue with uninstall? (y/n) ") {
		logInfo(ctx, "Uninstall cancelled")
		return nil
	}

	logInfo(ctx, "")

	dotfiles := os.Getenv("DOTDOTFILES")
	if dotfiles == "" {
		dotfiles = filepath.Join(os.Getenv("HOME"), ".dotfiles")
	}

	if err := removeHomeSymlinks(ctx, dotfiles); err != nil {
		return err
	}
	removeSSHSymlink(ctx, dotfiles)
	removeCursorConfig(ctx, dotfiles)
	removeClaudeConfig(ctx, dotfiles)
	removeCodexConfig(ctx, dotfiles)
	removeCopilotConfig(ctx, dotfiles)
	removeGitConfig(ctx, dotfiles)
	removeCacheFiles()
	removeHushlogin(ctx)
	removeBackups(ctx, dotfiles)
	if err := removePackages(ctx, purgePackages); err != nil {
		return err
	}

	logInfo(ctx, "")
	logInfo(ctx, "Uninstall complete!")
	logInfof(ctx, "The dotfiles directory (%s) was NOT removed", dotfiles)
	if !purgePackages {
		logInfo(ctx, "Installed packages were NOT removed (use --purge-packages)")
	}
	logInfo(ctx, "To fully remove, run: rm -rf "+dotfiles)
	return nil
}

func printUninstallBanner(ctx context.Context, purgePackages bool) error {
	if purgePackages {
		logInfo(ctx, "╔═══════════════════════════════════════════╗")
		logInfo(ctx, "║         Dotfiles Uninstaller              ║")
		logInfo(ctx, "║  This will remove symlinks & configs      ║")
		logInfo(ctx, "║  ⚠️  PACKAGES WILL ALSO BE REMOVED ⚠️      ║")
		logInfo(ctx, "╚═══════════════════════════════════════════╝")
		return nil
	}

	logInfo(ctx, "╔═══════════════════════════════════════════╗")
	logInfo(ctx, "║         Dotfiles Uninstaller              ║")
	logInfo(ctx, "║  This will remove symlinks & configs      ║")
	logInfo(ctx, "║  Packages will NOT be removed             ║")
	logInfo(ctx, "║  Use --purge-packages to remove them      ║")
	logInfo(ctx, "╚═══════════════════════════════════════════╝")
	return nil
}

func promptYesNo(ctx context.Context, prompt string) bool {
	logInfo(ctx, prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func removeHomeSymlinks(ctx context.Context, dotfiles string) error {
	logInfo(ctx, "Removing home directory symlinks...")
	homeDir := filepath.Join(dotfiles, "home")
	if _, err := os.Stat(filepath.Clean(homeDir)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.WarnContext(ctx, "uninstall: stat home symlinks dir", slog.Any("error", err))
		return fmt.Errorf("checking home symlinks directory: %w", err)
	}

	if err := filepath.WalkDir(filepath.Clean(homeDir), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			slog.WarnContext(ctx, "uninstall: walking home dir entry", slog.Any("error", err))
			return fmt.Errorf("walking %s: %w", path, err)
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(homeDir, path)
		if err != nil {
			slog.WarnContext(ctx, "uninstall: computing relative path", slog.Any("error", err))
			return fmt.Errorf("computing relative path for %s: %w", path, err)
		}
		target := filepath.Join(os.Getenv("HOME"), relative)
		return removeDotfilesSymlink(ctx, target, dotfiles)
	}); err != nil {
		slog.WarnContext(ctx, "uninstall: walk home dir", slog.Any("error", err))
		return fmt.Errorf("walking home dir %s: %w", homeDir, err)
	}
	return nil
}

func removeSSHSymlink(ctx context.Context, dotfiles string) {
	if err := removeDotfilesSymlink(ctx, filepath.Join(os.Getenv("HOME"), ".ssh", "config"), dotfiles); err != nil {
		logInfo(ctx, "Skipping ~/.ssh/config: "+err.Error())
	}
}

func removeCursorConfig(ctx context.Context, dotfiles string) {
	logInfo(ctx, "Removing Cursor configuration...")
	cursorDir := filepath.Join(os.Getenv("HOME"), ".cursor")
	removeDotfilesSymlinksInDir(ctx, filepath.Join(cursorDir, "commands"), dotfiles)
	removeDotfilesSymlinksInDir(ctx, filepath.Join(cursorDir, "skills"), dotfiles)
	removeDotfilesSymlinksInDir(ctx, filepath.Join(cursorDir, "rules"), dotfiles)
}

func removeClaudeConfig(ctx context.Context, dotfiles string) {
	logInfo(ctx, "Removing Claude configuration...")
	claudeDir := filepath.Join(os.Getenv("HOME"), ".claude")
	removeDotfilesSymlinksInDir(ctx, filepath.Join(claudeDir, "commands"), dotfiles)
	removeDotfilesSymlinksInDir(ctx, filepath.Join(claudeDir, "skills"), dotfiles)
	removeDotfilesSymlinksInDir(ctx, filepath.Join(claudeDir, "rules"), dotfiles)
	_ = removeGeneratedFileIfManaged(filepath.Join(claudeDir, "CLAUDE.md"))
}

func removeCodexConfig(ctx context.Context, dotfiles string) {
	logInfo(ctx, "Removing Codex configuration...")
	agentsDir := filepath.Join(os.Getenv("HOME"), ".agents")
	codexDir := filepath.Join(os.Getenv("HOME"), ".codex")
	removeDotfilesSymlinksInDir(ctx, filepath.Join(agentsDir, "skills"), dotfiles)
	removeDotfilesSymlinksInDir(ctx, filepath.Join(codexDir, "skills"), dotfiles)
	_ = removeManagedSkillDirs(filepath.Join(agentsDir, "skills"), "cursor-command-")
	_ = removeManagedSkillDirs(filepath.Join(codexDir, "skills"), "cursor-command-")
	_ = removeGeneratedFileIfManaged(filepath.Join(codexDir, "AGENTS.md"))
	_ = removeGeneratedFileIfManaged(filepath.Join(codexDir, "rules", "dotfiles.rules"))
}

func removeCopilotConfig(ctx context.Context, dotfiles string) {
	logInfo(ctx, "Removing Copilot configuration...")
	githubDir := filepath.Join(dotfiles, ".github")
	removeDotfilesSymlinksInDir(ctx, filepath.Join(githubDir, "skills"), dotfiles)
	removeDotfilesSymlinksInDir(ctx, filepath.Join(os.Getenv("HOME"), ".copilot", "skills"), dotfiles)
	_ = removeGeneratedFileIfManaged(filepath.Join(dotfiles, "AGENTS.md"))
	_ = removeGeneratedFileIfManaged(filepath.Join(githubDir, "copilot-instructions.md"))
	_ = removeGeneratedFilesIfManaged(filepath.Join(githubDir, "instructions"), ".instructions.md")
	_ = removeGeneratedFilesIfManaged(filepath.Join(githubDir, "prompts"), ".prompt.md")
}

func removeGitConfig(ctx context.Context, dotfiles string) {
	logInfo(ctx, "Removing git config include...")
	includePath := filepath.Join(dotfiles, "lib", ".gitconfig_incl")
	output, err := cmdexec.Output(ctx, "git", "config", "--global", "--get", "include.path")
	if err != nil {
		return
	}
	if strings.TrimSpace(output) != includePath {
		return
	}
	_ = cmdexec.Run(ctx, "git", "config", "--global", "--unset", "include.path")
}

func removeCacheFiles() {
	for _, name := range []string{
		"dotfiles_update.lock",
		"dotfiles_update.log",
		"dotfiles_update_error",
		"dotfiles_update_success",
		"dotfiles_debug_enabled",
		"dotfiles_dispatch.flock",
		"dotfiles_dispatch.log",
		"dotfiles_weekly_update",
	} {
		_ = removeIfExists(filepath.Join(os.Getenv("HOME"), ".cache", name))
	}
	_ = removeIfExists(filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_dispatch.lock"))
}

func removeHushlogin(ctx context.Context) {
	hush := filepath.Join(os.Getenv("HOME"), ".hushlogin")
	if _, err := os.Stat(filepath.Clean(hush)); err == nil {
		if promptYesNo(ctx, "Remove .hushlogin (show login message)? (y/n) ") {
			_ = removeIfExists(hush)
		}
	}
}

func removeBackups(ctx context.Context, dotfiles string) {
	backupsDir := filepath.Join(dotfiles, "backups")
	if _, err := os.Stat(filepath.Clean(backupsDir)); err == nil {
		if promptYesNo(ctx, "Remove backups directory? (y/n) ") {
			_ = removeIfExists(backupsDir)
		}
	}
}

func removePackages(ctx context.Context, purge bool) error {
	if !purge {
		return nil
	}
	logInfo(ctx, "Package removal requested...")
	lists, err := parsePackageLists()
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		return removeBrewPackages(ctx, lists)
	}
	if runtime.GOOS == "linux" {
		if err := removeAptPackages(ctx, lists); err != nil {
			return err
		}
		return removeSnapPackages(ctx, lists)
	}
	return nil
}

func parsePackageLists() (*packageLists, error) {
	cfg := catalog.DefaultPackageConfig()
	if cfg == nil {
		return nil, fmt.Errorf("package config is unavailable")
	}
	lists := &packageLists{
		common:    append([]string{}, cfg.CommonPackages...),
		brew:      append([]string{}, cfg.BrewSpecific...),
		apt:       append([]string{}, cfg.AptSpecific...),
		snap:      append([]string{}, cfg.SnapPackages...),
		brewCasks: make([]string, 0, len(cfg.BrewCasks)),
	}
	for cask := range cfg.BrewCasks {
		lists.brewCasks = append(lists.brewCasks, cask)
	}
	return lists, nil
}

func removeBrewPackages(ctx context.Context, lists *packageLists) error {
	if !runner.HasCommand("brew") {
		logWarn(ctx, "Homebrew not installed, skipping package removal")
		return nil
	}

	all := append(append([]string{}, lists.common...), lists.brew...)
	installedFormulae, err := cmdexec.Output(ctx, "brew", "list", "--formula")
	if err != nil {
		slog.WarnContext(ctx, "uninstall: listing brew formulae", slog.Any("error", err))
		return fmt.Errorf("listing installed brew formulae: %w", err)
	}
	toRemove := toInstallableSet(strings.Fields(installedFormulae), all)
	if len(toRemove) > 0 {
		logInfof(ctx, "Will remove %s formulae: %s", strconv.Itoa(len(toRemove)), strings.Join(toRemove, " "))
		if promptYesNo(ctx, "Continue? (y/n) ") {
			args := append([]string{"uninstall", "--force"}, toRemove...)
			_ = cmdexec.Run(ctx, "brew", args...)
		}
	} else {
		logInfo(ctx, "No matching formulae installed")
	}

	installedCasks, err := cmdexec.Output(ctx, "brew", "list", "--cask")
	if err != nil {
		slog.WarnContext(ctx, "uninstall: listing brew casks", slog.Any("error", err))
		return fmt.Errorf("listing installed brew casks: %w", err)
	}
	casksToRemove := toInstallableSet(strings.Fields(installedCasks), lists.brewCasks)
	if len(casksToRemove) > 0 {
		logInfof(ctx, "Will remove %s casks: %s", strconv.Itoa(len(casksToRemove)), strings.Join(casksToRemove, " "))
		if promptYesNo(ctx, "Continue? (y/n) ") {
			args := append([]string{"uninstall", "--cask", "--force"}, casksToRemove...)
			_ = cmdexec.Run(ctx, "brew", args...)
		}
	} else {
		logInfo(ctx, "No matching casks installed")
	}
	return nil
}

func removeAptPackages(ctx context.Context, lists *packageLists) error {
	if !runner.HasCommand("apt-get") {
		logWarn(ctx, "apt-get not found, skipping package removal")
		return nil
	}
	all := append(append([]string{}, lists.common...), lists.apt...)
	toRemove := make([]string, 0)
	for _, pkg := range deduplicate(all) {
		if _, err := cmdexec.Output(ctx, "dpkg", "-s", pkg); err == nil {
			toRemove = append(toRemove, pkg)
		}
	}
	if len(toRemove) == 0 {
		logInfo(ctx, "No matching APT packages installed")
		return nil
	}
	logInfof(ctx, "Will remove %s APT packages", strconv.Itoa(len(toRemove)))
	if !promptYesNo(ctx, "Continue? (y/n) ") {
		return nil
	}
	args := append([]string{"apt-get", "remove", "-y"}, toRemove...)
	_ = cmdexec.Run(ctx, "sudo", args...)
	_ = cmdexec.Run(ctx, "sudo", "apt-get", "autoremove", "-y")
	return nil
}

func removeSnapPackages(ctx context.Context, lists *packageLists) error {
	if !runner.HasCommand("snap") {
		return nil
	}
	toRemove := make([]string, 0)
	for _, pkg := range deduplicate(lists.snap) {
		if _, err := cmdexec.Output(ctx, "snap", "list", pkg); err == nil {
			toRemove = append(toRemove, pkg)
		}
	}
	if len(toRemove) == 0 {
		logInfo(ctx, "No matching Snap packages installed")
		return nil
	}
	logInfof(ctx, "Will remove %s Snap packages: %s", strconv.Itoa(len(toRemove)), strings.Join(toRemove, ", "))
	if !promptYesNo(ctx, "Continue? (y/n) ") {
		return nil
	}
	for _, pkg := range toRemove {
		_ = cmdexec.Run(ctx, "sudo", "snap", "remove", pkg)
	}
	return nil
}

func removeDotfilesSymlink(ctx context.Context, target string, dotfiles string) error {
	fi, err := os.Lstat(filepath.Clean(target))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.WarnContext(ctx, "Failed to read symlink target", "err", err)
		uninstallLogger.WarnContextWithErr(ctx, "Failed to read symlink target", err)
		return fmt.Errorf("read target: %w", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("not a symlink")
	}
	link, err := os.Readlink(target)
	if err != nil {
		return fmt.Errorf("reading symlink %s: %w", target, err)
	}
	dotfilesRoot := filepath.Clean(dotfiles)
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(target), link)
	}
	cleanLink := filepath.Clean(link)
	if cleanLink == dotfilesRoot || strings.HasPrefix(cleanLink, dotfilesRoot+string(filepath.Separator)) {
		if err := os.Remove(filepath.Clean(target)); err != nil {
			slog.WarnContext(ctx, "Failed to remove dotfiles symlink", "err", err)
			uninstallLogger.WarnContextWithErr(ctx, "Failed to remove dotfiles symlink", err)
			return fmt.Errorf("remove symlink: %w", err)
		}
		logDebugf(ctx, "Removed symlink: %s", target)
		return nil
	}
	logInfof(ctx, "Skipping %s (not a dotfiles symlink)", target)
	return nil
}

func removeDotfilesSymlinksInDir(ctx context.Context, dir string, dotfiles string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		_ = removeDotfilesSymlink(ctx, filepath.Join(dir, entry.Name()), dotfiles)
	}
}

func removeGeneratedFileIfManaged(path string) error {
	slog.Info("uninstall: removeGeneratedFileIfManaged")
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.Error("uninstall: removeGeneratedFileIfManaged", "err", err)
		return fmt.Errorf("read file: %w", err)
	}
	if !compilation.HasGeneratedMarker(string(content)) {
		return nil
	}
	if err := os.Remove(filepath.Clean(path)); err != nil {
		return fmt.Errorf("removing file %s: %w", path, err)
	}
	return nil
}

func removeGeneratedFilesIfManaged(dir string, suffix string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.Error("uninstall: removeGeneratedFilesIfManaged", "err", err)
		return fmt.Errorf("read dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		if err := removeGeneratedFileIfManaged(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func removeManagedSkillDirs(dir string, managedPrefix string) error {
	slog.Info("uninstall: removeManagedSkillDirs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.Error("uninstall: removeManagedSkillDirs", "err", err)
		return fmt.Errorf("read dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), managedPrefix) {
			continue
		}
		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		content, readErr := os.ReadFile(filepath.Clean(skillPath))
		if readErr != nil {
			continue
		}
		if !compilation.HasGeneratedMarker(string(content)) {
			continue
		}
		if err := os.RemoveAll(filepath.Clean(filepath.Join(dir, entry.Name()))); err != nil {
			return fmt.Errorf("removing skill directory: %w", err)
		}
	}
	return nil
}

func removeIfExists(path string) error {
	slog.Info("uninstall: removeIfExists")
	if _, err := os.Stat(filepath.Clean(path)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.Error("uninstall: removeIfExists", "err", err)
		return fmt.Errorf("stat path: %w", err)
	}
	if err := os.RemoveAll(filepath.Clean(path)); err != nil {
		return fmt.Errorf("removing path %s: %w", path, err)
	}
	return nil
}

func toInstallableSet(installed, targets []string) []string {
	installedMap := make(map[string]struct{}, len(installed))
	for _, name := range installed {
		installedMap[name] = struct{}{}
	}
	matches := make([]string, 0)
	for _, name := range deduplicate(targets) {
		if _, ok := installedMap[name]; ok {
			matches = append(matches, name)
		}
	}
	sort.Strings(matches)
	return matches
}

func deduplicate(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func logInfo(ctx context.Context, message string) {
	if uninstallLogger != nil {
		uninstallLogger.InfoContext(ctx, message)
	}
}

func logInfof(ctx context.Context, format string, args ...string) {
	if uninstallLogger != nil {
		uninstallLogger.InfoContext(ctx, formatString(format, args...))
	}
}

func logWarn(ctx context.Context, message string) {
	if uninstallLogger != nil {
		uninstallLogger.WarnContext(ctx, message)
	}
}

func logDebugf(ctx context.Context, format string, args ...string) {
	if uninstallLogger != nil {
		uninstallLogger.DebugContext(ctx, formatString(format, args...))
	}
}

func formatString(format string, args ...string) string {
	formatted := format
	for _, arg := range args {
		formatted = strings.Replace(formatted, "%s", arg, 1)
	}
	return formatted
}
