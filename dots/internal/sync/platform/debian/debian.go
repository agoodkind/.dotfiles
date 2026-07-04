// Package debian implements Debian-family sync setup.
package debian

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/sync/platform"
	"goodkind.io/.dotfiles/internal/sync/platform/toolchain"
	"goodkind.io/.dotfiles/internal/telemetry"
)

const ppaReleaseProbeTimeout = 15 * time.Second

type commandRunner interface {
	RunWithLogger(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error
	OutputWithLoggerAndEnv(ctx context.Context, logger *telemetry.Logger, env []string, command string, args ...string) (string, error)
}

type commandLookup interface {
	HasCommand(name string) bool
}

type catalogProvider interface {
	PackageConfig() *catalog.PackageConfig
}

type privilegedRunner interface {
	Run(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error
}

// ppaPublishChecker reports whether a launchpad PPA publishes packages for the
// running Ubuntu release. A freshly released Ubuntu (e.g. 26.04 "resolute")
// often runs weeks ahead of third-party PPAs, so adding a PPA that has no
// Release file for the current codename would poison every later apt-get with a
// 404. Probing first lets the bootstrap skip those PPAs cleanly.
type ppaPublishChecker interface {
	PublishesForCurrentRelease(ctx context.Context, ppa string) bool
}

// Deps holds the Debian installer dependencies.
type Deps struct {
	Commands   commandRunner
	Lookup     commandLookup
	Catalog    catalogProvider
	Privileged privilegedRunner
	PPAChecker ppaPublishChecker
}

// Installer applies Debian-family sync steps.
type Installer struct {
	deps      Deps
	toolchain *toolchain.Installer
}

// New builds a Debian installer from explicit dependencies.
func New(deps Deps, toolchainInstaller *toolchain.Installer) platform.Installer {
	return &Installer{
		deps:      deps,
		toolchain: toolchainInstaller,
	}
}

// NewRealDeps returns production dependencies for Debian setup.
func NewRealDeps() Deps {
	productionDeps := realDeps{}
	return Deps{
		Commands:   productionDeps,
		Lookup:     productionDeps,
		Catalog:    productionDeps,
		Privileged: productionDeps,
		PPAChecker: productionDeps,
	}
}

// Name returns the display name for this installer.
func (installer *Installer) Name() string {
	return "Debian/Ubuntu/Proxmox"
}

// Supports reports whether this installer handles the provided host.
func (installer *Installer) Supports(host platform.Host) bool {
	if host.GOOS != platform.GOOSLinux {
		return false
	}
	return host.Distribution == platform.DistributionDebian || host.Distribution == platform.DistributionUbuntu
}

// Install applies Debian-family sync setup.
func (installer *Installer) Install(ctx context.Context, request platform.Request) error {
	_ = request.UseDefaults

	installer.toolchain.EnsureBootstrapPathEntries()
	common.InfoContext(ctx, request.Logger, "  running Linux bootstrap")

	if err := common.EnsurePasswordlessSudo(ctx, request.Logger, "root"); err != nil {
		slog.WarnContext(ctx, "platform/debian: enable passwordless sudo", "err", err)
		common.WarnContext(ctx, request.Logger, "  failed to enable passwordless sudo; continuing without it")
	}
	if err := installer.installDebianPackages(ctx, request.Host, request.Logger); err != nil {
		return err
	}
	if err := installer.toolchain.InstallRustupIfNeeded(ctx, request.Logger); err != nil {
		slog.WarnContext(ctx, "platform/debian: install rustup", "err", err)
		return fmt.Errorf("install rustup: %w", err)
	}
	if err := installer.toolchain.InstallGoToolsIfNeeded(ctx, request.Logger); err != nil {
		slog.WarnContext(ctx, "platform/debian: install Go tools", "err", err)
		return fmt.Errorf("install Go tools: %w", err)
	}

	return nil
}

func (installer *Installer) installUbuntuPPAs(ctx context.Context, host platform.Host, cfg *catalog.PackageConfig, logger *telemetry.Logger) {
	if host.Distribution != platform.DistributionUbuntu {
		return
	}
	if len(cfg.UbuntuPPAs) == 0 {
		return
	}

	if err := installer.deps.Privileged.Run(ctx, logger, "apt-get", "install", "-y", "-qq", "software-properties-common"); err != nil {
		slog.WarnContext(ctx, "installUbuntuPPAs: installing software-properties-common", "err", err)
		common.WarnContext(ctx, logger, "  failed to install software-properties-common; skipping PPAs")
		return
	}

	added := false
	for _, ppa := range cfg.UbuntuPPAs {
		if installer.deps.PPAChecker != nil && !installer.deps.PPAChecker.PublishesForCurrentRelease(ctx, ppa) {
			common.InfoContextf(ctx, logger, "  skipping PPA %s: no release published for this Ubuntu version", ppa)
			continue
		}
		common.InfoContextf(ctx, logger, "  adding PPA %s", ppa)
		if err := installer.deps.Privileged.Run(ctx, logger, "add-apt-repository", "-y", ppa); err != nil {
			slog.WarnContext(ctx, "installUbuntuPPAs: add-apt-repository", "ppa", ppa, "err", err)
			common.WarnContextf(ctx, logger, "  failed to add PPA %s", ppa)
			continue
		}
		added = true
	}

	if added {
		if err := installer.deps.Privileged.Run(ctx, logger, "apt-get", "update", "-qq"); err != nil {
			slog.WarnContext(ctx, "installUbuntuPPAs: apt-get update after PPAs", "err", err)
		}
	}
}

// ppaPublishesForCurrentRelease reports whether a launchpad PPA publishes a
// Release file for the running Ubuntu codename. It treats only an explicit
// not-found as unpublished; a missing codename or any transient probe error
// resolves to true so a flaky network never drops an otherwise-valid PPA.
func ppaPublishesForCurrentRelease(ctx context.Context, ppa string) bool {
	owner, name, ok := parsePPAOwnerName(ppa)
	if !ok {
		slog.DebugContext(ctx, "debian: PPA is not in ppa:owner/name form; adding without a release probe", "ppa", ppa)
		return true
	}
	codename := ubuntuReleaseCodename()
	if codename == "" {
		slog.DebugContext(ctx, "debian: no Ubuntu codename found; adding PPA without a release probe", "ppa", ppa)
		return true
	}
	releaseURL := fmt.Sprintf("https://ppa.launchpadcontent.net/%s/%s/ubuntu/dists/%s/Release", owner, name, codename)
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, releaseURL, nil)
	if err != nil {
		slog.DebugContext(ctx, "debian: building PPA release probe failed; adding PPA anyway", "ppa", ppa, "err", err)
		return true
	}
	client := &http.Client{Timeout: ppaReleaseProbeTimeout}
	response, err := client.Do(request)
	if err != nil {
		slog.DebugContext(ctx, "debian: PPA release probe request failed; adding PPA anyway", "ppa", ppa, "err", err)
		return true
	}
	defer response.Body.Close()
	published := response.StatusCode != http.StatusNotFound && response.StatusCode != http.StatusGone
	slog.DebugContext(ctx, "debian: probed PPA release", "ppa", ppa, "codename", codename, "status", response.StatusCode, "published", published)
	return published
}

// parsePPAOwnerName splits a "ppa:owner/name" reference into its owner and name.
func parsePPAOwnerName(ppa string) (string, string, bool) {
	trimmed := strings.TrimPrefix(ppa, "ppa:")
	owner, name, found := strings.Cut(trimmed, "/")
	if !found || owner == "" || name == "" {
		return "", "", false
	}
	return owner, name, true
}

// ubuntuReleaseCodename reads VERSION_CODENAME from /etc/os-release (e.g. "noble"
// for 24.04, "resolute" for 26.04), returning "" when it cannot be determined.
func ubuntuReleaseCodename() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		slog.Debug("debian: reading /etc/os-release failed", "err", err)
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		value, ok := strings.CutPrefix(line, "VERSION_CODENAME=")
		if !ok {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"`)
	}
	return ""
}

func (installer *Installer) installDebianPackages(ctx context.Context, host platform.Host, logger *telemetry.Logger) error {
	if !installer.deps.Lookup.HasCommand("apt-get") {
		return nil
	}

	cfg := installer.deps.Catalog.PackageConfig()
	if cfg == nil {
		return nil
	}

	if err := installer.deps.Privileged.Run(ctx, logger, "apt-get", "update", "-qq"); err != nil {
		slog.WarnContext(ctx, "running apt-get update", "err", err)
		return fmt.Errorf("running apt-get update: %w", err)
	}

	installer.installUbuntuPPAs(ctx, host, cfg, logger)

	packages := make(map[string]struct{})
	aptPackages := make([]string, 0, len(cfg.CommonPackages)+len(cfg.AptSpecific))
	for _, item := range append(cfg.CommonPackages, cfg.AptSpecific...) {
		if isSnapPackage(item, cfg.SnapPackages) {
			continue
		}
		for mapped := range strings.FieldsSeq(aptPackageName(item)) {
			if _, ok := packages[mapped]; ok {
				continue
			}
			packages[mapped] = struct{}{}
			aptPackages = append(aptPackages, mapped)
		}
	}

	if len(aptPackages) > 0 {
		args := append([]string{"install", "-y", "-qq"}, aptPackages...)
		if err := installer.deps.Privileged.Run(ctx, logger, "apt-get", args...); err != nil {
			slog.WarnContext(ctx, "running apt-get install", "err", err)
			return fmt.Errorf("running apt-get install: %w", err)
		}
	}

	for _, pkg := range cfg.SnapPackages {
		target := snapPackageName(pkg)
		if target == "" {
			continue
		}
		if installer.deps.Lookup.HasCommand("snap") {
			if err := installer.installSnapPackage(ctx, target, logger); err != nil {
				common.WarnContext(ctx, logger, "  failed to install snap package "+target)
			}
		}
	}

	return nil
}

func (installer *Installer) installSnapPackage(ctx context.Context, packageName string, logger *telemetry.Logger) error {
	if installer.deps.Commands.RunWithLogger(ctx, logger, "snap", "list", packageName) == nil {
		return nil
	}

	args := []string{"install", packageName}
	if installer.isSnapClassic(ctx, packageName) {
		args = []string{"install", "--classic", packageName}
	}
	if err := installer.deps.Privileged.Run(ctx, logger, "snap", args...); err != nil {
		if installer.isSnapClassic(ctx, packageName) {
			slog.WarnContext(ctx, "running snap install", "err", err)
			return fmt.Errorf("running snap install: %w", err)
		}
		if installer.deps.Commands.RunWithLogger(ctx, logger, "snap", "info", packageName) != nil {
			slog.WarnContext(ctx, "running snap install", "err", err)
			return fmt.Errorf("running snap install: %w", err)
		}
		if err := installer.deps.Privileged.Run(ctx, logger, "snap", "install", "--classic", packageName); err != nil {
			slog.WarnContext(ctx, "running snap install", "err", err)
			return fmt.Errorf("running snap install: %w", err)
		}
	}

	return nil
}

func (installer *Installer) isSnapClassic(ctx context.Context, packageName string) bool {
	output, err := installer.deps.Commands.OutputWithLoggerAndEnv(ctx, nil, nil, "snap", "info", packageName)
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

func isSnapPackage(packageName string, snapList []string) bool {
	return slices.Contains(snapList, packageName)
}

func snapPackageName(packageName string) string {
	if packageName == "neovim" {
		return "nvim"
	}
	return packageName
}

type realDeps struct{}

func (realDeps) RunWithLogger(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error {
	if err := cmdexec.RunWithLogger(ctx, logger, command, args...); err != nil {
		slog.WarnContext(ctx, "platform/debian: run command", "command", command, "err", err)
		return fmt.Errorf("run %s: %w", command, err)
	}
	return nil
}

func (realDeps) OutputWithLoggerAndEnv(ctx context.Context, logger *telemetry.Logger, env []string, command string, args ...string) (string, error) {
	output, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, env, command, args...)
	if err != nil {
		slog.WarnContext(ctx, "platform/debian: output command", "command", command, "err", err)
		return output, fmt.Errorf("output %s: %w", command, err)
	}
	return output, nil
}

func (realDeps) HasCommand(name string) bool {
	return runner.HasCommand(name)
}

func (realDeps) PackageConfig() *catalog.PackageConfig {
	return catalog.DefaultPackageConfig()
}

func (realDeps) Run(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error {
	if err := common.RunDebianPrivilegedCommand(ctx, logger, command, args...); err != nil {
		slog.WarnContext(ctx, "platform/debian: run privileged command", "command", command, "err", err)
		return fmt.Errorf("run privileged %s: %w", command, err)
	}
	return nil
}

func (realDeps) PublishesForCurrentRelease(ctx context.Context, ppa string) bool {
	return ppaPublishesForCurrentRelease(ctx, ppa)
}
