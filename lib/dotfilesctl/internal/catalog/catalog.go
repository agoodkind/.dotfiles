package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/pelletier/go-toml/v2"
)

type PackageConfig struct {
	COMMON_PACKAGES    []string          `toml:"common_packages"`
	APT_SPECIFIC       []string          `toml:"apt_specific"`
	SNAP_PACKAGES      []string          `toml:"snap_packages"`
	BREW_SPECIFIC      []string          `toml:"brew_specific"`
	BREW_CASKS         map[string]string `toml:"brew_casks"`
	GO_PACKAGES        map[string]string `toml:"go_packages"`
	CARGO_PACKAGES     []string          `toml:"cargo_packages"`
	CARGO_GIT_PACKAGES map[string]string `toml:"cargo_git_packages"`
}

type ToolDeclaration struct {
	ID   string `toml:"id"`
	Bin  string `toml:"bin"`
	Repo string `toml:"repo"`
}

type DispatchWorker struct {
	Name    string `toml:"name"`
	Enabled bool   `toml:"enabled"`
}

type DispatchConfig struct {
	Workers            []DispatchWorker `toml:"workers"`
	StatusDir          string           `toml:"status_dir"`
	LockFile           string           `toml:"lock_file"`
	LogPath            string           `toml:"log_path"`
	WeeklyUpdateHours  int64            `toml:"weekly_update_hours"`
	WeeklyUpdateMarker string           `toml:"weekly_update_marker"`
}

type MacPatchConfig struct {
	Enabled      bool   `toml:"enabled"`
	Sentinel     string `toml:"sentinel"`
	ZProfilePath string `toml:"zprofile_path"`
	ZshrcPath    string `toml:"zshrc_path"`
	PatchScript  string `toml:"patch_script"`
}

type PreferCacheConfig struct {
	CacheFile      string   `toml:"cache_file"`
	InvalidateFile string   `toml:"invalidate_file"`
	SourceFiles    []string `toml:"source_files"`
}

type catalogDocument struct {
	Packages    PackageConfig     `toml:"packages"`
	Tools       []ToolDeclaration `toml:"tool"`
	Dispatch    DispatchConfig    `toml:"dispatch"`
	MacPatch    MacPatchConfig    `toml:"mac_patch"`
	PreferCache PreferCacheConfig `toml:"prefer_cache"`
}

var (
	loadOnce sync.Once
	mu       sync.RWMutex
	cached   catalogDocument
	loadErr  error
)

func DefaultPackageConfig() *PackageConfig {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if loadErr != nil {
		return &PackageConfig{}
	}
	source := cached.Packages
	duplicated := &PackageConfig{
		COMMON_PACKAGES:    append([]string{}, source.COMMON_PACKAGES...),
		APT_SPECIFIC:       append([]string{}, source.APT_SPECIFIC...),
		SNAP_PACKAGES:      append([]string{}, source.SNAP_PACKAGES...),
		BREW_SPECIFIC:      append([]string{}, source.BREW_SPECIFIC...),
		CARGO_PACKAGES:     append([]string{}, source.CARGO_PACKAGES...),
		BREW_CASKS:         make(map[string]string, len(source.BREW_CASKS)),
		GO_PACKAGES:        make(map[string]string, len(source.GO_PACKAGES)),
		CARGO_GIT_PACKAGES: make(map[string]string, len(source.CARGO_GIT_PACKAGES)),
	}
	for key, value := range source.BREW_CASKS {
		duplicated.BREW_CASKS[key] = value
	}
	for key, value := range source.GO_PACKAGES {
		duplicated.GO_PACKAGES[key] = value
	}
	for key, value := range source.CARGO_GIT_PACKAGES {
		duplicated.CARGO_GIT_PACKAGES[key] = value
	}
	return duplicated
}

func DefaultToolDeclarations() []ToolDeclaration {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if loadErr != nil {
		return nil
	}
	tools := make([]ToolDeclaration, 0, len(cached.Tools))
	for _, tool := range cached.Tools {
		tools = append(tools, tool)
	}
	return tools
}

func DefaultDispatchConfig() *DispatchConfig {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if loadErr != nil {
		return &DispatchConfig{
			StatusDir:          "$HOME/.cache/dotfiles_dispatch.lock",
			LockFile:           "$HOME/.cache/dotfiles_dispatch.flock",
			LogPath:            "$HOME/.cache/dotfiles_dispatch.log",
			WeeklyUpdateHours:  168,
			WeeklyUpdateMarker: "$HOME/.cache/dotfiles_weekly_update",
			Workers: []DispatchWorker{
				{Name: "updater", Enabled: true},
				{Name: "prefer-cache-rebuild", Enabled: true},
				{Name: "path-cache-rebuild", Enabled: true},
				{Name: "zwc-recompile", Enabled: true},
				{Name: "ssh-key-load-mac", Enabled: true},
			},
		}
	}
	source := cached.Dispatch
	copyWorkers := make([]DispatchWorker, 0, len(source.Workers))
	for _, worker := range source.Workers {
		copyWorkers = append(copyWorkers, worker)
	}
	return &DispatchConfig{
		Workers:            copyWorkers,
		StatusDir:          source.StatusDir,
		LockFile:           source.LockFile,
		LogPath:            source.LogPath,
		WeeklyUpdateHours:  source.WeeklyUpdateHours,
		WeeklyUpdateMarker: source.WeeklyUpdateMarker,
	}
}

func DefaultMacPatchConfig() *MacPatchConfig {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if loadErr != nil {
		return &MacPatchConfig{
			Enabled:      true,
			Sentinel:     "# DOTFILES_PERF_PATCH_V4",
			ZProfilePath: "/etc/zprofile",
			ZshrcPath:    "/etc/zshrc",
			PatchScript:  "$DOTDOTFILES/bash/setup/platform/patch-etc-zsh.bash",
		}
	}
	source := cached.MacPatch
	return &MacPatchConfig{
		Enabled:      source.Enabled,
		Sentinel:     source.Sentinel,
		ZProfilePath: source.ZProfilePath,
		ZshrcPath:    source.ZshrcPath,
		PatchScript:  source.PatchScript,
	}
}

func DefaultPreferCacheConfig() *PreferCacheConfig {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if loadErr != nil {
		return &PreferCacheConfig{
			CacheFile:      "$HOME/.cache/zsh_prefer_aliases.zsh",
			InvalidateFile: "$HOME/.cache/zsh_prefer_invalidate",
			SourceFiles: []string{
				"$DOTDOTFILES/zshrc/commands/prefer.zsh",
				"$DOTDOTFILES/home/.zshrc",
				"$DOTDOTFILES/.zshrc.local",
			},
		}
	}
	source := cached.PreferCache
	copySourceFiles := make([]string, 0, len(source.SourceFiles))
	for _, item := range source.SourceFiles {
		copySourceFiles = append(copySourceFiles, item)
	}
	return &PreferCacheConfig{
		CacheFile:      source.CacheFile,
		InvalidateFile: source.InvalidateFile,
		SourceFiles:    copySourceFiles,
	}
}

func loadCatalog() {
	loadOnce.Do(func() {
		catalogPath := resolveCatalogPath()
		data, err := os.ReadFile(catalogPath)
		if err != nil {
			loadErr = fmt.Errorf("read catalog file: %w", err)
			return
		}
		var doc catalogDocument
		if err := toml.Unmarshal(data, &doc); err != nil {
			loadErr = fmt.Errorf("parse catalog file: %w", err)
			return
		}
		mu.Lock()
		cached = doc
		loadErr = nil
		mu.Unlock()
	})
}

func resolveCatalogPath() string {
	if explicit := os.Getenv("DOTFILES_CATALOG_PATH"); explicit != "" {
		return explicit
	}
	if dotfiles := os.Getenv("DOTDOTFILES"); dotfiles != "" {
		return filepath.Join(dotfiles, "lib", "dotfilesctl", "config", "catalog.toml")
	}
	return filepath.Join(os.Getenv("HOME"), ".dotfiles", "lib", "dotfilesctl", "config", "catalog.toml")
}
