package corpus

import (
	"context"
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
