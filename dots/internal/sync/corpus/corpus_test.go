package corpus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	for _, name := range []RefStyleName{RefMDC, RefMD, RefInstructions, RefCodexDoc} {
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
	writeFile(t, filepath.Join(dotfiles, "corpus", "rules", "code.mdc"), "---\ndescription: c\n---\ncode body\n")
	writeFile(t, filepath.Join(dotfiles, "corpus", "rules", "writing.mdc"), "---\ndescription: w\n---\nSkill: {{.Skill \"make-readable\"}}\n")
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
