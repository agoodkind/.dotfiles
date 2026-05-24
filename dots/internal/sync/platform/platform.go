// Package platform implements platform-specific sync boundaries.
package platform

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/telemetry"
)

const (
	// GOOSDarwin is the runtime GOOS value for macOS hosts.
	GOOSDarwin = "darwin"
	// GOOSLinux is the runtime GOOS value for Linux hosts.
	GOOSLinux = "linux"

	// DistributionUnknown indicates that no supported Linux distribution was detected.
	DistributionUnknown = ""
	// DistributionDebian identifies Debian-family hosts reported as Debian.
	DistributionDebian = "debian"
	// DistributionUbuntu identifies Debian-family hosts reported as Ubuntu.
	DistributionUbuntu = "ubuntu"
)

// Host describes the current runtime platform.
type Host struct {
	GOOS         string
	Distribution string
}

// Request carries runtime configuration into a platform installer.
type Request struct {
	Host        Host
	Logger      *telemetry.Logger
	UseDefaults bool
	StrictMode  bool
}

// Installer applies platform-specific setup for a host.
type Installer interface {
	Name() string
	Supports(host Host) bool
	Install(ctx context.Context, request Request) error
}

// HostSource describes a runtime host detector.
type HostSource interface {
	Host(ctx context.Context) Host
}

// GOOSReader returns the runtime GOOS.
type GOOSReader interface {
	GOOS() string
}

// FileReader reads files from disk.
type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

// RuntimeHostSourceDeps are the side effects used by RuntimeHostSource.
type RuntimeHostSourceDeps struct {
	GOOS  GOOSReader
	Files FileReader
}

// RuntimeHostSource detects the current host from runtime and os-release data.
type RuntimeHostSource struct {
	deps RuntimeHostSourceDeps
}

// NewRuntimeHostSource builds a host source from explicit dependencies.
func NewRuntimeHostSource(deps RuntimeHostSourceDeps) *RuntimeHostSource {
	if deps.GOOS == nil || deps.Files == nil {
		runtimeDeps := realHostSourceDeps{}
		if deps.GOOS == nil {
			deps.GOOS = runtimeDeps
		}
		if deps.Files == nil {
			deps.Files = runtimeDeps
		}
	}
	return &RuntimeHostSource{deps: deps}
}

// NewRealHostSourceDeps returns production dependencies for host detection.
func NewRealHostSourceDeps() RuntimeHostSourceDeps {
	runtimeDeps := realHostSourceDeps{}
	return RuntimeHostSourceDeps{
		GOOS:  runtimeDeps,
		Files: runtimeDeps,
	}
}

// Host returns the current runtime host.
func (source *RuntimeHostSource) Host(_ context.Context) Host {
	host := Host{
		GOOS:         source.deps.GOOS.GOOS(),
		Distribution: DistributionUnknown,
	}
	if host.GOOS != GOOSLinux {
		return host
	}

	content, err := source.deps.Files.ReadFile("/etc/os-release")
	if err != nil {
		return host
	}

	lower := strings.ToLower(string(content))
	switch {
	case strings.Contains(lower, "id=ubuntu"):
		host.Distribution = DistributionUbuntu
	case strings.Contains(lower, "debian"):
		host.Distribution = DistributionDebian
	}

	return host
}

// RunInstall selects the first matching installer and runs it.
func RunInstall(ctx context.Context, hostSource HostSource, installers []Installer, request Request) error {
	host := Host{
		GOOS:         "",
		Distribution: DistributionUnknown,
	}
	if hostSource != nil {
		host = hostSource.Host(ctx)
	}
	request.Host = host

	for _, installer := range installers {
		if !installer.Supports(host) {
			continue
		}
		common.InfoContextf(ctx, request.Logger, "  Running %s setup", installer.Name())
		if err := installer.Install(ctx, request); err != nil {
			slog.WarnContext(ctx, "platform: installer failed", "name", installer.Name(), "err", err)
			return fmt.Errorf("run %s installer: %w", installer.Name(), err)
		}
		return nil
	}

	common.WarnContext(ctx, request.Logger, "  No OS-specific setup handler for this platform")
	return nil
}

type realHostSourceDeps struct{}

func (realHostSourceDeps) GOOS() string {
	return runtime.GOOS
}

func (realHostSourceDeps) ReadFile(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("platform: read file", "path", path, "err", err)
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return content, nil
}
