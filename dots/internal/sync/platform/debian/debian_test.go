package debian

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/sync/platform"
	"goodkind.io/.dotfiles/internal/telemetry"
)

type fakeCommandRunner struct {
	runCalls []commandCall
	runErrs  map[string]error
	outputs  map[string]fakeCommandOutput
}

type fakeCommandOutput struct {
	value string
	err   error
}

func (runner *fakeCommandRunner) RunWithLogger(_ context.Context, _ *telemetry.Logger, command string, args ...string) error {
	runner.runCalls = append(runner.runCalls, commandCall{command: command, args: append([]string{}, args...)})
	if err, ok := runner.runErrs[debianCommandKey(command, args...)]; ok {
		return err
	}
	return nil
}

func (runner *fakeCommandRunner) OutputWithLoggerAndEnv(_ context.Context, _ *telemetry.Logger, _ []string, command string, args ...string) (string, error) {
	output, ok := runner.outputs[debianCommandKey(command, args...)]
	if !ok {
		return "", errors.New("missing output")
	}
	return output.value, output.err
}

type fakeCommandLookup struct {
	commands map[string]bool
}

func (lookup fakeCommandLookup) HasCommand(name string) bool {
	return lookup.commands[name]
}

type fakeCatalogProvider struct {
	packageConfig *catalog.PackageConfig
}

func (provider fakeCatalogProvider) PackageConfig() *catalog.PackageConfig {
	return provider.packageConfig
}

type fakePrivilegedRunner struct {
	calls     []commandCall
	errs      map[string]error
	installed map[string]string
}

func (runner *fakePrivilegedRunner) Run(_ context.Context, _ *telemetry.Logger, command string, args ...string) error {
	runner.calls = append(runner.calls, commandCall{command: command, args: append([]string{}, args...)})
	if command == "install" && len(args) >= 4 && args[0] == "-m" {
		src := args[2]
		dest := args[3]
		if data, err := os.ReadFile(src); err == nil {
			if runner.installed == nil {
				runner.installed = map[string]string{}
			}
			runner.installed[dest] = string(data)
		}
	}
	if err, ok := runner.errs[debianCommandKey(command, args...)]; ok {
		return err
	}
	return nil
}

type fakeAptRepos struct {
	codename  string
	releases  map[string]bool
	downloads map[string][]byte
}

func (repos fakeAptRepos) HasRelease(_ context.Context, baseURL, suite string) bool {
	return repos.releases[baseURL+"\x00"+suite]
}

func (repos fakeAptRepos) Download(_ context.Context, fileURL string) ([]byte, error) {
	data, ok := repos.downloads[fileURL]
	if !ok {
		return nil, errors.New("missing download")
	}
	return data, nil
}

func (repos fakeAptRepos) Codename() string {
	return repos.codename
}

func ooklaAptRepo() catalog.AptRepo {
	return catalog.AptRepo{
		ID:         "ookla-speedtest",
		GPGURL:     "https://packagecloud.io/ookla/speedtest-cli/gpgkey",
		UbuntuBase: "https://packagecloud.io/ookla/speedtest-cli/ubuntu",
		DebianBase: "https://packagecloud.io/ookla/speedtest-cli/debian",
		Keyring:    "/etc/apt/keyrings/ookla_speedtest-cli-archive-keyring.gpg",
		ListPath:   "/etc/apt/sources.list.d/ookla_speedtest-cli.list",
		Component:  "main",
	}
}

type commandCall struct {
	command string
	args    []string
}

type fakePPAChecker struct {
	publishes map[string]bool
}

func (checker fakePPAChecker) PublishesForCurrentRelease(_ context.Context, ppa string) bool {
	return checker.publishes[ppa]
}

func TestAptPackageNameMappings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "ack", want: "ack-grep"},
		{input: "fd", want: "fd-find"},
		{input: "rg", want: "ripgrep"},
		{input: "openssh", want: "openssh-client openssh-server"},
		{input: "git", want: "git"},
	}

	for _, test := range tests {
		if got := aptPackageName(test.input); got != test.want {
			t.Fatalf("aptPackageName(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestInstallUbuntuPPAsRunsExpectedCommands(t *testing.T) {
	t.Parallel()

	privileged := &fakePrivilegedRunner{}
	installer := &Installer{
		deps: Deps{
			Privileged: privileged,
			PPAChecker: fakePPAChecker{publishes: map[string]bool{"ppa:example/test": true}},
		},
	}

	installer.installUbuntuPPAs(context.Background(), platform.Host{
		GOOS:         platform.GOOSLinux,
		Distribution: platform.DistributionUbuntu,
	}, &catalog.PackageConfig{
		UbuntuPPAs: []string{"ppa:example/test"},
	}, nil)

	want := []commandCall{
		{command: "apt-get", args: []string{"install", "-y", "-qq", "software-properties-common"}},
		{command: "add-apt-repository", args: []string{"-y", "ppa:example/test"}},
		{command: "apt-get", args: []string{"update", "-qq"}},
	}
	if !reflect.DeepEqual(privileged.calls, want) {
		t.Fatalf("privileged calls = %#v, want %#v", privileged.calls, want)
	}
}

func TestInstallUbuntuPPAsSkipsUnpublishedPPA(t *testing.T) {
	t.Parallel()

	privileged := &fakePrivilegedRunner{}
	installer := &Installer{
		deps: Deps{
			Privileged: privileged,
			PPAChecker: fakePPAChecker{publishes: map[string]bool{"ppa:has/release": true}},
		},
	}

	installer.installUbuntuPPAs(context.Background(), platform.Host{
		GOOS:         platform.GOOSLinux,
		Distribution: platform.DistributionUbuntu,
	}, &catalog.PackageConfig{
		UbuntuPPAs: []string{"ppa:has/release", "ppa:no/release"},
	}, nil)

	want := []commandCall{
		{command: "apt-get", args: []string{"install", "-y", "-qq", "software-properties-common"}},
		{command: "add-apt-repository", args: []string{"-y", "ppa:has/release"}},
		{command: "apt-get", args: []string{"update", "-qq"}},
	}
	if !reflect.DeepEqual(privileged.calls, want) {
		t.Fatalf("privileged calls = %#v, want %#v (unpublished PPA must be skipped without poisoning apt)", privileged.calls, want)
	}
}

func TestParsePPAOwnerName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input     string
		wantOwner string
		wantName  string
		wantOK    bool
	}{
		{input: "ppa:fujiapple/trippy", wantOwner: "fujiapple", wantName: "trippy", wantOK: true},
		{input: "zhangsongcui3371/fastfetch", wantOwner: "zhangsongcui3371", wantName: "fastfetch", wantOK: true},
		{input: "ppa:noslash", wantOwner: "", wantName: "", wantOK: false},
		{input: "ppa:/missing-owner", wantOwner: "", wantName: "", wantOK: false},
		{input: "ppa:owner/", wantOwner: "", wantName: "", wantOK: false},
	}
	for _, test := range tests {
		owner, name, ok := parsePPAOwnerName(test.input)
		if owner != test.wantOwner || name != test.wantName || ok != test.wantOK {
			t.Fatalf("parsePPAOwnerName(%q) = (%q, %q, %v), want (%q, %q, %v)", test.input, owner, name, ok, test.wantOwner, test.wantName, test.wantOK)
		}
	}
}

func TestInstallSnapPackageUsesSnapCommand(t *testing.T) {
	t.Parallel()

	commands := &fakeCommandRunner{
		runErrs: map[string]error{
			debianCommandKey("snap", "list", "nvim"): errors.New("missing"),
		},
		outputs: map[string]fakeCommandOutput{
			debianCommandKey("snap", "info", "nvim"): {value: "classic\nconfinement"},
		},
	}
	privileged := &fakePrivilegedRunner{}
	installer := &Installer{
		deps: Deps{
			Commands:   commands,
			Privileged: privileged,
		},
	}

	err := installer.installSnapPackage(context.Background(), "nvim", nil)
	if err != nil {
		t.Fatalf("installSnapPackage() returned error: %v", err)
	}
	if len(privileged.calls) != 1 {
		t.Fatalf("privileged call count = %d, want 1", len(privileged.calls))
	}
	if privileged.calls[0].command != "snap" {
		t.Fatalf("privileged command = %q, want snap", privileged.calls[0].command)
	}
	if !reflect.DeepEqual(privileged.calls[0].args, []string{"install", "--classic", "nvim"}) {
		t.Fatalf("privileged args = %#v, want snap install --classic nvim", privileged.calls[0].args)
	}
}

func TestSnapPackageNameMapsNeovim(t *testing.T) {
	t.Parallel()

	if got := snapPackageName("neovim"); got != "nvim" {
		t.Fatalf("snapPackageName(neovim) = %q, want nvim", got)
	}
}

func debianCommandKey(command string, args ...string) string {
	return strings.Join(append([]string{command}, args...), "\x00")
}

func TestInstallDebianPackagesAddsOoklaRepoAndSpeedtest(t *testing.T) {
	t.Parallel()

	privileged := &fakePrivilegedRunner{}
	commands := &fakeCommandRunner{}
	installer := &Installer{
		deps: Deps{
			Commands: commands,
			Lookup:   fakeCommandLookup{commands: map[string]bool{"apt-get": true}},
			Catalog: fakeCatalogProvider{packageConfig: &catalog.PackageConfig{
				AptSpecific: []string{"curl", "speedtest"},
				AptRepos:    []catalog.AptRepo{ooklaAptRepo()},
			}},
			Privileged: privileged,
			AptRepos: fakeAptRepos{
				codename: "noble",
				releases: map[string]bool{
					"https://packagecloud.io/ookla/speedtest-cli/ubuntu\x00noble": true,
				},
				downloads: map[string][]byte{
					"https://packagecloud.io/ookla/speedtest-cli/gpgkey": []byte("gpg-key"),
				},
			},
		},
	}

	if err := installer.installDebianPackages(context.Background(), platform.Host{
		GOOS:         platform.GOOSLinux,
		Distribution: platform.DistributionUbuntu,
	}, nil); err != nil {
		t.Fatalf("installDebianPackages() returned error: %v", err)
	}

	if !debianContainsCommand(privileged.calls, "apt-get", []string{"remove", "-y", "-qq", "speedtest-cli"}) {
		t.Fatal("expected apt-get remove speedtest-cli")
	}
	if _, ok := privileged.installed["/etc/apt/keyrings/ookla_speedtest-cli-archive-keyring.gpg"]; !ok {
		t.Fatal("expected ookla keyring install")
	}
	listBody, ok := privileged.installed["/etc/apt/sources.list.d/ookla_speedtest-cli.list"]
	if !ok {
		t.Fatal("expected ookla sources.list install")
	}
	if !strings.Contains(listBody, "noble") {
		t.Fatalf("sources list = %q, want noble suite", listBody)
	}
	if !strings.Contains(listBody, "signed-by=/etc/apt/keyrings/ookla_speedtest-cli-archive-keyring.gpg") {
		t.Fatalf("sources list = %q, want signed-by keyring", listBody)
	}

	installCall := debianAptGetInstallArgs(privileged.calls)
	if !containsString(installCall, "speedtest") || !containsString(installCall, "curl") {
		t.Fatalf("apt-get install args = %#v, want curl and speedtest", installCall)
	}
}

func TestInstallDebianPackagesSkipsSpeedtestWhenRepoUnpublished(t *testing.T) {
	t.Parallel()

	privileged := &fakePrivilegedRunner{}
	installer := &Installer{
		deps: Deps{
			Commands: &fakeCommandRunner{},
			Lookup:   fakeCommandLookup{commands: map[string]bool{"apt-get": true}},
			Catalog: fakeCatalogProvider{packageConfig: &catalog.PackageConfig{
				AptSpecific: []string{"curl", "speedtest"},
				AptRepos:    []catalog.AptRepo{ooklaAptRepo()},
			}},
			Privileged: privileged,
			AptRepos: fakeAptRepos{
				codename: "resolute",
				releases: map[string]bool{
					"https://packagecloud.io/ookla/speedtest-cli/ubuntu\x00resolute": false,
					"https://packagecloud.io/ookla/speedtest-cli/ubuntu\x00jammy":    false,
				},
				downloads: map[string][]byte{
					"https://packagecloud.io/ookla/speedtest-cli/gpgkey": []byte("gpg-key"),
				},
			},
		},
	}

	if err := installer.installDebianPackages(context.Background(), platform.Host{
		GOOS:         platform.GOOSLinux,
		Distribution: platform.DistributionUbuntu,
	}, nil); err != nil {
		t.Fatalf("installDebianPackages() returned error: %v", err)
	}

	if debianContainsCommand(privileged.calls, "apt-get", []string{"remove", "-y", "-qq", "speedtest-cli"}) {
		t.Fatal("did not expect apt-get remove speedtest-cli when repo is skipped")
	}
	if _, ok := privileged.installed["/etc/apt/sources.list.d/ookla_speedtest-cli.list"]; ok {
		t.Fatal("did not expect ookla sources.list when unpublished")
	}
	installCall := debianAptGetInstallArgs(privileged.calls)
	if containsString(installCall, "speedtest") {
		t.Fatalf("apt-get install args = %#v, did not want speedtest", installCall)
	}
	if !containsString(installCall, "curl") {
		t.Fatalf("apt-get install args = %#v, want curl", installCall)
	}
}

func TestSelectAptRepoSuiteFallsBackToJammy(t *testing.T) {
	t.Parallel()

	installer := &Installer{
		deps: Deps{
			AptRepos: fakeAptRepos{
				codename: "noble",
				releases: map[string]bool{
					"https://packagecloud.io/ookla/speedtest-cli/ubuntu\x00noble": false,
					"https://packagecloud.io/ookla/speedtest-cli/ubuntu\x00jammy": true,
				},
			},
		},
	}

	suite, ok := installer.selectAptRepoSuite(
		context.Background(),
		platform.Host{Distribution: platform.DistributionUbuntu},
		"https://packagecloud.io/ookla/speedtest-cli/ubuntu",
	)
	if !ok || suite != "jammy" {
		t.Fatalf("suite = %q ok = %v, want jammy true", suite, ok)
	}
}

func debianContainsCommand(calls []commandCall, command string, args []string) bool {
	for _, call := range calls {
		if call.command != command {
			continue
		}
		if reflect.DeepEqual(call.args, args) {
			return true
		}
	}
	return false
}

func debianAptGetInstallArgs(calls []commandCall) []string {
	for _, call := range calls {
		if call.command != "apt-get" || len(call.args) == 0 || call.args[0] != "install" {
			continue
		}
		if len(call.args) >= 4 && call.args[1] == "-y" && call.args[2] == "-qq" && call.args[3] == "gnupg" {
			continue
		}
		return call.args
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
