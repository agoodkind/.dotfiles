package corpus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
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

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestResolveRefStyle(t *testing.T) {
	for _, name := range []RefStyleName{RefMDC, RefMD} {
		if _, err := resolveRefStyle(name); err != nil {
			t.Errorf("resolveRefStyle(%q) unexpected error: %v", name, err)
		}
	}
	if _, err := resolveRefStyle("nope"); err == nil {
		t.Errorf("resolveRefStyle(\"nope\") expected error, got nil")
	}
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestName)
	writeFile(t, path, "[[output]]\nprovider = \"claude\"\nkind = \"skills\"\ndest = \".claude/skills\"\nref_style = \"md\"\n")
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(manifest.Outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(manifest.Outputs))
	}
	out := manifest.Outputs[0]
	if out.Provider != "claude" || out.Kind != KindSkills || out.RefStyle != RefMD {
		t.Errorf("unexpected parsed output: %+v", out)
	}
}

func TestSyncRendersAndGatesByOS(t *testing.T) {
	dotfiles := t.TempDir()
	writeFile(t, filepath.Join(dotfiles, "corpus", "rules", "code.mdc"), "---\ndescription: c\napplies_to:\n  - \"*.go\"\nalways: false\n---\ncode body\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", "rules", "writing.mdc"), "---\ndescription: w\napplies_to:\n  - \"*.md\"\nalways: false\n---\nSkill: {{.Skill \"make-readable\"}}\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", "skills", "enforce-rules", "SKILL.md.tmpl"), "---\nname: enforce-rules\n---\n\nOne: {{.Rule \"code\"}}\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", "skills", "make-readable", "SKILL.md.tmpl"), "---\nname: make-readable\n---\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", ManifestName),
		"[[output]]\nprovider=\"claude\"\nkind=\"skills\"\ndest=\".claude/skills\"\nref_style=\"md\"\n\n"+
			"[[output]]\nprovider=\"claude\"\nkind=\"rule-files\"\ndest=\".claude/rules\"\nrule_ext=\".md\"\nskill_dest=\".claude/skills\"\n\n"+
			"[[output]]\nprovider=\"claude\"\nkind=\"instruction-doc\"\ndest=\".claude/CLAUDE.md\"\ntitle=\"Claude Memory\"\nskill_dest=\".claude/skills\"\n\n"+
			"[[output]]\nprovider=\"never\"\nkind=\"instruction-doc\"\ndest=\".never/NEVER.md\"\ntitle=\"Never\"\nos=\"plan9\"\n")

	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Sync(context.Background(), dotfiles, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	skill := filepath.Join(home, ".claude", "skills", "enforce-rules", "SKILL.md")
	got, err := os.ReadFile(skill)
	if err != nil {
		t.Fatalf("reading rendered skill: %v", err)
	}
	if want := "[code.md](../../rules/code.md)"; !strings.Contains(string(got), want) {
		t.Errorf("rendered skill missing expanded rule link %q:\n%s", want, string(got))
	}

	rule, err := os.ReadFile(filepath.Join(home, ".claude", "rules", "writing.md"))
	if err != nil {
		t.Fatalf("reading rendered rule: %v", err)
	}
	if want := "[make-readable](../skills/make-readable/SKILL.md)"; !strings.Contains(string(rule), want) {
		t.Errorf("rendered rule missing expanded skill link %q:\n%s", want, string(rule))
	}

	doc, err := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	if want := "[make-readable](skills/make-readable/SKILL.md)"; !strings.Contains(string(doc), want) {
		t.Errorf("instruction doc missing expanded skill link %q:\n%s", want, string(doc))
	}
	if _, err := os.Stat(filepath.Join(home, ".never", "NEVER.md")); !os.IsNotExist(err) {
		t.Errorf("expected os-gated output to be skipped, stat err: %v", err)
	}
}

func TestSyncRendersProviderFrontmatter(t *testing.T) {
	dotfiles := t.TempDir()
	writeFile(t, filepath.Join(dotfiles, "corpus", "rules", "writing.mdc"),
		"---\ndescription: Writing rules\napplies_to:\n  - \"*.md\"\nalways: false\n---\n\nBody text\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", ManifestName),
		"[[output]]\nprovider=\"cursor\"\nkind=\"rule-files\"\ndest=\".cursor/rules\"\nrule_ext=\".mdc\"\n\n"+
			"[[output]]\nprovider=\"claude\"\nkind=\"rule-files\"\ndest=\".claude/rules\"\nrule_ext=\".md\"\n\n"+
			"[[output]]\nprovider=\"copilot\"\nkind=\"per-file-instructions\"\ndest=\".copilot/instructions\"\n")

	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Sync(context.Background(), dotfiles, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	cursorRule, err := os.ReadFile(filepath.Join(home, ".cursor", "rules", "writing.mdc"))
	if err != nil {
		t.Fatalf("reading cursor rule: %v", err)
	}
	if frontmatterStringField(t, string(cursorRule), "globs") != "*.md" {
		t.Errorf("cursor rule globs mismatch:\n%s", string(cursorRule))
	}

	claudeRule, err := os.ReadFile(filepath.Join(home, ".claude", "rules", "writing.md"))
	if err != nil {
		t.Fatalf("reading claude rule: %v", err)
	}
	claudePaths := frontmatterStringSliceField(t, string(claudeRule), "paths")
	if len(claudePaths) != 1 || claudePaths[0] != "*.md" {
		t.Errorf("claude rule paths = %#v, want [\"*.md\"]", claudePaths)
	}

	copilotRule, err := os.ReadFile(filepath.Join(home, ".copilot", "instructions", "writing.instructions.md"))
	if err != nil {
		t.Fatalf("reading copilot instruction: %v", err)
	}
	if frontmatterStringField(t, string(copilotRule), "name") != "writing" {
		t.Errorf("copilot name mismatch:\n%s", string(copilotRule))
	}
	if frontmatterStringField(t, string(copilotRule), "description") != "Writing rules" {
		t.Errorf("copilot description mismatch:\n%s", string(copilotRule))
	}
	if frontmatterStringField(t, string(copilotRule), "applyTo") != "*.md" {
		t.Errorf("copilot applyTo mismatch:\n%s", string(copilotRule))
	}
}

func TestSyncRendersImportedAgentsAndFallbackSkills(t *testing.T) {
	dotfiles := t.TempDir()
	importedRoot := filepath.Join(dotfiles, "lib", "reliability")
	writeFile(t, filepath.Join(importedRoot, "working-agreements.md"), "# Agreements\n\n```text\nVerify the premise.\n```\n")
	writeFile(t, filepath.Join(importedRoot, "skills", "trace", "SKILL.md"), "---\nname: trace-the-chain\ndescription: Trace evidence.\n---\n\nTrace body.\n")
	writeFile(t, filepath.Join(importedRoot, "agents", "code.md"), "---\nname: code-implementer\ndescription: Implement settled designs.\n---\n\nImplement body.\n")
	writeFile(t, filepath.Join(importedRoot, "agents", "evidence.md"), "---\nname: evidence-gatherer\ndescription: Gather cited evidence.\n---\n\nGather body.\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", ManifestName), `[[source]]
name = "reliability"
kind = "submodule-import"
root = "lib/reliability"
working_agreement = "working-agreements.md"
skills = ["skills/trace/SKILL.md"]
agents = ["agents/code.md", "agents/evidence.md"]
read_only_agents = ["evidence-gatherer"]

[[output]]
provider = "agents"
kind = "skills"
dest = ".agents/skills"
ref_style = "md"

[[output]]
provider = "claude"
kind = "agents"
dest = ".claude/agents"

[[output]]
provider = "cursor"
kind = "agents"
dest = ".cursor/agents"

[[output]]
provider = "gemini"
kind = "agents"
dest = ".gemini/agents"

[[output]]
provider = "codex"
kind = "agents"
dest = ".codex/agents"

[[output]]
provider = "copilot"
kind = "agents"
dest = ".copilot/agents"
`)

	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := Sync(context.Background(), dotfiles, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	assertions := map[string]string{
		filepath.Join(home, ".agents", "skills", "trace-the-chain", "SKILL.md"):         "Trace body.",
		filepath.Join(home, ".agents", "skills", "agent-code-implementer", "SKILL.md"):  "does not provide prompt-bearing subagent isolation",
		filepath.Join(home, ".agents", "skills", "agent-evidence-gatherer", "SKILL.md"): "This fallback is read-only.",
		filepath.Join(home, ".claude", "agents", "code-implementer.md"):                 "Implement body.",
		filepath.Join(home, ".cursor", "agents", "evidence-gatherer.md"):                "readonly: true",
		filepath.Join(home, ".gemini", "agents", "code-implementer.md"):                 "Implement body.",
		filepath.Join(home, ".codex", "agents", "evidence-gatherer.toml"):               "Gather body.",
		filepath.Join(home, ".copilot", "agents", "code-implementer.agent.md"):          "Implement body.",
	}
	for path, expected := range assertions {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		if !strings.Contains(string(content), expected) {
			t.Errorf("%s missing %q:\n%s", path, expected, string(content))
		}
	}
	codexPath := filepath.Join(home, ".codex", "agents", "evidence-gatherer.toml")
	codexContent, err := os.ReadFile(codexPath)
	if err != nil {
		t.Fatalf("reading %s: %v", codexPath, err)
	}
	var codexAgent struct {
		SandboxMode string `toml:"sandbox_mode"`
	}
	if err := toml.Unmarshal(codexContent, &codexAgent); err != nil {
		t.Fatalf("decoding %s: %v", codexPath, err)
	}
	if codexAgent.SandboxMode != "read-only" {
		t.Fatalf("Codex sandbox_mode = %q, want read-only", codexAgent.SandboxMode)
	}
}

func TestSyncValidationFailurePreservesExistingOutputs(t *testing.T) {
	dotfiles := t.TempDir()
	importedRoot := filepath.Join(dotfiles, "lib", "reliability")
	writeFile(t, filepath.Join(importedRoot, "agents", "broken.md"), "missing front matter\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", ManifestName), `[[source]]
name = "reliability"
kind = "submodule-import"
root = "lib/reliability"
agents = ["agents/broken.md"]

[[output]]
provider = "claude"
kind = "agents"
dest = ".claude/agents"
`)

	home := t.TempDir()
	t.Setenv("HOME", home)
	existingPath := filepath.Join(home, ".claude", "agents", "code-implementer.md")
	const existingContent = "existing managed output\n"
	writeFile(t, existingPath, existingContent)

	if err := Sync(context.Background(), dotfiles, nil); err == nil {
		t.Fatal("Sync expected validation error, got nil")
	}
	content, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("reading existing output: %v", err)
	}
	if string(content) != existingContent {
		t.Errorf("existing output changed after validation failure:\n%s", string(content))
	}
}

func TestSyncPreflightUsesExistingDestinations(t *testing.T) {
	dotfiles := t.TempDir()
	importedRoot := filepath.Join(dotfiles, "lib", "reliability")
	writeFile(t, filepath.Join(importedRoot, "agents", "code.md"), "---\nname: code-implementer\ndescription: Implement code.\n---\n\nImplement.\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", "skills", "conflict", "SKILL.md"), "---\nname: conflict\ndescription: Conflict.\n---\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", ManifestName), `[[source]]
name = "reliability"
kind = "submodule-import"
root = "lib/reliability"
agents = ["agents/code.md"]

[[output]]
provider = "claude"
kind = "agents"
dest = ".claude/agents"

[[output]]
provider = "claude"
kind = "skills"
dest = ".claude/skills"
ref_style = "md"
`)

	home := t.TempDir()
	t.Setenv("HOME", home)
	agentPath := filepath.Join(home, ".claude", "agents", "code-implementer.md")
	const existingAgent = "existing agent\n"
	writeFile(t, agentPath, existingAgent)
	conflictPath := filepath.Join(home, ".claude", "skills", "conflict", "SKILL.md")
	writeFile(t, conflictPath, "user-owned skill\n")

	err := Sync(context.Background(), dotfiles, nil)
	if err == nil {
		t.Fatal("Sync() returned nil, want existing destination conflict")
	}
	if !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("Sync() error = %v, want conflict", err)
	}
	content, readErr := os.ReadFile(agentPath)
	if readErr != nil {
		t.Fatalf("reading existing agent: %v", readErr)
	}
	if string(content) != existingAgent {
		t.Fatalf("earlier output changed before conflict: %s", content)
	}
}

func TestSyncRejectsEscapingOutputDestination(t *testing.T) {
	dotfiles := t.TempDir()
	writeFile(t, filepath.Join(dotfiles, "corpus", ManifestName), "[[output]]\nprovider=\"claude\"\nkind=\"agents\"\ndest=\"../outside\"\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", "rules", "general.mdc"), "---\ndescription: General\napplies_to:\n  - \"**/*\"\nalways: true\n---\nGeneral body\n")

	home := t.TempDir()
	t.Setenv("HOME", home)

	err := Sync(context.Background(), dotfiles, nil)
	if err == nil {
		t.Fatal("Sync() returned nil, want output destination error")
	}
	if !strings.Contains(err.Error(), "escapes home directory") {
		t.Fatalf("Sync() error = %v, want destination containment failure", err)
	}
}

func TestResolveOutputDestRejectsHomeDirectory(t *testing.T) {
	home := t.TempDir()
	for _, destination := range []string{"", "."} {
		t.Run(destination, func(t *testing.T) {
			_, err := resolveOutputDest(home, destination)
			if err == nil {
				t.Fatalf("resolveOutputDest(%q) returned nil, want strict descendant error", destination)
			}
			if !strings.Contains(err.Error(), "must name a path below") {
				t.Fatalf("resolveOutputDest(%q) error = %v, want strict descendant failure", destination, err)
			}
		})
	}
}

func TestResolveOutputDestRejectsSymlinkEscape(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	linkPath := filepath.Join(home, "linked")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("creating destination symlink: %v", err)
	}

	_, err := resolveOutputDest(home, filepath.Join("linked", "agents"))
	if err == nil {
		t.Fatal("resolveOutputDest() returned nil, want symlink containment error")
	}
	if !strings.Contains(err.Error(), "resolves outside home directory") {
		t.Fatalf("resolveOutputDest() error = %v, want resolved containment failure", err)
	}
}

func TestResolveOutputDestRejectsDestinationSymlinkEscape(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatalf("creating provider directory: %v", err)
	}
	destination := filepath.Join(home, ".codex", "agents")
	if err := os.Symlink(outside, destination); err != nil {
		t.Fatalf("creating destination symlink: %v", err)
	}

	_, err := resolveOutputDest(home, filepath.Join(".codex", "agents"))
	if err == nil {
		t.Fatal("resolveOutputDest() returned nil, want destination symlink error")
	}
	if !strings.Contains(err.Error(), "resolves outside home directory") {
		t.Fatalf("resolveOutputDest() error = %v, want destination symlink failure", err)
	}
}
