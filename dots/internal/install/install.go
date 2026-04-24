package install

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/runner"
	"github.com/agoodkind/.dotfiles/internal/sync"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

var installLogger *telemetry.Logger

func Run(ctx context.Context, args ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	logPath := filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "install.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		return err
	}
	installLogger = logger
	defer logger.Close()
	defer func() {
		installLogger = nil
		runner.SetLogger(nil)
	}()
	runner.SetLogger(logger)
	_ = os.Setenv("DOTFILES_LOG", logPath)
	done := logger.Section("Install")
	defer done()

	useDefaults := false
	syncOpts := sync.Options{}

	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			printInstallUsage()
			return nil
		case "--use-defaults", "-d":
			useDefaults = true
			syncOpts.UseDefaults = true
		case "--repair":
			syncOpts.RepairMode = true
		case "--quick":
			syncOpts.QuickMode = true
		case "--skip-git":
			syncOpts.SkipGit = true
		case "--skip-network":
			syncOpts.SkipNetwork = true
		default:
			return fmt.Errorf("unsupported install flag: %s", arg)
		}
	}
	syncOpts.UseDefaults = useDefaults

	if err := createSocketDir(); err != nil {
		return err
	}
	if err := configureGit(useDefaults); err != nil {
		return err
	}
	if err := sync.Run(ctx, syncOpts); err != nil {
		return err
	}
	if err := ensureLoginShell(); err != nil {
		return err
	}
	return nil
}

func printInstallUsage() {
	logInfo("Usage: dots install [--use-defaults] [--quick] [--skip-git] [--skip-network] [--repair]")
}

func createSocketDir() error {
	return os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".ssh", "sockets"), 0o700)
}

func configureGit(useDefaults bool) error {
	libPath := filepath.Join(dotfilesRoot(), "lib", ".gitconfig_incl")
	if err := cmdexec.Run(context.Background(), "git", "config", "--global", "include.path", libPath); err != nil {
		return fmt.Errorf("set git include path: %w", err)
	}

	name, err := gitConfig("user.name")
	if err != nil {
		return err
	}
	if name == "" {
		if useDefaults {
			logInfo("Skipping git user.name (use defaults mode)")
		} else {
			input := readLine("Enter your name for git (First Last): ")
			if input != "" {
				if err := cmdexec.Run(context.Background(), "git", "config", "--global", "user.name", input); err != nil {
					return err
				}
			}
		}
	}

	email, err := gitConfig("user.email")
	if err != nil {
		return err
	}
	if email == "" {
		if useDefaults {
			logInfo("Skipping git user email (use defaults mode)")
		} else {
			input := readLine("Enter your git email: ")
			if input != "" {
				if err := cmdexec.Run(context.Background(), "git", "config", "--global", "user.email", input); err != nil {
					return err
				}
			}
		}
	}

	currentCommand, err := gitConfig("gpg.ssh.defaultKeyCommand")
	if err != nil {
		return err
	}
	if currentCommand != "" {
		return nil
	}

	sshAddOutput, _ := cmdexec.OutputTrimmed(context.Background(), "ssh-add", "-L")
	for _, line := range strings.Split(strings.TrimSpace(sshAddOutput), "\n") {
		if line == "" {
			continue
		}
		if strings.Contains(line, "ed25519") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				keyID := fields[1]
				keyValue := line
				if err := cmdexec.Run(context.Background(), "git", "config", "--global", "gpg.ssh.defaultKeyCommand", fmt.Sprintf("ssh-add -L | grep '%s'", keyID)); err != nil {
					return err
				}
				return cmdexec.Run(context.Background(), "git", "config", "--global", "user.signingKey", "key::"+keyValue)
			}
			break
		}
	}

	publicKeyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519.pub")
	if publicKeyRaw, err := os.ReadFile(publicKeyPath); err == nil {
		line := strings.TrimSpace(string(publicKeyRaw))
		if line != "" {
			return cmdexec.Run(context.Background(), "git", "config", "--global", "user.signingKey", "key::"+line)
		}
	}

	if useDefaults {
		logInfo("Skipping SSH key setup (use defaults mode)")
		return nil
	}

	keyPath := readLine("Enter path to your SSH public key (or leave empty to skip): ")
	if keyPath == "" {
		return nil
	}
	keyRaw, err := os.ReadFile(strings.TrimSpace(keyPath))
	if err != nil {
		return fmt.Errorf("read ssh public key: %w", err)
	}
	line := strings.TrimSpace(string(keyRaw))
	if line == "" {
		return nil
	}
	return cmdexec.Run(context.Background(), "git", "config", "--global", "user.signingKey", "key::"+line)
}

func ensureLoginShell() error {
	zshPath, err := runner.LookPath("zsh")
	if err != nil {
		logInfo("Skipping login shell change (zsh not found)")
		return nil
	}
	currentShell, currentErr := detectCurrentShell()
	if currentErr != nil {
		currentShell = "unknown"
	}
	logInfof("Current login shell: %s", currentShell)
	logInfof("Target zsh path: %s", zshPath)

	isZsh := currentShell == zshPath
	if !isZsh && strings.HasSuffix(filepath.Base(currentShell), "zsh") {
		isZsh = true
	}
	if isZsh {
		logInfo("Shell is already zsh")
		return nil
	}

	if os.Getenv("GITHUB_ACTIONS") == "true" {
		logInfo("Skipping shell change in CI")
		return nil
	}
	if err := cmdexec.Run(context.Background(), "chsh", "-s", zshPath); err != nil {
		return fmt.Errorf("change login shell: %w", err)
	}
	logInfo("Login shell changed to zsh")
	return nil
}

func detectCurrentShell() (string, error) {
	if runtime.GOOS == "darwin" {
		userInfo, err := user.Current()
		if err != nil {
			return "", err
		}
		output, err := cmdexec.OutputTrimmed(context.Background(), "dscl", ".", "-read", fmt.Sprintf("/Users/%s", userInfo.Username), "UserShell")
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			line = strings.TrimSpace(line)
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "UserShell:" {
				return fields[1], nil
			}
		}
		return "", nil
	}

	output, err := cmdexec.OutputTrimmed(context.Background(), "getent", "passwd", os.Getenv("USER"))
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.TrimSpace(output), ":")
	if len(parts) < 7 {
		return "", nil
	}
	return parts[6], nil
}

func gitConfig(name string) (string, error) {
	output, err := cmdexec.OutputTrimmed(context.Background(), "git", "config", "--global", name)
	if err != nil {
		return "", nil
	}
	return output, nil
}

func readLine(prompt string) string {
	logInfo(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func logInfo(message string) {
	if installLogger != nil {
		installLogger.Info(message)
	}
}

func logInfof(format string, args ...any) {
	if installLogger != nil {
		installLogger.Info(fmt.Sprintf(format, args...))
	}
}

func dotfilesRoot() string {
	if value := os.Getenv("DOTDOTFILES"); value != "" {
		return value
	}
	return filepath.Join(os.Getenv("HOME"), ".dotfiles")
}
