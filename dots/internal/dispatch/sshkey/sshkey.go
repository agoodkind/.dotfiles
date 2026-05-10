// Package sshkey implements SSH key loading for the dispatch worker.
package sshkey

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// Load loads the ed25519 SSH key into the macOS keychain if it is not already present.
func Load(ctx context.Context, dispatchLogger *telemetry.Logger) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	output, err := cmdexec.OutputWithLogger(ctx, dispatchLogger, "/usr/bin/ssh-add", "-l")
	if err == nil && strings.Contains(output, "id_ed25519") {
		dispatchLogger.InfoContext(ctx, "ssh key already loaded")
		return nil
	}
	_, err = cmdexec.OutputWithLogger(
		ctx,
		dispatchLogger,
		"/usr/bin/ssh-add",
		"--apple-use-keychain",
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519"),
	)
	if err != nil {
		slog.WarnContext(ctx, "sshkey: Load: ssh-add failed", "err", err)
		return fmt.Errorf("running ssh-add: %w", err)
	}
	dispatchLogger.InfoContext(ctx, "loaded id_ed25519 into keychain")
	return nil
}
