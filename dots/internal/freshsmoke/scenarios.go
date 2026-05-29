package freshsmoke

// This file holds the bootstrap smoke scenarios shared by the Linux (Docker) and
// macOS (Tart VM) smokes, so every issue we have hit in production is reproduced
// on both platforms. Each scenario mutates the host it runs on (removes the
// cached binary, seeds a stale Go, holds the build lock), so callers must only
// run them inside an isolated container or VM, never against a real workstation.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"goodkind.io/.dotfiles/internal/clock"
)

const (
	// lockTimeoutHold is how long LockTimeoutSmoke holds the lock, and
	// lockTimeoutWaitSeconds is the bounded wait it gives the install. The
	// install must give up near the wait rather than block for the full hold.
	// lockTimeoutElapsedCap leaves room for the go-run fallback to recompile
	// from a warm cache while still proving the install did not block on the hold.
	lockTimeoutHold        = 20 * time.Second
	lockTimeoutWaitSeconds = 3
	lockTimeoutElapsedCap  = 15 * time.Second

	// staleGoStub reports a version below any modern go.mod floor. Only the
	// numeric version is parsed, so the OS/arch suffix is irrelevant and the
	// same stub serves both platforms.
	staleGoStub = "#!/bin/sh\necho \"go version go1.22.7\"\n"
)

// removeCachedBinary deletes the cached dots binary so the next install must
// rebuild, making the build count observable.
func removeCachedBinary(ctx context.Context, dotsBinaryDir string) error {
	if err := os.Remove(filepath.Join(dotsBinaryDir, "dots")); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.ErrorContext(ctx, "removing cached dots binary", "err", err)
		return fmt.Errorf("removing cached dots binary: %w", err)
	}
	return nil
}

// seedStaleGo writes an executable stub reporting an old version into
// GO_LOCAL_ROOT/bin/go, forcing bootstrap to treat the toolchain as too old. It
// uses WriteFile-then-Chmod to keep the WriteFile mode within gosec G306,
// matching the pattern in internal/sync/tools.
func seedStaleGo(ctx context.Context, env []string) error {
	goLocalRoot := EnvValue(env, "GO_LOCAL_ROOT")
	if goLocalRoot == "" {
		err := errors.New("GO_LOCAL_ROOT missing from smoke env")
		slog.ErrorContext(ctx, "seeding stale go", "err", err)
		return err
	}
	goBinary := filepath.Join(goLocalRoot, "bin", "go")
	if err := os.MkdirAll(filepath.Dir(goBinary), 0o755); err != nil {
		slog.ErrorContext(ctx, "creating stale go dir", "err", err)
		return fmt.Errorf("creating stale go dir: %w", err)
	}
	if err := os.WriteFile(goBinary, []byte(staleGoStub), 0o600); err != nil {
		slog.ErrorContext(ctx, "writing stale go stub", "err", err)
		return fmt.Errorf("writing stale go stub: %w", err)
	}
	if err := os.Chmod(goBinary, 0o755); err != nil {
		slog.ErrorContext(ctx, "making stale go stub executable", "err", err)
		return fmt.Errorf("making stale go stub executable: %w", err)
	}
	return nil
}

// appendComment appends a no-op comment line to a file so its content (and thus
// its content hash) changes without breaking compilation or parsing.
func appendComment(ctx context.Context, path string, comment string) error {
	file, err := os.OpenFile(filepath.Clean(path), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		slog.ErrorContext(ctx, "opening file to append", "path", path, "err", err)
		return fmt.Errorf("opening %s to append: %w", path, err)
	}
	defer file.Close()
	if _, err := file.WriteString(comment); err != nil {
		slog.ErrorContext(ctx, "appending to file", "path", path, "err", err)
		return fmt.Errorf("appending to %s: %w", path, err)
	}
	return nil
}

// LockSmoke holds the shared lock briefly while an install runs, asserting the
// install waits for the lock and then builds exactly once.
func LockSmoke(ctx context.Context, repoRoot, dotsBinaryDir, lockFile string, env []string, timeout time.Duration) error {
	slog.InfoContext(ctx, "running lock smoke")
	if err := removeCachedBinary(ctx, dotsBinaryDir); err != nil {
		return err
	}
	released, err := HoldBuildLockFor(lockFile, 2*time.Second)
	if err != nil {
		slog.ErrorContext(ctx, "holding build lock", "err", err)
		return fmt.Errorf("holding build lock: %w", err)
	}
	output, err := RunInstall(ctx, repoRoot, env, timeout, "--strict")
	<-released
	if err != nil {
		slog.ErrorContext(ctx, "lock install run", "err", err)
		return fmt.Errorf("lock install run: %w", err)
	}
	if err := AssertStrictInstallOutput(output); err != nil {
		slog.ErrorContext(ctx, "lock install strict output", "err", err)
		return fmt.Errorf("lock install strict output: %w", err)
	}
	if err := AssertContains(output, "dots: waiting for lock"); err != nil {
		slog.ErrorContext(ctx, "lock wait message", "err", err)
		return fmt.Errorf("lock wait message: %w", err)
	}
	if err := AssertBuildCount(output, 1); err != nil {
		slog.ErrorContext(ctx, "lock install build count", "err", err)
		return fmt.Errorf("lock install build count: %w", err)
	}
	return nil
}

// LockTimeoutSmoke holds the lock past the bounded wait and asserts the install
// gives up quickly instead of blocking. Before the bounded wait, a wedged build
// held this lock and every contending login blocked indefinitely, piling up
// stuck processes. Against that prior behavior this blocks for the full hold and
// fails the elapsed-time assertion.
func LockTimeoutSmoke(ctx context.Context, repoRoot, dotsBinaryDir, lockFile string, env []string, timeout time.Duration) error {
	slog.InfoContext(ctx, "running lock-timeout smoke")
	if err := removeCachedBinary(ctx, dotsBinaryDir); err != nil {
		return err
	}
	released, err := HoldBuildLockFor(lockFile, lockTimeoutHold)
	if err != nil {
		slog.ErrorContext(ctx, "holding build lock", "err", err)
		return fmt.Errorf("holding build lock: %w", err)
	}
	waitEnv := append(append([]string{}, env...), fmt.Sprintf("DOTS_BUILD_LOCK_WAIT_SECONDS=%d", lockTimeoutWaitSeconds))
	start := clock.Now()
	output, runErr := RunInstall(ctx, repoRoot, waitEnv, timeout, "--strict")
	elapsed := clock.Now().Sub(start)
	<-released
	if elapsed > lockTimeoutElapsedCap {
		err := fmt.Errorf("install blocked %s on a held lock; expected to give up near %ds", elapsed, lockTimeoutWaitSeconds)
		slog.ErrorContext(ctx, "lock-timeout did not bound the wait", "err", err)
		return err
	}
	if runErr != nil {
		slog.ErrorContext(ctx, "lock-timeout install run", "err", runErr)
		return fmt.Errorf("lock-timeout install run: %w", runErr)
	}
	if err := AssertContains(output, "waiting for lock"); err != nil {
		slog.ErrorContext(ctx, "lock-timeout wait message", "err", err)
		return fmt.Errorf("lock-timeout wait message: %w", err)
	}
	if err := AssertContains(output, "timed out"); err != nil {
		slog.ErrorContext(ctx, "lock-timeout give-up message", "err", err)
		return fmt.Errorf("lock-timeout give-up message: %w", err)
	}
	if err := AssertBuildCount(output, 0); err != nil {
		slog.ErrorContext(ctx, "lock-timeout build count", "err", err)
		return fmt.Errorf("lock-timeout build count: %w", err)
	}
	return nil
}

// StaleGoUpgradeSmoke seeds a too-old Go and asserts bootstrap re-downloads a Go
// that satisfies go.mod instead of reusing the stale one, then builds. This is
// the host state that wedged vault: an old Go left from an earlier bootstrap
// against a go.mod whose floor has since advanced. Against the prior bootstrap
// the stale Go is reused and the build fails, so the install run errors here.
func StaleGoUpgradeSmoke(ctx context.Context, repoRoot, dotsBinaryDir string, env []string, timeout time.Duration) error {
	slog.InfoContext(ctx, "running stale-go upgrade smoke")
	if err := seedStaleGo(ctx, env); err != nil {
		return err
	}
	if err := removeCachedBinary(ctx, dotsBinaryDir); err != nil {
		return err
	}
	output, err := RunInstall(ctx, repoRoot, env, timeout, "--strict")
	if err != nil {
		slog.ErrorContext(ctx, "stale-go upgrade install run", "err", err)
		return fmt.Errorf("stale-go upgrade install run: %w", err)
	}
	if err := AssertContains(output, "does not satisfy go.mod"); err != nil {
		slog.ErrorContext(ctx, "stale-go upgrade message", "err", err)
		return fmt.Errorf("stale-go upgrade message: %w", err)
	}
	if err := AssertBuildCount(output, 1); err != nil {
		slog.ErrorContext(ctx, "stale-go upgrade build count", "err", err)
		return fmt.Errorf("stale-go upgrade build count: %w", err)
	}
	if err := AssertStrictInstallOutput(output); err != nil {
		slog.ErrorContext(ctx, "stale-go upgrade strict output", "err", err)
		return fmt.Errorf("stale-go upgrade strict output: %w", err)
	}
	return nil
}

// InstallRaceSmoke seeds a too-old Go and holds the shared lock while the install
// runs, proving the Go install (not just the build) is serialized by the lock.
// The unlocked install raced the rm -rf and extract of GO_LOCAL_ROOT and on vault
// clobbered a freshly downloaded 1.26 back to 1.22; under the shared lock the
// install waits, then upgrades and builds once.
func InstallRaceSmoke(ctx context.Context, repoRoot, dotsBinaryDir, lockFile string, env []string, timeout time.Duration) error {
	slog.InfoContext(ctx, "running install-race smoke")
	if err := seedStaleGo(ctx, env); err != nil {
		return err
	}
	if err := removeCachedBinary(ctx, dotsBinaryDir); err != nil {
		return err
	}
	released, err := HoldBuildLockFor(lockFile, 2*time.Second)
	if err != nil {
		slog.ErrorContext(ctx, "holding build lock", "err", err)
		return fmt.Errorf("holding build lock: %w", err)
	}
	output, err := RunInstall(ctx, repoRoot, env, timeout, "--strict")
	<-released
	if err != nil {
		slog.ErrorContext(ctx, "install-race install run", "err", err)
		return fmt.Errorf("install-race install run: %w", err)
	}
	if err := AssertContains(output, "dots: waiting for lock"); err != nil {
		slog.ErrorContext(ctx, "install-race expected the install to wait for the lock", "err", err)
		return fmt.Errorf("install-race expected the install to wait for the lock: %w", err)
	}
	if err := AssertContains(output, "does not satisfy go.mod"); err != nil {
		slog.ErrorContext(ctx, "install-race upgrade message", "err", err)
		return fmt.Errorf("install-race upgrade message: %w", err)
	}
	if err := AssertBuildCount(output, 1); err != nil {
		slog.ErrorContext(ctx, "install-race build count", "err", err)
		return fmt.Errorf("install-race build count: %w", err)
	}
	if err := AssertStrictInstallOutput(output); err != nil {
		slog.ErrorContext(ctx, "install-race strict output", "err", err)
		return fmt.Errorf("install-race strict output: %w", err)
	}
	return nil
}

// StalenessSmoke proves the go-list build-input hash: editing a runtime-read
// config file does NOT rebuild (it is not a compiled input), while editing a
// compiled .go file DOES. configRelPath and goRelPath are relative to repoRoot.
func StalenessSmoke(ctx context.Context, repoRoot, configRelPath, goRelPath string, env []string, timeout time.Duration) error {
	slog.InfoContext(ctx, "running staleness smoke")

	if err := appendComment(ctx, filepath.Join(repoRoot, configRelPath), "\n# smoke: config edit must not rebuild\n"); err != nil {
		return err
	}
	configOutput, err := RunInstall(ctx, repoRoot, env, timeout, "--strict")
	if err != nil {
		slog.ErrorContext(ctx, "staleness config-edit install run", "err", err)
		return fmt.Errorf("staleness config-edit install run: %w", err)
	}
	if err := AssertBuildCount(configOutput, 0); err != nil {
		slog.ErrorContext(ctx, "config edit must not rebuild", "path", configRelPath, "err", err)
		return fmt.Errorf("editing %s rebuilt dots, but runtime config is not a build input: %w", configRelPath, err)
	}

	if err := appendComment(ctx, filepath.Join(repoRoot, goRelPath), "\n// smoke: source edit must rebuild\n"); err != nil {
		return err
	}
	goOutput, err := RunInstall(ctx, repoRoot, env, timeout, "--strict")
	if err != nil {
		slog.ErrorContext(ctx, "staleness go-edit install run", "err", err)
		return fmt.Errorf("staleness go-edit install run: %w", err)
	}
	if err := AssertBuildCount(goOutput, 1); err != nil {
		slog.ErrorContext(ctx, "source edit must rebuild", "path", goRelPath, "err", err)
		return fmt.Errorf("editing %s did not rebuild dots, but compiled sources are build inputs: %w", goRelPath, err)
	}
	return nil
}
