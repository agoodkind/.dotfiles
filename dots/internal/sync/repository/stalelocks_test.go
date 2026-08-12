package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"goodkind.io/.dotfiles/internal/gitdir"
)

// TestClearStaleGitLocksRemovesAbandonedSubmoduleLock reproduces the failure
// that blocked the updater on every login: a lock left behind in a submodule
// git directory, which lives under the shared directory rather than under the
// repository root.
func TestClearStaleGitLocksRemovesAbandonedSubmoduleLock(t *testing.T) {
	commonDir := t.TempDir()
	submoduleGitDir := filepath.Join(commonDir, "modules", "lib", "zinit")
	if err := os.MkdirAll(submoduleGitDir, 0o755); err != nil {
		t.Fatalf("creating submodule git dir: %v", err)
	}

	// The real lock was zero bytes and about two months old.
	staleLock := filepath.Join(submoduleGitDir, "index.lock")
	writeLock(t, staleLock, time.Now().Add(-48*time.Hour))

	layout := gitdir.Info{Root: t.TempDir(), GitDir: commonDir, CommonDir: commonDir, IsWorktree: false}
	clearStaleGitLocks(context.Background(), layout, nil)

	if _, err := os.Stat(staleLock); !os.IsNotExist(err) {
		t.Fatalf("stale submodule lock still present at %s (stat err = %v)", staleLock, err)
	}
}

// TestClearStaleGitLocksLeavesFreshLocks confirms a lock a live git still
// holds is not removed. Removing one would corrupt that operation.
func TestClearStaleGitLocksLeavesFreshLocks(t *testing.T) {
	commonDir := t.TempDir()
	freshLock := filepath.Join(commonDir, "index.lock")
	writeLock(t, freshLock, time.Now())

	layout := gitdir.Info{Root: t.TempDir(), GitDir: commonDir, CommonDir: commonDir, IsWorktree: false}
	clearStaleGitLocks(context.Background(), layout, nil)

	if _, err := os.Stat(freshLock); err != nil {
		t.Fatalf("fresh lock was removed from %s: %v", freshLock, err)
	}
}

// TestClearStaleGitLocksCoversEveryLockName confirms each lock git can leave
// behind is cleared, in the shared directory and in a submodule alike.
func TestClearStaleGitLocksCoversEveryLockName(t *testing.T) {
	commonDir := t.TempDir()
	submoduleGitDir := filepath.Join(commonDir, "modules", "lib", "zsh-defer")
	if err := os.MkdirAll(submoduleGitDir, 0o755); err != nil {
		t.Fatalf("creating submodule git dir: %v", err)
	}

	old := time.Now().Add(-24 * time.Hour)
	created := make([]string, 0, len(gitLockNames)*2)
	for _, dir := range []string{commonDir, submoduleGitDir} {
		for _, lockName := range gitLockNames {
			lockPath := filepath.Join(dir, lockName)
			if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
				t.Fatalf("creating lock parent: %v", err)
			}
			writeLock(t, lockPath, old)
			created = append(created, lockPath)
		}
	}

	layout := gitdir.Info{Root: t.TempDir(), GitDir: commonDir, CommonDir: commonDir, IsWorktree: false}
	clearStaleGitLocks(context.Background(), layout, nil)

	for _, lockPath := range created {
		if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
			t.Fatalf("lock still present at %s (stat err = %v)", lockPath, err)
		}
	}
}

func writeLock(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("writing lock %s: %v", path, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("setting lock mod time %s: %v", path, err)
	}
}
