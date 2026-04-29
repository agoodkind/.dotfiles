package agentsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/runner"
	"github.com/agoodkind/.dotfiles/internal/sync/compilation"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
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

	cursorDir := filepath.Join(repoInfo.path, ".cursor")
	if _, statErr := os.Stat(cursorDir); statErr != nil {
		return fmt.Errorf("no .cursor directory found in %s", repoInfo.path)
	}

	claudeDir := filepath.Join(repoInfo.path, ".claude")
	agentsDir := filepath.Join(repoInfo.path, ".agents")
	srcCommands := filepath.Join(cursorDir, "commands")
	srcSkills := filepath.Join(cursorDir, "skills")
	srcRules := filepath.Join(cursorDir, "rules")

	logInfof("Syncing %s into %s and %s", cursorDir, claudeDir, agentsDir)

	if err := compilation.SyncFilesToDir(srcCommands, filepath.Join(claudeDir, "commands")); err != nil {
		return err
	}
	if err := compilation.SyncSkillDirs(srcSkills, filepath.Join(claudeDir, "skills")); err != nil {
		return err
	}
	if err := compilation.SyncRulesFromDirAsMd(srcRules, filepath.Join(claudeDir, "rules")); err != nil {
		return err
	}
	if err := compilation.RenderRulesAsInstructionDoc(srcRules, filepath.Join(claudeDir, "CLAUDE.md"), "Claude Memory"); err != nil {
		return err
	}

	if err := compilation.SyncFilesToDir(srcCommands, filepath.Join(agentsDir, "commands")); err != nil {
		return err
	}
	if err := compilation.SyncRulesFromDir(srcRules, filepath.Join(agentsDir, "rules")); err != nil {
		return err
	}
	if err := compilation.SyncSkillDirs(srcSkills, filepath.Join(agentsDir, "skills")); err != nil {
		return err
	}
	if err := compilation.SyncCommandFilesAsSkillDirs(srcCommands, filepath.Join(agentsDir, "skills"), "cursor-command-"); err != nil {
		return err
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
