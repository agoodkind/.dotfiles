package syncer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goodkind.io/.dotfiles/internal/cursor/config"
	"goodkind.io/.dotfiles/internal/cursor/cursorapi"
	"goodkind.io/.dotfiles/internal/cursor/logging"
	"goodkind.io/.dotfiles/internal/cursor/models"
	"goodkind.io/.dotfiles/internal/cursor/rules"
)

func Run() error {
	if SyncRules() != 0 {
		return fmt.Errorf("cursor rule sync failed")
	}
	return nil
}

func buildRulePayload(title string, body string) string {
	return title + "\n\n" + body
}

func buildRemoteRuleIndex(remoteRules []models.RuleRecord) map[string]models.RuleRecord {
	index := map[string]models.RuleRecord{}
	for _, remoteRule := range remoteRules {
		index[remoteRule["title"]] = remoteRule
	}
	return index
}

func ruleTitleFromPath(resolvedFile string) string {
	return strings.TrimSuffix(filepath.Base(resolvedFile), filepath.Ext(resolvedFile))
}

func SyncRules() int {
	cfg := config.BuildSyncConfig()
	logging.Configure()

	logging.Info("Cursor rule sync start")

	if _, err := os.Stat(cfg.CursorDB); err != nil {
		logging.Error(fmt.Sprintf("Cursor database not found: %s", cfg.CursorDB))
		logging.Error("Make sure Cursor is installed and you're logged in.")
		return 1
	}

	rules.ValidateRuleDirectories(cfg.RuleDirectories)
	logging.Info(fmt.Sprintf("API endpoint: %s", cfg.APIBase))

	logging.Info("Authenticating...")
	token := cursorapi.GetCursorAuthToken(cfg.CursorDB)
	if token == "" {
		logging.Info("Authentication failed. Make sure you're logged into Cursor.")
		return 1
	}
	logging.Info("Authentication successful")

	logging.Info("Clearing existing cloud rules...")
	cursorapi.DeleteAllRules(token, cfg.APIBase)

	localRuleFiles := rules.CollectRuleFiles(cfg.RuleDirectories)
	if len(localRuleFiles) == 0 {
		logging.Info(fmt.Sprintf("No .mdc files found in: %s", strings.Join(cfg.RuleDirectories, ", ")))
		return 0
	}

	totalRules := len(localRuleFiles)
	logging.Info(fmt.Sprintf("Uploading %d rule(s) to cloud...", totalRules))

	attempted := 0
	succeeded := 0
	failed := 0
	for _, ruleFile := range localRuleFiles {
		resolvedFile := rules.ResolveRuleFile(ruleFile)
		displayFile := rules.FormatRuleSource(ruleFile)
		if _, err := os.Stat(resolvedFile); err != nil {
			continue
		}

		rawBody, readErr := os.ReadFile(resolvedFile)
		if readErr != nil {
			logging.Error(readErr.Error())
			failed++
			continue
		}
		title := ruleTitleFromPath(resolvedFile)
		body := rules.ParseMdcContent(string(rawBody))
		payload := buildRulePayload(title, body)
		attempted++

		logging.Info(fmt.Sprintf("[%d/%d] uploading %s", attempted, totalRules, title))
		logging.Debug("Source: " + displayFile)
		logging.Debug(fmt.Sprintf("Size: %d bytes", len(payload)))

		success, response := cursorapi.AddRule(
			token,
			cfg.APIBase,
			cfg.WorkspaceURL,
			title,
			payload,
		)
		if success {
			succeeded++
			if response != "" {
				logging.Debug("Response: " + response)
			}
			logging.Debug("Status: uploaded")
		} else {
			failed++
			logging.Info(fmt.Sprintf("Upload failed for %s: %s", title, response))
		}
	}

	logging.Info("Verifying uploaded rules...")
	remoteRules := cursorapi.ListRules(token, cfg.APIBase)
	remoteByTitle := buildRemoteRuleIndex(remoteRules)

	verified := 0
	verifyFailed := 0
	for _, ruleFile := range localRuleFiles {
		resolvedFile := rules.ResolveRuleFile(ruleFile)
		if _, err := os.Stat(resolvedFile); err != nil {
			continue
		}
		rawBody, readErr := os.ReadFile(resolvedFile)
		if readErr != nil {
			verifyFailed++
			continue
		}
		title := ruleTitleFromPath(resolvedFile)
		expectedContent := buildRulePayload(title, rules.ParseMdcContent(string(rawBody)))
		remoteRule, exists := remoteByTitle[title]
		if !exists {
			logging.Info("Verification failed, missing rule: " + title)
			verifyFailed++
			continue
		}
		if remoteRule["content"] != expectedContent {
			logging.Info("Verification failed, content mismatch: " + title)
			logging.Debug(fmt.Sprintf("Expected %d bytes, got %d bytes", len(expectedContent), len(remoteRule["content"])))
			verifyFailed++
			continue
		}
		logging.Debug(fmt.Sprintf("%s: content verified", title))
		verified++
	}

	if verifyFailed == 0 {
		logging.Info(fmt.Sprintf("Verification passed for %d rule(s)", verified))
	} else {
		logging.Info(fmt.Sprintf("Verification failed for %d rule(s)", verifyFailed))
	}

	if failed == 0 && verifyFailed == 0 {
		logging.Info(fmt.Sprintf("Sync complete: %d rule(s) uploaded and verified", succeeded))
		return 0
	}
	if failed > 0 {
		logging.Info(fmt.Sprintf("Sync completed with errors: %d/%d succeeded, %d failed", succeeded, attempted, failed))
		return 1
	}
	logging.Info(fmt.Sprintf("Sync completed but %d rule(s) failed content verification", verifyFailed))
	return 1
}
