// Package cursorapi implements a client for the Cursor AI API.
package cursorapi

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/cursor/constants"
	"goodkind.io/.dotfiles/internal/cursor/logging"
	"goodkind.io/.dotfiles/internal/cursor/models"
	"goodkind.io/.dotfiles/internal/cursor/protobuf"
)

// CallCursorAPI posts payload to the given Cursor API endpoint and returns the response body.
func CallCursorAPI(token string, apiBase string, endpoint string, payload []byte) []byte {
	_, responseBody := CallCursorAPIWithStatus(token, apiBase, endpoint, payload)
	return []byte(responseBody)
}

// CallCursorAPIWithStatus posts payload to the given Cursor API endpoint and returns the HTTP status code and response body.
func CallCursorAPIWithStatus(token string, apiBase string, endpoint string, payload []byte) (int, string) {
	output, err := cmdexec.CombinedOutputWithInput(
		context.Background(),
		nil,
		bytes.NewReader(payload),
		"curl",
		"-s",
		"-w",
		"%{http_code}",
		"-X",
		"POST",
		fmt.Sprintf("%s/%s", apiBase, endpoint),
		"-H",
		fmt.Sprintf("%s %s", constants.CursorRequestHeaderPrefix, token),
		"-H",
		constants.ContentTypeHeader,
		"-H",
		constants.ConnectProtocolHeader,
		"--data-binary",
		"@-",
	)
	outputText := output
	responseBody := outputText
	statusCode := 0

	if len(outputText) >= 3 {
		statusText := outputText[len(outputText)-3:]
		parsedStatus, parseError := strconv.Atoi(statusText)
		if parseError == nil {
			statusCode = parsedStatus
			responseBody = outputText[:len(outputText)-3]
		}
	}

	if err != nil {
		return 0, responseBody
	}

	return statusCode, responseBody
}

// GetCursorAuthToken reads the Cursor auth token from the given SQLite database path.
func GetCursorAuthToken(cursorDB string) string {
	output, err := cmdexec.Output(context.Background(), "sqlite3", cursorDB, constants.CursorItemTableQuery)
	if err != nil {
		return ""
	}
	token := strings.TrimSpace(output)
	return strings.Trim(token, "\"")
}

// BuildAddRulePayload encodes a protobuf payload for the add-rule endpoint.
func BuildAddRulePayload(content string, title string, workspaceURL string) []byte {
	payload := []byte{}
	payload = append(payload, protobuf.EncodeBytesField(constants.FieldAddContent, content)...)
	payload = append(payload, protobuf.EncodeBytesField(constants.FieldAddTitle, title)...)
	payload = append(payload, protobuf.EncodeBytesField(constants.FieldAddWorkspaceURL, workspaceURL)...)
	return payload
}

// ListRules fetches all rule records from the Cursor API.
func ListRules(token string, apiBase string) []models.RuleRecord {
	response := CallCursorAPI(token, apiBase, constants.EndpointList, []byte{})
	if len(response) == 0 {
		return []models.RuleRecord{}
	}
	parsedResponse := protobuf.ParseMessage(response)
	rawRuleMessages := CollectRawRuleMessages(parsedResponse)
	records := []models.RuleRecord{}
	for _, rawRuleMessage := range rawRuleMessages {
		parsedEntry := protobuf.ParseMessage(rawRuleMessage)
		record := buildRuleRecord(parsedEntry)
		if record["id"] != "" {
			records = append(records, record)
		}
	}
	return records
}

// ListRuleIDs returns the IDs of all rules from the Cursor API.
func ListRuleIDs(token string, apiBase string) []string {
	var ids []string
	for _, rule := range ListRules(token, apiBase) {
		if ruleID := rule["id"]; ruleID != "" {
			ids = append(ids, ruleID)
		}
	}
	return ids
}

// RemoveRule deletes the rule with the given ID from the Cursor API.
func RemoveRule(token string, apiBase string, ruleID string) error {
	payload := protobuf.EncodeBytesField(constants.FieldRemoveRuleID, ruleID)
	response := CallCursorAPI(token, apiBase, constants.EndpointRemove, payload)
	if len(response) == 0 {
		return nil
	}
	return nil
}

// AddRule adds a new rule to the Cursor API and reports whether it succeeded.
func AddRule(token string, apiBase string, workspaceURL string, title string, content string) (bool, string) {
	payload := BuildAddRulePayload(content, title, workspaceURL)
	statusCode, responseBody := CallCursorAPIWithStatus(token, apiBase, constants.EndpointAdd, payload)
	return 200 <= statusCode && statusCode < 300, responseBody
}

// DeleteAllRules removes all existing rules from the Cursor API and returns the count removed.
func DeleteAllRules(token string, apiBase string) int {
	existingRuleIDs := ListRuleIDs(token, apiBase)
	if len(existingRuleIDs) == 0 {
		logging.Info("No existing cloud rules found")
		return 0
	}

	total := len(existingRuleIDs)
	logging.Info(fmt.Sprintf("Removing %d existing cloud rule(s)", total))

	removalErrors := []string{}
	var removedCount atomic.Int64

	workerCount := min(constants.MaxParallelWorkers, total)
	jobs := make(chan string, total)
	type removalResult struct {
		err error
	}
	results := make(chan removalResult, total)

	var waitGroup sync.WaitGroup
	for range workerCount {
		waitGroup.Add(1)
		go func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					results <- removalResult{err: fmt.Errorf("remove worker panic: %v", recovered)}
				}
				waitGroup.Done()
			}()
			for ruleID := range jobs {
				results <- removalResult{err: RemoveRule(token, apiBase, ruleID)}
			}
		}()
	}

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				results <- removalResult{err: fmt.Errorf("remove coordinator panic: %v", recovered)}
			}
		}()
		for _, ruleID := range existingRuleIDs {
			jobs <- ruleID
		}
		close(jobs)
		waitGroup.Wait()
		close(results)
	}()

	for result := range results {
		if result.err != nil {
			removalErrors = append(removalErrors, result.err.Error())
			logging.ErrorWithErr("Rule remove failed", result.err)
			continue
		}
		index := removedCount.Add(1)
		logging.Debug(fmt.Sprintf("Removed rule %d/%d", index, total))
	}

	remainingRuleCount := len(ListRuleIDs(token, apiBase))
	if remainingRuleCount > 0 {
		logging.Info(fmt.Sprintf("%d rule(s) still present after deletion. The API may have rejected some removes.", remainingRuleCount))
	} else {
		logging.Info(fmt.Sprintf("Removed %d cloud rule(s)", total))
	}

	if len(removalErrors) > 0 {
		return total - len(removalErrors)
	}
	return total
}

func buildRuleRecord(parsedEntry models.ParsedMessage) models.RuleRecord {
	return models.RuleRecord{
		"id":        protobuf.DecodeBytesField(protobuf.GetBytesField(parsedEntry, constants.FieldRuleID), ""),
		"content":   protobuf.DecodeBytesField(protobuf.GetBytesField(parsedEntry, constants.FieldRuleContent), ""),
		"title":     protobuf.DecodeBytesField(protobuf.GetBytesField(parsedEntry, constants.FieldRuleTitle), ""),
		"timestamp": protobuf.DecodeBytesField(protobuf.GetBytesField(parsedEntry, constants.FieldRuleTimestamp), ""),
	}
}

// CollectRawRuleMessages extracts the raw bytes for each rule entry in a parsed API response.
func CollectRawRuleMessages(parsedResponse models.ParsedMessage) [][]byte {
	rawRule, hasRules := parsedResponse[constants.FieldRulesList]
	if !hasRules {
		return nil
	}
	if rawRule.Kind == models.ParsedValueBytes {
		return [][]byte{rawRule.Bytes}
	}
	if rawRule.Kind != models.ParsedValueList {
		return nil
	}
	rawRuleMessages := [][]byte{}
	for _, value := range rawRule.List {
		if value.Kind == models.ParsedValueBytes {
			rawRuleMessages = append(rawRuleMessages, value.Bytes)
		}
	}
	return rawRuleMessages
}
