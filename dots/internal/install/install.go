// Package install implements dotfiles installation routines.
package install

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

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
	flagStrict           installFlag = "--strict"
	dotfilesRepository   string      = "https://github.com/agoodkind/.dotfiles.git"
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
	logInstallSummary()

	lockFile, flockFdInt, releaseStatus, alreadyRunning, err := acquireInstallLock()
	if err != nil {
		return err
	}
	if alreadyRunning {
		logTTYLine("Another dotfiles install is already running in a different terminal.")
		return nil
	}
	defer releaseStatus()
	defer lockFile.Close()
	defer syscall.Flock(flockFdInt, syscall.LOCK_UN)

	useDefaults := false
	syncOpts := sync.Options{
		RepairMode:     false,
		QuickMode:      false,
		SkipGit:        false,
		SkipNetwork:    false,
		SkipCursorSync: false,
		DryRun:         false,
		UseDefaults:    false,
		StrictMode:     false,
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
		case flagStrict:
			syncOpts.StrictMode = true
		default:
			return fmt.Errorf("unsupported install flag: %s", arg)
		}
	}
	syncOpts.UseDefaults = useDefaults

	if err := createSocketDir(); err != nil {
		return err
	}
	configuredGit := false
	if runner.HasCommand("git") {
		if err := configureGit(ctx, useDefaults); err != nil {
			return err
		}
		configuredGit = true
	} else {
		logTTYLine("Git is not installed yet, so the installer will finish package setup first and ask for git identity later.")
	}
	if err := sync.Run(ctx, syncOpts); err != nil {
		slog.WarnContext(ctx, "running sync", "err", err)
		return fmt.Errorf("running sync: %w", err)
	}
	if err := ensureManagedRepository(ctx); err != nil {
		installLogger.WarnContextWithErr(ctx, "Finishing archive bootstrap failed", err)
	}
	if !configuredGit && runner.HasCommand("git") {
		if err := configureGit(ctx, useDefaults); err != nil {
			return err
		}
	}
	ensureLoginShell(ctx)
	return nil
}

func printInstallUsage(ctx context.Context) {
	logInfo(ctx, "Usage: dots install [--use-defaults] [--quick] [--skip-git] [--skip-network] [--repair] [--strict]")
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

	keyPath, foundKeys := resolveSigningKeyPath(ctx, useDefaults)
	if keyPath == "" {
		if !useDefaults && !foundKeys {
			logTTYLine("No SSH public key was found in ~/.ssh, so git SSH signing will stay unset for now.")
			logTTYLine("Run ssh-keygen after install, then rerun ./install.sh to enable signing.")
		}
		return nil
	}
	keyRaw, err := os.ReadFile(filepath.Clean(strings.TrimSpace(keyPath)))
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
	logInfof(ctx, "Using SSH public key for git signing: %s", displayPath(keyPath))
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

func ensureLoginShell(ctx context.Context) {
	zshPath, err := runner.LookPath("zsh")
	if err != nil {
		logInfo(ctx, "Skipping login shell change (zsh not found)")
		slog.WarnContext(ctx, "look up zsh", "err", err)
		return
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
		return
	}

	if os.Getenv("GITHUB_ACTIONS") == "true" {
		logInfo(ctx, "Skipping shell change in CI")
		return
	}
	if err := cmdexec.Run(ctx, "chsh", "-s", zshPath); err != nil {
		slog.WarnContext(ctx, "Failed to change login shell", "err", err)
		installLogger.WarnContextWithErr(ctx, "Failed to change login shell", err)
		slog.WarnContext(ctx, "change login shell", "err", err)
		return
	}
	logInfo(ctx, "Login shell changed to zsh")
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
		return "", nil
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
	logPrompt(ctx, prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func logPrompt(_ context.Context, prompt string) {
	if installLogger == nil {
		fmt.Fprintln(os.Stdout, prompt)
		fmt.Fprint(os.Stdout, "> ")
		return
	}
	installLogger.PrintTTYLine(prompt, "\x1b[1;36m›\x1b[0m "+prompt)
	_, _ = fmt.Fprint(os.Stdout, "> ")
}

func logTTYLine(message string) {
	if installLogger != nil {
		installLogger.PrintTTYLine(message, "\x1b[1;36m•\x1b[0m "+message)
		return
	}
	fmt.Fprintln(os.Stdout, message)
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

func logInstallSummary() {
	logTTYLine("The installer will set up this machine, sync your dotfiles, and configure git as soon as git is available.")
}

func acquireInstallLock() (*os.File, int, func(), bool, error) {
	cacheDir := filepath.Clean(filepath.Join(os.Getenv("HOME"), ".cache"))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, 0, nil, false, fmt.Errorf("creating cache directory: %w", err)
	}
	lockPath := filepath.Join(cacheDir, "dotfiles_install.flock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return nil, 0, nil, false, fmt.Errorf("opening install lock file: %w", err)
	}
	flockFd := lockFile.Fd()
	if uint64(flockFd) > uint64(^uint(0)>>1) {
		_ = lockFile.Close()
		return nil, 0, nil, false, fmt.Errorf("lock file descriptor %d exceeds int bounds", flockFd)
	}
	flockFdInt := int(flockFd)
	if err := syscall.Flock(flockFdInt, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lockFile.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, 0, nil, true, nil
		}
		return nil, 0, nil, false, fmt.Errorf("acquiring install lock: %w", err)
	}
	statusDir := filepath.Join(cacheDir, "dotfiles_install.lock")
	if err := os.MkdirAll(statusDir, 0o755); err != nil {
		_ = syscall.Flock(flockFdInt, syscall.LOCK_UN)
		_ = lockFile.Close()
		return nil, 0, nil, false, fmt.Errorf("creating install status directory: %w", err)
	}
	release := func() {
		_ = os.RemoveAll(statusDir)
	}
	return lockFile, flockFdInt, release, false, nil
}

func ensureManagedRepository(ctx context.Context) error {
	dotfiles := dotfilesRoot()
	if _, err := os.Stat(filepath.Clean(filepath.Join(dotfiles, ".git"))); err == nil {
		return nil
	}
	if !runner.HasCommand("git") {
		logTTYLine("Git is still unavailable, so this archive install will stay unmanaged until git is installed.")
		return nil
	}
	logTTYLine("Finishing the archive bootstrap by turning " + displayPath(dotfiles) + " into a git checkout.")
	if err := cmdexec.RunWithLogger(ctx, installLogger, "git", "-C", dotfiles, "init", "-b", "main"); err != nil {
		return fmt.Errorf("initializing git checkout: %w", err)
	}
	currentOrigin, _ := cmdexec.OutputTrimmed(ctx, "git", "-C", dotfiles, "config", "--local", "--get", "remote.origin.url")
	switch {
	case currentOrigin == "":
		if err := cmdexec.Run(ctx, "git", "-C", dotfiles, "remote", "add", "origin", dotfilesRepository); err != nil {
			return fmt.Errorf("adding git remote: %w", err)
		}
	case currentOrigin != dotfilesRepository:
		if err := cmdexec.Run(ctx, "git", "-C", dotfiles, "remote", "set-url", "origin", dotfilesRepository); err != nil {
			return fmt.Errorf("updating git remote: %w", err)
		}
	}
	if err := cmdexec.RunWithLogger(ctx, installLogger, "git", "-C", dotfiles, "fetch", "origin", "main"); err != nil {
		return fmt.Errorf("fetching dotfiles repository: %w", err)
	}
	if err := cmdexec.RunWithLogger(ctx, installLogger, "git", "-C", dotfiles, "reset", "--hard", "origin/main"); err != nil {
		return fmt.Errorf("resetting dotfiles repository: %w", err)
	}
	if err := cmdexec.RunWithLogger(ctx, installLogger, "git", "-C", dotfiles, "submodule", "update", "--init", "--recursive"); err != nil {
		return fmt.Errorf("initializing submodules: %w", err)
	}
	logTTYLine("The dotfiles checkout can now update itself with git.")
	return nil
}

func resolveSigningKeyPath(ctx context.Context, useDefaults bool) (string, bool) {
	candidates := sshPublicKeyCandidates()
	if len(candidates) == 0 {
		if useDefaults {
			logInfo(ctx, "Skipping SSH key setup (use defaults mode)")
		}
		return "", false
	}
	if useDefaults || len(candidates) == 1 {
		return candidates[0], true
	}
	logTTYLine("Choose an SSH public key for git signing, or press Enter to skip.")
	for index, candidate := range candidates {
		logTTYLine(formatString("%s. %s", strconv.Itoa(index+1), displayPath(candidate)))
	}
	choice := readLine(ctx, "SSH key number")
	if choice == "" {
		return "", true
	}
	selected, err := strconv.Atoi(choice)
	if err != nil || selected < 1 || selected > len(candidates) {
		logTTYLine("That SSH key selection was not valid, so git signing will stay unset for now.")
		return "", true
	}
	return candidates[selected-1], true
}

func sshPublicKeyCandidates() []string {
	home := os.Getenv("HOME")
	defaultKey := filepath.Join(home, ".ssh", "id_ed25519.pub")
	candidates := make([]string, 0, 4)
	seen := make(map[string]struct{})
	addCandidate := func(path string) {
		if _, ok := seen[path]; ok {
			return
		}
		if info, err := os.Stat(filepath.Clean(path)); err == nil && !info.IsDir() {
			seen[path] = struct{}{}
			candidates = append(candidates, path)
		}
	}
	addCandidate(defaultKey)
	matches, _ := filepath.Glob(filepath.Join(home, ".ssh", "*.pub"))
	for _, match := range matches {
		addCandidate(match)
	}
	return candidates
}

func displayPath(path string) string {
	home := os.Getenv("HOME")
	if home != "" && strings.HasPrefix(path, home+"/") {
		return "~/" + strings.TrimPrefix(path, home+"/")
	}
	return path
}

func dotfilesRoot() string {
	if value := os.Getenv("DOTDOTFILES"); value != "" {
		return value
	}
	return filepath.Join(os.Getenv("HOME"), ".dotfiles")
}
