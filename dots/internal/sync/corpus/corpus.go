// Package corpus renders the agent corpus to user-wide destinations from a
// declarative TOML manifest. Each manifest output names a kind, a destination,
// and the parameters that kind needs; the engine dispatches every output to the
// matching compilation primitive.
package corpus

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml/v2"

	"goodkind.io/.dotfiles/internal/sync/compilation"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// ManifestName is the manifest file expected at the corpus root.
const ManifestName = "targets.toml"

// OutputKind is the artifact kind an output produces.
type OutputKind string

const (
	// KindSkills renders skill directories from corpus/skills.
	KindSkills OutputKind = "skills"
	// KindRuleFiles writes each rule as a marker-stamped file.
	KindRuleFiles OutputKind = "rule-files"
	// KindInstructionDoc concatenates all rules into one document.
	KindInstructionDoc OutputKind = "instruction-doc"
	// KindPerFileInstructions renders each rule as a .instructions.md file.
	KindPerFileInstructions OutputKind = "per-file-instructions"
	// KindCodexRules concatenates all rules into the Codex .rules format.
	KindCodexRules OutputKind = "codex-rules"
)

// RefStyleName names a skill rule-reference style.
type RefStyleName string

const (
	// RefMDC links rules to sibling .mdc files.
	RefMDC RefStyleName = "mdc"
	// RefMD links rules to sibling .md files.
	RefMD RefStyleName = "md"
	// RefInstructions links rules to .instructions.md files.
	RefInstructions RefStyleName = "instructions"
	// RefCodexDoc links rules to anchors in the Codex AGENTS.md document.
	RefCodexDoc RefStyleName = "codex-doc"
)

// Output is one declarative fan-out artifact.
type Output struct {
	Provider  string       `toml:"provider"`
	Kind      OutputKind   `toml:"kind"`
	Dest      string       `toml:"dest"`
	RefStyle  RefStyleName `toml:"ref_style"`
	RuleExt   string       `toml:"rule_ext"`
	SkillDest string       `toml:"skill_dest"`
	Title     string       `toml:"title"`
	OS        string       `toml:"os"`
}

// Manifest is the parsed targets.toml.
type Manifest struct {
	Outputs []Output `toml:"output"`
}

// LoadManifest reads and parses the manifest at the given path.
func LoadManifest(path string) (Manifest, error) {
	var manifest Manifest
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		slog.Error("corpus: reading manifest", "path", path, "err", err)
		return manifest, fmt.Errorf("reading manifest %s: %w", path, err)
	}
	if err := toml.Unmarshal(raw, &manifest); err != nil {
		slog.Error("corpus: parsing manifest", "path", path, "err", err)
		return manifest, fmt.Errorf("parsing manifest %s: %w", path, err)
	}
	return manifest, nil
}

// Sync renders every manifest output that applies to the running OS.
func Sync(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	_ = logger

	home, err := os.UserHomeDir()
	if err != nil {
		slog.ErrorContext(ctx, "corpus: resolving home directory", "err", err)
		return fmt.Errorf("resolving home directory: %w", err)
	}

	source := compilation.ResolveCorpusSource(dotfiles)
	manifest, err := LoadManifest(filepath.Join(source.Root, ManifestName))
	if err != nil {
		return err
	}

	for _, output := range manifest.Outputs {
		if output.OS != "" && output.OS != runtime.GOOS {
			continue
		}
		dest := filepath.Join(home, output.Dest)
		if err := renderOutput(source, output, home, dest); err != nil {
			return err
		}
	}
	return nil
}

func renderOutput(source compilation.CorpusPaths, output Output, home string, dest string) error {
	var err error
	switch output.Kind {
	case KindSkills:
		style, styleErr := resolveRefStyle(output.RefStyle)
		if styleErr != nil {
			return styleErr
		}
		err = compilation.RenderSkillDirs(source.Skills, dest, style)
	case KindRuleFiles:
		if output.RuleExt == "" {
			return fmt.Errorf("rule-files output for %s requires rule_ext", output.Provider)
		}
		ruleStyle, ruleStyleErr := resolveRuleRenderStyle(output, home, dest)
		if ruleStyleErr != nil {
			return ruleStyleErr
		}
		err = compilation.RenderRuleFiles(source.Rules, dest, output.RuleExt, ruleStyle)
	case KindInstructionDoc:
		if output.Title == "" {
			return fmt.Errorf("instruction-doc output for %s requires title", output.Provider)
		}
		ruleStyle, ruleStyleErr := resolveRuleRenderStyle(output, home, dest)
		if ruleStyleErr != nil {
			return ruleStyleErr
		}
		err = compilation.RenderRulesAsInstructionDoc(source.Rules, dest, output.Title, ruleStyle)
	case KindPerFileInstructions:
		ruleStyle, ruleStyleErr := resolveRuleRenderStyle(output, home, dest)
		if ruleStyleErr != nil {
			return ruleStyleErr
		}
		err = compilation.RenderCopilotInstructionFiles(source.Rules, dest, ruleStyle)
	case KindCodexRules:
		err = compilation.RenderCodexRules(source.Rules, dest)
	default:
		return fmt.Errorf("unknown output kind %q for %s", output.Kind, output.Provider)
	}
	if err != nil {
		slog.Error("corpus: rendering output", "provider", output.Provider, "kind", string(output.Kind), "dest", dest, "err", err)
		return fmt.Errorf("rendering %s %s into %s: %w", output.Provider, output.Kind, dest, err)
	}
	return nil
}

func resolveRuleRenderStyle(output Output, home string, dest string) (compilation.RuleRenderStyle, error) {
	if output.SkillDest == "" {
		return compilation.RuleRenderStyle{}, nil
	}
	var linkBase string
	switch output.Kind {
	case KindRuleFiles, KindPerFileInstructions:
		linkBase = dest
	case KindInstructionDoc:
		linkBase = filepath.Dir(dest)
	default:
		return compilation.RuleRenderStyle{}, nil
	}
	skillRoot := filepath.Join(home, output.SkillDest)
	skillsRelDir, err := filepath.Rel(linkBase, skillRoot)
	if err != nil {
		return compilation.RuleRenderStyle{}, fmt.Errorf("computing skill link path for %s: %w", output.Provider, err)
	}
	return compilation.RuleRenderStyle{SkillsRelDir: filepath.ToSlash(skillsRelDir)}, nil
}

func resolveRefStyle(name RefStyleName) (compilation.SkillRefStyle, error) {
	switch name {
	case RefMDC:
		return compilation.SkillRefMDC, nil
	case RefMD:
		return compilation.SkillRefMD, nil
	case RefInstructions:
		return compilation.SkillRefInstructions, nil
	case RefCodexDoc:
		return compilation.SkillRefCodexDoc, nil
	default:
		return compilation.SkillRefStyle{}, fmt.Errorf("unknown ref_style %q", name)
	}
}
