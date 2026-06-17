// Command fresh-macos-bootstrap-smoke runs a fresh-host bootstrap smoke test
// on macOS. In direct mode (default, used by CI) it creates a temp HOME and
// restricts PATH so install.sh must bootstrap Homebrew and Go from scratch.
// In --tart mode it runs the assertions inside a pristine Tart macOS VM.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"goodkind.io/.dotfiles/internal/freshsmoke"
)

const (
	defaultTartImage = "ghcr.io/cirruslabs/macos-tahoe-base:latest"
	defaultTimeout   = 45 * time.Minute
)

type options struct {
	repoRoot        string
	tart            bool
	image           string
	inVM            bool
	githubTokenFile string
}

func main() {
	slog.Info("fresh-macos-bootstrap-smoke starting")
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fresh-macos-bootstrap: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	opts := parseOptions()
	switch {
	case opts.inVM:
		return runInsideVM(ctx, opts.repoRoot, opts.githubTokenFile)
	case opts.tart:
		return runWithTart(ctx, opts)
	default:
		return runDirect(ctx, opts.repoRoot)
	}
}

func parseOptions() options {
	var opts options
	flag.StringVar(&opts.repoRoot, "repo-root", "", "repository root")
	flag.BoolVar(&opts.tart, "tart", false, "run inside a Tart macOS VM (requires tart CLI)")
	flag.StringVar(&opts.image, "image", freshsmoke.GetenvDefault("DOTFILES_FRESH_MACOS_IMAGE", defaultTartImage), "Tart OCI image to clone")
	flag.BoolVar(&opts.inVM, "in-vm", false, "run assertions inside the Tart VM (internal use)")
	flag.StringVar(&opts.githubTokenFile, "github-token-file", "", "file containing a GitHub token for VM release downloads")
	flag.Parse()
	return opts
}

// runDirect is the CI path: creates a temp HOME, restricts PATH to macOS
// system dirs only (strips Homebrew and any pre-installed Go) so install.sh
// must bootstrap both from scratch.
func runDirect(ctx context.Context, repoRoot string) error {
	slog.InfoContext(ctx, "running direct mode")
	repoRoot, err := resolveRepoRoot(repoRoot)
	if err != nil {
		return err
	}
	if err := freshsmoke.AssertSmokeSubmodulesPresent(repoRoot); err != nil {
		slog.ErrorContext(ctx, "checking smoke submodules", "err", err)
		return fmt.Errorf("checking smoke submodules: %w", err)
	}
	home, err := os.MkdirTemp("", "dotfiles-fresh-macos-*")
	if err != nil {
		slog.ErrorContext(ctx, "creating smoke home", "err", err)
		return fmt.Errorf("creating smoke home: %w", err)
	}
	defer os.RemoveAll(home)

	dotsBinaryDir := filepath.Join(home, ".cache", "dots", "bin")
	lockFile := filepath.Join(dotsBinaryDir, ".dots.build.lock")
	env := append(os.Environ(),
		"HOME="+home,
		"DOTDOTFILES="+repoRoot,
		"DOTFILES_LOG_LEVEL=debug",
		"DOTS_BINARY_DIR="+dotsBinaryDir,
		"DOTS_BUILD_LOCK_FILE="+lockFile,
		"GO_LOCAL_ROOT="+filepath.Join(home, ".local", "go"),
		"GOMODCACHE="+filepath.Join(home, "go", "pkg", "mod"),
		"GOCACHE="+filepath.Join(home, ".cache", "go-build"),
		// /usr/bin and /bin cover curl, tar, sw_vers, mktemp on macOS.
		// /opt/homebrew/bin and /usr/local/go/bin are intentionally excluded
		// so bootstrap-go.sh must download Go and install.sh must install Homebrew.
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
	)
	env = appendGitHubTokenEnv(env, githubTokenFromEnv())

	expectedPath := macSmokePath(home, freshsmoke.EnvValue(env, "PATH"))

	firstOutput, err := freshsmoke.RunInstall(ctx, repoRoot, env, defaultTimeout, "--strict")
	if err != nil {
		slog.ErrorContext(ctx, "first install run", "err", err)
		return fmt.Errorf("first install run: %w", err)
	}
	if err := freshsmoke.AssertStrictInstallOutput(firstOutput); err != nil {
		slog.ErrorContext(ctx, "first install strict output", "err", err)
		return fmt.Errorf("first install strict output: %w", err)
	}
	if err := freshsmoke.AssertBuildCount(firstOutput, 1); err != nil {
		slog.ErrorContext(ctx, "first install build count", "err", err)
		return fmt.Errorf("first install build count: %w", err)
	}
	if err := freshsmoke.AssertCommandsOnPath(expectedPath, "go", "rg", "zsh"); err != nil {
		slog.ErrorContext(ctx, "first install commands", "err", err)
		return fmt.Errorf("first install commands: %w", err)
	}

	secondOutput, err := freshsmoke.RunInstall(ctx, repoRoot, env, defaultTimeout, "--strict")
	if err != nil {
		slog.ErrorContext(ctx, "second install run", "err", err)
		return fmt.Errorf("second install run: %w", err)
	}
	if err := freshsmoke.AssertStrictInstallOutput(secondOutput); err != nil {
		slog.ErrorContext(ctx, "second install strict output", "err", err)
		return fmt.Errorf("second install strict output: %w", err)
	}
	if err := freshsmoke.AssertBuildCount(secondOutput, 0); err != nil {
		slog.ErrorContext(ctx, "second install build count", "err", err)
		return fmt.Errorf("second install build count: %w", err)
	}

	if err := runSharedScenarios(ctx, repoRoot, dotsBinaryDir, lockFile, env); err != nil {
		return err
	}

	fmt.Println("fresh-macos-bootstrap: passed")
	return nil
}

// runSharedScenarios exercises the bootstrap issues hit in production, after the
// baseline first and second install checks. Lock scenarios are skipped when
// flock is unavailable, which is the default on macOS.
func runSharedScenarios(ctx context.Context, repoRoot, dotsBinaryDir, lockFile string, env []string) error {
	if err := freshsmoke.StalenessSmoke(ctx, repoRoot, "config/dispatch.toml", "dots/internal/util/path.go", env, defaultTimeout); err != nil {
		slog.ErrorContext(ctx, "staleness smoke", "err", err)
		return fmt.Errorf("staleness smoke: %w", err)
	}
	if freshsmoke.HasCommandOnPath("flock", freshsmoke.EnvValue(env, "PATH")) {
		if err := freshsmoke.LockSmoke(ctx, repoRoot, dotsBinaryDir, lockFile, env, defaultTimeout); err != nil {
			slog.ErrorContext(ctx, "lock smoke", "err", err)
			return fmt.Errorf("lock smoke: %w", err)
		}
		if err := freshsmoke.LockTimeoutSmoke(ctx, repoRoot, dotsBinaryDir, lockFile, env, defaultTimeout); err != nil {
			slog.ErrorContext(ctx, "lock-timeout smoke", "err", err)
			return fmt.Errorf("lock-timeout smoke: %w", err)
		}
		if err := freshsmoke.InstallRaceSmoke(ctx, repoRoot, dotsBinaryDir, lockFile, env, defaultTimeout); err != nil {
			slog.ErrorContext(ctx, "install-race smoke", "err", err)
			return fmt.Errorf("install-race smoke: %w", err)
		}
	}
	// Runs last: it replaces GO_LOCAL_ROOT's go to force a re-download.
	if err := freshsmoke.StaleGoUpgradeSmoke(ctx, repoRoot, dotsBinaryDir, env, defaultTimeout); err != nil {
		slog.ErrorContext(ctx, "stale-go upgrade smoke", "err", err)
		return fmt.Errorf("stale-go upgrade smoke: %w", err)
	}
	return nil
}

func macSmokePath(home string, basePath string) string {
	return freshsmoke.PathWithEntries(
		basePath,
		"/opt/homebrew/bin",
		"/usr/local/bin",
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".local", "go", "bin"),
		filepath.Join(home, ".cargo", "bin"),
	)
}

// runWithTart is the local path: clones a macOS Tart VM, shares the repo and
// the smoke binary via --dir, and runs --in-vm assertions using tart exec
// (no SSH required; Cirrus Labs base images pre-install the Tart Guest Agent).
// Requires: brew install cirruslabs/cli/tart
func runWithTart(ctx context.Context, opts options) error {
	repoRoot, err := resolveRepoRoot(opts.repoRoot)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("tart"); err != nil {
		return fmt.Errorf("tart not found — install with: brew install cirruslabs/cli/tart")
	}

	vmName := "dotfiles-smoke-" + strconv.FormatInt(time.Now().Unix(), 10)
	fmt.Printf("fresh-macos-bootstrap: cloning %s → %s\n", opts.image, vmName)
	if err := streamCommand(ctx, "tart", "clone", opts.image, vmName); err != nil {
		return fmt.Errorf("cloning Tart image: %w", err)
	}
	defer func() {
		fmt.Printf("fresh-macos-bootstrap: deleting VM %s\n", vmName)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cleanupCancel()
		_ = streamCommand(cleanupCtx, "tart", "delete", vmName)
	}()

	selfPath, err := os.Executable()
	if err != nil {
		slog.ErrorContext(ctx, "resolving smoke binary path", "err", err)
		return fmt.Errorf("resolving smoke binary path: %w", err)
	}
	tokenDir := ""
	tokenPathInVM := ""
	if token := githubTokenFromEnv(); token != "" {
		tokenDir, err = os.MkdirTemp("", "dotfiles-smoke-secrets-*")
		if err != nil {
			slog.ErrorContext(ctx, "creating smoke secrets dir", "err", err)
			return fmt.Errorf("creating smoke secrets dir: %w", err)
		}
		defer os.RemoveAll(tokenDir)
		tokenPath := filepath.Join(tokenDir, "gh")
		if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
			slog.ErrorContext(ctx, "writing GitHub token file", "err", err)
			return fmt.Errorf("writing GitHub token file: %w", err)
		}
		tokenPathInVM = filepath.Join("/Volumes/My Shared Files", "smoke-auth", "gh")
	}

	// Start the VM in the background; tart run blocks until the VM shuts down.
	// context.WithoutCancel creates a context that inherits values from ctx but
	// cannot be canceled, so the VM process outlives the SSH session context.
	vmCtx := context.WithoutCancel(ctx)
	runArgs := []string{
		"run", vmName,
		"--no-graphics",
		"--dir=workspace:" + repoRoot + ":ro",
		"--dir=smoke:" + filepath.Dir(selfPath) + ":ro",
	}
	if tokenDir != "" {
		runArgs = append(runArgs, "--dir=smoke-auth:"+tokenDir+":ro")
	}
	vmCmd := exec.CommandContext(vmCtx, "tart", runArgs...)
	vmCmd.Stdout = os.Stdout
	vmCmd.Stderr = os.Stderr
	if err := vmCmd.Start(); err != nil {
		slog.ErrorContext(ctx, "starting Tart VM", "vm", vmName, "err", err)
		return fmt.Errorf("starting Tart VM: %w", err)
	}
	defer func() { _ = vmCmd.Process.Kill() }()

	fmt.Printf("fresh-macos-bootstrap: waiting for VM %s Guest Agent\n", vmName)
	if err := waitForTartExec(ctx, vmName); err != nil {
		return err
	}
	fmt.Printf("fresh-macos-bootstrap: VM %s ready, running smoke via tart exec\n", vmName)

	smokePath := "/Volumes/My Shared Files/smoke/" + filepath.Base(selfPath)
	repoInVM := "/Volumes/My Shared Files/workspace"
	execArgs := []string{"exec", vmName, smokePath, "--in-vm", "--repo-root", repoInVM}
	if tokenPathInVM != "" {
		execArgs = append(execArgs, "--github-token-file", tokenPathInVM)
	}
	if err := streamCommand(
		ctx,
		"tart", execArgs...,
	); err != nil {
		return fmt.Errorf("smoke inside Tart VM: %w", err)
	}
	return nil
}

// runInsideVM runs assertions from inside a Tart VM where the dotfiles repo
// is mounted read-only (typically at /Volumes/My Shared Files/workspace).
const vmSmokeRepoDir = "/tmp/dotfiles-smoke-repo"

// cloneWorkspaceInVM copies the read-only VM workspace mount to a writable
// path, scrubs stale in-flight git state from the developer's machine, and
// redirects origin (and each submodule's origin) to the local copy so that
// any git fetch stays on disk without needing network access or write
// permission to the Tart shared-files mount.
func cloneWorkspaceInVM(ctx context.Context, roMount string) error {
	slog.InfoContext(ctx, "copying workspace to writable path", "src", roMount, "dest", vmSmokeRepoDir)
	// rsync --safe-links skips symlinks that point outside the source tree,
	// which handles .claude/ entries that symlink to absolute host paths.
	// filepath.Clean normalises roMount so gosec G204 is satisfied.
	rsSrc := filepath.Clean(roMount) + "/"
	cmd := exec.CommandContext(ctx, "rsync", "-a", "--safe-links", rsSrc, vmSmokeRepoDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.ErrorContext(ctx, "copying workspace", "err", err)
		return fmt.Errorf("copying workspace: %w", err)
	}
	for _, name := range []string{"MERGE_HEAD", "MERGE_MSG", "CHERRY_PICK_HEAD", "REVERT_HEAD"} {
		_ = os.Remove(filepath.Join(vmSmokeRepoDir, ".git", name))
	}
	if err := streamCommand(ctx, "git", "-C", vmSmokeRepoDir, "remote", "set-url", "origin", roMount); err != nil {
		return err
	}
	libDir := filepath.Join(vmSmokeRepoDir, "lib")
	entries, err := os.ReadDir(libDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		slog.ErrorContext(ctx, "reading lib dir", "err", err)
		return fmt.Errorf("reading lib dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		smDir := filepath.Join(libDir, entry.Name())
		if _, statErr := os.Stat(filepath.Join(smDir, ".git")); statErr != nil {
			continue
		}
		wsSmDir := filepath.Join(roMount, "lib", entry.Name())
		if err := streamCommand(ctx, "git", "-C", smDir, "remote", "set-url", "origin", wsSmDir); err != nil {
			return err
		}
	}
	return nil
}

func runInsideVM(ctx context.Context, repoRoot string, githubTokenFile string) error {
	if repoRoot == "" {
		repoRoot = "/Volumes/My Shared Files/workspace"
	}
	if err := freshsmoke.AssertAbsent("rg", "go", "shfmt", "ast-grep"); err != nil {
		slog.ErrorContext(ctx, "asserting absent tools", "err", err)
		return fmt.Errorf("asserting absent tools: %w", err)
	}
	if err := cloneWorkspaceInVM(ctx, repoRoot); err != nil {
		return err
	}
	repoRoot = vmSmokeRepoDir
	if err := freshsmoke.AssertSmokeSubmodulesPresent(repoRoot); err != nil {
		slog.ErrorContext(ctx, "checking smoke submodules", "err", err)
		return fmt.Errorf("checking smoke submodules: %w", err)
	}

	home, err := os.MkdirTemp("", "dotfiles-fresh-macos-*")
	if err != nil {
		slog.ErrorContext(ctx, "creating smoke home", "err", err)
		return fmt.Errorf("creating smoke home: %w", err)
	}

	dotsBinaryDir := filepath.Join(home, ".cache", "dots", "bin")
	lockFile := filepath.Join(dotsBinaryDir, ".dots.build.lock")
	env := append(os.Environ(),
		"HOME="+home,
		"DOTDOTFILES="+repoRoot,
		"DOTFILES_LOG_LEVEL=debug",
		"DOTS_BINARY_DIR="+dotsBinaryDir,
		"DOTS_BUILD_LOCK_FILE="+lockFile,
		"GO_LOCAL_ROOT="+filepath.Join(home, ".local", "go"),
		"GOMODCACHE="+filepath.Join(home, "go", "pkg", "mod"),
		"GOCACHE="+filepath.Join(home, ".cache", "go-build"),
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
	)
	token, err := readGitHubTokenFile(githubTokenFile)
	if err != nil {
		slog.ErrorContext(ctx, "reading GitHub token file", "err", err)
		return fmt.Errorf("reading GitHub token file: %w", err)
	}
	env = appendGitHubTokenEnv(env, token)

	expectedPath := macSmokePath(home, freshsmoke.EnvValue(env, "PATH"))

	firstOutput, err := freshsmoke.RunInstall(ctx, repoRoot, env, defaultTimeout, "--strict")
	if err != nil {
		slog.ErrorContext(ctx, "first install run", "err", err)
		return fmt.Errorf("first install run: %w", err)
	}
	if err := freshsmoke.AssertStrictInstallOutput(firstOutput); err != nil {
		slog.ErrorContext(ctx, "first install strict output", "err", err)
		return fmt.Errorf("first install strict output: %w", err)
	}
	if err := freshsmoke.AssertBuildCount(firstOutput, 1); err != nil {
		slog.ErrorContext(ctx, "first install build count", "err", err)
		return fmt.Errorf("first install build count: %w", err)
	}
	if err := freshsmoke.AssertCommandsOnPath(expectedPath, "go", "rg", "zsh"); err != nil {
		slog.ErrorContext(ctx, "first install commands", "err", err)
		return fmt.Errorf("first install commands: %w", err)
	}

	secondOutput, err := freshsmoke.RunInstall(ctx, repoRoot, env, defaultTimeout, "--strict")
	if err != nil {
		slog.ErrorContext(ctx, "second install run", "err", err)
		return fmt.Errorf("second install run: %w", err)
	}
	if err := freshsmoke.AssertStrictInstallOutput(secondOutput); err != nil {
		slog.ErrorContext(ctx, "second install strict output", "err", err)
		return fmt.Errorf("second install strict output: %w", err)
	}
	if err := freshsmoke.AssertBuildCount(secondOutput, 0); err != nil {
		slog.ErrorContext(ctx, "second install build count", "err", err)
		return fmt.Errorf("second install build count: %w", err)
	}

	if err := runSharedScenarios(ctx, repoRoot, dotsBinaryDir, lockFile, env); err != nil {
		return err
	}

	fmt.Println("fresh-macos-bootstrap: passed")
	return nil
}

func githubTokenFromEnv() string {
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv("GH_TOKEN"))
}

func readGitHubTokenFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		slog.Warn("reading GitHub token file", "err", err)
		return "", fmt.Errorf("reading GitHub token file %s: %w", path, err)
	}
	return strings.TrimSpace(string(contents)), nil
}

func appendGitHubTokenEnv(env []string, token string) []string {
	if token == "" {
		return env
	}
	return append(env, "GITHUB_TOKEN="+token)
}

// waitForTartExec polls "tart exec vmName true" until the Guest Agent responds
// (meaning the VM has booted), or ctx is canceled, or 5 minutes elapse.
// tart ip --wait returns immediately for freshly-cloned VMs before boot completes.
func waitForTartExec(ctx context.Context, vmName string) error {
	const pollInterval = 5 * time.Second
	const maxWait = 5 * time.Minute
	deadline := time.Now().Add(maxWait)
	for {
		cmd := exec.CommandContext(ctx, "tart", "exec", vmName, "true")
		if err := cmd.Run(); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			err := fmt.Errorf("tart Guest Agent in %s not ready after %v", vmName, maxWait)
			slog.ErrorContext(ctx, "tart Guest Agent timeout", "vm", vmName, "err", err)
			return err
		}
		select {
		case <-ctx.Done():
			err := fmt.Errorf("context canceled waiting for tart Guest Agent: %w", ctx.Err())
			slog.ErrorContext(ctx, "context canceled waiting for tart Guest Agent", "vm", vmName, "err", err)
			return err
		case <-time.After(pollInterval):
		}
	}
}

func streamCommand(ctx context.Context, command string, args ...string) error {
	slog.InfoContext(ctx, "running command", "command", command)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.ErrorContext(ctx, "running command", "command", command, "err", err)
		return fmt.Errorf("running %s: %w", command, err)
	}
	return nil
}

func resolveRepoRoot(repoRoot string) (string, error) {
	if repoRoot == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			slog.Error("resolving current directory", "err", err)
			return "", fmt.Errorf("resolving current directory: %w", err)
		}
		repoRoot = filepath.Dir(filepath.Dir(workingDirectory))
	}
	absolute, err := filepath.Abs(repoRoot)
	if err != nil {
		slog.Error("resolving repo root", "path", repoRoot, "err", err)
		return "", fmt.Errorf("resolving repo root %s: %w", repoRoot, err)
	}
	return absolute, nil
}
