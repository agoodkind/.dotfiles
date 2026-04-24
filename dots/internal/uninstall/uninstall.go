package uninstall

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/agoodkind/.dotfiles/internal/catalog"
	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/runner"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

var uninstallLogger *telemetry.Logger

func Run(ctx context.Context, args ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	logPath := filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "uninstall.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		return err
	}
	uninstallLogger = logger
	defer logger.Close()
	defer func() {
		uninstallLogger = nil
		runner.SetLogger(nil)
	}()
	runner.SetLogger(logger)
	_ = os.Setenv("DOTFILES_LOG", logPath)
	done := logger.Section("Uninstall")
	defer done()

	purgePackages := false
	for _, arg := range args {
		switch arg {
		case "--purge-packages":
			purgePackages = true
		case "--help", "-h":
			printUninstallUsage()
			return nil
		default:
			return fmt.Errorf("unsupported uninstall flag: %s", arg)
		}
	}

	if err := runUninstall(ctx, purgePackages); err != nil {
		return fmt.Errorf("uninstall flow failed: %w", err)
	}
	return nil
}

func printUninstallUsage() {
	logInfo("Usage: dots uninstall [--purge-packages]")
}

type packageLists struct {
	common    []string
	brew      []string
	apt       []string
	snap      []string
	brewCasks []string
}

func runUninstall(ctx context.Context, purgePackages bool) error {
	if err := printUninstallBanner(purgePackages); err != nil {
		return err
	}
	if !promptYesNo("Continue with uninstall? (y/n) ") {
		logInfo("Uninstall cancelled")
		return nil
	}

	logInfo("")

	dotfiles := os.Getenv("DOTDOTFILES")
	if dotfiles == "" {
		dotfiles = filepath.Join(os.Getenv("HOME"), ".dotfiles")
	}

	if err := removeHomeSymlinks(dotfiles); err != nil {
		return err
	}
	removeSSHSymlink(dotfiles)
	removeCursorConfig(dotfiles)
	removeScripts(dotfiles)
	removeGitConfig(dotfiles)
	removeCacheFiles()
	removeHushlogin()
	removeBackups(dotfiles)
	removeSystemdUpdater()
	if err := removePackages(ctx, purgePackages); err != nil {
		return err
	}

	logInfo("")
	logInfo("Uninstall complete!")
	logInfof("The dotfiles directory (%s) was NOT removed", dotfiles)
	if !purgePackages {
		logInfo("Installed packages were NOT removed (use --purge-packages)")
	}
	logInfo("To fully remove, run: rm -rf " + dotfiles)
	return nil
}

func printUninstallBanner(purgePackages bool) error {
	if purgePackages {
		logInfo("╔═══════════════════════════════════════════╗")
		logInfo("║         Dotfiles Uninstaller              ║")
		logInfo("║  This will remove symlinks & configs      ║")
		logInfo("║  ⚠️  PACKAGES WILL ALSO BE REMOVED ⚠️      ║")
		logInfo("╚═══════════════════════════════════════════╝")
		return nil
	}

	logInfo("╔═══════════════════════════════════════════╗")
	logInfo("║         Dotfiles Uninstaller              ║")
	logInfo("║  This will remove symlinks & configs      ║")
	logInfo("║  Packages will NOT be removed             ║")
	logInfo("║  Use --purge-packages to remove them      ║")
	logInfo("╚═══════════════════════════════════════════╝")
	return nil
}

func promptYesNo(prompt string) bool {
	logInfo(prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func removeHomeSymlinks(dotfiles string) error {
	logInfo("Removing home directory symlinks...")
	homeDir := filepath.Join(dotfiles, "home")
	if _, err := os.Stat(homeDir); err != nil {
		return nil
	}

	return filepath.WalkDir(homeDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(homeDir, path)
		if err != nil {
			return nil
		}
		target := filepath.Join(os.Getenv("HOME"), relative)
		return removeDotfilesSymlink(target, dotfiles)
	})
}

func removeSSHSymlink(dotfiles string) {
	if err := removeDotfilesSymlink(filepath.Join(os.Getenv("HOME"), ".ssh", "config"), dotfiles); err != nil {
		logInfof("Skipping ~/.ssh/config: %v", err)
	}
}

func removeCursorConfig(dotfiles string) {
	logInfo("Removing Cursor configuration...")
	cursorDir := filepath.Join(os.Getenv("HOME"), ".cursor")
	commands := filepath.Join(cursorDir, "commands")
	entries, err := os.ReadDir(commands)
	if err != nil {
		return
	}
	for _, entry := range entries {
		_ = removeDotfilesSymlink(filepath.Join(commands, entry.Name()), dotfiles)
	}
}

func removeScripts(dotfiles string) {
	logInfo("Removing scripts updater...")
	if runtime.GOOS == "darwin" {
		daemonName := "com.agoodkind.scripts-updater"
		daemonPlist := filepath.Join("/Library/LaunchDaemons", daemonName+".plist")
		oldAgentPlist := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", daemonName+".plist")
		scriptsDir := "/usr/local/opt/scripts"

		_ = cmdexec.Run(context.Background(), "sudo", "launchctl", "unload", daemonPlist)
		_ = removeIfExists(daemonPlist)
		if _, err := os.Stat(oldAgentPlist); err == nil {
			_ = cmdexec.Run(context.Background(), "launchctl", "unload", oldAgentPlist)
			_ = removeIfExists(oldAgentPlist)
		}
		_ = cmdexec.Run(context.Background(), "sudo", "rm", "-f", "/etc/paths.d/scripts")

		entries, err := os.ReadDir(scriptsDir)
		if err == nil {
			for _, entry := range entries {
				name := entry.Name()
				target := filepath.Join("/usr/local/bin", name)
				scriptTarget := filepath.Join(scriptsDir, name)
				_ = removeSymlinkTo(target, scriptTarget)
			}
			_ = removeIfExists(scriptsDir)
		}

		oldScripts := filepath.Join(os.Getenv("HOME"), ".local", "bin", "scripts")
		if _, err := os.Stat(oldScripts); err == nil {
			scriptEntries, err := os.ReadDir(oldScripts)
			if err == nil {
				for _, entry := range scriptEntries {
					_ = removeDotfilesSymlink(filepath.Join(oldScripts, entry.Name()), dotfiles)
				}
			}
			_ = removeIfExists(oldScripts)
		}
	} else {
		scriptsDir := filepath.Join(os.Getenv("HOME"), ".local", "bin", "scripts")
		if _, err := os.Stat(scriptsDir); err == nil {
			entries, err := os.ReadDir(scriptsDir)
			if err == nil {
				for _, entry := range entries {
					_ = removeDotfilesSymlink(filepath.Join(scriptsDir, entry.Name()), dotfiles)
				}
			}
			_ = removeIfExists(scriptsDir)
		}
	}
}

func removeGitConfig(dotfiles string) {
	logInfo("Removing git config include...")
	includePath := filepath.Join(dotfiles, "lib", ".gitconfig_incl")
	output, err := cmdexec.Output(context.Background(), "git", "config", "--global", "--get", "include.path")
	if err != nil {
		return
	}
	if strings.TrimSpace(output) != includePath {
		return
	}
	_ = cmdexec.Run(context.Background(), "git", "config", "--global", "--unset", "include.path")
}

func removeCacheFiles() {
	for _, name := range []string{
		"dotfiles_update.lock",
		"dotfiles_update.log",
		"dotfiles_update_error",
		"dotfiles_update_success",
		"dotfiles_debug_enabled",
	} {
		_ = removeIfExists(filepath.Join(os.Getenv("HOME"), ".cache", name))
	}
}

func removeHushlogin() {
	hush := filepath.Join(os.Getenv("HOME"), ".hushlogin")
	if _, err := os.Stat(hush); err == nil {
		if promptYesNo("Remove .hushlogin (show login message)? (y/n) ") {
			_ = removeIfExists(hush)
		}
	}
}

func removeBackups(dotfiles string) {
	backupsDir := filepath.Join(dotfiles, "backups")
	if _, err := os.Stat(backupsDir); err == nil {
		if promptYesNo("Remove backups directory? (y/n) ") {
			_ = removeIfExists(backupsDir)
		}
	}
}

func removeSystemdUpdater() {
	if runtime.GOOS == "darwin" {
		return
	}
	if _, err := cmdexec.Output(context.Background(), "systemctl", "is-enabled", "scripts-updater.timer"); err != nil {
		return
	}
	if !promptYesNo("Remove scripts-updater systemd timer? (y/n) ") {
		return
	}
	_ = cmdexec.Run(context.Background(), "sudo", "systemctl", "stop", "scripts-updater.timer")
	_ = cmdexec.Run(context.Background(), "sudo", "systemctl", "disable", "scripts-updater.timer")
	_ = removeIfExists("/etc/systemd/system/scripts-updater.service")
	_ = removeIfExists("/etc/systemd/system/scripts-updater.timer")
	_ = cmdexec.Run(context.Background(), "sudo", "systemctl", "daemon-reload")
}

func removePackages(ctx context.Context, purge bool) error {
	if !purge {
		return nil
	}
	logInfo("Package removal requested...")
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
		common:    append([]string{}, cfg.COMMON_PACKAGES...),
		brew:      append([]string{}, cfg.BREW_SPECIFIC...),
		apt:       append([]string{}, cfg.APT_SPECIFIC...),
		snap:      append([]string{}, cfg.SNAP_PACKAGES...),
		brewCasks: make([]string, 0, len(cfg.BREW_CASKS)),
	}
	for cask := range cfg.BREW_CASKS {
		lists.brewCasks = append(lists.brewCasks, cask)
	}
	return lists, nil
}

func removeBrewPackages(ctx context.Context, lists *packageLists) error {
	if !runner.HasCommand("brew") {
		logWarn("Homebrew not installed, skipping package removal")
		return nil
	}

	all := append(append([]string{}, lists.common...), lists.brew...)
	installedFormulae, err := cmdexec.Output(ctx, "brew", "list", "--formula")
	if err != nil {
		return nil
	}
	toRemove := toInstallableSet(strings.Fields(installedFormulae), all)
	if len(toRemove) > 0 {
		logInfof("Will remove %d formulae: %s", len(toRemove), strings.Join(toRemove, " "))
		if promptYesNo("Continue? (y/n) ") {
			args := append([]string{"uninstall", "--force"}, toRemove...)
			_ = cmdexec.Run(ctx, "brew", args...)
		}
	} else {
		logInfo("No matching formulae installed")
	}

	installedCasks, err := cmdexec.Output(ctx, "brew", "list", "--cask")
	if err != nil {
		return nil
	}
	casksToRemove := toInstallableSet(strings.Fields(installedCasks), lists.brewCasks)
	if len(casksToRemove) > 0 {
		logInfof("Will remove %d casks: %s", len(casksToRemove), strings.Join(casksToRemove, " "))
		if promptYesNo("Continue? (y/n) ") {
			args := append([]string{"uninstall", "--cask", "--force"}, casksToRemove...)
			_ = cmdexec.Run(ctx, "brew", args...)
		}
	} else {
		logInfo("No matching casks installed")
	}
	return nil
}

func removeAptPackages(ctx context.Context, lists *packageLists) error {
	if !runner.HasCommand("apt-get") {
		logWarn("apt-get not found, skipping package removal")
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
		logInfo("No matching APT packages installed")
		return nil
	}
	logInfof("Will remove %d APT packages", len(toRemove))
	if !promptYesNo("Continue? (y/n) ") {
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
		logInfo("No matching Snap packages installed")
		return nil
	}
	logInfof("Will remove %d Snap packages: %s", len(toRemove), strings.Join(toRemove, ", "))
	if !promptYesNo("Continue? (y/n) ") {
		return nil
	}
	for _, pkg := range toRemove {
		_ = cmdexec.Run(ctx, "sudo", "snap", "remove", pkg)
	}
	return nil
}

func removeDotfilesSymlink(target string, dotfiles string) error {
	fi, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read target: %w", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("not a symlink")
	}
	link, err := os.Readlink(target)
	if err != nil {
		return err
	}
	dotfilesRoot := filepath.Clean(dotfiles)
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(target), link)
	}
	cleanLink := filepath.Clean(link)
	if cleanLink == dotfilesRoot || strings.HasPrefix(cleanLink, dotfilesRoot+string(filepath.Separator)) {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("remove symlink: %w", err)
		}
		logDebugf("Removed symlink: %s", target)
		return nil
	}
	logInfof("Skipping %s (not a dotfiles symlink)", target)
	return nil
}

func removeSymlinkTo(target string, expected string) error {
	fi, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read target: %w", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("not a symlink")
	}
	link, err := os.Readlink(target)
	if err != nil {
		return err
	}
	if link != expected {
		logInfof("Skipping %s (not managed script symlink)", target)
		return nil
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("remove symlink: %w", err)
	}
	logDebugf("Removed symlink: %s", target)
	return nil
}

func removeIfExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return os.RemoveAll(path)
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

func logInfo(message string) {
	if uninstallLogger != nil {
		uninstallLogger.Info(message)
	}
}

func logInfof(format string, args ...any) {
	if uninstallLogger != nil {
		uninstallLogger.Info(fmt.Sprintf(format, args...))
	}
}

func logWarn(message string) {
	if uninstallLogger != nil {
		uninstallLogger.Warn(message)
	}
}

func logDebugf(format string, args ...any) {
	if uninstallLogger != nil {
		uninstallLogger.Debug(fmt.Sprintf(format, args...))
	}
}
