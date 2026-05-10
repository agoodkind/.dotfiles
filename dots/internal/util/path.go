// Package util provides path and filesystem utility functions.
package util

import "os"

// ResolveConfigPath expands environment variables in value, substituting dotfiles for $DOTDOTFILES.
func ResolveConfigPath(value string, dotfiles string) string {
	if value == "" {
		return value
	}
	return os.Expand(value, func(key string) string {
		if key == "DOTDOTFILES" {
			return dotfiles
		}
		return os.Getenv(key)
	})
}
