// Package runner provides a command runner abstraction for dots operations.
package runner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"

	"goodkind.io/.dotfiles/internal/telemetry"
)

var commandLogger *telemetry.Logger

// SetLogger sets the logger used by CommandWithContext to capture command output.
func SetLogger(logger *telemetry.Logger) {
	commandLogger = logger
}

// CommandWithContext creates a command that runs name with args in dir, wiring stdout/stderr to the active logger.
func CommandWithContext(ctx context.Context, dir string, name string, args ...string) *exec.Cmd {
	slog.InfoContext(ctx, "runner: creating command", "command", name)
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

// HasCommand reports whether name is available on PATH.
func HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// LookPath returns the absolute path of name on PATH, or an error if not found.
func LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		slog.Warn("runner: command not found", "name", name, "err", err)
		return "", fmt.Errorf("looking up %s: %w", name, err)
	}
	return path, nil
}

// CommandOutputWriterConfig configures a writer that routes command output to a logger or a fallback writer.
type CommandOutputWriterConfig struct {
	Logger   *telemetry.Logger
	Fallback io.Writer
}

// NewCommandOutputWriter creates an output writer configured with config.
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
		n, err := w.fallback.Write(p)
		if err != nil {
			return n, fmt.Errorf("writing to fallback: %w", err)
		}
		return n, nil
	}
	return len(p), nil
}
