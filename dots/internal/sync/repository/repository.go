package repository

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/sync/common"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

func UpdateRepo(ctx context.Context, dotfiles string, logger *telemetry.Logger) (bool, string, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if dotfiles == "" {
		dotfiles = filepath.Join(os.Getenv("HOME"), ".dotfiles")
	}
	_ = os.Setenv("DOTDOTFILES", dotfiles)

	preSHA, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "HEAD")
	if err != nil {
		return false, "", "", err
	}
	output, err := runDotfilesUpdate(ctx, dotfiles, logger)
	if err != nil {
		return false, "", "", err
	}

	oldSHA, newSHA := parsePulledLine(output)
	if oldSHA == "" || newSHA == "" {
		postSHA, postErr := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "HEAD")
		if postErr != nil {
			return false, "", "", postErr
		}
		if strings.TrimSpace(preSHA) != strings.TrimSpace(postSHA) {
			return true, strings.TrimSpace(preSHA), strings.TrimSpace(postSHA), nil
		}
		return false, "", "", nil
	}
	return true, oldSHA, newSHA, nil
}

func UpdateGitRepoSync(ctx context.Context, skipGit bool, logger *telemetry.Logger) error {
	if skipGit {
		return nil
	}
	_, _, _, err := UpdateRepo(ctx, os.Getenv("DOTDOTFILES"), logger)
	return err
}

func runDotfilesUpdate(ctx context.Context, dotfiles string, logger *telemetry.Logger) (string, error) {
	reason := checkDotfilesGitHealth(ctx, dotfiles, logger)
	if reason != "" {
		return "", fmt.Errorf("skip: %s", reason)
	}
	if err := clearGitLocks(dotfiles); err != nil {
		common.Warn(logger, "  failed clearing git locks: "+err.Error())
	}

	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "fetch", "origin", "--prune"); err != nil {
		return "fetch failed", err
	}

	remoteStatus := getRemoteStatus(ctx, dotfiles, "origin/main", logger)
	common.Info(logger, "  remote status: "+remoteStatus)
	switch remoteStatus {
	case "up-to-date":
		_ = syncDotfilesSubmodules(ctx, dotfiles, logger)
		return "", nil
	case "diverged":
		return "", fmt.Errorf("local history diverged from origin/main, needs manual fix")
	case "behind":
		// continue below
	case "unknown":
		return "", fmt.Errorf("unable to determine remote status")
	default:
		return "", fmt.Errorf("unknown remote status: %s", remoteStatus)
	}

	hasChanges, err := hasLocalChanges(ctx, dotfiles, logger)
	if err != nil {
		return "", err
	}
	if hasChanges {
		if conflicting, err := hasConflictingChanges(ctx, dotfiles, logger); err == nil && conflicting {
			return "", fmt.Errorf("upstream changes conflict with local work (overlapping files)")
		}
		if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "stash", "--include-untracked"); err != nil {
			return "", err
		}
	}
	prePullHead, err := updateWithRevert(ctx, dotfiles, hasChanges, logger)
	if err != nil {
		return "", err
	}
	if hasChanges {
		if restoreErr := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "stash", "pop"); restoreErr != nil {
			return "", fmt.Errorf("stash pop failed after pull, repository restored: %w", restoreErr)
		}
	}
	postPullHead, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(prePullHead) != strings.TrimSpace(postPullHead) {
		_ = syncDotfilesSubmodules(ctx, dotfiles, logger)
		return "pulled:" + strings.TrimSpace(prePullHead) + ":" + strings.TrimSpace(postPullHead), nil
	}
	return "", nil
}

func checkDotfilesGitHealth(ctx context.Context, dotfiles string, logger *telemetry.Logger) string {
	branch, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "symbolic-ref", "-q", "HEAD")
	if err != nil || strings.TrimSpace(branch) == "" {
		return "detached HEAD"
	}
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "MERGE_HEAD"); err == nil {
		return "merge in progress"
	}
	if _, err := os.Stat(filepath.Join(dotfiles, ".git", "rebase-merge")); err == nil {
		return "rebase in progress"
	}
	if _, err := os.Stat(filepath.Join(dotfiles, ".git", "rebase-apply")); err == nil {
		return "rebase in progress"
	}
	if output, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "ls-files", "-u"); err == nil && strings.TrimSpace(output) != "" {
		return "unmerged paths"
	}
	return ""
}

func clearGitLocks(dotfiles string) error {
	paths := []string{
		filepath.Join(dotfiles, ".git", "index.lock"),
		filepath.Join(dotfiles, ".git", "objects", "info", "commit-graph-chain.lock"),
	}
	for _, path := range paths {
		_ = os.Remove(path)
	}
	return nil
}

func getRemoteStatus(ctx context.Context, dotfiles string, remoteRef string, logger *telemetry.Logger) string {
	latest, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", remoteRef)
	if err != nil || strings.TrimSpace(latest) == "" {
		return "unknown"
	}
	current, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(current) == "" {
		return "unknown"
	}
	latest = strings.TrimSpace(latest)
	current = strings.TrimSpace(current)
	if current == latest {
		return "up-to-date"
	}
	if isMergeBaseAncestor(ctx, dotfiles, current, latest, logger) {
		return "behind"
	}
	if isMergeBaseAncestor(ctx, dotfiles, latest, current, logger) {
		return "up-to-date"
	}
	return "diverged"
}

func isMergeBaseAncestor(ctx context.Context, dotfiles string, ancestor string, descendant string, logger *telemetry.Logger) bool {
	cmdErr := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "merge-base", "--is-ancestor", ancestor, descendant)
	return cmdErr == nil
}

func hasLocalChanges(ctx context.Context, dotfiles string, logger *telemetry.Logger) (bool, error) {
	output, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "status", "--porcelain", "--untracked-files=no", "--ignore-submodules")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func hasConflictingChanges(ctx context.Context, dotfiles string, logger *telemetry.Logger) (bool, error) {
	upstream, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "diff", "--name-only", "HEAD", "origin/main")
	if err != nil {
		return false, nil
	}
	localChanged, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "diff", "--name-only")
	if err != nil {
		return false, nil
	}
	upstreamSet := make(map[string]struct{})
	for _, line := range strings.Split(strings.TrimSpace(upstream), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			upstreamSet[line] = struct{}{}
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(localChanged), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if _, ok := upstreamSet[line]; ok {
				return true, nil
			}
		}
	}
	return false, nil
}

func updateWithRevert(ctx context.Context, dotfiles string, hadChanges bool, logger *telemetry.Logger) (string, error) {
	prePullHead, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	prePullHead = strings.TrimSpace(prePullHead)
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "pull", "--ff-only", "origin/main"); err != nil {
		_ = cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "reset", "--hard", prePullHead)
		if hadChanges {
			_ = cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "stash", "pop")
		}
		return prePullHead, fmt.Errorf("pull failed, rolled back")
	}
	return prePullHead, nil
}

func syncDotfilesSubmodules(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	_ = cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "submodule", "update", "--init")
	subs := []string{"lib/zinit", "lib/zsh-defer"}
	for _, sub := range subs {
		if err := syncOneSubmodule(ctx, dotfiles, sub, logger); err != nil {
			return nil
		}
	}

	cachedDiff, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "diff", "--cached", "--name-only", "--ignore-submodules")
	if err == nil && strings.TrimSpace(cachedDiff) != "" {
		return nil
	}

	pointerDirty := false
	for _, sub := range subs {
		if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "diff", "--quiet", "--", sub); err != nil {
			pointerDirty = true
			_ = cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "add", "--", sub)
		}
	}
	if pointerDirty {
		_ = cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "commit", "-m", "Update submodule pointers", "--", "lib/zinit", "lib/zsh-defer")
	}
	return nil
}

func syncOneSubmodule(ctx context.Context, dotfiles string, subPath string, logger *telemetry.Logger) error {
	subAbs := filepath.Join(dotfiles, subPath)
	if _, err := os.Stat(filepath.Join(subAbs, ".git")); err != nil {
		return nil
	}
	branch, _ := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "config", "-f", filepath.Join(dotfiles, ".gitmodules"), "--get", "submodule."+subPath+".branch")
	branch = strings.TrimSpace(branch)
	if branch == "" {
		if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "rev-parse", "origin/main"); err == nil {
			branch = "main"
		} else if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "rev-parse", "origin/master"); err == nil {
			branch = "master"
		} else {
			branch = "main"
		}
	}
	_ = cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "fetch")
	_ = cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "checkout", branch)
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "pull", "--rebase", "origin", branch); err != nil {
		_ = cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "rebase", "--abort")
		common.Warn(logger, "  pull --rebase failed in "+subPath)
	}
	return nil
}

func LoadOverrides() {
	overrides := filepath.Join(os.Getenv("HOME"), ".overrides.local")
	file, err := os.Open(overrides)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(parts[1], "\"'")
		if key != "" {
			_ = os.Setenv(key, value)
		}
	}
}

func parsePulledLine(output string) (string, string) {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "pulled:") {
			parts := strings.SplitN(strings.TrimPrefix(line, "pulled:"), ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			}
			return "", ""
		}
	}
	return "", ""
}
