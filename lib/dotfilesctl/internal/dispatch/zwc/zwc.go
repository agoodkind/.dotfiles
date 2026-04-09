package zwc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/runner"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

func Recompile(_ context.Context, dotfiles string, dispatchLogger *telemetry.Logger) error {
	if !runner.HasCommand("zsh") {
		return nil
	}
	dirs := []string{
		filepath.Join(dotfiles, "zshrc"),
		filepath.Join(dotfiles, "lib", "zinit"),
		filepath.Join(dotfiles, "lib", "zsh-defer"),
		filepath.Join(dotfiles, "home"),
		filepath.Join(dotfiles, "bin"),
		filepath.Join(os.Getenv("HOME"), ".local", "share", "zinit", "plugins"),
		filepath.Join(os.Getenv("HOME"), ".local", "share", "zinit", "snippets"),
	}
	compiled := 0
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if !(strings.HasSuffix(path, ".zsh") || filepath.Base(path) == ".zshrc") {
				return nil
			}
			if needsCompile(path) {
				_, err := cmdexec.OutputWithLogger(context.Background(), dispatchLogger, "zsh", "-c", fmt.Sprintf("zcompile %q", path))
				if err == nil {
					compiled++
				}
			}
			return nil
		})
	}
	dispatchLogger.Info(fmt.Sprintf("compiled %d file(s)", compiled))
	return nil
}

func needsCompile(path string) bool {
	sourceInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	compiledInfo, err := os.Stat(path + ".zwc")
	if err != nil {
		return true
	}
	return sourceInfo.ModTime().After(compiledInfo.ModTime())
}
