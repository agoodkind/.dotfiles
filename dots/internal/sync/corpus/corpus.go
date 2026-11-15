// Package corpus renders agent corpus outputs from a TOML manifest.
//
// Each manifest output records a kind, a destination, and the parameters that
// kind needs. The sync engine dispatches every output to the matching
// compilation primitive.
package corpus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/sync/compilation"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// ManifestName is the manifest file expected at the corpus root.
const ManifestName = "targets.toml"

var (
	errInstructionDocRequiresTitle = errors.New("instruction-doc output requires title")
	errUnknownOutputKind           = errors.New("unknown output kind")
	errRuleFilesRequireExt         = errors.New("rule-files output requires rule_ext")
	errOutputDestEscapesHome       = errors.New("output destination escapes home directory")
	errOutputDestMustBeBelowHome   = errors.New("output destination must be a path below home")
	errUnsupportedAgentProvider    = errors.New("unsupported agent provider")
	errUnknownRefStyle             = errors.New("unknown ref_style")
)

var errOutputDestResolvesOutside = errors.New("output destination resolves outside home directory")

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

// RefStyleName is the skill rule-reference style.
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
//
// Sync and LoadSourceSet call LoadManifest so every render starts from the same
// parsed targets.toml.
//
// Example:
//
//	manifest, err := LoadManifest(filepath.Join(corpusRoot, ManifestName))
//	if err != nil {
//		return err
//	}
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

// LoadSourceSet loads local and imported corpus artifacts into one set.
//
// Sync and tests call LoadSourceSet so every render starts from the same
// normalized rules, skills, and agents.
//
// Example:
//
//	sourceSet, err := LoadSourceSet(dotfiles)
//	if err != nil {
//		return err
//	}
func LoadSourceSet(dotfiles string) (compilation.CorpusSourceSet, error) {
	corpusPaths := compilation.ResolveCorpusSource(dotfiles)
	manifest, err := LoadManifest(filepath.Join(corpusPaths.Root, ManifestName))
	if err != nil {
		return compilation.CorpusSourceSet{}, err
	}

	return loadSourceSet(dotfiles, manifest)
}

// Sync renders every manifest output that applies to the running OS.
//
// The sync pipeline calls Sync after loading the corpus so each provider
// destination matches the current normalized source set.
//
// Example:
//
//	if err := Sync(ctx, dotfiles, logger); err != nil {
//		return err
//	}
func Sync(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
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

	var report compilation.ChangeReport

	for _, output := range manifest.Outputs {
		if output.OS != "" && output.OS != runtime.GOOS {
			continue
		}

		dest, err := ResolveOutputDest(home, output.Dest)
		if err != nil {
			return err
		}

		outputReport, err := renderOutput(sourceSet, output, home, dest)
		if err != nil {
			return err
		}

		report.Merge(outputReport)
	}

	logCorpusChangeReport(ctx, logger, home, report)

	return nil
}

func logCorpusChangeReport(
	ctx context.Context,
	logger *telemetry.Logger,
	home string,
	report compilation.ChangeReport,
) {
	for _, path := range report.Created {
		common.InfoContext(ctx, logger, "  created "+displayHomePath(home, path))
	}

	for _, path := range report.Updated {
		common.InfoContext(ctx, logger, "  updated "+displayHomePath(home, path))
	}

	for _, path := range report.Removed {
		common.InfoContext(ctx, logger, "  removed "+displayHomePath(home, path))
	}

	for _, path := range report.SkippedUnmanaged {
		common.WarnContext(ctx, logger, "  skipped unmanaged "+displayHomePath(home, path))
	}

	common.InfoContextf(
		ctx,
		logger,
		"  Corpus: written: %s unchanged: %s skipped unmanaged: %s removed: %s",
		strconv.Itoa(report.WrittenCount()),
		strconv.Itoa(len(report.Unchanged)),
		strconv.Itoa(len(report.SkippedUnmanaged)),
		strconv.Itoa(len(report.Removed)),
	)
}

func displayHomePath(home string, path string) string {
	cleanHome := filepath.Clean(home)
	cleanPath := filepath.Clean(path)
	if cleanPath == cleanHome {
		return "~"
	}

	prefix := cleanHome + string(os.PathSeparator)
	if rest, ok := strings.CutPrefix(cleanPath, prefix); ok {
		return "~/" + filepath.ToSlash(rest)
	}

	return filepath.ToSlash(cleanPath)
}

func preflightOutputPlans(
	sourceSet compilation.CorpusSourceSet,
	outputs []Output,
	home string,
) error {
	for _, output := range outputs {
		if output.OS != "" && output.OS != runtime.GOOS {
			continue
		}

		dest, err := ResolveOutputDest(home, output.Dest)
		if err != nil {
			return err
		}

		if err := validateOutput(sourceSet, output, home, dest); err != nil {
			slog.Warn(
				"corpus: preflighting output",
				"provider",
				output.Provider,
				"kind",
				output.Kind,
				"err",
				err,
			)

			return fmt.Errorf(
				"preflighting %s output for %s: %w",
				output.Kind,
				output.Provider,
				err,
			)
		}
	}

	return nil
}

func validateOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	home string,
	dest string,
) error {
	switch output.Kind {
	case KindSkills:
		return validateSkillsOutput(sourceSet, output, dest)
	case KindRuleFiles:
		return validateRuleFileOutput(sourceSet, output, home, dest)
	case KindInstructionDoc:
		return validateInstructionDocOutput(sourceSet, output, home, dest)
	case KindPerFileInstructions:
		return validatePerFileInstructionsOutput(sourceSet, output, home, dest)
	case KindAgents:
		return validateAgentsOutput(sourceSet, output, dest)
	default:
		slog.Warn("corpus: unknown output kind", "provider", output.Provider, "kind", output.Kind)

		return fmt.Errorf("%w %q for %s", errUnknownOutputKind, output.Kind, output.Provider)
	}
}

func validateSkillsOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	dest string,
) error {
	style, err := resolveRefStyle(output.RefStyle)
	if err != nil {
		return err
	}

	style.Harness = string(output.Provider)
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
}

func validateInstructionDocOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	home string,
	dest string,
) error {
	if output.Title == "" {
		return fmt.Errorf("%w for %s", errInstructionDocRequiresTitle, output.Provider)
	}

	ruleStyle, err := resolveRuleRenderStyle(output, home, dest)
	if err != nil {
		return err
	}

	if err := compilation.ValidateRulesAsInstructionDocFromSourceSet(
		sourceSet,
		dest,
		ruleStyle,
	); err != nil {
		slog.Warn(
			"corpus: validating instruction document",
			"provider",
			output.Provider,
			"err",
			err,
		)

		return fmt.Errorf("validating instruction document: %w", err)
	}

	return nil
}

func validatePerFileInstructionsOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	home string,
	dest string,
) error {
	ruleStyle, err := resolveRuleRenderStyle(output, home, dest)
	if err != nil {
		return err
	}

	if err := compilation.ValidateCopilotInstructionFilesFromSourceSet(
		sourceSet,
		dest,
		ruleStyle,
	); err != nil {
		slog.Warn(
			"corpus: validating Copilot instructions",
			"provider",
			output.Provider,
			"err",
			err,
		)

		return fmt.Errorf("validating Copilot instructions: %w", err)
	}

	return nil
}

func validateAgentsOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	dest string,
) error {
	format, err := resolveAgentFormat(output.Provider)
	if err != nil {
		return err
	}

	if err := compilation.ValidateAgentFiles(sourceSet.Agents, dest, format); err != nil {
		slog.Warn("corpus: validating agent output", "provider", output.Provider, "err", err)

		return fmt.Errorf("validating agent output: %w", err)
	}

	return nil
}

func validateRuleFileOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	home string,
	dest string,
) error {
	if output.RuleExt == "" {
		return fmt.Errorf("%w for %s", errRuleFilesRequireExt, output.Provider)
	}

	ruleStyle, err := resolveRuleRenderStyle(output, home, dest)
	if err != nil {
		return err
	}

	format, err := compilation.RuleTargetFormatFromExt(output.RuleExt)
	if err != nil {
		slog.Warn("corpus: resolving rule format", "provider", output.Provider, "err", err)

		return fmt.Errorf(
			"unsupported rule_ext %q for %s: %w",
			output.RuleExt,
			output.Provider,
			err,
		)
	}

	if err := compilation.ValidateRuleFilesFromSourceSet(
		sourceSet,
		dest,
		output.RuleExt,
		format,
		ruleStyle,
	); err != nil {
		slog.Warn("corpus: validating rule output", "provider", output.Provider, "err", err)

		return fmt.Errorf("validating rule output: %w", err)
	}

	return nil
}

// ResolveOutputDest rejects destinations outside the resolved home directory.
//
// Sync and preflight call ResolveOutputDest so every corpus output stays under
// the caller home directory before any files are written.
//
// Example:
//
//	dest, err := ResolveOutputDest(home, output.Dest)
//	if err != nil {
//		return err
//	}
func ResolveOutputDest(home string, relativeDest string) (string, error) {
	cleanHome := filepath.Clean(home)
	cleanDest := filepath.Clean(relativeDest)

	if filepath.IsAbs(cleanDest) {
		return "", fmt.Errorf("%w: %q (home %s)", errOutputDestEscapesHome, relativeDest, cleanHome)
	}

	if cleanDest == "." {
		return "", fmt.Errorf(
			"%w: %q (home %s)",
			errOutputDestMustBeBelowHome,
			relativeDest,
			cleanHome,
		)
	}

	joined := filepath.Clean(filepath.Join(cleanHome, cleanDest))
	if !strings.HasPrefix(joined, cleanHome+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: %q (home %s)", errOutputDestEscapesHome, relativeDest, cleanHome)
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
			if resolvedParent != resolvedHome &&
				!strings.HasPrefix(resolvedParent, resolvedHome+string(os.PathSeparator)) {
				return "", fmt.Errorf(
					"%w: %q (home %s)",
					errOutputDestResolvesOutside,
					relativeDest,
					cleanHome,
				)
			}

			break
		}

		if !os.IsNotExist(resolveErr) {
			return "", fmt.Errorf(
				"resolving output destination path %s: %w",
				existingPath,
				resolveErr,
			)
		}

		nextPath := filepath.Dir(existingPath)
		if nextPath == existingPath {
			return "", fmt.Errorf(
				"resolving output destination path %s: %w",
				existingPath,
				resolveErr,
			)
		}

		existingPath = nextPath
	}

	return joined, nil
}

func renderOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	home string,
	dest string,
) (compilation.ChangeReport, error) {
	report, err := renderOutputByKind(sourceSet, output, home, dest)
	if err != nil {
		slog.Error(
			"corpus: rendering output",
			"provider",
			output.Provider,
			"kind",
			string(output.Kind),
			"dest",
			dest,
			"err",
			err,
		)

		return report, fmt.Errorf(
			"rendering %s %s into %s: %w",
			output.Provider,
			output.Kind,
			dest,
			err,
		)
	}

	return report, nil
}

func renderOutputByKind(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	home string,
	dest string,
) (compilation.ChangeReport, error) {
	switch output.Kind {
	case KindSkills:
		return renderSkillsOutput(sourceSet, output, dest)
	case KindRuleFiles:
		return renderRuleFilesOutput(sourceSet, output, home, dest)
	case KindInstructionDoc:
		return renderInstructionDocOutput(sourceSet, output, home, dest)
	case KindPerFileInstructions:
		return renderPerFileInstructionsOutput(sourceSet, output, home, dest)
	case KindAgents:
		return renderAgentsOutput(sourceSet, output, dest)
	default:
		slog.Warn("corpus: unknown output kind", "provider", output.Provider, "kind", output.Kind)

		return compilation.ChangeReport{}, fmt.Errorf(
			"%w %q for %s",
			errUnknownOutputKind,
			output.Kind,
			output.Provider,
		)
	}
}

func renderSkillsOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	dest string,
) (compilation.ChangeReport, error) {
	style, err := resolveRefStyle(output.RefStyle)
	if err != nil {
		return compilation.ChangeReport{}, err
	}

	style.Harness = string(output.Provider)
	renderSourceSet := sourceSet
	if output.Provider == providerAgents {
		renderSourceSet, err = withAgentFallbackSkills(sourceSet)
		if err != nil {
			return compilation.ChangeReport{}, err
		}
	}

	report, err := compilation.RenderSkillDirsFromSourceSet(renderSourceSet, dest, style)
	if err != nil {
		slog.Error("corpus: rendering skills", "provider", output.Provider, "err", err)

		return report, fmt.Errorf("rendering skills: %w", err)
	}

	return report, nil
}

func renderRuleFilesOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	home string,
	dest string,
) (compilation.ChangeReport, error) {
	if output.RuleExt == "" {
		return compilation.ChangeReport{}, fmt.Errorf(
			"%w for %s",
			errRuleFilesRequireExt,
			output.Provider,
		)
	}

	ruleStyle, err := resolveRuleRenderStyle(output, home, dest)
	if err != nil {
		return compilation.ChangeReport{}, err
	}

	format, err := compilation.RuleTargetFormatFromExt(output.RuleExt)
	if err != nil {
		slog.Error(
			"corpus: unsupported rule_ext",
			"provider",
			output.Provider,
			"rule_ext",
			output.RuleExt,
			"err",
			err,
		)

		return compilation.ChangeReport{}, fmt.Errorf(
			"unsupported rule_ext %q for %s: %w",
			output.RuleExt,
			output.Provider,
			err,
		)
	}

	report, err := compilation.RenderRuleFilesFromSourceSet(
		sourceSet,
		dest,
		output.RuleExt,
		format,
		ruleStyle,
	)
	if err != nil {
		return report, fmt.Errorf("rendering rule files: %w", err)
	}

	return report, nil
}

func renderInstructionDocOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	home string,
	dest string,
) (compilation.ChangeReport, error) {
	if output.Title == "" {
		slog.Warn("corpus: instruction-doc missing title", "provider", output.Provider)

		return compilation.ChangeReport{}, fmt.Errorf(
			"%w for %s",
			errInstructionDocRequiresTitle,
			output.Provider,
		)
	}

	ruleStyle, err := resolveRuleRenderStyle(output, home, dest)
	if err != nil {
		return compilation.ChangeReport{}, err
	}

	report, err := compilation.RenderRulesAsInstructionDocFromSourceSet(
		sourceSet,
		dest,
		output.Title,
		ruleStyle,
	)
	if err != nil {
		slog.Error("corpus: rendering instruction doc", "provider", output.Provider, "err", err)

		return report, fmt.Errorf("rendering instruction doc: %w", err)
	}

	return report, nil
}

func renderPerFileInstructionsOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	home string,
	dest string,
) (compilation.ChangeReport, error) {
	ruleStyle, err := resolveRuleRenderStyle(output, home, dest)
	if err != nil {
		return compilation.ChangeReport{}, err
	}

	report, err := compilation.RenderCopilotInstructionFilesFromSourceSet(
		sourceSet,
		dest,
		ruleStyle,
	)
	if err != nil {
		slog.Error(
			"corpus: rendering Copilot instructions",
			"provider",
			output.Provider,
			"err",
			err,
		)

		return report, fmt.Errorf("rendering Copilot instructions: %w", err)
	}

	return report, nil
}

func renderAgentsOutput(
	sourceSet compilation.CorpusSourceSet,
	output Output,
	dest string,
) (compilation.ChangeReport, error) {
	format, err := resolveAgentFormat(output.Provider)
	if err != nil {
		return compilation.ChangeReport{}, err
	}

	report, err := compilation.RenderAgentFiles(sourceSet.Agents, dest, format)
	if err != nil {
		slog.Error("corpus: rendering agents", "provider", output.Provider, "err", err)

		return report, fmt.Errorf("rendering agents: %w", err)
	}

	return report, nil
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
		slog.Warn("corpus: unsupported agent provider", "provider", provider)

		return "", fmt.Errorf("%w %q", errUnsupportedAgentProvider, provider)
	default:
		slog.Warn("corpus: unsupported agent provider", "provider", provider)

		return "", fmt.Errorf("%w %q", errUnsupportedAgentProvider, provider)
	}
}

func resolveRuleRenderStyle(
	output Output,
	home string,
	dest string,
) (compilation.RuleRenderStyle, error) {
	emptyStyle := compilation.EmptyRuleRenderStyle(string(output.Provider))
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

	style := compilation.EmptyRuleRenderStyle(string(output.Provider))
	style.SkillsRelDir = filepath.ToSlash(skillsRelDir)

	return style, nil
}

func resolveRefStyle(name RefStyleName) (compilation.SkillRefStyle, error) {
	switch name {
	case RefMDC:
		return compilation.SkillRefMDC, nil
	case RefMD:
		return compilation.SkillRefMD, nil
	default:
		slog.Warn("corpus: unknown ref_style", "ref_style", name)

		return compilation.SkillRefStyle{}, fmt.Errorf("%w %q", errUnknownRefStyle, name)
	}
}
