package compilation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RenderRuleFiles(src string, dst string, ext string, format RuleTargetFormat, style RuleRenderStyle) error {
	sourceSet, err := loadRuleSourceSetForTest(src)
	if err != nil {
		return err
	}
	return RenderRuleFilesFromSourceSet(sourceSet, dst, ext, format, style)
}

func RenderRulesAsInstructionDoc(src string, dst string, title string, style RuleRenderStyle) error {
	sourceSet, err := loadRuleSourceSetForTest(src)
	if err != nil {
		return err
	}
	return RenderRulesAsInstructionDocFromSourceSet(sourceSet, dst, title, style)
}

func RenderCopilotInstructionFiles(src string, dst string, style RuleRenderStyle) error {
	sourceSet, err := loadRuleSourceSetForTest(src)
	if err != nil {
		return err
	}
	return RenderCopilotInstructionFilesFromSourceSet(sourceSet, dst, style)
}

func RenderRulesForUpload(src string, style RuleRenderStyle) ([]RenderedRule, error) {
	sourceSet, err := loadRuleSourceSetForTest(src)
	if err != nil {
		return nil, err
	}
	return RenderRulesForUploadFromSourceSet(sourceSet, style)
}

func RenderSkillDirs(src string, dst string, style SkillRefStyle) error {
	sourceSet, err := loadRuleSourceSetForTest(filepath.Join(filepath.Dir(src), "rules"))
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("reading skills directory %s: %w", src, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files := make(map[string]string)
		if err := readSkillFilesForTest(filepath.Join(src, entry.Name()), "", files); err != nil {
			return err
		}
		sourceSet.Skills[entry.Name()] = SkillSource{Name: entry.Name(), Files: files}
	}
	return RenderSkillDirsFromSourceSet(sourceSet, dst, style)
}

func loadRuleSourceSetForTest(rulesDir string) (CorpusSourceSet, error) {
	sourceSet := CorpusSourceSet{
		Rules:  make(map[string]RuleSource),
		Skills: make(map[string]SkillSource),
		Agents: make(map[string]AgentSource),
	}
	files, err := filepath.Glob(filepath.Join(rulesDir, "*.mdc"))
	if err != nil {
		return sourceSet, fmt.Errorf("globbing test rules: %w", err)
	}
	for _, path := range files {
		content, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return sourceSet, fmt.Errorf("reading test rule %s: %w", path, err)
		}
		rule, err := ParseRuleSource(string(content))
		if err != nil {
			return sourceSet, fmt.Errorf("parsing test rule %s: %w", path, err)
		}
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		sourceSet.Rules[name] = rule
	}
	return sourceSet, nil
}

func readSkillFilesForTest(root string, relativeDir string, files map[string]string) error {
	current := filepath.Join(root, relativeDir)
	entries, err := os.ReadDir(current)
	if err != nil {
		return fmt.Errorf("reading test skill directory %s: %w", current, err)
	}
	for _, entry := range entries {
		relativePath := filepath.Join(relativeDir, entry.Name())
		if entry.IsDir() {
			if err := readSkillFilesForTest(root, relativePath, files); err != nil {
				return err
			}
			continue
		}
		content, err := os.ReadFile(filepath.Clean(filepath.Join(root, relativePath)))
		if err != nil {
			return fmt.Errorf("reading test skill file %s: %w", relativePath, err)
		}
		files[filepath.ToSlash(relativePath)] = string(content)
	}
	return nil
}
