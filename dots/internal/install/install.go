// Package install implements dotfiles installation routines.
package install

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync"
	"goodkind.io/.dotfiles/internal/telemetry"
)

var installLogger *telemetry.Logger

type installFlag string

const (
	flagHelp             installFlag = "--help"
	flagHelpShort        installFlag = "-h"
	flagUseDefaults      installFlag = "--use-defaults"
	flagUseDefaultsShort installFlag = "-d"
	flagRepair           installFlag = "--repair"
	flagQuick            installFlag = "--quick"
	flagSkipGit          installFlag = "--skip-git"
	flagSkipNetwork      installFlag = "--skip-network"
)

// Run executes the dotfiles install workflow with the given arguments.
func Run(ctx context.Context, args ...string) error {
	logPath := filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "install.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		slog.WarnContext(ctx, "creating logger", "err", err)
		return fmt.Errorf("creating logger: %w", err)
	}
	installLogger = logger
	defer logger.Close()
	defer func() {
		installLogger = nil
		runner.SetLogger(nil)
	}()
	runner.SetLogger(logger)
	_ = os.Setenv("DOTFILES_LOG", logPath)
	done := logger.SectionContext(ctx, "Install")
	defer done()

	useDefaults := false
	syncOpts := sync.Options{
		RepairMode:     false,
		QuickMode:      false,
		SkipGit:        false,
		SkipNetwork:    false,
		SkipCursorSync: false,
		DryRun:         false,
		UseDefaults:    false,
	}

	for _, arg := range args {
		switch installFlag(arg) {
		case flagHelp, flagHelpShort:
			printInstallUsage(ctx)
			return nil
		case flagUseDefaults, flagUseDefaultsShort:
			useDefaults = true
			syncOpts.UseDefaults = true
		case flagRepair:
			syncOpts.RepairMode = true
		case flagQuick:
			syncOpts.QuickMode = true
		case flagSkipGit:
			syncOpts.SkipGit = true
		case flagSkipNetwork:
			syncOpts.SkipNetwork = true
		default:
			return fmt.Errorf("unsupported install flag: %s", arg)
		}
	}
	syncOpts.UseDefaults = useDefaults

	if err := createSocketDir(); err != nil {
		return err
	}
	if err := configureGit(ctx, useDefaults); err != nil {
		return err
	}
	if err := sync.Run(ctx, syncOpts); err != nil {
		slog.WarnContext(ctx, "running sync", "err", err)
		return fmt.Errorf("running sync: %w", err)
	}
	if err := ensureLoginShell(ctx); err != nil {
		return err
	}
	return nil
}

func printInstallUsage(ctx context.Context) {
	logInfo(ctx, "Usage: dots install [--use-defaults] [--quick] [--skip-git] [--skip-network] [--repair]")
}

func createSocketDir() error {
	if err := os.MkdirAll(filepath.Clean(filepath.Join(os.Getenv("HOME"), ".ssh", "sockets")), 0o700); err != nil {
		slog.Warn("creating socket directory", "err", err)
		return fmt.Errorf("creating socket directory: %w", err)
	}
	return nil
}

func configureGit(ctx context.Context, useDefaults bool) error {
	libPath := filepath.Join(dotfilesRoot(), "lib", ".gitconfig_incl")
	if err := cmdexec.Run(ctx, "git", "config", "--global", "include.path", libPath); err != nil {
		slog.WarnContext(ctx, "Failed to set git include path", "err", err)
		installLogger.WarnContextWithErr(ctx, "Failed to set git include path", err)
		slog.WarnContext(ctx, "set git include path", "err", err)
		return fmt.Errorf("set git include path: %w", err)
	}

	name := gitConfig(ctx, "user.name")
	if name == "" {
		if err := promptAndSetGitConfig(ctx, useDefaults, "Skipping git user.name (use defaults mode)", "Enter your name for git (First Last): ", "user.name"); err != nil {
			return err
		}
	}

	email := gitConfig(ctx, "user.email")
	if email == "" {
		if err := promptAndSetGitConfig(ctx, useDefaults, "Skipping git user email (use defaults mode)", "Enter your git email: ", "user.email"); err != nil {
			return err
		}
	}

	currentCommand := gitConfig(ctx, "gpg.ssh.defaultKeyCommand")
	if currentCommand != "" {
		return nil
	}

	sshAddOutput, _ := cmdexec.OutputTrimmed(ctx, "ssh-add", "-L")
	agentDone, agentErr := setSigningKeyFromAgent(ctx, sshAddOutput)
	if agentErr != nil {
		return agentErr
	}
	if agentDone {
		return nil
	}

	publicKeyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519.pub")
	if publicKeyRaw, err := os.ReadFile(filepath.Clean(publicKeyPath)); err == nil {
		line := strings.TrimSpace(string(publicKeyRaw))
		if line != "" {
			if err := cmdexec.Run(ctx, "git", "config", "--global", "user.signingKey", "key::"+line); err != nil {
				slog.WarnContext(ctx, "running git config", "err", err)
				return fmt.Errorf("running git config: %w", err)
			}
			return nil
		}
	}

	if useDefaults {
		logInfo(ctx, "Skipping SSH key setup (use defaults mode)")
		return nil
	}

	keyPath := readLine(ctx, "Enter path to your SSH public key (or leave empty to skip): ")
	if keyPath == "" {
		return nil
	}
	keyRaw, err := os.ReadFile(strings.TrimSpace(keyPath))
	if err != nil {
		slog.WarnContext(ctx, "Failed to read SSH public key", "err", err)
		installLogger.WarnContextWithErr(ctx, "Failed to read SSH public key", err)
		slog.WarnContext(ctx, "read ssh public key", "err", err)
		return fmt.Errorf("read ssh public key: %w", err)
	}
	line := strings.TrimSpace(string(keyRaw))
	if line == "" {
		return nil
	}
	if err := cmdexec.Run(ctx, "git", "config", "--global", "user.signingKey", "key::"+line); err != nil {
		slog.WarnContext(ctx, "running git config", "err", err)
		return fmt.Errorf("running git config: %w", err)
	}
	return nil
}

func setSigningKeyFromAgent(ctx context.Context, sshAddOutput string) (bool, error) {
	for line := range strings.SplitSeq(strings.TrimSpace(sshAddOutput), "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(line, "ed25519") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		keyID := fields[1]
		if err := cmdexec.Run(ctx, "git", "config", "--global", "gpg.ssh.defaultKeyCommand", fmt.Sprintf("ssh-add -L | grep '%s'", keyID)); err != nil {
			slog.WarnContext(ctx, "running git config", "err", err)
			return false, fmt.Errorf("running git config: %w", err)
		}
		if err := cmdexec.Run(ctx, "git", "config", "--global", "user.signingKey", "key::"+line); err != nil {
			slog.WarnContext(ctx, "running git config", "err", err)
			return false, fmt.Errorf("running git config: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func ensureLoginShell(ctx context.Context) error {
	zshPath, err := runner.LookPath("zsh")
	if err != nil {
		logInfo(ctx, "Skipping login shell change (zsh not found)")
		slog.WarnContext(ctx, "look up zsh", "err", err)
		return fmt.Errorf("look up zsh: %w", err)
	}
	currentShell, currentErr := detectCurrentShell(ctx)
	if currentErr != nil {
		currentShell = "unknown"
	}
	logInfof(ctx, "Current login shell: %s", currentShell)
	logInfof(ctx, "Target zsh path: %s", zshPath)

	isZsh := currentShell == zshPath
	if !isZsh && strings.HasSuffix(filepath.Base(currentShell), "zsh") {
		isZsh = true
	}
	if isZsh {
		logInfo(ctx, "Shell is already zsh")
		return nil
	}

	if os.Getenv("GITHUB_ACTIONS") == "true" {
		logInfo(ctx, "Skipping shell change in CI")
		return nil
	}
	if err := cmdexec.Run(ctx, "chsh", "-s", zshPath); err != nil {
		slog.WarnContext(ctx, "Failed to change login shell", "err", err)
		installLogger.WarnContextWithErr(ctx, "Failed to change login shell", err)
		slog.WarnContext(ctx, "change login shell", "err", err)
		return fmt.Errorf("change login shell: %w", err)
	}
	logInfo(ctx, "Login shell changed to zsh")
	return nil
}

func detectCurrentShell(ctx context.Context) (string, error) {
	if runtime.GOOS == "darwin" {
		userInfo, err := user.Current()
		if err != nil {
			slog.WarnContext(ctx, "getting current user", "err", err)
			return "", fmt.Errorf("getting current user: %w", err)
		}
		output, err := cmdexec.OutputTrimmed(ctx, "dscl", ".", "-read", "/Users/"+userInfo.Username, "UserShell")
		if err != nil {
			slog.WarnContext(ctx, "running dscl", "err", err)
			return "", fmt.Errorf("running dscl: %w", err)
		}
		for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
			line = strings.TrimSpace(line)
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "UserShell:" {
				return fields[1], nil
			}
		}
		return "", nil
	}

	output, err := cmdexec.OutputTrimmed(ctx, "getent", "passwd", os.Getenv("USER"))
	if err != nil {
		slog.WarnContext(ctx, "running getent", "err", err)
		return "", fmt.Errorf("running getent: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(output), ":")
	if len(parts) < 7 {
		return "", nil
	}
	return parts[6], nil
}

func promptAndSetGitConfig(ctx context.Context, useDefaults bool, skipMsg, prompt, key string) error {
	if useDefaults {
		logInfo(ctx, skipMsg)
		return nil
	}
	input := readLine(ctx, prompt)
	if input == "" {
		return nil
	}
	if err := cmdexec.Run(ctx, "git", "config", "--global", key, input); err != nil {
		slog.WarnContext(ctx, "running git config", "err", err)
		return fmt.Errorf("running git config: %w", err)
	}
	return nil
}

func gitConfig(ctx context.Context, name string) string {
	output, _ := cmdexec.OutputTrimmed(ctx, "git", "config", "--global", name)
	return output
}

func readLine(ctx context.Context, prompt string) string {
	logInfo(ctx, prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func logInfo(ctx context.Context, message string) {
	if installLogger != nil {
		installLogger.InfoContext(ctx, message)
	}
}

func logInfof(ctx context.Context, format string, args ...string) {
	if installLogger != nil {
		installLogger.InfoContext(ctx, formatString(format, args...))
	}
}

func formatString(format string, args ...string) string {
	formatted := format
	for _, arg := range args {
		formatted = strings.Replace(formatted, "%s", arg, 1)
	}
	return formatted
}

func dotfilesRoot() string {
	if value := os.Getenv("DOTDOTFILES"); value != "" {
		return value
	}
	return filepath.Join(os.Getenv("HOME"), ".dotfiles")
}
