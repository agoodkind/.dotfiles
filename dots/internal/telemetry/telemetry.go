// Package telemetry provides structured logging and telemetry for dots.
package telemetry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"goodkind.io/.dotfiles/internal/clock"
	"goodkind.io/gklog/correlation"
)

const (
	colorReset             = "\x1b[0m"
	colorBlue              = "\x1b[34m"
	colorGreen             = "\x1b[32m"
	colorYellow            = "\x1b[33m"
	colorRed               = "\x1b[31m"
	colorGray              = "\x1b[90m"
	displayTimestampFormat = "2006-01-02 15:04:05"
)

type environmentLogLevel string

const (
	environmentLogLevelDebug   environmentLogLevel = "debug"
	environmentLogLevelWarn    environmentLogLevel = "warn"
	environmentLogLevelWarning environmentLogLevel = "warning"
	environmentLogLevelError   environmentLogLevel = "error"
)

// ConfigureDefaultSlogFromEnv configures package-level slog output for helper packages.
func ConfigureDefaultSlogFromEnv() {
	level := slog.LevelWarn
	rawLevel := environmentLogLevel(strings.ToLower(strings.TrimSpace(os.Getenv("DOTFILES_LOG_LEVEL"))))
	switch rawLevel {
	case environmentLogLevelDebug:
		level = slog.LevelDebug
	case environmentLogLevelWarn, environmentLogLevelWarning:
		level = slog.LevelWarn
	case environmentLogLevelError:
		level = slog.LevelError
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))
}

// Logger provides structured logging to both a JSON log file and a TTY-aware stdout/stderr.
type Logger struct {
	mu         sync.Mutex
	filePath   string
	logFile    *os.File
	slogLogger *slog.Logger
	stdout     io.Writer
	stderr     io.Writer

	interactive bool
	spinProgram *tea.Program
}

// NewLogger creates and opens a Logger that writes structured JSON logs to logPath.
func NewLogger(logPath string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		slog.Error("telemetry: NewLogger: creating log directory", "path", logPath, "err", err)
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Error("telemetry: NewLogger: opening log file", "path", logPath, "err", err)
		return nil, fmt.Errorf("opening log file: %w", err)
	}

	fileHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	})
	correlatedHandler := correlation.SlogHandler(fileHandler, correlation.HandlerOptions{})

	return &Logger{
		mu:          sync.Mutex{},
		filePath:    logPath,
		logFile:     logFile,
		slogLogger:  slog.New(correlatedHandler),
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		interactive: isTTY(os.Stdout),
		spinProgram: nil,
	}, nil
}

// Close flushes any in-progress spinner and closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerLocked()
	if err := l.logFile.Close(); err != nil {
		slog.Error("telemetry: Close: closing log file", "err", err)
		return fmt.Errorf("closing log file: %w", err)
	}
	return nil
}

// Section starts a named section header and returns a stop function.
func (l *Logger) Section(title string) func() {
	return l.SectionContext(context.Background(), title)
}

// SectionContext starts a named section header with context and returns a stop function.
func (l *Logger) SectionContext(ctx context.Context, title string) func() {
	lines := strings.Repeat("═", 56)
	if l.interactive {
		l.logStructured(ctx, "SECTION", slog.LevelInfo, title, slog.String("stream", "stdout"))
		l.writeRaw(l.stdout, fmt.Sprintf("\n%s%s%s", colorBlue, lines, colorReset))
		l.writeRaw(l.stdout, fmt.Sprintf("%s%s%s", colorBlue, title, colorReset))
		l.startSpinner(ctx, title)
		return l.stopSpinner
	}

	l.InfoContext(ctx, lines)
	l.InfoContext(ctx, title)
	return func() {}
}

// Info logs a message at info level.
func (l *Logger) Info(message string) {
	l.InfoContext(context.Background(), message)
}

// InfoContext logs a message at info level with context.
func (l *Logger) InfoContext(ctx context.Context, message string) {
	l.log(ctx, "INFO", slog.LevelInfo, message, l.stdout, "info", colorBlue)
}

// Debug logs a message at debug level.
func (l *Logger) Debug(message string) {
	l.DebugContext(context.Background(), message)
}

// DebugContext logs a message at debug level with context.
func (l *Logger) DebugContext(ctx context.Context, message string) {
	l.log(ctx, "DEBUG", slog.LevelDebug, message, l.stdout, "dbg", colorGray)
}

// Warn logs a message at warn level.
func (l *Logger) Warn(message string) {
	l.WarnContext(context.Background(), message)
}

// WarnContext logs a message at warn level with context.
func (l *Logger) WarnContext(ctx context.Context, message string) {
	l.log(ctx, "WARN", slog.LevelWarn, message, l.stderr, "warn", colorYellow)
}

// WarnWithErr logs a warning message along with an error.
func (l *Logger) WarnWithErr(message string, err error) {
	l.WarnContextWithErr(context.Background(), message, err)
}

// WarnContextWithErr logs a warning message with context and an error.
func (l *Logger) WarnContextWithErr(ctx context.Context, message string, err error) {
	if err == nil {
		l.WarnContext(ctx, message)
		return
	}
	l.log(ctx, "WARN", slog.LevelWarn, message, l.stderr, "warn", colorYellow, slog.String("err", err.Error()))
}

// ErrorWithErr logs an error message along with an error.
func (l *Logger) ErrorWithErr(message string, err error) {
	l.ErrorContextWithErr(context.Background(), message, err)
}

// ErrorContextWithErr logs an error message with context and an error.
func (l *Logger) ErrorContextWithErr(ctx context.Context, message string, err error) {
	if err == nil {
		l.log(ctx, "ERROR", slog.LevelError, message, l.stderr, "err", colorRed)
		return
	}
	l.log(ctx, "ERROR", slog.LevelError, message, l.stderr, "err", colorRed, slog.String("err", err.Error()))
}

// Success logs a success message at info level.
func (l *Logger) Success(message string) {
	l.SuccessContext(context.Background(), message)
}

// SuccessContext logs a success message at info level with context.
func (l *Logger) SuccessContext(ctx context.Context, message string) {
	l.log(ctx, "OK", slog.LevelInfo, message, l.stdout, "ok", colorGreen)
}

// RawOutput logs raw command output lines, stripping blank lines.
func (l *Logger) RawOutput(output string) {
	l.RawOutputContext(context.Background(), output)
}

// RawOutputContext logs raw command output lines with the given context.
func (l *Logger) RawOutputContext(ctx context.Context, output string) {
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		l.logStructured(ctx, "OUTPUT", slog.LevelDebug, line, slog.String("source", "command"))
		if !l.interactive {
			l.log(ctx, "OUTPUT", slog.LevelDebug, line, l.stdout, "out", colorGray)
			continue
		}
		if suppressInteractiveRawLine(line) {
			continue
		}
		l.write(l.stdout, fmt.Sprintf("%s- %s%s", colorGray, line, colorReset))
	}
}

func suppressInteractiveRawLine(line string) bool {
	return strings.HasPrefix(line, "go: downloading ") || strings.HasPrefix(line, "go: finding module for package ")
}

func (l *Logger) log(
	ctx context.Context,
	levelLabel string,
	level slog.Level,
	message string,
	stream io.Writer,
	icon string,
	color string,
	attributes ...slog.Attr,
) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	timestamp := clock.Now().Format(displayTimestampFormat)
	attrs := []slog.Attr{
		slog.String("stream", streamName(stream)),
		slog.String("timestamp", timestamp),
	}
	attrs = append(attrs, attributes...)
	l.logStructured(ctx, levelLabel, level, message, attrs...)

	humanLine := fmt.Sprintf("%s[%s] %s%s", color, levelLabel, message, colorReset)
	if l.interactive {
		humanLine = fmt.Sprintf("%s %s%s", icon, message, colorReset)
	}

	l.write(stream, humanLine)
}

func (l *Logger) logStructured(ctx context.Context, levelLabel string, level slog.Level, message string, attributes ...slog.Attr) {
	if l.slogLogger == nil {
		return
	}

	attrs := make([]slog.Attr, 0, 2+len(attributes))
	attrs = append(attrs, slog.String("level_label", levelLabel), slog.String("log_file", l.filePath))
	attrs = append(attrs, attributes...)
	l.slogLogger.LogAttrs(ctx, level, message, attrs...)
}

func (l *Logger) write(stream io.Writer, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.interactive {
		l.stopSpinnerLocked()
	}
	l.writeLine(stream, message)
}

func (l *Logger) writeRaw(stream io.Writer, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writeLine(stream, message)
}

func (l *Logger) writeLine(stream io.Writer, message string) {
	_, _ = fmt.Fprintln(stream, message)
}

// PrintTTYLine writes one human-facing line without log-level prefixes (no "info"/"warn"
// icons). When interactive and ttyStyled is non-empty and NO_COLOR is unset, ttyStyled is
// printed; otherwise plain is printed. The plain text is always recorded in the JSON log.
func (l *Logger) PrintTTYLine(plain string, ttyStyled string) {
	l.PrintTTYLineContext(context.Background(), plain, ttyStyled)
}

// PrintTTYLineContext writes one human-facing line and records the plain text in structured logs.
func (l *Logger) PrintTTYLineContext(ctx context.Context, plain string, ttyStyled string) {
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return
	}

	l.logStructured(ctx, "TTY", slog.LevelInfo, plain, slog.String("stream", "stdout"))
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.interactive {
		l.stopSpinnerLocked()
	}
	out := plain
	if l.interactive && ttyStyled != "" && os.Getenv("NO_COLOR") == "" {
		out = ttyStyled
	}
	_, _ = fmt.Fprintln(l.stdout, out)
}

func (l *Logger) startSpinner(ctx context.Context, message string) {
	if !l.interactive {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.spinProgram != nil {
		return
	}

	m := sectionSpinnerModel{
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		title:   message,
	}
	spinProgram := tea.NewProgram(
		m,
		tea.WithOutput(l.stdout),
		tea.WithInput(nil),
		tea.WithoutSignalHandler(),
		tea.WithoutRenderer(),
	)
	l.spinProgram = spinProgram
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				l.ErrorContextWithErr(ctx, "spinner crashed", fmt.Errorf("panic: %v", recovered))
			}
		}()
		_, _ = spinProgram.Run()
	}()
}

func (l *Logger) stopSpinner() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerLocked()
}

func (l *Logger) stopSpinnerLocked() {
	if l.spinProgram == nil {
		return
	}

	l.spinProgram.Quit()
	l.spinProgram.Wait()
	l.spinProgram = nil
}

func isTTY(output *os.File) bool {
	info, err := output.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func streamName(stream io.Writer) string {
	if stream == os.Stdout {
		return "stdout"
	}
	if stream == os.Stderr {
		return "stderr"
	}
	return "other"
}

// WithRun returns a child of ctx carrying a per-run correlation identity (a
// trace id and span id), minting one when ctx does not already carry it. Every
// record logged with the returned context then carries trace_id and span_id,
// and RunID exposes the durable id for the notification banner.
func WithRun(ctx context.Context) context.Context {
	runCtx, _ := correlation.Ensure(ctx, "")
	return runCtx
}

// RunID returns the durable correlation id (the trace id) carried by ctx, or
// the empty string when ctx carries no run identity.
func RunID(ctx context.Context) string {
	return string(correlation.FromContext(ctx).TraceID)
}

// Notify appends a notification entry for the shell to render on the next
// prompt. The on-disk format is timestamp|level|logPath|runID|message; runID is
// the durable correlation id so a reader can grep the run's lines in logPath.
func Notify(level, message, logPath, runID string) error {
	notifyPath := filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "notifications")
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(notifyPath)), 0o755); err != nil {
		slog.Error("telemetry: Notify: creating notification directory", "err", err)
		return fmt.Errorf("creating notification directory: %w", err)
	}
	timestamp := clock.Now().Format(displayTimestampFormat)
	return appendLine(notifyPath, fmt.Sprintf("%s|%s|%s|%s|%s", timestamp, level, logPath, runID, message))
}

func appendLine(path string, line string) error {
	cleanPath := filepath.Clean(path)
	file, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Error("telemetry: appendLine: opening file", "path", cleanPath, "err", err)
		return fmt.Errorf("opening notification file: %w", err)
	}
	defer file.Close()
	if _, err = fmt.Fprintln(file, line); err != nil {
		slog.Error("telemetry: appendLine: writing notification", "err", err)
		return fmt.Errorf("writing notification: %w", err)
	}
	return nil
}

type sectionSpinnerModel struct {
	spinner spinner.Model
	title   string
}

func (m sectionSpinnerModel) Init() tea.Cmd {
	return func() tea.Msg {
		return m.spinner.Tick()
	}
}

func (m sectionSpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.spinner.Update(msg)
	m.spinner = updated
	return m, cmd
}

func (m sectionSpinnerModel) View() tea.View {
	return tea.NewView(fmt.Sprintf("%s %s", m.spinner.View(), m.title))
}
