// Package syncer implements the Cursor configuration sync workflow.
package syncer

import (
	"fmt"
	"os"
	"strings"

	"goodkind.io/.dotfiles/internal/cursor/config"
	"goodkind.io/.dotfiles/internal/cursor/cursorapi"
	"goodkind.io/.dotfiles/internal/cursor/logging"
	"goodkind.io/.dotfiles/internal/cursor/models"
	"goodkind.io/.dotfiles/internal/sync/compilation"
)

// Run executes the Cursor rule sync workflow, returning an error if any step fails.
func Run(rules []compilation.RenderedRule) error {
	if SyncRules(rules) != 0 {
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

type ruleUploadResult struct {
	readErr  bool
	title    string
	uploaded bool
}

func uploadRule(token, apiBase, workspaceURL string, rule compilation.RenderedRule, attemptNum, totalRules int) ruleUploadResult {
	payload := buildRulePayload(rule.Title, rule.Body)

	logging.Debug(fmt.Sprintf("Uploading rule %d/%d: %s", attemptNum, totalRules, rule.Title))
	logging.Debug(fmt.Sprintf("Size: %d bytes", len(payload)))

	success, response := cursorapi.AddRule(token, apiBase, workspaceURL, rule.Title, payload)
	if success {
		if response != "" {
			logging.Debug("Response: " + response)
		}
		logging.Debug("Status: uploaded")
		return ruleUploadResult{readErr: false, title: rule.Title, uploaded: true}
	}
	logging.Debug(fmt.Sprintf("Upload failed for %s: %s", rule.Title, response))
	return ruleUploadResult{readErr: false, title: rule.Title, uploaded: false}
}

// SyncRules uploads rendered corpus rules to the Cursor workspace, returning the number of failures.
func SyncRules(rules []compilation.RenderedRule) int {
	cfg := config.BuildSyncConfig()
	logging.Configure()

	logging.Info("Cursor rule sync start")

	if _, err := os.Stat(cfg.CursorDB); err != nil {
		logging.ErrorWithErr("Cursor database not found: "+cfg.CursorDB, err)
		logging.Info("Make sure Cursor is installed and you're logged in.")
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

	if len(rules) == 0 {
		logging.Info("No corpus rules to upload")
		return 0
	}

	logging.Info(fmt.Sprintf("Uploading %d rule(s) to cloud...", len(rules)))
	attempted, succeeded, failed, uploadedTitles, failedUploads := doUploadRules(token, cfg.APIBase, cfg.WorkspaceURL, rules)
	if len(failedUploads) > 0 {
		logging.Info("Upload failures: " + strings.Join(failedUploads, ", "))
	}
	if len(uploadedTitles) > 0 {
		logging.Debug("Uploaded rules: " + strings.Join(uploadedTitles, ", "))
	}

	logging.Info("Verifying uploaded rules...")
	remoteRules := cursorapi.ListRules(token, cfg.APIBase)
	remoteByTitle := buildRemoteRuleIndex(remoteRules)
	verified, verifyFailed, missingTitles, mismatchedTitles := doVerifyRules(rules, remoteByTitle)

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

func doUploadRules(token, apiBase, workspaceURL string, rules []compilation.RenderedRule) (int, int, int, []string, []string) {
	var attempted, succeeded, failed int
	var uploadedTitles, failedUploads []string
	totalRules := len(rules)
	for index, rule := range rules {
		res := uploadRule(token, apiBase, workspaceURL, rule, index+1, totalRules)
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

func doVerifyRules(rules []compilation.RenderedRule, remoteByTitle map[string]models.RuleRecord) (int, int, []string, []string) {
	var verified, verifyFailed int
	var missingTitles, mismatchedTitles []string
	for _, rule := range rules {
		expectedContent := buildRulePayload(rule.Title, rule.Body)
		remoteRule, exists := remoteByTitle[rule.Title]
		if !exists {
			missingTitles = append(missingTitles, rule.Title)
			verifyFailed++
			continue
		}
		if remoteRule["content"] != expectedContent {
			mismatchedTitles = append(mismatchedTitles, rule.Title)
			logging.Debug(fmt.Sprintf("Expected %d bytes, got %d bytes", len(expectedContent), len(remoteRule["content"])))
			verifyFailed++
			continue
		}
		logging.Debug(rule.Title + ": content verified")
		verified++
	}
	return verified, verifyFailed, missingTitles, mismatchedTitles
}
