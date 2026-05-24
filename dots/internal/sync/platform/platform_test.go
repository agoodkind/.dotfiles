package platform

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goodkind.io/.dotfiles/internal/telemetry"
)

type fakeHostSource struct {
	host Host
}

func (source fakeHostSource) Host(context.Context) Host {
	return source.host
}

type fakeInstaller struct {
	name      string
	supports  func(host Host) bool
	called    int
	request   Request
	returnErr error
}

func (installer *fakeInstaller) Name() string {
	return installer.name
}

func (installer *fakeInstaller) Supports(host Host) bool {
	return installer.supports(host)
}

func (installer *fakeInstaller) Install(_ context.Context, request Request) error {
	installer.called++
	installer.request = request
	return installer.returnErr
}

func TestRunInstallUsesFirstMatchingInstaller(t *testing.T) {
	t.Parallel()

	host := Host{GOOS: GOOSDarwin}
	first := &fakeInstaller{
		name: "macOS",
		supports: func(candidate Host) bool {
			return candidate.GOOS == GOOSDarwin
		},
	}
	second := &fakeInstaller{
		name: "fallback",
		supports: func(Host) bool {
			return true
		},
	}

	err := RunInstall(context.Background(), fakeHostSource{host: host}, []Installer{first, second}, Request{
		UseDefaults: true,
		StrictMode:  true,
	})
	if err != nil {
		t.Fatalf("RunInstall() returned error: %v", err)
	}
	if first.called != 1 {
		t.Fatalf("first installer called %d times, want 1", first.called)
	}
	if second.called != 0 {
		t.Fatalf("second installer called %d times, want 0", second.called)
	}
	if first.request.Host != host {
		t.Fatalf("installer host = %#v, want %#v", first.request.Host, host)
	}
	if !first.request.UseDefaults {
		t.Fatal("installer request UseDefaults = false, want true")
	}
	if !first.request.StrictMode {
		t.Fatal("installer request StrictMode = false, want true")
	}
}

func TestRunInstallLogsSelectedInstaller(t *testing.T) {
	t.Parallel()

	logger, logPath := newTestLogger(t)
	installer := &fakeInstaller{
		name: "macOS",
		supports: func(host Host) bool {
			return host.GOOS == GOOSDarwin
		},
	}

	err := RunInstall(context.Background(), fakeHostSource{host: Host{GOOS: GOOSDarwin}}, []Installer{installer}, Request{
		Logger: logger,
	})
	if err != nil {
		t.Fatalf("RunInstall() returned error: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("closing logger: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(logBytes), "Running macOS setup") {
		t.Fatalf("log output did not include selected installer:\n%s", logBytes)
	}
}

func TestRunInstallWarnsWhenNoInstallerMatches(t *testing.T) {
	t.Parallel()

	logger, logPath := newTestLogger(t)
	installer := &fakeInstaller{
		name: "macOS",
		supports: func(host Host) bool {
			return host.GOOS == GOOSDarwin
		},
	}

	err := RunInstall(context.Background(), fakeHostSource{host: Host{GOOS: GOOSLinux}}, []Installer{installer}, Request{
		Logger: logger,
	})
	if err != nil {
		t.Fatalf("RunInstall() returned error: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("closing logger: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(logBytes), "No OS-specific setup handler for this platform") {
		t.Fatalf("log output did not include unsupported host warning:\n%s", logBytes)
	}
}

func TestRuntimeHostSourceDetectsUbuntu(t *testing.T) {
	t.Parallel()

	source := NewRuntimeHostSource(RuntimeHostSourceDeps{
		GOOS: fakeGOOSReader{value: GOOSLinux},
		Files: fakeFileReader{
			content: "ID=ubuntu\nNAME=Ubuntu\n",
		},
	})

	host := source.Host(context.Background())
	if host.GOOS != GOOSLinux {
		t.Fatalf("host GOOS = %q, want %q", host.GOOS, GOOSLinux)
	}
	if host.Distribution != DistributionUbuntu {
		t.Fatalf("host distribution = %q, want %q", host.Distribution, DistributionUbuntu)
	}
}

type fakeGOOSReader struct {
	value string
}

func (reader fakeGOOSReader) GOOS() string {
	return reader.value
}

type fakeFileReader struct {
	content string
	err     error
}

func (reader fakeFileReader) ReadFile(string) ([]byte, error) {
	if reader.err != nil {
		return nil, reader.err
	}
	return []byte(reader.content), nil
}

func newTestLogger(t *testing.T) (*telemetry.Logger, string) {
	t.Helper()

	logPath := filepath.Join(t.TempDir(), "sync.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		t.Fatalf("creating logger: %v", err)
	}
	t.Cleanup(func() {
		_ = logger.Close()
	})
	return logger, logPath
}
