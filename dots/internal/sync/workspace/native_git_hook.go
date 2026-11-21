package workspace

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"goodkind.io/.dotfiles/internal/telemetry"
)

const (
	preCommitEvent       = "pre-commit"
	nativePreCommitHook  = "dotfiles-pre-commit"
	nativePreCommitKey   = "hook." + nativePreCommitHook
	nativePreCommitCmd   = nativePreCommitKey + ".command"
	nativePreCommitEvent = nativePreCommitKey + ".event"
)

var (
	errGitHookVersion = errors.New(
		"the dotfiles hooks require Git 2.54 or newer; update Git and run sync again",
	)
	errDotsCommand        = errors.New("run install.sh to build the dots hook binary")
	errHookNotSelected    = errors.New("inspect the effective Git hook settings")
	errHookCommandChanged = errors.New("inspect the effective Git hook command")
)

func installNativeGitHooks(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	dotfiles, err := filepath.Abs(dotfiles)
	if err != nil {
		slog.WarnContext(ctx, "cannot resolve the dotfiles directory", "err", err)

		return fmt.Errorf("resolve the dotfiles directory: %w", err)
	}

	if err := requireNativeHookGit(ctx); err != nil {
		return err
	}

	binary, err := installedDotsBinary(ctx)
	if err != nil {
		return err
	}

	if err := setDefaultGlobalHooksPath(
		ctx,
		filepath.Join(dotfiles, "git-global-hooks"),
	); err != nil {
		return err
	}

	return syncNativePreCommit(ctx, dotfiles, binary, logger)
}

func setDefaultGlobalHooksPath(ctx context.Context, hooks string) error {
	for _, scope := range []string{"--global", "--system"} {
		values, err := readHookConfig(ctx, "", scope, "--includes", "--get-all", "core.hooksPath")
		if err != nil {
			return err
		}

		if len(values) != 0 {
			return nil
		}
	}

	return setGlobalHookConfig(ctx, "core.hooksPath", hooks)
}

func requireNativeHookGit(ctx context.Context) error {
	output, err := hookGitOutput(ctx, "", "--version")
	if err != nil {
		slog.WarnContext(ctx, "cannot determine the Git version", "err", err)

		return fmt.Errorf("%w: %w", errGitHookVersion, err)
	}

	const minimumMajor = 2
	const minimumMinor = 54
	version, _ := strings.CutPrefix(output, "git version ")
	majorText, rest, hasMinor := strings.Cut(version, ".")
	minorText, _, _ := strings.Cut(rest, ".")
	major, majorErr := strconv.Atoi(majorText)
	minor, minorErr := strconv.Atoi(minorText)

	if hasMinor && majorErr == nil && minorErr == nil &&
		(major > minimumMajor || major == minimumMajor && minor >= minimumMinor) {
		return nil
	}

	return fmt.Errorf("%w (found %q)", errGitHookVersion, strings.TrimSpace(output))
}

func dotsBinaryPath(ctx context.Context) (string, error) {
	if path := os.Getenv("DOTS_BINARY"); path != "" {
		return path, nil
	}

	if dir := os.Getenv("DOTS_BINARY_DIR"); dir != "" {
		return filepath.Join(dir, "dots"), nil
	}

	if cache := os.Getenv("XDG_CACHE_HOME"); cache != "" {
		return filepath.Join(cache, "dots", "bin", "dots"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		slog.WarnContext(ctx, "cannot locate the dots cache", "err", err)

		return "", fmt.Errorf("locate the dots cache: %w", err)
	}

	return filepath.Join(home, ".cache", "dots", "bin", "dots"), nil
}

func installedDotsBinary(ctx context.Context) (string, error) {
	path, err := dotsBinaryPath(ctx)
	if err != nil {
		return "", err
	}

	path, err = filepath.Abs(path)
	if err != nil {
		slog.WarnContext(ctx, "cannot resolve the installed dots binary", "err", err)

		return "", fmt.Errorf("resolve the installed dots binary: %w", err)
	}

	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("run install.sh to build the dots hook binary at %q: %w", path, err)
	}

	if info.Path != "goodkind.io/.dotfiles/cmd/dots" {
		return "", fmt.Errorf("%w at %q; found Go command %q", errDotsCommand, path, info.Path)
	}

	// The installer checks the executable identified by the dotfiles owner.
	command := exec.CommandContext(
		ctx,
		path,
		"git-hook",
		preCommitEvent,
		"--check-installation",
	) // #nosec G702
	if output, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf(
			"run install.sh to rebuild the dots hook binary at %q: %w: %s",
			path,
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return path, nil
}

func gitHookCommand(binary, dotfiles string) string {
	command := shellSingleQuote(binary) + " git-hook " + preCommitEvent

	return "DOTDOTFILES=" + shellSingleQuote(dotfiles) + " " + command
}

// RemoveNativePreCommit unregisters the dotfiles pre-commit hook.
// Uninstall unregisters the hook before deleting the binary.
//
// Example:
//
//	if err := RemoveNativePreCommit(ctx); err != nil {
//		return err
//	}
func RemoveNativePreCommit(ctx context.Context) error {
	for _, key := range []string{nativePreCommitEvent, nativePreCommitCmd} {
		values, err := readHookConfig(ctx, "", "--global", "--get-all", key)
		if err != nil {
			return err
		}

		if len(values) == 0 {
			continue
		}

		if _, err := hookGitOutput(ctx, "", "config", "--global", "--unset-all", key); err != nil {
			return err
		}
	}

	return nil
}

func syncNativePreCommit(
	ctx context.Context,
	dotfiles string,
	binary string,
	logger *telemetry.Logger,
) error {
	command := gitHookCommand(binary, dotfiles)
	if err := setGlobalHookConfig(ctx, nativePreCommitCmd, command); err != nil {
		return err
	}

	if err := setGlobalHookConfig(ctx, nativePreCommitEvent, preCommitEvent); err != nil {
		return err
	}

	repositories, err := configuredHookRepositories(ctx, dotfiles)
	if err != nil {
		return err
	}

	for _, repository := range repositories {
		if err := verifyNativePreCommit(ctx, repository, command); err != nil {
			return err
		}
	}

	if logger != nil {
		logger.InfoContext(
			ctx,
			fmt.Sprintf(
				"Verified the configured pre-commit hook in %d repositories",
				len(repositories),
			),
		)
	}

	return nil
}

func configuredHookRepositories(ctx context.Context, dotfiles string) ([]string, error) {
	paths, err := readHookConfig(
		ctx,
		dotfiles,
		"--path",
		"--get-all",
		"goodkind.protectedBranchEnforcePath",
	)
	if err != nil {
		return nil, err
	}

	var repositories []string
	seen := make(map[string]bool)
	addRepository := func(path string) (bool, error) {
		found, err := hasHookRepository(ctx, path)
		if !found || err != nil {
			return found, err
		}

		if !seen[path] {
			seen[path] = true
			repositories = append(repositories, path)
		}

		return true, nil
	}
	if _, err := addRepository(dotfiles); err != nil {
		return nil, err
	}

	for _, path := range paths {
		if !filepath.IsAbs(path) {
			continue
		}

		root, err := filepath.EvalSymlinks(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}

		if err != nil {
			slog.WarnContext(
				ctx,
				"cannot resolve the Git enforcement path",
				"path",
				path,
				"err",
				err,
			)

			return nil, fmt.Errorf("cannot resolve the Git enforcement path %q: %w", path, err)
		}

		err = filepath.WalkDir(
			root,
			func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}

				if !entry.IsDir() {
					return nil
				}

				found, err := addRepository(path)
				if err != nil {
					return err
				}

				if found {
					return filepath.SkipDir
				}

				return nil
			},
		)
		if err != nil {
			slog.WarnContext(ctx, "cannot list Git repositories", "root", root, "err", err)

			return nil, fmt.Errorf("cannot list Git repositories under %q: %w", root, err)
		}
	}

	return repositories, nil
}

func hasHookRepository(ctx context.Context, path string) (bool, error) {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err == nil && info.IsDir() {
		_, err = os.Stat(filepath.Join(gitPath, "HEAD"))
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		slog.WarnContext(ctx, "cannot inspect the Git repository", "path", path, "err", err)

		return false, fmt.Errorf("cannot inspect the Git repository at %q: %w", path, err)
	}

	return true, nil
}

func verifyNativePreCommit(ctx context.Context, repository, expectedCommand string) error {
	output, err := hookGitOutput(ctx, repository, "hook", "list", "-z", preCommitEvent)
	if err != nil {
		slog.WarnContext(
			ctx,
			"cannot verify the pre-commit hook",
			"repository",
			repository,
			"err",
			err,
		)

		return fmt.Errorf("cannot verify the configured pre-commit hook in %q: %w", repository, err)
	}

	if !slices.Contains(strings.Split(output, "\x00"), nativePreCommitHook) {
		return fmt.Errorf(
			"hook %s is not active in %q; %w",
			nativePreCommitHook,
			repository,
			errHookNotSelected,
		)
	}

	commands, err := readHookConfig(ctx, repository, "--get", nativePreCommitCmd)
	if err != nil {
		return err
	}

	if !slices.Equal(commands, []string{expectedCommand}) {
		return fmt.Errorf(
			"unexpected value for %s in %q; %w",
			nativePreCommitCmd,
			repository,
			errHookCommandChanged,
		)
	}

	return nil
}

func setGlobalHookConfig(ctx context.Context, key, value string) error {
	values, err := readHookConfig(ctx, "", "--global", "--get-all", key)
	if err != nil {
		return err
	}

	if slices.Equal(values, []string{value}) {
		return nil
	}

	_, err = hookGitOutput(ctx, "", "config", "--global", "--replace-all", key, value)

	return err
}

func readHookConfig(ctx context.Context, repository string, args ...string) ([]string, error) {
	args = append([]string{"config", "--null"}, args...)
	output, err := hookGitOutput(ctx, repository, args...)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && len(exitErr.Stderr) == 0 {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return strings.Split(strings.TrimSuffix(output, "\x00"), "\x00"), nil
}

func hookGitOutput(ctx context.Context, repository string, args ...string) (string, error) {
	slog.DebugContext(ctx, "reading Git hook configuration", "operation", args[0])

	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = repository
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) != 0 {
			slog.WarnContext(ctx, "Git hook command failed", "operation", args[0], "err", err)

			return "", fmt.Errorf(
				"git %s: %w: %s",
				args[0],
				err,
				strings.TrimSpace(string(exitErr.Stderr)),
			)
		}

		return "", fmt.Errorf("git %s: %w", args[0], err)
	}

	return string(output), nil
}
