// Package cmdexec provides utilities for executing shell commands.
package cmdexec

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// Run runs command with the given context and args.
func Run(ctx context.Context, command string, args ...string) error {
	return RunWithEnv(ctx, nil, command, args...)
}

// RunWithEnv runs command with the given context, env, and args.
func RunWithEnv(ctx context.Context, env map[string]string, command string, args ...string) error {
	return RunWithDirAndEnv(ctx, "", env, command, args...)
}

// RunWithDirAndEnv runs command with the given context, working directory, env, and args.
func RunWithDirAndEnv(ctx context.Context, dir string, env map[string]string, command string, args ...string) error {
	cmd := runner.CommandWithContext(ctx, dir, command, args...)
	cmd.Env = mergeEnv(env, os.Environ())
	if err := cmd.Run(); err != nil {
		slog.WarnContext(ctx, "cmdexec: RunWithDirAndEnv failed", "command", command, "err", err)
		return fmt.Errorf("running %s: %w", command, err)
	}
	return nil
}

// Output runs command and returns its stdout.
func Output(ctx context.Context, command string, args ...string) (string, error) {
	return OutputWithEnv(ctx, nil, command, args...)
}

// OutputWithEnv runs command with the given env and returns its stdout.
func OutputWithEnv(ctx context.Context, env map[string]string, command string, args ...string) (string, error) {
	slog.InfoContext(ctx, "cmdexec: running command", "command", command)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = mergeEnv(env, os.Environ())
	output, err := cmd.Output()
	return string(output), err
}

// OutputTrimmed runs command and returns its stdout with leading and trailing whitespace trimmed.
func OutputTrimmed(ctx context.Context, command string, args ...string) (string, error) {
	output, err := Output(ctx, command, args...)
	return strings.TrimSpace(output), err
}

// RunWithLogger runs command with the given context and logger.
func RunWithLogger(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error {
	return RunWithLoggerAndEnv(ctx, logger, nil, command, args...)
}

// RunWithLoggerAndEnv runs command with the given context, logger, and env.
func RunWithLoggerAndEnv(ctx context.Context, logger *telemetry.Logger, env []string, command string, args ...string) error {
	return runWithLogger(ctx, logger, env, command, args...)
}

// OutputWithLogger runs command and returns combined output, routing it through logger.
func OutputWithLogger(ctx context.Context, logger *telemetry.Logger, command string, args ...string) (string, error) {
	return OutputWithLoggerAndEnv(ctx, logger, nil, command, args...)
}

// OutputWithLoggerAndEnv runs command with the given logger and env and returns combined output.
func OutputWithLoggerAndEnv(ctx context.Context, logger *telemetry.Logger, env []string, command string, args ...string) (string, error) {
	return outputWithLogger(ctx, logger, env, command, args...)
}

// CombinedOutputWithInput runs command with the given env and stdin, returning combined output.
func CombinedOutputWithInput(ctx context.Context, env map[string]string, input io.Reader, command string, args ...string) (string, error) {
	slog.InfoContext(ctx, "cmdexec: running command with input", "command", command)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = mergeEnv(env, os.Environ())
	cmd.Stdin = input
	output, err := cmd.Output()
	return string(output), err
}

func runWithLogger(ctx context.Context, logger *telemetry.Logger, env []string, command string, args ...string) error {
	slog.InfoContext(ctx, "cmdexec: running command with logger", "command", command)
	cmd := runner.CommandWithContext(ctx, "", command, args...)
	if env != nil {
		cmd.Env = env
	} else if command != "bash" && !strings.Contains(command, "bash") {
		cmd.Env = append(os.Environ(), "DOTDOTFILES="+os.Getenv("DOTDOTFILES"), "DOTFILES_LOG="+os.Getenv("DOTFILES_LOG"))
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = runner.NewCommandOutputWriter(runner.CommandOutputWriterConfig{
		Logger:   logger,
		Fallback: os.Stdout,
	})
	cmd.Stderr = runner.NewCommandOutputWriter(runner.CommandOutputWriterConfig{
		Logger:   logger,
		Fallback: os.Stderr,
	})
	if err := cmd.Run(); err != nil {
		slog.WarnContext(ctx, "cmdexec: command failed", "command", command, "err", err)
		return fmt.Errorf("running %s: %w", command, err)
	}
	return nil
}

func outputWithLogger(ctx context.Context, logger *telemetry.Logger, env []string, command string, args ...string) (string, error) {
	slog.InfoContext(ctx, "cmdexec: outputWithLogger", "command", command)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = os.Stdin
	if env != nil {
		cmd.Env = env
	} else if command != "bash" && !strings.Contains(command, "bash") {
		cmd.Env = append(os.Environ(), "DOTDOTFILES="+os.Getenv("DOTDOTFILES"), "DOTFILES_LOG="+os.Getenv("DOTFILES_LOG"))
	}
	out, err := cmd.CombinedOutput()
	if logger != nil {
		logger.RawOutputContext(ctx, string(out))
	}
	return string(out), err
}

func mergeEnv(extra map[string]string, base []string) []string {
	if len(extra) == 0 {
		return base
	}
	merged := make([]string, 0, len(base)+len(extra))
	merged = append(merged, base...)
	for key, value := range extra {
		merged = append(merged, key+"="+value)
	}
	return merged
}
