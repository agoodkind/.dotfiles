package tools_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goodkind.io/.dotfiles/internal/sync/compilation"
	"goodkind.io/.dotfiles/internal/sync/corpus"
	"goodkind.io/.dotfiles/internal/sync/tools"
	"goodkind.io/.dotfiles/internal/telemetry"
)

func TestGitHubInstallersIntegration(t *testing.T) {
	if os.Getenv("DOTFILES_GH_INTEGRATION") != "1" {
		t.Skip("Set DOTFILES_GH_INTEGRATION=1 to install real GitHub dependencies in temporary directories.")
	}
	if os.Getenv("DOTFILES_GH_INTEGRATION_STAGE") != "" {
		runGitHubInstallerIntegrationStage(t)
		return
	}
	if _, err := exec.LookPath("gh"); err != nil {
		t.Fatalf("Install GitHub CLI with skill support before running this integration test: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	tokenOutput, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	if err != nil {
		t.Fatalf("Authenticate GitHub CLI or set GH_TOKEN before running this integration test: %v", err)
	}
	token := strings.TrimSpace(string(tokenOutput))
	if token == "" {
		t.Fatal("Authenticate GitHub CLI or set GH_TOKEN before running this integration test.")
	}
	repository, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	configDir := filepath.Join(home, "catalog")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"packages.toml", "dispatch.toml", "platform-macos.toml"} {
		if err := os.WriteFile(filepath.Join(configDir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeGitHubIntegrationCatalog(t, configDir, "github/gh-stack")
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Join(home, ".local", "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_CONFIG_DIR", filepath.Join(home, "gh-config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("DOTFILES_CONFIG_DIR", configDir)
	t.Setenv("DOTFILES_GH_INTEGRATION_REPOSITORY", repository)
	t.Setenv("GH_TOKEN", token)
	runGitHubIntegrationChild(t, ctx, "install")
	agentsSkill := snapshotGitHubIntegrationTree(t, filepath.Join(home, ".agents", "skills", "gh-stack"))
	claudeSkill := snapshotGitHubIntegrationTree(t, filepath.Join(home, ".claude", "skills", "gh-stack"))
	writeGitHubIntegrationCatalog(t, configDir, "other/gh-stack")
	runGitHubIntegrationChild(t, ctx, "conflict")
	assertGitHubIntegrationTree(t, filepath.Join(home, ".agents", "skills", "gh-stack"), agentsSkill)
	assertGitHubIntegrationTree(t, filepath.Join(home, ".claude", "skills", "gh-stack"), claudeSkill)
}

func writeGitHubIntegrationCatalog(t *testing.T, configDir, repository string) {
	t.Helper()
	content := "[[tool]]\nid = \"gh\"\nbin = \"gh\"\nrepo = \"cli/cli\"\ninstall_method = \"github-release\"\nplatforms = [\"linux\"]\nos_linux = \"linux\"\narch_amd64 = \"amd64\"\narch_arm64 = \"arm64\"\narchive_ext = \".tar.gz\"\nminimum_version = \"2.101.0\"\n\n" +
		"[[tool]]\nid = \"gh-stack\"\nbin = \"stack\"\ninstall_method = \"gh-extension\"\nrepo = \"" + repository + "\"\n\n" +
		"[[tool]]\nid = \"gh-stack-skill\"\nbin = \"gh-stack\"\ninstall_method = \"gh-skill\"\nrepo = \"" + repository + "\"\ndestinations = [\".agents/skills\", \".claude/skills\"]\n"
	if err := os.WriteFile(filepath.Join(configDir, "tools.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runGitHubIntegrationChild(t *testing.T, ctx context.Context, stage string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, executable, "-test.run=^TestGitHubInstallersIntegration$", "-test.v")
	command.Env = append(os.Environ(), "DOTFILES_GH_INTEGRATION_STAGE="+stage)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		t.Fatalf("GitHub installer stage %s failed: %v", stage, err)
	}
}

func runGitHubInstallerIntegrationStage(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
	logger, err := telemetry.NewLogger(filepath.Join(os.Getenv("HOME"), "installer.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := logger.Close(); err != nil {
			t.Error(err)
		}
	})
	repository := os.Getenv("DOTFILES_GH_INTEGRATION_REPOSITORY")
	if os.Getenv("DOTFILES_GH_INTEGRATION_STAGE") == "conflict" {
		err := tools.InstallCustomTools(ctx, repository, true, logger)
		if err == nil {
			t.Fatal("The installer accepted an extension and skill from a different source.")
		}
		if !strings.Contains(err.Error(), "failures: gh-stack, gh-stack-skill") {
			t.Fatalf("The installer did not reject both source conflicts: %v", err)
		}
		command := exec.CommandContext(ctx, "gh", "stack", "--help")
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			t.Fatalf("The conflicting declaration changed the usable extension: %v", err)
		}
		return
	}
	for _, strictMode := range []bool{true, true, false} {
		if err := tools.InstallCustomTools(ctx, repository, strictMode, logger); err != nil {
			t.Fatalf("Installing real dependencies with strict mode %t: %v", strictMode, err)
		}
	}
	command := exec.CommandContext(ctx, "gh", "stack", "--help")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		t.Fatalf("The installed extension cannot display help: %v", err)
	}
	for _, destination := range []string{".agents/skills", ".claude/skills"} {
		skillRoot := filepath.Join(os.Getenv("HOME"), destination)
		skillPath := filepath.Join(skillRoot, "gh-stack")
		before := snapshotGitHubIntegrationTree(t, skillPath)
		if len(before) < 2 {
			t.Fatal("The installed skill lacks its reference files.")
		}
		stalePath := filepath.Join(skillRoot, "graphite")
		if err := os.MkdirAll(stalePath, 0o755); err != nil {
			t.Fatal(err)
		}
		staleSkill := "---\nname: graphite\ndescription: Stale generated skill.\n---\n\n" + compilation.GeneratedAgentHTMLMarker + "\n"
		if err := os.WriteFile(filepath.Join(stalePath, "SKILL.md"), []byte(staleSkill), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := corpus.Sync(ctx, repository, logger); err != nil {
			t.Fatalf("Rendering the production corpus: %v", err)
		}
		if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
			t.Fatalf("Corpus sync did not remove the generated Graphite directory: %v", err)
		}
		assertGitHubIntegrationTree(t, skillPath, before)
	}
}

func snapshotGitHubIntegrationTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[relative] = payload
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func assertGitHubIntegrationTree(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	after := snapshotGitHubIntegrationTree(t, root)
	if len(before) != len(after) {
		t.Fatalf("Skill file count changed from %d to %d in %s.", len(before), len(after), root)
	}
	for name, payload := range before {
		if !bytes.Equal(payload, after[name]) {
			t.Errorf("The installer or corpus renderer changed %s.", filepath.Join(root, name))
		}
	}
}
