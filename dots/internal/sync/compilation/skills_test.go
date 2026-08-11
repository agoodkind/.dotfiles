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
	handEditedContent := "---\nname: enforce-rules\ndescription: hand edited\n---\n\nhand edited without marker\n"
	writeTestFile(t, handEdited, handEditedContent)

	if err := RenderSkillDirs(src, dst, SkillRefMDC); err != nil {
		t.Fatalf("RenderSkillDirs: %v", err)
	}

	got, err := os.ReadFile(handEdited)
	if err != nil {
		t.Fatalf("reading skill: %v", err)
	}
	if string(got) != handEditedContent {
		t.Errorf("expected hand-edited skill with valid frontmatter to be preserved\nwant: %q\ngot: %q", handEditedContent, string(got))
	}
}

func TestRenderSkillDirsFailsOnCorruptFrontmatter(t *testing.T) {
	src := setupAgentSource(t)
	dst := filepath.Join(t.TempDir(), "skills")

	corrupt := filepath.Join(dst, "enforce-rules", "SKILL.md")
	corruptContent := "---\n\n## name: enforce-rules\ndescription: broken\n"
	writeTestFile(t, corrupt, corruptContent)

	err := RenderSkillDirs(src, dst, SkillRefMDC)
	if err == nil {
		t.Fatal("expected RenderSkillDirs to fail on corrupt frontmatter, got nil")
	}
	if !strings.Contains(err.Error(), corrupt) {
		t.Errorf("expected error to name corrupt skill path %q, got: %v", corrupt, err)
	}
	if !strings.Contains(err.Error(), "unusable frontmatter") {
		t.Errorf("expected unusable frontmatter in error, got: %v", err)
	}
}

func TestRenderSkillDirsFailsOnInvalidYAMLFrontmatter(t *testing.T) {
	src := setupAgentSource(t)
	dst := filepath.Join(t.TempDir(), "skills")

	corrupt := filepath.Join(dst, "enforce-rules", "SKILL.md")
	corruptContent := "---\nname: enforce-rules\ndescription: \"broken\n---\n"
	writeTestFile(t, corrupt, corruptContent)

	err := RenderSkillDirs(src, dst, SkillRefMDC)
	if err == nil {
		t.Fatal("expected RenderSkillDirs to fail on invalid YAML frontmatter, got nil")
	}
	if !strings.Contains(err.Error(), corrupt) {
		t.Errorf("expected error to name corrupt skill path %q, got: %v", corrupt, err)
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

func TestRenderSkillDirsSkillLink(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "rules", "code.mdc"), "---\ndescription: c\n---\ncode body\n")
	skillsDir := filepath.Join(root, "skills")
	writeTestFile(
		t,
		filepath.Join(skillsDir, "split-to-prs", "SKILL.md.tmpl"),
		"---\nname: split-to-prs\ndescription: d\n---\n\nSee {{.Skill \"graphite\"}}.\n",
	)
	dst := filepath.Join(t.TempDir(), "skills")
	if err := RenderSkillDirs(skillsDir, dst, SkillRefMDC); err != nil {
		t.Fatalf("RenderSkillDirs: %v", err)
	}
	rendered, err := os.ReadFile(filepath.Join(dst, "split-to-prs", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading rendered skill: %v", err)
	}
	want := "See [graphite](../graphite/SKILL.md)."
	if !strings.Contains(string(rendered), want) {
		t.Errorf("rendered skill missing skill link\nwant substring:\n%s\ngot:\n%s", want, string(rendered))
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

func TestRenderSkillDirsRuleBodyTransclusion(t *testing.T) {
	root := t.TempDir()
	rulesDir := filepath.Join(root, "rules")
	writeTestFile(t, filepath.Join(rulesDir, "writing.mdc"), strings.Join([]string{
		"---",
		"description: Writing rules",
		"---",
		"",
		"# Writing rule",
		"",
		"See {{.Skill \"make-readable\"}} for help.",
		"",
		"Anti-hardcode guidance lives here.",
	}, "\n"))
	skillsDir := filepath.Join(root, "skills")
	writeTestFile(
		t,
		filepath.Join(skillsDir, "enforce-rules", "SKILL.md.tmpl"),
		"---\nname: enforce-rules\ndescription: d\n---\n\n## Writing\n\n{{.RuleBody \"writing\"}}\n",
	)
	dst := filepath.Join(t.TempDir(), "skills")
	if err := RenderSkillDirs(skillsDir, dst, SkillRefMDC); err != nil {
		t.Fatalf("RenderSkillDirs: %v", err)
	}
	rendered, err := os.ReadFile(filepath.Join(dst, "enforce-rules", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading rendered skill: %v", err)
	}
	got := string(rendered)
	if !strings.Contains(got, "# Writing rule") {
		t.Errorf("rendered skill missing inlined rule heading:\n%s", got)
	}
	if !strings.Contains(got, "Anti-hardcode guidance lives here.") {
		t.Errorf("rendered skill missing inlined rule body:\n%s", got)
	}
	wantSkillLink := "See [make-readable](../make-readable/SKILL.md) for help."
	if !strings.Contains(got, wantSkillLink) {
		t.Errorf("rendered skill missing rewritten skill link\nwant substring:\n%s\ngot:\n%s", wantSkillLink, got)
	}
	if strings.Count(got, GeneratedAgentHTMLMarker) != 1 {
		t.Errorf("expected exactly one generated marker, got %d in:\n%s", strings.Count(got, GeneratedAgentHTMLMarker), got)
	}
	if strings.Contains(got, "{{") {
		t.Errorf("rendered skill still contains unexpanded token:\n%s", got)
	}
}

func TestRenderSkillDirsSkillBodyTransclusion(t *testing.T) {
	root := t.TempDir()
	writeTestFile(
		t,
		filepath.Join(root, "rules", "writing.mdc"),
		"---\ndescription: writing\n---\nWriting guidance.\n",
	)
	skillsDir := filepath.Join(root, "skills")
	writeTestFile(
		t,
		filepath.Join(skillsDir, "shared", "SKILL.md.tmpl"),
		"---\nname: shared\ndescription: shared guidance\n---\n\nShared body.\n\n{{.RuleBody \"writing\"}}\n",
	)
	writeTestFile(
		t,
		filepath.Join(skillsDir, "root", "SKILL.md.tmpl"),
		"---\nname: root\ndescription: root guidance\n---\n\nRoot body.\n\n{{.SkillBody \"shared\"}}\n",
	)

	dst := filepath.Join(t.TempDir(), "skills")
	if err := RenderSkillDirs(skillsDir, dst, SkillRefMDC); err != nil {
		t.Fatalf("RenderSkillDirs: %v", err)
	}
	rendered, err := os.ReadFile(filepath.Join(dst, "root", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading rendered skill: %v", err)
	}
	got := string(rendered)
	if !strings.Contains(got, "Shared body.\n\nWriting guidance.") {
		t.Errorf("rendered skill missing expanded skill and rule bodies:\n%s", got)
	}
	if strings.Contains(got, "name: shared") || strings.Contains(got, "description: shared guidance") {
		t.Errorf("rendered skill retained embedded front matter:\n%s", got)
	}
	if strings.Count(got, GeneratedAgentHTMLMarker) != 1 {
		t.Errorf("expected exactly one generated marker, got %d in:\n%s", strings.Count(got, GeneratedAgentHTMLMarker), got)
	}
	if strings.Contains(got, "{{") {
		t.Errorf("rendered skill still contains unexpanded token:\n%s", got)
	}
}

func TestRenderSkillDirsSkillBodyPreservesMarkdownWhitespace(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	writeTestFile(
		t,
		filepath.Join(skillsDir, "plain", "SKILL.md"),
		"---\nname: plain\ndescription: plain guidance\n---\n    Keep {{ example }} literal.\n\nPlain hard break.  \n",
	)
	writeTestFile(
		t,
		filepath.Join(skillsDir, "templated", "SKILL.md.tmpl"),
		"---\nname: templated\ndescription: templated guidance\n---\n    Link to {{.Skill \"root\"}}.\n\nTemplated hard break.  \n",
	)
	writeTestFile(
		t,
		filepath.Join(skillsDir, "root", "SKILL.md.tmpl"),
		"---\nname: root\ndescription: root guidance\n---\n\nBefore plain\n{{.SkillBody \"plain\"}}After plain\n\nBefore templated\n{{.SkillBody \"templated\"}}After templated\n",
	)

	dst := filepath.Join(t.TempDir(), "skills")
	if err := RenderSkillDirs(skillsDir, dst, SkillRefMDC); err != nil {
		t.Fatalf("RenderSkillDirs: %v", err)
	}
	rendered, err := os.ReadFile(filepath.Join(dst, "root", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading rendered skill: %v", err)
	}
	got := string(rendered)
	wantPlain := "Before plain\n    Keep {{ example }} literal.\n\nPlain hard break.  \nAfter plain"
	if !strings.Contains(got, wantPlain) {
		t.Errorf("rendered skill changed plain Markdown whitespace\nwant substring: %q\ngot:\n%s", wantPlain, got)
	}
	wantTemplated := "Before templated\n    Link to [root](../root/SKILL.md).\n\nTemplated hard break.  \nAfter templated"
	if !strings.Contains(got, wantTemplated) {
		t.Errorf("rendered skill changed templated Markdown whitespace\nwant substring: %q\ngot:\n%s", wantTemplated, got)
	}
}

func TestRenderSkillDirsBodyEmbeddingRejectsCycles(t *testing.T) {
	testCases := []struct {
		name      string
		rootSkill string
		rules     map[string]string
		skills    map[string]string
		wantCycle string
	}{
		{
			name:      "direct skill cycle",
			rootSkill: "a",
			skills: map[string]string{
				"a": "{{.SkillBody \"a\"}}",
			},
			wantCycle: "skill:a -> skill:a",
		},
		{
			name:      "nested skill cycle",
			rootSkill: "a",
			skills: map[string]string{
				"a": "{{.SkillBody \"b\"}}",
				"b": "{{.SkillBody \"a\"}}",
			},
			wantCycle: "skill:a -> skill:b -> skill:a",
		},
		{
			name:      "nested rule cycle",
			rootSkill: "root",
			rules: map[string]string{
				"a": "{{.RuleBody \"b\"}}",
				"b": "{{.RuleBody \"a\"}}",
			},
			skills: map[string]string{
				"root": "{{.RuleBody \"a\"}}",
			},
			wantCycle: "rule:a -> rule:b -> rule:a",
		},
		{
			name:      "cross type cycle",
			rootSkill: "a",
			rules: map[string]string{
				"bridge": "{{.SkillBody \"a\"}}",
			},
			skills: map[string]string{
				"a": "{{.RuleBody \"bridge\"}}",
			},
			wantCycle: "skill:a -> rule:bridge -> skill:a",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			for name, body := range testCase.rules {
				content := "---\ndescription: test rule\n---\n" + body + "\n"
				writeTestFile(t, filepath.Join(root, "rules", name+".mdc"), content)
			}
			skillsDir := filepath.Join(root, "skills")
			for name, body := range testCase.skills {
				content := "---\nname: " + name + "\ndescription: test skill\n---\n\n" + body + "\n"
				writeTestFile(t, filepath.Join(skillsDir, name, "SKILL.md.tmpl"), content)
			}

			dst := filepath.Join(t.TempDir(), "skills")
			err := RenderSkillDirs(skillsDir, dst, SkillRefMDC)
			if err == nil {
				t.Fatal("expected RenderSkillDirs to fail on cyclic body embedding, got nil")
			}
			if !strings.Contains(err.Error(), testCase.wantCycle) {
				t.Errorf("expected cycle %q in error, got: %v", testCase.wantCycle, err)
			}
			if _, statErr := os.Stat(filepath.Join(dst, testCase.rootSkill, "SKILL.md")); !os.IsNotExist(statErr) {
				t.Errorf("expected no rendered root skill, stat error: %v", statErr)
			}
		})
	}
}

func TestRenderSkillDirsSkillBodyRejectsInvalidNames(t *testing.T) {
	testCases := []struct {
		name      string
		reference string
		wantError string
	}{
		{name: "missing", reference: "missing", wantError: "unknown skill body"},
		{name: "traversal", reference: "../secrets", wantError: "invalid skill body name"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			skillsDir := filepath.Join(root, "skills")
			content := "---\nname: root\ndescription: test skill\n---\n\n{{.SkillBody \"" + testCase.reference + "\"}}\n"
			writeTestFile(t, filepath.Join(skillsDir, "root", "SKILL.md.tmpl"), content)

			err := RenderSkillDirs(skillsDir, filepath.Join(t.TempDir(), "skills"), SkillRefMDC)
			if err == nil {
				t.Fatal("expected RenderSkillDirs to reject invalid SkillBody reference, got nil")
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Errorf("expected %q in error, got: %v", testCase.wantError, err)
			}
		})
	}
}

func TestRenderSkillDirsRuleBodyMissing(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "rules", "code.mdc"), "---\ndescription: c\n---\ncode body\n")
	skillsDir := filepath.Join(root, "skills")
	writeTestFile(
		t,
		filepath.Join(skillsDir, "broken", "SKILL.md.tmpl"),
		"---\nname: broken\ndescription: d\n---\n\n{{.RuleBody \"missing\"}}\n",
	)
	dst := filepath.Join(t.TempDir(), "skills")
	err := RenderSkillDirs(skillsDir, dst, SkillRefMDC)
	if err == nil {
		t.Fatal("expected RenderSkillDirs to fail on missing rule body, got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("expected missing rule name in error, got: %v", err)
	}
}

func TestRenderSkillDirsRuleBodyRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "rules", "code.mdc"), "---\ndescription: c\n---\ncode body\n")
	skillsDir := filepath.Join(root, "skills")
	writeTestFile(
		t,
		filepath.Join(skillsDir, "broken", "SKILL.md.tmpl"),
		"---\nname: broken\ndescription: d\n---\n\n{{.RuleBody \"../secrets\"}}\n",
	)
	dst := filepath.Join(t.TempDir(), "skills")
	err := RenderSkillDirs(skillsDir, dst, SkillRefMDC)
	if err == nil {
		t.Fatal("expected RenderSkillDirs to fail on traversal rule name, got nil")
	}
	if !strings.Contains(err.Error(), "invalid rule body name") {
		t.Errorf("expected invalid rule body name in error, got: %v", err)
	}
}

func TestRenderRulesForUpload(t *testing.T) {
	srcRoot := t.TempDir()
	writeTestFile(t, filepath.Join(srcRoot, "writing.mdc"),
		"---\ndescription: Writing rules\n---\n\nSee {{.Skill \"make-readable\"}} for help.\n")
	writeTestFile(t, filepath.Join(srcRoot, "general.mdc"),
		"---\ndescription: Global rules\n---\n\nGlobal body\n")

	style := RuleRenderStyle{SkillsRelDir: "../skills"}
	rules, err := RenderRulesForUpload(srcRoot, style)
	if err != nil {
		t.Fatalf("RenderRulesForUpload: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	byTitle := map[string]RenderedRule{}
	for _, rule := range rules {
		byTitle[rule.Title] = rule
	}

	general, ok := byTitle["general"]
	if !ok {
		t.Fatal("expected general rule")
	}
	if general.Body != "Global body" {
		t.Errorf("general body\nwant: %q\ngot:  %q", "Global body", general.Body)
	}
	if strings.Contains(general.Body, "---") {
		t.Errorf("general body should not contain frontmatter:\n%s", general.Body)
	}

	writing, ok := byTitle["writing"]
	if !ok {
		t.Fatal("expected writing rule")
	}
	want := "See [make-readable](../skills/make-readable/SKILL.md) for help."
	if writing.Body != want {
		t.Errorf("writing body\nwant: %q\ngot:  %q", want, writing.Body)
	}
}
