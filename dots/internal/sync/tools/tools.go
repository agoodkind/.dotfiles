// Package tools implements custom tool installation during sync.
package tools

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/telemetry"
)

type installMethod string

const (
	installMethodScript        installMethod = "script"
	installMethodCargo         installMethod = "cargo"
	installMethodGitHubRelease installMethod = "github-release"
)

// InstallCustomTools installs or updates the custom CLI tools defined in the catalog.
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
	entries := catalog.DefaultToolDeclarations()
	failed := make([]string, 0)
	for _, tool := range entries {
		if tool.ID == "" || tool.Bin == "" {
			continue
		}
		if err := installCustomTool(ctx, tool, logger); err != nil {
			failed = append(failed, tool.ID)
			_ = telemetry.Notify("warn", "tool install/upgrade failed: "+tool.ID, getSyncLogPath())
			logger.WarnContext(ctx, "  "+tool.ID+": failed")
		}
	}
	if len(failed) > 0 {
		logger.WarnContext(ctx, "  custom tools completed with failures: "+strings.Join(failed, ", "))
		return nil
	}
	return nil
}

func installCustomTool(ctx context.Context, tool catalog.ToolDeclaration, logger *telemetry.Logger) error {
	if !isPlatformAllowed(tool) {
		return nil
	}
	if tool.InstallMethod == "" {
		logger.WarnContext(ctx, "  skipping tool with no install method: "+tool.ID)
		return nil
	}
	current := getCurrentToolVersion(ctx, tool.Bin, logger)
	latest, err := resolveLatestVersion(ctx, tool)
	if err != nil {
		return err
	}
	if current != "" && latest != "" && shouldSkipToolUpgrade(current, latest) {
		logger.InfoContext(ctx, "  "+tool.ID+" is up to date ("+current+")")
		return nil
	}
	logger.InfoContext(ctx, "  installing "+tool.ID)
	switch installMethod(tool.InstallMethod) {
	case installMethodScript:
		return installToolFromScript(ctx, tool.ID, tool.ScriptURL, tool.ScriptArgs, logger)
	case installMethodCargo:
		return installToolViaCargo(ctx, tool, logger)
	case installMethodGitHubRelease:
		return installToolFromGitHubRelease(ctx, tool.ID, tool.Bin, tool.Repo, resolveOSTag(tool), resolveArchTag(tool), tool.ArchiveExt, logger)
	default:
		logger.WarnContext(ctx, "  skipping tool with unknown install method: "+tool.ID+" ("+tool.InstallMethod+")")
		return nil
	}
}

func isPlatformAllowed(tool catalog.ToolDeclaration) bool {
	if len(tool.Platforms) == 0 {
		return true
	}
	return slices.Contains(tool.Platforms, runtime.GOOS)
}

func resolveOSTag(tool catalog.ToolDeclaration) string {
	if runtime.GOOS == "darwin" {
		return tool.OSDarwin
	}
	return tool.OSLinux
}

func resolveArchTag(tool catalog.ToolDeclaration) string {
	if runtime.GOARCH == "amd64" {
		return tool.ArchAMD64
	}
	return tool.ArchARM64
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

func resolveLatestVersion(ctx context.Context, tool catalog.ToolDeclaration) (string, error) {
	if tool.Repo != "" {
		return getLatestGitHubVersion(ctx, tool.Repo)
	}
	if tool.CrateName != "" {
		return getLatestCrateVersion(ctx, tool.CrateName)
	}
	return "", nil
}

func installToolViaCargo(ctx context.Context, tool catalog.ToolDeclaration, logger *telemetry.Logger) error {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return nil
	}
	if err := cmdexec.RunWithLogger(ctx, logger, "cargo", "install", tool.CrateName, "--locked", "--force"); err != nil {
		slog.WarnContext(ctx, "tools: installToolViaCargo failed", "crate", tool.CrateName, "err", err)
		return fmt.Errorf("running cargo install %s: %w", tool.CrateName, err)
	}
	return nil
}

func installToolFromScript(ctx context.Context, name, scriptURL string, args []string, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "tools: installToolFromScript")
	logger.InfoContext(ctx, "  installing "+name)
	scriptPath, err := downloadToTempFile(ctx, scriptURL)
	if err != nil {
		return err
	}
	defer os.Remove(scriptPath)
	cmdArgs := append([]string{scriptPath}, args...)
	if err := cmdexec.RunWithLogger(ctx, logger, "sh", cmdArgs...); err != nil {
		slog.WarnContext(ctx, "tools: installToolFromScript run failed", "name", name, "err", err)
		return fmt.Errorf("running install script for %s: %w", name, err)
	}
	return nil
}

func installToolFromGitHubRelease(ctx context.Context, name, bin, repo, osTag, archTag, ext string, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "tools: installToolFromGitHubRelease")
	release, err := fetchLatestRelease(ctx, repo)
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
	return installToolArtifact(ctx, logger, bin, localPath)
}

func fetchLatestRelease(ctx context.Context, repo string) (githubRelease, error) {
	var rel githubRelease
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		slog.ErrorContext(ctx, "creating github request", "repo", repo, "err", err)
		return rel, fmt.Errorf("creating github request for %s: %w", repo, err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return rel, fmt.Errorf("executing github request for %s: %w", repo, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return rel, fmt.Errorf("github api error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return rel, fmt.Errorf("decoding github response for %s: %w", repo, err)
	}
	return rel, nil
}

func getLatestGitHubVersion(ctx context.Context, repo string) (string, error) {
	release, err := fetchLatestRelease(ctx, repo)
	if err != nil {
		return "", err
	}
	return normalizeSemver(release.TagName), nil
}

func getLatestCrateVersion(ctx context.Context, crateName string) (string, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://crates.io/api/v1/crates/"+crateName, nil)
	if err != nil {
		slog.ErrorContext(ctx, "creating crates request", "crate", crateName, "err", err)
		return "", fmt.Errorf("creating crates request for %s: %w", crateName, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing crates request for %s: %w", crateName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("crates api error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload crateResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decoding crates response for %s: %w", crateName, err)
	}
	return normalizeSemver(payload.MaxVersion), nil
}

func normalizeSemver(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	return value
}

func installToolArtifact(ctx context.Context, logger *telemetry.Logger, tool string, artifactPath string) error {
	slog.InfoContext(ctx, "tools: installToolArtifact")
	switch {
	case strings.HasSuffix(artifactPath, ".deb") || strings.HasSuffix(artifactPath, ".rpm"):
		if err := cmdexec.RunWithLogger(ctx, logger, "sudo", "dpkg", "-i", artifactPath); err != nil {
			slog.WarnContext(ctx, "tools: installToolArtifact dpkg failed", "artifact", filepath.Base(artifactPath), "err", err)
			return fmt.Errorf("installing deb/rpm package %s: %w", filepath.Base(artifactPath), err)
		}
		return nil
	case strings.HasSuffix(artifactPath, ".gz") && !strings.Contains(artifactPath, ".tar"):
		binPath := filepath.Join(os.Getenv("HOME"), ".local", "bin", tool)
		if err := os.MkdirAll(filepath.Dir(filepath.Clean(binPath)), 0o755); err != nil {
			slog.WarnContext(ctx, "tools: installToolArtifact mkdir bin failed", "err", err)
			return fmt.Errorf("creating bin directory: %w", err)
		}
		if err := decompressGzipBinary(artifactPath, binPath); err != nil {
			return err
		}
		return nil
	case strings.HasSuffix(artifactPath, ".zip") || strings.HasSuffix(artifactPath, ".tar.gz") || strings.HasSuffix(artifactPath, ".tar.xz"):
		tmpDir, err := os.MkdirTemp("", "dotfiles-tool-*")
		if err != nil {
			slog.WarnContext(ctx, "tools: installToolArtifact mkdirtemp failed", "err", err)
			return fmt.Errorf("creating temp directory: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		switch {
		case strings.HasSuffix(artifactPath, ".zip"):
			if err := cmdexec.RunWithLogger(ctx, logger, "unzip", "-o", artifactPath, "-d", tmpDir); err != nil {
				slog.WarnContext(ctx, "tools: installToolArtifact unzip failed", "err", err)
				return fmt.Errorf("extracting zip archive: %w", err)
			}
		case strings.HasSuffix(artifactPath, ".xz"):
			if err := cmdexec.RunWithLogger(ctx, logger, "tar", "xJf", artifactPath, "-C", tmpDir); err != nil {
				slog.WarnContext(ctx, "tools: installToolArtifact tar xz failed", "err", err)
				return fmt.Errorf("extracting xz archive: %w", err)
			}
		default:
			if err := cmdexec.RunWithLogger(ctx, logger, "tar", "xzf", artifactPath, "-C", tmpDir); err != nil {
				slog.WarnContext(ctx, "tools: installToolArtifact tar gz failed", "err", err)
				return fmt.Errorf("extracting tar archive: %w", err)
			}
		}

		return installFromDir(tool, tmpDir)
	default:
		return fmt.Errorf("unsupported artifact type for %s", filepath.Base(artifactPath))
	}
}

func decompressGzipBinary(source, destination string) error {
	slog.Info("tools: decompressGzipBinary")
	input, err := os.Open(source)
	if err != nil {
		slog.Warn("tools: decompressGzipBinary open failed", "source", source, "err", err)
		return fmt.Errorf("opening gzip file %s: %w", source, err)
	}
	defer input.Close()
	reader, err := gzip.NewReader(input)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer reader.Close()

	if err := os.MkdirAll(filepath.Dir(filepath.Clean(destination)), 0o755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}
	output, err := os.Create(filepath.Clean(destination))
	if err != nil {
		return fmt.Errorf("creating destination file %s: %w", destination, err)
	}
	defer output.Close()
	const maxDecompressSize = 512 * 1024 * 1024 // 512MB
	if _, err := io.Copy(output, io.LimitReader(reader, maxDecompressSize)); err != nil {
		return fmt.Errorf("decompressing gzip data: %w", err)
	}
	if err := os.Chmod(filepath.Clean(destination), 0o755); err != nil {
		return fmt.Errorf("setting permissions on %s: %w", destination, err)
	}
	return nil
}

func installFromDir(tool, dir string) error {
	targetDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	if err := os.MkdirAll(filepath.Clean(targetDir), 0o755); err != nil {
		slog.Warn("tools: installFromDir mkdir failed", "err", err)
		return fmt.Errorf("creating target directory: %w", err)
	}
	bin := filepath.Join(targetDir, tool)
	found := ""
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == tool || strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())) == tool {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	if found == "" {
		slog.Warn("tools: installFromDir binary not found", "tool", tool)
		return fmt.Errorf("binary %s not found in archive", tool)
	}
	return copyExecutable(found, bin)
}

func copyExecutable(src, dst string) error {
	cleanSrc := filepath.Clean(src)
	cleanDst := filepath.Clean(dst)
	slog.Info("tools: copyExecutable")
	payload, err := os.ReadFile(cleanSrc)
	if err != nil {
		slog.Warn("tools: copyExecutable read failed", "err", err)
		return fmt.Errorf("reading executable %s: %w", src, err)
	}
	if err := os.WriteFile(cleanDst, payload, 0o600); err != nil {
		slog.Warn("tools: copyExecutable write failed", "err", err)
		return fmt.Errorf("writing executable %s: %w", dst, err)
	}
	if err := os.Chmod(cleanDst, 0o755); err != nil {
		slog.Warn("tools: copyExecutable chmod failed", "err", err)
		return fmt.Errorf("setting permissions on %s: %w", dst, err)
	}
	return nil
}

func getSyncLogPath() string {
	if path := os.Getenv("DOTFILES_LOG"); path != "" {
		return path
	}
	return filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "sync.log")
}

// DownloadToTempFile downloads fileURL to a temporary file and returns its path.
func DownloadToTempFile(ctx context.Context, _ *telemetry.Logger, fileURL string) (string, error) {
	return downloadToTempFile(ctx, fileURL)
}

func downloadToTempFile(ctx context.Context, fileURL string) (string, error) {
	slog.InfoContext(ctx, "tools: downloadToTempFile")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		slog.ErrorContext(ctx, "creating download request", "url", fileURL, "err", err)
		return "", fmt.Errorf("creating request for %s: %w", fileURL, err)
	}
	request.Header.Set("User-Agent", "dots")
	client := &http.Client{Timeout: 120 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("executing download request for %s: %w", fileURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		return "", fmt.Errorf("download failed for %s: %d %s", fileURL, response.StatusCode, strings.TrimSpace(string(body)))
	}

	tmp, err := os.CreateTemp("", "dots-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	_, err = io.Copy(tmp, response.Body)
	if err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("copying response body: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", fmt.Errorf("setting permissions on temp file: %w", err)
	}
	return tmp.Name(), nil
}
