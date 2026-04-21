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
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

const (
	colorReset  = "\x1b[0m"
	colorBlue   = "\x1b[34m"
	colorGreen  = "\x1b[32m"
	colorYellow = "\x1b[33m"
	colorRed    = "\x1b[31m"
	colorGray   = "\x1b[90m"
)

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

func NewLogger(logPath string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	fileHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	})

	return &Logger{
		filePath:    logPath,
		logFile:     logFile,
		slogLogger:  slog.New(fileHandler),
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		interactive: isTTY(os.Stdout),
	}, nil
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopSpinnerLocked()
	return l.logFile.Close()
}

func (l *Logger) Section(title string) func() {
	lines := strings.Repeat("═", 56)
	if l.interactive {
		l.logStructured("SECTION", slog.LevelInfo, title, "stream", "stdout")
		l.writeRaw(l.stdout, fmt.Sprintf("\n%s%s%s", colorBlue, lines, colorReset))
		l.writeRaw(l.stdout, fmt.Sprintf("%s%s%s", colorBlue, title, colorReset))
		l.startSpinner(title)
		return l.stopSpinner
	}

	l.Info(lines)
	l.Info(title)
	return func() {}
}

func (l *Logger) Info(message string) {
	l.log("INFO", slog.LevelInfo, message, l.stdout, "info", colorBlue)
}

func (l *Logger) Debug(message string) {
	l.log("DEBUG", slog.LevelDebug, message, l.stdout, "dbg", colorGray)
}

func (l *Logger) Warn(message string) {
	l.log("WARN", slog.LevelWarn, message, l.stderr, "warn", colorYellow)
}

func (l *Logger) Error(message string) {
	l.log("ERROR", slog.LevelError, message, l.stderr, "err", colorRed)
}

func (l *Logger) Success(message string) {
	l.log("OK", slog.LevelInfo, message, l.stdout, "ok", colorGreen)
}

func (l *Logger) RawOutput(output string) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		l.logStructured("OUTPUT", slog.LevelDebug, line, "source", "command")
		if !l.interactive {
			l.log("OUTPUT", slog.LevelDebug, line, l.stdout, "out", colorGray)
			continue
		}
		l.write(l.stdout, fmt.Sprintf("%s- %s%s", colorGray, line, colorReset))
	}
}

func (l *Logger) log(levelLabel string, level slog.Level, message string, stream io.Writer, icon string, color string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	l.logStructured(levelLabel, level, message, "stream", streamName(stream), "timestamp", timestamp)

	humanLine := fmt.Sprintf("%s[%s] %s%s", color, levelLabel, message, colorReset)
	if l.interactive {
		humanLine = fmt.Sprintf("%s %s%s", icon, message, colorReset)
	}

	l.write(stream, humanLine)
}

func (l *Logger) logStructured(levelLabel string, level slog.Level, message string, attributes ...any) {
	if l.slogLogger == nil {
		return
	}

	attrs := make([]any, 0, 2+len(attributes))
	attrs = append(attrs, slog.String("level_label", levelLabel), slog.String("log_file", l.filePath))
	attrs = append(attrs, attributes...)
	l.slogLogger.Log(context.Background(), level, message, attrs...)
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
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return
	}

	l.logStructured("TTY", slog.LevelInfo, plain, "stream", "stdout")
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

func (l *Logger) startSpinner(message string) {
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

func Notify(level, message, logPath string) error {
	notifyPath := filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "notifications")
	if err := os.MkdirAll(filepath.Dir(notifyPath), 0o755); err != nil {
		return err
	}
	return appendLine(notifyPath, fmt.Sprintf("%s|%s|%s", level, logPath, message))
}

func appendLine(path string, line string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintln(file, line)
	return err
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
