package cursorapi

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"cursor-sync/internal/constants"
	"cursor-sync/internal/logging"
	"cursor-sync/internal/models"
	"cursor-sync/internal/protobuf"
)

func CallCursorAPI(token string, apiBase string, endpoint string, payload []byte) []byte {
	_, responseBody := CallCursorAPIWithStatus(token, apiBase, endpoint, payload)
	return []byte(responseBody)
}

func CallCursorAPIWithStatus(token string, apiBase string, endpoint string, payload []byte) (int, string) {
	curlCommand := exec.Command(
		"curl",
		"-s",
		"-w",
		"%{http_code}",
		"-X",
		"POST",
		fmt.Sprintf("%s/%s", apiBase, endpoint),
		"-H",
		fmt.Sprintf("%s %s", constants.CursorAuthHeader, token),
		"-H",
		constants.ContentTypeHeader,
		"-H",
		constants.ConnectProtocolHeader,
		"--data-binary",
		"@-",
	)
	curlCommand.Stdin = bytes.NewReader(payload)
	output, err := curlCommand.Output()
	outputText := string(output)
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

func GetCursorAuthToken(cursorDB string) string {
	command := exec.Command(
		"sqlite3",
		cursorDB,
		constants.CursorAuthTokenQuery,
	)
	output, err := command.Output()
	if err != nil {
		return ""
	}
	token := strings.TrimSpace(string(output))
	return strings.Trim(token, "\"")
}

func BuildAddRulePayload(content string, title string, workspaceURL string) []byte {
	payload := []byte{}
	payload = append(payload, protobuf.EncodeBytesField(constants.FieldAddContent, content)...)
	payload = append(payload, protobuf.EncodeBytesField(constants.FieldAddTitle, title)...)
	payload = append(payload, protobuf.EncodeBytesField(constants.FieldAddWorkspaceURL, workspaceURL)...)
	return payload
}

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

func ListRuleIDs(token string, apiBase string) []string {
	var ids []string
	for _, rule := range ListRules(token, apiBase) {
		if ruleID := rule["id"]; ruleID != "" {
			ids = append(ids, ruleID)
		}
	}
	return ids
}

func RemoveRule(token string, apiBase string, ruleID string) error {
	payload := protobuf.EncodeBytesField(constants.FieldRemoveRuleID, ruleID)
	response := CallCursorAPI(token, apiBase, constants.EndpointRemove, payload)
	if len(response) == 0 {
		return nil
	}
	return nil
}

func AddRule(token string, apiBase string, workspaceURL string, title string, content string) (bool, string) {
	payload := BuildAddRulePayload(content, title, workspaceURL)
	statusCode, responseBody := CallCursorAPIWithStatus(token, apiBase, constants.EndpointAdd, payload)
	return 200 <= statusCode && statusCode < 300, responseBody
}

func DeleteAllRules(token string, apiBase string) int {
	existingRuleIDs := ListRuleIDs(token, apiBase)
	if len(existingRuleIDs) == 0 {
		logging.Info("No existing cloud rules found")
		return 0
	}

	total := len(existingRuleIDs)
	logging.Info(fmt.Sprintf("Removing %d existing cloud rule(s)", total))

	removalErrors := []string{}
	var removedCount int64

	workerCount := constants.MaxParallelWorkers
	if total < workerCount {
		workerCount = total
	}
	jobs := make(chan string, total)
	type removalResult struct {
		err error
	}
	results := make(chan removalResult, total)

	var waitGroup sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for ruleID := range jobs {
				results <- removalResult{err: RemoveRule(token, apiBase, ruleID)}
			}
		}()
	}

	go func() {
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
			logging.Error(fmt.Sprintf("Rule remove failed: %s", result.err))
			continue
		}
		index := atomic.AddInt64(&removedCount, 1)
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

func CollectRawRuleMessages(parsedResponse models.ParsedMessage) [][]byte {
	rawRules, hasRules := parsedResponse[constants.FieldRulesList]
	if !hasRules {
		return nil
	}
	if rawRule, ok := rawRules.([]byte); ok {
		return [][]byte{rawRule}
	}
	rawRuleList, ok := rawRules.([]interface{})
	if !ok {
		return nil
	}
	rawRuleMessages := [][]byte{}
	for _, rawRule := range rawRuleList {
		if rawRuleBytes, ok := rawRule.([]byte); ok {
			rawRuleMessages = append(rawRuleMessages, rawRuleBytes)
		}
	}
	return rawRuleMessages
}
