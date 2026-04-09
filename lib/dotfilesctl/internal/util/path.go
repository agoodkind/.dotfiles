package util

import "os"

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
