// Package catalog implements the configuration catalog for dotfiles sync targets.
package catalog

import (
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sync"

	"github.com/pelletier/go-toml/v2"
)

// PackageConfig holds the package lists for each supported package manager.
type PackageConfig struct {
	CommonPackages   []string          `toml:"common_packages"`
	AptSpecific      []string          `toml:"apt_specific"`
	UbuntuPPAs       []string          `toml:"ubuntu_ppas"`
	SnapPackages     []string          `toml:"snap_packages"`
	BrewSpecific     []string          `toml:"brew_specific"`
	BrewCasks        map[string]string `toml:"brew_casks"`
	GoPackages       map[string]string `toml:"go_packages"`
	CargoPackages    []string          `toml:"cargo_packages"`
	CargoGitPackages map[string]string `toml:"cargo_git_packages"`
}

// ToolDeclaration describes a custom tool to install or upgrade.
type ToolDeclaration struct {
	// ID is the canonical tool identifier (matches the catalog.toml [[tool]] id field).
	ID string `toml:"id"`
	// Bin is the binary name used to invoke the tool and check its version.
	Bin string `toml:"bin"`
	// Repo is the GitHub repository slug (owner/name) for version and release lookup.
	Repo string `toml:"repo"`
	// InstallMethod is one of "script", "cargo", or "github-release".
	InstallMethod string `toml:"install_method"`
	// Platforms restricts installation to the listed GOOS values. Empty means all platforms.
	Platforms []string `toml:"platforms"`
	// ScriptURL is the URL of the install script (used when InstallMethod is "script").
	ScriptURL string `toml:"script_url"`
	// ScriptArgs are extra arguments passed to the install script after the script path.
	ScriptArgs []string `toml:"script_args"`
	// ArchiveExt is the file extension of the release asset (e.g. ".tar.gz", ".zip", ".gz", ".deb").
	ArchiveExt string `toml:"archive_ext"`
	// OSDarwin is the OS tag embedded in the release asset filename when running on macOS.
	OSDarwin string `toml:"os_darwin"`
	// OSLinux is the OS tag embedded in the release asset filename when running on Linux.
	OSLinux string `toml:"os_linux"`
	// ArchAMD64 is the architecture tag embedded in the release asset filename for amd64.
	ArchAMD64 string `toml:"arch_amd64"`
	// ArchARM64 is the architecture tag embedded in the release asset filename for arm64.
	ArchARM64 string `toml:"arch_arm64"`
	// CrateName is the crates.io crate name (used when InstallMethod is "cargo").
	CrateName string `toml:"crate_name"`
	// Version pins a tool when the latest upstream version is not compatible with the bootstrap toolchain.
	Version string `toml:"version"`
}

// DispatchWorker represents a named background worker entry in the dispatch config.
type DispatchWorker struct {
	Name    string `toml:"name"`
	Enabled bool   `toml:"enabled"`
}

// DispatchConfig holds configuration for the background dispatch system.
type DispatchConfig struct {
	Workers            []DispatchWorker `toml:"workers"`
	StatusDir          string           `toml:"status_dir"`
	LockFile           string           `toml:"lock_file"`
	LogPath            string           `toml:"log_path"`
	WeeklyUpdateHours  int64            `toml:"weekly_update_hours"`
	WeeklyUpdateMarker string           `toml:"weekly_update_marker"`
}

// MacPatchConfig holds configuration for the macOS /etc/zsh patch.
type MacPatchConfig struct {
	Enabled      bool   `toml:"enabled"`
	Sentinel     string `toml:"sentinel"`
	ZProfilePath string `toml:"zprofile_path"`
	ZshrcPath    string `toml:"zshrc_path"`
	PatchScript  string `toml:"patch_script"`
}

// PreferCacheConfig holds configuration for the prefer-alias cache.
type PreferCacheConfig struct {
	CacheFile      string   `toml:"cache_file"`
	InvalidateFile string   `toml:"invalidate_file"`
	SourceFiles    []string `toml:"source_files"`
}

// MacConfig holds macOS defaults applied during OS setup.
type MacConfig struct {
	ScreenshotDir string `toml:"screenshot_dir"`
}

type catalogDocument struct {
	Packages    PackageConfig     `toml:"packages"`
	Tools       []ToolDeclaration `toml:"tool"`
	Dispatch    DispatchConfig    `toml:"dispatch"`
	MacPatch    MacPatchConfig    `toml:"mac_patch"`
	Macos       MacConfig         `toml:"macos"`
	PreferCache PreferCacheConfig `toml:"prefer_cache"`
}

var (
	loadOnce sync.Once
	mu       sync.RWMutex
	cached   catalogDocument
	errLoad  error
)

// DefaultPackageConfig returns the package config from the catalog, or a zero-value config on load error.
func DefaultPackageConfig() *PackageConfig {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if errLoad != nil {
		return &PackageConfig{
			CommonPackages:   nil,
			AptSpecific:      nil,
			UbuntuPPAs:       nil,
			SnapPackages:     nil,
			BrewSpecific:     nil,
			BrewCasks:        nil,
			GoPackages:       nil,
			CargoPackages:    nil,
			CargoGitPackages: nil,
		}
	}
	source := cached.Packages
	duplicated := &PackageConfig{
		CommonPackages:   append([]string{}, source.CommonPackages...),
		AptSpecific:      append([]string{}, source.AptSpecific...),
		UbuntuPPAs:       append([]string{}, source.UbuntuPPAs...),
		SnapPackages:     append([]string{}, source.SnapPackages...),
		BrewSpecific:     append([]string{}, source.BrewSpecific...),
		CargoPackages:    append([]string{}, source.CargoPackages...),
		BrewCasks:        make(map[string]string, len(source.BrewCasks)),
		GoPackages:       make(map[string]string, len(source.GoPackages)),
		CargoGitPackages: make(map[string]string, len(source.CargoGitPackages)),
	}
	maps.Copy(duplicated.BrewCasks, source.BrewCasks)
	maps.Copy(duplicated.GoPackages, source.GoPackages)
	maps.Copy(duplicated.CargoGitPackages, source.CargoGitPackages)
	return duplicated
}

// DefaultToolDeclarations returns the tool declarations from the catalog, or nil on load error.
func DefaultToolDeclarations() []ToolDeclaration {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if errLoad != nil {
		return nil
	}
	tools := make([]ToolDeclaration, 0, len(cached.Tools))
	tools = append(tools, cached.Tools...)
	return tools
}

// DefaultDispatchConfig returns the dispatch config from the catalog, or a built-in default on load error.
func DefaultDispatchConfig() *DispatchConfig {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if errLoad != nil {
		return &DispatchConfig{
			Workers:            nil,
			StatusDir:          "",
			LockFile:           "",
			LogPath:            "",
			WeeklyUpdateHours:  0,
			WeeklyUpdateMarker: "",
		}
	}
	source := cached.Dispatch
	copyWorkers := make([]DispatchWorker, 0, len(source.Workers))
	copyWorkers = append(copyWorkers, source.Workers...)
	return &DispatchConfig{
		Workers:            copyWorkers,
		StatusDir:          source.StatusDir,
		LockFile:           source.LockFile,
		LogPath:            source.LogPath,
		WeeklyUpdateHours:  source.WeeklyUpdateHours,
		WeeklyUpdateMarker: source.WeeklyUpdateMarker,
	}
}

// DefaultMacPatchConfig returns the macOS patch config from the catalog, or a built-in default on load error.
func DefaultMacPatchConfig() *MacPatchConfig {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if errLoad != nil {
		return &MacPatchConfig{
			Enabled:      false,
			Sentinel:     "",
			ZProfilePath: "",
			ZshrcPath:    "",
			PatchScript:  "",
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

// DefaultMacConfig returns the macOS config from the catalog, or a zero-value config on load error.
func DefaultMacConfig() *MacConfig {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if errLoad != nil {
		return &MacConfig{ScreenshotDir: ""}
	}
	return &MacConfig{ScreenshotDir: cached.Macos.ScreenshotDir}
}

// DefaultPreferCacheConfig returns the prefer-cache config from the catalog, or a built-in default on load error.
func DefaultPreferCacheConfig() *PreferCacheConfig {
	loadCatalog()
	mu.RLock()
	defer mu.RUnlock()
	if errLoad != nil {
		return &PreferCacheConfig{
			CacheFile:      "",
			InvalidateFile: "",
			SourceFiles:    nil,
		}
	}
	source := cached.PreferCache
	copySourceFiles := make([]string, 0, len(source.SourceFiles))
	copySourceFiles = append(copySourceFiles, source.SourceFiles...)
	return &PreferCacheConfig{
		CacheFile:      source.CacheFile,
		InvalidateFile: source.InvalidateFile,
		SourceFiles:    copySourceFiles,
	}
}

func loadCatalog() {
	loadOnce.Do(func() {
		dir := resolveConfigDir()
		var merged catalogDocument
		sections := []struct {
			file  string
			apply func(target *catalogDocument, parsed catalogDocument)
		}{
			{"packages.toml", func(target *catalogDocument, parsed catalogDocument) { target.Packages = parsed.Packages }},
			{"tools.toml", func(target *catalogDocument, parsed catalogDocument) { target.Tools = parsed.Tools }},
			{"dispatch.toml", func(target *catalogDocument, parsed catalogDocument) {
				target.Dispatch = parsed.Dispatch
				target.PreferCache = parsed.PreferCache
			}},
			{"platform-macos.toml", func(target *catalogDocument, parsed catalogDocument) {
				target.MacPatch = parsed.MacPatch
				target.Macos = parsed.Macos
			}},
		}
		for _, section := range sections {
			path := filepath.Join(dir, section.file)
			data, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				slog.Error("catalog: reading config file", "path", path, "err", err)
				errLoad = fmt.Errorf("read config file %s: %w", section.file, err)
				return
			}
			var parsed catalogDocument
			if err := toml.Unmarshal(data, &parsed); err != nil {
				slog.Error("catalog: parsing config file", "path", path, "err", err)
				errLoad = fmt.Errorf("parse config file %s: %w", section.file, err)
				return
			}
			section.apply(&merged, parsed)
		}
		mu.Lock()
		cached = merged
		errLoad = nil
		mu.Unlock()
	})
}

// resolveConfigDir returns the directory holding the split config TOMLs,
// honoring DOTFILES_CONFIG_DIR, then $DOTDOTFILES/config, then ~/.dotfiles/config.
// The config is committed alongside the code, so it is always present; a missing
// file surfaces as a logged load error rather than a silent fallback.
func resolveConfigDir() string {
	if explicit := os.Getenv("DOTFILES_CONFIG_DIR"); explicit != "" {
		return explicit
	}
	if dotfiles := os.Getenv("DOTDOTFILES"); dotfiles != "" {
		return filepath.Join(dotfiles, "config")
	}
	return filepath.Join(os.Getenv("HOME"), ".dotfiles", "config")
}
