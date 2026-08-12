package compilation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestRenderSkillDirsFromSourceSetRendersTemplatesAndImportedSkills(t *testing.T) {
	sourceSet := CorpusSourceSet{
		Rules: map[string]RuleSource{
			"code":    {Body: "code body"},
			"general": {Body: "general body"},
		},
		Skills: map[string]SkillSource{
			"local-skill": {
				Name: "local-skill",
				Files: map[string]string{
					"SKILL.md.tmpl": strings.Join([]string{
						"---",
						"name: local-skill",
						"description: local",
						"---",
						"",
						"Rules:",
						"",
						"{{.Rules}}",
					}, "\n") + "\n",
				},
			},
			"trace-the-chain": {
				Name: "trace-the-chain",
				Files: map[string]string{
					"SKILL.md": strings.Join([]string{
						"---",
						"name: trace-the-chain",
						"description: imported",
						"---",
						"",
						"Imported body",
					}, "\n") + "\n",
				},
			},
		},
	}

	dst := filepath.Join(t.TempDir(), "skills")
	if err := RenderSkillDirsFromSourceSet(sourceSet, dst, SkillRefMDC); err != nil {
		t.Fatalf("RenderSkillDirsFromSourceSet: %v", err)
	}

	localSkill, err := os.ReadFile(filepath.Join(dst, "local-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading rendered local skill: %v", err)
	}
	if !strings.Contains(string(localSkill), "[code.mdc](../../rules/code.mdc)") {
		t.Fatalf("rendered local skill missing rule link:\n%s", localSkill)
	}
	if strings.Contains(string(localSkill), "{{") {
		t.Fatalf("rendered local skill still contains template tokens:\n%s", localSkill)
	}
	if !HasGeneratedMarker(string(localSkill)) {
		t.Fatalf("rendered local skill missing generated marker:\n%s", localSkill)
	}

	importedSkill, err := os.ReadFile(filepath.Join(dst, "trace-the-chain", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading imported skill: %v", err)
	}
	if !strings.Contains(string(importedSkill), "Imported body") {
		t.Fatalf("imported skill body missing:\n%s", importedSkill)
	}
	if !HasGeneratedMarker(string(importedSkill)) {
		t.Fatalf("imported skill missing generated marker:\n%s", importedSkill)
	}
}

func TestRenderAgentFilesWritesProviderSpecificFormats(t *testing.T) {
	testCases := []struct {
		name         string
		format       AgentTargetFormat
		fileName     string
		wantContains []string
	}{
		{
			name:     "claude",
			format:   AgentTargetClaude,
			fileName: "evidence-gatherer.md",
			wantContains: []string{
				"name: evidence-gatherer",
				"description: Evidence specialist",
				"- Read",
				"Gather evidence",
			},
		},
		{
			name:     "cursor",
			format:   AgentTargetCursor,
			fileName: "evidence-gatherer.md",
			wantContains: []string{
				"name: evidence-gatherer",
				"readonly: true",
				"Gather evidence",
			},
		},
		{
			name:     "gemini",
			format:   AgentTargetGemini,
			fileName: "evidence-gatherer.md",
			wantContains: []string{
				"name: evidence-gatherer",
				"description: Evidence specialist",
				"- read_file",
				"Gather evidence",
			},
		},
		{
			name:     "codex",
			format:   AgentTargetCodex,
			fileName: "evidence-gatherer.toml",
			wantContains: []string{
				"name = ",
				"sandbox_mode = ",
				"developer_instructions = ",
				"Gather evidence",
			},
		},
		{
			name:     "copilot",
			format:   AgentTargetCopilot,
			fileName: "evidence-gatherer.agent.md",
			wantContains: []string{
				"name: evidence-gatherer",
				"description: Evidence specialist",
				"- read",
				"Gather evidence",
			},
		},
	}

	agents := map[string]AgentSource{
		"code-implementer": {
			Name:        "code-implementer",
			Description: "Implementation specialist",
			Body:        "Implement precisely",
		},
		"evidence-gatherer": {
			Name:        "evidence-gatherer",
			Description: "Evidence specialist",
			Body:        "Gather evidence",
			ReadOnly:    true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "agents")
			if err := RenderAgentFiles(agents, dst, testCase.format); err != nil {
				t.Fatalf("RenderAgentFiles(%s): %v", testCase.format, err)
			}
			payload, err := os.ReadFile(filepath.Join(dst, testCase.fileName))
			if err != nil {
				t.Fatalf("reading rendered agent: %v", err)
			}
			rendered := string(payload)
			for _, want := range testCase.wantContains {
				if !strings.Contains(rendered, want) {
					t.Fatalf("rendered agent missing %q:\n%s", want, rendered)
				}
			}
		})
	}
}

func TestRenderCodexAgentProducesValidTOML(t *testing.T) {
	agent := AgentSource{
		Name:        "evidence-gatherer",
		Description: "Evidence specialist",
		Body:        "Control byte: \x01",
		ReadOnly:    true,
	}

	rendered, err := renderAgent(agent, AgentTargetCodex)
	if err != nil {
		t.Fatalf("renderAgent: %v", err)
	}
	var decoded struct {
		Name                  string `toml:"name"`
		Description           string `toml:"description"`
		SandboxMode           string `toml:"sandbox_mode"`
		DeveloperInstructions string `toml:"developer_instructions"`
	}
	if err := toml.Unmarshal([]byte(rendered), &decoded); err != nil {
		t.Fatalf("rendered Codex agent is invalid TOML: %v\n%s", err, rendered)
	}
	if decoded.DeveloperInstructions != agent.Body {
		t.Fatalf("developer instructions = %q, want %q", decoded.DeveloperInstructions, agent.Body)
	}
}

func TestRenderSkillDirsFromSourceSetDoesNotMutateOnTemplateError(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(dst, "local-skill"), 0o755); err != nil {
		t.Fatalf("mkdir existing skill: %v", err)
	}
	const existingContent = "existing content\n"
	if err := os.WriteFile(filepath.Join(dst, "local-skill", "SKILL.md"), []byte(existingContent), 0o600); err != nil {
		t.Fatalf("write existing skill: %v", err)
	}

	sourceSet := CorpusSourceSet{
		Rules: map[string]RuleSource{},
		Skills: map[string]SkillSource{
			"broken-skill": {
				Name: "broken-skill",
				Files: map[string]string{
					"SKILL.md.tmpl": "{{.RuleBody \"missing\"}}\n",
				},
			},
			"local-skill": {
				Name: "local-skill",
				Files: map[string]string{
					"SKILL.md": "replacement\n",
				},
			},
		},
	}

	err := RenderSkillDirsFromSourceSet(sourceSet, dst, SkillRefMD)
	if err == nil {
		t.Fatal("RenderSkillDirsFromSourceSet() returned nil, want template error")
	}
	content, readErr := os.ReadFile(filepath.Join(dst, "local-skill", "SKILL.md"))
	if readErr != nil {
		t.Fatalf("reading existing skill: %v", readErr)
	}
	if string(content) != existingContent {
		t.Fatalf("existing skill changed after template error:\n%s", string(content))
	}
}

func TestRenderSkillDirsFromSourceSetDoesNotMutateOnConflict(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "skills")
	conflictPath := filepath.Join(dst, "conflicting-skill", "SKILL.md")
	const conflictContent = "invalid unmanaged skill\n"
	if err := os.MkdirAll(filepath.Dir(conflictPath), 0o755); err != nil {
		t.Fatalf("mkdir conflict skill: %v", err)
	}
	if err := os.WriteFile(conflictPath, []byte(conflictContent), 0o600); err != nil {
		t.Fatalf("write conflict skill: %v", err)
	}
	existingPath := filepath.Join(dst, "local-skill", "SKILL.md")
	const existingContent = GeneratedAgentHTMLMarker + "\nexisting managed output\n"
	if err := os.MkdirAll(filepath.Dir(existingPath), 0o755); err != nil {
		t.Fatalf("mkdir existing skill: %v", err)
	}
	if err := os.WriteFile(existingPath, []byte(existingContent), 0o600); err != nil {
		t.Fatalf("write existing skill: %v", err)
	}

	sourceSet := CorpusSourceSet{
		Rules: map[string]RuleSource{},
		Skills: map[string]SkillSource{
			"conflicting-skill": {Name: "conflicting-skill", Files: map[string]string{"SKILL.md": "replacement conflict\n"}},
			"local-skill":       {Name: "local-skill", Files: map[string]string{"SKILL.md": "replacement managed\n"}},
		},
	}

	err := RenderSkillDirsFromSourceSet(sourceSet, dst, SkillRefMD)
	if err == nil {
		t.Fatal("RenderSkillDirsFromSourceSet() returned nil, want conflict error")
	}
	content, readErr := os.ReadFile(existingPath)
	if readErr != nil {
		t.Fatalf("reading existing skill: %v", readErr)
	}
	if string(content) != existingContent {
		t.Fatalf("existing skill changed after conflict:\n%s", string(content))
	}
}

func TestRenderAgentFilesReplacesSymlinkTargetsSafely(t *testing.T) {
	agents := map[string]AgentSource{
		"evidence-gatherer": {
			Name:        "evidence-gatherer",
			Description: "Evidence specialist",
			Body:        "Gather evidence",
			ReadOnly:    true,
		},
	}
	dst := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir agents dir: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	linkPath := filepath.Join(dst, "evidence-gatherer.md")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if err := RenderAgentFiles(agents, dst, AgentTargetClaude); err != nil {
		t.Fatalf("RenderAgentFiles: %v", err)
	}
	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat rendered agent: %v", err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("rendered agent remained symlink")
	}
	outsideContent, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("read outside file: %v", err)
	}
	if string(outsideContent) != "outside\n" {
		t.Fatalf("outside file changed:\n%s", string(outsideContent))
	}
}

func TestRenderAgentFilesDoesNotMutateOnInvalidName(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir agents dir: %v", err)
	}
	const existingContent = "existing content\n"
	if err := os.WriteFile(filepath.Join(dst, "code-implementer.md"), []byte(existingContent), 0o600); err != nil {
		t.Fatalf("write existing agent: %v", err)
	}

	agents := map[string]AgentSource{
		"code-implementer": {
			Name:        "code-implementer",
			Description: "Implementation specialist",
			Body:        "Implement precisely",
		},
		"z/bad-agent": {
			Name:        "z/bad-agent",
			Description: "Bad agent",
			Body:        "Should fail",
		},
	}

	err := RenderAgentFiles(agents, dst, AgentTargetClaude)
	if err == nil {
		t.Fatal("RenderAgentFiles() returned nil, want invalid agent name error")
	}
	content, readErr := os.ReadFile(filepath.Join(dst, "code-implementer.md"))
	if readErr != nil {
		t.Fatalf("reading existing agent: %v", readErr)
	}
	if string(content) != existingContent {
		t.Fatalf("existing agent changed after validation error:\n%s", string(content))
	}
}

func TestRenderRuleFilesFromSourceSetReplacesSymlinkTargetsSafely(t *testing.T) {
	sourceSet := CorpusSourceSet{
		Rules: map[string]RuleSource{
			"general": {
				Description: "General rules",
				AppliesTo:   []string{"**/*"},
				Always:      true,
				Body:        "General body",
			},
		},
		Skills: map[string]SkillSource{},
		Agents: map[string]AgentSource{},
	}
	dst := filepath.Join(t.TempDir(), "rules")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("creating rules directory: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.mdc")
	const outsideContent = "outside\n"
	if err := os.WriteFile(outside, []byte(outsideContent), 0o600); err != nil {
		t.Fatalf("writing outside file: %v", err)
	}
	target := filepath.Join(dst, "general.mdc")
	if err := os.Symlink(outside, target); err != nil {
		t.Fatalf("creating target symlink: %v", err)
	}

	if err := RenderRuleFilesFromSourceSet(sourceSet, dst, ".mdc", RuleTargetCursor, RuleRenderStyle{}); err != nil {
		t.Fatalf("RenderRuleFilesFromSourceSet: %v", err)
	}
	outsidePayload, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("reading outside file: %v", err)
	}
	if string(outsidePayload) != outsideContent {
		t.Fatalf("outside file changed through symlink:\n%s", outsidePayload)
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("lstat rendered rule: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("rendered rule remained a symlink")
	}
}

func TestRenderSkillDirsFromSourceSetRejectsNestedSymlink(t *testing.T) {
	sourceSet := CorpusSourceSet{
		Rules: map[string]RuleSource{},
		Skills: map[string]SkillSource{
			"nested": {
				Name: "nested",
				Files: map[string]string{
					"SKILL.md":            "---\nname: nested\ndescription: Nested skill.\n---\n",
					"references/guide.md": "guide\n",
				},
			},
		},
		Agents: map[string]AgentSource{},
	}
	dst := filepath.Join(t.TempDir(), "skills")
	targetDir := filepath.Join(dst, "nested")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("creating skill directory: %v", err)
	}
	outside := t.TempDir()
	outsideGuide := filepath.Join(outside, "guide.md")
	if err := os.WriteFile(outsideGuide, []byte("outside\n"), 0o600); err != nil {
		t.Fatalf("writing outside guide: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(targetDir, "references")); err != nil {
		t.Fatalf("creating nested symlink: %v", err)
	}

	err := RenderSkillDirsFromSourceSet(sourceSet, dst, SkillRefMD)
	if err == nil {
		t.Fatal("RenderSkillDirsFromSourceSet() returned nil, want nested symlink error")
	}
	if !strings.Contains(err.Error(), "path component") {
		t.Fatalf("RenderSkillDirsFromSourceSet() error = %v, want nested symlink error", err)
	}
	content, readErr := os.ReadFile(outsideGuide)
	if readErr != nil {
		t.Fatalf("reading outside guide: %v", readErr)
	}
	if string(content) != "outside\n" {
		t.Fatalf("outside guide changed through nested symlink: %s", content)
	}
}

func TestRenderRulesAsInstructionDocFromSourceSetRemovesStaleManagedOutput(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "AGENTS.md")
	content := GeneratedAgentHTMLMarker + "\nmanaged\n"
	if err := os.WriteFile(destination, []byte(content), 0o600); err != nil {
		t.Fatalf("writing managed instruction document: %v", err)
	}
	sourceSet := CorpusSourceSet{
		Rules:  map[string]RuleSource{},
		Skills: map[string]SkillSource{},
		Agents: map[string]AgentSource{},
	}

	if err := RenderRulesAsInstructionDocFromSourceSet(
		sourceSet,
		destination,
		"Instructions",
		RuleRenderStyle{},
	); err != nil {
		t.Fatalf("RenderRulesAsInstructionDocFromSourceSet() returned error: %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("managed instruction document still exists: %v", err)
	}
}

func TestRenderRulesAsInstructionDocFromSourceSetPreservesUnmanagedOutput(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "AGENTS.md")
	const content = "user-owned\n"
	if err := os.WriteFile(destination, []byte(content), 0o600); err != nil {
		t.Fatalf("writing unmanaged instruction document: %v", err)
	}
	sourceSet := CorpusSourceSet{
		Rules:  map[string]RuleSource{},
		Skills: map[string]SkillSource{},
		Agents: map[string]AgentSource{},
	}

	if err := RenderRulesAsInstructionDocFromSourceSet(
		sourceSet,
		destination,
		"Instructions",
		RuleRenderStyle{},
	); err != nil {
		t.Fatalf("RenderRulesAsInstructionDocFromSourceSet() returned error: %v", err)
	}
	rendered, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("reading unmanaged instruction document: %v", err)
	}
	if string(rendered) != content {
		t.Fatalf("unmanaged instruction document = %q, want %q", rendered, content)
	}
}
