// Package gitdir resolves the git layout of a working tree so callers can tell
// a canonical checkout from a linked worktree, and can find the shared git
// directory instead of assuming that <root>/.git is one.
package gitdir

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// Info describes the git layout of a working tree.
type Info struct {
	// Root is the working tree the caller asked about.
	Root string
	// GitDir is the absolute per-worktree git directory. Per-worktree state
	// such as an in-progress rebase lives here.
	GitDir string
	// CommonDir is the absolute git directory shared by every worktree.
	// Hooks, the object store, and submodule git directories live here.
	CommonDir string
	// IsWorktree reports whether Root is a linked worktree rather than the
	// canonical checkout. It is true exactly when GitDir differs from
	// CommonDir.
	IsWorktree bool
}

// MainWorktree returns the canonical working tree, which is the directory
// holding CommonDir. It returns an empty string when CommonDir is unset.
func (i Info) MainWorktree() string {
	if i.CommonDir == "" {
		return ""
	}
	return filepath.Dir(i.CommonDir)
}

// ModulesDir returns the directory holding submodule git directories. Git
// keeps these under the common directory, so every worktree shares them.
func (i Info) ModulesDir() string {
	if i.CommonDir == "" {
		return ""
	}
	return filepath.Join(i.CommonDir, "modules")
}

// Resolve reports the git layout of root. It returns an error when root is not
// inside a git working tree. Callers should read that error as "not a git
// checkout" and carry on, since an archive install has no git at all.
func Resolve(ctx context.Context, root string, logger *telemetry.Logger) (Info, error) {
	gitDir, err := revParsePath(ctx, root, "--git-dir", logger)
	if err != nil {
		return Info{Root: root, GitDir: "", CommonDir: "", IsWorktree: false}, err
	}
	commonDir, err := revParsePath(ctx, root, "--git-common-dir", logger)
	if err != nil {
		return Info{Root: root, GitDir: "", CommonDir: "", IsWorktree: false}, err
	}
	return Info{
		Root:       root,
		GitDir:     gitDir,
		CommonDir:  commonDir,
		IsWorktree: gitDir != commonDir,
	}, nil
}

// LinkedWorktree reports whether root is a linked worktree, along with the
// layout that decided it. A root that is not a git checkout is not a linked
// worktree, so the answer is false and the returned Info is zero.
func LinkedWorktree(ctx context.Context, root string, logger *telemetry.Logger) (Info, bool) {
	layout, err := Resolve(ctx, root, logger)
	if err != nil {
		return Info{Root: root, GitDir: "", CommonDir: "", IsWorktree: false}, false
	}
	return layout, layout.IsWorktree
}

// revParsePath runs one git rev-parse flag and returns its answer as an
// absolute path. Git answers relative to root when it is run from the working
// tree root and absolute when it is run from a linked worktree, so the value
// must be joined against root before two answers can be compared.
func revParsePath(ctx context.Context, root string, flag string, logger *telemetry.Logger) (string, error) {
	output, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, "git", "-C", root, "rev-parse", flag)
	if err != nil {
		slog.WarnContext(ctx, "gitdir: revParsePath: git rev-parse failed", "flag", flag, "root", root, "err", err)
		return "", fmt.Errorf("running git rev-parse %s: %w", flag, err)
	}
	path := strings.TrimSpace(output)
	if path == "" {
		return "", fmt.Errorf("git rev-parse %s returned no path", flag)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return canonicalPath(path), nil
}

// canonicalPath resolves symlinks so two spellings of one directory compare
// equal. On macOS a home directory is reachable through /System/Volumes/Data,
// so comparing git's answer against a caller-supplied path can otherwise
// differ for paths naming the same inode.
func canonicalPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolved
}
