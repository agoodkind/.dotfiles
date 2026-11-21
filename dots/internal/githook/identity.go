package githook

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type commitIdentity struct {
	email string
	name  string
}

func checkCommitIdentity(ctx context.Context, repoRoot, dotfilesRoot, home string) error {
	allowed, err := allowedCommitIdentity(ctx, repoRoot, dotfilesRoot, home)
	if err != nil {
		return err
	}

	if allowed.email == "" && allowed.name == "" {
		return nil
	}

	current, err := readCommitIdentity(ctx, repoRoot, "user.email", "user.name")
	if err != nil {
		return err
	}

	if current == allowed {
		return nil
	}

	return &RefusalError{
		lines: []string{
			"error: commit blocked: Git identity does not match the allowed identity.",
			fmt.Sprintf("  Current Git identity: %q <%s>", current.name, current.email),
			fmt.Sprintf("  Allowed Git identity: %q <%s>", allowed.name, allowed.email),
			"  Set user.name and user.email to the allowed values with git config.",
		},
	}
}

func allowedCommitIdentity(
	ctx context.Context,
	repoRoot string,
	dotfilesRoot string,
	home string,
) (commitIdentity, error) {
	configured, err := readCommitIdentity(
		ctx,
		repoRoot,
		"goodkind.allowedEmail",
		"goodkind.allowedName",
	)
	if err != nil {
		return commitIdentity{}, err
	}

	if configured.email != "" && configured.name != "" {
		return configured, nil
	}

	path, err := commitIdentityRulesPath(ctx, repoRoot, dotfilesRoot, home)
	if err != nil {
		return commitIdentity{}, err
	}

	base, err := readCommitIdentityRules(ctx, path)
	if err != nil || base == nil {
		return commitIdentity{}, err
	}

	local, err := readCommitIdentityRules(ctx, path+".local")
	if err != nil {
		return commitIdentity{}, err
	}

	return matchCommitIdentityRules(repoRoot, home, base, local), nil
}

func commitIdentityRulesPath(
	ctx context.Context,
	repoRoot string,
	dotfilesRoot string,
	home string,
) (string, error) {
	if path := os.Getenv("GIT_EMAIL_RULES"); path != "" {
		if filepath.IsAbs(path) {
			return path, nil
		}

		return filepath.Join(repoRoot, path), nil
	}

	path := filepath.Join(home, ".config", "git", "email-rules")
	exists, err := isHookConfigFile(ctx, path)
	if err != nil {
		return "", err
	}

	if exists {
		return path, nil
	}

	return filepath.Join(dotfilesRoot, "git-global-hooks", "email-rules"), nil
}

func matchCommitIdentityRules(repoRoot, home string, ruleFiles ...[]byte) commitIdentity {
	var allowed commitIdentity

	for _, rules := range ruleFiles {
		for line := range strings.SplitSeq(string(rules), "\n") {
			line, _, _ = strings.Cut(line, "#")
			prefix, rest := splitCommitIdentityField(line)
			if prefix == "" {
				continue
			}

			email, name := splitCommitIdentityField(rest)
			identity := commitIdentity{email: email, name: name}
			if prefix == "*" {
				if allowed.email == "" {
					allowed = identity
				}

				continue
			}

			if suffix, ok := strings.CutPrefix(prefix, "~"); ok {
				prefix = home + suffix
			}

			if strings.HasPrefix(repoRoot, prefix) {
				allowed = identity
			}
		}
	}

	return allowed
}

func splitCommitIdentityField(value string) (field, rest string) {
	value = strings.Trim(value, " \t")
	if index := strings.IndexAny(value, " \t"); index >= 0 {
		return value[:index], strings.TrimLeft(value[index:], " \t")
	}

	return value, ""
}

func readCommitIdentityRules(ctx context.Context, path string) ([]byte, error) {
	root, err := openHookConfigRoot(ctx, filepath.Dir(path))
	if err != nil || root == nil {
		return nil, err
	}

	defer closeHookConfigRoot(ctx, root)

	name := filepath.Base(path)
	exists, err := isRegularHookConfig(ctx, root, name)
	if err != nil || !exists {
		return nil, err
	}

	rules, err := root.ReadFile(name)
	if err != nil {
		slog.WarnContext(ctx, "cannot read Git identity rules", "err", errors.Unwrap(err))

		return nil, fmt.Errorf("read identity rules: %w", err)
	}

	return rules, nil
}

func isHookConfigFile(ctx context.Context, path string) (bool, error) {
	root, err := openHookConfigRoot(ctx, filepath.Dir(path))
	if err != nil || root == nil {
		return false, err
	}

	defer closeHookConfigRoot(ctx, root)

	return isRegularHookConfig(ctx, root, filepath.Base(path))
}

func openHookConfigRoot(ctx context.Context, dir string) (*os.Root, error) {
	root, err := os.OpenRoot(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		slog.WarnContext(
			ctx,
			"cannot open Git hook configuration directory",
			"err",
			errors.Unwrap(err),
		)

		return nil, fmt.Errorf("open hook configuration directory: %w", err)
	}

	return root, nil
}

func closeHookConfigRoot(ctx context.Context, root *os.Root) {
	if err := root.Close(); err != nil {
		slog.WarnContext(
			ctx,
			"cannot close Git hook configuration directory",
			"err",
			errors.Unwrap(err),
		)
	}
}

func isRegularHookConfig(ctx context.Context, root *os.Root, name string) (bool, error) {
	info, err := root.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		slog.WarnContext(ctx, "cannot inspect Git hook configuration", "err", errors.Unwrap(err))

		return false, fmt.Errorf("inspect hook configuration: %w", err)
	}

	return info.Mode().IsRegular(), nil
}

func readCommitIdentity(
	ctx context.Context,
	repoRoot string,
	emailKey string,
	nameKey string,
) (commitIdentity, error) {
	email, err := readCommitIdentityConfig(ctx, repoRoot, emailKey)
	if err != nil {
		return commitIdentity{}, err
	}

	name, err := readCommitIdentityConfig(ctx, repoRoot, nameKey)
	if err != nil {
		return commitIdentity{}, err
	}

	return commitIdentity{email: email, name: name}, nil
}

func readCommitIdentityConfig(ctx context.Context, repoRoot, key string) (string, error) {
	slog.DebugContext(ctx, "reading Git identity configuration", "key", key)

	command := exec.CommandContext(ctx, "git", "config", "--get", key)
	command.Dir = repoRoot
	output, err := command.Output()
	exitErr, exited := errors.AsType[*exec.ExitError](err)
	if exited && exitErr.ExitCode() == 1 && len(exitErr.Stderr) == 0 {
		return "", nil
	}

	if err != nil {
		slog.WarnContext(ctx, "cannot read Git identity configuration", "key", key, "err", err)

		return "", fmt.Errorf("read Git config %s: %w", key, err)
	}

	return strings.TrimRight(string(output), "\n"), nil
}
