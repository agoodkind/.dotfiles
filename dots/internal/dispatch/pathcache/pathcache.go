// Package pathcache implements caching of shell PATH entries.
package pathcache

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// pathSourceDir holds the drop-in files path_helper reads.
const pathSourceDir = "/etc/paths.d"

// systemPathFile is the base path list path_helper reads.
const systemPathFile = "/etc/paths"

// cacheIsStale reports whether a cache written at cacheModTime must be rebuilt.
//
// The consumer is home/.zshenv, which sources the cache only when
// `$cache -nt /etc/paths.d` holds. That test compares against the directory
// inode, whose modification time changes when an entry is added, removed, or
// renamed, independently of the files that remain inside. Checking only the
// surviving children therefore misses a deletion, and the two sides can wedge:
// zsh rejects a cache that this worker considers current, and nothing rebuilds
// it. Including the directory itself keeps this test a superset of the shell's.
//
// A source counts as newer, and therefore triggers a rebuild, whenever its
// modification time is not strictly before the cache's. Equal modification
// times count as newer for this same reason: the shell's -nt requires the
// cache to be strictly newer than every source, so an equal timestamp already
// makes the shell reject the cache, and this worker must rebuild it too.
func cacheIsStale(cacheModTime time.Time, baseFile string, sourceDir string) bool {
	sources := []string{baseFile, sourceDir}
	if entries, err := os.ReadDir(sourceDir); err == nil {
		for _, entry := range entries {
			sources = append(sources, filepath.Join(sourceDir, entry.Name()))
		}
	}
	for _, source := range sources {
		info, err := os.Stat(filepath.Clean(source))
		if err != nil {
			continue
		}
		if !info.ModTime().Before(cacheModTime) {
			return true
		}
	}
	return false
}

// Rebuild regenerates the macOS path_helper cache and writes it to disk.
func Rebuild(ctx context.Context, dispatchLogger *telemetry.Logger) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	if _, err := os.Stat("/usr/libexec/path_helper"); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.WarnContext(ctx, "pathcache: stat path_helper", slog.Any("error", err))
		return fmt.Errorf("stat path_helper: %w", err)
	}

	cacheDir := filepath.Join(os.Getenv("HOME"), ".cache", "zsh_startup")
	if err := os.MkdirAll(filepath.Clean(cacheDir), 0o755); err != nil {
		slog.WarnContext(ctx, "pathcache: creating path cache dir", slog.Any("error", err))
		return fmt.Errorf("creating path cache directory: %w", err)
	}
	cacheFile := filepath.Join(cacheDir, "path_cache.zsh")

	cacheInfo, err := os.Stat(filepath.Clean(cacheFile))
	needsRebuild := err != nil || cacheIsStale(cacheInfo.ModTime(), systemPathFile, pathSourceDir)
	if !needsRebuild {
		dispatchLogger.InfoContext(ctx, "path cache up to date, skipping")
		return nil
	}

	output, err := cmdexec.OutputWithLogger(ctx, dispatchLogger, "/usr/libexec/path_helper", "-s")
	if err != nil {
		slog.WarnContext(ctx, "pathcache: running path_helper", slog.Any("error", err))
		return fmt.Errorf("running path_helper: %w", err)
	}
	if err := os.WriteFile(filepath.Clean(cacheFile), []byte(output), 0o600); err != nil {
		slog.WarnContext(ctx, "pathcache: writing path cache", slog.Any("error", err))
		return fmt.Errorf("writing path cache: %w", err)
	}
	return nil
}
