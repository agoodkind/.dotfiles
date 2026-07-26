package corpus

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"goodkind.io/.dotfiles/internal/sync/compilation"
)

type promptFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func loadSourceSet(dotfiles string, manifest Manifest) (compilation.CorpusSourceSet, error) {
	source := compilation.ResolveCorpusSource(dotfiles)
	sourceSet := compilation.CorpusSourceSet{
		Rules:  make(map[string]compilation.RuleSource),
		Skills: make(map[string]compilation.SkillSource),
		Agents: make(map[string]compilation.AgentSource),
	}
	if err := loadLocalRules(source.Rules, &sourceSet); err != nil {
		return sourceSet, err
	}
	if err := loadLocalSkills(source.Skills, &sourceSet); err != nil {
		return sourceSet, err
	}
	for _, importedSource := range manifest.Sources {
		switch importedSource.Kind {
		case KindSubmoduleImport:
			if err := loadSubmoduleImport(dotfiles, importedSource, &sourceSet); err != nil {
				return sourceSet, err
			}
		default:
			return sourceSet, fmt.Errorf("unknown source kind %q for %s", importedSource.Kind, importedSource.Name)
		}
	}
	return sourceSet, nil
}

func loadLocalRules(rulesDir string, sourceSet *compilation.CorpusSourceSet) error {
	files, err := filepath.Glob(filepath.Join(rulesDir, "*.mdc"))
	if err != nil {
		slog.Warn("corpus: globbing local rules", "directory", rulesDir, "err", err)
		return fmt.Errorf("globbing corpus rules: %w", err)
	}
	sort.Strings(files)
	for _, file := range files {
		content, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			return fmt.Errorf("reading rule %s: %w", file, err)
		}
		rule, err := compilation.ParseRuleSource(string(content))
		if err != nil {
			return fmt.Errorf("parsing rule %s: %w", file, err)
		}
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		if err := addRule(sourceSet, name, rule); err != nil {
			return err
		}
	}
	return nil
}

func loadLocalSkills(skillsDir string, sourceSet *compilation.CorpusSourceSet) error {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.Warn("corpus: reading local skills", "directory", skillsDir, "err", err)
		return fmt.Errorf("reading corpus skills: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(skillsDir, entry.Name())
		files, err := readSkillFiles(skillDir)
		if err != nil {
			return err
		}
		if err := addSkill(sourceSet, compilation.SkillSource{Name: entry.Name(), Files: files}); err != nil {
			return err
		}
	}
	return nil
}

func readSkillFiles(skillDir string) (map[string]string, error) {
	files := make(map[string]string)
	if err := readSkillFilesInDir(skillDir, "", files); err != nil {
		return nil, err
	}
	return files, nil
}

func readSkillFilesInDir(skillDir string, relativeDir string, files map[string]string) error {
	currentDir := filepath.Join(skillDir, relativeDir)
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		slog.Warn("corpus: reading skill directory", "directory", currentDir, "err", err)
		return fmt.Errorf("reading skill directory %s: %w", currentDir, err)
	}
	for _, entry := range entries {
		relativePath := filepath.Join(relativeDir, entry.Name())
		if entry.IsDir() {
			if err := readSkillFilesInDir(skillDir, relativePath, files); err != nil {
				return err
			}
			continue
		}
		path := filepath.Join(skillDir, relativePath)
		content, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return fmt.Errorf("reading skill file %s: %w", path, err)
		}
		files[filepath.ToSlash(relativePath)] = string(content)
	}
	return nil
}

func loadSubmoduleImport(dotfiles string, importedSource Source, sourceSet *compilation.CorpusSourceSet) error {
	root, err := resolveImportedRoot(dotfiles, importedSource.Root)
	if err != nil {
		return err
	}
	if importedSource.WorkingAgreement != "" {
		ruleName := strings.TrimSuffix(filepath.Base(importedSource.WorkingAgreement), filepath.Ext(importedSource.WorkingAgreement))
		rule, err := loadWorkingAgreementRule(root, importedSource.WorkingAgreement)
		if err != nil {
			return err
		}
		if err := addRule(sourceSet, ruleName, rule); err != nil {
			return err
		}
	}
	for _, skillPath := range importedSource.Skills {
		content, err := readImportedFile(root, skillPath)
		if err != nil {
			return err
		}
		name, _, _, err := parsePromptMarkdown(string(content))
		if err != nil {
			return err
		}
		if err := addSkill(sourceSet, compilation.SkillSource{Name: name, Files: map[string]string{"SKILL.md": string(content)}}); err != nil {
			return err
		}
	}
	readOnlyAgents := make(map[string]struct{}, len(importedSource.ReadOnlyAgents))
	for _, name := range importedSource.ReadOnlyAgents {
		readOnlyAgents[name] = struct{}{}
	}
	for _, agentPath := range importedSource.Agents {
		content, err := readImportedFile(root, agentPath)
		if err != nil {
			return err
		}
		name, description, body, err := parsePromptMarkdown(string(content))
		if err != nil {
			return err
		}
		_, readOnly := readOnlyAgents[name]
		if readOnly {
			delete(readOnlyAgents, name)
		}
		if err := addAgent(sourceSet, compilation.AgentSource{
			Name:        name,
			Description: description,
			Body:        body,
			ReadOnly:    readOnly,
		}); err != nil {
			return err
		}
	}
	if len(readOnlyAgents) > 0 {
		unmatchedNames := make([]string, 0, len(readOnlyAgents))
		for name := range readOnlyAgents {
			unmatchedNames = append(unmatchedNames, name)
		}
		sort.Strings(unmatchedNames)
		return fmt.Errorf("read-only agent metadata did not match imported agents: %s", strings.Join(unmatchedNames, ", "))
	}
	return nil
}

func resolveImportedRoot(dotfiles string, relativeRoot string) (string, error) {
	return resolveContainedPath(dotfiles, relativeRoot, "dotfiles root", "source root")
}

func readImportedFile(root string, relativePath string) ([]byte, error) {
	resolvedPath, err := resolveImportedPath(root, relativePath)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(filepath.Clean(resolvedPath))
	if err != nil {
		slog.Warn("corpus: reading imported file", "path", relativePath, "err", err)
		return nil, fmt.Errorf("reading imported file %s: %w", relativePath, err)
	}
	return content, nil
}

func resolveImportedPath(root string, relativePath string) (string, error) {
	return resolveContainedPath(root, relativePath, "source root", "import path")
}

func resolveContainedPath(base string, relative string, baseLabel string, pathLabel string) (string, error) {
	cleanBase := filepath.Clean(base)
	candidatePath := filepath.Clean(filepath.Join(cleanBase, relative))
	if candidatePath != cleanBase && !strings.HasPrefix(candidatePath, cleanBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s %q escapes %s %s", pathLabel, relative, baseLabel, cleanBase)
	}
	resolvedBase, err := filepath.EvalSymlinks(cleanBase)
	if err != nil {
		slog.Warn("corpus: resolving contained path base", "label", baseLabel, "root", cleanBase, "err", err)
		return "", fmt.Errorf("resolving %s %s: %w", baseLabel, cleanBase, err)
	}
	resolvedPath, err := filepath.EvalSymlinks(candidatePath)
	if err != nil {
		return "", fmt.Errorf("resolving %s %s: %w", pathLabel, relative, err)
	}
	if resolvedPath != resolvedBase && !strings.HasPrefix(resolvedPath, resolvedBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s %q resolves outside %s %s", pathLabel, relative, baseLabel, cleanBase)
	}
	return resolvedPath, nil
}

func loadWorkingAgreementRule(root string, relativePath string) (compilation.RuleSource, error) {
	content, err := readImportedFile(root, relativePath)
	if err != nil {
		return compilation.RuleSource{}, err
	}
	body, err := extractFirstFencedBlock(string(content))
	if err != nil {
		slog.Warn("corpus: extracting working agreement", "path", relativePath, "err", err)
		return compilation.RuleSource{}, fmt.Errorf("extracting working agreement body from %s: %w", relativePath, err)
	}
	return compilation.RuleSource{
		Description: "Imported working agreements",
		AppliesTo:   []string{"**/*"},
		Always:      true,
		Body:        body,
	}, nil
}

func extractFirstFencedBlock(content string) (string, error) {
	openFence := strings.Index(content, "```")
	if openFence == -1 {
		return "", fmt.Errorf("missing opening fenced block")
	}
	afterFence := content[openFence+3:]
	newlineIndex := strings.Index(afterFence, "\n")
	if newlineIndex == -1 {
		return "", fmt.Errorf("missing fenced block body")
	}
	bodyStart := openFence + 3 + newlineIndex + 1
	closeFence := strings.Index(content[bodyStart:], "\n```")
	if closeFence == -1 {
		return "", fmt.Errorf("missing closing fenced block")
	}
	return strings.TrimSpace(content[bodyStart : bodyStart+closeFence]), nil
}

func parsePromptMarkdown(content string) (string, string, string, error) {
	if !strings.HasPrefix(content, "---\n") {
		return "", "", "", fmt.Errorf("missing opening frontmatter delimiter")
	}
	endMarker := "\n---\n"
	frontmatterEnd := strings.Index(content[4:], endMarker)
	if frontmatterEnd == -1 {
		return "", "", "", fmt.Errorf("missing closing frontmatter delimiter")
	}
	rawFrontmatter := content[4 : 4+frontmatterEnd]
	var frontmatter promptFrontmatter
	if err := yaml.Unmarshal([]byte(rawFrontmatter), &frontmatter); err != nil {
		slog.Warn("corpus: parsing prompt frontmatter", "err", err)
		return "", "", "", fmt.Errorf("parsing prompt front matter: %w", err)
	}
	promptName := strings.TrimSpace(frontmatter.Name)
	if promptName == "" {
		return "", "", "", fmt.Errorf("missing prompt name")
	}
	if err := validatePromptName(promptName); err != nil {
		return "", "", "", err
	}
	if strings.TrimSpace(frontmatter.Description) == "" {
		return "", "", "", fmt.Errorf("missing prompt description")
	}
	bodyStart := 4 + frontmatterEnd + len(endMarker)
	body := strings.TrimSpace(content[bodyStart:])
	return promptName, strings.TrimSpace(frontmatter.Description), body, nil
}

func validatePromptName(name string) error {
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || cleanName == ".." || filepath.IsAbs(cleanName) {
		return fmt.Errorf("invalid prompt name %q", name)
	}
	if strings.Contains(name, "/") || strings.Contains(name, string(filepath.Separator)) {
		return fmt.Errorf("invalid prompt name %q", name)
	}
	if strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) || cleanName != name {
		return fmt.Errorf("invalid prompt name %q", name)
	}
	return nil
}

func addRule(sourceSet *compilation.CorpusSourceSet, name string, rule compilation.RuleSource) error {
	if _, exists := sourceSet.Rules[name]; exists {
		return fmt.Errorf("duplicate rule %q", name)
	}
	sourceSet.Rules[name] = rule
	return nil
}

func addSkill(sourceSet *compilation.CorpusSourceSet, skill compilation.SkillSource) error {
	if _, exists := sourceSet.Skills[skill.Name]; exists {
		return fmt.Errorf("duplicate skill %q", skill.Name)
	}
	sourceSet.Skills[skill.Name] = skill
	return nil
}

func addAgent(sourceSet *compilation.CorpusSourceSet, agent compilation.AgentSource) error {
	if _, exists := sourceSet.Agents[agent.Name]; exists {
		return fmt.Errorf("duplicate agent %q", agent.Name)
	}
	sourceSet.Agents[agent.Name] = agent
	return nil
}
