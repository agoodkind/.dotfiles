package pathcache

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/telemetry"
)

func Rebuild(dispatchLogger *telemetry.Logger) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	if _, err := os.Stat("/usr/libexec/path_helper"); err != nil {
		return nil
	}

	cacheDir := filepath.Join(os.Getenv("HOME"), ".cache", "zsh_startup")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	cacheFile := filepath.Join(cacheDir, "path_cache.zsh")

	cacheInfo, err := os.Stat(cacheFile)
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
		dispatchLogger.Info("path cache up to date, skipping")
		return nil
	}

	output, err := cmdexec.OutputWithLogger(context.Background(), dispatchLogger, "/usr/libexec/path_helper", "-s")
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFile, []byte(output), 0o644)
}
