// Package common provides shared utilities for sync subpackages.
package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/telemetry"
)

var errDebianPrivilegeUnavailable = errors.New("root or passwordless sudo is required for Debian package operations")

// Warn logs message at warn level via logger, if logger is non-nil.
func Warn(logger *telemetry.Logger, message string) {
	if logger != nil {
		logger.Warn(message)
	}
}

// Warnf formats a message using printf-style %s substitution and logs it at warn level via logger.
func Warnf(logger *telemetry.Logger, format string, args ...string) {
	if logger != nil {
		logger.Warn(formatString(format, args...))
	}
}

// InfoContext logs message at info level via logger with context, if logger is non-nil.
func InfoContext(ctx context.Context, logger *telemetry.Logger, message string) {
	if logger != nil {
		logger.InfoContext(ctx, message)
	}
}

// WarnContext logs message at warn level via logger with context, if logger is non-nil.
func WarnContext(ctx context.Context, logger *telemetry.Logger, message string) {
	if logger != nil {
		logger.WarnContext(ctx, message)
	}
}

// InfoContextf formats a message and logs it at info level via logger with context, if logger is non-nil.
func InfoContextf(ctx context.Context, logger *telemetry.Logger, format string, args ...string) {
	if logger != nil {
		logger.InfoContext(ctx, formatString(format, args...))
	}
}

// WarnContextf formats a message and logs it at warn level via logger with context, if logger is non-nil.
func WarnContextf(ctx context.Context, logger *telemetry.Logger, format string, args ...string) {
	if logger != nil {
		logger.WarnContext(ctx, formatString(format, args...))
	}
}

func formatString(format string, args ...string) string {
	formatted := format
	for _, arg := range args {
		formatted = strings.Replace(formatted, "%s", arg, 1)
	}
	return formatted
}

// IsSymlinkTo reports whether target is a symlink that resolves to source.
func IsSymlinkTo(target, source string) bool {
	info, err := os.Lstat(target)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	actual, err := os.Readlink(target)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(actual) {
		actual = filepath.Join(filepath.Dir(target), actual)
	}
	expected, err := filepath.Abs(source)
	if err != nil {
		expected = source
	}
	return filepath.Clean(actual) == filepath.Clean(expected)
}

// Touch creates the file at path if it does not already exist, with mode 0600.
func Touch(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		slog.Error("common: Touch: opening file", "path", path, "err", err)
		return fmt.Errorf("opening file %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		slog.Error("common: Touch: closing file", "path", path, "err", err)
		return fmt.Errorf("closing file %s: %w", path, err)
	}
	return nil
}

// IsWorkLaptop reports whether the current machine is a work laptop based on the WORK_DIR_PATH env var.
func IsWorkLaptop() bool {
	return os.Getenv("WORK_DIR_PATH") != ""
}

// IsUbuntu reports whether the current OS is Linux and identifies as Ubuntu or Debian via /etc/os-release.
func IsUbuntu() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(content))
	return strings.Contains(lower, "ubuntu") || strings.Contains(lower, "debian")
}

// IsUbuntuOnly reports whether the current OS is Linux and identifies specifically as Ubuntu (not Debian).
// Use this instead of IsUbuntu when behaviour must be restricted to Ubuntu, e.g. adding Launchpad PPAs.
func IsUbuntuOnly() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(content))
	return strings.Contains(lower, "id=ubuntu")
}

// HasSudoAccess reports whether the current user can run sudo without a password prompt.
func HasSudoAccess(ctx context.Context, logger *telemetry.Logger) bool {
	if !runner.HasCommand("sudo") {
		return false
	}
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "sudo", "-n", "true"); err == nil {
		return true
	}
	return cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "sudo", "-v") == nil
}

// DebianPrivilegePrefix returns the command prefix required for Debian package operations.
func DebianPrivilegePrefix(euid int, sudoAvailable bool, sudoUsable bool) ([]string, error) {
	if euid == 0 {
		return nil, nil
	}
	if sudoAvailable && sudoUsable {
		return []string{"sudo", "-n"}, nil
	}
	return nil, errDebianPrivilegeUnavailable
}

// CurrentDebianPrivilegePrefix returns the command prefix for the current process user.
func CurrentDebianPrivilegePrefix(ctx context.Context, logger *telemetry.Logger) ([]string, error) {
	sudoAvailable := false
	sudoUsable := false
	if os.Geteuid() != 0 {
		sudoAvailable = runner.HasCommand("sudo")
		if sudoAvailable {
			sudoUsable = HasSudoAccess(ctx, logger)
		}
	}
	return DebianPrivilegePrefix(os.Geteuid(), sudoAvailable, sudoUsable)
}

// DebianPrivilegedCommand applies a Debian privilege prefix to a command.
func DebianPrivilegedCommand(prefix []string, command string, args ...string) (string, []string) {
	if len(prefix) == 0 {
		return command, args
	}
	prefixedArgs := append([]string{}, prefix[1:]...)
	prefixedArgs = append(prefixedArgs, command)
	prefixedArgs = append(prefixedArgs, args...)
	return prefix[0], prefixedArgs
}

// RunDebianPrivilegedCommand runs a command as root or through passwordless sudo.
func RunDebianPrivilegedCommand(ctx context.Context, logger *telemetry.Logger, command string, args ...string) error {
	prefix, err := CurrentDebianPrivilegePrefix(ctx, logger)
	if err != nil {
		return err
	}
	actualCommand, actualArgs := DebianPrivilegedCommand(prefix, command, args...)
	if err := cmdexec.RunWithLogger(ctx, logger, actualCommand, actualArgs...); err != nil {
		slog.ErrorContext(ctx, "running privileged command", "command", command, "err", err)
		return fmt.Errorf("running privileged command %s: %w", command, err)
	}
	return nil
}

// OutputDebianPrivilegedCommand runs a command as root or through passwordless sudo and returns its output.
func OutputDebianPrivilegedCommand(ctx context.Context, logger *telemetry.Logger, command string, args ...string) (string, error) {
	prefix, err := CurrentDebianPrivilegePrefix(ctx, logger)
	if err != nil {
		return "", err
	}
	actualCommand, actualArgs := DebianPrivilegedCommand(prefix, command, args...)
	out, err := cmdexec.OutputWithLogger(ctx, logger, actualCommand, actualArgs...)
	if err != nil {
		slog.ErrorContext(ctx, "running privileged command", "command", command, "err", err)
		return "", fmt.Errorf("running privileged command %s: %w", command, err)
	}
	return out, nil
}

// VersionAtLeast reports whether the current dot-separated version string is greater than or equal to minimum.
func VersionAtLeast(current, minimum string) bool {
	currentParts := strings.Split(current, ".")
	minimumParts := strings.Split(minimum, ".")
	parts := max(len(currentParts), len(minimumParts))
	for i := range parts {
		cur := 0
		minPart := 0
		if i < len(currentParts) {
			cur, _ = strconv.Atoi(strings.Split(currentParts[i], "-")[0])
		}
		if i < len(minimumParts) {
			minPart, _ = strconv.Atoi(strings.Split(minimumParts[i], "-")[0])
		}
		if cur > minPart {
			return true
		}
		if cur < minPart {
			return false
		}
	}
	return true
}

// DefaultPackageConfig returns the default package configuration from the catalog.
func DefaultPackageConfig() *catalog.PackageConfig {
	return catalog.DefaultPackageConfig()
}
