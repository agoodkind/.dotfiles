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
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/telemetry"
)

var installLogger *telemetry.Logger

type pendingGitConfig struct {
	name                    string
	email                   string
	signingKey              string
	gpgSshDefaultKeyCommand string
}

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
	promptMarkerStyle    string      = "\x1b[1;36m›\x1b[0m "
	bulletMarkerStyle    string      = "\x1b[1;36m•\x1b[0m "
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

	lockFile, releaseStatus, alreadyRunning, err := acquireInstallLock()
	if err != nil {
		return err
	}
	if alreadyRunning {
		logTTYLine("Another dotfiles install is already running in a different terminal.")
		return nil
	}
	defer releaseStatus()
	defer lockFile.Close()

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
	primePrivilegeCredentials(ctx)
	configuredGit := false
	pending := pendingGitConfig{}
	if runner.HasCommand("git") {
		if err := configureGit(ctx, useDefaults, pending); err != nil {
			return err
		}
		configuredGit = true
	} else {
		logTTYLine("Git is not installed yet, so the installer will collect git details now and apply them after package setup.")
		pending, err = collectGitConfigInputs(ctx, useDefaults)
		if err != nil {
			return err
		}
	}
	if err := sync.Run(ctx, syncOpts); err != nil {
		slog.WarnContext(ctx, "running sync", "err", err)
		return fmt.Errorf("running sync: %w", err)
	}
	if err := ensureManagedRepository(ctx); err != nil {
		installLogger.WarnContextWithErr(ctx, "Finishing archive bootstrap failed", err)
	}
	if !configuredGit && runner.HasCommand("git") {
		if err := configureGit(ctx, useDefaults, pending); err != nil {
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

func configureGit(ctx context.Context, useDefaults bool, pending pendingGitConfig) error {
	libPath := filepath.Join(dotfilesRoot(), "lib", ".gitconfig_incl")
	if err := cmdexec.Run(ctx, "git", "config", "--global", "include.path", libPath); err != nil {
		slog.WarnContext(ctx, "Failed to set git include path", "err", err)
		installLogger.WarnContextWithErr(ctx, "Failed to set git include path", err)
		slog.WarnContext(ctx, "set git include path", "err", err)
		return fmt.Errorf("set git include path: %w", err)
	}

	name := gitConfig(ctx, "user.name")
	if name == "" {
		if err := setOrPromptGitConfig(ctx, useDefaults, pending.name, "Skipping git user.name (use defaults mode)", "Enter your name for git (First Last): ", "user.name"); err != nil {
			return err
		}
	}

	email := gitConfig(ctx, "user.email")
	if email == "" {
		if err := setOrPromptGitConfig(ctx, useDefaults, pending.email, "Skipping git user email (use defaults mode)", "Enter your git email: ", "user.email"); err != nil {
			return err
		}
	}

	currentCommand := gitConfig(ctx, "gpg.ssh.defaultKeyCommand")
	if currentCommand != "" {
		return nil
	}

	signingKey, keyCommand, err := resolveSigningKey(ctx, useDefaults, pending)
	if err != nil {
		return err
	}
	if signingKey == "" {
		return nil
	}
	if keyCommand != "" {
		if err := cmdexec.Run(ctx, "git", "config", "--global", "gpg.ssh.defaultKeyCommand", keyCommand); err != nil {
			slog.WarnContext(ctx, "running git config", "err", err)
			return fmt.Errorf("running git config: %w", err)
		}
	}
	if err := cmdexec.Run(ctx, "git", "config", "--global", "user.signingKey", "key::"+signingKey); err != nil {
		slog.WarnContext(ctx, "running git config", "err", err)
		return fmt.Errorf("running git config: %w", err)
	}
	return nil
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

func setOrPromptGitConfig(ctx context.Context, useDefaults bool, existingValue, skipMsg, prompt, key string) error {
	if useDefaults {
		logInfo(ctx, skipMsg)
		return nil
	}
	input := existingValue
	if input == "" {
		input = readLine(ctx, prompt)
	}
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
	_ = ctx
	logPrompt(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func logPrompt(prompt string) {
	if installLogger == nil {
		fmt.Fprintln(os.Stdout, prompt)
		fmt.Fprint(os.Stdout, "> ")
		return
	}
	installLogger.PrintTTYLine(prompt, promptMarkerStyle+prompt)
	_, _ = fmt.Fprint(os.Stdout, "> ")
}

func logTTYLine(message string) {
	if installLogger != nil {
		installLogger.PrintTTYLine(message, bulletMarkerStyle+message)
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

func primePrivilegeCredentials(ctx context.Context) {
	if os.Geteuid() == 0 || !runner.HasCommand("sudo") {
		return
	}
	logTTYLine("Checking sudo access now so setup does not pause later.")
	if common.HasSudoAccess(ctx, installLogger) {
		logTTYLine("Sudo access is ready for the rest of install.")
		return
	}
	logTTYLine("Sudo access is not available right now, so privileged setup may fail later.")
}

func acquireInstallLock() (*os.File, func(), bool, error) {
	cacheDir := filepath.Clean(filepath.Join(os.Getenv("HOME"), ".cache"))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, nil, false, fmt.Errorf("creating cache directory: %w", err)
	}
	lockPath := filepath.Join(cacheDir, "dotfiles_install.flock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, nil, false, fmt.Errorf("opening install lock file: %w", err)
	}
	flockFdInt := int(lockFile.Fd())
	if err := syscall.Flock(flockFdInt, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lockFile.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, nil, true, nil
		}
		return nil, nil, false, fmt.Errorf("acquiring install lock: %w", err)
	}
	statusDir := filepath.Join(cacheDir, "dotfiles_install.lock")
	if err := os.MkdirAll(statusDir, 0o755); err != nil {
		_ = syscall.Flock(flockFdInt, syscall.LOCK_UN)
		_ = lockFile.Close()
		return nil, nil, false, fmt.Errorf("creating install status directory: %w", err)
	}
	release := func() {
		_ = os.RemoveAll(statusDir)
	}
	return lockFile, release, false, nil
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

func collectGitConfigInputs(ctx context.Context, useDefaults bool) (pendingGitConfig, error) {
	pending := pendingGitConfig{}
	if useDefaults {
		return pending, nil
	}
	pending.name = readLine(ctx, "Enter your name for git (First Last): ")
	pending.email = readLine(ctx, "Enter your git email: ")
	signingKeyPath, foundKeys := resolveSigningKeyPath(ctx, useDefaults)
	if signingKeyPath == "" {
		if !foundKeys {
			logTTYLine("No SSH public key was found in ~/.ssh, so git SSH signing will stay unset for now.")
			logTTYLine("Run ssh-keygen after install, then rerun ./install.sh to enable signing.")
		}
		return pending, nil
	}
	signingKey, err := readSigningKey(signingKeyPath)
	if err != nil {
		return pending, err
	}
	pending.signingKey = signingKey
	logInfof(ctx, "Using SSH public key for git signing: %s", displayPath(signingKeyPath))
	return pending, nil
}

func resolveSigningKey(ctx context.Context, useDefaults bool, pending pendingGitConfig) (string, string, error) {
	if pending.signingKey != "" {
		return pending.signingKey, pending.gpgSshDefaultKeyCommand, nil
	}
	sshAddOutput, _ := cmdexec.OutputTrimmed(ctx, "ssh-add", "-L")
	if signingKey, keyCommand, ok := detectSigningKeyFromAgent(sshAddOutput); ok {
		return signingKey, keyCommand, nil
	}
	keyPath, foundKeys := resolveSigningKeyPath(ctx, useDefaults)
	if keyPath == "" {
		if !useDefaults && !foundKeys {
			logTTYLine("No SSH public key was found in ~/.ssh, so git SSH signing will stay unset for now.")
			logTTYLine("Run ssh-keygen after install, then rerun ./install.sh to enable signing.")
		}
		return "", "", nil
	}
	signingKey, err := readSigningKey(keyPath)
	if err != nil {
		return "", "", err
	}
	logInfof(ctx, "Using SSH public key for git signing: %s", displayPath(keyPath))
	return signingKey, "", nil
}

// detectSigningKeyFromAgent returns the ssh public key line, the matching
// gpg.ssh.defaultKeyCommand value, and whether an ed25519 key was found.
func detectSigningKeyFromAgent(sshAddOutput string) (string, string, bool) {
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
		return line, fmt.Sprintf("ssh-add -L | grep '%s'", keyID), true
	}
	return "", "", false
}

func resolveSigningKeyPath(ctx context.Context, useDefaults bool) (string, bool) {
	candidates := sshPublicKeyCandidates()
	if len(candidates) == 0 {
		return "", false
	}
	if useDefaults || len(candidates) == 1 {
		return candidates[0], true
	}
	logTTYLine("Choose an SSH public key for git signing.")
	for index, candidate := range candidates {
		logTTYLine(fmt.Sprintf("%d. %s", index+1, displayPath(candidate)))
	}
	choice := readLine(ctx, "SSH key number, or press Enter to skip")
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

func readSigningKey(keyPath string) (string, error) {
	keyRaw, err := os.ReadFile(filepath.Clean(strings.TrimSpace(keyPath)))
	if err != nil {
		slog.Warn("Failed to read SSH public key", "err", err)
		if installLogger != nil {
			installLogger.WarnContextWithErr(context.Background(), "Failed to read SSH public key", err)
		}
		return "", fmt.Errorf("read ssh public key: %w", err)
	}
	return strings.TrimSpace(string(keyRaw)), nil
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
