// Package pathcache implements caching of shell PATH entries.
package pathcache

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/telemetry"
)

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
	needsRebuild := err != nil
	if !needsRebuild {
		systemPathInfo, systemErr := os.Stat("/etc/paths")
		if systemErr == nil && systemPathInfo.ModTime().After(cacheInfo.ModTime()) {
			needsRebuild = true
		}
	}
	if !needsRebuild {
		entries, err := os.ReadDir("/etc/paths.d")
		if err == nil {
			for _, entry := range entries {
				entryPath := filepath.Join("/etc/paths.d", entry.Name())
				info, err := os.Stat(entryPath)
				if err == nil && info.ModTime().After(cacheInfo.ModTime()) {
					needsRebuild = true
					break
				}
			}
		}
	}
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
