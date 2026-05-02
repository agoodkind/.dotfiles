package agentsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/compilation"
	"goodkind.io/.dotfiles/internal/telemetry"
)

var agentSyncLogger *telemetry.Logger

type resolvedRepo struct {
	path        string
	showedUsage bool
}

func Run(ctx context.Context, args ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	logPath := filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "sync-agent-repo.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		return err
	}
	agentSyncLogger = logger
	defer logger.Close()
	defer func() {
		agentSyncLogger = nil
		runner.SetLogger(nil)
	}()
	runner.SetLogger(agentSyncLogger)
	_ = os.Setenv("DOTFILES_LOG", logPath)

	done := logger.Section("Agent repo sync")
	defer done()

	repoInfo, err := resolveRepoRoot(ctx, args)
	if err != nil {
		return err
	}
	if repoInfo.showedUsage {
		return nil
	}

	source := compilation.ResolveAgentSource(repoInfo.path)
	if _, statErr := os.Stat(source.Root); statErr != nil {
		return fmt.Errorf("no .agents directory found in %s", repoInfo.path)
	}
	if err := compilation.EnsureCursorCompatibilityLink(repoInfo.path); err != nil {
		return err
	}

	claudeDir := filepath.Join(repoInfo.path, ".claude")
	agentsDir := filepath.Join(repoInfo.path, ".agents")
	githubDir := filepath.Join(repoInfo.path, ".github")

	logInfof("Syncing %s into agent targets", source.Root)

	if err := compilation.SyncFilesToDir(source.Commands, filepath.Join(claudeDir, "commands")); err != nil {
		return err
	}
	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(claudeDir, "skills")); err != nil {
		return err
	}
	if err := compilation.SyncRulesFromDirAsMd(source.Rules, filepath.Join(claudeDir, "rules")); err != nil {
		return err
	}
	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(claudeDir, "CLAUDE.md"), "Claude Memory"); err != nil {
		return err
	}

	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(repoInfo.path, "AGENTS.md"), "Agent Instructions"); err != nil {
		return err
	}
	if err := compilation.RenderRulesAsInstructionDoc(source.Rules, filepath.Join(githubDir, "copilot-instructions.md"), "Copilot Instructions"); err != nil {
		return err
	}
	if err := compilation.RenderCopilotInstructionFiles(source.Rules, filepath.Join(githubDir, "instructions")); err != nil {
		return err
	}
	if err := compilation.RenderCopilotPromptFiles(source.Commands, filepath.Join(githubDir, "prompts")); err != nil {
		return err
	}
	if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(githubDir, "skills")); err != nil {
		return err
	}

	if filepath.Clean(source.Root) != filepath.Clean(agentsDir) {
		if err := compilation.SyncFilesToDir(source.Commands, filepath.Join(agentsDir, "commands")); err != nil {
			return err
		}
		if err := compilation.SyncRulesFromDir(source.Rules, filepath.Join(agentsDir, "rules")); err != nil {
			return err
		}
		if err := compilation.SyncSkillDirs(source.Skills, filepath.Join(agentsDir, "skills")); err != nil {
			return err
		}
		if err := compilation.SyncCommandFilesAsSkillDirs(source.Commands, filepath.Join(agentsDir, "skills"), "cursor-command-"); err != nil {
			return err
		}
	}

	logSuccess("Done.")
	return nil
}

func resolveRepoRoot(ctx context.Context, args []string) (resolvedRepo, error) {
	repo := ""
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			logInfo("Usage: dots sync-agent-repo [path]")
			return resolvedRepo{showedUsage: true}, nil
		}
		if strings.HasPrefix(arg, "-") {
			return resolvedRepo{}, fmt.Errorf("unsupported flag: %s", arg)
		}
		if repo != "" {
			return resolvedRepo{}, fmt.Errorf("too many arguments, expected at most one repo path")
		}
		repo = arg
	}

	if repo == "" {
		output, err := cmdexec.OutputWithLogger(ctx, nil, "git", "rev-parse", "--show-toplevel")
		if err == nil {
			trimmed := strings.TrimSpace(output)
			if trimmed != "" {
				return resolvedRepo{path: filepath.Clean(trimmed)}, nil
			}
		}
		repo, err = os.Getwd()
		if err != nil {
			return resolvedRepo{}, err
		}
	}

	return resolvedRepo{path: filepath.Clean(repo)}, nil
}

func logSuccess(message string) {
	if agentSyncLogger != nil {
		agentSyncLogger.Success(message)
	}
}

func logInfo(message string) {
	if agentSyncLogger != nil {
		agentSyncLogger.Info(message)
	}
}

func logInfof(format string, args ...any) {
	if agentSyncLogger != nil {
		agentSyncLogger.Info(fmt.Sprintf(format, args...))
	}
}
