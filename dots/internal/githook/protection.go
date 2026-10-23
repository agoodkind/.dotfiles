package githook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"goodkind.io/.dotfiles/internal/cmdexec"
)

func primaryMutationsBlocked(ctx context.Context) bool {
	protected, err := defaultBranchRequiresPullRequest(ctx)
	if err != nil {
		slog.WarnContext(ctx, "allowing the mutation because GitHub protection lookup failed", "err", err)
		return false
	}
	return protected
}

func defaultBranchRequiresPullRequest(ctx context.Context) (bool, error) {
	slog.DebugContext(ctx, "reading GitHub pull request protection")
	root := os.Getenv("DOTDOTFILES")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			slog.WarnContext(ctx, "locating dotfiles failed", "err", err)
			return false, fmt.Errorf("locate dotfiles: %w", err)
		}
		root = filepath.Join(home, ".dotfiles")
	}
	helper := filepath.Join(root, "git-global-hooks", "github-pr-protection.sh")
	output, err := cmdexec.Output(ctx, helper, ".")
	if err != nil {
		stderr := ""
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			stderr = string(exitError.Stderr)
		}
		slog.WarnContext(ctx, "GitHub protection helper failed", "helper", helper, "err", err, "stderr", stderr)
		return false, fmt.Errorf("%s: %w: %s", helper, err, stderr)
	}
	var result struct {
		Protected *bool `json:"pr_protected"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		slog.WarnContext(ctx, "decoding GitHub protection result failed", "err", err)
		return false, fmt.Errorf("decode GitHub protection result: %w", err)
	}
	if result.Protected == nil {
		slog.WarnContext(ctx, "GitHub protection result does not include pr_protected")
		return false, fmt.Errorf("GitHub protection result does not include pr_protected")
	}
	return *result.Protected, nil
}
