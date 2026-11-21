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
	"strings"

	"go.yaml.in/yaml/v3"
)

const gitGuardianNestedConfigVersion = 2

var (
	errGitGuardianShowSecretsArg = errors.New(
		"remove --show-secrets from the ggshield pre-commit arguments",
	)
	errGitGuardianInvalidConfig = errors.New("fix the invalid ggshield configuration")
	errGitGuardianShowSecrets   = errors.New(
		"set show_secrets to false in the ggshield configuration before committing",
	)
)

func scanStagedSecrets(
	ctx context.Context,
	repoRoot string,
	dotfilesRoot string,
	args []string,
	stderr io.Writer,
) error {
	if err := scanWithGitGuardian(ctx, repoRoot, args, stderr); err != nil {
		return err
	}

	gitleaks, err := exec.LookPath("gitleaks")
	if errors.Is(err, exec.ErrNotFound) {
		fmt.Fprintln(
			stderr,
			"warning: gitleaks not found; secret scan skipped (install: brew install gitleaks)",
		)

		return nil
	}

	if err != nil {
		slog.WarnContext(ctx, "cannot locate gitleaks", "err", err)

		return fmt.Errorf("locate gitleaks: %w", err)
	}

	config, err := stagedGitleaksConfig(ctx, repoRoot, dotfilesRoot)
	if err != nil {
		return err
	}

	scanArgs := []string{"protect", "--staged", "--no-banner", "--redact"}
	if config != "" {
		scanArgs = append(scanArgs, "--config", config)
	}

	return runStagedSecretScan(ctx, repoRoot, gitleaks, scanArgs, stderr)
}

func scanWithGitGuardian(
	ctx context.Context,
	repoRoot string,
	args []string,
	stderr io.Writer,
) error {
	ggshield, err := exec.LookPath("ggshield")
	if errors.Is(err, exec.ErrNotFound) {
		fmt.Fprintln(
			stderr,
			"warning: ggshield not found; GitGuardian scan skipped (install: brew install ggshield)",
		)

		return nil
	}

	if err != nil {
		slog.WarnContext(ctx, "cannot locate ggshield", "err", err)

		return fmt.Errorf("locate ggshield: %w", err)
	}

	status := exec.CommandContext(ctx, ggshield, "api-status", "--no-check-for-updates")
	status.Dir = repoRoot

	if err := status.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			slog.WarnContext(ctx, "ggshield authentication check did not finish", "err", ctxErr)

			return fmt.Errorf("run ggshield api-status: %w", ctxErr)
		}

		if _, exited := errors.AsType[*exec.ExitError](err); !exited {
			slog.WarnContext(ctx, "cannot check ggshield authentication", "err", err)

			return fmt.Errorf("run ggshield api-status: %w", err)
		}

		fmt.Fprintln(stderr, "warning: ggshield is not authenticated; GitGuardian scan skipped")
		fmt.Fprintln(stderr, "hint: run 'ggshield auth login' to finish GitGuardian setup")

		return nil
	}

	if err := checkGitGuardianRedaction(ctx, repoRoot, args); err != nil {
		return err
	}

	scanArgs := append([]string{"secret", "scan", "pre-commit", "--no-check-for-updates"}, args...)

	return runStagedSecretScan(ctx, repoRoot, ggshield, scanArgs, stderr)
}

func checkGitGuardianRedaction(ctx context.Context, repoRoot string, args []string) error {
	for _, arg := range args {
		if arg == "--" {
			break
		}

		option, _, _ := strings.Cut(arg, "=")
		if option == "--show-secrets" {
			return errGitGuardianShowSecretsArg
		}
	}

	home := os.Getenv("GG_USER_HOME_DIR")
	if home == "" {
		home = os.Getenv("HOME")
	}

	redactionDisabled := false

	for _, dir := range []string{home, repoRoot} {
		if dir == "" {
			continue
		}

		showSecrets, err := gitGuardianShowSecrets(ctx, dir)
		if err != nil {
			return err
		}

		if showSecrets != nil {
			redactionDisabled = *showSecrets
		}
	}

	if redactionDisabled {
		return errGitGuardianShowSecrets
	}

	return nil
}

func gitGuardianShowSecrets(ctx context.Context, dir string) (*bool, error) {
	root, err := openHookConfigRoot(ctx, dir)
	if err != nil || root == nil {
		return nil, err
	}

	defer closeHookConfigRoot(ctx, root)

	for _, name := range [...]string{".gitguardian", ".gitguardian.yml", ".gitguardian.yaml"} {
		data, err := root.ReadFile(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}

		if err != nil {
			slog.WarnContext(ctx, "cannot read ggshield configuration", "err", errors.Unwrap(err))

			return nil, fmt.Errorf("read ggshield configuration: %w", err)
		}

		showSecrets, err := unmarshalGitGuardianShowSecrets(data)
		if err != nil {
			slog.WarnContext(
				ctx,
				"cannot parse ggshield configuration",
				"err",
				errGitGuardianInvalidConfig,
			)

			return nil, fmt.Errorf(
				"%w in %q",
				errGitGuardianInvalidConfig,
				filepath.Join(dir, name),
			)
		}

		return showSecrets, nil
	}

	return nil, nil
}

func unmarshalGitGuardianShowSecrets(data []byte) (*bool, error) {
	var config struct {
		Version int `yaml:"version"`
		Secret  struct {
			ShowSecrets   *bool `yaml:"show_secrets"`
			ShowSecretsV1 *bool `yaml:"show-secrets"`
		} `yaml:"secret"`
		ShowSecrets   *bool `yaml:"show_secrets"`
		ShowSecretsV1 *bool `yaml:"show-secrets"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errGitGuardianInvalidConfig
	}

	showSecrets, dashed := config.ShowSecrets, config.ShowSecretsV1
	if config.Version == gitGuardianNestedConfigVersion {
		showSecrets, dashed = config.Secret.ShowSecrets, config.Secret.ShowSecretsV1
	}

	if showSecrets == nil {
		return dashed, nil
	}

	return showSecrets, nil
}

func stagedGitleaksConfig(ctx context.Context, repoRoot, dotfilesRoot string) (string, error) {
	for _, root := range []string{repoRoot, dotfilesRoot} {
		path := filepath.Join(root, ".gitleaks.toml")
		exists, err := isHookConfigFile(ctx, path)
		if err != nil {
			return "", err
		}

		if exists {
			return path, nil
		}
	}

	return "", nil
}

func runStagedSecretScan(
	ctx context.Context,
	repoRoot string,
	executable string,
	args []string,
	stderr io.Writer,
) error {
	scanner := filepath.Base(executable)
	slog.DebugContext(ctx, "scanning staged changes", "scanner", scanner)

	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = repoRoot
	command.Stdout = stderr
	command.Stderr = stderr

	if err := command.Run(); err != nil {
		slog.WarnContext(ctx, "staged secret scan failed", "scanner", scanner, "err", err)

		return fmt.Errorf("%s staged secret scan failed: %w", scanner, err)
	}

	return nil
}
