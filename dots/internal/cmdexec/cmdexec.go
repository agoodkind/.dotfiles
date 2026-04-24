package cmdexec

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/agoodkind/.dotfiles/internal/runner"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

func Run(ctx context.Context, command string, args ...string) error {
	return RunWithEnv(ctx, nil, command, args...)
}

func RunWithEnv(ctx context.Context, env map[string]string, command string, args ...string) error {
	return RunWithDirAndEnv(ctx, "", env, command, args...)
}

func RunWithDir(ctx context.Context, dir string, command string, args ...string) error {
	return RunWithDirAndEnv(ctx, dir, nil, command, args...)
}

func RunWithDirAndEnv(ctx context.Context, dir string, env map[string]string, command string, args ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := runner.CommandWithContext(ctx, dir, command, args...)
	cmd.Env = mergeEnv(env, os.Environ())
	return cmd.Run()
}

func Output(ctx context.Context, command string, args ...string) (string, error) {
	return OutputWithEnv(ctx, nil, command, args...)
}

func OutputWithEnv(ctx context.Context, env map[string]string, command string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = mergeEnv(env, os.Environ())
	output, err := cmd.Output()
	return string(output), err
}

func OutputTrimmed(ctx context.Context, command string, args ...string) (string, error) {
	output, err := Output(ctx, command, args...)
	return strings.TrimSpace(output), err
}

func RunWithLogger(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error {
	return RunWithLoggerAndEnv(ctx, logger, nil, command, args...)
}

func RunWithLoggerAndEnv(ctx context.Context, logger *telemetry.Logger, env []string, command string, args ...string) error {
	return runWithLogger(ctx, logger, env, command, args...)
}

func OutputWithLogger(ctx context.Context, logger *telemetry.Logger, command string, args ...string) (string, error) {
	return OutputWithLoggerAndEnv(ctx, logger, nil, command, args...)
}

func OutputWithLoggerAndEnv(ctx context.Context, logger *telemetry.Logger, env []string, command string, args ...string) (string, error) {
	return outputWithLogger(ctx, logger, env, command, args...)
}

func CombinedOutput(ctx context.Context, command string, args ...string) (string, error) {
	return CombinedOutputWithEnv(ctx, nil, command, args...)
}

func CombinedOutputWithEnv(ctx context.Context, env map[string]string, command string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = mergeEnv(env, os.Environ())
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func CombinedOutputWithInput(ctx context.Context, env map[string]string, input io.Reader, command string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = mergeEnv(env, os.Environ())
	cmd.Stdin = input
	output, err := cmd.Output()
	return string(output), err
}

func HasCommand(name string) bool {
	return runner.HasCommand(name)
}

func runWithLogger(ctx context.Context, logger *telemetry.Logger, env []string, command string, args ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
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
	return cmd.Run()
}

func outputWithLogger(ctx context.Context, logger *telemetry.Logger, env []string, command string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = os.Stdin
	if env != nil {
		cmd.Env = env
	} else if command != "bash" && !strings.Contains(command, "bash") {
		cmd.Env = append(os.Environ(), "DOTDOTFILES="+os.Getenv("DOTDOTFILES"), "DOTFILES_LOG="+os.Getenv("DOTFILES_LOG"))
	}
	out, err := cmd.CombinedOutput()
	if logger != nil {
		logger.RawOutput(string(out))
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
