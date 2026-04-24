package compilation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/prefercache"
	"github.com/agoodkind/.dotfiles/internal/runner"
	"github.com/agoodkind/.dotfiles/internal/sync/common"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

func CompileZshFiles(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	dirs := []string{
		filepath.Join(dotfiles, "zshrc"),
		filepath.Join(dotfiles, "lib", "zinit"),
		filepath.Join(dotfiles, "lib", "zsh-defer"),
		filepath.Join(dotfiles, "home"),
		filepath.Join(dotfiles, "bin"),
	}
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			name := filepath.Base(path)
			if !(strings.HasSuffix(name, ".zsh") || name == ".zshrc") {
				return nil
			}
			compiled := path + ".zwc"
			if NeedsCompile(path, compiled) {
				_ = cmdexec.RunWithLogger(ctx, logger, "zsh", "-c", fmt.Sprintf("zcompile %q", path))
			}
			return nil
		})
	}
	return nil
}

func NeedsCompile(source string, target string) bool {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return false
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		return true
	}
	return sourceInfo.ModTime().After(targetInfo.ModTime())
}

func RebuildZcompdump(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	_ = os.RemoveAll(filepath.Join(os.Getenv("HOME"), ".zcompdump"))
	if !runner.HasCommand("zsh") {
		return nil
	}
	fpath := filepath.Join(dotfiles, "zshrc", "completions")
	return cmdexec.RunWithLogger(ctx, logger, "zsh", "-c", fmt.Sprintf("fpath=(\"%s\" $fpath)\nautoload -Uz compinit\ncompinit -d ~/.zcompdump\nzcompile ~/.zcompdump\n", fpath))
}

func RebuildPreferCache(ctx context.Context, dotfiles string, logger *telemetry.Logger) error {
	return prefercache.Rebuild(ctx, dotfiles, false, logger)
}

func CreateHushLogin(_ context.Context, _ *telemetry.Logger) error {
	path := filepath.Join(os.Getenv("HOME"), ".hushlogin")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return common.Touch(path)
}

func SyncRulesFromDir(src string, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := filepath.Glob(filepath.Join(src, "*.mdc"))
	if err != nil {
		return nil
	}
	for _, file := range entries {
		target := filepath.Join(dst, filepath.Base(file))
		if err := SyncFile(file, target); err != nil {
			return err
		}
	}
	return nil
}

func SyncRulesFromDirAsMd(src string, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := filepath.Glob(filepath.Join(src, "*.mdc"))
	if err != nil {
		return nil
	}
	for _, file := range entries {
		base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		target := filepath.Join(dst, base+".md")
		if err := SyncFile(file, target); err != nil {
			return err
		}
	}
	return nil
}

func SyncFilesToDir(src string, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := SyncFile(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func SyncSkillDirs(src string, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		source := filepath.Join(src, entry.Name())
		target := filepath.Join(dst, entry.Name())
		_ = os.Remove(target)
		if err := os.Symlink(source, target); err != nil {
			return err
		}
	}
	return nil
}

func SyncFile(src string, dst string) error {
	if common.IsSymlinkTo(dst, src) {
		return nil
	}
	_ = os.Remove(dst)
	return os.Symlink(src, dst)
}
