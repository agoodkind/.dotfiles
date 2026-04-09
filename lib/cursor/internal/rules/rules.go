package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cursor-sync/internal/logging"
)

func ParseMdcContent(rawContent string) string {
	if strings.HasPrefix(rawContent, "---") {
		separatorIndex := strings.Index(rawContent[3:], "\n---")
		if separatorIndex != -1 {
			return strings.TrimSpace(strings.TrimLeft(rawContent[separatorIndex+4:], "\n"))
		}
	}
	return strings.TrimSpace(rawContent)
}

func CollectRuleFiles(ruleDirectories []string) []string {
	selectedByName := map[string]string{}

	for _, ruleDirectory := range ruleDirectories {
		if _, err := os.Stat(ruleDirectory); err != nil {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(ruleDirectory, "*.mdc"))
		if err != nil {
			continue
		}
		sort.Strings(matches)
		for _, ruleFile := range matches {
			stem := strings.TrimSuffix(filepath.Base(ruleFile), filepath.Ext(ruleFile))
			selectedByName[stem] = ruleFile
		}
	}

	stems := make([]string, 0, len(selectedByName))
	for stem := range selectedByName {
		stems = append(stems, stem)
	}
	sort.Strings(stems)

	ordered := make([]string, 0, len(stems))
	for _, stem := range stems {
		ordered = append(ordered, selectedByName[stem])
	}
	return ordered
}

func ResolveRuleFile(ruleFile string) string {
	info, err := os.Lstat(ruleFile)
	if err != nil {
		return ruleFile
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return ruleFile
	}
	resolvedFile, err := filepath.EvalSymlinks(ruleFile)
	if err != nil {
		return ruleFile
	}
	return resolvedFile
}

func FormatRuleSource(ruleFile string) string {
	info, err := os.Lstat(ruleFile)
	if err != nil {
		return ruleFile
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return ruleFile
	}
	resolvedFile, err := filepath.EvalSymlinks(ruleFile)
	if err != nil {
		return ruleFile
	}
	return ruleFile + " -> " + resolvedFile
}

func ValidateRuleDirectories(ruleDirectories []string) {
	existing := []string{}
	for _, ruleDirectory := range ruleDirectories {
		if _, err := os.Stat(ruleDirectory); err == nil {
			existing = append(existing, ruleDirectory)
		}
	}
	if len(existing) == 0 {
		logging.Info("No rules directories found. Checked: " + strings.Join(ruleDirectories, ", "))
		os.Exit(1)
	}
	for _, ruleDirectory := range ruleDirectories {
		status := "missing"
		if _, err := os.Stat(ruleDirectory); err == nil {
			status = "present"
		}
		logging.Info("Rules directory: " + ruleDirectory + " (" + status + ")")
	}
}
