package githook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"goodkind.io/.dotfiles/internal/cmdexec"
)

const protectionLookupFailure = "GitHub pull request protection lookup failed: "

var (
	githubRepositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	errGitHubDefaultBranch  = errors.New("GitHub did not return a default branch")
	errGitHubOrigin         = errors.New(
		"origin does not specify a valid GitHub owner and repository",
	)
	errGitHubProtection = errors.New("GitHub returned an invalid protection result")
)

func primaryMutationsBlocked(ctx context.Context) bool {
	protected, err := defaultBranchRequiresPullRequest(ctx)
	if err != nil {
		slog.WarnContext(
			ctx,
			"allowing the mutation because GitHub protection lookup failed",
			"err",
			err,
		)

		return false
	}

	return protected
}

func defaultBranchRequiresPullRequest(ctx context.Context) (bool, error) {
	slog.DebugContext(ctx, "reading GitHub pull request protection")
	repository, err := githubOriginRepository(ctx)
	if err != nil || repository == "" {
		return false, err
	}

	output, err := cmdexec.Output(
		ctx,
		"gh",
		"repo",
		"view",
		"github.com/"+repository,
		"--json",
		"defaultBranchRef",
	)
	if err != nil {
		return false, fmt.Errorf(
			protectionLookupFailure+"cannot read the default branch of %s: %w",
			repository,
			err,
		)
	}

	var result struct {
		DefaultBranch struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return false, fmt.Errorf(
			protectionLookupFailure+"cannot read the default branch of %s: %w",
			repository,
			err,
		)
	}

	if result.DefaultBranch.Name == "" {
		return false, fmt.Errorf(
			protectionLookupFailure+"%w for %s",
			errGitHubDefaultBranch,
			repository,
		)
	}

	return githubBranchRequiresPullRequest(ctx, repository, result.DefaultBranch.Name)
}

func githubOriginRepository(ctx context.Context) (string, error) {
	remotes, err := cmdexec.Output(ctx, "git", "remote")
	if err != nil {
		return "", fmt.Errorf(protectionLookupFailure+"cannot read Git remotes for .: %w", err)
	}

	if !slices.Contains(strings.Split(remotes, "\n"), "origin") {
		return "", nil
	}

	origin, err := cmdexec.Output(ctx, "git", "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf(protectionLookupFailure+"cannot read the origin URL for .: %w", err)
	}

	origin = strings.TrimRight(origin, "\n")
	prefixes := [...]string{
		"https://github.com/",
		"http://github.com/",
		"git@github.com:",
		"ssh://git@github.com/",
	}

	for _, prefix := range prefixes {
		repository, matches := strings.CutPrefix(origin, prefix)
		if !matches {
			continue
		}

		repository = strings.TrimSuffix(strings.TrimSuffix(repository, "/"), ".git")
		if !githubRepositoryPattern.MatchString(repository) {
			return "", fmt.Errorf(protectionLookupFailure+"%w", errGitHubOrigin)
		}

		return repository, nil
	}

	return "", nil
}

func githubBranchRequiresPullRequest(ctx context.Context, repository, branch string) (bool, error) {
	encodedBranch := strings.ReplaceAll(url.QueryEscape(branch), "+", "%20")
	endpoint := "repos/" + repository + "/rules/branches/" + encodedBranch
	output, err := cmdexec.Output(
		ctx,
		"gh",
		"api",
		"--hostname",
		"github.com",
		endpoint,
		"--paginate",
		"--slurp",
	)
	if err != nil {
		return false, fmt.Errorf(
			protectionLookupFailure+"cannot read active rules for the default branch of %s: %w",
			repository,
			err,
		)
	}

	var pages [][]struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(output), &pages); err != nil {
		return false, fmt.Errorf(
			protectionLookupFailure+"%w for %s: %w",
			errGitHubProtection,
			repository,
			err,
		)
	}

	if len(pages) == 0 {
		return false, fmt.Errorf(
			protectionLookupFailure+"%w for %s",
			errGitHubProtection,
			repository,
		)
	}

	protected := false

	for _, page := range pages {
		if page == nil {
			return false, fmt.Errorf(
				protectionLookupFailure+"%w for %s",
				errGitHubProtection,
				repository,
			)
		}

		for _, rule := range page {
			if rule.Type == "pull_request" {
				protected = true
			}
		}
	}

	return protected, nil
}
