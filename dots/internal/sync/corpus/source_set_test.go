package corpus

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestParsesImportedSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestName)
	writeFile(t, path, strings.Join([]string{
		"[[source]]",
		"name = \"reliability\"",
		"kind = \"submodule-import\"",
		"root = \"lib/Claude-Opus-5-tools\"",
		"source_url = \"https://github.com/Lunarsong/Claude-Opus-5-tools.git\"",
		"source_branch = \"main\"",
		"working_agreement = \"working-agreements.md\"",
		"skills = [\".claude/skills/trace-the-chain/SKILL.md\"]",
		"agents = [\".claude/agents/code-implementer.md\"]",
		"read_only_agents = [\"code-implementer\"]",
	}, "\n")+"\n")

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(manifest.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(manifest.Sources))
	}
	source := manifest.Sources[0]
	if source.Name != "reliability" || source.Kind != KindSubmoduleImport {
		t.Fatalf("unexpected source parsed: %+v", source)
	}
	if source.Root != "lib/Claude-Opus-5-tools" || source.WorkingAgreement != "working-agreements.md" {
		t.Fatalf("unexpected source paths: %+v", source)
	}
	if len(source.Skills) != 1 || source.Skills[0] != ".claude/skills/trace-the-chain/SKILL.md" {
		t.Fatalf("unexpected source skills: %+v", source.Skills)
	}
	if len(source.Agents) != 1 || source.Agents[0] != ".claude/agents/code-implementer.md" {
		t.Fatalf("unexpected source agents: %+v", source.Agents)
	}
	if len(source.ReadOnlyAgents) != 1 || source.ReadOnlyAgents[0] != "code-implementer" {
		t.Fatalf("unexpected read-only agents: %+v", source.ReadOnlyAgents)
	}
}

func TestLoadSourceSetIncludesImportedArtifacts(t *testing.T) {
	dotfiles := t.TempDir()
	writeFile(t, filepath.Join(dotfiles, "corpus", "rules", "general.mdc"), strings.Join([]string{
		"---",
		"description: General rules",
		"applies_to:",
		"  - \"**/*\"",
		"always: true",
		"---",
		"General body",
	}, "\n")+"\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", "skills", "local-skill", "SKILL.md.tmpl"), strings.Join([]string{
		"---",
		"name: local-skill",
		"description: local",
		"---",
		"",
		"Local body",
	}, "\n")+"\n")
	writeFile(t, filepath.Join(dotfiles, "lib", "Claude-Opus-5-tools", "working-agreements.md"), strings.Join([]string{
		"# Working agreements",
		"",
		"```markdown",
		"## Subagent Working Agreements",
		"",
		"- Verify the brief first.",
		"```",
	}, "\n")+"\n")
	writeFile(t, filepath.Join(dotfiles, "lib", "Claude-Opus-5-tools", ".claude", "skills", "trace-the-chain", "SKILL.md"), strings.Join([]string{
		"---",
		"name: trace-the-chain",
		"description: imported skill",
		"---",
		"",
		"Trace body",
	}, "\n")+"\n")
	writeFile(t, filepath.Join(dotfiles, "lib", "Claude-Opus-5-tools", ".claude", "agents", "code-implementer.md"), strings.Join([]string{
		"---",
		"name: code-implementer",
		"description: imported agent",
		"---",
		"",
		"Implement body",
	}, "\n")+"\n")
	manifest := Manifest{
		Sources: []Source{{
			Name:             "reliability",
			Kind:             KindSubmoduleImport,
			Root:             "lib/Claude-Opus-5-tools",
			WorkingAgreement: "working-agreements.md",
			Skills:           []string{".claude/skills/trace-the-chain/SKILL.md"},
			Agents:           []string{".claude/agents/code-implementer.md"},
			ReadOnlyAgents:   []string{"code-implementer"},
		}},
	}

	sourceSet, err := loadSourceSet(dotfiles, manifest)
	if err != nil {
		t.Fatalf("loadSourceSet() returned error: %v", err)
	}
	if _, ok := sourceSet.Rules["general"]; !ok {
		t.Fatalf("local rules missing from source set: %#v", sourceSet.Rules)
	}
	workingAgreements, ok := sourceSet.Rules["working-agreements"]
	if !ok {
		t.Fatalf("imported working agreement rule missing: %#v", sourceSet.Rules)
	}
	if !workingAgreements.Always {
		t.Fatalf("working agreements rule should always apply: %+v", workingAgreements)
	}
	if !strings.Contains(workingAgreements.Body, "## Subagent Working Agreements") {
		t.Fatalf("working agreements body was not extracted from fenced block: %q", workingAgreements.Body)
	}
	traceSkill, ok := sourceSet.Skills["trace-the-chain"]
	if !ok {
		t.Fatalf("imported skill missing from source set: %#v", sourceSet.Skills)
	}
	if traceSkill.Files["SKILL.md"] == "" {
		t.Fatalf("imported skill files missing SKILL.md: %#v", traceSkill.Files)
	}
	codeImplementer, ok := sourceSet.Agents["code-implementer"]
	if !ok {
		t.Fatalf("imported agent missing from source set: %#v", sourceSet.Agents)
	}
	if codeImplementer.Description != "imported agent" || !strings.Contains(codeImplementer.Body, "Implement body") {
		t.Fatalf("unexpected imported agent content: %+v", codeImplementer)
	}
	if !codeImplementer.ReadOnly {
		t.Fatal("imported agent did not preserve manifest read-only metadata")
	}
}

func TestLoadSourceSetRejectsDuplicateSkillNames(t *testing.T) {
	dotfiles := t.TempDir()
	writeFile(t, filepath.Join(dotfiles, "corpus", "skills", "trace-the-chain", "SKILL.md.tmpl"), strings.Join([]string{
		"---",
		"name: trace-the-chain",
		"description: local duplicate",
		"---",
		"",
		"Local duplicate",
	}, "\n")+"\n")
	writeFile(t, filepath.Join(dotfiles, "lib", "Claude-Opus-5-tools", ".claude", "skills", "trace-the-chain", "SKILL.md"), strings.Join([]string{
		"---",
		"name: trace-the-chain",
		"description: imported duplicate",
		"---",
		"",
		"Imported duplicate",
	}, "\n")+"\n")
	manifest := Manifest{
		Sources: []Source{{
			Name:   "reliability",
			Kind:   KindSubmoduleImport,
			Root:   "lib/Claude-Opus-5-tools",
			Skills: []string{".claude/skills/trace-the-chain/SKILL.md"},
		}},
	}

	_, err := loadSourceSet(dotfiles, manifest)
	if err == nil {
		t.Fatal("loadSourceSet() returned nil, want duplicate skill error")
	}
	if !strings.Contains(err.Error(), "duplicate skill") || !strings.Contains(err.Error(), "trace-the-chain") {
		t.Fatalf("duplicate skill error = %v, want trace-the-chain duplicate", err)
	}
}

func TestLoadSourceSetRejectsEscapedImportedRoot(t *testing.T) {
	dotfiles := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, ".claude", "agents", "code-implementer.md"), strings.Join([]string{
		"---",
		"name: code-implementer",
		"description: imported agent",
		"---",
		"",
		"Implement body",
	}, "\n")+"\n")
	manifest := Manifest{
		Sources: []Source{{
			Name:   "reliability",
			Kind:   KindSubmoduleImport,
			Root:   filepath.Join("..", filepath.Base(outside)),
			Agents: []string{".claude/agents/code-implementer.md"},
		}},
	}

	_, err := loadSourceSet(dotfiles, manifest)
	if err == nil {
		t.Fatal("loadSourceSet() returned nil, want escaped root error")
	}
	if !strings.Contains(err.Error(), "escapes dotfiles root") {
		t.Fatalf("escaped root error = %v, want containment failure", err)
	}
}

func TestLoadSourceSetRejectsImportedPromptNameTraversal(t *testing.T) {
	dotfiles := t.TempDir()
	writeFile(t, filepath.Join(dotfiles, "lib", "Claude-Opus-5-tools", ".claude", "agents", "code-implementer.md"), strings.Join([]string{
		"---",
		"name: ../../outside",
		"description: imported agent",
		"---",
		"",
		"Implement body",
	}, "\n")+"\n")
	manifest := Manifest{
		Sources: []Source{{
			Name:   "reliability",
			Kind:   KindSubmoduleImport,
			Root:   "lib/Claude-Opus-5-tools",
			Agents: []string{".claude/agents/code-implementer.md"},
		}},
	}

	_, err := loadSourceSet(dotfiles, manifest)
	if err == nil {
		t.Fatal("loadSourceSet() returned nil, want invalid prompt name error")
	}
	if !strings.Contains(err.Error(), "invalid prompt name") {
		t.Fatalf("invalid prompt name error = %v, want prompt name validation", err)
	}
}

func TestLoadSourceSetRejectsUnmatchedReadOnlyAgent(t *testing.T) {
	dotfiles := t.TempDir()
	agentPath := filepath.Join(dotfiles, "lib", "reliability", "agents", "evidence.md")
	writeFile(t, agentPath, strings.Join([]string{
		"---",
		"name: renamed-evidence-gatherer",
		"description: imported agent",
		"---",
		"",
		"Gather evidence",
	}, "\n")+"\n")
	manifest := Manifest{
		Sources: []Source{{
			Name:           "reliability",
			Kind:           KindSubmoduleImport,
			Root:           "lib/reliability",
			Agents:         []string{"agents/evidence.md"},
			ReadOnlyAgents: []string{"evidence-gatherer"},
		}},
	}

	_, err := loadSourceSet(dotfiles, manifest)
	if err == nil {
		t.Fatal("loadSourceSet() returned nil, want unmatched read-only agent error")
	}
	if !strings.Contains(err.Error(), "read-only agent") || !strings.Contains(err.Error(), "evidence-gatherer") {
		t.Fatalf("loadSourceSet() error = %v, want unmatched read-only agent", err)
	}
}
