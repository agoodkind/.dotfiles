// Package logging provides logging utilities for Cursor sync operations.
package logging

import (
	"os"
	"path/filepath"

	"goodkind.io/.dotfiles/internal/telemetry"
)

var (
	syncLogger   *telemetry.Logger
	debugEnabled bool
)

// Configure initialises the package-level sync logger using DOTFILES_LOG or a default path.
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

// ConfigureWithLogger sets the sync logger to logger, falling back to Configure if logger is nil.
func ConfigureWithLogger(logger *telemetry.Logger) {
	if logger != nil {
		syncLogger = logger
		return
	}
	Configure()
}

// Info logs an informational message through the sync logger.
func Info(message string) {
	logger := ensureLogger()
	if logger != nil {
		logger.Info(message)
		return
	}
}

// Debug logs a debug message through the sync logger when debug mode is enabled.
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

// ErrorWithErr logs an error message and its associated error through the sync logger.
func ErrorWithErr(message string, err error) {
	logger := ensureLogger()
	if logger != nil {
		logger.ErrorWithErr(message, err)
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
