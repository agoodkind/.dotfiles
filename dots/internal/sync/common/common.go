// Package common provides shared utilities for sync subpackages.
package common

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/telemetry"
)

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

// HasSudoAccess reports whether the current user can run sudo without a password prompt.
func HasSudoAccess(ctx context.Context, logger *telemetry.Logger) bool {
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "sudo", "-n", "true"); err == nil {
		return true
	}
	return cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "sudo", "-v") == nil
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
