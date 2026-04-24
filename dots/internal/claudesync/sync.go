package claudesync

import (
	"context"
	"fmt"
	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/runner"
	"github.com/agoodkind/.dotfiles/internal/sync/common"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
	"os"
	"path/filepath"
	"strings"
)

var claudeSyncLogger *telemetry.Logger

func Run(ctx context.Context, args ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	logPath := filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "sync-claude-repo.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		return err
	}
	claudeSyncLogger = logger
	defer logger.Close()
	defer func() {
		claudeSyncLogger = nil
		runner.SetLogger(nil)
	}()
	runner.SetLogger(claudeSyncLogger)
	_ = os.Setenv("DOTFILES_LOG", logPath)

	done := logger.Section("CLAUDE repo sync")
	defer done()

	repo := ""
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			logInfo("Usage: dots sync-claude-repo [path]")
			return nil
		}
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("unsupported flag: %s", arg)
		}
		if repo != "" {
			return fmt.Errorf("too many arguments, expected at most one repo path")
		}
		repo = arg
	}

	if repo == "" {
		repo, err = repoRoot(ctx)
		if err != nil {
			return err
		}
	}
	repo = filepath.Clean(repo)

	cursorDir := filepath.Join(repo, ".cursor")
	claudeDir := filepath.Join(repo, ".claude")
	if _, err := os.Stat(cursorDir); err != nil {
		return fmt.Errorf("No .cursor/ dir found in %s", repo)
	}

	logInfof("Syncing %s → %s", cursorDir, claudeDir)

	commandsLinked, err := linkFiles(filepath.Join(cursorDir, "commands"), filepath.Join(claudeDir, "commands"), ".md", ".md")
	if err != nil {
		return err
	}
	rulesLinked, err := linkFiles(filepath.Join(cursorDir, "rules"), filepath.Join(claudeDir, "rules"), ".mdc", ".md")
	if err != nil {
		return err
	}
	skillsLinked, err := linkDirs(filepath.Join(cursorDir, "skills"), filepath.Join(claudeDir, "skills"))
	if err != nil {
		return err
	}
	logInfo(linkedSummary(commandsLinked, rulesLinked, skillsLinked))
	logSuccess("Done.")
	return nil
}

func logSuccess(message string) {
	if claudeSyncLogger != nil {
		claudeSyncLogger.Success(message)
	}
}

func linkFiles(sourceDir, destinationDir, sourceExt, destinationExt string) (int, error) {
	if destinationExt == "" {
		destinationExt = sourceExt
	}
	if _, err := os.Stat(sourceDir); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return 0, err
	}

	pattern := filepath.Join(sourceDir, "*"+sourceExt)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return 0, err
	}
	linked := 0
	for _, src := range matches {
		info, err := os.Stat(src)
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		base := filepath.Base(src)
		dstName := strings.TrimSuffix(base, sourceExt) + destinationExt
		dst := filepath.Join(destinationDir, dstName)
		if common.IsSymlinkTo(dst, src) {
			continue
		}
		if err := os.RemoveAll(dst); err != nil {
			return linked, err
		}
		if err := os.Symlink(src, dst); err != nil {
			return linked, err
		}
		linked++
	}
	if linked > 0 {
		logInfof(
			"  linked %d file(s): %s → %s",
			linked,
			filepath.Base(sourceDir),
			filepath.Base(destinationDir),
		)
	}
	return linked, nil
}

func linkDirs(sourceDir, destinationDir string) (int, error) {
	if _, err := os.Stat(sourceDir); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return 0, err
	}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return 0, err
	}

	linked := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		src := filepath.Join(sourceDir, entry.Name())
		dst := filepath.Join(destinationDir, entry.Name())
		if common.IsSymlinkTo(dst, src) {
			continue
		}
		if err := os.RemoveAll(dst); err != nil {
			return linked, err
		}
		if err := os.Symlink(src, dst); err != nil {
			return linked, err
		}
		linked++
	}
	if linked > 0 {
		logInfof(
			"  linked %d skill(s): %s → %s",
			linked,
			filepath.Base(sourceDir),
			filepath.Base(destinationDir),
		)
	}
	return linked, nil
}

func repoRoot(ctx context.Context) (string, error) {
	output, err := cmdexec.OutputWithLogger(ctx, nil, "git", "rev-parse", "--show-toplevel")
	if err == nil {
		repo := strings.TrimSpace(string(output))
		if repo != "" {
			return repo, nil
		}
	}

	return os.Getwd()
}

func logInfo(message string) {
	if claudeSyncLogger != nil {
		claudeSyncLogger.Info(message)
	}
}

func logInfof(format string, args ...any) {
	if claudeSyncLogger != nil {
		claudeSyncLogger.Info(fmt.Sprintf(format, args...))
	}
}

func linkedSummary(commands, rules, skills int) string {
	if commands == 0 && rules == 0 && skills == 0 {
		return "No new links were needed."
	}
	return fmt.Sprintf("linked %d file(s), %d rule(s), %d skill(s)", commands, rules, skills)
}
