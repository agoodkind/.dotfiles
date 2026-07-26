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
	"strings"

	"github.com/pelletier/go-toml/v2"

	"goodkind.io/.dotfiles/internal/sync/compilation"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// ManifestName is the manifest file expected at the corpus root.
const ManifestName = "targets.toml"

// OutputKind is the artifact kind an output produces.
type OutputKind string

// SourceKind is the kind of imported corpus source to load before rendering.
type SourceKind string

// ProviderName identifies one corpus output provider.
type ProviderName string

const (
	// KindSubmoduleImport loads selected artifacts from a checked-out submodule.
	KindSubmoduleImport SourceKind = "submodule-import"

	// KindSkills renders skill directories from corpus/skills.
	KindSkills OutputKind = "skills"
	// KindRuleFiles writes each rule as a marker-stamped file.
	KindRuleFiles OutputKind = "rule-files"
	// KindInstructionDoc concatenates all rules into one document.
	KindInstructionDoc OutputKind = "instruction-doc"
	// KindPerFileInstructions renders each rule as a .instructions.md file.
	KindPerFileInstructions OutputKind = "per-file-instructions"
	// KindAgents renders native agent definitions for one provider.
	KindAgents OutputKind = "agents"
)

const (
	providerAgents  ProviderName = "agents"
	providerClaude  ProviderName = "claude"
	providerCursor  ProviderName = "cursor"
	providerGemini  ProviderName = "gemini"
	providerCodex   ProviderName = "codex"
	providerCopilot ProviderName = "copilot"
)

// RefStyleName names a skill rule-reference style.
type RefStyleName string

const (
	// RefMDC links rules to sibling .mdc files.
	RefMDC RefStyleName = "mdc"
	// RefMD links rules to sibling .md files.
	RefMD RefStyleName = "md"
)

// Output is one declarative fan-out artifact.
type Output struct {
	Provider  ProviderName `toml:"provider"`
	Kind      OutputKind   `toml:"kind"`
	Dest      string       `toml:"dest"`
	RefStyle  RefStyleName `toml:"ref_style"`
	RuleExt   string       `toml:"rule_ext"`
	SkillDest string       `toml:"skill_dest"`
	Title     string       `toml:"title"`
	OS        string       `toml:"os"`
}

// Source is one imported source declaration in the corpus manifest.
type Source struct {
	Name             string     `toml:"name"`
	Kind             SourceKind `toml:"kind"`
	Root             string     `toml:"root"`
	SourceURL        string     `toml:"source_url"`
	SourceBranch     string     `toml:"source_branch"`
	WorkingAgreement string     `toml:"working_agreement"`
	Skills           []string   `toml:"skills"`
	Agents           []string   `toml:"agents"`
	ReadOnlyAgents   []string   `toml:"read_only_agents"`
}

// Manifest is the parsed targets.toml.
type Manifest struct {
	Sources []Source `toml:"source"`
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

// LoadSourceSet loads local and imported corpus artifacts into one normalized set.
func LoadSourceSet(dotfiles string) (compilation.CorpusSourceSet, error) {
	corpusPaths := compilation.ResolveCorpusSource(dotfiles)
	manifest, err := LoadManifest(filepath.Join(corpusPaths.Root, ManifestName))
	if err != nil {
		return compilation.CorpusSourceSet{}, err
	}
	return loadSourceSet(dotfiles, manifest)
}

// Sync renders every manifest output that applies to the running OS.
func Sync(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	_ = logger

	home, err := os.UserHomeDir()
	if err != nil {
		slog.ErrorContext(ctx, "corpus: resolving home directory", "err", err)
		return fmt.Errorf("resolving home directory: %w", err)
	}

	corpusPaths := compilation.ResolveCorpusSource(dotfiles)
	manifest, err := LoadManifest(filepath.Join(corpusPaths.Root, ManifestName))
	if err != nil {
		return err
	}
	sourceSet, err := loadSourceSet(dotfiles, manifest)
	if err != nil {
		return err
	}

	if err := preflightOutputPlans(sourceSet, manifest.Outputs, home); err != nil {
		return err
	}
	for _, output := range manifest.Outputs {
		if output.OS != "" && output.OS != runtime.GOOS {
			continue
		}
		dest, err := resolveOutputDest(home, output.Dest)
		if err != nil {
			return err
		}
		if err := renderOutput(sourceSet, output, home, dest); err != nil {
			return err
		}
	}
	return nil
}

func preflightOutputPlans(sourceSet compilation.CorpusSourceSet, outputs []Output, home string) error {
	for _, output := range outputs {
		if output.OS != "" && output.OS != runtime.GOOS {
			continue
		}
		dest, err := resolveOutputDest(home, output.Dest)
		if err != nil {
			return err
		}
		if err := validateOutput(sourceSet, output, home, dest); err != nil {
			slog.Warn("corpus: preflighting output", "provider", output.Provider, "kind", output.Kind, "err", err)
			return fmt.Errorf("preflighting %s output for %s: %w", output.Kind, output.Provider, err)
		}
	}
	return nil
}

func validateOutput(sourceSet compilation.CorpusSourceSet, output Output, home string, dest string) error {
	var err error
	switch output.Kind {
	case KindSkills:
		style, styleErr := resolveRefStyle(output.RefStyle)
		if styleErr != nil {
			return styleErr
		}
		renderSourceSet := sourceSet
		if output.Provider == providerAgents {
			renderSourceSet, err = withAgentFallbackSkills(sourceSet)
			if err != nil {
				return err
			}
		}
		if err := compilation.ValidateSkillDirsFromSourceSet(renderSourceSet, dest, style); err != nil {
			slog.Warn("corpus: validating skill output", "provider", output.Provider, "err", err)
			return fmt.Errorf("validating skill output: %w", err)
		}
		return nil
	case KindRuleFiles:
		return validateRuleFileOutput(sourceSet, output, home, dest)
	case KindInstructionDoc:
		if output.Title == "" {
			return fmt.Errorf("instruction-doc output for %s requires title", output.Provider)
		}
		ruleStyle, styleErr := resolveRuleRenderStyle(output, home, dest)
		if styleErr != nil {
			return styleErr
		}
		if err := compilation.ValidateRulesAsInstructionDocFromSourceSet(sourceSet, dest, ruleStyle); err != nil {
			slog.Warn("corpus: validating instruction document", "provider", output.Provider, "err", err)
			return fmt.Errorf("validating instruction document: %w", err)
		}
		return nil
	case KindPerFileInstructions:
		ruleStyle, styleErr := resolveRuleRenderStyle(output, home, dest)
		if styleErr != nil {
			return styleErr
		}
		if err := compilation.ValidateCopilotInstructionFilesFromSourceSet(sourceSet, dest, ruleStyle); err != nil {
			slog.Warn("corpus: validating Copilot instructions", "provider", output.Provider, "err", err)
			return fmt.Errorf("validating Copilot instructions: %w", err)
		}
		return nil
	case KindAgents:
		format, formatErr := resolveAgentFormat(output.Provider)
		if formatErr != nil {
			return formatErr
		}
		if err := compilation.ValidateAgentFiles(sourceSet.Agents, dest, format); err != nil {
			slog.Warn("corpus: validating agent output", "provider", output.Provider, "err", err)
			return fmt.Errorf("validating agent output: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown output kind %q for %s", output.Kind, output.Provider)
	}
}

func validateRuleFileOutput(sourceSet compilation.CorpusSourceSet, output Output, home string, dest string) error {
	if output.RuleExt == "" {
		return fmt.Errorf("rule-files output for %s requires rule_ext", output.Provider)
	}
	ruleStyle, err := resolveRuleRenderStyle(output, home, dest)
	if err != nil {
		return err
	}
	format, err := compilation.RuleTargetFormatFromExt(output.RuleExt)
	if err != nil {
		slog.Warn("corpus: resolving rule format", "provider", output.Provider, "err", err)
		return fmt.Errorf("unsupported rule_ext %q for %s: %w", output.RuleExt, output.Provider, err)
	}
	if err := compilation.ValidateRuleFilesFromSourceSet(sourceSet, dest, output.RuleExt, format, ruleStyle); err != nil {
		slog.Warn("corpus: validating rule output", "provider", output.Provider, "err", err)
		return fmt.Errorf("validating rule output: %w", err)
	}
	return nil
}

func resolveOutputDest(home string, relativeDest string) (string, error) {
	cleanHome := filepath.Clean(home)
	cleanDest := filepath.Clean(relativeDest)

	if filepath.IsAbs(cleanDest) {
		return "", fmt.Errorf("output destination %q escapes home directory %s", relativeDest, cleanHome)
	}
	if cleanDest == "." {
		return "", fmt.Errorf("output destination %q must name a path below %s", relativeDest, cleanHome)
	}
	joined := filepath.Clean(filepath.Join(cleanHome, cleanDest))
	if !strings.HasPrefix(joined, cleanHome+string(os.PathSeparator)) {
		return "", fmt.Errorf("output destination %q escapes home directory %s", relativeDest, cleanHome)
	}
	resolvedHome, err := filepath.EvalSymlinks(cleanHome)
	if err != nil {
		slog.Warn("corpus: resolving home directory", "home", cleanHome, "err", err)
		return "", fmt.Errorf("resolving home directory %s: %w", cleanHome, err)
	}
	existingPath := joined
	for {
		resolvedParent, resolveErr := filepath.EvalSymlinks(existingPath)
		if resolveErr == nil {
			if resolvedParent != resolvedHome && !strings.HasPrefix(resolvedParent, resolvedHome+string(os.PathSeparator)) {
				return "", fmt.Errorf("output destination %q resolves outside home directory %s", relativeDest, cleanHome)
			}
			break
		}
		if !os.IsNotExist(resolveErr) {
			return "", fmt.Errorf("resolving output destination path %s: %w", existingPath, resolveErr)
		}
		nextPath := filepath.Dir(existingPath)
		if nextPath == existingPath {
			return "", fmt.Errorf("resolving output destination path %s: %w", existingPath, resolveErr)
		}
		existingPath = nextPath
	}
	return joined, nil
}

func renderOutput(sourceSet compilation.CorpusSourceSet, output Output, home string, dest string) error {
	var err error
	switch output.Kind {
	case KindSkills:
		style, styleErr := resolveRefStyle(output.RefStyle)
		if styleErr != nil {
			return styleErr
		}
		renderSourceSet := sourceSet
		if output.Provider == providerAgents {
			renderSourceSet, err = withAgentFallbackSkills(sourceSet)
			if err != nil {
				return err
			}
		}
		err = compilation.RenderSkillDirsFromSourceSet(renderSourceSet, dest, style)
	case KindRuleFiles:
		if output.RuleExt == "" {
			return fmt.Errorf("rule-files output for %s requires rule_ext", output.Provider)
		}
		ruleStyle, ruleStyleErr := resolveRuleRenderStyle(output, home, dest)
		if ruleStyleErr != nil {
			return ruleStyleErr
		}
		format, formatErr := compilation.RuleTargetFormatFromExt(output.RuleExt)
		if formatErr != nil {
			slog.Error("corpus: unsupported rule_ext", "provider", output.Provider, "rule_ext", output.RuleExt, "err", formatErr)
			return fmt.Errorf("unsupported rule_ext %q for %s: %w", output.RuleExt, output.Provider, formatErr)
		}
		err = compilation.RenderRuleFilesFromSourceSet(sourceSet, dest, output.RuleExt, format, ruleStyle)
	case KindInstructionDoc:
		if output.Title == "" {
			return fmt.Errorf("instruction-doc output for %s requires title", output.Provider)
		}
		ruleStyle, ruleStyleErr := resolveRuleRenderStyle(output, home, dest)
		if ruleStyleErr != nil {
			return ruleStyleErr
		}
		err = compilation.RenderRulesAsInstructionDocFromSourceSet(sourceSet, dest, output.Title, ruleStyle)
	case KindPerFileInstructions:
		ruleStyle, ruleStyleErr := resolveRuleRenderStyle(output, home, dest)
		if ruleStyleErr != nil {
			return ruleStyleErr
		}
		err = compilation.RenderCopilotInstructionFilesFromSourceSet(sourceSet, dest, ruleStyle)
	case KindAgents:
		format, formatErr := resolveAgentFormat(output.Provider)
		if formatErr != nil {
			return formatErr
		}
		err = compilation.RenderAgentFiles(sourceSet.Agents, dest, format)
	default:
		return fmt.Errorf("unknown output kind %q for %s", output.Kind, output.Provider)
	}
	if err != nil {
		slog.Error("corpus: rendering output", "provider", output.Provider, "kind", string(output.Kind), "dest", dest, "err", err)
		return fmt.Errorf("rendering %s %s into %s: %w", output.Provider, output.Kind, dest, err)
	}
	return nil
}

func resolveAgentFormat(provider ProviderName) (compilation.AgentTargetFormat, error) {
	switch provider {
	case providerClaude:
		return compilation.AgentTargetClaude, nil
	case providerCursor:
		return compilation.AgentTargetCursor, nil
	case providerGemini:
		return compilation.AgentTargetGemini, nil
	case providerCodex:
		return compilation.AgentTargetCodex, nil
	case providerCopilot:
		return compilation.AgentTargetCopilot, nil
	case providerAgents:
		return "", fmt.Errorf("unsupported agent provider %q", provider)
	default:
		return "", fmt.Errorf("unsupported agent provider %q", provider)
	}
}

func resolveRuleRenderStyle(output Output, home string, dest string) (compilation.RuleRenderStyle, error) {
	emptyStyle := compilation.RuleRenderStyle{SkillsRelDir: ""}
	if output.SkillDest == "" {
		return emptyStyle, nil
	}
	var linkBase string
	switch output.Kind {
	case KindSkills, KindAgents:
		return emptyStyle, nil
	case KindRuleFiles, KindPerFileInstructions:
		linkBase = dest
	case KindInstructionDoc:
		linkBase = filepath.Dir(dest)
	default:
		return emptyStyle, nil
	}
	skillRoot := filepath.Join(home, output.SkillDest)
	skillsRelDir, err := filepath.Rel(linkBase, skillRoot)
	if err != nil {
		slog.Error("corpus: computing skill link path", "provider", output.Provider, "err", err)
		return emptyStyle, fmt.Errorf("computing skill link path for %s: %w", output.Provider, err)
	}
	return compilation.RuleRenderStyle{SkillsRelDir: filepath.ToSlash(skillsRelDir)}, nil
}

func resolveRefStyle(name RefStyleName) (compilation.SkillRefStyle, error) {
	switch name {
	case RefMDC:
		return compilation.SkillRefMDC, nil
	case RefMD:
		return compilation.SkillRefMD, nil
	default:
		return compilation.SkillRefStyle{}, fmt.Errorf("unknown ref_style %q", name)
	}
}
