// Package config provides Cursor editor configuration management.
package config

import (
	"os"
	"path/filepath"

	"goodkind.io/.dotfiles/internal/cursor/constants"
	"goodkind.io/.dotfiles/internal/cursor/models"
)

// BuildSyncConfig assembles a SyncConfig from environment variables and defaults.
func BuildSyncConfig() models.SyncConfig {
	workspaceURL := os.Getenv("DEFAULT_RULE_URL")
	if workspaceURL == "" {
		workspaceURL = constants.DefaultWorkspaceURL
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		home = "."
	}

	return models.SyncConfig{
		CursorDB:     filepath.Join(home, "Library/Application Support/Cursor/User/globalStorage/state.vscdb"),
		APIBase:      constants.APIBase,
		WorkspaceURL: workspaceURL,
	}
}
