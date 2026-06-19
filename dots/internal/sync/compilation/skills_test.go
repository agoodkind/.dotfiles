package compilation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func setupAgentSource(t *testing.T) (skillsDir string) {
	t.Helper()
	root := t.TempDir()
	rulesDir := filepath.Join(root, "rules")
	writeTestFile(t, filepath.Join(rulesDir, "general.mdc"), "---\ndescription: g\n---\ngeneral body\n")
	writeTestFile(t, filepath.Join(rulesDir, "code.mdc"), "---\ndescription: c\n---\ncode body\n")
	skillsDir = filepath.Join(root, "skills")
	skillBody := "---\nname: enforce-rules\ndescription: d\n---\n\nRead:\n\n{{.Rules}}\n\nOne: {{.Rule \"code\"}}\n"
	writeTestFile(t, filepath.Join(skillsDir, "enforce-rules", "SKILL.md.tmpl"), skillBody)
	return skillsDir
}

func TestRenderSkillDirsTokenExpansion(t *testing.T) {
	testCases := []struct {
		name         string
		style        SkillRefStyle
		wantRulesTop string
		wantInline   string
	}{
		{
			name:         "mdc",
			style:        SkillRefMDC,
			wantRulesTop: "- [code.mdc](../../rules/code.mdc)\n- [general.mdc](../../rules/general.mdc)",
			wantInline:   "One: [code.mdc](../../rules/code.mdc)",
		},
		{
			name:         "md",
			style:        SkillRefMD,
			wantRulesTop: "- [code.md](../../rules/code.md)\n- [general.md](../../rules/general.md)",
			wantInline:   "One: [code.md](../../rules/code.md)",
		},
		{
			name:         "instructions",
			style:        SkillRefInstructions,
			wantRulesTop: "- [code.instructions.md](../../instructions/code.instructions.md)\n- [general.instructions.md](../../instructions/general.instructions.md)",
			wantInline:   "One: [code.instructions.md](../../instructions/code.instructions.md)",
		},
		{
			name:         "codex-doc",
			style:        SkillRefCodexDoc,
			wantRulesTop: "- [code](../../AGENTS.md#code)\n- [general](../../AGENTS.md#general)",
			wantInline:   "One: [code](../../AGENTS.md#code)",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			src := setupAgentSource(t)
			dst := filepath.Join(t.TempDir(), "skills")
			if err := RenderSkillDirs(src, dst, testCase.style); err != nil {
				t.Fatalf("RenderSkillDirs: %v", err)
			}
			rendered, err := os.ReadFile(filepath.Join(dst, "enforce-rules", "SKILL.md"))
			if err != nil {
				t.Fatalf("reading rendered skill: %v", err)
			}
			got := string(rendered)
			if !strings.Contains(got, testCase.wantRulesTop) {
				t.Errorf("rendered skill missing rules list\nwant substring:\n%s\ngot:\n%s", testCase.wantRulesTop, got)
			}
			if !strings.Contains(got, testCase.wantInline) {
				t.Errorf("rendered skill missing inline rule link\nwant substring:\n%s\ngot:\n%s", testCase.wantInline, got)
			}
			if !HasGeneratedMarker(got) {
				t.Errorf("rendered skill missing generated marker:\n%s", got)
			}
			if strings.Contains(got, "{{") {
				t.Errorf("rendered skill still contains unexpanded token:\n%s", got)
			}
		})
	}
}

func TestRenderSkillDirsPrunesStaleDirs(t *testing.T) {
	src := setupAgentSource(t)
	dst := filepath.Join(t.TempDir(), "skills")

	staleGenerated := filepath.Join(dst, "old-skill", "SKILL.md")
	writeTestFile(t, staleGenerated, GeneratedAgentHTMLMarker+"\nstale\n")

	handAuthored := filepath.Join(dst, "manual-skill", "SKILL.md")
	writeTestFile(t, handAuthored, "no marker here\n")

	staleSymlink := filepath.Join(dst, "linked-skill")
	if err := os.Symlink(filepath.Join(src, "enforce-rules"), staleSymlink); err != nil {
		t.Fatalf("creating stale symlink: %v", err)
	}

	if err := RenderSkillDirs(src, dst, SkillRefMDC); err != nil {
		t.Fatalf("RenderSkillDirs: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "old-skill")); !os.IsNotExist(err) {
		t.Errorf("expected stale generated skill dir to be pruned, stat err: %v", err)
	}
	if _, err := os.Lstat(staleSymlink); !os.IsNotExist(err) {
		t.Errorf("expected stale skill symlink to be pruned, lstat err: %v", err)
	}
	if _, err := os.Stat(handAuthored); err != nil {
		t.Errorf("expected hand-authored skill dir to survive, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "enforce-rules", "SKILL.md")); err != nil {
		t.Errorf("expected rendered skill to exist, stat err: %v", err)
	}
}

func TestRenderSkillDirsPreservesHandEditedFiles(t *testing.T) {
	src := setupAgentSource(t)
	dst := filepath.Join(t.TempDir(), "skills")

	handEdited := filepath.Join(dst, "enforce-rules", "SKILL.md")
	handEditedContent := "hand edited without marker\n"
	writeTestFile(t, handEdited, handEditedContent)

	if err := RenderSkillDirs(src, dst, SkillRefMDC); err != nil {
		t.Fatalf("RenderSkillDirs: %v", err)
	}

	got, err := os.ReadFile(handEdited)
	if err != nil {
		t.Fatalf("reading skill: %v", err)
	}
	if string(got) != handEditedContent {
		t.Errorf("expected hand-edited skill without marker to be preserved\nwant: %q\ngot: %q", handEditedContent, string(got))
	}
}

func TestRenderSkillDirsReplacesSymlinkedSkillDir(t *testing.T) {
	src := setupAgentSource(t)
	dst := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("creating dst: %v", err)
	}
	link := filepath.Join(dst, "enforce-rules")
	if err := os.Symlink(filepath.Join(src, "enforce-rules"), link); err != nil {
		t.Fatalf("creating dir symlink: %v", err)
	}

	if err := RenderSkillDirs(src, dst, SkillRefMDC); err != nil {
		t.Fatalf("RenderSkillDirs: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat skill dir: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected skill dir to be a regular directory, got symlink")
	}
	rendered, err := os.ReadFile(filepath.Join(link, "SKILL.md"))
	if err != nil {
		t.Fatalf("reading rendered skill: %v", err)
	}
	if strings.Contains(string(rendered), "{{") {
		t.Errorf("rendered skill still contains unexpanded token:\n%s", string(rendered))
	}

	source, err := os.ReadFile(filepath.Join(src, "enforce-rules", "SKILL.md.tmpl"))
	if err != nil {
		t.Fatalf("reading source skill: %v", err)
	}
	if !strings.Contains(string(source), "{{.Rules}}") {
		t.Errorf("source skill template was modified, expected {{.Rules}} token to remain:\n%s", string(source))
	}
}

func TestRenderSkillDirsReplacesSymlinkTarget(t *testing.T) {
	src := setupAgentSource(t)
	dst := filepath.Join(t.TempDir(), "skills")

	if err := os.MkdirAll(filepath.Join(dst, "enforce-rules"), 0o755); err != nil {
		t.Fatalf("creating skill dir: %v", err)
	}
	link := filepath.Join(dst, "enforce-rules", "SKILL.md")
	if err := os.Symlink(filepath.Join(src, "enforce-rules", "SKILL.md.tmpl"), link); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	if err := RenderSkillDirs(src, dst, SkillRefMDC); err != nil {
		t.Fatalf("RenderSkillDirs: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat rendered skill: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("expected rendered skill to be a regular file, got symlink")
	}
	rendered, err := os.ReadFile(link)
	if err != nil {
		t.Fatalf("reading rendered skill: %v", err)
	}
	if !HasGeneratedMarker(string(rendered)) {
		t.Errorf("expected rendered skill to carry generated marker:\n%s", string(rendered))
	}
}

func TestRenderSkillDirsTemplateParseError(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "rules", "code.mdc"), "---\ndescription: c\n---\ncode body\n")
	skillsDir := filepath.Join(root, "skills")
	writeTestFile(t, filepath.Join(skillsDir, "broken", "SKILL.md.tmpl"), "---\nname: broken\n---\n\n{{.Rule \"code\"\n")

	dst := filepath.Join(t.TempDir(), "skills")
	if err := RenderSkillDirs(skillsDir, dst, SkillRefMDC); err == nil {
		t.Fatalf("expected a parse error for the malformed template, got nil")
	}
}

func TestRenderRuleTemplateSkillLink(t *testing.T) {
	style := RuleRenderStyle{SkillsRelDir: "../skills"}
	got, err := renderRuleTemplate("See {{.Skill \"make-readable\"}} for help.", style, "writing.mdc")
	if err != nil {
		t.Fatalf("renderRuleTemplate: %v", err)
	}
	want := "See [make-readable](../skills/make-readable/SKILL.md) for help."
	if got != want {
		t.Errorf("renderRuleTemplate skill link\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderRuleTemplateMissingSkillDest(t *testing.T) {
	_, err := renderRuleTemplate("See {{.Skill \"make-readable\"}}.", RuleRenderStyle{}, "writing.mdc")
	if err == nil {
		t.Fatal("expected error when skill_dest is not configured, got nil")
	}
	if !strings.Contains(err.Error(), "skill_dest") {
		t.Errorf("expected skill_dest in error, got: %v", err)
	}
}

func TestRenderRuleTemplatePassthrough(t *testing.T) {
	content := "plain body without tokens\n"
	got, err := renderRuleTemplate(content, RuleRenderStyle{}, "general.mdc")
	if err != nil {
		t.Fatalf("renderRuleTemplate: %v", err)
	}
	if got != content {
		t.Errorf("expected passthrough, got: %q", got)
	}
}

func TestRenderRuleFiles(t *testing.T) {
	srcRoot := t.TempDir()
	writeTestFile(t, filepath.Join(srcRoot, "general.mdc"), "---\ndescription: g\n---\ngeneral body\n")
	writeTestFile(t, filepath.Join(srcRoot, "code.mdc"), "---\ndescription: c\n---\ncode body\n")

	dst := filepath.Join(t.TempDir(), "rules")
	stale := filepath.Join(dst, "old.mdc")
	writeTestFile(t, stale, GeneratedAgentHTMLMarker+"\nstale\n")

	if err := RenderRuleFiles(srcRoot, dst, ".mdc", RuleTargetCursor, RuleRenderStyle{}); err != nil {
		t.Fatalf("RenderRuleFiles: %v", err)
	}
	for _, name := range []string{"general.mdc", "code.mdc"} {
		got, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if !HasGeneratedMarker(string(got)) {
			t.Errorf("rule file %s missing generated marker:\n%s", name, string(got))
		}
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("expected stale managed rule file to be pruned, stat err: %v", err)
	}
}
