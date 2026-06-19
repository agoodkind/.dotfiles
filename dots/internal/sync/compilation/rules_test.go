package compilation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	for _, want := range []string{"description: Writing rules", "globs: '*.md,*.txt'", "Body text"} {
		if !strings.Contains(got, want) {
			t.Errorf("cursor output missing %q:\n%s", want, got)
		}
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
	if !strings.Contains(got, "paths:") || !strings.Contains(got, "'*.py'") {
		t.Errorf("claude scoped output missing paths:\n%s", got)
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
	for _, want := range []string{"name: writing", "description: Writing rules", "applyTo: '*.md'", GeneratedAgentHTMLMarker, "Write plainly."} {
		if !strings.Contains(got, want) {
			t.Errorf("copilot output missing %q:\n%s", want, got)
		}
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
	if !strings.Contains(got, "applyTo: '**/*'") {
		t.Errorf("copilot global output missing applyTo:\n%s", got)
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
	if !strings.Contains(string(cursorGot), "globs: '*.md'") {
		t.Errorf("cursor rule missing globs:\n%s", string(cursorGot))
	}

	claudeDir := filepath.Join(t.TempDir(), "claude")
	if err := RenderRuleFiles(srcRoot, claudeDir, ".md", RuleTargetClaude, RuleRenderStyle{}); err != nil {
		t.Fatalf("RenderRuleFiles claude: %v", err)
	}
	scoped, err := os.ReadFile(filepath.Join(claudeDir, "writing.md"))
	if err != nil {
		t.Fatalf("reading claude scoped rule: %v", err)
	}
	if !strings.Contains(string(scoped), "paths:") || !strings.Contains(string(scoped), "'*.md'") {
		t.Errorf("claude scoped rule missing paths:\n%s", string(scoped))
	}
	global, err := os.ReadFile(filepath.Join(claudeDir, "general.md"))
	if err != nil {
		t.Fatalf("reading claude global rule: %v", err)
	}
	if strings.Contains(string(global), "paths:") {
		t.Errorf("claude global rule should omit paths front matter:\n%s", string(global))
	}
}
