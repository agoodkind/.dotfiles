package pathcache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// shellRejectsCache mirrors the test in home/.zshenv, which sources the cache
// only when `$cache -nt $sourceDir` holds. zsh's -nt requires the cache to be
// strictly newer than the directory inode.
func shellRejectsCache(cacheModTime time.Time, sourceDirModTime time.Time) bool {
	return !cacheModTime.After(sourceDirModTime)
}

// TestCacheIsStaleCoversEveryCaseTheShellRejects locks the contract between
// this worker and its shell consumer. Any state in which zsh refuses to source
// the cache must also make this worker rebuild it, or the two wedge and the
// cache is never regenerated.
func TestCacheIsStaleCoversEveryCaseTheShellRejects(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)
	offsets := []time.Duration{-48 * time.Hour, -time.Hour, 0, time.Hour, 48 * time.Hour}

	for _, cacheOffset := range offsets {
		for _, dirOffset := range offsets {
			for _, childOffset := range offsets {
				for _, baseFileOffset := range offsets {
					cacheModTime := base.Add(cacheOffset)
					sourceDir := t.TempDir()
					baseFile := filepath.Join(t.TempDir(), "paths")

					writeFileWithModTime(t, baseFile, base.Add(baseFileOffset))
					writeFileWithModTime(t, filepath.Join(sourceDir, "homebrew"), base.Add(childOffset))
					// The directory time is set last, since writing a child
					// updates it.
					setModTime(t, sourceDir, base.Add(dirOffset))

					goSaysStale := cacheIsStale(cacheModTime, baseFile, sourceDir)
					shellRejects := shellRejectsCache(cacheModTime, base.Add(dirOffset))

					if shellRejects && !goSaysStale {
						t.Fatalf("shell rejects the cache but the worker keeps it: cache=%v dir=%v child=%v base=%v",
							cacheOffset, dirOffset, childOffset, baseFileOffset)
					}
				}
			}
		}
	}
}

// TestCacheIsStaleDetectsDeletedDropIn reproduces the state this machine was
// stuck in: an entry was removed from the drop-in directory, which bumped the
// directory's own time while leaving every surviving child older than the
// cache. Checking only the children misses it.
func TestCacheIsStaleDetectsDeletedDropIn(t *testing.T) {
	t.Parallel()

	cacheModTime := time.Date(2026, time.July, 5, 8, 46, 0, 0, time.UTC)
	childModTime := time.Date(2026, time.June, 24, 19, 29, 0, 0, time.UTC)
	dirModTime := time.Date(2026, time.July, 10, 17, 40, 0, 0, time.UTC)

	sourceDir := t.TempDir()
	baseFile := filepath.Join(t.TempDir(), "paths")
	writeFileWithModTime(t, baseFile, childModTime)
	writeFileWithModTime(t, filepath.Join(sourceDir, "homebrew"), childModTime)
	setModTime(t, sourceDir, dirModTime)

	if !cacheIsStale(cacheModTime, baseFile, sourceDir) {
		t.Fatal("cacheIsStale = false, want true: the drop-in directory is newer than the cache")
	}
}

// TestCacheIsStaleKeepsAFreshCache confirms the worker does not rebuild on
// every run once the cache is genuinely newer than every source.
func TestCacheIsStaleKeepsAFreshCache(t *testing.T) {
	t.Parallel()

	sourceModTime := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	cacheModTime := sourceModTime.Add(time.Hour)

	sourceDir := t.TempDir()
	baseFile := filepath.Join(t.TempDir(), "paths")
	writeFileWithModTime(t, baseFile, sourceModTime)
	writeFileWithModTime(t, filepath.Join(sourceDir, "homebrew"), sourceModTime)
	setModTime(t, sourceDir, sourceModTime)

	if cacheIsStale(cacheModTime, baseFile, sourceDir) {
		t.Fatal("cacheIsStale = true, want false: the cache is newer than every source")
	}
}

func writeFileWithModTime(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("PATH=\"/usr/bin\"\n"), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	setModTime(t, path, modTime)
}

func setModTime(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("setting mod time on %s: %v", path, err)
	}
}
