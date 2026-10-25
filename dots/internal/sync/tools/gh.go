package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/sync/corpus"
	"goodkind.io/.dotfiles/internal/telemetry"
)

type ghSkillDocument struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Metadata    struct {
		Repository string `yaml:"github-repo"`
		TreeSHA    string `yaml:"github-tree-sha"`
	} `yaml:"metadata"`
}

var skillReferencePattern = regexp.MustCompile("`(references/[^`]+)`|\\[[^\\]]*\\]\\((references/[^)]+)\\)")

func installGHExtension(ctx context.Context, tool catalog.ToolDeclaration, strictMode bool, logger *telemetry.Logger) error {
	if tool.Repo == "" {
		return fmt.Errorf("configure repo for GitHub CLI extension %s", tool.ID)
	}
	output, err := cmdexec.OutputWithLogger(ctx, logger, "gh", "extension", "list")
	if err != nil {
		slog.WarnContext(ctx, "listing GitHub CLI extensions", "err", err)
		return fmt.Errorf("listing GitHub CLI extensions: %w; install and authenticate gh", err)
	}
	installed := false
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] != "gh "+tool.Bin {
			continue
		}
		if fields[1] != tool.Repo {
			return fmt.Errorf("extension %s uses %s instead of %s; resolve the installation conflict", tool.Bin, fields[1], tool.Repo)
		}
		installed = true
	}
	if !installed {
		if err := cmdexec.RunWithLogger(ctx, logger, "gh", "extension", "install", tool.Repo); err != nil {
			slog.WarnContext(ctx, "installing GitHub CLI extension", "repo", tool.Repo, "err", err)
			return fmt.Errorf("installing extension %s: %w", tool.Repo, err)
		}
	} else if !strictMode {
		if err := cmdexec.RunWithLogger(ctx, logger, "gh", "extension", "upgrade", tool.Bin); err != nil {
			slog.WarnContext(ctx, "upgrading GitHub CLI extension", "extension", tool.Bin, "err", err)
			return fmt.Errorf("upgrading extension %s: %w", tool.Bin, err)
		}
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "gh", tool.Bin, "--help"); err != nil {
		slog.WarnContext(ctx, "verifying GitHub CLI extension", "extension", tool.Bin, "err", err)
		return fmt.Errorf("verifying extension %s: %w", tool.Bin, err)
	}
	return nil
}

func installGHSkill(ctx context.Context, tool catalog.ToolDeclaration, strictMode bool, logger *telemetry.Logger) error {
	if tool.Repo == "" || len(tool.Destinations) == 0 {
		return fmt.Errorf("configure repo and destinations for GitHub skill %s", tool.ID)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		slog.WarnContext(ctx, "resolving skill installation home", "err", err)
		return fmt.Errorf("resolving skill installation home: %w", err)
	}
	for _, relative := range tool.Destinations {
		destination, err := corpus.ResolveOutputDest(home, relative)
		if err != nil {
			slog.WarnContext(ctx, "resolving skill destination", "destination", relative, "err", err)
			return fmt.Errorf("resolving skill destination %s: %w", relative, err)
		}
		if err := installGHSkillAt(ctx, tool, destination, strictMode, logger); err != nil {
			return err
		}
	}
	return nil
}

func installGHSkillAt(ctx context.Context, tool catalog.ToolDeclaration, destination string, strictMode bool, logger *telemetry.Logger) error {
	skillPath := filepath.Join(destination, tool.Bin)
	info, err := os.Lstat(skillPath)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlinked skill directory %s requires a local installation before updating", skillPath)
	}
	switch {
	case os.IsNotExist(err):
		if err := cmdexec.RunWithLogger(ctx, logger, "gh", "skill", "install", tool.Repo, tool.Bin, "--dir", destination); err != nil {
			slog.WarnContext(ctx, "installing GitHub skill", "skill", tool.Bin, "err", err)
			return fmt.Errorf("installing skill %s: %w; use gh with skill support", tool.Bin, err)
		}
	case err != nil:
		slog.WarnContext(ctx, "checking skill directory", "path", skillPath, "err", err)
		return fmt.Errorf("checking skill directory %s: %w", skillPath, err)
	default:
		if err := verifyGHSkill(skillPath, tool); err != nil {
			return err
		}
		if !strictMode {
			if err := cmdexec.RunWithLogger(ctx, logger, "gh", "skill", "update", tool.Bin, "--all", "--dir", destination); err != nil {
				slog.WarnContext(ctx, "updating GitHub skill", "skill", tool.Bin, "err", err)
				return fmt.Errorf("updating skill %s: %w", tool.Bin, err)
			}
		}
	}
	return verifyGHSkill(skillPath, tool)
}

func verifyGHSkill(skillPath string, tool catalog.ToolDeclaration) error {
	content, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if err != nil {
		slog.Warn("reading GitHub skill", "skill", tool.Bin, "err", err)
		return fmt.Errorf("reading skill %s: %w", tool.Bin, err)
	}
	parts := strings.SplitN(strings.ReplaceAll(string(content), "\r\n", "\n"), "---\n", 3)
	if len(parts) != 3 || parts[0] != "" {
		return fmt.Errorf("skill %s has invalid frontmatter; resolve the installation conflict", tool.Bin)
	}
	var document ghSkillDocument
	if err := yaml.Unmarshal([]byte(parts[1]), &document); err != nil {
		slog.Warn("parsing GitHub skill frontmatter", "skill", tool.Bin, "err", err)
		return fmt.Errorf("parsing skill %s frontmatter: %w", tool.Bin, err)
	}
	if document.Name != tool.Bin || strings.TrimSpace(document.Description) == "" {
		return fmt.Errorf("skill %s requires matching name and description", tool.Bin)
	}
	if strings.TrimSuffix(document.Metadata.Repository, "/") != "https://github.com/"+tool.Repo || document.Metadata.TreeSHA == "" {
		return fmt.Errorf("skill %s at %s lacks source metadata for %s; resolve the installation conflict", tool.Bin, skillPath, tool.Repo)
	}
	resolvedSkill, err := filepath.EvalSymlinks(skillPath)
	if err != nil {
		slog.Warn("resolving GitHub skill directory", "path", skillPath, "err", err)
		return fmt.Errorf("resolving skill directory %s: %w", skillPath, err)
	}
	for _, match := range skillReferencePattern.FindAllStringSubmatch(parts[2], -1) {
		reference := match[1]
		if reference == "" {
			reference = match[2]
		}
		reference = strings.SplitN(reference, "#", 2)[0]
		if err := verifySkillReference(resolvedSkill, reference); err != nil {
			return err
		}
	}
	return nil
}

func verifySkillReference(skillPath, reference string) error {
	resolved, err := filepath.EvalSymlinks(filepath.Join(skillPath, filepath.FromSlash(reference)))
	if err != nil {
		slog.Warn("resolving GitHub skill reference", "reference", reference, "err", err)
		return fmt.Errorf("resolving skill reference %s: %w", reference, err)
	}
	relative, err := filepath.Rel(skillPath, resolved)
	if err != nil {
		slog.Warn("checking GitHub skill reference directory", "reference", reference, "err", err)
		return fmt.Errorf("checking skill reference %s: %w", reference, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("skill reference %s resolves outside skill directory", reference)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		slog.Warn("reading GitHub skill reference", "reference", reference, "err", err)
		return fmt.Errorf("reading skill reference %s: %w", reference, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("skill reference %s must be a regular file", reference)
	}
	return nil
}
