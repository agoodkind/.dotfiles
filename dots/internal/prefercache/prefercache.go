// Package prefercache implements prefer-alias cache management.
package prefercache

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	configassets "goodkind.io/.dotfiles/config"
	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/telemetry"
	"goodkind.io/.dotfiles/internal/util"
)

// Rebuild regenerates the prefer-alias cache if it is stale or force is true.
func Rebuild(ctx context.Context, dotfiles string, force bool, cacheLogger *telemetry.Logger) error {
	cfg := catalog.DefaultPreferCacheConfig()
	if cfg == nil {
		return nil
	}
	cacheFile := util.ResolveConfigPath(cfg.CacheFile, dotfiles)
	invalidateFile := util.ResolveConfigPath(cfg.InvalidateFile, dotfiles)

	sourceFiles := make([]string, 0, len(cfg.SourceFiles))
	for _, sourceFile := range cfg.SourceFiles {
		sourceFiles = append(sourceFiles, util.ResolveConfigPath(sourceFile, dotfiles))
	}

	if !shouldRebuildPreferCache(cacheFile, invalidateFile, sourceFiles, force) {
		if cacheLogger != nil {
			cacheLogger.InfoContext(ctx, "prefer cache up to date, skipping")
		}
		return nil
	}

	script, err := configassets.RenderTemplate("prefercache-bootstrap.zsh.tmpl", map[string]string{
		"Dotfiles": dotfiles,
	})
	if err != nil {
		slog.ErrorContext(ctx, "prefercache: Rebuild: rendering template", "err", err)
		return fmt.Errorf("rendering prefer cache template: %w", err)
	}
	_, err = cmdexec.OutputWithLoggerAndEnv(
		ctx,
		cacheLogger,
		append(os.Environ(), "DOTDOTFILES="+dotfiles),
		"zsh",
		"-c",
		script,
	)
	if err != nil {
		slog.ErrorContext(ctx, "prefercache: Rebuild: running bootstrap", "err", err)
		return fmt.Errorf("running prefer cache bootstrap: %w", err)
	}
	return nil
}

func shouldRebuildPreferCache(cachePath, marker string, sourceFiles []string, force bool) bool {
	if force {
		return true
	}
	if _, err := os.Stat(marker); err == nil {
		return true
	}
	cacheInfo, err := os.Stat(cachePath)
	if err != nil {
		return true
	}
	for _, path := range sourceFiles {
		info, err := os.Stat(path)
		if err == nil && info.ModTime().After(cacheInfo.ModTime()) {
			return true
		}
	}
	return false
}
