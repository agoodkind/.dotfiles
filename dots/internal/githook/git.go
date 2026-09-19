// Package githook implements agent-only git hook policy invoked by `dots git-hook`.
package githook

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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
	gitlinkMode = "160000"
	mergedStage = "0"
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

// pinnedSubmoduleCheckout reports whether detaching HEAD at commit checks out
// the gitlink that the superproject index records for this submodule. `git
// submodule update` clones a submodule onto its default branch and then
// detaches HEAD at that gitlink.
func pinnedSubmoduleCheckout(ctx context.Context, commit string) bool {
	superproject, err := gitOutput(ctx, "rev-parse", "--show-superproject-working-tree")
	if err != nil || superproject == "" {
		return false
	}
	toplevel, err := gitOutput(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	path, err := filepath.Rel(canonicalGitPath(superproject), canonicalGitPath(toplevel))
	if err != nil {
		return false
	}
	gitlink, ok := superprojectGitlink(ctx, superproject, filepath.ToSlash(path))
	return ok && gitlink == commit
}

// superprojectGitlink reads the gitlink recorded at path in the superproject
// index. Git runs hooks with variables such as GIT_DIR that name the
// submodule, so the command drops every local repository variable to read the
// superproject instead.
func superprojectGitlink(ctx context.Context, superproject string, path string) (string, bool) {
	localVars, err := gitOutput(ctx, "rev-parse", "--local-env-vars")
	if err != nil {
		return "", false
	}
	cleared := strings.Fields(localVars)
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !slices.Contains(cleared, name) {
			env = append(env, entry)
		}
	}
	// -z keeps paths unquoted, since ls-files otherwise C-quotes non-ASCII
	// characters, tabs, newlines, and quotes.
	command := exec.CommandContext(ctx, "git", "-C", superproject, "--literal-pathspecs", "ls-files", "-z", "--stage", "--", path)
	command.Env = env
	output, err := command.Output()
	if err != nil {
		slog.WarnContext(ctx, "reading superproject gitlink failed", "superproject", superproject, "path", path, "err", err)
		return "", false
	}
	records := strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
	if len(records) != 1 || records[0] == "" {
		return "", false
	}
	entry := records[0]
	metadata, entryPath, ok := strings.Cut(entry, "\t")
	if !ok || entryPath != path {
		return "", false
	}
	fields := strings.Fields(metadata)
	if len(fields) != 3 || fields[0] != gitlinkMode || fields[2] != mergedStage {
		return "", false
	}
	return fields[1], true
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
