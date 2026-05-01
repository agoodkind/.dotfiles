package common

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/telemetry"
)

func Info(logger *telemetry.Logger, message string) {
	if logger != nil {
		logger.Info(message)
	}
}

func Warn(logger *telemetry.Logger, message string) {
	if logger != nil {
		logger.Warn(message)
	}
}

func Infof(logger *telemetry.Logger, format string, args ...any) {
	if logger != nil {
		logger.Info(fmt.Sprintf(format, args...))
	}
}

func Warnf(logger *telemetry.Logger, format string, args ...any) {
	if logger != nil {
		logger.Warn(fmt.Sprintf(format, args...))
	}
}

func Debugf(logger *telemetry.Logger, format string, args ...any) {
	if logger != nil {
		logger.Debug(fmt.Sprintf(format, args...))
	}
}

func LoadOverrides() {
	overrides := filepath.Join(os.Getenv("HOME"), ".overrides.local")
	file, err := os.Open(overrides)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(parts[1], "\"'")
		if key != "" {
			_ = os.Setenv(key, value)
		}
	}
}

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

func Touch(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

func IsWorkLaptop() bool {
	return os.Getenv("WORK_DIR_PATH") != ""
}

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

func HasSudoAccess(ctx context.Context, logger *telemetry.Logger) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "sudo", "-n", "true"); err == nil {
		return true
	}
	return cmdexec.RunWithLoggerAndEnv(ctx, logger, nil, "sudo", "-v") == nil
}

func VersionAtLeast(current, minimum string) bool {
	currentParts := strings.Split(current, ".")
	minimumParts := strings.Split(minimum, ".")
	parts := len(currentParts)
	if len(minimumParts) > parts {
		parts = len(minimumParts)
	}
	for i := 0; i < parts; i++ {
		cur := 0
		min := 0
		if i < len(currentParts) {
			cur, _ = strconv.Atoi(strings.Split(currentParts[i], "-")[0])
		}
		if i < len(minimumParts) {
			min, _ = strconv.Atoi(strings.Split(minimumParts[i], "-")[0])
		}
		if cur > min {
			return true
		}
		if cur < min {
			return false
		}
	}
	return true
}

func DefaultPackageConfig() *catalog.PackageConfig {
	return catalog.DefaultPackageConfig()
}

func DefaultCustomToolDeclarations() []catalog.ToolDeclaration {
	tools := catalog.DefaultToolDeclarations()
	out := make([]catalog.ToolDeclaration, 0, len(tools))
	for _, tool := range tools {
		out = append(out, tool)
	}
	return out
}
