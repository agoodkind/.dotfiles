package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/agoodkind/.dotfiles/internal/catalog"
	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/runner"
	configassets "github.com/agoodkind/.dotfiles/config"
	"github.com/agoodkind/.dotfiles/internal/sync/common"
	"github.com/agoodkind/.dotfiles/internal/sync/tools"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
	"github.com/agoodkind/.dotfiles/internal/util"
)

func RunOSInstall(ctx context.Context, quickMode bool, useDefaults bool, logger *telemetry.Logger) error {
	if quickMode {
		return nil
	}

	var osType string
	if runtime.GOOS == "darwin" {
		osType = "macOS"
		common.Infof(logger, "  Running %s setup", osType)
		return runMacSetup(ctx, useDefaults, logger)
	} else if common.IsUbuntu() {
		osType = "Debian/Ubuntu/Proxmox"
		common.Infof(logger, "  Running %s setup", osType)
		return runDebianSetup(ctx, useDefaults, logger)
	}
	common.Warn(logger, "  No OS-specific setup handler for this platform")
	return nil
}

func InstallScriptsUpdaterMac(ctx context.Context, logger *telemetry.Logger) error {
	if !common.HasSudoAccess(ctx, logger) {
		return fmt.Errorf("no sudo access for scripts-updater installation")
	}

	const scriptsDir = "/usr/local/opt/scripts"
	const serviceName = "com.agoodkind.scripts-updater"
	const plistPath = "/Library/LaunchDaemons/" + serviceName + ".plist"
	const updaterRepo = "https://github.com/agoodkind/scripts.git"

	common.Info(logger, "  syncing scripts-updater service")
	if err := ensureScriptsDirectory(ctx, scriptsDir, updaterRepo, true, logger); err != nil {
		return err
	}

	_ = cmdexec.RunWithLogger(context.Background(), logger, "sudo", "mkdir", "-p", "/usr/local/bin")
	_ = cmdexec.RunWithLogger(context.Background(), logger, "sudo", "chown", "root:wheel", scriptsDir)

	plist, err := configassets.RenderTemplate("platform-launchd.plist.tmpl", map[string]any{
		"ServiceName": serviceName,
		"ScriptsDir":  scriptsDir,
	})
	if err != nil {
		return err
	}
	daemon := []byte(plist)
	if err := writeFileAsRoot(plistPath, daemon, 0o644); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "chown", "root:wheel", plistPath); err != nil {
		return err
	}

	_ = cmdexec.RunWithLogger(context.Background(), logger, "sudo", "launchctl", "unload", plistPath)
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "launchctl", "load", plistPath); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "mkdir", "-p", "/etc/paths.d"); err != nil {
		return err
	}
	if _, err := os.Stat("/etc/paths.d/scripts"); err != nil {
		pathContent, err := configassets.RenderTemplate("platform-paths-d-entry.tmpl", map[string]any{
			"ScriptsDir": scriptsDir,
		})
		if err != nil {
			return err
		}
		if err := writeFileAsRoot("/etc/paths.d/scripts", []byte(pathContent), 0o644); err != nil {
			return err
		}
	}

	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if strings.HasPrefix(name, ".") || lower == "license" || lower == "license.txt" || lower == "readme" || lower == "readme.md" || strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".txt") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue
		}
		target := filepath.Join("/usr/local/bin", name)
		_ = cmdexec.RunWithLogger(context.Background(), logger, "sudo", "rm", "-f", target)
		if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "ln", "-sf", filepath.Join(scriptsDir, name), target); err != nil {
			return err
		}
	}
	return cmdexec.RunWithLogger(context.Background(), logger, "launchctl", "start", serviceName)
}

func InstallScriptsUpdaterLinux(ctx context.Context, logger *telemetry.Logger) error {
	if !common.HasSudoAccess(ctx, logger) {
		return fmt.Errorf("no sudo access for scripts-updater installation")
	}

	const scriptsDir = "/opt/scripts"
	const serviceName = "scripts-updater"
	const updaterRepo = "https://github.com/agoodkind/scripts.git"

	if err := ensureScriptsDirectory(ctx, scriptsDir, updaterRepo, false, logger); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "cp", filepath.Join(scriptsDir, "updater", "scripts-updater.service"), filepath.Join("/etc/systemd/system", serviceName+".service")); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "cp", filepath.Join(scriptsDir, "updater", "scripts-updater.timer"), filepath.Join("/etc/systemd/system", serviceName+".timer")); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "systemctl", "enable", serviceName+".timer"); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "systemctl", "start", serviceName+".timer"); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "systemctl", "start", serviceName+".service"); err != nil {
		return err
	}
	if _, err := os.Stat("/etc/profile.d/opt-scripts.sh"); err != nil {
		profileContent, err := configassets.RenderTemplate("platform-opt-scripts-profile.sh.tmpl", map[string]any{
			"ScriptsDir": scriptsDir,
		})
		if err != nil {
			return err
		}
		if err := writeFileAsRoot("/etc/profile.d/opt-scripts.sh", []byte(profileContent), 0o644); err != nil {
			return err
		}
	}
	if _, err := os.Stat("/etc/systemd/system.conf.d/path.conf"); err != nil {
		if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "mkdir", "-p", "/etc/systemd/system.conf.d"); err != nil {
			return err
		}
		pathEnv := "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/snap/bin:/opt/scripts"
		pathConfContent, err := configassets.RenderTemplate("platform-systemd-path-conf.tmpl", map[string]any{
			"PathEnv": pathEnv,
		})
		if err != nil {
			return err
		}
		if err := writeFileAsRoot("/etc/systemd/system.conf.d/path.conf", []byte(pathConfContent), 0o644); err != nil {
			return err
		}
		if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "systemctl", "daemon-reload"); err != nil {
			return err
		}
	}
	return nil
}

func ensureScriptsDirectory(ctx context.Context, path, repo string, ensureParent bool, logger *telemetry.Logger) error {
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "git", "-C", path, "fetch", "origin"); err != nil {
			return err
		}
		if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "git", "-C", path, "reset", "--hard", "origin/main"); err != nil {
			common.Warn(logger, "  scripts repo reset failed, using existing checkout")
		}
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "rm", "-rf", path); err != nil {
			return err
		}
	}
	if ensureParent {
		if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "mkdir", "-p", filepath.Dir(path)); err != nil {
			return err
		}
		_ = cmdexec.RunWithLogger(context.Background(), logger, "sudo", "chown", os.Getenv("USER"), filepath.Dir(path))
	}
	return cmdexec.RunWithLogger(context.Background(), logger, "sudo", "git", "clone", repo, path)
}

func writeFileAsRoot(path string, contents []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp("", filepath.Base(path))
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpName)
	if err := os.WriteFile(tmpName, contents, perm); err != nil {
		return err
	}
	return cmdexec.RunWithLogger(context.Background(), nil, "sudo", "cp", tmpName, path)
}

func runMacSetup(ctx context.Context, useDefaults bool, logger *telemetry.Logger) error {
	_ = useDefaults
	common.Info(logger, "  running macOS bootstrap")
	if err := ensureHomebrewInstalled(ctx, logger); err != nil {
		return err
	}
	if err := runMacDefaults(ctx, logger); err != nil {
		return err
	}
	if runner.HasCommand("brew") {
		_ = cmdexec.RunWithLogger(context.Background(), logger, "brew", "cleanup")
	}
	if err := installMacPackages(ctx, logger); err != nil {
		return err
	}
	if err := installRustupIfNeeded(ctx, logger); err != nil {
		return err
	}
	if err := installTouchIDHelper(ctx, logger); err != nil {
		return err
	}
	return ensureMacPatchIfNeeded(ctx, logger)
}

func runDebianSetup(ctx context.Context, useDefaults bool, logger *telemetry.Logger) error {
	_ = useDefaults
	common.Info(logger, "  running Linux bootstrap")
	if err := installDebianPackages(ctx, logger); err != nil {
		return err
	}
	if err := installRustupIfNeeded(ctx, logger); err != nil {
		return err
	}
	if err := installGoToolsIfNeeded(ctx, logger); err != nil {
		return err
	}
	return nil
}

func ensureHomebrewInstalled(ctx context.Context, logger *telemetry.Logger) error {
	if runner.HasCommand("brew") {
		return nil
	}
	installer, err := tools.DownloadToTempFile(ctx, logger, "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh")
	if err != nil {
		return err
	}
	defer os.Remove(installer)
	return cmdexec.RunWithLogger(context.Background(), logger, "bash", installer)
}

func runMacDefaults(ctx context.Context, logger *telemetry.Logger) error {
	if err := cmdexec.RunWithLogger(context.Background(), logger, "defaults", "write", "com.apple.finder", "AppleShowAllFiles", "-bool", "true"); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "defaults", "write", "com.apple.finder", "ShowPathbar", "-bool", "true"); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "defaults", "write", "com.apple.finder", "ShowStatusBar", "-bool", "true"); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "defaults", "write", "NSGlobalDomain", "AppleShowAllExtensions", "-bool", "true"); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "defaults", "write", "com.apple.desktopservices", "DSDontWriteNetworkStores", "-bool", "true"); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "defaults", "write", "com.apple.desktopservices", "DSDontWriteUSBStores", "-bool", "true"); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "defaults", "write", "com.apple.dock", "mouse-over-hilite-stack", "-bool", "true"); err != nil {
		return err
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "defaults", "write", "com.apple.dock", "mineffect", "-string", "suck"); err != nil {
		return err
	}
	shotDir := filepath.Join(os.Getenv("HOME"), "Documents", "Screenshots")
	if err := os.MkdirAll(shotDir, 0o755); err == nil {
		_ = cmdexec.RunWithLogger(context.Background(), logger, "defaults", "write", "com.apple.screencapture", "location", shotDir)
	}
	_ = cmdexec.RunWithLogger(context.Background(), logger, "defaults", "write", "com.google.Chrome", "BuiltInDnsClientEnabled", "-bool", "false")
	_ = cmdexec.RunWithLogger(context.Background(), logger, "killall", "Finder")
	_ = cmdexec.RunWithLogger(context.Background(), logger, "killall", "Dock")
	_ = cmdexec.RunWithLogger(context.Background(), logger, "killall", "SystemUIServer")
	return nil
}

func installTouchIDHelper(ctx context.Context, logger *telemetry.Logger) error {
	scriptPath, err := tools.DownloadToTempFile(ctx, logger, "https://git.io/sudo-touch-id")
	if err != nil {
		return err
	}
	defer os.Remove(scriptPath)
	return cmdexec.RunWithLogger(context.Background(), logger, "sh", scriptPath)
}

func ensureMacPatchIfNeeded(ctx context.Context, logger *telemetry.Logger) error {
	cfg := catalog.DefaultMacPatchConfig()
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	patch := cfg.PatchScript
	if patch == "" {
		patch = filepath.Join(os.Getenv("DOTDOTFILES"), "bash", "setup", "platform", "patch-etc-zsh.bash")
	}
	patch = util.ResolveConfigPath(patch, os.Getenv("DOTDOTFILES"))
	zprofilePath := cfg.ZProfilePath
	if zprofilePath == "" {
		zprofilePath = "/etc/zprofile"
	}
	zshrcPath := cfg.ZshrcPath
	if zshrcPath == "" {
		zshrcPath = "/etc/zshrc"
	}
	if cfg.Sentinel == "" {
		cfg.Sentinel = "# DOTFILES_PERF_PATCH_V4"
	}
	zprofile, err := os.ReadFile(zprofilePath)
	if err != nil {
		return nil
	}
	zshrc, err2 := os.ReadFile(zshrcPath)
	if err2 != nil {
		return nil
	}
	if strings.Contains(string(zprofile), cfg.Sentinel) && strings.Contains(string(zshrc), cfg.Sentinel) {
		return nil
	}
	if !common.HasSudoAccess(ctx, logger) {
		return nil
	}
	return cmdexec.RunWithLogger(context.Background(), logger, "sudo", "bash", patch)
}

func installRustupIfNeeded(ctx context.Context, logger *telemetry.Logger) error {
	if runner.HasCommand("rustup") {
		return nil
	}
	inst, err := tools.DownloadToTempFile(ctx, logger, "https://sh.rustup.rs")
	if err != nil {
		return err
	}
	defer os.Remove(inst)
	return cmdexec.RunWithLogger(context.Background(), logger, "sh", inst, "-s", "--", "-y")
}

func installGoToolsIfNeeded(ctx context.Context, logger *telemetry.Logger) error {
	if !runner.HasCommand("go") {
		return nil
	}
	common.Info(logger, "  checking Go tools")
	cfg := common.DefaultPackageConfig()
	if cfg == nil {
		common.Warn(logger, "  failed to parse package config for Go tools: no config")
		return nil
	}
	for bin, pkg := range cfg.GO_PACKAGES {
		if runner.HasCommand(bin) {
			continue
		}
		common.Infof(logger, "  installing go tool %s", bin)
		if err := cmdexec.RunWithLogger(context.Background(), logger, "go", "install", pkg); err != nil {
			return err
		}
	}
	_ = installCargoToolsIfNeeded(cfg, logger)
	return nil
}

func installCargoToolsIfNeeded(cfg *catalog.PackageConfig, logger *telemetry.Logger) error {
	if !runner.HasCommand("cargo") {
		return nil
	}
	if len(cfg.CARGO_PACKAGES) == 0 {
		return nil
	}
	for _, tool := range cfg.CARGO_PACKAGES {
		if runner.HasCommand(tool) {
			continue
		}
		if err := cmdexec.RunWithLogger(context.Background(), logger, "cargo", "install", tool); err != nil {
			return err
		}
	}
	for tool, repo := range cfg.CARGO_GIT_PACKAGES {
		if tool == "" || repo == "" || runner.HasCommand(tool) {
			continue
		}
		parts := strings.SplitN(repo, "|", 2)
		features := ""
		if len(parts) == 2 {
			features = strings.TrimSpace(parts[1])
		}
		args := []string{"install", "--git", parts[0]}
		if features != "" {
			args = append(args, "--features", features)
		}
		if err := cmdexec.RunWithLogger(context.Background(), logger, "cargo", args...); err != nil {
			return err
		}
	}
	return nil
}

func installMacPackages(ctx context.Context, logger *telemetry.Logger) error {
	if !runner.HasCommand("brew") {
		return nil
	}
	cfg := common.DefaultPackageConfig()
	if cfg == nil {
		return nil
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "brew", "update", "--quiet"); err != nil {
		common.Warn(logger, "  brew update failed, continuing")
	}
	formulae := make(map[string]struct{})
	var formulaList []string
	for _, item := range append(cfg.COMMON_PACKAGES, cfg.BREW_SPECIFIC...) {
		name := brewPackageName(item)
		if name == "" {
			continue
		}
		if _, ok := formulae[name]; ok {
			continue
		}
		formulae[name] = struct{}{}
		formulaList = append(formulaList, name)
	}
	if len(formulaList) > 0 {
		brewArgs := append([]string{"install"}, formulaList...)
		if err := cmdexec.RunWithLogger(context.Background(), logger, "brew", brewArgs...); err != nil {
			common.Warn(logger, "  brew formula install returned an error")
		}
	}
	for cask := range cfg.BREW_CASKS {
		app := cfg.BREW_CASKS[cask]
		if app != "" {
			if _, err := os.Stat(filepath.Join("/Applications", app)); err == nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), "Applications", app)); err == nil {
				continue
			}
		}
		if err := cmdexec.RunWithLogger(context.Background(), logger, "brew", "install", "--cask", cask); err != nil {
			common.Warnf(logger, "  failed to install cask %s", cask)
		}
	}
	return nil
}

func installDebianPackages(ctx context.Context, logger *telemetry.Logger) error {
	if !runner.HasCommand("apt-get") {
		return nil
	}
	cfg := common.DefaultPackageConfig()
	if cfg == nil {
		return nil
	}
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "apt-get", "update", "-qq"); err != nil {
		return err
	}

	packages := make(map[string]struct{})
	var aptPkgs []string
	for _, item := range append(cfg.COMMON_PACKAGES, cfg.APT_SPECIFIC...) {
		if isSnapPackage(item, cfg.SNAP_PACKAGES) {
			continue
		}
		for _, mapped := range strings.Fields(aptPackageName(item)) {
			if _, ok := packages[mapped]; ok {
				continue
			}
			packages[mapped] = struct{}{}
			aptPkgs = append(aptPkgs, mapped)
		}
	}
	if len(aptPkgs) > 0 {
		aptArgs := append([]string{"install", "-y", "-qq"}, aptPkgs...)
		if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", aptArgs...); err != nil {
			return err
		}
	}
	for _, pkg := range cfg.SNAP_PACKAGES {
		target := snapPackageName(pkg)
		if target == "" {
			continue
		}
		if runner.HasCommand("snap") {
			if err := installSnapPackage(ctx, target, logger); err != nil {
				common.Warn(logger, "  failed to install snap package "+target)
			}
		}
	}
	return nil
}

func installSnapPackage(ctx context.Context, packageName string, logger *telemetry.Logger) error {
	if cmdexec.RunWithLogger(context.Background(), logger, "snap", "list", packageName) == nil {
		return nil
	}
	args := []string{"install", packageName}
	if isSnapClassic(packageName) {
		args = []string{"install", "--classic", packageName}
	}
	snapArgs := append([]string{}, args...)
	if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", snapArgs...); err != nil {
		if isSnapClassic(packageName) {
			return err
		}
		if cmdexec.RunWithLogger(context.Background(), logger, "snap", "info", packageName) != nil {
			return err
		}
		if err := cmdexec.RunWithLogger(context.Background(), logger, "sudo", "snap", "install", "--classic", packageName); err != nil {
			return err
		}
	}
	return nil
}

func isSnapClassic(packageName string) bool {
	output, err := cmdexec.OutputWithLoggerAndEnv(context.Background(), nil, nil, "snap", "info", packageName)
	if err != nil {
		return false
	}
	text := string(output)
	return strings.Contains(text, "classic") && strings.Contains(text, "confinement")
}

func aptPackageName(packageName string) string {
	switch packageName {
	case "ack":
		return "ack-grep"
	case "fd":
		return "fd-find"
	case "rg":
		return "ripgrep"
	case "openssh":
		return "openssh-client openssh-server"
	}
	return packageName
}

func brewPackageName(packageName string) string {
	switch packageName {
	case "tshark":
		return "wireshark"
	}
	return packageName
}

func isSnapPackage(packageName string, snapList []string) bool {
	for _, snap := range snapList {
		if snap == packageName {
			return true
		}
	}
	return false
}

func snapPackageName(packageName string) string {
	if packageName == "neovim" {
		return "nvim"
	}
	return packageName
}

func ensureRustupPath() []string {
	gopath, err := cmdexec.Output(context.Background(), "go", "env", "GOPATH")
	if err != nil {
		return os.Environ()
	}
	return append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+strings.TrimSpace(string(gopath))+string(filepath.Separator)+"bin")
}
