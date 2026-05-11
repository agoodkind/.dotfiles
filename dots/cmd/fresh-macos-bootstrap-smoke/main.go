// Command fresh-macos-bootstrap-smoke runs a fresh-host bootstrap smoke test
// on macOS. In direct mode (default, used by CI) it creates a temp HOME and
// restricts PATH so install.sh must bootstrap Homebrew and Go from scratch.
// In --tart mode it runs the assertions inside a pristine Tart macOS VM.
package main

import (
	"context"
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
	repoRoot string
	tart     bool
	image    string
	inVM     bool
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
		return runInsideVM(ctx, opts.repoRoot)
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

	firstOutput, err := freshsmoke.RunInstall(ctx, repoRoot, env, defaultTimeout)
	if err != nil {
		slog.ErrorContext(ctx, "first install run", "err", err)
		return fmt.Errorf("first install run: %w", err)
	}
	if err := freshsmoke.AssertBuildCount(firstOutput, 1); err != nil {
		slog.ErrorContext(ctx, "first install build count", "err", err)
		return fmt.Errorf("first install build count: %w", err)
	}

	if freshsmoke.HasCommandOnPath("flock", freshsmoke.EnvValue(env, "PATH")) {
		if err := runLockSmoke(ctx, dotsBinaryDir, lockFile, repoRoot, env); err != nil {
			return err
		}
	}

	secondOutput, err := freshsmoke.RunInstall(ctx, repoRoot, env, defaultTimeout)
	if err != nil {
		slog.ErrorContext(ctx, "second install run", "err", err)
		return fmt.Errorf("second install run: %w", err)
	}
	if err := freshsmoke.AssertBuildCount(secondOutput, 0); err != nil {
		slog.ErrorContext(ctx, "second install build count", "err", err)
		return fmt.Errorf("second install build count: %w", err)
	}

	fmt.Println("fresh-macos-bootstrap: passed")
	return nil
}

func runLockSmoke(ctx context.Context, dotsBinaryDir string, lockFile string, repoRoot string, env []string) error {
	slog.InfoContext(ctx, "running lock smoke")
	dotsBinary := filepath.Join(dotsBinaryDir, "dots")
	if err := os.Remove(dotsBinary); err != nil {
		slog.ErrorContext(ctx, "removing cached dots binary for lock smoke", "err", err)
		return fmt.Errorf("removing cached dots binary for lock smoke: %w", err)
	}
	lockReleased, err := freshsmoke.HoldBuildLockFor(lockFile, 2*time.Second)
	if err != nil {
		slog.ErrorContext(ctx, "holding build lock", "err", err)
		return fmt.Errorf("holding build lock: %w", err)
	}
	lockOutput, err := freshsmoke.RunInstall(ctx, repoRoot, env, defaultTimeout)
	if err != nil {
		slog.ErrorContext(ctx, "lock install run", "err", err)
		return fmt.Errorf("lock install run: %w", err)
	}
	<-lockReleased
	if err := freshsmoke.AssertContains(lockOutput, "dots: waiting for binary build lock"); err != nil {
		slog.ErrorContext(ctx, "asserting lock wait message", "err", err)
		return fmt.Errorf("asserting lock wait message: %w", err)
	}
	if err := freshsmoke.AssertBuildCount(lockOutput, 1); err != nil {
		slog.ErrorContext(ctx, "lock install build count", "err", err)
		return fmt.Errorf("lock install build count: %w", err)
	}
	return nil
}

// runWithTart is the local path: clones a macOS Tart VM, shares the repo and
// the smoke binary via --dir, SSHes in, and runs --in-vm assertions inside a
// truly pristine VM. Requires: brew install cirruslabs/cli/tart
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

	// Start the VM in the background; tart run blocks until the VM shuts down.
	// context.WithoutCancel creates a context that inherits values from ctx but
	// cannot be canceled, so the VM process outlives the SSH session context.
	vmCtx := context.WithoutCancel(ctx)
	vmCmd := exec.CommandContext(
		vmCtx,
		"tart", "run", vmName,
		"--no-graphics",
		"--dir=workspace:"+repoRoot+":ro",
		"--dir=smoke:"+filepath.Dir(selfPath)+":ro",
	)
	vmCmd.Stdout = os.Stdout
	vmCmd.Stderr = os.Stderr
	if err := vmCmd.Start(); err != nil {
		slog.ErrorContext(ctx, "starting Tart VM", "vm", vmName, "err", err)
		return fmt.Errorf("starting Tart VM: %w", err)
	}
	defer func() { _ = vmCmd.Process.Kill() }()

	fmt.Printf("fresh-macos-bootstrap: waiting for VM %s to boot\n", vmName)
	ip, err := tartIP(ctx, vmName)
	if err != nil {
		return err
	}
	fmt.Printf("fresh-macos-bootstrap: VM %s ready at %s\n", vmName, ip)

	smokePath := "/Volumes/My Shared Files/smoke/" + filepath.Base(selfPath)
	repoInVM := "/Volumes/My Shared Files/workspace"
	sshCmd := strings.Join([]string{
		smokePath,
		"--in-vm",
		"--repo-root", "'" + repoInVM + "'",
	}, " ")

	if err := streamCommand(
		ctx,
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"admin@"+ip,
		sshCmd,
	); err != nil {
		return fmt.Errorf("smoke inside Tart VM: %w", err)
	}
	return nil
}

// runInsideVM runs assertions from inside a Tart VM where the dotfiles repo
// is mounted read-only (typically at /Volumes/My Shared Files/workspace).
func runInsideVM(ctx context.Context, repoRoot string) error {
	if repoRoot == "" {
		repoRoot = "/Volumes/My Shared Files/workspace"
	}
	if err := freshsmoke.AssertAbsent("rg", "go", "shfmt", "ast-grep"); err != nil {
		slog.ErrorContext(ctx, "asserting absent tools", "err", err)
		return fmt.Errorf("asserting absent tools: %w", err)
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
		"DOTS_BINARY_DIR="+dotsBinaryDir,
		"DOTS_BUILD_LOCK_FILE="+lockFile,
		"GO_LOCAL_ROOT="+filepath.Join(home, ".local", "go"),
		"GOMODCACHE="+filepath.Join(home, "go", "pkg", "mod"),
		"GOCACHE="+filepath.Join(home, ".cache", "go-build"),
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
	)

	firstOutput, err := freshsmoke.RunInstall(ctx, repoRoot, env, defaultTimeout)
	if err != nil {
		slog.ErrorContext(ctx, "first install run", "err", err)
		return fmt.Errorf("first install run: %w", err)
	}
	if err := freshsmoke.AssertBuildCount(firstOutput, 1); err != nil {
		slog.ErrorContext(ctx, "first install build count", "err", err)
		return fmt.Errorf("first install build count: %w", err)
	}

	secondOutput, err := freshsmoke.RunInstall(ctx, repoRoot, env, defaultTimeout)
	if err != nil {
		slog.ErrorContext(ctx, "second install run", "err", err)
		return fmt.Errorf("second install run: %w", err)
	}
	if err := freshsmoke.AssertBuildCount(secondOutput, 0); err != nil {
		slog.ErrorContext(ctx, "second install build count", "err", err)
		return fmt.Errorf("second install build count: %w", err)
	}

	fmt.Println("fresh-macos-bootstrap: passed")
	return nil
}

func tartIP(ctx context.Context, vmName string) (string, error) {
	cmd := exec.CommandContext(ctx, "tart", "ip", vmName, "--wait", "60")
	out, err := cmd.Output()
	if err != nil {
		slog.ErrorContext(ctx, "getting Tart VM IP", "vm", vmName, "err", err)
		return "", fmt.Errorf("getting Tart VM IP for %s: %w", vmName, err)
	}
	return strings.TrimSpace(string(out)), nil
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
