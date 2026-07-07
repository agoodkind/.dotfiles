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
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
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
func InstallCustomTools(ctx context.Context, _ string, strictMode bool, logger *telemetry.Logger) error {
	return runCustomTools(ctx, strictMode, logger)
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type crateResponse struct {
	Crate struct {
		MaxVersion string `json:"max_version"`
	} `json:"crate"`
}

func runCustomTools(ctx context.Context, strictMode bool, logger *telemetry.Logger) error {
	entries := catalog.DefaultToolDeclarations()
	failed := make([]string, 0)
	for _, tool := range entries {
		if tool.ID == "" || tool.Bin == "" {
			continue
		}
		if err := installCustomTool(ctx, tool, strictMode, logger); err != nil {
			failed = append(failed, tool.ID)
			_ = telemetry.Notify("warn", "tool install/upgrade failed: "+tool.ID, common.SyncLogPath(), telemetry.RunID(ctx))
			common.WarnContextf(ctx, logger, "  %s: failed: %s", tool.ID, err.Error())
			logger.WarnContextWithErr(ctx, "  "+tool.ID+": failed", err)
		}
	}
	if len(failed) > 0 {
		message := "custom tools completed with failures: " + strings.Join(failed, ", ")
		logger.WarnContext(ctx, "  "+message)
		if strictMode {
			return fmt.Errorf("%s", message)
		}
		return nil
	}
	return nil
}

func installCustomTool(ctx context.Context, tool catalog.ToolDeclaration, strictMode bool, logger *telemetry.Logger) error {
	if !isPlatformAllowed(tool) {
		return nil
	}
	if tool.InstallMethod == "" {
		logger.WarnContext(ctx, "  skipping tool with no install method: "+tool.ID)
		return nil
	}
	current := getCurrentToolVersion(ctx, tool.Bin, logger)
	if strictMode && current != "" {
		logger.InfoContext(ctx, "  "+tool.ID+" is installed ("+current+")")
		return nil
	}
	if current == "" {
		return runToolInstall(ctx, tool, strictMode, logger)
	}
	latest, err := resolveLatestVersion(ctx, tool)
	if err != nil {
		return err
	}
	if latest != "" && shouldSkipToolUpgrade(current, latest) {
		logger.InfoContext(ctx, "  "+tool.ID+" is up to date ("+current+")")
		return nil
	}
	return runToolInstall(ctx, tool, strictMode, logger)
}

func runToolInstall(ctx context.Context, tool catalog.ToolDeclaration, strictMode bool, logger *telemetry.Logger) error {
	logger.InfoContext(ctx, "  installing "+tool.ID)
	switch installMethod(tool.InstallMethod) {
	case installMethodScript:
		return installToolFromScript(ctx, tool.ID, tool.ScriptURL, tool.ScriptArgs, logger)
	case installMethodCargo:
		return installToolViaCargo(ctx, tool, strictMode, logger)
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
	if tool.Version != "" {
		return tool.Version, nil
	}
	if tool.Repo != "" {
		return getLatestGitHubVersion(ctx, tool.Repo)
	}
	if tool.CrateName != "" {
		return getLatestCrateVersion(ctx, tool.CrateName)
	}
	return "", nil
}

func installToolViaCargo(ctx context.Context, tool catalog.ToolDeclaration, strictMode bool, logger *telemetry.Logger) error {
	if !strictMode && os.Getenv("GITHUB_ACTIONS") == "true" {
		return nil
	}
	crateName := tool.CrateName
	if tool.Version != "" {
		crateName += "@" + tool.Version
	}
	if err := cmdexec.RunWithLogger(ctx, logger, CargoExecutable(), "install", crateName, "--locked", "--force"); err != nil {
		slog.WarnContext(ctx, "tools: installToolViaCargo failed", "crate", tool.CrateName, "err", err)
		return fmt.Errorf("running cargo install %s: %w", tool.CrateName, err)
	}
	return nil
}

// CargoExecutable resolves the cargo binary to a concrete path so installs work
// even when ~/.cargo/bin is not yet on PATH within this process. The rust
// bootstrap, and CI's rust-toolchain action, installs cargo under $CARGO_HOME or
// $HOME/.cargo, which the restricted bootstrap PATH does not include, so a bare
// "cargo" lookup misses it (this is what failed the macOS smoke: CARGO_HOME held
// the real binary while the smoke ran under a temp HOME and a system-only PATH).
// This mirrors how the Go bootstrap resolves GO_BINARY to an absolute path.
// Resolution order: PATH, then $CARGO_HOME/bin, then $HOME/.cargo/bin, then the
// bare name as a last resort.
func CargoExecutable() string {
	if resolved, err := exec.LookPath("cargo"); err == nil {
		return resolved
	}
	for _, dir := range cargoBinDirs() {
		candidate := filepath.Join(dir, "cargo")
		if isExecutableFile(candidate) {
			return candidate
		}
	}
	return "cargo"
}

// CargoAvailable reports whether cargo can be resolved on PATH or at the
// canonical $CARGO_HOME / $HOME/.cargo install location.
func CargoAvailable() bool {
	return CargoExecutable() != "cargo"
}

func cargoBinDirs() []string {
	dirs := make([]string, 0, 2)
	if cargoHome := os.Getenv("CARGO_HOME"); cargoHome != "" {
		dirs = append(dirs, filepath.Join(cargoHome, "bin"))
	}
	if home := os.Getenv("HOME"); home != "" {
		dirs = append(dirs, filepath.Join(home, ".cargo", "bin"))
	}
	return dirs
}

func isExecutableFile(candidate string) bool {
	info, err := os.Stat(filepath.Clean(candidate))
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Mode()&0o111 != 0
}

func installToolFromScript(ctx context.Context, name, scriptURL string, args []string, logger *telemetry.Logger) error {
	slog.InfoContext(ctx, "tools: installToolFromScript")
	logger.InfoContext(ctx, "  installing "+name)
	scriptPath, err := downloadToTempFile(ctx, scriptURL)
	if err != nil {
		return err
	}
	defer os.Remove(scriptPath)
	localBin := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		slog.WarnContext(ctx, "tools: installToolFromScript mkdir local bin failed", "name", name, "err", err)
		return fmt.Errorf("creating local bin for %s: %w", name, err)
	}
	expandedArgs := expandScriptArgs(args)
	cmdArgs := append([]string{scriptPath}, expandedArgs...)
	if err := cmdexec.RunWithLogger(ctx, logger, "sh", cmdArgs...); err != nil {
		slog.WarnContext(ctx, "tools: installToolFromScript run failed", "name", name, "err", err)
		return fmt.Errorf("running install script for %s: %w", name, err)
	}
	return nil
}

func expandScriptArgs(args []string) []string {
	expandedArgs := make([]string, 0, len(args))
	home := os.Getenv("HOME")
	for _, arg := range args {
		expandedArg := strings.ReplaceAll(arg, "${HOME}", home)
		expandedArg = strings.ReplaceAll(expandedArg, "$HOME", home)
		expandedArgs = append(expandedArgs, expandedArg)
	}
	return expandedArgs
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

// githubToken returns a token for authenticating GitHub API requests, checking
// GITHUB_TOKEN, then GH_TOKEN, then the gh CLI's stored credential. Without one
// the releases API rate-limits unauthenticated requests, which is what made tool
// installs fail on hosts whose dispatch shell never exported a token.
func githubToken(ctx context.Context) string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token
	}
	if _, err := exec.LookPath("gh"); err != nil {
		slog.DebugContext(ctx, "github token: no GITHUB_TOKEN/GH_TOKEN and gh not on PATH; requests will be unauthenticated")
		return ""
	}
	out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	if err != nil {
		slog.DebugContext(ctx, "github token: gh auth token failed; requests will be unauthenticated", "err", err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

func fetchLatestRelease(ctx context.Context, repo string) (githubRelease, error) {
	var rel githubRelease
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		slog.ErrorContext(ctx, "creating github request", "repo", repo, "err", err)
		return rel, fmt.Errorf("creating github request for %s: %w", repo, err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "dots")
	if token := githubToken(ctx); token != "" {
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
	req.Header.Set("User-Agent", "dots (+https://goodkind.io/dots)")
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
	return normalizeSemver(payload.Crate.MaxVersion), nil
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
		command := "sudo"
		args := []string{"dpkg", "-i", artifactPath}
		if os.Geteuid() == 0 || !runnerHasSudo() {
			command = "dpkg"
			args = []string{"-i", artifactPath}
		}
		if err := cmdexec.RunWithLogger(ctx, logger, command, args...); err != nil {
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
	found, err := findArchiveToolBinary(dir, tool)
	if err != nil {
		return err
	}
	if found == "" {
		slog.Warn("tools: installFromDir binary not found", "tool", tool)
		return fmt.Errorf("binary %s not found in archive", tool)
	}
	return copyExecutable(found, bin)
}

func findArchiveToolBinary(dir string, tool string) (string, error) {
	found, err := findArchiveEntry(dir, func(_ string, entry os.DirEntry) (bool, error) {
		return entry.Name() == tool || strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())) == tool, nil
	})
	if err != nil || found != "" {
		return found, err
	}
	return findArchiveEntry(dir, func(path string, entry os.DirEntry) (bool, error) {
		return isPrefixedExecutableArchiveEntry(path, entry, tool)
	})
}

func findArchiveEntry(dir string, matches func(string, os.DirEntry) (bool, error)) (string, error) {
	found := ""
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("tools: findArchiveEntry walk failed", "err", err)
			return fmt.Errorf("walking archive directory: %w", err)
		}
		if entry.IsDir() {
			return nil
		}
		matched, matchErr := matches(path, entry)
		if matchErr != nil {
			return matchErr
		}
		if matched {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return found, fmt.Errorf("walking archive directory %s: %w", dir, err)
	}
	return found, nil
}

func isPrefixedExecutableArchiveEntry(path string, entry os.DirEntry, tool string) (bool, error) {
	if !strings.HasPrefix(entry.Name(), tool+"_") && !strings.HasPrefix(entry.Name(), tool+"-") {
		return false, nil
	}
	info, err := entry.Info()
	if err != nil {
		slog.Warn("tools: isPrefixedExecutableArchiveEntry info failed", "path", path, "err", err)
		return false, fmt.Errorf("reading archive entry %s: %w", path, err)
	}
	return info.Mode()&0o111 != 0, nil
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

	tmp, err := os.CreateTemp("", "dots-*"+artifactSuffix(fileURL))
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

func artifactSuffix(fileURL string) string {
	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return ""
	}
	name := path.Base(parsedURL.Path)
	for _, suffix := range []string{".tar.gz", ".tar.xz", ".deb", ".rpm", ".zip", ".gz"} {
		if strings.HasSuffix(name, suffix) {
			return suffix
		}
	}
	return ""
}

func runnerHasSudo() bool {
	return runner.HasCommand("sudo")
}
