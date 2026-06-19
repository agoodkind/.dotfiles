package compilation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goodkind.io/.dotfiles/internal/sync/testfrontmatter"
)

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
	if testfrontmatter.StringField(t, got, "description") != "Writing rules" {
		t.Errorf("cursor description mismatch:\n%s", got)
	}
	if testfrontmatter.StringField(t, got, "globs") != "*.md,*.txt" {
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
	paths := testfrontmatter.StringSliceField(t, got, "paths")
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
	if testfrontmatter.StringField(t, got, "name") != "writing" {
		t.Errorf("copilot name mismatch:\n%s", got)
	}
	if testfrontmatter.StringField(t, got, "description") != "Writing rules" {
		t.Errorf("copilot description mismatch:\n%s", got)
	}
	if testfrontmatter.StringField(t, got, "applyTo") != "*.md" {
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
	if testfrontmatter.StringField(t, got, "applyTo") != "**/*" {
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
	if testfrontmatter.StringField(t, string(cursorGot), "globs") != "*.md" {
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
	scopedPaths := testfrontmatter.StringSliceField(t, string(scoped), "paths")
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

func TestParseRuleSourceUnclosedFrontmatter(t *testing.T) {
	_, err := ParseRuleSource("---\ndescription: Broken\n")
	if err == nil {
		t.Fatal("expected error for unclosed front matter")
	}
}

func TestRuleTargetFormatFromExt(t *testing.T) {
	format, err := RuleTargetFormatFromExt(".md")
	if err != nil || format != RuleTargetClaude {
		t.Errorf("RuleTargetFormatFromExt(.md) = (%q, %v), want (claude, nil)", format, err)
	}
	format, err = RuleTargetFormatFromExt(".mdc")
	if err != nil || format != RuleTargetCursor {
		t.Errorf("RuleTargetFormatFromExt(.mdc) = (%q, %v), want (cursor, nil)", format, err)
	}
	if _, err := RuleTargetFormatFromExt(".txt"); err == nil {
		t.Error("RuleTargetFormatFromExt(.txt) expected error, got nil")
	}
}
