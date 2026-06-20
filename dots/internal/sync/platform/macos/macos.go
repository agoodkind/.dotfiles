// Package macos implements macOS-specific sync setup.
package macos

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/sync/platform"
	"goodkind.io/.dotfiles/internal/sync/platform/toolchain"
	"goodkind.io/.dotfiles/internal/sync/tools"
	"goodkind.io/.dotfiles/internal/telemetry"
	"goodkind.io/.dotfiles/internal/util"
)

type commandRunner interface {
	RunWithLogger(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error
	CommandSucceeds(ctx context.Context, command string, args ...string) bool
}

type commandLookup interface {
	HasCommand(name string) bool
}

type downloader interface {
	DownloadToTempFile(ctx context.Context, logger *telemetry.Logger, fileURL string) (string, error)
}

type fileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	PathExists(path string) bool
	ReadFile(path string) ([]byte, error)
	Remove(path string) error
}

type environment interface {
	Getenv(key string) string
}

type catalogProvider interface {
	MacPatchConfig() *catalog.MacPatchConfig
	PackageConfig() *catalog.PackageConfig
}

type pathResolver interface {
	ResolveConfigPath(value string, dotfiles string) string
}

type sudoChecker interface {
	HasSudoAccess(ctx context.Context, logger *telemetry.Logger) bool
}

// Deps holds the macOS installer dependencies.
type Deps struct {
	Commands commandRunner
	Lookup   commandLookup
	Download downloader
	Files    fileSystem
	Env      environment
	Catalog  catalogProvider
	Paths    pathResolver
	Sudo     sudoChecker
}

// Installer applies macOS-specific sync steps.
type Installer struct {
	deps      Deps
	toolchain *toolchain.Installer
}

// New builds a macOS installer from explicit dependencies.
func New(deps Deps, toolchainInstaller *toolchain.Installer) platform.Installer {
	return &Installer{
		deps:      deps,
		toolchain: toolchainInstaller,
	}
}

// NewRealDeps returns production dependencies for macOS setup.
func NewRealDeps() Deps {
	productionDeps := realDeps{}
	return Deps{
		Commands: productionDeps,
		Lookup:   productionDeps,
		Download: productionDeps,
		Files:    productionDeps,
		Env:      productionDeps,
		Catalog:  productionDeps,
		Paths:    productionDeps,
		Sudo:     productionDeps,
	}
}

// Name returns the display name for this installer.
func (installer *Installer) Name() string {
	return "macOS"
}

// Supports reports whether this installer handles the provided host.
func (installer *Installer) Supports(host platform.Host) bool {
	return host.GOOS == platform.GOOSDarwin
}

// Install applies macOS-specific sync setup.
func (installer *Installer) Install(ctx context.Context, request platform.Request) error {
	_ = request.UseDefaults

	installer.toolchain.EnsureBootstrapPathEntries()
	common.InfoContext(ctx, request.Logger, "  running macOS bootstrap")

	if err := installer.ensureHomebrewInstalled(ctx, request.Logger); err != nil {
		return err
	}
	if err := installer.runMacDefaults(ctx, request.Logger); err != nil {
		return err
	}
	if installer.deps.Lookup.HasCommand("brew") {
		_ = installer.deps.Commands.RunWithLogger(ctx, request.Logger, "brew", "cleanup")
	}
	if err := installer.installMacPackages(ctx, request.StrictMode, request.Logger); err != nil {
		return err
	}
	if err := installer.toolchain.InstallRustupIfNeeded(ctx, request.Logger); err != nil {
		slog.WarnContext(ctx, "platform/macos: install rustup", "err", err)
		return fmt.Errorf("install rustup: %w", err)
	}
	if err := installer.installTouchIDHelper(ctx, request.Logger); err != nil {
		return err
	}
	return installer.ensureMacPatchIfNeeded(ctx, request.Logger)
}

func (installer *Installer) ensureHomebrewInstalled(ctx context.Context, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "platform/macos: ensureHomebrewInstalled")
	if installer.deps.Lookup.HasCommand("brew") {
		return nil
	}

	scriptPath, err := installer.deps.Download.DownloadToTempFile(ctx, logger, "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh")
	if err != nil {
		slog.WarnContext(ctx, "downloading tool", "err", err)
		return fmt.Errorf("downloading tool: %w", err)
	}
	defer installer.deps.Files.Remove(scriptPath)

	if err := installer.deps.Commands.RunWithLogger(ctx, logger, "bash", scriptPath); err != nil {
		slog.WarnContext(ctx, "running bash", "err", err)
		return fmt.Errorf("running bash: %w", err)
	}

	return nil
}

func (installer *Installer) runMacDefaults(ctx context.Context, logger *telemetry.Logger) error {
	commands := [][]string{
		{"defaults", "write", "com.apple.finder", "AppleShowAllFiles", "-bool", "true"},
		{"defaults", "write", "com.apple.finder", "ShowPathbar", "-bool", "true"},
		{"defaults", "write", "com.apple.finder", "ShowStatusBar", "-bool", "true"},
		{"defaults", "write", "NSGlobalDomain", "AppleShowAllExtensions", "-bool", "true"},
		{"defaults", "write", "com.apple.desktopservices", "DSDontWriteNetworkStores", "-bool", "true"},
		{"defaults", "write", "com.apple.desktopservices", "DSDontWriteUSBStores", "-bool", "true"},
		{"defaults", "write", "com.apple.dock", "mouse-over-hilite-stack", "-bool", "true"},
		{"defaults", "write", "com.apple.dock", "mineffect", "-string", "suck"},
	}

	for _, args := range commands {
		if err := installer.deps.Commands.RunWithLogger(ctx, logger, args[0], args[1:]...); err != nil {
			slog.WarnContext(ctx, "running defaults", "err", err)
			return fmt.Errorf("running defaults: %w", err)
		}
	}

	screenshotDir := catalog.DefaultMacConfig().ScreenshotDir
	if screenshotDir != "" {
		shotDir := os.Expand(screenshotDir, func(key string) string {
			return installer.deps.Env.Getenv(key)
		})
		if err := installer.deps.Files.MkdirAll(filepath.Clean(shotDir), 0o755); err == nil {
			_ = installer.deps.Commands.RunWithLogger(ctx, logger, "defaults", "write", "com.apple.screencapture", "location", shotDir)
		}
	}
	_ = installer.deps.Commands.RunWithLogger(ctx, logger, "defaults", "write", "com.google.Chrome", "BuiltInDnsClientEnabled", "-bool", "false")
	_ = installer.deps.Commands.RunWithLogger(ctx, logger, "killall", "Finder")
	_ = installer.deps.Commands.RunWithLogger(ctx, logger, "killall", "Dock")
	_ = installer.deps.Commands.RunWithLogger(ctx, logger, "killall", "SystemUIServer")

	return nil
}

func (installer *Installer) installTouchIDHelper(ctx context.Context, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "platform/macos: installTouchIDHelper")
	scriptPath, err := installer.deps.Download.DownloadToTempFile(ctx, logger, "https://git.io/sudo-touch-id")
	if err != nil {
		slog.WarnContext(ctx, "downloading tool", "err", err)
		return fmt.Errorf("downloading tool: %w", err)
	}
	defer installer.deps.Files.Remove(scriptPath)

	if err := installer.deps.Commands.RunWithLogger(ctx, logger, "sh", scriptPath); err != nil {
		slog.WarnContext(ctx, "running sh", "err", err)
		return fmt.Errorf("running sh: %w", err)
	}

	return nil
}

func (installer *Installer) ensureMacPatchIfNeeded(ctx context.Context, logger *telemetry.Logger) error {
	cfg := installer.deps.Catalog.MacPatchConfig()
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	patchScript := cfg.PatchScript
	if patchScript == "" {
		patchScript = filepath.Join(installer.deps.Env.Getenv("DOTDOTFILES"), "bash", "setup", "platform", "patch-etc-zsh.bash")
	}
	patchScript = installer.deps.Paths.ResolveConfigPath(patchScript, installer.deps.Env.Getenv("DOTDOTFILES"))

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

	zprofile, err := installer.deps.Files.ReadFile(zprofilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			if logger != nil {
				logger.WarnContextWithErr(ctx, "read zprofile", err)
			}
			return fmt.Errorf("read zprofile: %w", err)
		}
		return nil
	}

	zshrc, err := installer.deps.Files.ReadFile(zshrcPath)
	if err != nil {
		if !os.IsNotExist(err) {
			if logger != nil {
				logger.WarnContextWithErr(ctx, "read zshrc", err)
			}
			return fmt.Errorf("read zshrc: %w", err)
		}
		return nil
	}

	if strings.Contains(string(zprofile), cfg.Sentinel) && strings.Contains(string(zshrc), cfg.Sentinel) {
		return nil
	}
	if !installer.deps.Sudo.HasSudoAccess(ctx, logger) {
		return nil
	}

	if err := installer.deps.Commands.RunWithLogger(ctx, logger, "sudo", "bash", patchScript); err != nil {
		slog.WarnContext(ctx, "running sudo bash", "err", err)
		return fmt.Errorf("running sudo bash: %w", err)
	}

	return nil
}

func (installer *Installer) installMacPackages(ctx context.Context, strictMode bool, logger *telemetry.Logger) error {
	if !installer.deps.Lookup.HasCommand("brew") {
		return nil
	}

	cfg := installer.deps.Catalog.PackageConfig()
	if cfg == nil {
		return nil
	}

	if err := installer.deps.Commands.RunWithLogger(ctx, logger, "brew", "update", "--quiet"); err != nil {
		common.WarnContext(ctx, logger, "  brew update failed, continuing")
		if strictMode {
			slog.WarnContext(ctx, "running brew update", "err", err)
			return fmt.Errorf("running brew update: %w", err)
		}
	}

	installer.trustMacTapPackages(ctx, cfg, logger)

	if err := installer.installMacFormulae(ctx, cfg, strictMode, logger); err != nil {
		return err
	}
	return installer.installMacCasks(ctx, cfg, strictMode, logger)
}

func (installer *Installer) trustMacTapPackages(ctx context.Context, cfg *catalog.PackageConfig, logger *telemetry.Logger) {
	formulae := tapQualifiedNames(append(append([]string{}, cfg.CommonPackages...), cfg.BrewSpecific...))
	for _, name := range formulae {
		_ = installer.deps.Commands.RunWithLogger(ctx, logger, "brew", "trust", "--formula", name)
	}

	caskNames := make([]string, 0, len(cfg.BrewCasks))
	for cask := range cfg.BrewCasks {
		caskNames = append(caskNames, cask)
	}
	for _, name := range tapQualifiedNames(caskNames) {
		_ = installer.deps.Commands.RunWithLogger(ctx, logger, "brew", "trust", "--cask", name)
	}
}

func tapQualifiedNames(names []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, name := range names {
		if !strings.Contains(name, "/") {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func (installer *Installer) installMacFormulae(ctx context.Context, cfg *catalog.PackageConfig, strictMode bool, logger *telemetry.Logger) error {
	formulae := make(map[string]struct{})
	formulaList := make([]string, 0, len(cfg.CommonPackages)+len(cfg.BrewSpecific))
	for _, item := range append(cfg.CommonPackages, cfg.BrewSpecific...) {
		name := brewPackageName(item)
		if name == "" {
			continue
		}
		if installer.brewFormulaInstalled(ctx, name) {
			continue
		}
		if _, ok := formulae[name]; ok {
			continue
		}
		formulae[name] = struct{}{}
		formulaList = append(formulaList, name)
	}

	if len(formulaList) == 0 {
		return nil
	}

	args := append([]string{"install"}, formulaList...)
	if err := installer.deps.Commands.RunWithLogger(ctx, logger, "brew", args...); err != nil {
		common.WarnContext(ctx, logger, "  brew formula install returned an error")
		if strictMode {
			slog.WarnContext(ctx, "running brew install", "err", err)
			return fmt.Errorf("running brew install: %w", err)
		}
	}

	return nil
}

func (installer *Installer) installMacCasks(ctx context.Context, cfg *catalog.PackageConfig, strictMode bool, logger *telemetry.Logger) error {
	for cask, app := range cfg.BrewCasks {
		if !installer.macCaskNeedsInstall(ctx, cask, app) {
			continue
		}
		if err := installer.deps.Commands.RunWithLogger(ctx, logger, "brew", "install", "--cask", cask); err != nil {
			common.WarnContextf(ctx, logger, "  failed to install cask %s", cask)
			if strictMode {
				slog.WarnContext(ctx, "running brew cask install", "cask", cask, "err", err)
				return fmt.Errorf("running brew cask install %s: %w", cask, err)
			}
		}
	}

	return nil
}

func (installer *Installer) macCaskNeedsInstall(ctx context.Context, cask string, app string) bool {
	if installer.brewCaskInstalled(ctx, cask) {
		return false
	}
	if app == "" {
		return true
	}
	return !installer.macCaskAppExists(app, installer.deps.Env.Getenv("HOME"))
}

func (installer *Installer) macCaskAppExists(app string, home string) bool {
	return slices.ContainsFunc(installer.macCaskAppPaths(app, home), installer.deps.Files.PathExists)
}

func (installer *Installer) macCaskAppPaths(app string, home string) []string {
	appNames := []string{app}
	if !strings.HasSuffix(app, ".app") {
		appNames = append(appNames, app+".app")
	}

	applicationsDirs := []string{"/Applications"}
	if home != "" {
		applicationsDirs = append(applicationsDirs, filepath.Join(home, "Applications"))
	}

	paths := make([]string, 0, len(appNames)*len(applicationsDirs))
	for _, applicationsDir := range applicationsDirs {
		for _, appName := range appNames {
			paths = append(paths, filepath.Clean(filepath.Join(applicationsDir, appName)))
		}
	}
	return paths
}

func (installer *Installer) brewFormulaInstalled(ctx context.Context, formula string) bool {
	return installer.deps.Commands.CommandSucceeds(ctx, "brew", "list", "--formula", formula)
}

func (installer *Installer) brewCaskInstalled(ctx context.Context, cask string) bool {
	return installer.deps.Commands.CommandSucceeds(ctx, "brew", "list", "--cask", cask)
}

func brewPackageName(packageName string) string {
	if packageName == "tshark" {
		return "wireshark"
	}
	return packageName
}

type realDeps struct{}

func (realDeps) RunWithLogger(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error {
	if err := cmdexec.RunWithLogger(ctx, logger, command, args...); err != nil {
		slog.WarnContext(ctx, "platform/macos: run command", "command", command, "err", err)
		return fmt.Errorf("run %s: %w", command, err)
	}
	return nil
}

func (realDeps) CommandSucceeds(ctx context.Context, command string, args ...string) bool {
	_, err := cmdexec.OutputWithLoggerAndEnv(ctx, nil, nil, command, args...)
	return err == nil
}

func (realDeps) HasCommand(name string) bool {
	return runner.HasCommand(name)
}

func (realDeps) DownloadToTempFile(ctx context.Context, logger *telemetry.Logger, fileURL string) (string, error) {
	path, err := tools.DownloadToTempFile(ctx, logger, fileURL)
	if err != nil {
		slog.WarnContext(ctx, "platform/macos: download file", "url", fileURL, "err", err)
		return "", fmt.Errorf("download %s: %w", fileURL, err)
	}
	return path, nil
}

func (realDeps) MkdirAll(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		slog.Warn("platform/macos: mkdir", "path", path, "err", err)
		return fmt.Errorf("mkdir %s: %w", path, err)
	}
	return nil
}

func (realDeps) PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (realDeps) ReadFile(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("platform/macos: read file", "path", path, "err", err)
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return content, nil
}

func (realDeps) Remove(path string) error {
	slog.Debug("platform/macos: remove file", "path", path)
	if err := os.Remove(path); err != nil {
		slog.Warn("platform/macos: remove file", "path", path, "err", err)
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func (realDeps) Getenv(key string) string {
	return os.Getenv(key)
}

func (realDeps) PackageConfig() *catalog.PackageConfig {
	return catalog.DefaultPackageConfig()
}

func (realDeps) MacPatchConfig() *catalog.MacPatchConfig {
	return catalog.DefaultMacPatchConfig()
}

func (realDeps) ResolveConfigPath(value string, dotfiles string) string {
	return util.ResolveConfigPath(value, dotfiles)
}

func (realDeps) HasSudoAccess(ctx context.Context, logger *telemetry.Logger) bool {
	return common.HasSudoAccess(ctx, logger)
}
