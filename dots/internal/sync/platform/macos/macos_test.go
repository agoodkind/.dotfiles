package macos

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/telemetry"
)

type commandCall struct {
	command string
	args    []string
}

type fakeCommands struct {
	runCalls []commandCall
	runErrs  map[string]error
	succeeds map[string]bool
}

func (commands *fakeCommands) RunWithLogger(_ context.Context, _ *telemetry.Logger, command string, args ...string) error {
	commands.runCalls = append(commands.runCalls, commandCall{command: command, args: append([]string{}, args...)})
	if err, ok := commands.runErrs[commandKey(command, args...)]; ok {
		return err
	}
	return nil
}

func (commands *fakeCommands) CommandSucceeds(_ context.Context, command string, args ...string) bool {
	return commands.succeeds[commandKey(command, args...)]
}

type fakeLookup struct {
	commands map[string]bool
}

func (lookup fakeLookup) HasCommand(name string) bool {
	return lookup.commands[name]
}

type fakeDownloader struct {
	path string
	url  string
	err  error
}

func (downloader *fakeDownloader) DownloadToTempFile(_ context.Context, _ *telemetry.Logger, fileURL string) (string, error) {
	downloader.url = fileURL
	if downloader.err != nil {
		return "", downloader.err
	}
	return downloader.path, nil
}

type fakeFiles struct {
	readFiles   map[string][]byte
	readErrs    map[string]error
	existing    map[string]bool
	removeCalls []string
	mkdirCalls  []string
}

func (files *fakeFiles) MkdirAll(path string, _ os.FileMode) error {
	files.mkdirCalls = append(files.mkdirCalls, path)
	return nil
}

func (files *fakeFiles) ReadFile(path string) ([]byte, error) {
	if err, ok := files.readErrs[path]; ok {
		return nil, err
	}
	content, ok := files.readFiles[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return content, nil
}

func (files *fakeFiles) Remove(path string) error {
	files.removeCalls = append(files.removeCalls, path)
	return nil
}

func (files *fakeFiles) PathExists(path string) bool {
	return files.existing[path]
}

type fakeEnv struct {
	values map[string]string
}

func (env fakeEnv) Getenv(key string) string {
	return env.values[key]
}

type fakeCatalog struct {
	packageConfig  *catalog.PackageConfig
	macPatchConfig *catalog.MacPatchConfig
}

func (catalog fakeCatalog) PackageConfig() *catalog.PackageConfig {
	return catalog.packageConfig
}

func (catalog fakeCatalog) MacPatchConfig() *catalog.MacPatchConfig {
	return catalog.macPatchConfig
}

type fakePaths struct{}

func (fakePaths) ResolveConfigPath(value string, _ string) string {
	return value
}

type fakeSudo struct {
	allowed bool
}

func (sudo fakeSudo) HasSudoAccess(context.Context, *telemetry.Logger) bool {
	return sudo.allowed
}

func TestMacCaskAppPathsChecksAppBundleSuffix(t *testing.T) {
	t.Parallel()

	installer := &Installer{}
	home := filepath.Join(string(filepath.Separator), "tmp", "home")

	got := installer.macCaskAppPaths("Visual Studio Code", home)
	want := []string{
		filepath.Join(string(filepath.Separator), "Applications", "Visual Studio Code"),
		filepath.Join(string(filepath.Separator), "Applications", "Visual Studio Code.app"),
		filepath.Join(home, "Applications", "Visual Studio Code"),
		filepath.Join(home, "Applications", "Visual Studio Code.app"),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("macCaskAppPaths() = %#v, want %#v", got, want)
	}
}

func TestMacCaskAppPathsDoesNotDuplicateAppSuffix(t *testing.T) {
	t.Parallel()

	installer := &Installer{}
	home := filepath.Join(string(filepath.Separator), "tmp", "home")

	got := installer.macCaskAppPaths("Ghostty.app", home)
	want := []string{
		filepath.Join(string(filepath.Separator), "Applications", "Ghostty.app"),
		filepath.Join(home, "Applications", "Ghostty.app"),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("macCaskAppPaths() = %#v, want %#v", got, want)
	}
}

func TestMacCaskAppExistsFindsHomeAppBundle(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	appPath := filepath.Join(home, "Applications", "Ghostty.app")
	installer := &Installer{
		deps: Deps{
			Files: &fakeFiles{
				existing: map[string]bool{appPath: true},
			},
		},
	}

	if !installer.macCaskAppExists("Ghostty", home) {
		t.Fatal("macCaskAppExists() = false, want true")
	}
}

func TestEnsureHomebrewInstalledDownloadsAndRunsInstaller(t *testing.T) {
	t.Parallel()

	commands := &fakeCommands{}
	downloader := &fakeDownloader{path: "/tmp/homebrew-install.sh"}
	files := &fakeFiles{}
	installer := &Installer{
		deps: Deps{
			Commands: commands,
			Lookup: fakeLookup{commands: map[string]bool{
				"brew": false,
			}},
			Download: downloader,
			Files:    files,
		},
	}

	err := installer.ensureHomebrewInstalled(context.Background(), nil)
	if err != nil {
		t.Fatalf("ensureHomebrewInstalled() returned error: %v", err)
	}
	if downloader.url != "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh" {
		t.Fatalf("download URL = %q", downloader.url)
	}
	if len(commands.runCalls) != 1 {
		t.Fatalf("run call count = %d, want 1", len(commands.runCalls))
	}
	if commands.runCalls[0].command != "bash" || !reflect.DeepEqual(commands.runCalls[0].args, []string{"/tmp/homebrew-install.sh"}) {
		t.Fatalf("run call = %#v, want bash installer", commands.runCalls[0])
	}
	if !reflect.DeepEqual(files.removeCalls, []string{"/tmp/homebrew-install.sh"}) {
		t.Fatalf("remove calls = %#v, want installer cleanup", files.removeCalls)
	}
}

func TestInstallMacPackagesStrictModeReturnsUpdateError(t *testing.T) {
	t.Parallel()

	commands := &fakeCommands{
		runErrs: map[string]error{
			commandKey("brew", "update", "--quiet"): errors.New("brew update failed"),
		},
	}
	installer := &Installer{
		deps: Deps{
			Commands: commands,
			Lookup: fakeLookup{commands: map[string]bool{
				"brew": true,
			}},
			Catalog: fakeCatalog{
				packageConfig: &catalog.PackageConfig{},
			},
		},
	}

	err := installer.installMacPackages(context.Background(), true, nil)
	if err == nil || !strings.Contains(err.Error(), "running brew update") {
		t.Fatalf("installMacPackages() error = %v, want brew update error", err)
	}
}

func TestInstallMacPackagesLenientModeContinuesAfterUpdateError(t *testing.T) {
	t.Parallel()

	commands := &fakeCommands{
		runErrs: map[string]error{
			commandKey("brew", "update", "--quiet"): errors.New("brew update failed"),
		},
	}
	installer := &Installer{
		deps: Deps{
			Commands: commands,
			Lookup: fakeLookup{commands: map[string]bool{
				"brew": true,
			}},
			Catalog: fakeCatalog{
				packageConfig: &catalog.PackageConfig{},
			},
		},
	}

	if err := installer.installMacPackages(context.Background(), false, nil); err != nil {
		t.Fatalf("installMacPackages() returned error: %v", err)
	}
}

func TestEnsureMacPatchSkipsWithoutSudo(t *testing.T) {
	t.Parallel()

	commands := &fakeCommands{}
	files := &fakeFiles{
		readFiles: map[string][]byte{
			"/etc/zprofile": []byte("plain"),
			"/etc/zshrc":    []byte("plain"),
		},
	}
	installer := &Installer{
		deps: Deps{
			Commands: commands,
			Files:    files,
			Env: fakeEnv{values: map[string]string{
				"DOTDOTFILES": "/repo",
			}},
			Catalog: fakeCatalog{
				macPatchConfig: &catalog.MacPatchConfig{
					Enabled:      true,
					PatchScript:  "/repo/bash/setup/platform/patch-etc-zsh.bash",
					ZProfilePath: "/etc/zprofile",
					ZshrcPath:    "/etc/zshrc",
					Sentinel:     "# patch",
				},
			},
			Paths: fakePaths{},
			Sudo:  fakeSudo{allowed: false},
		},
	}

	if err := installer.ensureMacPatchIfNeeded(context.Background(), nil); err != nil {
		t.Fatalf("ensureMacPatchIfNeeded() returned error: %v", err)
	}
	if len(commands.runCalls) != 0 {
		t.Fatalf("run calls = %#v, want none", commands.runCalls)
	}
}

func TestEnsureMacPatchSkipsWhenSentinelPresent(t *testing.T) {
	t.Parallel()

	commands := &fakeCommands{}
	files := &fakeFiles{
		readFiles: map[string][]byte{
			"/etc/zprofile": []byte("# patch"),
			"/etc/zshrc":    []byte("# patch"),
		},
	}
	installer := &Installer{
		deps: Deps{
			Commands: commands,
			Files:    files,
			Env: fakeEnv{values: map[string]string{
				"DOTDOTFILES": "/repo",
			}},
			Catalog: fakeCatalog{
				macPatchConfig: &catalog.MacPatchConfig{
					Enabled:      true,
					PatchScript:  "/repo/bash/setup/platform/patch-etc-zsh.bash",
					ZProfilePath: "/etc/zprofile",
					ZshrcPath:    "/etc/zshrc",
					Sentinel:     "# patch",
				},
			},
			Paths: fakePaths{},
			Sudo:  fakeSudo{allowed: true},
		},
	}

	if err := installer.ensureMacPatchIfNeeded(context.Background(), nil); err != nil {
		t.Fatalf("ensureMacPatchIfNeeded() returned error: %v", err)
	}
	if len(commands.runCalls) != 0 {
		t.Fatalf("run calls = %#v, want none", commands.runCalls)
	}
}

func commandKey(command string, args ...string) string {
	return strings.Join(append([]string{command}, args...), "\x00")
}
