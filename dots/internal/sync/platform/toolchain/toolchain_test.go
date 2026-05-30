package toolchain

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/sync/tools"
	"goodkind.io/.dotfiles/internal/telemetry"
)

type fakeToolchainCommands struct {
	calls []toolchainCommandCall
	errs  map[string]error
}

type toolchainCommandCall struct {
	command string
	args    []string
}

func (commands *fakeToolchainCommands) RunWithLogger(_ context.Context, _ *telemetry.Logger, command string, args ...string) error {
	commands.calls = append(commands.calls, toolchainCommandCall{command: command, args: append([]string{}, args...)})
	if err, ok := commands.errs[toolchainCommandKey(command, args...)]; ok {
		return err
	}
	return nil
}

type fakeToolchainLookup struct {
	commands map[string]bool
}

func (lookup fakeToolchainLookup) HasCommand(name string) bool {
	return lookup.commands[name]
}

type fakeToolchainDownloader struct {
	path string
	url  string
	err  error
}

func (downloader *fakeToolchainDownloader) DownloadToTempFile(_ context.Context, _ *telemetry.Logger, fileURL string) (string, error) {
	downloader.url = fileURL
	if downloader.err != nil {
		return "", downloader.err
	}
	return downloader.path, nil
}

type fakeToolchainFiles struct {
	removeCalls []string
}

func (files *fakeToolchainFiles) Remove(path string) error {
	files.removeCalls = append(files.removeCalls, path)
	return nil
}

type fakeToolchainEnv struct {
	values map[string]string
}

func (env *fakeToolchainEnv) Getenv(key string) string {
	return env.values[key]
}

func (env *fakeToolchainEnv) Setenv(key string, value string) error {
	env.values[key] = value
	return nil
}

type fakeToolchainCatalog struct {
	packageConfig *catalog.PackageConfig
}

func (provider fakeToolchainCatalog) PackageConfig() *catalog.PackageConfig {
	return provider.packageConfig
}

func TestEnsureBootstrapPathEntriesPrependsUniquePaths(t *testing.T) {
	t.Parallel()

	env := &fakeToolchainEnv{values: map[string]string{
		"HOME": "/tmp/home",
		"PATH": "/usr/local/bin:/bin",
	}}
	installer := New(Deps{
		Env: env,
	})

	installer.EnsureBootstrapPathEntries()

	want := "/opt/homebrew/bin:/tmp/home/.local/bin:/tmp/home/.cargo/bin:/tmp/home/.local/go/bin:/usr/local/bin:/bin"
	if got := env.values["PATH"]; got != want {
		t.Fatalf("PATH = %q, want %q", got, want)
	}
}

func TestInstallRustupIfNeededDownloadsAndRunsScript(t *testing.T) {
	t.Parallel()

	commands := &fakeToolchainCommands{}
	downloader := &fakeToolchainDownloader{path: "/tmp/rustup.sh"}
	files := &fakeToolchainFiles{}
	env := &fakeToolchainEnv{values: map[string]string{
		"HOME": "/tmp/home",
		"PATH": "/bin",
	}}
	installer := New(Deps{
		Commands:   commands,
		Lookup:     fakeToolchainLookup{commands: map[string]bool{}},
		Downloader: downloader,
		Files:      files,
		Env:        env,
	})

	err := installer.InstallRustupIfNeeded(context.Background(), nil)
	if err != nil {
		t.Fatalf("InstallRustupIfNeeded() returned error: %v", err)
	}
	if downloader.url != "https://sh.rustup.rs" {
		t.Fatalf("download URL = %q", downloader.url)
	}
	wantCalls := []toolchainCommandCall{
		{command: "sh", args: []string{"/tmp/rustup.sh", "-y"}},
	}
	if !reflect.DeepEqual(commands.calls, wantCalls) {
		t.Fatalf("command calls = %#v, want %#v", commands.calls, wantCalls)
	}
	if !reflect.DeepEqual(files.removeCalls, []string{"/tmp/rustup.sh"}) {
		t.Fatalf("remove calls = %#v, want rustup cleanup", files.removeCalls)
	}
	if !strings.Contains(env.values["PATH"], "/tmp/home/.cargo/bin") {
		t.Fatalf("PATH = %q, want cargo bin entry", env.values["PATH"])
	}
}

func TestInstallGoToolsIfNeededSkipsInstalledAndParsesCargoGitFeatures(t *testing.T) {
	t.Parallel()

	commands := &fakeToolchainCommands{}
	installer := New(Deps{
		Commands: commands,
		Lookup: fakeToolchainLookup{commands: map[string]bool{
			"go":       true,
			"cargo":    true,
			"existing": true,
		}},
		Catalog: fakeToolchainCatalog{
			packageConfig: &catalog.PackageConfig{
				GoPackages: map[string]string{
					"existing": "example.com/existing@latest",
					"missing":  "example.com/missing@latest",
				},
				CargoPackages: []string{"cargo-missing"},
				CargoGitPackages: map[string]string{
					"cargo-git": "https://example.com/repo|feat-a,feat-b",
				},
			},
		},
	})

	err := installer.InstallGoToolsIfNeeded(context.Background(), nil)
	if err != nil {
		t.Fatalf("InstallGoToolsIfNeeded() returned error: %v", err)
	}

	// installCargoToolsIfNeeded resolves cargo through tools.CargoExecutable, so
	// the expected command name is whatever that resolver returns on this host.
	cargo := tools.CargoExecutable()
	wantCalls := []toolchainCommandCall{
		{command: "go", args: []string{"install", "example.com/missing@latest"}},
		{command: cargo, args: []string{"install", "cargo-missing"}},
		{command: cargo, args: []string{"install", "--git", "https://example.com/repo", "--features", "feat-a,feat-b"}},
	}
	if !reflect.DeepEqual(commands.calls, wantCalls) {
		t.Fatalf("command calls = %#v, want %#v", commands.calls, wantCalls)
	}
}

func toolchainCommandKey(command string, args ...string) string {
	return strings.Join(append([]string{command}, args...), "\x00")
}
