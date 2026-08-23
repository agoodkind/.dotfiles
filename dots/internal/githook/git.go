// Package githook implements agent-only git hook policy invoked by `dots git-hook`.
package githook

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"goodkind.io/.dotfiles/internal/cmdexec"
)

const (
	headsPrefix = "refs/heads/"
	mainBranch  = "main"
	stashRef    = "refs/stash"
	stashPrefix = "refs/stash/"
	headRef     = "HEAD"
	refPrefix   = "ref:"
)

type repoLayout struct {
	ready   bool
	primary bool
}

func inspectRepo(ctx context.Context) repoLayout {
	gitDir, err := cmdexec.OutputTrimmed(ctx, "git", "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return repoLayout{ready: false, primary: false}
	}
	commonDir, err := cmdexec.OutputTrimmed(ctx, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return repoLayout{ready: false, primary: false}
	}
	gitDir = canonicalGitPath(gitDir)
	commonDir = canonicalGitPath(commonDir)
	return repoLayout{ready: true, primary: gitDir == commonDir}
}

func canonicalGitPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolved
}

func gitOutput(ctx context.Context, args ...string) (string, error) {
	output, err := cmdexec.OutputTrimmed(ctx, "git", args...)
	if err != nil {
		slog.WarnContext(ctx, "git command failed", "args", args, "err", err)
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func currentBranch(ctx context.Context) (string, bool) {
	ref, err := gitOutput(ctx, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		return "", false
	}
	branch, ok := strings.CutPrefix(ref, headsPrefix)
	if !ok {
		return "", false
	}
	return branch, true
}

func defaultBranchNames(ctx context.Context) ([]string, error) {
	names := []string{mainBranch}
	remotes, err := gitOutput(ctx, "remote")
	if err != nil {
		return nil, err
	}
	for remote := range strings.FieldsSeq(remotes) {
		head, headErr := gitOutput(ctx, "symbolic-ref", "--quiet", "refs/remotes/"+remote+"/HEAD")
		if headErr != nil {
			continue
		}
		prefix := "refs/remotes/" + remote + "/"
		if branch, ok := strings.CutPrefix(head, prefix); ok {
			names = append(names, branch)
		}
	}
	configured, configErr := gitOutput(ctx, "config", "--get", "init.defaultBranch")
	if configErr == nil && configured != "" {
		names = append(names, configured)
	}
	return names, nil
}

func isDefaultBranch(ctx context.Context, name string) (bool, error) {
	names, err := defaultBranchNames(ctx)
	if err != nil {
		return false, err
	}
	return slices.Contains(names, name), nil
}

func currentBranchIsDefault(ctx context.Context) (bool, error) {
	name, ok := currentBranch(ctx)
	if !ok {
		return false, nil
	}
	return isDefaultBranch(ctx, name)
}

func branchFromSymbolicValue(value string) (string, bool) {
	target, ok := strings.CutPrefix(value, refPrefix)
	if !ok {
		return "", false
	}
	branch, ok := strings.CutPrefix(target, headsPrefix)
	if !ok {
		return "", false
	}
	return branch, true
}

func isStashRef(name string) bool {
	if name == stashRef {
		return true
	}
	return strings.HasPrefix(name, stashPrefix)
}

func isZeroOID(value string) bool {
	if value == "" || strings.HasPrefix(value, refPrefix) {
		return false
	}
	for _, char := range value {
		if char != '0' {
			return false
		}
	}
	return len(value) >= 40
}

func worktreeHeadSetup(ctx context.Context) bool {
	commonDir, err := gitOutput(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return false
	}
	worktreesDir := filepath.Join(canonicalGitPath(commonDir), "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(worktreesDir, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "gitdir")); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "HEAD")); os.IsNotExist(err) {
			return true
		}
	}
	return false
}
