// Command fresh-linux-bootstrap-smoke runs a fresh-host bootstrap smoke test
// inside a Linux container (debian:trixie or ubuntu:24.04) using the Docker
// Engine API. It verifies that install.sh works on a machine with no Go, no
// ripgrep, and no shfmt pre-installed.
package main

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"goodkind.io/.dotfiles/internal/freshsmoke"
)

const (
	defaultImage     = "debian:trixie"
	containerTimeout = 90 * time.Minute // outer: covers apt + clone + two installs
	installTimeout   = 40 * time.Minute // per RunInstall call inside the container
	smokeLabelKey    = "goodkind.dotfiles.smoke"
	smokeLabel       = "fresh-linux-bootstrap"
)

type options struct {
	image     string
	repoRoot  string
	container bool
}

func main() {
	slog.Info("fresh-linux-bootstrap-smoke starting")
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fresh-linux-bootstrap: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	opts := parseOptions()
	if opts.container || os.Getenv("DOTFILES_FRESH_LINUX_SMOKE_IN_CONTAINER") == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), containerTimeout)
		defer cancel()
		return runInsideContainer(ctx)
	}
	return runContainer(opts)
}

func parseOptions() options {
	var opts options
	flag.StringVar(&opts.image, "image", freshsmoke.GetenvDefault("DOTFILES_FRESH_LINUX_IMAGE", defaultImage), "container image")
	flag.StringVar(&opts.repoRoot, "repo-root", "", "repository root to mount")
	flag.BoolVar(&opts.container, "container", false, "run the in-container smoke")
	flag.Parse()
	return opts
}

func runContainer(opts options) error {
	repoRoot, err := resolveRepoRoot(opts.repoRoot)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), containerTimeout)
	defer cancel()
	slog.InfoContext(ctx, "running container", "image", opts.image)

	smokeBinary, err := buildSmokeBinary(ctx)
	if err != nil {
		return err
	}
	defer os.Remove(smokeBinary)

	apiClient, err := client.New(client.FromEnv, client.WithUserAgent("dotfiles-fresh-bootstrap/1.0"))
	if err != nil {
		slog.ErrorContext(ctx, "creating Docker Engine client", "err", err)
		return fmt.Errorf("creating Docker Engine client: %w", err)
	}
	defer apiClient.Close()

	if err := pullImage(ctx, apiClient, opts.image); err != nil {
		return err
	}
	cleanupSmokeContainers(ctx, apiClient)
	defer cleanupSmokeContainers(context.Background(), apiClient)

	containerID, err := createSmokeContainer(ctx, apiClient, opts.image, repoRoot)
	if err != nil {
		return err
	}
	defer removeContainer(apiClient, containerID)

	if err := copySmokeBinary(ctx, apiClient, containerID, smokeBinary); err != nil {
		return err
	}
	return attachStartAndWait(ctx, apiClient, containerID)
}

func buildSmokeBinary(ctx context.Context) (string, error) {
	tmpDirectory, err := os.MkdirTemp("", "dotfiles-fresh-bootstrap-*")
	if err != nil {
		slog.ErrorContext(ctx, "creating smoke build directory", "err", err)
		return "", fmt.Errorf("creating smoke build directory: %w", err)
	}
	outputPath := filepath.Join(tmpDirectory, "fresh-linux-bootstrap-smoke")
	cmd := exec.CommandContext(
		ctx,
		"go",
		"build",
		"-o",
		outputPath,
		"./cmd/fresh-linux-bootstrap-smoke",
	)
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+containerGoArch(), "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmpDirectory)
		slog.ErrorContext(ctx, "building container smoke binary", "err", err)
		return "", fmt.Errorf("building container smoke binary: %w", err)
	}
	return outputPath, nil
}

func containerGoArch() string {
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "amd64"
}

// containerMemoryBytes returns the memory limit for the smoke container.
// Override with DOTFILES_FRESH_LINUX_MEMORY (bytes as a decimal integer).
func containerMemoryBytes() int64 {
	const defaultMemory = 4 * 1024 * 1024 * 1024 // 4 GiB
	raw := os.Getenv("DOTFILES_FRESH_LINUX_MEMORY")
	if raw == "" {
		return defaultMemory
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return defaultMemory
	}
	return value
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

func pullImage(ctx context.Context, apiClient *client.Client, image string) error {
	pullReader, err := apiClient.ImagePull(ctx, image, client.ImagePullOptions{})
	if err != nil {
		slog.ErrorContext(ctx, "pulling image", "image", image, "err", err)
		return fmt.Errorf("pulling image %s: %w", image, err)
	}
	defer pullReader.Close()
	if _, err := io.Copy(io.Discard, pullReader); err != nil {
		slog.ErrorContext(ctx, "reading image pull stream", "err", err)
		return fmt.Errorf("reading image pull stream: %w", err)
	}
	return nil
}

func createSmokeContainer(ctx context.Context, apiClient *client.Client, image string, repoRoot string) (string, error) {
	containerEnv := []string{
		"DOTDOTFILES=/workspace",
		"DOTFILES_FRESH_LINUX_SMOKE_IN_CONTAINER=1",
	}
	for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if value := os.Getenv(name); value != "" {
			containerEnv = append(containerEnv, name+"="+value)
		}
	}
	createResponse, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Image: image,
		Config: &container.Config{
			Cmd: []string{
				"/tmp/fresh-linux-bootstrap-smoke",
				"--container",
			},
			Env:        containerEnv,
			WorkingDir: "/workspace",
			Labels: map[string]string{
				smokeLabelKey: smokeLabel,
			},
		},
		HostConfig: &container.HostConfig{
			Binds:     []string{repoRoot + ":/workspace:ro"},
			Resources: container.Resources{Memory: containerMemoryBytes()},
		},
	})
	if err != nil {
		slog.ErrorContext(ctx, "creating container", "err", err)
		return "", fmt.Errorf("creating container: %w", err)
	}
	return createResponse.ID, nil
}

func copySmokeBinary(ctx context.Context, apiClient *client.Client, containerID string, smokeBinary string) error {
	archive, err := tarBinary(smokeBinary)
	if err != nil {
		return err
	}
	_, err = apiClient.CopyToContainer(ctx, containerID, client.CopyToContainerOptions{
		DestinationPath: "/tmp",
		Content:         bytes.NewReader(archive),
	})
	if err != nil {
		slog.ErrorContext(ctx, "copying smoke binary to container", "err", err)
		return fmt.Errorf("copying smoke binary to container: %w", err)
	}
	return nil
}

func tarBinary(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		slog.Error("reading smoke binary", "path", path, "err", err)
		return nil, fmt.Errorf("reading smoke binary: %w", err)
	}
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	if err := writer.WriteHeader(&tar.Header{
		Name: "fresh-linux-bootstrap-smoke",
		Mode: 0o755,
		Size: int64(len(content)),
	}); err != nil {
		slog.Error("writing smoke binary tar header", "err", err)
		return nil, fmt.Errorf("writing smoke binary tar header: %w", err)
	}
	if _, err := writer.Write(content); err != nil {
		slog.Error("writing smoke binary tar body", "err", err)
		return nil, fmt.Errorf("writing smoke binary tar body: %w", err)
	}
	if err := writer.Close(); err != nil {
		slog.Error("closing smoke binary tar", "err", err)
		return nil, fmt.Errorf("closing smoke binary tar: %w", err)
	}
	return archive.Bytes(), nil
}

func attachStartAndWait(ctx context.Context, apiClient *client.Client, containerID string) error {
	attachResult, err := apiClient.ContainerAttach(ctx, containerID, client.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		slog.ErrorContext(ctx, "attaching to container logs", "err", err)
		return fmt.Errorf("attaching to container logs: %w", err)
	}
	defer attachResult.Close()

	logsDone := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr := fmt.Errorf("%v", r)
				slog.ErrorContext(ctx, "panic in log-copy goroutine", "err", panicErr)
				logsDone <- fmt.Errorf("panic in log-copy goroutine: %w", panicErr)
			}
		}()
		_, err := stdcopy.StdCopy(os.Stdout, os.Stderr, attachResult.Reader)
		if err != nil {
			logsDone <- fmt.Errorf("copying container log stream: %w", err)
			return
		}
		logsDone <- nil
	}()

	if _, err := apiClient.ContainerStart(ctx, containerID, client.ContainerStartOptions{}); err != nil {
		slog.ErrorContext(ctx, "starting container", "err", err)
		return fmt.Errorf("starting container: %w", err)
	}

	wait := apiClient.ContainerWait(ctx, containerID, client.ContainerWaitOptions{})
	var waitErr error
	select {
	case err := <-wait.Error:
		if err != nil {
			slog.ErrorContext(ctx, "waiting for container", "err", err)
			waitErr = fmt.Errorf("waiting for container: %w", err)
		}
	case result := <-wait.Result:
		if result.StatusCode != 0 {
			waitErr = fmt.Errorf("container exited with status %d", result.StatusCode)
		}
	}

	select {
	case err := <-logsDone:
		if err != nil {
			return err
		}
	case <-time.After(5 * time.Second):
	}
	return waitErr
}

func removeContainer(apiClient *client.Client, containerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = apiClient.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
}

func cleanupSmokeContainers(ctx context.Context, apiClient *client.Client) {
	filters := client.Filters{}.Add("label", smokeLabelKey+"="+smokeLabel)
	containers, err := apiClient.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: filters,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "fresh-linux-bootstrap: cleanup list failed: %v\n", err)
		return
	}
	for _, item := range containers.Items {
		_, err := apiClient.ContainerRemove(ctx, item.ID, client.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "fresh-linux-bootstrap: cleanup remove %s failed: %v\n", item.ID, err)
		}
	}
}

// smokeRepoDir is the writable git clone of the read-only /workspace bind mount.
// Cloning gives dots sync a writable .git/ for FETCH_HEAD and produces a clean
// repo with no stale MERGE_HEAD artifacts from the developer's machine.
const (
	smokeRepoDir   = "/tmp/dotfiles-smoke-repo"
	smokeOriginDir = "/tmp/dotfiles-smoke-origin.git"
)

func cloneWorkspace(ctx context.Context) error {
	slog.InfoContext(ctx, "copying workspace to writable path", "dest", smokeRepoDir)
	// Use cp -rp rather than git clone so submodules are already checked out,
	// avoiding git fetch calls to GitHub remote URLs which hang inside Docker.
	cmd := exec.CommandContext(ctx, "cp", "-rp", "/workspace", smokeRepoDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.ErrorContext(ctx, "copying workspace", "err", err)
		return fmt.Errorf("copying workspace: %w", err)
	}
	// Remove stale in-flight git state from the developer's machine.
	for _, name := range []string{"MERGE_HEAD", "MERGE_MSG", "CHERRY_PICK_HEAD", "REVERT_HEAD"} {
		_ = os.Remove(filepath.Join(smokeRepoDir, ".git", name))
	}
	// Fix ownership: cp -rp preserves the original owner (CI runner UID != 0),
	// which causes git 2.35.2+ to reject the repo as "dubious ownership" when
	// the container runs as root. Chowning to root eliminates the mismatch for
	// the entire tree (repo root + submodule dirs) in one step.
	if err := runStreamingCommand(ctx, "chown", "-R", "root:root", smokeRepoDir); err != nil {
		return err
	}
	// Use a local bare origin owned by the container user so sync can exercise
	// git fetch without relying on broad safe.directory overrides.
	if err := runStreamingCommand(ctx, "git", "clone", "--bare", smokeRepoDir, smokeOriginDir); err != nil {
		return err
	}
	if err := runStreamingCommand(ctx, "git", "-C", smokeRepoDir, "remote", "set-url", "origin", smokeOriginDir); err != nil {
		return err
	}
	// Redirect each submodule's origin to a local bare counterpart.
	libDir := filepath.Join(smokeRepoDir, "lib")
	entries, err := os.ReadDir(libDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // lib/ absent — nothing to redirect
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
		originDir := filepath.Join("/tmp", "dotfiles-smoke-origin-"+entry.Name()+".git")
		if err := runStreamingCommand(ctx, "git", "clone", "--bare", smDir, originDir); err != nil {
			return err
		}
		if err := runStreamingCommand(ctx, "git", "-C", smDir, "remote", "set-url", "origin", originDir); err != nil {
			return err
		}
	}
	return nil
}

func runInsideContainer(ctx context.Context) error {
	slog.InfoContext(ctx, "running inside container")
	if err := ensureDownloadTool(ctx); err != nil {
		return err
	}
	if err := cloneWorkspace(ctx); err != nil {
		return err
	}
	if err := freshsmoke.AssertSmokeSubmodulesPresent(smokeRepoDir); err != nil {
		slog.ErrorContext(ctx, "checking smoke submodules", "err", err)
		return fmt.Errorf("checking smoke submodules: %w", err)
	}
	if err := freshsmoke.AssertAbsent("rg", "go", "shfmt", "ast-grep"); err != nil {
		slog.ErrorContext(ctx, "asserting absent tools", "err", err)
		return fmt.Errorf("asserting absent tools: %w", err)
	}

	home := freshsmoke.GetenvDefault("DOTFILES_FRESH_LINUX_HOME", "/tmp/dotfiles-fresh-home")
	dotsBinaryDirectory := filepath.Join(home, ".cache", "dots", "bin")
	lockFile := filepath.Join(dotsBinaryDirectory, ".dots.build.lock")
	env := append(os.Environ(),
		"HOME="+home,
		"DOTDOTFILES="+smokeRepoDir,
		"DOTFILES_LOG_LEVEL=debug",
		"DOTS_BINARY_DIR="+dotsBinaryDirectory,
		"DOTS_BUILD_LOCK_FILE="+lockFile,
		"GO_LOCAL_ROOT="+filepath.Join(home, ".local", "go"),
		"GOMODCACHE="+filepath.Join(home, "go", "pkg", "mod"),
		"GOCACHE="+filepath.Join(home, ".cache", "go-build"),
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)

	if err := os.MkdirAll(home, 0o755); err != nil {
		slog.ErrorContext(ctx, "creating smoke home", "err", err)
		return fmt.Errorf("creating smoke home: %w", err)
	}

	expectedPath := linuxSmokePath(home, freshsmoke.EnvValue(env, "PATH"))

	firstOutput, err := freshsmoke.RunInstall(ctx, smokeRepoDir, env, installTimeout, "--strict")
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

	hasFlock := freshsmoke.HasCommandOnPath("flock", freshsmoke.EnvValue(env, "PATH"))
	if hasFlock {
		if err := freshsmoke.LockSmoke(ctx, smokeRepoDir, dotsBinaryDirectory, lockFile, env, installTimeout); err != nil {
			return fmt.Errorf("lock smoke: %w", err)
		}
	}

	secondOutput, err := freshsmoke.RunInstall(ctx, smokeRepoDir, env, installTimeout, "--strict")
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

	if err := freshsmoke.StalenessSmoke(ctx, smokeRepoDir, "config/dispatch.toml", "dots/internal/util/path.go", env, installTimeout); err != nil {
		return fmt.Errorf("staleness smoke: %w", err)
	}

	if hasFlock {
		if err := freshsmoke.LockTimeoutSmoke(ctx, smokeRepoDir, dotsBinaryDirectory, lockFile, env, installTimeout); err != nil {
			return fmt.Errorf("lock-timeout smoke: %w", err)
		}
		if err := freshsmoke.InstallRaceSmoke(ctx, smokeRepoDir, dotsBinaryDirectory, lockFile, env, installTimeout); err != nil {
			return fmt.Errorf("install-race smoke: %w", err)
		}
	}

	// Runs last: it replaces GO_LOCAL_ROOT's go to force a re-download.
	if err := freshsmoke.StaleGoUpgradeSmoke(ctx, smokeRepoDir, dotsBinaryDirectory, env, installTimeout); err != nil {
		return fmt.Errorf("stale-go upgrade smoke: %w", err)
	}

	fmt.Println("fresh-linux-bootstrap: passed")
	return nil
}

func linuxSmokePath(home string, basePath string) string {
	return freshsmoke.PathWithEntries(
		basePath,
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".local", "go", "bin"),
		filepath.Join(home, ".cargo", "bin"),
	)
}

func ensureDownloadTool(ctx context.Context) error {
	needsCurl := !freshsmoke.HasCommand("curl") && !freshsmoke.HasCommand("wget")
	needsGit := !freshsmoke.HasCommand("git")
	if !needsCurl && !needsGit {
		return nil
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("curl/wget and git are required when the smoke is not running as root")
	}
	if err := runStreamingCommand(ctx, "apt-get", "update", "-qq"); err != nil {
		return err
	}
	packages := []string{
		"apt-get", "install", "-y", "-qq", "--no-install-recommends",
		"ca-certificates",
	}
	if needsCurl {
		packages = append(packages, "curl")
	}
	if needsGit {
		packages = append(packages, "git")
	}
	return runStreamingCommand(ctx, packages[0], packages[1:]...)
}

func runStreamingCommand(ctx context.Context, command string, args ...string) error {
	slog.InfoContext(ctx, "running command", "command", command)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.ErrorContext(ctx, "running command", "command", command, "err", err)
		return fmt.Errorf("running %s: %w", command, err)
	}
	return nil
}
