package compilation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func parseFrontmatterMap(t *testing.T, content string) map[string]any {
	t.Helper()
	if !strings.HasPrefix(content, "---\n") {
		t.Fatalf("expected front matter in content:\n%s", content)
	}
	endMarker := "\n---\n"
	frontmatterEnd := strings.Index(content[4:], endMarker)
	if frontmatterEnd == -1 {
		t.Fatalf("expected closing front matter delimiter in content:\n%s", content)
	}
	rawFrontmatter := content[4 : 4+frontmatterEnd]
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(rawFrontmatter), &metadata); err != nil {
		t.Fatalf("unmarshaling front matter: %v\n%s", err, rawFrontmatter)
	}
	return metadata
}

func frontmatterStringField(t *testing.T, content string, key string) string {
	t.Helper()
	metadata := parseFrontmatterMap(t, content)
	value, ok := metadata[key].(string)
	if !ok {
		t.Fatalf("front matter key %q missing or not a string in %#v", key, metadata)
	}
	return value
}

func frontmatterStringSliceField(t *testing.T, content string, key string) []string {
	t.Helper()
	metadata := parseFrontmatterMap(t, content)
	rawValues, ok := metadata[key].([]any)
	if !ok {
		t.Fatalf("front matter key %q missing or not a string slice in %#v", key, metadata)
	}
	values := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		stringValue, ok := rawValue.(string)
		if !ok {
			t.Fatalf("front matter key %q contains non-string value in %#v", key, metadata)
		}
		values = append(values, stringValue)
	}
	return values
}

func TestParseRuleSourceNeutralSchema(t *testing.T) {
	content := "---\ndescription: Writing rules\napplies_to:\n  - \"*.md\"\n  - \"*.txt\"\nalways: false\n---\n\nBody text\n"
	rule, err := ParseRuleSource(content)
	if err != nil {
		t.Fatalf("ParseRuleSource: %v", err)
	}
	if rule.Description != "Writing rules" {
		t.Errorf("Description = %q", rule.Description)
	}
	if len(rule.AppliesTo) != 2 || rule.AppliesTo[0] != "*.md" {
		t.Errorf("AppliesTo = %#v", rule.AppliesTo)
	}
	if rule.Always {
		t.Error("expected Always=false")
	}
	if strings.TrimSpace(rule.Body) != "Body text" {
		t.Errorf("Body = %q", rule.Body)
	}
}

func TestParseRuleSourceLegacyCursorFields(t *testing.T) {
	content := "---\ndescription: Legacy\nalwaysApply: true\nglobs: \"**/*\"\n---\n\nLegacy body\n"
	rule, err := ParseRuleSource(content)
	if err != nil {
		t.Fatalf("ParseRuleSource: %v", err)
	}
	if !rule.Always {
		t.Error("expected Always=true from legacy alwaysApply")
	}
	if len(rule.AppliesTo) != 1 || rule.AppliesTo[0] != "**/*" {
		t.Errorf("AppliesTo = %#v", rule.AppliesTo)
	}
}

func TestRenderCursorFrontmatter(t *testing.T) {
	rule := RuleSource{
		Description: "Writing rules",
		AppliesTo:   []string{"*.md", "*.txt"},
		Always:      false,
		Body:        "Body text",
	}
	got, err := rule.RenderForTarget(RuleTargetCursor)
	if err != nil {
		t.Fatalf("RenderForTarget: %v", err)
	}
	if frontmatterStringField(t, got, "description") != "Writing rules" {
		t.Errorf("cursor description mismatch:\n%s", got)
	}
	if frontmatterStringField(t, got, "globs") != "*.md,*.txt" {
		t.Errorf("cursor globs mismatch:\n%s", got)
	}
	if !strings.Contains(got, "Body text") {
		t.Errorf("cursor output missing body:\n%s", got)
	}
	if strings.Contains(got, "alwaysApply:") {
		t.Errorf("cursor output should omit alwaysApply when false:\n%s", got)
	}
}

func TestRenderClaudeScopedFrontmatter(t *testing.T) {
	rule := RuleSource{
		Description: "Python rules",
		AppliesTo:   []string{"*.py"},
		Always:      false,
		Body:        "Use uv.",
	}
	got, err := rule.RenderForTarget(RuleTargetClaude)
	if err != nil {
		t.Fatalf("RenderForTarget: %v", err)
	}
	paths := frontmatterStringSliceField(t, got, "paths")
	if len(paths) != 1 || paths[0] != "*.py" {
		t.Errorf("claude scoped paths = %#v, want [\"*.py\"]", paths)
	}
	if !strings.Contains(got, "Use uv.") {
		t.Errorf("claude scoped output missing body:\n%s", got)
	}
}

func TestRenderClaudeGlobalOmitsFrontmatter(t *testing.T) {
	rule := RuleSource{
		Description: "Global rules",
		AppliesTo:   []string{"**/*"},
		Always:      true,
		Body:        "Global body",
	}
	got, err := rule.RenderForTarget(RuleTargetClaude)
	if err != nil {
		t.Fatalf("RenderForTarget: %v", err)
	}
	if strings.HasPrefix(got, "---") {
		t.Errorf("expected no front matter for global Claude rule:\n%s", got)
	}
	if strings.TrimSpace(got) != "Global body" {
		t.Errorf("unexpected body: %q", got)
	}
}

func TestRenderCopilotFrontmatter(t *testing.T) {
	rule := RuleSource{
		Description: "Writing rules",
		AppliesTo:   []string{"*.md"},
		Always:      false,
		Body:        "Write plainly.",
	}
	got, err := rule.RenderCopilot("writing")
	if err != nil {
		t.Fatalf("RenderCopilot: %v", err)
	}
	if frontmatterStringField(t, got, "name") != "writing" {
		t.Errorf("copilot name mismatch:\n%s", got)
	}
	if frontmatterStringField(t, got, "description") != "Writing rules" {
		t.Errorf("copilot description mismatch:\n%s", got)
	}
	if frontmatterStringField(t, got, "applyTo") != "*.md" {
		t.Errorf("copilot applyTo mismatch:\n%s", got)
	}
	if !strings.Contains(got, GeneratedAgentHTMLMarker) {
		t.Errorf("copilot output missing generated marker:\n%s", got)
	}
	if !strings.Contains(got, "Write plainly.") {
		t.Errorf("copilot output missing body:\n%s", got)
	}
}

func TestRenderCopilotGlobalApplyTo(t *testing.T) {
	rule := RuleSource{
		Description: "Global rules",
		AppliesTo:   []string{"**/*"},
		Always:      true,
		Body:        "Global body",
	}
	got, err := rule.RenderCopilot("general")
	if err != nil {
		t.Fatalf("RenderCopilot: %v", err)
	}
	if frontmatterStringField(t, got, "applyTo") != "**/*" {
		t.Errorf("copilot global applyTo mismatch:\n%s", got)
	}
}

func TestRenderRuleFilesTargetFrontmatter(t *testing.T) {
	srcRoot := t.TempDir()
	writeTestFile(t, filepath.Join(srcRoot, "writing.mdc"),
		"---\ndescription: Writing rules\napplies_to:\n  - \"*.md\"\nalways: false\n---\n\nBody text\n")
	writeTestFile(t, filepath.Join(srcRoot, "general.mdc"),
		"---\ndescription: Global rules\napplies_to:\n  - \"**/*\"\nalways: true\n---\n\nGlobal body\n")

	cursorDir := filepath.Join(t.TempDir(), "cursor")
	if err := RenderRuleFiles(srcRoot, cursorDir, ".mdc", RuleTargetCursor, RuleRenderStyle{}); err != nil {
		t.Fatalf("RenderRuleFiles cursor: %v", err)
	}
	cursorGot, err := os.ReadFile(filepath.Join(cursorDir, "writing.mdc"))
	if err != nil {
		t.Fatalf("reading cursor rule: %v", err)
	}
	if frontmatterStringField(t, string(cursorGot), "globs") != "*.md" {
		t.Errorf("cursor rule globs mismatch:\n%s", string(cursorGot))
	}

	claudeDir := filepath.Join(t.TempDir(), "claude")
	if err := RenderRuleFiles(srcRoot, claudeDir, ".md", RuleTargetClaude, RuleRenderStyle{}); err != nil {
		t.Fatalf("RenderRuleFiles claude: %v", err)
	}
	scoped, err := os.ReadFile(filepath.Join(claudeDir, "writing.md"))
	if err != nil {
		t.Fatalf("reading claude scoped rule: %v", err)
	}
	scopedPaths := frontmatterStringSliceField(t, string(scoped), "paths")
	if len(scopedPaths) != 1 || scopedPaths[0] != "*.md" {
		t.Errorf("claude scoped paths = %#v, want [\"*.md\"]", scopedPaths)
	}
	global, err := os.ReadFile(filepath.Join(claudeDir, "general.md"))
	if err != nil {
		t.Fatalf("reading claude global rule: %v", err)
	}
	if strings.Contains(string(global), "paths:") {
		t.Errorf("claude global rule should omit paths front matter:\n%s", string(global))
	}
}
