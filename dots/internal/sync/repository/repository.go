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
	"time"

	"goodkind.io/.dotfiles/internal/clock"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/gitdir"
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
	layout, layoutErr := gitdir.Resolve(ctx, dotfiles, logger)
	reason := checkDotfilesGitHealth(ctx, dotfiles, layout, logger)
	if reason != "" {
		return "", fmt.Errorf("skip: %s", reason)
	}
	if layoutErr == nil {
		clearStaleGitLocks(ctx, layout, logger)
	}

	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "fetch", "origin", "--prune", "--no-recurse-submodules"); err != nil {
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

	if err := checkSubmoduleUpdates(ctx, dotfiles, "origin/main", "HEAD", logger); err != nil {
		return "", err
	}
	hasChanges, err := hasLocalChanges(ctx, dotfiles, logger)
	if err != nil {
		return "", err
	}
	if hasChanges {
		conflicting, err := hasConflictingChanges(ctx, dotfiles, logger)
		if err != nil {
			return "", err
		}
		if conflicting {
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
	if err := syncRecordedSubmodules(ctx, dotfiles, prePullHead, logger); err != nil {
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
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "stash", "pop", "--index"); err != nil {
		slog.ErrorContext(ctx, "repository: restoring stashed changes", "err", err)
		return fmt.Errorf("restoring stashed changes: %w", err)
	}
	return nil
}

func rollbackRepositoryUpdate(ctx context.Context, dotfiles string, head string, logger *telemetry.Logger) error {
	previous, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("reading pre-rollback parent HEAD: %w", err)
	}
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "reset", "--hard", head); err != nil {
		slog.WarnContext(ctx, "repository: resetting parent repository", "head", head, "err", err)
		return fmt.Errorf("resetting parent repository to %s: %w", head, err)
	}
	return syncRecordedSubmodules(ctx, dotfiles, strings.TrimSpace(previous), logger)
}

func firstError(current error, candidate error) error {
	if current != nil {
		return current
	}
	return candidate
}

func checkDotfilesGitHealth(ctx context.Context, dotfiles string, layout gitdir.Info, logger *telemetry.Logger) string {
	branch, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "symbolic-ref", "-q", "HEAD")
	if err != nil || strings.TrimSpace(branch) == "" {
		return "detached HEAD"
	}
	if gitCommandSucceeds(ctx, dotfiles, "rev-parse", "-q", "--verify", "MERGE_HEAD") {
		return "merge in progress"
	}
	// Rebase state is per-worktree, so it lives in the worktree git directory
	// rather than the shared one.
	gitDirPath := layout.GitDir
	if gitDirPath == "" {
		gitDirPath = filepath.Join(dotfiles, ".git")
	}
	if _, err := os.Stat(filepath.Clean(filepath.Join(gitDirPath, "rebase-merge"))); err == nil {
		return "rebase in progress"
	}
	if _, err := os.Stat(filepath.Clean(filepath.Join(gitDirPath, "rebase-apply"))); err == nil {
		return "rebase in progress"
	}
	if output, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "ls-files", "-u"); err == nil && strings.TrimSpace(output) != "" {
		return "unmerged paths"
	}
	return ""
}

// staleLockAge is how old a git lock file must be before it is treated as
// abandoned. No real git operation holds a lock for anything close to this, so
// a lock older than this belongs to a process that died without cleaning up.
const staleLockAge = time.Hour

// gitLockNames are the lock files git creates inside a git directory. Any one
// of them left behind by a crashed process blocks later operations.
var gitLockNames = []string{
	"index.lock",
	"HEAD.lock",
	"config.lock",
	"shallow.lock",
	"packed-refs.lock",
	filepath.Join("objects", "info", "commit-graph-chain.lock"),
}

// clearStaleGitLocks removes abandoned lock files from the shared git
// directory and from every submodule git directory beneath it.
//
// Submodule git directories live under <common dir>/modules, so a lock left
// there fails `git checkout` inside the submodule while the parent repository
// looks healthy. Locks are removed only once they are older than
// [staleLockAge], since removing a lock a live git still holds would corrupt
// that operation.
func clearStaleGitLocks(ctx context.Context, layout gitdir.Info, logger *telemetry.Logger) {
	removed := make([]string, 0)
	for _, gitDir := range gitDirsToScan(layout) {
		for _, lockName := range gitLockNames {
			lockPath := filepath.Clean(filepath.Join(gitDir, lockName))
			lockInfo, err := os.Stat(lockPath)
			if err != nil {
				continue
			}
			age := clock.Now().Sub(lockInfo.ModTime())
			if age < staleLockAge {
				continue
			}
			if err := os.Remove(lockPath); err != nil {
				slog.WarnContext(ctx, "repository: clearStaleGitLocks: removing stale lock", "path", lockPath, "err", err)
				continue
			}
			removed = append(removed, fmt.Sprintf("%s (age %s)", lockPath, age.Round(time.Minute)))
		}
	}
	if len(removed) > 0 && logger != nil {
		logger.InfoContext(ctx, "  cleared stale git locks: "+strings.Join(removed, ", "))
	}
}

// gitDirsToScan returns the shared git directory plus every submodule git
// directory under it.
//
// Git nests a submodule's git directory by the submodule's path, so `lib/zinit`
// lands at <modules>/lib/zinit. The depth therefore varies with the submodule
// path and the tree must be walked rather than listed.
func gitDirsToScan(layout gitdir.Info) []string {
	gitDirs := make([]string, 0, 1)
	if layout.CommonDir != "" {
		gitDirs = append(gitDirs, layout.CommonDir)
	}
	modules := layout.ModulesDir()
	if modules == "" {
		return gitDirs
	}
	return append(gitDirs, subdirectories(filepath.Clean(modules))...)
}

// subdirectories returns every directory beneath root, at any depth. An
// unreadable directory contributes nothing rather than failing the caller,
// since a lock sweep is best effort.
func subdirectories(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	found := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		child := filepath.Join(root, entry.Name())
		found = append(found, child)
		found = append(found, subdirectories(child)...)
	}
	return found
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
	localChanged, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "diff", "--name-only", "HEAD")
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
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "merge", "--ff-only", "--no-overwrite-ignore", "origin/main"); err != nil {
		if hadChanges {
			if restoreErr := restoreStashedChanges(ctx, dotfiles, logger); restoreErr != nil {
				return prePullHead, fmt.Errorf("fast-forward merge failed: %w; restoring local changes: %w", err, restoreErr)
			}
		}
		return prePullHead, fmt.Errorf("fast-forward merge failed: %w", err)
	}
	return prePullHead, nil
}

func syncDotfilesSubmodules(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	return syncRecordedSubmodules(ctx, dotfiles, "HEAD", logger)
}

func syncRecordedSubmodules(ctx context.Context, dotfiles string, previous string, logger *telemetry.Logger) error {
	if err := checkSubmoduleUpdates(ctx, dotfiles, "HEAD", previous, logger); err != nil {
		return err
	}
	subs, err := declaredSubmodulePaths(ctx, dotfiles, logger)
	if err != nil {
		return err
	}
	for _, sub := range subs {
		previousSubmodule, err := recordedSubmoduleCommit(ctx, dotfiles, previous, sub, logger)
		if err != nil {
			return err
		}
		if err := checkoutRecordedSubmodule(ctx, dotfiles, sub, logger); err != nil {
			return err
		}
		if previousSubmodule == "" {
			previousSubmodule = "HEAD"
		}
		if err := syncRecordedSubmodules(ctx, filepath.Join(dotfiles, sub), previousSubmodule, logger); err != nil {
			return err
		}
	}
	return nil
}

func checkoutRecordedSubmodule(ctx context.Context, dotfiles string, sub string, logger *telemetry.Logger) error {
	subAbs := filepath.Join(dotfiles, sub)
	if _, err := os.Stat(filepath.Clean(filepath.Join(subAbs, ".git"))); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.WarnContext(ctx, "repository: checking submodule git directory", "submodule", subAbs, "err", err)
			return fmt.Errorf("checking submodule %s: %w", sub, err)
		}
		if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "submodule", "update", "--init", "--checkout", "--", sub); err != nil {
			slog.WarnContext(ctx, "repository: initializing recorded submodule", "submodule", subAbs, "err", err)
			return fmt.Errorf("initializing recorded submodule %s: %w", sub, err)
		}
		return nil
	}
	recorded, err := recordedSubmoduleCommit(ctx, dotfiles, "HEAD", sub, logger)
	if err != nil {
		return err
	}
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "checkout", "--detach", "--no-overwrite-ignore", recorded); err != nil {
		slog.WarnContext(ctx, "repository: checking out recorded submodule", "submodule", subAbs, "commit", recorded, "err", err)
		return fmt.Errorf("checking out recorded submodule %s: %w", sub, err)
	}
	return nil
}

func checkSubmoduleUpdates(ctx context.Context, dotfiles string, target string, previous string, logger *telemetry.Logger) error {
	subs, err := declaredSubmodulePaths(ctx, dotfiles, logger)
	if err != nil {
		return err
	}
	for _, sub := range subs {
		if err := checkOneSubmoduleUpdate(ctx, dotfiles, sub, target, previous, logger); err != nil {
			return err
		}
	}
	return nil
}

func checkOneSubmoduleUpdate(ctx context.Context, dotfiles string, sub string, target string, previous string, logger *telemetry.Logger) error {
	staged, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "diff", "--cached", "--name-only", "--", sub)
	if err != nil {
		slog.WarnContext(ctx, "repository: checking staged submodule", "repository", dotfiles, "submodule", sub, "err", err)
		return fmt.Errorf("checking staged submodule %s: %w", sub, err)
	}
	if strings.TrimSpace(staged) != "" {
		return fmt.Errorf("submodule %s has a staged gitlink change that blocks synchronization in %s", sub, dotfiles)
	}
	subAbs := filepath.Join(dotfiles, sub)
	if _, err := os.Stat(filepath.Clean(filepath.Join(subAbs, ".git"))); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		slog.WarnContext(ctx, "repository: checking submodule git directory", "submodule", subAbs, "err", err)
		return fmt.Errorf("checking submodule %s: %w", sub, err)
	}
	recorded, err := recordedSubmoduleCommit(ctx, dotfiles, target, sub, logger)
	if err != nil {
		return err
	}
	if recorded == "" {
		return nil
	}
	if err := fetchRecordedSubmoduleCommit(ctx, subAbs, recorded, logger); err != nil {
		return err
	}
	current, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "rev-parse", "HEAD")
	if err != nil {
		slog.WarnContext(ctx, "repository: reading submodule HEAD", "submodule", subAbs, "err", err)
		return fmt.Errorf("reading submodule %s HEAD: %w", sub, err)
	}
	current = strings.TrimSpace(current)
	if current == recorded {
		if err := checkSubmoduleUpdates(ctx, subAbs, recorded, current, logger); err != nil {
			return err
		}
		return nil
	}
	old, err := recordedSubmoduleCommit(ctx, dotfiles, previous, sub, logger)
	if err != nil {
		return err
	}
	if current != old && !gitCommandSucceeds(ctx, subAbs, "merge-base", "--is-ancestor", current, recorded) {
		return fmt.Errorf("submodule %s has local commit %s outside recorded update %s in %s", sub, current, recorded, subAbs)
	}
	status, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", subAbs, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		slog.WarnContext(ctx, "repository: checking submodule local files", "submodule", subAbs, "err", err)
		return fmt.Errorf("checking submodule %s local files: %w", sub, err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("submodule %s has local file changes that block recorded update %s in %s", sub, recorded, subAbs)
	}
	if err := checkIgnoredFileCollisions(ctx, subAbs, recorded, logger); err != nil {
		return err
	}
	if err := checkSubmoduleUpdates(ctx, subAbs, recorded, current, logger); err != nil {
		return err
	}
	return nil
}

func checkIgnoredFileCollisions(ctx context.Context, repository string, target string, logger *telemetry.Logger) error {
	ignored, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", repository, "ls-files", "--others", "--ignored", "--exclude-standard", "-z")
	if err != nil {
		slog.WarnContext(ctx, "repository: checking ignored files", "repository", repository, "err", err)
		return fmt.Errorf("checking ignored files in %s: %w", repository, err)
	}
	if ignored == "" {
		return nil
	}
	tracked, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", repository, "ls-tree", "-r", "-z", target)
	if err != nil {
		slog.WarnContext(ctx, "repository: reading recorded tree", "repository", repository, "revision", target, "err", err)
		return fmt.Errorf("reading recorded tree in %s: %w", repository, err)
	}
	for ignoredPath := range strings.SplitSeq(ignored, "\x00") {
		if ignoredPath == "" {
			continue
		}
		for record := range strings.SplitSeq(tracked, "\x00") {
			metadata, trackedPath, valid := strings.Cut(record, "\t")
			if !valid || strings.HasPrefix(metadata, "160000 ") {
				continue
			}
			if ignoredPath == trackedPath || strings.HasPrefix(ignoredPath, trackedPath+"/") || strings.HasPrefix(trackedPath, ignoredPath+"/") {
				return fmt.Errorf("ignored file %s conflicts with recorded path %s in %s", ignoredPath, trackedPath, repository)
			}
		}
	}
	return nil
}

func fetchRecordedSubmoduleCommit(ctx context.Context, submodule string, commit string, logger *telemetry.Logger) error {
	if gitCommandSucceeds(ctx, submodule, "cat-file", "-e", commit+"^{commit}") {
		return nil
	}
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "git", "-C", submodule, "fetch", "--no-recurse-submodules", "origin", commit); err != nil {
		slog.WarnContext(ctx, "repository: fetching recorded submodule commit", "submodule", submodule, "commit", commit, "err", err)
		return fmt.Errorf("fetching recorded commit %s in %s: %w", commit, submodule, err)
	}
	return nil
}

func recordedSubmoduleCommit(ctx context.Context, dotfiles string, revision string, sub string, logger *telemetry.Logger) (string, error) {
	output, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", dotfiles, "ls-tree", revision, "--", sub)
	if err != nil {
		slog.WarnContext(ctx, "repository: reading recorded submodule", "repository", dotfiles, "submodule", sub, "revision", revision, "err", err)
		return "", fmt.Errorf("reading recorded submodule %s: %w", sub, err)
	}
	if strings.TrimSpace(output) == "" {
		return "", nil
	}
	fields := strings.Fields(output)
	if len(fields) < 3 || fields[0] != "160000" {
		return "", fmt.Errorf("submodule %s has no recorded gitlink in %s", sub, revision)
	}
	return fields[2], nil
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
