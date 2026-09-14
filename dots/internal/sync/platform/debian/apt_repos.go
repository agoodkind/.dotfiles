package debian

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/sync/platform"
	"goodkind.io/.dotfiles/internal/telemetry"
)

const (
	officialSpeedtestPackage            = "speedtest"
	unofficialSpeedtestCLIPackage       = "speedtest-cli"
	ooklaSpeedtestRepoID                = "ookla-speedtest"
	ubuntuJammyFallback                 = "jammy"
	aptRepoDownloadTimeout              = 30 * time.Second
	aptRepoDownloadByteLimit      int64 = 1 << 20
)

func (installer *Installer) installAptRepos(
	ctx context.Context,
	host platform.Host,
	cfg *catalog.PackageConfig,
	logger *telemetry.Logger,
) bool {
	if cfg == nil || len(cfg.AptRepos) == 0 {
		return false
	}

	if err := installer.deps.Privileged.Run(ctx, logger, "apt-get", "install", "-y", "-qq", "gnupg", "ca-certificates"); err != nil {
		slog.WarnContext(ctx, "installAptRepos: installing gnupg", "err", err)
		common.WarnContext(ctx, logger, "  failed to install gnupg; skipping apt repos")
		return false
	}

	ooklaReady := false
	added := false
	for _, repo := range cfg.AptRepos {
		ready, err := installer.installAptRepo(ctx, host, repo, logger)
		if err != nil {
			slog.WarnContext(ctx, "installAptRepos: adding apt repo", "id", repo.ID, "err", err)
			common.WarnContextf(ctx, logger, "  failed to add apt repo %s", repo.ID)
			continue
		}
		if !ready {
			continue
		}
		added = true
		if repo.ID == ooklaSpeedtestRepoID {
			ooklaReady = true
		}
	}

	if added {
		if err := installer.deps.Privileged.Run(ctx, logger, "apt-get", "update", "-qq"); err != nil {
			slog.WarnContext(ctx, "installAptRepos: apt-get update after repos", "err", err)
			common.WarnContext(ctx, logger, "  apt-get update after apt repos failed")
			return false
		}
	}

	return ooklaReady
}

func (installer *Installer) installAptRepo(
	ctx context.Context,
	host platform.Host,
	repo catalog.AptRepo,
	logger *telemetry.Logger,
) (bool, error) {
	if repo.GPGURL == "" || repo.Keyring == "" || repo.ListPath == "" {
		return false, fmt.Errorf("apt repo %s is missing gpg_url, keyring, or list_path", repo.ID)
	}

	baseURL := aptRepoBaseURL(repo, host)
	if baseURL == "" {
		return false, fmt.Errorf("apt repo %s has no base URL for this distribution", repo.ID)
	}

	suite, ok := installer.selectAptRepoSuite(ctx, host, baseURL)
	if !ok {
		common.InfoContextf(ctx, logger, "  skipping apt repo %s: no release published for this OS version", repo.ID)
		return false, nil
	}

	key, err := installer.aptRepoDownload(ctx, repo.GPGURL)
	if err != nil {
		return false, fmt.Errorf("download gpg key: %w", err)
	}

	armored, err := os.CreateTemp("", "dotfiles-apt-gpg-*")
	if err != nil {
		return false, fmt.Errorf("create gpg temp file: %w", err)
	}
	armoredPath := armored.Name()
	defer os.Remove(armoredPath)
	if _, err := armored.Write(key); err != nil {
		_ = armored.Close()
		return false, fmt.Errorf("write gpg temp file: %w", err)
	}
	if err := armored.Close(); err != nil {
		return false, fmt.Errorf("close gpg temp file: %w", err)
	}

	dearmored, err := os.CreateTemp("", "dotfiles-apt-keyring-*")
	if err != nil {
		return false, fmt.Errorf("create keyring temp file: %w", err)
	}
	dearmoredPath := dearmored.Name()
	defer os.Remove(dearmoredPath)
	if err := dearmored.Close(); err != nil {
		return false, fmt.Errorf("close keyring temp file: %w", err)
	}

	if installer.deps.Commands == nil {
		return false, fmt.Errorf("gpg is unavailable")
	}
	if err := installer.deps.Commands.RunWithLogger(
		ctx,
		logger,
		"gpg",
		"--batch",
		"--yes",
		"--dearmor",
		"-o",
		dearmoredPath,
		armoredPath,
	); err != nil {
		return false, fmt.Errorf("dearmor gpg key: %w", err)
	}

	keyringDir := filepath.Dir(repo.Keyring)
	if err := installer.deps.Privileged.Run(ctx, logger, "install", "-d", "-m", "755", keyringDir); err != nil {
		return false, fmt.Errorf("create keyring directory: %w", err)
	}
	if err := installer.deps.Privileged.Run(ctx, logger, "install", "-m", "644", dearmoredPath, repo.Keyring); err != nil {
		return false, fmt.Errorf("install keyring: %w", err)
	}

	component := repo.Component
	if component == "" {
		component = "main"
	}
	listLine := fmt.Sprintf(
		"deb [signed-by=%s] %s %s %s\n",
		repo.Keyring,
		strings.TrimSuffix(baseURL, "/")+"/",
		suite,
		component,
	)
	listFile, err := os.CreateTemp("", "dotfiles-apt-list-*")
	if err != nil {
		return false, fmt.Errorf("create list temp file: %w", err)
	}
	listPath := listFile.Name()
	defer os.Remove(listPath)
	if _, err := listFile.WriteString(listLine); err != nil {
		_ = listFile.Close()
		return false, fmt.Errorf("write list temp file: %w", err)
	}
	if err := listFile.Close(); err != nil {
		return false, fmt.Errorf("close list temp file: %w", err)
	}

	listDir := filepath.Dir(repo.ListPath)
	if err := installer.deps.Privileged.Run(ctx, logger, "install", "-d", "-m", "755", listDir); err != nil {
		return false, fmt.Errorf("create sources directory: %w", err)
	}
	if err := installer.deps.Privileged.Run(ctx, logger, "install", "-m", "644", listPath, repo.ListPath); err != nil {
		return false, fmt.Errorf("install sources list: %w", err)
	}

	common.InfoContextf(ctx, logger, "  added apt repo %s (%s)", repo.ID, suite)
	return true, nil
}

func (installer *Installer) selectAptRepoSuite(ctx context.Context, host platform.Host, baseURL string) (string, bool) {
	codename := installer.aptRepoCodename()
	if codename != "" && installer.aptRepoHasRelease(ctx, baseURL, codename) {
		return codename, true
	}
	if host.Distribution == platform.DistributionUbuntu && installer.aptRepoHasRelease(ctx, baseURL, ubuntuJammyFallback) {
		return ubuntuJammyFallback, true
	}
	return "", false
}

func (installer *Installer) aptRepoCodename() string {
	if installer.deps.AptRepos != nil {
		return installer.deps.AptRepos.Codename()
	}
	return ubuntuReleaseCodename()
}

func (installer *Installer) aptRepoHasRelease(ctx context.Context, baseURL, suite string) bool {
	if installer.deps.AptRepos != nil {
		return installer.deps.AptRepos.HasRelease(ctx, baseURL, suite)
	}
	return aptRepoPublishesRelease(ctx, baseURL, suite)
}

func (installer *Installer) aptRepoDownload(ctx context.Context, fileURL string) ([]byte, error) {
	if installer.deps.AptRepos != nil {
		return installer.deps.AptRepos.Download(ctx, fileURL)
	}
	return downloadAptRepoFile(ctx, fileURL)
}

func aptRepoBaseURL(repo catalog.AptRepo, host platform.Host) string {
	if host.Distribution == platform.DistributionDebian {
		return repo.DebianBase
	}
	return repo.UbuntuBase
}

func aptRepoPublishesRelease(ctx context.Context, baseURL, suite string) bool {
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(suite) == "" {
		return false
	}
	releaseURL := strings.TrimSuffix(baseURL, "/") + "/dists/" + suite + "/Release"
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, releaseURL, nil)
	if err != nil {
		slog.DebugContext(ctx, "debian: building apt repo release probe failed; treating as published", "url", releaseURL, "err", err)
		return true
	}
	client := &http.Client{Timeout: ppaReleaseProbeTimeout}
	response, err := client.Do(request)
	if err != nil {
		slog.DebugContext(ctx, "debian: apt repo release probe request failed; treating as published", "url", releaseURL, "err", err)
		return true
	}
	defer response.Body.Close()
	published := response.StatusCode != http.StatusNotFound && response.StatusCode != http.StatusGone
	slog.DebugContext(ctx, "debian: probed apt repo release", "url", releaseURL, "status", response.StatusCode, "published", published)
	return published
}

func downloadAptRepoFile(ctx context.Context, fileURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		slog.WarnContext(ctx, "debian: building apt repo download failed", "url", fileURL, "err", err)
		return nil, fmt.Errorf("build download for %s: %w", fileURL, err)
	}
	client := &http.Client{Timeout: aptRepoDownloadTimeout}
	response, err := client.Do(request)
	if err != nil {
		slog.WarnContext(ctx, "debian: apt repo download failed", "url", fileURL, "err", err)
		return nil, fmt.Errorf("download %s: %w", fileURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s: status %d", fileURL, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, aptRepoDownloadByteLimit))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", fileURL, err)
	}
	return body, nil
}
