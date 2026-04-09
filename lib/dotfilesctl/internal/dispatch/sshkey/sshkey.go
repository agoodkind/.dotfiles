package sshkey

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

func Load(_ context.Context, dispatchLogger *telemetry.Logger) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	output, err := cmdexec.OutputWithLogger(context.Background(), dispatchLogger, "/usr/bin/ssh-add", "-l")
	if err == nil && strings.Contains(output, "id_ed25519") {
		dispatchLogger.Info("ssh key already loaded")
		return nil
	}
	_, err = cmdexec.OutputWithLogger(
		context.Background(),
		dispatchLogger,
		"/usr/bin/ssh-add",
		"--apple-use-keychain",
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519"),
	)
	if err == nil {
		dispatchLogger.Info("loaded id_ed25519 into keychain")
	}
	return err
}
