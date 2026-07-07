// Package common provides shared utilities for sync subpackages.
package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

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

// SyncLogPath returns the log file path for the current sync run, honoring
// the DOTFILES_LOG override so notifications point at the same log a running
// sync is writing to.
func SyncLogPath() string {
	if path := os.Getenv("DOTFILES_LOG"); path != "" {
		return path
	}
	return filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "sync.log")
}

// IsWorkLaptop reports whether the current machine is a work laptop based on the WORK_DIR_PATH env var.
func IsWorkLaptop() bool {
	return os.Getenv("WORK_DIR_PATH") != ""
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

// SudoersNopasswdEntry returns the sudoers drop-in line granting username
// passwordless sudo access.
func SudoersNopasswdEntry(username string) string {
	return username + " ALL=(ALL) NOPASSWD: ALL\n"
}

// SudoersDropInPath returns the /etc/sudoers.d path for username's passwordless entry.
func SudoersDropInPath(username string) string {
	return filepath.Join("/etc/sudoers.d", username+"-nopasswd")
}

// isValidSudoersUsername reports whether username is safe to embed verbatim in
// a sudoers file and a /etc/sudoers.d file name. It rejects whitespace,
// newlines, and path separators, so a malformed username can neither corrupt
// the sudoers entry nor let SudoersDropInPath escape /etc/sudoers.d. It also
// rejects a leading % or #, which sudoers reads as a group (%group) or numeric
// uid (#uid) rather than a plain user name.
func isValidSudoersUsername(username string) bool {
	if username == "" {
		return false
	}
	if strings.ContainsAny(username, " \t\n\r/\\") {
		return false
	}
	if strings.HasPrefix(username, "%") || strings.HasPrefix(username, "#") {
		return false
	}
	return true
}

// EnsurePasswordlessSudo grants the current user NOPASSWD sudo access by
// installing a /etc/sudoers.d drop-in, so later bootstrap steps and the
// background dots dispatch never block on an interactive sudo password
// prompt. It is a no-op when sudo is unavailable, the process is already
// root, or non-interactive sudo already works. The check here deliberately
// avoids HasSudoAccess, which falls back to an interactive `sudo -v`, since
// a password prompt succeeding there would report passwordless sudo as
// already configured and skip installing the drop-in. The drop-in is
// validated with `visudo -c` before being installed, so a malformed entry
// never reaches /etc/sudoers.d, and installing it still goes through `sudo`,
// which prompts once interactively the first time this runs in a real
// terminal.
func EnsurePasswordlessSudo(ctx context.Context, logger *telemetry.Logger, group string) error {
	if !runner.HasCommand("sudo") {
		return nil
	}
	if os.Geteuid() == 0 {
		return nil
	}
	// -k resets any cached sudo timestamp before the -n probe, so a recently
	// entered password cannot make this look like NOPASSWD is already set.
	if cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "sudo", "-k", "-n", "true") == nil {
		return nil
	}

	currentUser, err := user.Current()
	if err != nil {
		slog.WarnContext(ctx, "common: resolving current user for passwordless sudo", "err", err)
		return fmt.Errorf("resolving current user: %w", err)
	}
	if !isValidSudoersUsername(currentUser.Username) {
		slog.WarnContext(ctx, "common: username is not safe for a sudoers entry", "username", currentUser.Username)
		return fmt.Errorf("username %q is not safe for a sudoers entry", currentUser.Username)
	}

	dropInPath := SudoersDropInPath(currentUser.Username)
	if filepath.Dir(dropInPath) != "/etc/sudoers.d" {
		slog.WarnContext(ctx, "common: sudoers drop-in path escaped /etc/sudoers.d", "path", dropInPath)
		return fmt.Errorf("sudoers drop-in path %q escaped /etc/sudoers.d", dropInPath)
	}

	entry := SudoersNopasswdEntry(currentUser.Username)

	// Check the entry by feeding it to visudo on stdin (-f -), not a temp file.
	// A `visudo -c -f <file>` check can enforce sudoers ownership/mode on some
	// platforms, which a user-owned 0600 temp file would fail; stdin has no such
	// file to check, so the syntax check works on every platform.
	//
	// visudo lives in /usr/sbin (or /sbin) on Debian-family systems, which is
	// often absent from a login-shell PATH, so add those dirs for this call to
	// avoid a spurious "executable file not found". The captured output is logged
	// on failure so a real syntax error is diagnosable.
	visudoEnv := map[string]string{"PATH": os.Getenv("PATH") + ":/usr/sbin:/sbin"}
	visudoOutput, err := cmdexec.CombinedOutputWithInput(ctx, visudoEnv, strings.NewReader(entry), "visudo", "-c", "-f", "-")
	if err != nil {
		slog.WarnContext(ctx, "common: sudoers drop-in failed validation", "err", err, "visudo_output", strings.TrimSpace(visudoOutput))
		return fmt.Errorf("validating sudoers drop-in: %w", err)
	}

	tempFile, err := os.CreateTemp("", "sudoers-nopasswd-*")
	if err != nil {
		slog.WarnContext(ctx, "common: creating sudoers temp file", "err", err)
		return fmt.Errorf("creating sudoers temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.WriteString(entry); err != nil {
		tempFile.Close()
		slog.WarnContext(ctx, "common: writing sudoers temp file", "err", err)
		return fmt.Errorf("writing sudoers temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		slog.WarnContext(ctx, "common: closing sudoers temp file", "err", err)
		return fmt.Errorf("closing sudoers temp file: %w", err)
	}

	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "sudo", "install", "-m", "0440", "-o", "root", "-g", group, tempPath, dropInPath); err != nil {
		slog.WarnContext(ctx, "common: installing sudoers drop-in", "err", err)
		return fmt.Errorf("installing sudoers drop-in: %w", err)
	}

	// Confirm the drop-in actually took effect, since a sudoers.d include rule or
	// an unusable file could leave passwordless sudo inactive even after a clean
	// install, and later background work would then still block on a prompt.
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "sudo", "-k", "-n", "true"); err != nil {
		slog.WarnContext(ctx, "common: installed sudoers drop-in but passwordless sudo is still inactive", "path", dropInPath, "err", err)
		return fmt.Errorf("installed sudoers drop-in at %s but passwordless sudo is still inactive: %w", dropInPath, err)
	}

	InfoContext(ctx, logger, "  enabled passwordless sudo for "+currentUser.Username)
	return nil
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
