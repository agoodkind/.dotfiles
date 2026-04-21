package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

var commandLogger *telemetry.Logger

func SetLogger(logger *telemetry.Logger) {
	commandLogger = logger
}

func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdout = NewCommandOutputWriter(CommandOutputWriterConfig{
		Logger:   commandLogger,
		Fallback: os.Stdout,
	})
	cmd.Stderr = NewCommandOutputWriter(CommandOutputWriterConfig{
		Logger:   commandLogger,
		Fallback: os.Stderr,
	})
	cmd.Stdin = os.Stdin
	return cmd
}

func CommandWithContext(ctx context.Context, dir string, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = NewCommandOutputWriter(CommandOutputWriterConfig{
		Logger:   commandLogger,
		Fallback: os.Stdout,
	})
	cmd.Stderr = NewCommandOutputWriter(CommandOutputWriterConfig{
		Logger:   commandLogger,
		Fallback: os.Stderr,
	})
	cmd.Stdin = os.Stdin
	return cmd
}

func HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func RunCommand(ctx context.Context, cmd *exec.Cmd) error {
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

func RunShell(ctx context.Context, script string, env map[string]string, dir string) error {
	cmd := exec.CommandContext(ctx, "bash", "-lc", script)
	cmd.Dir = dir
	cmd.Stdout = NewCommandOutputWriter(CommandOutputWriterConfig{
		Logger:   commandLogger,
		Fallback: os.Stdout,
	})
	cmd.Stderr = NewCommandOutputWriter(CommandOutputWriterConfig{
		Logger:   commandLogger,
		Fallback: os.Stderr,
	})
	cmd.Stdin = os.Stdin

	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	return cmd.Run()
}

type CommandOutputWriterConfig struct {
	Logger   *telemetry.Logger
	Fallback io.Writer
}

func NewCommandOutputWriter(config CommandOutputWriterConfig) io.Writer {
	return commandOutputWriter{
		logger:   config.Logger,
		fallback: config.Fallback,
	}
}

type commandOutputWriter struct {
	logger   *telemetry.Logger
	fallback io.Writer
}

func (w commandOutputWriter) Write(p []byte) (int, error) {
	if w.logger != nil {
		w.logger.RawOutput(string(p))
		return len(p), nil
	}
	if w.fallback != nil {
		return w.fallback.Write(p)
	}
	return len(p), nil
}
