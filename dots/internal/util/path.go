// Package util provides path and filesystem utility functions.
package util

import (
	"os"
	"path/filepath"
)

// ResolveConfigPath expands environment variables in value, substituting dotfiles
// for $DOTDOTFILES. It also supplies XDG base-directory defaults, so config may
// use $XDG_CACHE_HOME and friends and still resolve on hosts that do not set them
// (e.g. $XDG_CACHE_HOME falls back to $HOME/.cache, leaving paths unchanged there).
func ResolveConfigPath(value string, dotfiles string) string {
	if value == "" {
		return value
	}
	home := os.Getenv("HOME")
	xdgDefaults := map[string]string{
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
	}
	return os.Expand(value, func(key string) string {
		if key == "DOTDOTFILES" {
			return dotfiles
		}
		if fallback, ok := xdgDefaults[key]; ok {
			if value := os.Getenv(key); value != "" {
				return value
			}
			return fallback
		}
		return os.Getenv(key)
	})
}
