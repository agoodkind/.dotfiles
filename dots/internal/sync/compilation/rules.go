package compilation

import (
	"fmt"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"
)

// RuleTargetFormat selects provider-specific front matter for rendered rule files.
type RuleTargetFormat string

const (
	// RuleTargetCursor renders Cursor .mdc front matter with description, globs, and alwaysApply.
	RuleTargetCursor RuleTargetFormat = "cursor"
	// RuleTargetClaude renders Claude .md front matter with paths for scoped rules only.
	RuleTargetClaude RuleTargetFormat = "claude"
)

type ruleFileExt string

const (
	ruleFileExtMD  ruleFileExt = ".md"
	ruleFileExtMDC ruleFileExt = ".mdc"
)

// RuleTargetFormatFromExt maps a rule file extension to the target harness format.
func RuleTargetFormatFromExt(ext string) (RuleTargetFormat, error) {
	switch ruleFileExt(ext) {
	case ruleFileExtMD:
		return RuleTargetClaude, nil
	case ruleFileExtMDC:
		return RuleTargetCursor, nil
	default:
		return "", fmt.Errorf("unsupported rule_ext %q", ext)
	}
}

// RuleSource is a parsed neutral corpus rule with metadata and Markdown body.
type RuleSource struct {
	Description string
	AppliesTo   []string
	Always      bool
	Body        string
}

type ruleSourceYAML struct {
	Description string   `yaml:"description,omitempty"`
	AppliesTo   []string `yaml:"applies_to,omitempty"`
	Always      *bool    `yaml:"always,omitempty"`
	Globs       string   `yaml:"globs,omitempty"`
	AlwaysApply *bool    `yaml:"alwaysApply,omitempty"`
}

type cursorRuleFrontmatter struct {
	Description string `yaml:"description,omitempty"`
	Globs       string `yaml:"globs,omitempty"`
	AlwaysApply bool   `yaml:"alwaysApply,omitempty"`
}

type claudeRuleFrontmatter struct {
	Paths []string `yaml:"paths,omitempty"`
}

type copilotRuleFrontmatter struct {
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
	ApplyTo     string `yaml:"applyTo,omitempty"`
}

// ParseRuleSource reads neutral corpus rule metadata and body from file content.
func ParseRuleSource(content string) (RuleSource, error) {
	if !strings.HasPrefix(content, "---\n") {
		return emptyRuleSource(content), nil
	}
	endMarker := "\n---\n"
	frontmatterEnd := strings.Index(content[4:], endMarker)
	if frontmatterEnd == -1 {
		return RuleSource{Description: "", AppliesTo: nil, Always: false, Body: ""}, fmt.Errorf("rule source front matter missing closing delimiter")
	}

	var raw ruleSourceYAML
	rawFrontmatter := content[4 : 4+frontmatterEnd]
	if err := yaml.Unmarshal([]byte(rawFrontmatter), &raw); err != nil {
		slog.Warn("compilation: ParseRuleSource yaml failed", "err", err)
		return RuleSource{Description: "", AppliesTo: nil, Always: false, Body: ""}, fmt.Errorf("parsing rule source front matter: %w", err)
	}

	bodyStart := 4 + frontmatterEnd + len(endMarker)
	rule := RuleSource{
		Description: strings.TrimSpace(raw.Description),
		AppliesTo:   append([]string(nil), raw.AppliesTo...),
		Always:      false,
		Body:        content[bodyStart:],
	}
	if raw.Always != nil {
		rule.Always = *raw.Always
	} else if raw.AlwaysApply != nil {
		rule.Always = *raw.AlwaysApply
	}
	if len(rule.AppliesTo) == 0 && strings.TrimSpace(raw.Globs) != "" {
		rule.AppliesTo = splitGlobs(raw.Globs)
	}
	return rule, nil
}

func emptyRuleSource(body string) RuleSource {
	return RuleSource{Description: "", AppliesTo: nil, Always: false, Body: body}
}

func splitGlobs(value string) []string {
	parts := strings.Split(value, ",")
	globs := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			globs = append(globs, trimmed)
		}
	}
	return globs
}

func (r RuleSource) hasMetadata() bool {
	return r.Description != "" || len(r.AppliesTo) > 0 || r.Always
}

func (r RuleSource) isScoped() bool {
	return len(r.AppliesTo) > 0 && !r.Always
}

func (r RuleSource) globsString() string {
	return strings.Join(r.AppliesTo, ",")
}

func (r RuleSource) applyToString() string {
	if r.Always {
		return "**/*"
	}
	return r.globsString()
}

// RenderForTarget renders the rule for the given harness format.
func (r RuleSource) RenderForTarget(format RuleTargetFormat) (string, error) {
	switch format {
	case RuleTargetClaude:
		return r.renderClaude()
	case RuleTargetCursor:
		return r.renderCursor()
	default:
		return "", fmt.Errorf("unknown rule target format %q", format)
	}
}

func (r RuleSource) renderCursor() (string, error) {
	body := strings.TrimSpace(r.Body)
	if !r.hasMetadata() {
		if body == "" {
			return "", nil
		}
		return body + "\n", nil
	}
	frontmatter := cursorRuleFrontmatter{
		Description: r.Description,
		Globs:       r.globsString(),
		AlwaysApply: r.Always,
	}
	renderedFrontmatter, err := marshalFrontmatter(frontmatter)
	if err != nil {
		return "", err
	}
	if body == "" {
		return renderedFrontmatter + "\n", nil
	}
	return renderedFrontmatter + "\n" + body + "\n", nil
}

func (r RuleSource) renderClaude() (string, error) {
	body := strings.TrimSpace(r.Body)
	if !r.isScoped() {
		if body == "" {
			return "", nil
		}
		return body + "\n", nil
	}
	frontmatter := claudeRuleFrontmatter{Paths: r.AppliesTo}
	renderedFrontmatter, err := marshalFrontmatter(frontmatter)
	if err != nil {
		return "", err
	}
	if body == "" {
		return renderedFrontmatter + "\n", nil
	}
	return renderedFrontmatter + "\n" + body + "\n", nil
}

// RenderCopilot renders a Copilot .instructions.md file for the named rule.
func (r RuleSource) RenderCopilot(name string) (string, error) {
	body := strings.TrimSpace(r.Body)
	frontmatter := copilotRuleFrontmatter{
		Name:        name,
		Description: r.Description,
		ApplyTo:     r.applyToString(),
	}
	renderedFrontmatter, err := marshalFrontmatter(frontmatter)
	if err != nil {
		return "", err
	}
	if body == "" {
		return renderedFrontmatter + "\n" + GeneratedAgentHTMLMarker + "\n", nil
	}
	return renderedFrontmatter + "\n" + GeneratedAgentHTMLMarker + "\n\n" + body + "\n", nil
}

func marshalFrontmatter[T cursorRuleFrontmatter | claudeRuleFrontmatter | copilotRuleFrontmatter](metadata T) (string, error) {
	content, err := yaml.Marshal(metadata)
	if err != nil {
		slog.Warn("compilation: marshalFrontmatter yaml failed", "err", err)
		return "", fmt.Errorf("marshaling front matter: %w", err)
	}
	return "---\n" + string(content) + "---\n", nil
}
