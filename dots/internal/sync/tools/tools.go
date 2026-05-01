package tools

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/telemetry"
)

func InstallCustomTools(ctx context.Context, _ string, logger *telemetry.Logger) error {
	return runCustomTools(ctx, logger)
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type crateResponse struct {
	MaxVersion string `json:"max_version"`
}

func runCustomTools(ctx context.Context, logger *telemetry.Logger) error {
	entries := common.DefaultCustomToolDeclarations()
	failed := make([]string, 0)
	for _, tool := range entries {
		if tool.ID == "" || tool.Bin == "" {
			continue
		}
		if err := installCustomTool(ctx, tool, logger); err != nil {
			failed = append(failed, tool.ID)
			_ = telemetry.Notify("warn", "tool install/upgrade failed: "+tool.ID, getSyncLogPath())
			common.Warn(logger, "  "+tool.ID+": failed")
		}
	}
	if len(failed) > 0 {
		common.Warn(logger, "  custom tools completed with failures: "+strings.Join(failed, ", "))
		return nil
	}
	return nil
}

func installCustomTool(ctx context.Context, tool catalog.ToolDeclaration, logger *telemetry.Logger) error {
	if shouldSkipToolByPlatform(tool.ID) {
		return nil
	}

	if !isToolSupported(tool.ID) {
		common.Warn(logger, "  skipping unsupported tool: "+tool.ID)
		return nil
	}

	current := getCurrentToolVersion(ctx, tool.Bin, logger)
	if tool.Repo != "" {
		latest, err := getLatestGitHubVersion(tool.Repo)
		if err != nil {
			return err
		}
		if current != "" && shouldSkipToolUpgrade(current, latest) {
			common.Infof(logger, "  %s is up to date (%s)", tool.ID, current)
			return nil
		}
	}

	common.Infof(logger, "  installing %s", tool.ID)
	switch tool.ID {
	case "zoxide":
		return installToolFromScript(ctx, "zoxide", "https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh", nil, logger)
	case "async-cmd":
		return installAsyncCmd(ctx, logger)
	case "starship":
		return installToolFromScript(ctx, "starship", "https://starship.rs/install.sh", []string{"--", "--yes"}, logger)
	case "fastfetch":
		return installToolFromGitHubRelease(ctx, "fastfetch", "fastfetch-cli/fastfetch", "linux", "amd64", ".deb", logger)
	case "procs":
		return installToolFromGitHubRelease(ctx, "procs", "dalance/procs", mapOSForTool(tool.ID), mapArchForTool(tool.ID), ".zip", logger)
	case "tokei":
		return installToolFromGitHubRelease(ctx, "tokei", "XAMPPRocky/tokei", mapOSForTokei(), mapArchForTool(tool.ID), ".tar.gz", logger)
	case "tree-sitter":
		return installToolFromGitHubRelease(ctx, "tree-sitter", "tree-sitter/tree-sitter", mapOSForTool(tool.ID), mapArchTreeSitter(), ".gz", logger)
	case "fzf":
		return installToolFromGitHubRelease(ctx, "fzf", "junegunn/fzf", mapOSForFZF(), mapArchForTool(tool.ID), ".tar.gz", logger)
	case "xh":
		return installToolFromGitHubRelease(ctx, "xh", "ducaale/xh", mapOSForXH(), mapArchForTool(tool.ID), ".tar.gz", logger)
	case "cloudflare-speed-cli":
		return installToolFromGitHubRelease(ctx, "cloudflare-speed-cli", "kavehtehrani/cloudflare-speed-cli", mapOSForTool(tool.ID), mapArchForTool(tool.ID), ".tar.xz", logger)
	case "yq":
		return installToolFromGitHubRelease(ctx, "yq", "mikefarah/yq", "linux", mapArchForTool(tool.ID), ".tar.gz", logger)
	default:
		common.Warn(logger, "  skipping unknown tool: "+tool.ID)
		return nil
	}
}

func shouldSkipToolByPlatform(toolID string) bool {
	switch toolID {
	case "fastfetch":
		return runtime.GOOS != "linux"
	case "yq":
		return runtime.GOOS != "linux"
	case "cloudflare-speed-cli":
		return !(runtime.GOOS == "darwin" || runtime.GOOS == "linux")
	default:
		return false
	}
}

func isToolSupported(toolID string) bool {
	return toolID == "zoxide" || toolID == "async-cmd" || toolID == "starship" || toolID == "fastfetch" || toolID == "procs" || toolID == "tokei" || toolID == "tree-sitter" || toolID == "fzf" || toolID == "xh" || toolID == "cloudflare-speed-cli" || toolID == "yq"
}

func shouldSkipToolUpgrade(current, target string) bool {
	if current == "" || target == "" {
		return false
	}
	return common.VersionAtLeast(current, normalizeSemver(target))
}

func getCurrentToolVersion(ctx context.Context, toolName string, logger *telemetry.Logger) string {
	output, err := cmdexec.OutputWithLoggerAndEnv(ctx, logger, nil, toolName, "--version")
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)
	m := re.FindString(output)
	return strings.TrimSpace(m)
}

func installAsyncCmd(ctx context.Context, logger *telemetry.Logger) error {
	latest, err := getLatestCrateVersion("async-cmd")
	if err != nil {
		return err
	}
	current := getCurrentToolVersion(ctx, "async", logger)
	if shouldSkipToolUpgrade(current, latest) && current != "" {
		common.Infof(logger, "  async-cmd is up to date (%s)", current)
		return nil
	}
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return nil
	}
	return cmdexec.RunWithLogger(ctx, logger, "cargo", "install", "async-cmd", "--locked", "--force")
}

func installToolFromScript(ctx context.Context, name, scriptURL string, args []string, logger *telemetry.Logger) error {
	common.Infof(logger, "  installing %s", name)
	scriptPath, err := downloadToTempFile(ctx, scriptURL)
	if err != nil {
		return err
	}
	defer os.Remove(scriptPath)
	cmdArgs := append([]string{scriptPath}, args...)
	return cmdexec.RunWithLogger(ctx, logger, "sh", cmdArgs...)
}

func installToolFromGitHubRelease(ctx context.Context, name, repo, osTag, archTag, ext string, logger *telemetry.Logger) error {
	release, err := fetchLatestRelease(repo)
	if err != nil {
		return err
	}
	var assetURL string
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, osTag) && strings.Contains(asset.Name, archTag) && strings.HasSuffix(asset.Name, ext) {
			assetURL = asset.BrowserDownloadURL
			break
		}
	}
	if assetURL == "" {
		return fmt.Errorf("no asset found for %s", name)
	}
	localPath, err := downloadToTempFile(ctx, assetURL)
	if err != nil {
		return err
	}
	defer os.Remove(localPath)
	return installToolArtifact(ctx, logger, name, localPath, ext)
}

func fetchLatestRelease(repo string) (githubRelease, error) {
	var rel githubRelease
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return rel, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return rel, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return rel, fmt.Errorf("github api error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return rel, err
	}
	return rel, nil
}

func getLatestGitHubVersion(repo string) (string, error) {
	release, err := fetchLatestRelease(repo)
	if err != nil {
		return "", err
	}
	return normalizeSemver(release.TagName), nil
}

func getLatestCrateVersion(crateName string) (string, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://crates.io/api/v1/crates/"+crateName, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("crates api error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload crateResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return normalizeSemver(payload.MaxVersion), nil
}

func normalizeSemver(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	return value
}

func installToolArtifact(ctx context.Context, logger *telemetry.Logger, tool string, artifactPath string, ext string) error {
	switch {
	case strings.HasSuffix(artifactPath, ".deb") || strings.HasSuffix(artifactPath, ".rpm"):
		return cmdexec.RunWithLogger(ctx, logger, "sudo", "dpkg", "-i", artifactPath)
	case strings.HasSuffix(artifactPath, ".gz") && !strings.Contains(artifactPath, ".tar"):
		binPath := filepath.Join(os.Getenv("HOME"), ".local", "bin", tool)
		if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
			return err
		}
		if err := decompressGzipBinary(artifactPath, binPath); err != nil {
			return err
		}
		return nil
	case strings.HasSuffix(artifactPath, ".zip") || strings.HasSuffix(artifactPath, ".tar.gz") || strings.HasSuffix(artifactPath, ".tar.xz"):
		tmpDir, err := os.MkdirTemp("", "dotfiles-tool-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		if strings.HasSuffix(artifactPath, ".zip") {
			if err := cmdexec.RunWithLogger(ctx, logger, "unzip", "-o", artifactPath, "-d", tmpDir); err != nil {
				return err
			}
		} else if strings.HasSuffix(artifactPath, ".xz") {
			if err := cmdexec.RunWithLogger(ctx, logger, "tar", "xJf", artifactPath, "-C", tmpDir); err != nil {
				return err
			}
		} else {
			if err := cmdexec.RunWithLogger(ctx, logger, "tar", "xzf", artifactPath, "-C", tmpDir); err != nil {
				return err
			}
		}

		return installFromDir(tool, tmpDir)
	default:
		return fmt.Errorf("unsupported artifact type for %s", filepath.Base(artifactPath))
	}
}

func decompressGzipBinary(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	reader, err := gzip.NewReader(input)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer output.Close()
	if _, err := io.Copy(output, reader); err != nil {
		return err
	}
	return os.Chmod(destination, 0o755)
}

func installFromDir(tool string, dir string) error {
	targetDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	bin := filepath.Join(targetDir, tool)
	found := ""
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if entry.Name() == tool || strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())) == tool {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	if found == "" {
		return fmt.Errorf("binary %s not found in archive", tool)
	}
	return copyExecutable(found, bin)
}

func copyExecutable(src, dst string) error {
	payload, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, payload, 0o755); err != nil {
		return err
	}
	return os.Chmod(dst, 0o755)
}

func getSyncLogPath() string {
	if path := os.Getenv("DOTFILES_LOG"); path != "" {
		return path
	}
	return filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "sync.log")
}

func DownloadToTempFile(ctx context.Context, _ *telemetry.Logger, fileURL string) (string, error) {
	return downloadToTempFile(ctx, fileURL)
}

func downloadToTempFile(ctx context.Context, fileURL string) (string, error) {
	request, err := http.NewRequest(http.MethodGet, fileURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "dots")
	client := &http.Client{Timeout: 120 * time.Second}
	response, err := client.Do(request.WithContext(ctx))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		return "", fmt.Errorf("download failed for %s: %d %s", fileURL, response.StatusCode, strings.TrimSpace(string(body)))
	}

	tmp, err := os.CreateTemp("", "dots-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	_, err = io.Copy(tmp, response.Body)
	if err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

func mapOSForTool(tool string) string {
	if runtime.GOOS == "darwin" {
		if tool == "procs" || tool == "fzf" {
			return "mac"
		}
		if tool == "tree-sitter" {
			return "macos"
		}
		return "apple-darwin"
	}
	if runtime.GOOS == "linux" {
		return "linux"
	}
	return ""
}

func mapOSForFZF() string {
	if runtime.GOOS == "darwin" {
		return "darwin"
	}
	return "linux"
}

func mapOSForXH() string {
	if runtime.GOOS == "darwin" {
		return "apple-darwin"
	}
	return "unknown-linux-musl"
}

func mapOSForTokei() string {
	if runtime.GOOS == "darwin" {
		return "apple-darwin"
	}
	return "unknown-linux-gnu"
}

func mapArchForTool(tool string) string {
	switch {
	case tool == "procs":
		if isIntel() {
			return "amd64"
		}
		return "aarch64"
	case tool == "tree-sitter":
		if runtime.GOOS == "darwin" {
			return mapArchTreeSitter()
		}
		if isIntel() {
			return "x64"
		}
		return "arm64"
	case tool == "cloudflare-speed-cli", tool == "xh":
		if isIntel() {
			return "x86_64"
		}
		return "aarch64"
	case tool == "tokei":
		if isIntel() {
			return "x86_64"
		}
		return "aarch64"
	case tool == "fzf", tool == "yq":
		if isIntel() {
			return "amd64"
		}
		return "arm64"
	default:
		if isIntel() {
			return "amd64"
		}
		return "aarch64"
	}
}

func mapArchTreeSitter() string {
	if isIntel() {
		return "x64"
	}
	return "arm64"
}

func isIntel() bool {
	return runtime.GOARCH == "amd64"
}

func aptPackageName(packageName string) string {
	switch packageName {
	case "ack":
		return "ack-grep"
	case "fd":
		return "fd-find"
	case "rg":
		return "ripgrep"
	case "openssh":
		return "openssh-client openssh-server"
	}
	return packageName
}

func brewPackageName(packageName string) string {
	switch packageName {
	case "tshark":
		return "wireshark"
	}
	return packageName
}

func isSnapPackage(packageName string, snapList []string) bool {
	for _, snap := range snapList {
		if snap == packageName {
			return true
		}
	}
	return false
}

func snapPackageName(packageName string) string {
	if packageName == "neovim" {
		return "nvim"
	}
	return packageName
}
