package githook

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

var errUnknownCommitWorktree = errors.New("cannot identify the worktree for the commit")

func runPreCommit(ctx context.Context, args []string, stdin io.Reader, stderr io.Writer) error {
	if slices.Equal(args, []string{"--check-installation"}) {
		return nil
	}

	repoRoot, err := gitOutput(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		slog.WarnContext(ctx, "cannot locate the home directory for pre-commit", "err", err)

		return fmt.Errorf("locate the home directory for pre-commit: %w", err)
	}

	dotfilesRoot := os.Getenv("DOTDOTFILES")
	if dotfilesRoot == "" {
		dotfilesRoot = filepath.Join(home, ".dotfiles")
	}

	if err := applyCommitPolicy(ctx, repoRoot, home, stderr); err != nil {
		return err
	}

	if err := checkCommitIdentity(ctx, repoRoot, dotfilesRoot, home); err != nil {
		return err
	}

	if err := scanStagedSecrets(ctx, repoRoot, dotfilesRoot, args, stderr); err != nil {
		return err
	}

	return runChainedPreCommit(ctx, home, args, stdin, stderr)
}

func applyCommitPolicy(ctx context.Context, repoRoot, home string, stderr io.Writer) error {
	if !harnessActive() {
		return nil
	}

	enforced, err := protectedBranchPath(ctx, repoRoot, home)
	if err != nil || !enforced {
		return err
	}

	layout := inspectRepo(ctx)
	if !layout.ready {
		return errUnknownCommitWorktree
	}

	if !layout.primary {
		branch, attached := currentBranch(ctx)
		if !attached {
			return nil
		}

		names, err := defaultBranchNames(ctx)
		if err != nil {
			return err
		}

		if !slices.Contains(names, branch) {
			return nil
		}
	}

	protected, err := defaultBranchRequiresPullRequest(ctx)
	if err != nil {
		fmt.Fprintf(
			stderr,
			"warning: allowing the commit because GitHub protection lookup failed: %v\n",
			err,
		)

		return nil
	}

	if protected {
		return refuse(
			"error: commit blocked on protected branch or primary checkout for this agent.",
			hintPrimaryWrite,
		)
	}

	return nil
}

func protectedBranchPath(ctx context.Context, repoRoot, home string) (bool, error) {
	command := exec.CommandContext(
		ctx,
		"git",
		"config",
		"--null",
		"--get-all",
		"goodkind.protectedBranchEnforcePath",
	)
	output, err := command.Output()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 &&
		len(exitErr.Stderr) == 0 {
		return false, nil
	}

	if err != nil {
		slog.WarnContext(ctx, "cannot read protected branch enforcement paths", "err", err)

		return false, fmt.Errorf("read goodkind.protectedBranchEnforcePath: %w", err)
	}

	repoRoot = canonicalGitPath(repoRoot)

	for path := range strings.SplitSeq(string(output), "\x00") {
		if path == "" {
			continue
		}

		if suffix, ok := strings.CutPrefix(path, "~"); ok {
			path = home + suffix
		}

		if !filepath.IsAbs(path) {
			continue
		}

		relative, err := filepath.Rel(canonicalGitPath(path), repoRoot)
		if err == nil && relative != ".." &&
			!strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true, nil
		}
	}

	return false, nil
}

func runChainedPreCommit(
	ctx context.Context,
	home string,
	args []string,
	stdin io.Reader,
	stderr io.Writer,
) error {
	const hook = "pre-commit"
	activeHook, err := gitOutput(
		ctx,
		"rev-parse",
		"--path-format=absolute",
		"--git-path",
		"hooks/"+hook,
	)
	if err != nil {
		return err
	}

	gitDir, err := gitOutput(ctx, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return err
	}

	commonDir, err := gitOutput(ctx, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}

	self, err := os.Executable()
	if err != nil {
		slog.WarnContext(ctx, "cannot locate the running hook binary", "err", err)

		return fmt.Errorf("locate the compiled hook: %w", err)
	}

	var seen []os.FileInfo

	for _, path := range []string{self, activeHook} {
		if info, err := os.Stat(path); err == nil {
			seen = append(seen, info)
		}
	}

	for _, path := range []string{
		filepath.Join(gitDir, "hooks", hook),
		filepath.Join(commonDir, "hooks", hook),
		filepath.Join(home, ".git-hooks", hook),
	} {
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}

		if err != nil {
			slog.WarnContext(ctx, "cannot inspect chained pre-commit hook", "err", err)

			return fmt.Errorf("inspect chained hook %q: %w", path, err)
		}

		if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			continue
		}

		if slices.ContainsFunc(
			seen,
			func(previous os.FileInfo) bool { return os.SameFile(previous, info) },
		) {
			continue
		}

		seen = append(seen, info)
		command := exec.CommandContext(ctx, path, args...)
		command.Stdin = stdin
		command.Stdout = stderr
		command.Stderr = stderr

		if err := command.Run(); err != nil {
			slog.WarnContext(ctx, "chained pre-commit hook failed", "err", err)

			return fmt.Errorf("chained pre-commit hook %q failed: %w", path, err)
		}
	}

	return nil
}
