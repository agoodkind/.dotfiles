package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// CleanupNeovimRepair removes stale Neovim plugin and swap file directories.
//
// Repair invokes CleanupNeovimRepair during sync repair.
// It deletes unfinished lazy.nvim clones and empty plugin directories
// under the XDG data and state paths.
//
// Example:
//
//	if err := CleanupNeovimRepair(ctx, logger); err != nil {
//		return err
//	}
func CleanupNeovimRepair(ctx context.Context, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "workspace: CleanupNeovimRepair")
	nvimData := filepath.Join(xdgDataDir(), "nvim")
	lazyDir := filepath.Join(nvimData, "lazy")
	if entries, err := os.ReadDir(lazyDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasSuffix(name, ".cloning") {
				_ = os.RemoveAll(filepath.Clean(filepath.Join(lazyDir, name)))
			}
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			entries2, err := os.ReadDir(filepath.Join(lazyDir, entry.Name()))
			if err != nil || len(entries2) == 0 {
				_ = os.RemoveAll(filepath.Clean(filepath.Join(lazyDir, entry.Name())))
			}
		}
	}

	swapDirs := []string{
		filepath.Join(xdgStateDir(), "nvim", "swap"),
		filepath.Join(xdgDataDir(), "nvim", "swap"),
	}
	for _, dir := range swapDirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.swp"))
		if err == nil {
			for _, file := range files {
				_ = os.Remove(filepath.Clean(file))
			}
		}
	}

	return nil
}

const treeSitterVersionFieldCount = 2

const treeSitterInstallCommand = "+lua require('nvim-treesitter').install({" +
	"'bash','lua','vim','vimdoc','python','javascript','typescript','json','yaml'" +
	"})"

// UpdateNeovimPlugins runs Neovim lazy sync and tree-sitter parser installs.
//
// Sync invokes it when nvim and tree-sitter are installed and tree-sitter meets
// the minimum version for parser installs.
//
// Example:
//
//	if err := UpdateNeovimPlugins(ctx, logger); err != nil {
//		return err
//	}
func UpdateNeovimPlugins(ctx context.Context, logger *telemetry.Logger) error {
	if !runner.HasCommand("nvim") {
		return nil
	}

	if err := cmdexec.RunWithLogger(
		ctx,
		logger,
		"nvim",
		"--headless",
		"-c",
		"lua require('lazy').sync()",
		"-c",
		"qa",
	); err != nil {
		slog.WarnContext(ctx, "workspace: running neovim lazy sync", "err", err)

		return fmt.Errorf("running neovim lazy sync: %w", err)
	}

	if !runner.HasCommand("tree-sitter") {
		return nil
	}

	if versionOutput, err := cmdexec.OutputWithLoggerAndEnv(
		ctx,
		logger,
		nil,
		"tree-sitter",
		"--version",
	); err == nil {
		parts := strings.Fields(versionOutput)
		if len(parts) >= treeSitterVersionFieldCount {
			version := strings.TrimPrefix(parts[1], "v")
			if common.VersionAtLeast(version, "0.21.0") {
				common.InfoContext(ctx, logger, "  treesitter parsers")
				_ = cmdexec.RunWithLogger(
					ctx,
					logger,
					"nvim",
					"--headless",
					treeSitterInstallCommand,
					"+sleep 10",
					"+qa",
				)
			}
		}
	}

	return nil
}
