// Package platform implements platform-specific sync steps.
package platform

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/sync/tools"
	"goodkind.io/.dotfiles/internal/telemetry"
	"goodkind.io/.dotfiles/internal/util"
)

// RunOSInstall runs the platform-specific package installation and setup steps.
func RunOSInstall(ctx context.Context, quickMode bool, useDefaults bool, logger *telemetry.Logger) error {
	if quickMode {
		return nil
	}

	var osType string
	if runtime.GOOS == "darwin" {
		osType = "macOS"
		common.InfoContextf(ctx, logger, "  Running %s setup", osType)
		return runMacSetup(ctx, useDefaults, logger)
	} else if common.IsUbuntu() {
		osType = "Debian/Ubuntu/Proxmox"
		common.InfoContextf(ctx, logger, "  Running %s setup", osType)
		return runDebianSetup(ctx, useDefaults, logger)
	}
	common.WarnContext(ctx, logger, "  No OS-specific setup handler for this platform")
	return nil
}

func runMacSetup(ctx context.Context, useDefaults bool, logger *telemetry.Logger) error {
	_ = useDefaults
	common.InfoContext(ctx, logger, "  running macOS bootstrap")
	if err := ensureHomebrewInstalled(ctx, logger); err != nil {
		return err
	}
	if err := runMacDefaults(ctx, logger); err != nil {
		return err
	}
	if runner.HasCommand("brew") {
		_ = cmdexec.RunWithLogger(ctx, logger, "brew", "cleanup")
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
	common.InfoContext(ctx, logger, "  running Linux bootstrap")
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
	slog.InfoContext(ctx, "platform: ensureHomebrewInstalled")
	if runner.HasCommand("brew") {
		return nil
	}
	installer, err := tools.DownloadToTempFile(ctx, logger, "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh")
	if err != nil {
		slog.WarnContext(ctx, "downloading tool", "err", err)
		return fmt.Errorf("downloading tool: %w", err)
	}
	defer os.Remove(installer)
	if err := cmdexec.RunWithLogger(ctx, logger, "bash", installer); err != nil {
		slog.WarnContext(ctx, "running bash", "err", err)
		return fmt.Errorf("running bash: %w", err)
	}
	return nil
}

func runMacDefaults(ctx context.Context, logger *telemetry.Logger) error {
	if err := cmdexec.RunWithLogger(ctx, logger, "defaults", "write", "com.apple.finder", "AppleShowAllFiles", "-bool", "true"); err != nil {
		slog.WarnContext(ctx, "running defaults", "err", err)
		return fmt.Errorf("running defaults: %w", err)
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "defaults", "write", "com.apple.finder", "ShowPathbar", "-bool", "true"); err != nil {
		slog.WarnContext(ctx, "running defaults", "err", err)
		return fmt.Errorf("running defaults: %w", err)
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "defaults", "write", "com.apple.finder", "ShowStatusBar", "-bool", "true"); err != nil {
		slog.WarnContext(ctx, "running defaults", "err", err)
		return fmt.Errorf("running defaults: %w", err)
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "defaults", "write", "NSGlobalDomain", "AppleShowAllExtensions", "-bool", "true"); err != nil {
		slog.WarnContext(ctx, "running defaults", "err", err)
		return fmt.Errorf("running defaults: %w", err)
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "defaults", "write", "com.apple.desktopservices", "DSDontWriteNetworkStores", "-bool", "true"); err != nil {
		slog.WarnContext(ctx, "running defaults", "err", err)
		return fmt.Errorf("running defaults: %w", err)
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "defaults", "write", "com.apple.desktopservices", "DSDontWriteUSBStores", "-bool", "true"); err != nil {
		slog.WarnContext(ctx, "running defaults", "err", err)
		return fmt.Errorf("running defaults: %w", err)
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "defaults", "write", "com.apple.dock", "mouse-over-hilite-stack", "-bool", "true"); err != nil {
		slog.WarnContext(ctx, "running defaults", "err", err)
		return fmt.Errorf("running defaults: %w", err)
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "defaults", "write", "com.apple.dock", "mineffect", "-string", "suck"); err != nil {
		slog.WarnContext(ctx, "running defaults", "err", err)
		return fmt.Errorf("running defaults: %w", err)
	}
	shotDir := filepath.Join(os.Getenv("HOME"), "Documents", "Screenshots")
	if err := os.MkdirAll(filepath.Clean(shotDir), 0o755); err == nil {
		_ = cmdexec.RunWithLogger(ctx, logger, "defaults", "write", "com.apple.screencapture", "location", shotDir)
	}
	_ = cmdexec.RunWithLogger(ctx, logger, "defaults", "write", "com.google.Chrome", "BuiltInDnsClientEnabled", "-bool", "false")
	_ = cmdexec.RunWithLogger(ctx, logger, "killall", "Finder")
	_ = cmdexec.RunWithLogger(ctx, logger, "killall", "Dock")
	_ = cmdexec.RunWithLogger(ctx, logger, "killall", "SystemUIServer")
	return nil
}

func installTouchIDHelper(ctx context.Context, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "platform: installTouchIDHelper")
	scriptPath, err := tools.DownloadToTempFile(ctx, logger, "https://git.io/sudo-touch-id")
	if err != nil {
		slog.WarnContext(ctx, "downloading tool", "err", err)
		return fmt.Errorf("downloading tool: %w", err)
	}
	defer os.Remove(scriptPath)
	if err := cmdexec.RunWithLogger(ctx, logger, "sh", scriptPath); err != nil {
		slog.WarnContext(ctx, "running sh", "err", err)
		return fmt.Errorf("running sh: %w", err)
	}
	return nil
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
		if !os.IsNotExist(err) {
			logger.WarnContextWithErr(ctx, "read zprofile", err)
			return fmt.Errorf("read zprofile: %w", err)
		}
		return nil
	}
	zshrc, err2 := os.ReadFile(zshrcPath)
	if err2 != nil {
		if !os.IsNotExist(err2) {
			logger.WarnContextWithErr(ctx, "read zshrc", err2)
			return fmt.Errorf("read zshrc: %w", err2)
		}
		return nil
	}
	if strings.Contains(string(zprofile), cfg.Sentinel) && strings.Contains(string(zshrc), cfg.Sentinel) {
		return nil
	}
	if !common.HasSudoAccess(ctx, logger) {
		return nil
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "sudo", "bash", patch); err != nil {
		slog.WarnContext(ctx, "running sudo bash", "err", err)
		return fmt.Errorf("running sudo bash: %w", err)
	}
	return nil
}

func installRustupIfNeeded(ctx context.Context, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "platform: installRustupIfNeeded")
	if runner.HasCommand("rustup") {
		return nil
	}
	inst, err := tools.DownloadToTempFile(ctx, logger, "https://sh.rustup.rs")
	if err != nil {
		slog.WarnContext(ctx, "downloading tool", "err", err)
		return fmt.Errorf("downloading tool: %w", err)
	}
	defer os.Remove(inst)
	if err := cmdexec.RunWithLogger(ctx, logger, "sh", inst, "-s", "--", "-y"); err != nil {
		slog.WarnContext(ctx, "running sh", "err", err)
		return fmt.Errorf("running sh: %w", err)
	}
	return nil
}

func installGoToolsIfNeeded(ctx context.Context, logger *telemetry.Logger) error {
	if !runner.HasCommand("go") {
		return nil
	}
	common.InfoContext(ctx, logger, "  checking Go tools")
	cfg := common.DefaultPackageConfig()
	if cfg == nil {
		common.WarnContext(ctx, logger, "  failed to parse package config for Go tools: no config")
		return nil
	}
	for bin, pkg := range cfg.GoPackages {
		if runner.HasCommand(bin) {
			continue
		}
		common.InfoContextf(ctx, logger, "  installing go tool %s", bin)
		if err := cmdexec.RunWithLogger(ctx, logger, "go", "install", pkg); err != nil {
			slog.WarnContext(ctx, "running go install", "err", err)
			return fmt.Errorf("running go install: %w", err)
		}
	}
	_ = installCargoToolsIfNeeded(ctx, cfg, logger)
	return nil
}

func installCargoToolsIfNeeded(ctx context.Context, cfg *catalog.PackageConfig, logger *telemetry.Logger) error {
	if !runner.HasCommand("cargo") {
		return nil
	}
	if len(cfg.CargoPackages) == 0 {
		return nil
	}
	for _, tool := range cfg.CargoPackages {
		if runner.HasCommand(tool) {
			continue
		}
		if err := cmdexec.RunWithLogger(ctx, logger, "cargo", "install", tool); err != nil {
			slog.WarnContext(ctx, "running cargo install", "err", err)
			return fmt.Errorf("running cargo install: %w", err)
		}
	}
	for tool, repo := range cfg.CargoGitPackages {
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
		if err := cmdexec.RunWithLogger(ctx, logger, "cargo", args...); err != nil {
			slog.WarnContext(ctx, "running cargo install", "err", err)
			return fmt.Errorf("running cargo install: %w", err)
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
	if err := cmdexec.RunWithLogger(ctx, logger, "brew", "update", "--quiet"); err != nil {
		common.WarnContext(ctx, logger, "  brew update failed, continuing")
	}
	formulae := make(map[string]struct{})
	var formulaList []string
	for _, item := range append(cfg.CommonPackages, cfg.BrewSpecific...) {
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
		if err := cmdexec.RunWithLogger(ctx, logger, "brew", brewArgs...); err != nil {
			common.WarnContext(ctx, logger, "  brew formula install returned an error")
		}
	}
	for cask := range cfg.BrewCasks {
		app := cfg.BrewCasks[cask]
		if app != "" {
			if _, err := os.Stat(filepath.Join("/Applications", app)); err == nil {
				continue
			}
			if _, err := os.Stat(filepath.Clean(filepath.Join(os.Getenv("HOME"), "Applications", app))); err == nil {
				continue
			}
		}
		if err := cmdexec.RunWithLogger(ctx, logger, "brew", "install", "--cask", cask); err != nil {
			common.WarnContextf(ctx, logger, "  failed to install cask %s", cask)
		}
	}
	return nil
}

// installUbuntuPPAs adds third-party Launchpad PPAs on Ubuntu hosts before the main apt-get install batch.
// This is a no-op on Debian (which has these packages natively) and on non-Ubuntu systems.
// Failures are non-fatal: each PPA add is attempted independently and only a warning is logged on error.
func installUbuntuPPAs(ctx context.Context, cfg *catalog.PackageConfig, logger *telemetry.Logger) {
	if !common.IsUbuntuOnly() {
		return
	}
	if len(cfg.UbuntuPPAs) == 0 {
		return
	}
	// software-properties-common provides add-apt-repository.
	if err := common.RunDebianPrivilegedCommand(ctx, logger, "apt-get", "install", "-y", "-qq", "software-properties-common"); err != nil {
		slog.WarnContext(ctx, "installUbuntuPPAs: installing software-properties-common", "err", err)
		common.WarnContext(ctx, logger, "  failed to install software-properties-common; skipping PPAs")
		return
	}
	added := false
	for _, ppa := range cfg.UbuntuPPAs {
		common.InfoContextf(ctx, logger, "  adding PPA %s", ppa)
		if err := common.RunDebianPrivilegedCommand(ctx, logger, "add-apt-repository", "-y", ppa); err != nil {
			slog.WarnContext(ctx, "installUbuntuPPAs: add-apt-repository", "ppa", ppa, "err", err)
			common.WarnContextf(ctx, logger, "  failed to add PPA %s", ppa)
			continue
		}
		added = true
	}
	if added {
		if err := common.RunDebianPrivilegedCommand(ctx, logger, "apt-get", "update", "-qq"); err != nil {
			slog.WarnContext(ctx, "installUbuntuPPAs: apt-get update after PPAs", "err", err)
		}
	}
}

func installDebianPackages(ctx context.Context, logger *telemetry.Logger) error {
	if !runner.HasCommand("apt-get") {
		return nil
	}
	cfg := common.DefaultPackageConfig()
	if cfg == nil {
		return nil
	}
	if err := common.RunDebianPrivilegedCommand(ctx, logger, "apt-get", "update", "-qq"); err != nil {
		slog.WarnContext(ctx, "running apt-get update", "err", err)
		return fmt.Errorf("running apt-get update: %w", err)
	}

	installUbuntuPPAs(ctx, cfg, logger)

	packages := make(map[string]struct{})
	var aptPkgs []string
	for _, item := range append(cfg.CommonPackages, cfg.AptSpecific...) {
		if isSnapPackage(item, cfg.SnapPackages) {
			continue
		}
		for mapped := range strings.FieldsSeq(aptPackageName(item)) {
			if _, ok := packages[mapped]; ok {
				continue
			}
			packages[mapped] = struct{}{}
			aptPkgs = append(aptPkgs, mapped)
		}
	}
	if len(aptPkgs) > 0 {
		aptArgs := append([]string{"install", "-y", "-qq"}, aptPkgs...)
		if err := common.RunDebianPrivilegedCommand(ctx, logger, "apt-get", aptArgs...); err != nil {
			slog.WarnContext(ctx, "running apt-get install", "err", err)
			return fmt.Errorf("running apt-get install: %w", err)
		}
	}
	for _, pkg := range cfg.SnapPackages {
		target := snapPackageName(pkg)
		if target == "" {
			continue
		}
		if runner.HasCommand("snap") {
			if err := installSnapPackage(ctx, target, logger); err != nil {
				common.WarnContext(ctx, logger, "  failed to install snap package "+target)
			}
		}
	}
	return nil
}

func installSnapPackage(ctx context.Context, packageName string, logger *telemetry.Logger) error {
	if cmdexec.RunWithLogger(ctx, logger, "snap", "list", packageName) == nil {
		return nil
	}
	args := []string{"install", packageName}
	if isSnapClassic(ctx, packageName) {
		args = []string{"install", "--classic", packageName}
	}
	snapArgs := append([]string{}, args...)
	if err := cmdexec.RunWithLogger(ctx, logger, "sudo", snapArgs...); err != nil {
		if isSnapClassic(ctx, packageName) {
			slog.WarnContext(ctx, "running snap install", "err", err)
			return fmt.Errorf("running snap install: %w", err)
		}
		if cmdexec.RunWithLogger(ctx, logger, "snap", "info", packageName) != nil {
			slog.WarnContext(ctx, "running snap install", "err", err)
			return fmt.Errorf("running snap install: %w", err)
		}
		if err := cmdexec.RunWithLogger(ctx, logger, "sudo", "snap", "install", "--classic", packageName); err != nil {
			slog.WarnContext(ctx, "running snap install", "err", err)
			return fmt.Errorf("running snap install: %w", err)
		}
	}
	return nil
}

func isSnapClassic(ctx context.Context, packageName string) bool {
	output, err := cmdexec.OutputWithLoggerAndEnv(ctx, nil, nil, "snap", "info", packageName)
	if err != nil {
		return false
	}
	return strings.Contains(output, "classic") && strings.Contains(output, "confinement")
}

type toolName string

const (
	toolAck     toolName = "ack"
	toolFd      toolName = "fd"
	toolRg      toolName = "rg"
	toolOpenssh toolName = "openssh"
)

func aptPackageName(packageName string) string {
	switch toolName(packageName) {
	case toolAck:
		return "ack-grep"
	case toolFd:
		return "fd-find"
	case toolRg:
		return "ripgrep"
	case toolOpenssh:
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
	return slices.Contains(snapList, packageName)
}

func snapPackageName(packageName string) string {
	if packageName == "neovim" {
		return "nvim"
	}
	return packageName
}
