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
	addedRepos := make([]catalog.AptRepo, 0, len(cfg.AptRepos))
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
		addedRepos = append(addedRepos, repo)
		if repo.ID == ooklaSpeedtestRepoID {
			ooklaReady = true
		}
	}

	if len(addedRepos) == 0 {
		return ooklaReady
	}
	if err := installer.deps.Privileged.Run(ctx, logger, "apt-get", "update", "-qq"); err != nil {
		slog.WarnContext(ctx, "installAptRepos: apt-get update after repos", "err", err)
		common.WarnContext(ctx, logger, "  apt-get update after apt repos failed")
		installer.rollbackAptRepos(ctx, logger, addedRepos)
		return false
	}

	return ooklaReady
}

func (installer *Installer) rollbackAptRepos(
	ctx context.Context,
	logger *telemetry.Logger,
	repos []catalog.AptRepo,
) {
	slog.WarnContext(ctx, "debian: rolling back apt repos after apt-get update failure", "count", len(repos))
	for _, repo := range repos {
		if repo.ListPath != "" {
			if err := installer.deps.Privileged.Run(ctx, logger, "rm", "-f", repo.ListPath); err != nil {
				slog.WarnContext(ctx, "debian: removing apt repo list during rollback", "id", repo.ID, "path", repo.ListPath, "err", err)
				common.WarnContextf(ctx, logger, "  failed to remove apt repo list %s", repo.ListPath)
			}
		}
		if repo.Keyring != "" {
			if err := installer.deps.Privileged.Run(ctx, logger, "rm", "-f", repo.Keyring); err != nil {
				slog.WarnContext(ctx, "debian: removing apt repo keyring during rollback", "id", repo.ID, "path", repo.Keyring, "err", err)
				common.WarnContextf(ctx, logger, "  failed to remove apt repo keyring %s", repo.Keyring)
			}
		}
	}
}

func (installer *Installer) installAptRepo(
	ctx context.Context,
	host platform.Host,
	repo catalog.AptRepo,
	logger *telemetry.Logger,
) (bool, error) {
	slog.InfoContext(ctx, "debian: installing apt repo", "id", repo.ID)
	if repo.GPGURL == "" || repo.Keyring == "" || repo.ListPath == "" {
		err := fmt.Errorf("apt repo %s is missing gpg_url, keyring, or list_path", repo.ID)
		slog.WarnContext(ctx, "installAptRepo: invalid repo", "id", repo.ID, "err", err)
		return false, err
	}

	baseURL := aptRepoBaseURL(repo, host)
	if baseURL == "" {
		err := fmt.Errorf("apt repo %s has no base URL for this distribution", repo.ID)
		slog.WarnContext(ctx, "installAptRepo: missing base URL", "id", repo.ID, "err", err)
		return false, err
	}

	suite, ok := installer.selectAptRepoSuite(ctx, host, baseURL)
	if !ok {
		slog.InfoContext(ctx, "debian: skipping unpublished apt repo", "id", repo.ID)
		common.InfoContextf(ctx, logger, "  skipping apt repo %s: no release published for this OS version", repo.ID)
		return false, nil
	}

	if err := installer.installAptRepoKeyring(ctx, repo, logger); err != nil {
		slog.WarnContext(ctx, "installAptRepo: keyring", "id", repo.ID, "err", err)
		return false, err
	}
	if err := installer.installAptRepoList(ctx, repo, baseURL, suite, logger); err != nil {
		slog.WarnContext(ctx, "installAptRepo: sources list", "id", repo.ID, "err", err)
		return false, err
	}

	slog.InfoContext(ctx, "debian: added apt repo", "id", repo.ID, "suite", suite)
	common.InfoContextf(ctx, logger, "  added apt repo %s (%s)", repo.ID, suite)
	return true, nil
}

func (installer *Installer) installAptRepoKeyring(
	ctx context.Context,
	repo catalog.AptRepo,
	logger *telemetry.Logger,
) error {
	slog.InfoContext(ctx, "debian: installing apt repo keyring", "id", repo.ID, "keyring", repo.Keyring)
	key, err := installer.aptRepoDownload(ctx, repo.GPGURL)
	if err != nil {
		slog.WarnContext(ctx, "installAptRepoKeyring: download gpg key", "id", repo.ID, "err", err)
		return fmt.Errorf("download gpg key: %w", err)
	}
	armoredPath, cleanupArmored, err := writeAptRepoTempFile("dotfiles-apt-gpg-*", key)
	if err != nil {
		slog.WarnContext(ctx, "installAptRepoKeyring: write gpg temp file", "id", repo.ID, "err", err)
		return fmt.Errorf("write gpg temp file: %w", err)
	}
	defer cleanupArmored()

	dearmoredPath, cleanupDearmored, err := writeAptRepoTempFile("dotfiles-apt-keyring-*", nil)
	if err != nil {
		slog.WarnContext(ctx, "installAptRepoKeyring: create keyring temp file", "id", repo.ID, "err", err)
		return fmt.Errorf("create keyring temp file: %w", err)
	}
	defer cleanupDearmored()

	if installer.deps.Commands == nil {
		err := fmt.Errorf("gpg is unavailable")
		slog.WarnContext(ctx, "installAptRepoKeyring: gpg unavailable", "id", repo.ID, "err", err)
		return err
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
		slog.WarnContext(ctx, "installAptRepoKeyring: dearmor gpg key", "id", repo.ID, "err", err)
		return fmt.Errorf("dearmor gpg key: %w", err)
	}

	keyringDir := filepath.Dir(repo.Keyring)
	if err := installer.deps.Privileged.Run(ctx, logger, "install", "-d", "-m", "755", keyringDir); err != nil {
		slog.WarnContext(ctx, "installAptRepoKeyring: create keyring directory", "id", repo.ID, "err", err)
		return fmt.Errorf("create keyring directory: %w", err)
	}
	if err := installer.deps.Privileged.Run(ctx, logger, "install", "-m", "644", dearmoredPath, repo.Keyring); err != nil {
		slog.WarnContext(ctx, "installAptRepoKeyring: install keyring", "id", repo.ID, "err", err)
		return fmt.Errorf("install keyring: %w", err)
	}
	return nil
}

func (installer *Installer) installAptRepoList(
	ctx context.Context,
	repo catalog.AptRepo,
	baseURL string,
	suite string,
	logger *telemetry.Logger,
) error {
	slog.InfoContext(ctx, "debian: installing apt repo sources list", "id", repo.ID, "list_path", repo.ListPath)
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
	listPath, cleanup, err := writeAptRepoTempFile("dotfiles-apt-list-*", []byte(listLine))
	if err != nil {
		slog.WarnContext(ctx, "installAptRepoList: write list temp file", "id", repo.ID, "err", err)
		return fmt.Errorf("write list temp file: %w", err)
	}
	defer cleanup()

	listDir := filepath.Dir(repo.ListPath)
	if err := installer.deps.Privileged.Run(ctx, logger, "install", "-d", "-m", "755", listDir); err != nil {
		slog.WarnContext(ctx, "installAptRepoList: create sources directory", "id", repo.ID, "err", err)
		return fmt.Errorf("create sources directory: %w", err)
	}
	if err := installer.deps.Privileged.Run(ctx, logger, "install", "-m", "644", listPath, repo.ListPath); err != nil {
		slog.WarnContext(ctx, "installAptRepoList: install sources list", "id", repo.ID, "err", err)
		return fmt.Errorf("install sources list: %w", err)
	}
	return nil
}

func writeAptRepoTempFile(pattern string, data []byte) (string, func(), error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
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
		slog.DebugContext(ctx, "debian: building apt repo release probe failed; treating as unpublished", "url", releaseURL, "err", err)
		return false
	}
	client := &http.Client{Timeout: ppaReleaseProbeTimeout}
	response, err := client.Do(request)
	if err != nil {
		slog.DebugContext(ctx, "debian: apt repo release probe request failed; treating as unpublished", "url", releaseURL, "err", err)
		return false
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
