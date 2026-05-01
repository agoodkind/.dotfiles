package logging

import (
	"os"
	"path/filepath"

	"goodkind.io/.dotfiles/internal/telemetry"
)

var syncLogger *telemetry.Logger
var debugEnabled bool

func Configure() {
	debugEnabled = true
	logPath := os.Getenv("DOTFILES_LOG")
	if logPath == "" {
		logPath = filepathForCursorLog()
		_ = os.Setenv("DOTFILES_LOG", logPath)
	}
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		syncLogger = nil
		return
	}
	syncLogger = logger
}

func ConfigureWithLogger(logger *telemetry.Logger) {
	if logger != nil {
		syncLogger = logger
		return
	}
	Configure()
}

func Info(message string) {
	logger := ensureLogger()
	if logger != nil {
		logger.Info(message)
		return
	}
}

func Debug(message string) {
	if !debugEnabled {
		return
	}
	logger := ensureLogger()
	if logger != nil {
		logger.Debug(message)
		return
	}
}

func Error(message string) {
	logger := ensureLogger()
	if logger != nil {
		logger.Error(message)
		return
	}
}

func filepathForCursorLog() string {
	home := os.Getenv("HOME")
	return filepath.Join(home, ".cache", "dotfiles", "cursor.log")
}

func ensureLogger() *telemetry.Logger {
	if syncLogger != nil {
		return syncLogger
	}

	logPath := os.Getenv("DOTFILES_LOG")
	if logPath == "" {
		logPath = filepathForCursorLog()
		_ = os.Setenv("DOTFILES_LOG", logPath)
	}
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		return nil
	}
	syncLogger = logger
	return syncLogger
}
