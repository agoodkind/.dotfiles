package compilation

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

type renderedSkillPlan struct {
	name      string
	files     map[string]string
	conflicts []string
}

// RenderSkillDirsFromSourceSet renders normalized skills into managed provider directories.
func RenderSkillDirsFromSourceSet(sourceSet CorpusSourceSet, dst string, refStyle SkillRefStyle) error {
	slog.Info("compilation: rendering skill directories", "dest", dst)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		slog.Warn("compilation: creating skill directory", "dest", dst, "err", err)
		return fmt.Errorf("creating directory %s: %w", dst, err)
	}
	skillNames := sortedSkillNames(sourceSet.Skills)
	ruleNames := sortedRuleNames(sourceSet.Rules)
	activeSkills := make(map[string]struct{}, len(skillNames))
	plans := make([]renderedSkillPlan, 0, len(skillNames))
	conflicts := make([]string, 0)
	for _, skillName := range skillNames {
		activeSkills[skillName] = struct{}{}
		plan, err := renderSourceSkillPlan(sourceSet.Skills[skillName], filepath.Join(dst, skillName), refStyle, sourceSet, ruleNames)
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		conflicts = append(conflicts, plan.conflicts...)
		if err := validateSkillPlanPaths(filepath.Join(dst, skillName), plan); err != nil {
			return err
		}
	}
	if err := skillRenderConflictsError(conflicts); err != nil {
		return err
	}
	if err := applyRenderedSkillPlans(plans, dst); err != nil {
		return err
	}
	if err := removeMissingManagedSkillDirs(dst, activeSkills); err != nil {
		return err
	}
	return nil
}

func applyRenderedSkillPlans(plans []renderedSkillPlan, dst string) error {
	slog.Info("compilation: applying rendered skill plans", "dest", dst)
	for _, plan := range plans {
		targetDir := filepath.Join(dst, plan.name)
		if isSymlink(targetDir) {
			if err := os.Remove(targetDir); err != nil {
				slog.Warn("compilation: removing skill directory symlink", "path", targetDir, "err", err)
				return fmt.Errorf("removing symlink %s: %w", targetDir, err)
			}
		}
		fileNames := make([]string, 0, len(plan.files))
		for fileName := range plan.files {
			fileNames = append(fileNames, fileName)
		}
		sort.Strings(fileNames)
		for _, fileName := range fileNames {
			target := filepath.Join(targetDir, fileName)
			if err := validateNoNestedSymlink(targetDir, target); err != nil {
				return err
			}
			if isSymlink(target) {
				if err := os.Remove(target); err != nil {
					return fmt.Errorf("removing symlink %s: %w", target, err)
				}
			}
			if err := writeFileIfChanged(target, []byte(plan.files[fileName])); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateSkillDirsFromSourceSet validates skill rendering against the real destination without writing files.
func ValidateSkillDirsFromSourceSet(sourceSet CorpusSourceSet, dst string, refStyle SkillRefStyle) error {
	skillNames := sortedSkillNames(sourceSet.Skills)
	ruleNames := sortedRuleNames(sourceSet.Rules)
	conflicts := make([]string, 0)
	for _, skillName := range skillNames {
		targetDir := filepath.Join(dst, skillName)
		plan, err := renderSourceSkillPlan(sourceSet.Skills[skillName], targetDir, refStyle, sourceSet, ruleNames)
		if err != nil {
			return err
		}
		conflicts = append(conflicts, plan.conflicts...)
		if err := validateSkillPlanPaths(targetDir, plan); err != nil {
			return err
		}
	}
	return skillRenderConflictsError(conflicts)
}

func validateSkillPlanPaths(targetDir string, plan renderedSkillPlan) error {
	for fileName := range plan.files {
		if err := validateNoNestedSymlink(targetDir, filepath.Join(targetDir, fileName)); err != nil {
			return err
		}
	}
	return nil
}

func renderSourceSkillPlan(skill SkillSource, dst string, style SkillRefStyle, sourceSet CorpusSourceSet, ruleNames []string) (renderedSkillPlan, error) {
	plan := renderedSkillPlan{
		name:      skill.Name,
		files:     make(map[string]string),
		conflicts: make([]string, 0),
	}
	fileNames := make([]string, 0, len(skill.Files))
	for fileName := range skill.Files {
		fileNames = append(fileNames, fileName)
	}
	sort.Strings(fileNames)
	for _, fileName := range fileNames {
		cleanName := filepath.Clean(filepath.FromSlash(fileName))
		if cleanName == "." || cleanName == ".." || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) {
			return renderedSkillPlan{}, fmt.Errorf("invalid skill file path %q for %s", fileName, skill.Name)
		}
		outputName := cleanName
		output := skill.Files[fileName]
		if base, ok := strings.CutSuffix(cleanName, ".tmpl"); ok {
			outputName = base
			rootSkillName := ""
			if filepath.ToSlash(cleanName) == "SKILL.md.tmpl" {
				rootSkillName = skill.Name
			}
			rendered, err := renderSourceSkillTemplate(output, style, sourceSet, ruleNames, fileName, rootSkillName)
			if err != nil {
				return renderedSkillPlan{}, err
			}
			output = rendered
		}
		if filepath.ToSlash(outputName) == "SKILL.md" {
			output = injectSkillMarker(output)
			target := filepath.Join(dst, outputName)
			if conflict := skillRenderConflict(target); conflict != "" {
				plan.conflicts = append(plan.conflicts, conflict)
				continue
			}
		}
		plan.files[outputName] = output
	}
	return plan, nil
}

type sourceSkillTemplateData struct {
	style     SkillRefStyle
	sourceSet CorpusSourceSet
	ruleNames []string
	execErr   *error
	stack     []sourceBodyRef
	active    map[sourceBodyRef]int
}

type sourceBodyKind string

const (
	sourceBodyRule  sourceBodyKind = "rule"
	sourceBodySkill sourceBodyKind = "skill"
)

type sourceBodyRef struct {
	kind sourceBodyKind
	name string
}

type sourceBody struct {
	content    string
	isTemplate bool
}

func (r sourceBodyRef) String() string {
	return string(r.kind) + ":" + r.name
}

func (d *sourceSkillTemplateData) Rules() string            { return renderRulesList(d.style, d.ruleNames) }
func (d *sourceSkillTemplateData) Rule(name string) string  { return renderRuleLink(d.style, name) }
func (d *sourceSkillTemplateData) Skill(name string) string { return renderSkillSiblingLink(name) }

func (d *sourceSkillTemplateData) RuleBody(name string) string {
	return d.renderBody(sourceBodyRef{kind: sourceBodyRule, name: name})
}

func (d *sourceSkillTemplateData) SkillBody(name string) string {
	return d.renderBody(sourceBodyRef{kind: sourceBodySkill, name: name})
}

func (d *sourceSkillTemplateData) renderBody(ref sourceBodyRef) string {
	if err := validateSourceBodyRef(ref); err != nil {
		d.setError(err)
		return ""
	}
	if cycleStart, exists := d.active[ref]; exists {
		cycle := append(append([]sourceBodyRef(nil), d.stack[cycleStart:]...), ref)
		cycleNames := make([]string, 0, len(cycle))
		for _, cycleRef := range cycle {
			cycleNames = append(cycleNames, cycleRef.String())
		}
		d.setError(fmt.Errorf("body embedding cycle: %s", strings.Join(cycleNames, " -> ")))
		return ""
	}
	body, err := d.readBody(ref)
	if err != nil {
		d.setError(err)
		return ""
	}
	d.push(ref)
	defer d.pop(ref)
	if !body.isTemplate || !strings.Contains(body.content, "{{") {
		return body.content
	}
	parsed, err := template.New(ref.String()).Parse(body.content)
	if err != nil {
		d.setError(fmt.Errorf("parsing %s body template: %w", ref, err))
		return ""
	}
	var buffer bytes.Buffer
	if err := parsed.Execute(&buffer, d); err != nil {
		d.setError(fmt.Errorf("executing %s body template: %w", ref, err))
		return ""
	}
	return buffer.String()
}

func validateSourceBodyRef(ref sourceBodyRef) error {
	name := ref.name
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") || strings.Contains(name, string(filepath.Separator)) {
		return fmt.Errorf("invalid %s body name %q", ref.kind, name)
	}
	return nil
}

func (d *sourceSkillTemplateData) readBody(ref sourceBodyRef) (sourceBody, error) {
	switch ref.kind {
	case sourceBodyRule:
		rule, ok := d.sourceSet.Rules[ref.name]
		if !ok {
			return sourceBody{}, fmt.Errorf("unknown rule body %q", ref.name)
		}
		return sourceBody{content: rule.Body, isTemplate: true}, nil
	case sourceBodySkill:
		skill, ok := d.sourceSet.Skills[ref.name]
		if !ok {
			return sourceBody{}, fmt.Errorf("unknown skill body %q", ref.name)
		}
		content, ok := skill.Files["SKILL.md.tmpl"]
		isTemplate := ok
		if !ok {
			content, ok = skill.Files["SKILL.md"]
		}
		if !ok {
			return sourceBody{}, fmt.Errorf("skill body %q has no SKILL.md source", ref.name)
		}
		_, body, err := parseSkillDocument(content)
		if err != nil {
			slog.Warn("compilation: parsing embedded skill body", "skill", ref.name, "err", err)
			return sourceBody{}, fmt.Errorf("parsing skill body %q: %w", ref.name, err)
		}
		return sourceBody{content: body, isTemplate: isTemplate}, nil
	default:
		return sourceBody{}, fmt.Errorf("unknown body kind %q", ref.kind)
	}
}

func (d *sourceSkillTemplateData) push(ref sourceBodyRef) {
	d.active[ref] = len(d.stack)
	d.stack = append(d.stack, ref)
}

func (d *sourceSkillTemplateData) pop(ref sourceBodyRef) {
	delete(d.active, ref)
	d.stack = d.stack[:len(d.stack)-1]
}

func (d *sourceSkillTemplateData) setError(err error) {
	if d.execErr != nil && *d.execErr == nil {
		*d.execErr = err
	}
}

func renderSourceSkillTemplate(content string, style SkillRefStyle, sourceSet CorpusSourceSet, ruleNames []string, name string, rootSkillName string) (string, error) {
	parsed, err := template.New(name).Parse(content)
	if err != nil {
		slog.Warn("compilation: parsing skill template", "name", name, "err", err)
		return "", fmt.Errorf("parsing skill template %s: %w", name, err)
	}
	var execErr error
	data := &sourceSkillTemplateData{
		style:     style,
		sourceSet: sourceSet,
		ruleNames: ruleNames,
		execErr:   &execErr,
		stack:     make([]sourceBodyRef, 0),
		active:    make(map[sourceBodyRef]int),
	}
	if rootSkillName != "" {
		data.push(sourceBodyRef{kind: sourceBodySkill, name: rootSkillName})
	}
	var buffer bytes.Buffer
	if err := parsed.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("executing skill template %s: %w", name, err)
	}
	if execErr != nil {
		return "", fmt.Errorf("executing skill template %s: %w", name, execErr)
	}
	return buffer.String(), nil
}

// ValidateRuleFilesFromSourceSet validates rule-file rendering without writing files.
func ValidateRuleFilesFromSourceSet(sourceSet CorpusSourceSet, dst string, ext string, format RuleTargetFormat, style RuleRenderStyle) error {
	for _, name := range sortedRuleNames(sourceSet.Rules) {
		rule := sourceSet.Rules[name]
		renderedBody, err := renderRuleTemplate(strings.TrimSpace(rule.Body), style, name)
		if err != nil {
			slog.Warn("compilation: validating rule template", "name", name, "err", err)
			return fmt.Errorf("rendering rule template %s: %w", name, err)
		}
		rule.Body = renderedBody
		if _, err := rule.RenderForTarget(format); err != nil {
			slog.Warn("compilation: validating rendered rule", "name", name, "err", err)
			return fmt.Errorf("rendering rule %s for format %q: %w", name, format, err)
		}
		if err := validateNoNestedSymlink(dst, filepath.Join(dst, name+ext)); err != nil {
			return err
		}
	}
	return nil
}

// RenderRuleFilesFromSourceSet renders normalized rules as managed files for one provider format.
func RenderRuleFilesFromSourceSet(sourceSet CorpusSourceSet, dst string, ext string, format RuleTargetFormat, style RuleRenderStyle) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		slog.Warn("compilation: creating rule directory", "dest", dst, "err", err)
		return fmt.Errorf("creating directory %s: %w", dst, err)
	}
	names := sortedRuleNames(sourceSet.Rules)
	activeFiles := make(map[string]struct{}, len(names))
	for _, name := range names {
		targetName := name + ext
		activeFiles[targetName] = struct{}{}
		rule := sourceSet.Rules[name]
		renderedBody, err := renderRuleTemplate(strings.TrimSpace(rule.Body), style, name)
		if err != nil {
			return fmt.Errorf("rendering rule template %s: %w", name, err)
		}
		rule.Body = renderedBody
		rendered, err := rule.RenderForTarget(format)
		if err != nil {
			return fmt.Errorf("rendering rule %s for format %q: %w", name, format, err)
		}
		target := filepath.Join(dst, targetName)
		if err := removeRenderTargetSymlink(target); err != nil {
			return err
		}
		if err := writeFileIfChanged(target, []byte(injectSkillMarker(rendered))); err != nil {
			return err
		}
	}
	return removeMissingManagedFiles(dst, ext, activeFiles)
}

// ValidateRulesAsInstructionDocFromSourceSet validates instruction-document rendering without writing files.
func ValidateRulesAsInstructionDocFromSourceSet(sourceSet CorpusSourceSet, dst string, style RuleRenderStyle) error {
	for _, name := range sortedRuleNames(sourceSet.Rules) {
		if _, err := renderRuleTemplate(strings.TrimSpace(sourceSet.Rules[name].Body), style, name); err != nil {
			slog.Warn("compilation: validating instruction rule", "name", name, "err", err)
			return fmt.Errorf("rendering rule template %s: %w", name, err)
		}
	}
	return validateNoNestedSymlink(filepath.Dir(dst), dst)
}

// RenderRulesAsInstructionDocFromSourceSet renders normalized rules into one managed instruction document.
func RenderRulesAsInstructionDocFromSourceSet(sourceSet CorpusSourceSet, dst string, title string, style RuleRenderStyle) error {
	names := sortedRuleNames(sourceSet.Rules)
	if len(names) == 0 {
		return removeManagedRenderTarget(dst)
	}
	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(title)
	builder.WriteString("\n\n")
	builder.WriteString(GeneratedAgentHTMLMarker)
	builder.WriteString("\n")
	for _, name := range names {
		builder.WriteString("\n## ")
		builder.WriteString(name)
		builder.WriteString("\n\n")
		rendered, err := renderRuleTemplate(strings.TrimSpace(sourceSet.Rules[name].Body), style, name)
		if err != nil {
			slog.Warn("compilation: rendering instruction rule", "name", name, "err", err)
			return fmt.Errorf("rendering rule template %s: %w", name, err)
		}
		builder.WriteString(rendered)
		builder.WriteString("\n")
	}
	if err := removeRenderTargetSymlink(dst); err != nil {
		return err
	}
	return writeFileIfChanged(dst, []byte(builder.String()))
}

// ValidateCopilotInstructionFilesFromSourceSet validates Copilot instruction rendering without writing files.
func ValidateCopilotInstructionFilesFromSourceSet(sourceSet CorpusSourceSet, dst string, style RuleRenderStyle) error {
	for _, name := range sortedRuleNames(sourceSet.Rules) {
		rule := sourceSet.Rules[name]
		renderedBody, err := renderRuleTemplate(strings.TrimSpace(rule.Body), style, name)
		if err != nil {
			slog.Warn("compilation: validating Copilot rule template", "name", name, "err", err)
			return fmt.Errorf("rendering rule template %s: %w", name, err)
		}
		rule.Body = renderedBody
		if _, err := rule.RenderCopilot(name); err != nil {
			slog.Warn("compilation: validating Copilot instruction", "name", name, "err", err)
			return fmt.Errorf("rendering copilot instruction %s: %w", name, err)
		}
		if err := validateNoNestedSymlink(dst, filepath.Join(dst, name+".instructions.md")); err != nil {
			return err
		}
	}
	return nil
}

// RenderCopilotInstructionFilesFromSourceSet renders normalized rules as managed Copilot instruction files.
func RenderCopilotInstructionFilesFromSourceSet(sourceSet CorpusSourceSet, dst string, style RuleRenderStyle) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		slog.Warn("compilation: creating Copilot instruction directory", "dest", dst, "err", err)
		return fmt.Errorf("creating directory %s: %w", dst, err)
	}
	names := sortedRuleNames(sourceSet.Rules)
	activeFiles := make(map[string]struct{}, len(names))
	for _, name := range names {
		targetName := name + ".instructions.md"
		activeFiles[targetName] = struct{}{}
		rule := sourceSet.Rules[name]
		renderedBody, err := renderRuleTemplate(strings.TrimSpace(rule.Body), style, name)
		if err != nil {
			return fmt.Errorf("rendering rule template %s: %w", name, err)
		}
		rule.Body = renderedBody
		rendered, err := rule.RenderCopilot(name)
		if err != nil {
			return fmt.Errorf("rendering copilot instruction %s: %w", name, err)
		}
		target := filepath.Join(dst, targetName)
		if err := removeRenderTargetSymlink(target); err != nil {
			return err
		}
		if err := writeFileIfChanged(target, []byte(rendered)); err != nil {
			return err
		}
	}
	return removeMissingManagedFiles(dst, ".instructions.md", activeFiles)
}

// RenderRulesForUploadFromSourceSet renders normalized rules for direct cloud upload.
func RenderRulesForUploadFromSourceSet(sourceSet CorpusSourceSet, style RuleRenderStyle) ([]RenderedRule, error) {
	names := sortedRuleNames(sourceSet.Rules)
	renderedRules := make([]RenderedRule, 0, len(names))
	for _, name := range names {
		rendered, err := renderRuleTemplate(strings.TrimSpace(sourceSet.Rules[name].Body), style, name)
		if err != nil {
			slog.Warn("compilation: rendering rule for upload", "name", name, "err", err)
			return nil, fmt.Errorf("rendering rule template %s: %w", name, err)
		}
		renderedRules = append(renderedRules, RenderedRule{Title: name, Body: strings.TrimSpace(rendered)})
	}
	return renderedRules, nil
}

func removeRenderTargetSymlink(path string) error {
	if !isSymlink(path) {
		return nil
	}
	if err := os.Remove(path); err != nil {
		slog.Warn("compilation: removing render target symlink", "path", path, "err", err)
		return fmt.Errorf("removing symlink %s: %w", path, err)
	}
	return nil
}

func removeManagedRenderTarget(path string) error {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.Warn("compilation: reading managed render target", "path", path, "err", err)
		return fmt.Errorf("reading managed render target %s: %w", path, err)
	}
	if !HasGeneratedMarker(string(content)) {
		return nil
	}
	if err := os.Remove(filepath.Clean(path)); err != nil {
		slog.Warn("compilation: removing managed render target", "path", path, "err", err)
		return fmt.Errorf("removing managed render target %s: %w", path, err)
	}
	return nil
}

func validateNoNestedSymlink(root string, target string) error {
	relativePath, err := filepath.Rel(root, target)
	if err != nil {
		slog.Warn("compilation: resolving nested render target", "root", root, "target", target, "err", err)
		return fmt.Errorf("resolving render target %s below %s: %w", target, root, err)
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("render target %s escapes destination %s", target, root)
	}
	components := strings.Split(relativePath, string(os.PathSeparator))
	currentPath := root
	for _, component := range components[:len(components)-1] {
		currentPath = filepath.Join(currentPath, component)
		info, statErr := os.Lstat(currentPath)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			slog.Warn("compilation: checking nested render path", "path", currentPath, "err", statErr)
			return fmt.Errorf("checking render path %s: %w", currentPath, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("render path component %s is a symlink", currentPath)
		}
	}
	return nil
}

func sortedRuleNames(rules map[string]RuleSource) []string {
	names := make([]string, 0, len(rules))
	for name := range rules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedSkillNames(skills map[string]SkillSource) []string {
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
