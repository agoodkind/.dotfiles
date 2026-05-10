// Package config provides Cursor editor configuration management.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"goodkind.io/.dotfiles/internal/cursor/models"
)

func splitRuleDirectories(rawRuleDirs string) []string {
	var directories []string
	for rawRuleDir := range strings.SplitSeq(rawRuleDirs, ":") {
		trimmedRuleDir := strings.TrimSpace(rawRuleDir)
		if trimmedRuleDir == "" {
			continue
		}
		directories = append(directories, trimmedRuleDir)
	}
	return directories
}

func loadRuleDirectories() []string {
	rawDefaultRuleDir := os.Getenv("CURSOR_RULES_DIR")
	if rawDefaultRuleDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			rawDefaultRuleDir = ".dotfiles/.agents/rules"
		} else {
			rawDefaultRuleDir = filepath.Join(home, ".dotfiles/.agents/rules")
		}
	}

	extraRuleDirs := splitRuleDirectories(os.Getenv("CURSOR_EXTRA_RULE_DIRS"))
	return append([]string{rawDefaultRuleDir}, extraRuleDirs...)
}

// BuildSyncConfig assembles a SyncConfig from environment variables and defaults.
func BuildSyncConfig() models.SyncConfig {
	workspaceURL := os.Getenv("DEFAULT_RULE_URL")
	if workspaceURL == "" {
		workspaceURL = "https://github.com/agoodkind/.dotfiles"
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		home = "."
	}

	return models.SyncConfig{
		CursorDB:        filepath.Join(home, "Library/Application Support/Cursor/User/globalStorage/state.vscdb"),
		APIBase:         "https://api2.cursor.sh/aiserver.v1.AiService",
		WorkspaceURL:    workspaceURL,
		RuleDirectories: loadRuleDirectories(),
	}
}
