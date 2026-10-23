// Package homebrew runs package maintenance and reports individual failures.
package homebrew

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"goodkind.io/.dotfiles/internal/telemetry"
)

// Result reports failures from a completed maintenance attempt.
type Result struct {
	Failures int
}

// PartialError reports a completed attempt with package failures.
type PartialError struct {
	Failures int
}

// Error reports the number of failed maintenance operations.
func (failure PartialError) Error() string {
	return fmt.Sprintf("Homebrew maintenance completed with %d errors", failure.Failures)
}

type outdatedPackage struct {
	Name   string `json:"name"`
	Pinned bool   `json:"pinned"`
}

type outdatedPackages struct {
	Formulae *[]outdatedPackage `json:"formulae"`
	Casks    *[]outdatedPackage `json:"casks"`
}

type maintenance struct {
	logger        *telemetry.Logger
	notifyLogPath string
	result        Result
}

const notificationDiagnosticLimit = 600

// Run upgrades each outdated package and continues after package failures.
// Cancellation returns an error before the attempt completes.
func Run(ctx context.Context, logger *telemetry.Logger, notifyLogPath string) (Result, error) {
	current := maintenance{logger: logger, notifyLogPath: notifyLogPath, result: Result{Failures: 0}}
	current.runCommand(ctx, "Homebrew update", "update")
	if err := ctx.Err(); err != nil {
		slog.WarnContext(ctx, "Homebrew maintenance interrupted", "err", err)
		return current.result, fmt.Errorf("running Homebrew maintenance: %w", err)
	}
	stdout, diagnostic, err := current.output(ctx, "outdated", "--json=v2", "--greedy")
	if err != nil {
		current.recordFailure(ctx, "Homebrew package listing", diagnostic, err)
	} else {
		packages, decodeErr := decodeOutdated(stdout)
		if decodeErr != nil {
			current.recordFailure(ctx, "Homebrew package listing", stdout, decodeErr)
		} else {
			current.upgradePackages(ctx, *packages.Formulae, "--formula")
			current.upgradePackages(ctx, *packages.Casks, "--cask", "--greedy")
		}
	}
	if err := ctx.Err(); err != nil {
		return current.result, fmt.Errorf("running Homebrew maintenance: %w", err)
	}
	current.runCommand(ctx, "Homebrew cleanup", "cleanup", "--prune=all")
	if err := ctx.Err(); err != nil {
		return current.result, fmt.Errorf("running Homebrew maintenance: %w", err)
	}
	return current.result, nil
}

func decodeOutdated(output string) (outdatedPackages, error) {
	var packages outdatedPackages
	if err := json.Unmarshal([]byte(output), &packages); err != nil {
		slog.Warn("decoding Homebrew outdated packages", "err", err)
		return packages, fmt.Errorf("decoding outdated packages: %w", err)
	}
	if packages.Formulae == nil || packages.Casks == nil {
		return packages, fmt.Errorf("listing Homebrew packages: response must include formulae and casks arrays")
	}
	for _, group := range [][]outdatedPackage{*packages.Formulae, *packages.Casks} {
		for _, item := range group {
			if strings.TrimSpace(item.Name) == "" {
				return packages, fmt.Errorf("listing Homebrew packages: response contains a package without a name")
			}
		}
	}
	return packages, nil
}

func (current *maintenance) upgradePackages(ctx context.Context, packages []outdatedPackage, options ...string) {
	for _, item := range packages {
		if ctx.Err() != nil {
			return
		}
		if item.Pinned {
			continue
		}
		arguments := append([]string{"upgrade", "--no-ask"}, options...)
		arguments = append(arguments, item.Name)
		current.runCommand(ctx, "Homebrew package "+item.Name, arguments...)
	}
}

func (current *maintenance) runCommand(ctx context.Context, subject string, arguments ...string) {
	_, diagnostic, err := current.output(ctx, arguments...)
	if err != nil && ctx.Err() == nil {
		current.recordFailure(ctx, subject, diagnostic, err)
	}
}

func (current *maintenance) output(ctx context.Context, arguments ...string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, "brew", arguments...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	diagnostic := stdout.String() + stderr.String()
	current.logger.RawOutputContext(ctx, diagnostic)
	if err != nil {
		slog.WarnContext(ctx, "Homebrew command failed", "args", arguments, "err", err)
	}
	return stdout.String(), diagnostic, err
}

func (current *maintenance) recordFailure(ctx context.Context, subject, diagnostic string, err error) {
	current.result.Failures++
	message := subject + " failed: " + err.Error()
	if strings.TrimSpace(diagnostic) != "" {
		message += ": " + failureDiagnostic(diagnostic)
	}
	current.logger.WarnContextWithErr(ctx, subject+" failed", err)
	if notifyErr := telemetry.Notify("error", message, current.notifyLogPath, telemetry.RunID(ctx)); notifyErr != nil {
		current.logger.WarnContextWithErr(ctx, "notification write failed", notifyErr)
	}
}

func failureDiagnostic(output string) string {
	lines := strings.Split(ansi.Strip(output), "\n")
	selected := make([]string, 0, 3)
	lastLine := ""
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			continue
		}
		lastLine = line
		lower := strings.ToLower(line)
		if len(selected) == 0 && !strings.Contains(lower, "error:") && !strings.Contains(lower, "exception") && !strings.Contains(lower, "failed") {
			continue
		}
		selected = append(selected, line)
		if len(selected) == 3 {
			break
		}
	}
	diagnostic := lastLine
	if len(selected) > 0 {
		diagnostic = strings.Join(selected, " ")
	}
	characters := []rune(diagnostic)
	if len(characters) > notificationDiagnosticLimit {
		diagnostic = string(characters[:notificationDiagnosticLimit]) + " (full diagnostic in log)"
	}
	return diagnostic
}
