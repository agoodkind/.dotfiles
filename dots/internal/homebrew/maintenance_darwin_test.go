package homebrew_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"goodkind.io/.dotfiles/internal/homebrew"
	"goodkind.io/.dotfiles/internal/telemetry"
)

type formulaData struct {
	ClassName string
	SourceURL string
	SHA256    string
	Version   string
	Fail      bool
}

func TestMaintenanceContinuesAfterPackageFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := strings.TrimSpace(runCommand(t, ctx, "", "brew", "--repository"))
	prefix := filepath.Join(root, "brew")
	runCommand(t, ctx, "", "git", "clone", "--shared", source, prefix)
	runCommand(t, ctx, "", "cp", "-R", filepath.Join(source, "Library", "Homebrew", "vendor", "portable-ruby"), filepath.Join(prefix, "Library", "Homebrew", "vendor", "portable-ruby"))
	if err := os.Chmod(filepath.Join(prefix, "bin", "brew"), 0o555); err != nil {
		t.Fatal(err)
	}

	for key, value := range map[string]string{
		"HOME":                                   filepath.Join(root, "home"),
		"PATH":                                   filepath.Join(prefix, "bin") + ":/usr/bin:/bin:/usr/sbin:/sbin",
		"HOMEBREW_CELLAR":                        filepath.Join(prefix, "Cellar"),
		"HOMEBREW_REPOSITORY":                    prefix,
		"HOMEBREW_CACHE":                         filepath.Join(root, "cache"),
		"HOMEBREW_LOGS":                          filepath.Join(root, "logs"),
		"HOMEBREW_TEMP":                          filepath.Join(root, "tmp"),
		"HOMEBREW_BREW_GIT_REMOTE":               source,
		"HOMEBREW_NO_AUTO_UPDATE":                "1",
		"HOMEBREW_NO_INSTALL_CLEANUP":            "1",
		"HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK": "1",
		"HOMEBREW_NO_ANALYTICS":                  "1",
		"HOMEBREW_NO_ENV_HINTS":                  "1",
		"HOMEBREW_NO_BOOTSNAP":                   "1",
		"HOMEBREW_NO_COLOR":                      "1",
	} {
		t.Setenv(key, value)
	}
	for argument, expected := range map[string]string{
		"--prefix":     prefix,
		"--repository": prefix,
		"--cellar":     filepath.Join(prefix, "Cellar"),
	} {
		if got := strings.TrimSpace(runCommand(t, ctx, "", "brew", argument)); got != expected {
			t.Fatalf("brew %s = %q, want isolated path %q", argument, got, expected)
		}
	}

	tap := filepath.Join(root, "tap")
	if err := os.MkdirAll(filepath.Join(tap, "Formula"), 0o755); err != nil {
		t.Fatal(err)
	}
	runCommand(t, ctx, tap, "git", "init", "-b", "main")
	writeFormulae(t, root, tap, "1.0", false)
	commitTap(t, ctx, tap)
	runCommand(t, ctx, "", "brew", "tap", "test/maintenance", tap)
	runCommand(t, ctx, "", "brew", "trust", "test/maintenance")
	for _, name := range []string{"maintenance-broken", "maintenance-later", "maintenance-pinned"} {
		runCommand(t, ctx, "", "brew", "install", "--no-ask", "--build-from-source", "test/maintenance/"+name)
	}
	runCommand(t, ctx, "", "brew", "pin", "test/maintenance/maintenance-pinned")
	writeFormulae(t, root, tap, "2.0", true)
	commitTap(t, ctx, tap)

	logPath := filepath.Join(root, "maintenance.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := logger.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	ctx = telemetry.WithRun(ctx)
	result, err := homebrew.Run(ctx, logger, logPath)
	if err != nil {
		t.Fatalf("maintenance did not complete: %v", err)
	}
	if result.Failures != 1 {
		t.Fatalf("maintenance failures = %d, want the broken package failure", result.Failures)
	}
	t.Logf("The maintenance failure count is %d.", result.Failures)
	assertArtifact(t, prefix, "maintenance-later", "2.0")
	assertArtifact(t, prefix, "maintenance-pinned", "1.0")
	queuePath := filepath.Join(root, "home", ".cache", "dotfiles", "notifications")
	queue, err := os.ReadFile(queuePath)
	if err != nil {
		t.Fatal(err)
	}
	entries := strings.Split(strings.TrimSpace(string(queue)), "\n")
	if len(entries) != 1 {
		t.Fatalf("notification entries = %d, want one package error: %s", len(entries), queue)
	}
	fields := strings.SplitN(entries[0], "|", 5)
	if len(fields) != 5 || fields[1] != "error" || fields[2] != logPath || fields[3] != telemetry.RunID(ctx) {
		t.Fatalf("notification metadata does not identify the maintenance error: %s", entries[0])
	}
	if !strings.Contains(fields[4], "test/maintenance/maintenance-broken") || !strings.Contains(fields[4], "maintenance fixture upgrade failed") {
		t.Fatalf("notification does not include the package diagnostic: %s", fields[4])
	}
	t.Logf("The notifier records %q.", fields[4])

	canceled, stop := context.WithCancel(ctx)
	stop()
	_, err = homebrew.Run(canceled, logger, logPath)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled maintenance error = %v, want context cancellation", err)
	}
	t.Logf("Canceled maintenance returned %v.", err)
}

func writeFormulae(t *testing.T, root, tap, version string, fail bool) {
	t.Helper()
	archive := sourceArchive(t, root, version)
	digest := sha256.Sum256(archive)
	formula, err := template.ParseFiles("testdata/formula.rb")
	if err != nil {
		t.Fatal(err)
	}
	for name, className := range map[string]string{
		"maintenance-broken": "MaintenanceBroken",
		"maintenance-later":  "MaintenanceLater",
		"maintenance-pinned": "MaintenancePinned",
	} {
		var output bytes.Buffer
		data := formulaData{ClassName: className, SourceURL: "file://" + filepath.Join(root, "source-"+version+".tar.gz"), SHA256: fmt.Sprintf("%x", digest), Version: version, Fail: fail && name == "maintenance-broken"}
		if err := formula.Execute(&output, data); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tap, "Formula", name+".rb"), output.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func sourceArchive(t *testing.T, root, version string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{Name: "payload.txt", Mode: 0o644, Size: int64(len(version))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(version)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source-"+version+".tar.gz"), buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func commitTap(t *testing.T, ctx context.Context, tap string) {
	t.Helper()
	runCommand(t, ctx, tap, "git", "add", "Formula")
	runCommand(t, ctx, tap, "git", "-c", "commit.gpgsign=false", "-c", "user.name=Maintenance test", "-c", "user.email=maintenance@example.com", "commit", "-m", "Update test formulae")
}

func assertArtifact(t *testing.T, prefix, name, version string) {
	t.Helper()
	artifact, err := os.ReadFile(filepath.Join(prefix, "Cellar", name, version, "share", name, "payload.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(artifact) != version {
		t.Fatalf("%s installed artifact = %q, want %q", name, artifact, version)
	}
	t.Logf("The %s artifact contains version %s.", name, version)
}

func runCommand(t *testing.T, ctx context.Context, directory, command string, arguments ...string) string {
	t.Helper()
	process := exec.CommandContext(ctx, command, arguments...)
	process.Dir = directory
	output, err := process.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", command, arguments, err, output)
	}
	return string(output)
}
