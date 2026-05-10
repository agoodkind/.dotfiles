// Package syncer implements the Cursor configuration sync workflow.
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

// Run executes the Cursor rule sync workflow, returning an error if any step fails.
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

type ruleUploadResult struct {
	skipped  bool
	readErr  bool
	title    string
	uploaded bool
}

func uploadRuleFile(token, apiBase, workspaceURL, ruleFile string, attemptNum, totalRules int) ruleUploadResult {
	resolvedFile := rules.ResolveRuleFile(ruleFile)
	displayFile := rules.FormatRuleSource(ruleFile)
	if _, err := os.Stat(resolvedFile); err != nil {
		return ruleUploadResult{skipped: true, readErr: false, title: "", uploaded: false}
	}

	rawBody, readErr := os.ReadFile(resolvedFile)
	if readErr != nil {
		logging.ErrorWithErr("Read rule file failed", readErr)
		return ruleUploadResult{skipped: false, readErr: true, title: "", uploaded: false}
	}
	title := ruleTitleFromPath(resolvedFile)
	body := rules.ParseMdcContent(string(rawBody))
	payload := buildRulePayload(title, body)

	logging.Debug(fmt.Sprintf("Uploading rule %d/%d: %s", attemptNum, totalRules, title))
	logging.Debug("Source: " + displayFile)
	logging.Debug(fmt.Sprintf("Size: %d bytes", len(payload)))

	success, response := cursorapi.AddRule(token, apiBase, workspaceURL, title, payload)
	if success {
		if response != "" {
			logging.Debug("Response: " + response)
		}
		logging.Debug("Status: uploaded")
		return ruleUploadResult{skipped: false, readErr: false, title: title, uploaded: true}
	}
	logging.Debug(fmt.Sprintf("Upload failed for %s: %s", title, response))
	return ruleUploadResult{skipped: false, readErr: false, title: title, uploaded: false}
}

// SyncRules uploads local rule files to the Cursor workspace, returning the number of failures.
func SyncRules() int {
	cfg := config.BuildSyncConfig()
	logging.Configure()

	logging.Info("Cursor rule sync start")

	if _, err := os.Stat(cfg.CursorDB); err != nil {
		logging.ErrorWithErr("Cursor database not found: "+cfg.CursorDB, err)
		logging.Info("Make sure Cursor is installed and you're logged in.")
		return 1
	}

	if err := rules.ValidateRuleDirectories(cfg.RuleDirectories); err != nil {
		logging.ErrorWithErr("Cursor rule directories unavailable", err)
		return 1
	}
	logging.Info("API endpoint: " + cfg.APIBase)

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
		logging.Info("No .mdc files found in: " + strings.Join(cfg.RuleDirectories, ", "))
		return 0
	}

	logging.Info(fmt.Sprintf("Uploading %d rule(s) to cloud...", len(localRuleFiles)))
	attempted, succeeded, failed, uploadedTitles, failedUploads := doUploadRules(token, cfg.APIBase, cfg.WorkspaceURL, localRuleFiles)
	if len(failedUploads) > 0 {
		logging.Info("Upload failures: " + strings.Join(failedUploads, ", "))
	}
	if len(uploadedTitles) > 0 {
		logging.Debug("Uploaded rules: " + strings.Join(uploadedTitles, ", "))
	}

	logging.Info("Verifying uploaded rules...")
	remoteRules := cursorapi.ListRules(token, cfg.APIBase)
	remoteByTitle := buildRemoteRuleIndex(remoteRules)
	verified, verifyFailed, missingTitles, mismatchedTitles := doVerifyRules(localRuleFiles, remoteByTitle)

	if len(missingTitles) > 0 {
		logging.Info("Verification missing rules: " + strings.Join(missingTitles, ", "))
	}
	if len(mismatchedTitles) > 0 {
		logging.Info("Verification content mismatches: " + strings.Join(mismatchedTitles, ", "))
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

func doUploadRules(token, apiBase, workspaceURL string, localRuleFiles []string) (int, int, int, []string, []string) {
	var attempted, succeeded, failed int
	var uploadedTitles, failedUploads []string
	totalRules := len(localRuleFiles)
	for i, ruleFile := range localRuleFiles {
		res := uploadRuleFile(token, apiBase, workspaceURL, ruleFile, i+1, totalRules)
		if res.skipped {
			continue
		}
		if res.readErr {
			failed++
			continue
		}
		attempted++
		if !res.uploaded {
			failed++
			failedUploads = append(failedUploads, res.title)
			continue
		}
		succeeded++
		uploadedTitles = append(uploadedTitles, res.title)
	}
	return attempted, succeeded, failed, uploadedTitles, failedUploads
}

func doVerifyRules(localRuleFiles []string, remoteByTitle map[string]models.RuleRecord) (int, int, []string, []string) {
	var verified, verifyFailed int
	var missingTitles, mismatchedTitles []string
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
			missingTitles = append(missingTitles, title)
			verifyFailed++
			continue
		}
		if remoteRule["content"] != expectedContent {
			mismatchedTitles = append(mismatchedTitles, title)
			logging.Debug(fmt.Sprintf("Expected %d bytes, got %d bytes", len(expectedContent), len(remoteRule["content"])))
			verifyFailed++
			continue
		}
		logging.Debug(title + ": content verified")
		verified++
	}
	return verified, verifyFailed, missingTitles, mismatchedTitles
}
