// Package freshsmoke provides shared assertion helpers for fresh-bootstrap smoke tests.
package freshsmoke

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// HasCommand reports whether command is findable via [exec.LookPath].
func HasCommand(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// AssertAbsent returns an error if any of the named commands are findable via [exec.LookPath].
func AssertAbsent(commands ...string) error {
	for _, command := range commands {
		if HasCommand(command) {
			return fmt.Errorf("expected %s to be absent before bootstrap", command)
		}
	}
	return nil
}

// AssertContains returns an error if output does not contain substr.
func AssertContains(output, substr string) error {
	if strings.Contains(output, substr) {
		return nil
	}
	return fmt.Errorf("expected output to contain %q\n%s", substr, output)
}

// AssertBuildCount returns an error if output does not contain exactly
// expectedCount occurrences of "dots: building binary".
func AssertBuildCount(output string, expectedCount int) error {
	actualCount := strings.Count(output, "dots: building binary")
	if actualCount != expectedCount {
		return fmt.Errorf("expected %d build lines, got %d\n%s", expectedCount, actualCount, output)
	}
	return nil
}

// AssertNoStrictWarnings returns an error if output contains sync warning markers.
func AssertNoStrictWarnings(output string) error {
	markers := []string{
		"[WARN]",
		"level=WARN",
		"WARN:",
		"sync step failed",
		"non-critical failures",
		"custom tools completed with failures",
	}
	for _, marker := range markers {
		if strings.Contains(output, marker) {
			return fmt.Errorf("strict smoke output contains warning marker %q\n%s", marker, output)
		}
	}
	return nil
}

// AssertStrictInstallOutput verifies the common strict-smoke output contract.
func AssertStrictInstallOutput(output string) error {
	if err := AssertNoStrictWarnings(output); err != nil {
		return err
	}
	if err := AssertContains(output, "dots debug logging enabled"); err != nil {
		return err
	}
	return nil
}

// AssertCommandsOnPath returns an error if any command is absent from pathEnv.
func AssertCommandsOnPath(pathEnv string, commands ...string) error {
	missing := make([]string, 0)
	for _, command := range commands {
		if HasCommandOnPath(command, pathEnv) {
			continue
		}
		missing = append(missing, command)
	}
	if len(missing) > 0 {
		return fmt.Errorf("expected commands on PATH, missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

// PathWithEntries appends entries to a PATH string while preserving order and uniqueness.
func PathWithEntries(pathEnv string, entries ...string) string {
	seen := make(map[string]struct{})
	parts := make([]string, 0)
	for part := range strings.SplitSeq(pathEnv, ":") {
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		parts = append(parts, part)
	}
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		parts = append(parts, entry)
	}
	return strings.Join(parts, ":")
}

// HoldBuildLockFor holds an exclusive flock on lockFile for duration then
// releases it asynchronously. Returns a channel that closes when released.
func HoldBuildLockFor(lockFile string, duration time.Duration) (<-chan struct{}, error) {
	if err := os.MkdirAll(filepath.Dir(lockFile), 0o755); err != nil {
		slog.Error("creating lock directory", "err", err)
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		slog.Error("opening build lock", "err", err)
		return nil, fmt.Errorf("opening build lock: %w", err)
	}
	// Bounds-check before converting uintptr fd to int (satisfies gosec G115).
	// POSIX fds are small non-negative integers; this guard is a no-op in practice.
	rawFD := file.Fd()
	if rawFD > uintptr(^uint(0)>>1) {
		_ = file.Close()
		return nil, fmt.Errorf("file descriptor %d exceeds valid flock range", rawFD)
	}
	fd := int(rawFD)
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		_ = file.Close()
		slog.Error("holding build lock", "err", err)
		return nil, fmt.Errorf("holding build lock: %w", err)
	}
	released := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in hold-lock goroutine", "err", fmt.Errorf("%v", r))
			}
		}()
		defer close(released)
		timer := time.NewTimer(duration)
		defer timer.Stop()
		<-timer.C
		_ = syscall.Flock(fd, syscall.LOCK_UN)
		_ = file.Close()
	}()
	return released, nil
}

// liveStream forwards smoke output to a live stream (stdout or stderr) while
// swallowing that stream's write errors. Go's exec closes the child's output
// pipe read-end if the copy goroutine's Write returns an error, which then kills
// the child (and any subprocess sharing the pipe) with SIGPIPE, surfacing as
// exit status 141. CI log collectors can transiently close the smoke's stdout,
// so swallowing the live stream's errors keeps the pipe drained, leaves the
// captured buffer the assertions read intact, and lets the child run to
// completion.
type liveStream struct {
	stream io.Writer
}

func (s liveStream) Write(payload []byte) (int, error) {
	_, _ = s.stream.Write(payload)
	return len(payload), nil
}

// RunInstall runs install.sh --use-defaults from repoRoot with the given env,
// streaming output to [os.Stdout]/[os.Stderr] and returning the combined output.
func RunInstall(ctx context.Context, repoRoot string, env []string, timeout time.Duration, extraArgs ...string) (string, error) {
	slog.InfoContext(ctx, "running install.sh", "repoRoot", repoRoot)
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	installScript := filepath.Join(repoRoot, "install.sh")
	args := append([]string{installScript, "--use-defaults"}, extraArgs...)
	cmd := exec.CommandContext(callCtx, "bash", args...)
	cmd.Env = env
	var output bytes.Buffer
	cmd.Stdout = io.MultiWriter(liveStream{os.Stdout}, &output)
	cmd.Stderr = io.MultiWriter(liveStream{os.Stderr}, &output)

	err := cmd.Run()
	text := output.String()
	if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		return text, fmt.Errorf("install timed out")
	}
	if err != nil {
		slog.ErrorContext(ctx, "install failed", "err", err)
		return text, fmt.Errorf("install failed: %w\n%s", err, text)
	}
	return text, nil
}

// GetenvDefault returns the value of environment variable name if non-empty,
// otherwise fallback.
func GetenvDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// EnvValue returns the value of key in an [os.Environ]-style slice (KEY=VALUE pairs),
// or "" if the key is absent.
func EnvValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return entry[len(prefix):]
		}
	}
	return ""
}

// HasCommandOnPath reports whether command exists as an executable in any directory
// listed in pathEnv (a colon-separated PATH string).
func HasCommandOnPath(command, pathEnv string) bool {
	for dir := range strings.SplitSeq(pathEnv, ":") {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, command)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return true
		}
	}
	return false
}
