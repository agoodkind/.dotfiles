// Package rules implements management of Cursor rules files.
package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"goodkind.io/.dotfiles/internal/cursor/logging"
)

// ParseMdcContent strips YAML frontmatter from rawContent and returns the trimmed body.
func ParseMdcContent(rawContent string) string {
	if strings.HasPrefix(rawContent, "---") {
		separatorIndex := strings.Index(rawContent[3:], "\n---")
		if separatorIndex != -1 {
			return strings.TrimSpace(strings.TrimLeft(rawContent[separatorIndex+4:], "\n"))
		}
	}
	return strings.TrimSpace(rawContent)
}

// CollectRuleFiles gathers .mdc files from ruleDirectories, deduplicating by stem and sorting by name.
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

// ResolveRuleFile resolves ruleFile to its symlink target if it is a symbolic link.
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

// FormatRuleSource returns a display string for ruleFile, including the symlink target when applicable.
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

// ValidateRuleDirectories returns an error when none of ruleDirectories exist on disk.
func ValidateRuleDirectories(ruleDirectories []string) error {
	existing := []string{}
	statuses := []string{}
	for _, ruleDirectory := range ruleDirectories {
		status := "missing"
		if _, err := os.Stat(ruleDirectory); err == nil {
			existing = append(existing, ruleDirectory)
			status = "present"
		}
		statuses = append(statuses, ruleDirectory+" ("+status+")")
	}
	if len(existing) == 0 {
		return fmt.Errorf("no rules directories found; checked: %s", strings.Join(ruleDirectories, ", "))
	}
	logging.Info("Rules directories: " + strings.Join(statuses, "; "))
	return nil
}
