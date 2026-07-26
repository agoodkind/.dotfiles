package compilation

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type AgentTargetFormat string

const (
	AgentTargetClaude  AgentTargetFormat = "claude"
	AgentTargetCursor  AgentTargetFormat = "cursor"
	AgentTargetGemini  AgentTargetFormat = "gemini"
	AgentTargetCodex   AgentTargetFormat = "codex"
	AgentTargetCopilot AgentTargetFormat = "copilot"
)

type markdownAgentFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	ReadOnly    bool     `yaml:"readonly,omitempty"`
	Tools       []string `yaml:"tools,omitempty"`
}

type codexAgentFile struct {
	Name                  string `toml:"name"`
	Description           string `toml:"description"`
	SandboxMode           string `toml:"sandbox_mode,omitempty"`
	DeveloperInstructions string `toml:"developer_instructions"`
}

// ValidateAgentFiles validates provider-specific agent rendering without writing files.
func ValidateAgentFiles(agents map[string]AgentSource, dst string, format AgentTargetFormat) error {
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	extension, err := agentExtension(format)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := validateAgentName(name); err != nil {
			return err
		}
		if _, err := renderAgent(agents[name], format); err != nil {
			slog.Warn("compilation: validating agent render", "name", name, "format", format, "err", err)
			return fmt.Errorf("rendering agent %s: %w", name, err)
		}
		if err := validateNoNestedSymlink(dst, filepath.Join(dst, name+extension)); err != nil {
			return err
		}
	}
	return nil
}

func RenderAgentFiles(agents map[string]AgentSource, dst string, format AgentTargetFormat) error {
	slog.Info("compilation: rendering agent files", "dest", dst, "format", string(format))
	if err := os.MkdirAll(dst, 0o755); err != nil {
		slog.Warn("compilation: creating agent directory", "dest", dst, "err", err)
		return fmt.Errorf("creating directory %s: %w", dst, err)
	}
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	extension, err := agentExtension(format)
	if err != nil {
		return err
	}
	activeFiles := make(map[string]struct{}, len(names))
	renderedAgents := make(map[string]string, len(names))
	for _, name := range names {
		if err := validateAgentName(name); err != nil {
			return err
		}
		fileName := name + extension
		activeFiles[fileName] = struct{}{}
		rendered, err := renderAgent(agents[name], format)
		if err != nil {
			return fmt.Errorf("rendering agent %s: %w", name, err)
		}
		renderedAgents[fileName] = rendered
	}
	for _, name := range names {
		fileName := name + extension
		targetPath := filepath.Join(dst, fileName)
		if isSymlink(targetPath) {
			if err := os.Remove(targetPath); err != nil {
				return fmt.Errorf("removing symlink %s: %w", targetPath, err)
			}
		}
		if err := writeFileIfChanged(targetPath, []byte(renderedAgents[fileName])); err != nil {
			return err
		}
	}
	return removeMissingManagedFiles(dst, extension, activeFiles)
}

func agentExtension(format AgentTargetFormat) (string, error) {
	switch format {
	case AgentTargetClaude, AgentTargetCursor, AgentTargetGemini:
		return ".md", nil
	case AgentTargetCodex:
		return ".toml", nil
	case AgentTargetCopilot:
		return ".agent.md", nil
	default:
		return "", fmt.Errorf("unknown agent target format %q", format)
	}
}

func renderAgent(agent AgentSource, format AgentTargetFormat) (string, error) {
	switch format {
	case AgentTargetClaude:
		return renderMarkdownAgent(agent, false, readOnlyTools(agent.ReadOnly, []string{"Read", "Grep", "Glob", "WebFetch", "WebSearch"}))
	case AgentTargetCursor:
		return renderMarkdownAgent(agent, agent.ReadOnly, nil)
	case AgentTargetGemini:
		return renderMarkdownAgent(agent, false, readOnlyTools(agent.ReadOnly, []string{"read_file", "grep_search", "glob", "list_directory", "web_fetch", "google_web_search"}))
	case AgentTargetCodex:
		return renderCodexAgent(agent)
	case AgentTargetCopilot:
		return renderMarkdownAgent(agent, false, readOnlyTools(agent.ReadOnly, []string{"read", "search"}))
	default:
		return "", fmt.Errorf("unknown agent target format %q", format)
	}
}

func readOnlyTools(readOnly bool, tools []string) []string {
	if !readOnly {
		return nil
	}
	return tools
}

func renderMarkdownAgent(agent AgentSource, readOnly bool, tools []string) (string, error) {
	frontmatter := markdownAgentFrontmatter{
		Name:        agent.Name,
		Description: agent.Description,
		ReadOnly:    readOnly,
		Tools:       tools,
	}
	rawFrontmatter, err := yaml.Marshal(frontmatter)
	if err != nil {
		slog.Warn("compilation: marshaling agent frontmatter", "agent", agent.Name, "err", err)
		return "", fmt.Errorf("marshaling agent front matter: %w", err)
	}
	content := "---\n" + string(rawFrontmatter) + "---\n\n" + strings.TrimSpace(agent.Body) + "\n"
	return injectSkillMarker(content), nil
}

func renderCodexAgent(agent AgentSource) (string, error) {
	sandboxMode := ""
	if agent.ReadOnly {
		sandboxMode = "read-only"
	}
	payload, err := toml.Marshal(codexAgentFile{
		Name:                  agent.Name,
		Description:           agent.Description,
		SandboxMode:           sandboxMode,
		DeveloperInstructions: strings.TrimSpace(agent.Body),
	})
	if err != nil {
		slog.Warn("compilation: marshaling Codex agent", "agent", agent.Name, "err", err)
		return "", fmt.Errorf("marshaling Codex agent: %w", err)
	}
	return GeneratedAgentMarker + "\n\n" + string(payload), nil
}

func validateAgentName(name string) error {
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || cleanName == ".." || filepath.IsAbs(cleanName) {
		return fmt.Errorf("invalid agent name %q", name)
	}
	if strings.Contains(name, "/") || strings.Contains(name, string(filepath.Separator)) {
		return fmt.Errorf("invalid agent name %q", name)
	}
	if strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) || cleanName != name {
		return fmt.Errorf("invalid agent name %q", name)
	}
	return nil
}
