// Package repository implements git repository sync operations.
package repository

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/telemetry"
)

type remoteStatusCode string

const (
	remoteStatusUpToDate remoteStatusCode = "up-to-date"
	remoteStatusDiverged remoteStatusCode = "diverged"
	remoteStatusBehind   remoteStatusCode = "behind"
	remoteStatusUnknown  remoteStatusCode = "unknown"
)

// UpdateRepo fetches and fast-forwards the dotfiles git repository, returning whether commits were pulled and the old/new SHAs.
func UpdateRepo(ctx context.Context, dotfiles string, logger *telemetry.Logger) (bool, string, string, error) {
	if dotfiles == "" {
		dotfiles = filepath.Join(os.Getenv("HOME"), ".dotfiles")
	}
	_ = os.Setenv("DOTDOTFILES", dotfiles)

	preSHA, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "HEAD")
	if err != nil {
		slog.ErrorContext(ctx, "repository: UpdateRepo: rev-parse HEAD", "err", err)
		return false, "", "", fmt.Errorf("running git rev-parse HEAD: %w", err)
	}
	output, err := runDotfilesUpdate(ctx, dotfiles, logger)
	if err != nil {
		return false, "", "", err
	}

	oldSHA, newSHA := parsePulledLine(output)
	if oldSHA == "" || newSHA == "" {
		postSHA, postErr := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "HEAD")
		if postErr != nil {
			slog.ErrorContext(ctx, "repository: UpdateRepo: post rev-parse HEAD", "err", postErr)
			return false, "", "", fmt.Errorf("running git rev-parse HEAD: %w", postErr)
		}
		if strings.TrimSpace(preSHA) != strings.TrimSpace(postSHA) {
			return true, strings.TrimSpace(preSHA), strings.TrimSpace(postSHA), nil
		}
		return false, "", "", nil
	}
	return true, oldSHA, newSHA, nil
}

// UpdateGitRepoSync fetches the dotfiles repository unless skipGit is true.
func UpdateGitRepoSync(ctx context.Context, skipGit bool, logger *telemetry.Logger) error {
	if skipGit {
		return nil
	}
	pulled, _, _, err := UpdateRepo(ctx, os.Getenv("DOTDOTFILES"), logger)
	if pulled {
		slog.DebugContext(ctx, "repository: synced")
	}
	return err
}

func runDotfilesUpdate(ctx context.Context, dotfiles string, logger *telemetry.Logger) (string, error) {
	reason := checkDotfilesGitHealth(ctx, dotfiles, logger)
	if reason != "" {
		return "", fmt.Errorf("skip: %s", reason)
	}
	clearGitLocks(dotfiles)

	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "fetch", "origin", "--prune"); err != nil {
		return "fetch failed", fmt.Errorf("running git fetch: %w", err)
	}

	remoteStatus := getRemoteStatus(ctx, dotfiles, "origin/main", logger)
	logger.InfoContext(ctx, "  remote status: "+remoteStatus)
	switch remoteStatusCode(remoteStatus) {
	case remoteStatusUpToDate:
		_ = syncDotfilesSubmodules(ctx, dotfiles, logger)
		return "", nil
	case remoteStatusDiverged:
		return "", fmt.Errorf("local history diverged from origin/main, needs manual fix")
	case remoteStatusBehind:
		// continue below
	case remoteStatusUnknown:
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
			return "", fmt.Errorf("running git stash: %w", err)
		}
	}
	prePullHead, err := updateWithRevert(ctx, dotfiles, hasChanges, logger)
	if err != nil {
		return "", err
	}
	if hasChanges {
		if restoreErr := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "stash", "pop"); restoreErr != nil {
			slog.ErrorContext(ctx, "repository: runDotfilesUpdate: stash pop failed", "err", restoreErr)
			return "", fmt.Errorf("stash pop failed after pull, repository restored: %w", restoreErr)
		}
	}
	postPullHead, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("running git rev-parse HEAD: %w", err)
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
	if _, err := os.Stat(filepath.Clean(filepath.Join(dotfiles, ".git", "rebase-merge"))); err == nil {
		return "rebase in progress"
	}
	if _, err := os.Stat(filepath.Clean(filepath.Join(dotfiles, ".git", "rebase-apply"))); err == nil {
		return "rebase in progress"
	}
	if output, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "ls-files", "-u"); err == nil && strings.TrimSpace(output) != "" {
		return "unmerged paths"
	}
	return ""
}

func clearGitLocks(dotfiles string) {
	slog.Info("repository: clearGitLocks")
	paths := []string{
		filepath.Join(dotfiles, ".git", "index.lock"),
		filepath.Join(dotfiles, ".git", "objects", "info", "commit-graph-chain.lock"),
	}
	for _, path := range paths {
		_ = os.Remove(filepath.Clean(path))
	}
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
		slog.ErrorContext(ctx, "repository: hasLocalChanges: git status", "err", err)
		return false, fmt.Errorf("running git status: %w", err)
	}
	return strings.TrimSpace(output) != "", nil
}

func hasConflictingChanges(ctx context.Context, dotfiles string, logger *telemetry.Logger) (bool, error) {
	upstream, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "diff", "--name-only", "HEAD", "origin/main")
	if err != nil {
		slog.ErrorContext(ctx, "repository: hasConflictingChanges: git diff", "err", err)
		return false, fmt.Errorf("running git diff: %w", err)
	}
	localChanged, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "diff", "--name-only")
	if err != nil {
		slog.ErrorContext(ctx, "repository: hasConflictingChanges: git diff local", "err", err)
		return false, fmt.Errorf("running git diff: %w", err)
	}
	upstreamSet := make(map[string]struct{})
	for line := range strings.SplitSeq(strings.TrimSpace(upstream), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			upstreamSet[line] = struct{}{}
		}
	}
	for line := range strings.SplitSeq(strings.TrimSpace(localChanged), "\n") {
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
		slog.ErrorContext(ctx, "repository: updateWithRevert: rev-parse HEAD", "err", err)
		return "", fmt.Errorf("running git rev-parse HEAD: %w", err)
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
			return err
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
	if _, err := os.Stat(filepath.Clean(filepath.Join(subAbs, ".git"))); err != nil {
		if !os.IsNotExist(err) {
			logger.WarnContextWithErr(ctx, "stat submodule .git", err)
			return fmt.Errorf("stat submodule .git: %w", err)
		}
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
		logger.WarnContext(ctx, "  pull --rebase failed in "+subPath)
	}
	return nil
}

// LoadOverrides reads machine-specific override settings from the local environment.
func LoadOverrides() {
	overrides := filepath.Join(os.Getenv("HOME"), ".overrides.local")
	file, err := os.Open(filepath.Clean(overrides))
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
		if after, ok := strings.CutPrefix(line, "export "); ok {
			line = strings.TrimSpace(after)
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
	for line := range strings.SplitSeq(output, "\n") {
		if after, ok := strings.CutPrefix(line, "pulled:"); ok {
			parts := strings.SplitN(after, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			}
			return "", ""
		}
	}
	return "", ""
}
