// Package toolchain implements shared bootstrap path and toolchain setup.
package toolchain

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/sync/tools"
	"goodkind.io/.dotfiles/internal/telemetry"
)

type commandRunner interface {
	RunWithLogger(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error
}

type commandLookup interface {
	HasCommand(name string) bool
}

type downloader interface {
	DownloadToTempFile(ctx context.Context, logger *telemetry.Logger, fileURL string) (string, error)
}

type fileSystem interface {
	Remove(path string) error
}

type environment interface {
	Getenv(key string) string
	Setenv(key string, value string) error
}

type catalogProvider interface {
	PackageConfig() *catalog.PackageConfig
}

// Deps holds the toolchain package dependencies.
type Deps struct {
	Commands   commandRunner
	Lookup     commandLookup
	Downloader downloader
	Files      fileSystem
	Env        environment
	Catalog    catalogProvider
}

// Installer manages shared toolchain setup.
type Installer struct {
	deps Deps
}

// New builds a toolchain installer from explicit dependencies.
func New(deps Deps) *Installer {
	return &Installer{deps: deps}
}

// NewRealDeps returns production dependencies for toolchain setup.
func NewRealDeps() Deps {
	productionDeps := realDeps{}
	return Deps{
		Commands:   productionDeps,
		Lookup:     productionDeps,
		Downloader: productionDeps,
		Files:      productionDeps,
		Env:        productionDeps,
		Catalog:    productionDeps,
	}
}

// EnsureBootstrapPathEntries prepends required bootstrap paths to PATH.
func (installer *Installer) EnsureBootstrapPathEntries() {
	home := installer.deps.Env.Getenv("HOME")
	goLocalRoot := installer.deps.Env.Getenv("GO_LOCAL_ROOT")
	if goLocalRoot == "" && home != "" {
		goLocalRoot = filepath.Join(home, ".local", "go")
	}

	entries := []string{
		"/opt/homebrew/bin",
		"/usr/local/bin",
	}
	if home != "" {
		entries = append(entries, filepath.Join(home, ".local", "bin"))
		entries = append(entries, filepath.Join(home, ".cargo", "bin"))
	}
	if goLocalRoot != "" {
		entries = append(entries, filepath.Join(goLocalRoot, "bin"))
	}

	installer.prependPathEntries(entries)
}

// InstallRustupIfNeeded installs rustup when Rust is not already available.
func (installer *Installer) InstallRustupIfNeeded(ctx context.Context, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "platform/toolchain: InstallRustupIfNeeded")
	if installer.deps.Lookup.HasCommand("rustup") {
		return nil
	}
	if installer.deps.Lookup.HasCommand("cargo") && installer.deps.Lookup.HasCommand("rustc") {
		return nil
	}

	scriptPath, err := installer.deps.Downloader.DownloadToTempFile(ctx, logger, "https://sh.rustup.rs")
	if err != nil {
		slog.WarnContext(ctx, "downloading tool", "err", err)
		return fmt.Errorf("downloading tool: %w", err)
	}
	defer installer.deps.Files.Remove(scriptPath)

	if err := installer.deps.Commands.RunWithLogger(ctx, logger, "sh", scriptPath, "-y"); err != nil {
		slog.WarnContext(ctx, "running sh", "err", err)
		return fmt.Errorf("running sh: %w", err)
	}

	installer.EnsureBootstrapPathEntries()
	return nil
}

// InstallGoToolsIfNeeded installs missing Go and Cargo tools from the catalog.
func (installer *Installer) InstallGoToolsIfNeeded(ctx context.Context, logger *telemetry.Logger) error {
	if !installer.deps.Lookup.HasCommand("go") {
		return nil
	}

	common.InfoContext(ctx, logger, "  checking Go tools")
	cfg := installer.deps.Catalog.PackageConfig()
	if cfg == nil {
		common.WarnContext(ctx, logger, "  failed to parse package config for Go tools: no config")
		return nil
	}

	for bin, pkg := range cfg.GoPackages {
		if installer.deps.Lookup.HasCommand(bin) {
			continue
		}
		common.InfoContextf(ctx, logger, "  installing go tool %s", bin)
		if err := installer.deps.Commands.RunWithLogger(ctx, logger, "go", "install", pkg); err != nil {
			slog.WarnContext(ctx, "running go install", "err", err)
			return fmt.Errorf("running go install: %w", err)
		}
	}

	_ = installer.installCargoToolsIfNeeded(ctx, cfg, logger)
	return nil
}

func (installer *Installer) prependPathEntries(entries []string) {
	currentPath := installer.deps.Env.Getenv("PATH")
	seen := make(map[string]struct{})
	for pathEntry := range strings.SplitSeq(currentPath, ":") {
		if pathEntry == "" {
			continue
		}
		seen[pathEntry] = struct{}{}
	}

	newEntries := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		newEntries = append(newEntries, entry)
	}
	if len(newEntries) == 0 {
		return
	}

	updatedEntries := append([]string{}, newEntries...)
	updatedEntries = append(updatedEntries, strings.Split(currentPath, ":")...)
	_ = installer.deps.Env.Setenv("PATH", strings.Join(updatedEntries, ":"))
}

func (installer *Installer) installCargoToolsIfNeeded(ctx context.Context, cfg *catalog.PackageConfig, logger *telemetry.Logger) error {
	// The injected lookup keeps tests deterministic; tools.CargoAvailable adds the
	// real-host case where cargo sits at $CARGO_HOME/$HOME/.cargo off PATH so the
	// rust bootstrap's freshly installed cargo is not silently skipped.
	if !installer.deps.Lookup.HasCommand("cargo") && !tools.CargoAvailable() {
		return nil
	}
	if len(cfg.CargoPackages) == 0 {
		return nil
	}

	cargo := tools.CargoExecutable()
	for _, tool := range cfg.CargoPackages {
		if installer.deps.Lookup.HasCommand(tool) {
			continue
		}
		if err := installer.deps.Commands.RunWithLogger(ctx, logger, cargo, "install", tool); err != nil {
			slog.WarnContext(ctx, "running cargo install", "err", err)
			return fmt.Errorf("running cargo install: %w", err)
		}
	}

	for tool, repo := range cfg.CargoGitPackages {
		if tool == "" || repo == "" || installer.deps.Lookup.HasCommand(tool) {
			continue
		}
		parts := strings.SplitN(repo, "|", 2)
		args := []string{"install", "--git", parts[0]}
		if len(parts) == 2 {
			features := strings.TrimSpace(parts[1])
			if features != "" {
				args = append(args, "--features", features)
			}
		}
		if err := installer.deps.Commands.RunWithLogger(ctx, logger, cargo, args...); err != nil {
			slog.WarnContext(ctx, "running cargo install", "err", err)
			return fmt.Errorf("running cargo install: %w", err)
		}
	}

	return nil
}

type realDeps struct{}

func (realDeps) RunWithLogger(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error {
	if err := cmdexec.RunWithLogger(ctx, logger, command, args...); err != nil {
		slog.WarnContext(ctx, "platform/toolchain: run command", "command", command, "err", err)
		return fmt.Errorf("run %s: %w", command, err)
	}
	return nil
}

func (realDeps) HasCommand(name string) bool {
	return runner.HasCommand(name)
}

func (realDeps) DownloadToTempFile(ctx context.Context, logger *telemetry.Logger, fileURL string) (string, error) {
	path, err := tools.DownloadToTempFile(ctx, logger, fileURL)
	if err != nil {
		slog.WarnContext(ctx, "platform/toolchain: download file", "url", fileURL, "err", err)
		return "", fmt.Errorf("download %s: %w", fileURL, err)
	}
	return path, nil
}

func (realDeps) Remove(path string) error {
	slog.Debug("platform/toolchain: remove file", "path", path)
	if err := os.Remove(path); err != nil {
		slog.Warn("platform/toolchain: remove file", "path", path, "err", err)
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func (realDeps) Getenv(key string) string {
	return os.Getenv(key)
}

func (realDeps) Setenv(key string, value string) error {
	if err := os.Setenv(key, value); err != nil {
		slog.Warn("platform/toolchain: setenv", "key", key, "err", err)
		return fmt.Errorf("setenv %s: %w", key, err)
	}
	return nil
}

func (realDeps) PackageConfig() *catalog.PackageConfig {
	return catalog.DefaultPackageConfig()
}
