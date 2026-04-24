package prefercache

import (
	"context"
	"os"

	"github.com/agoodkind/.dotfiles/internal/catalog"
	configassets "github.com/agoodkind/.dotfiles/config"
	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
	"github.com/agoodkind/.dotfiles/internal/util"
)

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
			cacheLogger.Info("prefer cache up to date, skipping")
		}
		return nil
	}

	script, err := configassets.RenderTemplate("prefercache-bootstrap.zsh.tmpl", map[string]any{
		"Dotfiles": dotfiles,
	})
	if err != nil {
		return err
	}
	_, err = cmdexec.OutputWithLoggerAndEnv(
		context.Background(),
		cacheLogger,
		append(os.Environ(), "DOTDOTFILES="+dotfiles),
		"zsh",
		"-c",
		script,
	)
	return err
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
