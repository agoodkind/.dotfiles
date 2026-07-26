// Package repository implements git repository sync operations.
package repository

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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
		if err := syncDotfilesSubmodules(ctx, dotfiles, logger); err != nil {
			slog.WarnContext(ctx, "repository: syncing submodules", "err", err)
			return "", fmt.Errorf("syncing submodules: %w", err)
		}
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
	postPullHead, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "HEAD")
	if err != nil {
		if hasChanges {
			_ = restoreStashedChanges(ctx, dotfiles, logger)
		}
		return "", fmt.Errorf("running git rev-parse HEAD: %w", err)
	}
	pulled := strings.TrimSpace(prePullHead) != strings.TrimSpace(postPullHead)
	if pulled {
		if err := syncPulledSubmodules(ctx, dotfiles, strings.TrimSpace(prePullHead), hasChanges, logger); err != nil {
			return "", err
		}
	}
	if hasChanges {
		if err := restoreStashedChanges(ctx, dotfiles, logger); err != nil {
			return "", err
		}
	}
	if pulled {
		return "pulled:" + strings.TrimSpace(prePullHead) + ":" + strings.TrimSpace(postPullHead), nil
	}
	return "", nil
}

func syncPulledSubmodules(ctx context.Context, dotfiles string, prePullHead string, hadChanges bool, logger *telemetry.Logger) error {
	if err := syncDotfilesSubmodules(ctx, dotfiles, logger); err != nil {
		slog.WarnContext(ctx, "repository: syncing submodules after pull", "err", err)
		rollbackErr := rollbackRepositoryUpdate(ctx, dotfiles, prePullHead, logger)
		if hadChanges {
			rollbackErr = firstError(rollbackErr, restoreStashedChanges(ctx, dotfiles, logger))
		}
		if rollbackErr != nil {
			return fmt.Errorf("syncing submodules after pull: %w; rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("syncing submodules after pull: %w", err)
	}
	return nil
}

func restoreStashedChanges(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "stash", "pop"); err != nil {
		slog.ErrorContext(ctx, "repository: restoring stashed changes", "err", err)
		return fmt.Errorf("restoring stashed changes: %w", err)
	}
	return nil
}

func rollbackRepositoryUpdate(ctx context.Context, dotfiles string, head string, logger *telemetry.Logger) error {
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "reset", "--hard", head); err != nil {
		slog.WarnContext(ctx, "repository: resetting parent repository", "head", head, "err", err)
		return fmt.Errorf("resetting parent repository to %s: %w", head, err)
	}
	return restoreSubmoduleWorktrees(ctx, dotfiles, logger)
}

func firstError(current error, candidate error) error {
	if current != nil {
		return current
	}
	return candidate
}

func checkDotfilesGitHealth(ctx context.Context, dotfiles string, logger *telemetry.Logger) string {
	branch, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "symbolic-ref", "-q", "HEAD")
	if err != nil || strings.TrimSpace(branch) == "" {
		return "detached HEAD"
	}
	if gitCommandSucceeds(ctx, dotfiles, "rev-parse", "-q", "--verify", "MERGE_HEAD") {
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
	if isMergeBaseAncestor(ctx, dotfiles, current, latest) {
		return "behind"
	}
	if isMergeBaseAncestor(ctx, dotfiles, latest, current) {
		return "up-to-date"
	}
	return "diverged"
}

func isMergeBaseAncestor(ctx context.Context, dotfiles string, ancestor string, descendant string) bool {
	return gitCommandSucceeds(ctx, dotfiles, "merge-base", "--is-ancestor", ancestor, descendant)
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
	subs, err := declaredSubmodulePaths(ctx, dotfiles, logger)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "submodule", "update", "--init", "--recursive"); err != nil {
		slog.WarnContext(ctx, "repository: initializing submodules", "err", err)
		return fmt.Errorf("initializing submodules: %w", err)
	}
	for _, sub := range subs {
		if err := syncOneSubmodule(ctx, dotfiles, sub, logger); err != nil {
			if restoreErr := restoreSubmoduleWorktrees(ctx, dotfiles, logger); restoreErr != nil {
				return fmt.Errorf("%w; restoring submodules failed: %w", err, restoreErr)
			}
			return err
		}
	}

	cachedDiff, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "diff", "--cached", "--name-only", "--ignore-submodules")
	if err != nil {
		return fmt.Errorf("checking staged parent changes: %w", err)
	}
	if strings.TrimSpace(cachedDiff) != "" {
		return nil
	}

	pointerDirty := false
	for _, sub := range subs {
		if !gitCommandSucceeds(ctx, dotfiles, "diff", "--quiet", "--", sub) {
			if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "add", "--", sub); err != nil {
				_ = unstageSubmodulePointers(ctx, dotfiles, subs, logger)
				return fmt.Errorf("staging submodule pointer %s: %w", sub, err)
			}
			if !gitCommandSucceeds(ctx, dotfiles, "diff", "--cached", "--quiet", "--", sub) {
				pointerDirty = true
			}
		}
	}
	if pointerDirty {
		commitArgs := []string{"-C", dotfiles, "commit", "-m", "Update submodule pointers", "--"}
		commitArgs = append(commitArgs, subs...)
		if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", commitArgs...); err != nil {
			unstageErr := unstageSubmodulePointers(ctx, dotfiles, subs, logger)
			if unstageErr != nil {
				return fmt.Errorf("committing submodule pointers: %w; unstaging failed: %w", err, unstageErr)
			}
			return fmt.Errorf("committing submodule pointers: %w", err)
		}
	}
	return nil
}

func restoreSubmoduleWorktrees(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "submodule", "update", "--init", "--recursive"); err != nil {
		slog.WarnContext(ctx, "repository: restoring submodule worktrees", "err", err)
		return fmt.Errorf("restoring submodule worktrees: %w", err)
	}
	return nil
}

func unstageSubmodulePointers(ctx context.Context, dotfiles string, subs []string, logger *telemetry.Logger) error {
	args := []string{"-C", dotfiles, "reset", "--"}
	args = append(args, subs...)
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", args...); err != nil {
		slog.WarnContext(ctx, "repository: unstaging submodule pointers", "err", err)
		return fmt.Errorf("unstaging submodule pointers: %w", err)
	}
	return nil
}

func syncOneSubmodule(ctx context.Context, dotfiles string, subPath string, logger *telemetry.Logger) error {
	subAbs := filepath.Join(dotfiles, subPath)
	if _, err := os.Stat(filepath.Clean(filepath.Join(subAbs, ".git"))); err != nil {
		if !os.IsNotExist(err) {
			logger.WarnContextWithErr(ctx, "stat submodule .git", err)
			slog.WarnContext(ctx, "repository: stat submodule .git", "submodule", subPath, "err", err)
			return fmt.Errorf("stat submodule .git: %w", err)
		}
		return nil
	}
	branch, err := declaredSubmoduleBranch(ctx, dotfiles, subPath, logger)
	if err != nil {
		return err
	}
	if branch == "" {
		switch {
		case gitCommandSucceeds(ctx, subAbs, "rev-parse", "-q", "--verify", "origin/main"):
			branch = "main"
		case gitCommandSucceeds(ctx, subAbs, "rev-parse", "-q", "--verify", "origin/master"):
			branch = "master"
		default:
			branch = "main"
		}
	}
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "fetch"); err != nil {
		logger.WarnContextWithErr(ctx, "fetch submodule "+subPath, err)
		return fmt.Errorf("fetching submodule %s: %w", subPath, err)
	}
	current, err := submoduleBranchIsCurrent(ctx, subAbs, branch, logger)
	if err != nil {
		return fmt.Errorf("checking submodule %s branch %s: %w", subPath, branch, err)
	}
	if current {
		return nil
	}
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "checkout", branch); err != nil {
		logger.WarnContextWithErr(ctx, "checkout submodule "+subPath, err)
		return fmt.Errorf("checking out submodule %s branch %s: %w", subPath, branch, err)
	}
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "pull", "--rebase", "origin", branch); err != nil {
		_ = cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "rebase", "--abort")
		logger.WarnContextWithErr(ctx, "pull --rebase failed in "+subPath, err)
		return fmt.Errorf("pulling submodule %s branch %s: %w", subPath, branch, err)
	}
	return nil
}

func submoduleBranchIsCurrent(
	ctx context.Context,
	submodule string,
	branch string,
	logger *telemetry.Logger,
) (bool, error) {
	currentBranch, err := cmdexec.OutputWithLoggerAndEnv(
		ctx,
		logger,
		nil,
		"git",
		"-C",
		submodule,
		"branch",
		"--show-current",
	)
	if err != nil {
		slog.WarnContext(ctx, "repository: reading current submodule branch", "submodule", submodule, "err", err)
		return false, fmt.Errorf("reading current branch: %w", err)
	}
	if strings.TrimSpace(currentBranch) != branch {
		return false, nil
	}
	return isMergeBaseAncestor(ctx, submodule, "origin/"+branch, "HEAD"), nil
}

type declaredSubmodule struct {
	Path   string
	Branch string
}

type submoduleConfigKey string

const (
	submoduleConfigPath   submoduleConfigKey = "path"
	submoduleConfigBranch submoduleConfigKey = "branch"
)

func declaredSubmodulePaths(
	ctx context.Context,
	dotfiles string,
	logger *telemetry.Logger,
) ([]string, error) {
	submodules, err := declaredSubmodules(ctx, dotfiles, logger)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(submodules))
	for _, submodule := range submodules {
		paths = append(paths, submodule.Path)
	}
	return paths, nil
}

func declaredSubmoduleBranch(
	ctx context.Context,
	dotfiles string,
	subPath string,
	logger *telemetry.Logger,
) (string, error) {
	submodules, err := declaredSubmodules(ctx, dotfiles, logger)
	if err != nil {
		return "", err
	}
	for _, submodule := range submodules {
		if submodule.Path == subPath {
			return submodule.Branch, nil
		}
	}
	return "", nil
}

func declaredSubmodules(
	ctx context.Context,
	dotfiles string,
	logger *telemetry.Logger,
) ([]declaredSubmodule, error) {
	gitmodulesPath := filepath.Join(dotfiles, ".gitmodules")
	if _, err := os.Stat(filepath.Clean(gitmodulesPath)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		slog.WarnContext(ctx, "repository: checking .gitmodules", "path", gitmodulesPath, "err", err)
		return nil, fmt.Errorf("checking .gitmodules: %w", err)
	}
	output, err := cmdexec.OutputWithLoggerAndEnv(
		ctx,
		logger,
		nil,
		"git",
		"-C",
		dotfiles,
		"config",
		"--null",
		"--file",
		gitmodulesPath,
		"--get-regexp",
		`^submodule\..*\.(path|branch)$`,
	)
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return nil, nil
		}
		slog.WarnContext(ctx, "repository: reading .gitmodules", "path", gitmodulesPath, "err", err)
		return nil, fmt.Errorf("reading .gitmodules: %w", err)
	}

	orderedSections := make([]string, 0)
	bySection := make(map[string]declaredSubmodule)
	for record := range strings.SplitSeq(output, "\x00") {
		configName, value, ok := strings.Cut(record, "\n")
		if !ok {
			continue
		}
		section, key, ok := parseSubmoduleConfigName(configName)
		if !ok {
			continue
		}
		current, exists := bySection[section]
		if !exists {
			orderedSections = append(orderedSections, section)
			current = declaredSubmodule{Path: "", Branch: ""}
		}
		switch key {
		case submoduleConfigPath:
			current.Path = value
		case submoduleConfigBranch:
			current.Branch = value
		}
		bySection[section] = current
	}

	submodules := make([]declaredSubmodule, 0, len(orderedSections))
	for _, section := range orderedSections {
		current := bySection[section]
		if current.Path == "" {
			continue
		}
		validated, validationErr := validateSubmodulePath(dotfiles, current.Path)
		if validationErr != nil {
			return nil, validationErr
		}
		current.Path = validated
		submodules = append(submodules, current)
	}
	return submodules, nil
}

func parseSubmoduleConfigName(name string) (string, submoduleConfigKey, bool) {
	for _, key := range []submoduleConfigKey{submoduleConfigPath, submoduleConfigBranch} {
		suffix := "." + string(key)
		section, ok := strings.CutSuffix(name, suffix)
		if ok && strings.HasPrefix(section, "submodule.") {
			return section, key, true
		}
	}
	return "", "", false
}

func validateSubmodulePath(dotfiles string, subPath string) (string, error) {
	cleanRoot := filepath.Clean(dotfiles)
	cleanPath := filepath.Clean(subPath)
	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("submodule path %q escapes repository root %s", subPath, cleanRoot)
	}
	if cleanPath == "." {
		return "", fmt.Errorf("submodule path %q must be below repository root %s", subPath, cleanRoot)
	}
	joined := filepath.Clean(filepath.Join(cleanRoot, cleanPath))
	if !strings.HasPrefix(joined, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("submodule path %q escapes repository root %s", subPath, cleanRoot)
	}
	return cleanPath, nil
}

func gitCommandSucceeds(ctx context.Context, worktree string, args ...string) bool {
	commandArgs := append([]string{"-C", worktree}, args...)
	_, err := cmdexec.OutputWithLoggerAndEnv(ctx, nil, nil, "git", commandArgs...)
	return err == nil
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
