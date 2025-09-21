package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v50/github"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gocd/internal/logger"
)

type GitHubClient struct {
	client *github.Client
	ctx    context.Context
}

type RepositoryChange struct {
	Type       string    // "push" or "pr"
	Commit     string    // Latest commit hash
	Branch     string    // Branch name
	Timestamp  time.Time // When the change occurred
	PullNumber int       // PR number (if applicable)
}

func NewGitHubClient(token string) *GitHubClient {
	ctx := context.Background()
	var client *github.Client

	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	} else {
		// Use unauthenticated client (lower rate limits)
		client = github.NewClient(nil)
	}

	return &GitHubClient{
		client: client,
		ctx:    ctx,
	}
}

func (gc *GitHubClient) CheckForChanges(repoURL, branch, trigger string, lastSync time.Time) (*RepositoryChange, error) {
	owner, repo, err := gc.parseRepoURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository URL: %w", err)
	}

	logger.Debug("Checking for changes",
		zap.String("owner", owner),
		zap.String("repo", repo),
		zap.String("branch", branch),
		zap.String("trigger", trigger),
		zap.Time("last_sync", lastSync),
	)

	switch trigger {
	case "push":
		return gc.checkForPushChanges(owner, repo, branch, lastSync)
	case "pr":
		return gc.checkForPRChanges(owner, repo, branch, lastSync)
	case "both":
		// Check for both push and PR changes, return the most recent
		pushChange, pushErr := gc.checkForPushChanges(owner, repo, branch, lastSync)
		prChange, prErr := gc.checkForPRChanges(owner, repo, branch, lastSync)

		if pushErr != nil && prErr != nil {
			return nil, fmt.Errorf("failed to check both push and PR changes: push error: %v, PR error: %v", pushErr, prErr)
		}

		// Return the most recent change
		if pushChange != nil && prChange != nil {
			if pushChange.Timestamp.After(prChange.Timestamp) {
				return pushChange, nil
			}
			return prChange, nil
		}

		if pushChange != nil {
			return pushChange, nil
		}

		return prChange, nil
	default:
		return nil, fmt.Errorf("invalid trigger type: %s", trigger)
	}
}

func (gc *GitHubClient) checkForPushChanges(owner, repo, branch string, lastSync time.Time) (*RepositoryChange, error) {
	// Get the latest commit on the specified branch
	commits, _, err := gc.client.Repositories.ListCommits(gc.ctx, owner, repo, &github.CommitsListOptions{
		SHA:   branch,
		Since: lastSync,
		ListOptions: github.ListOptions{
			PerPage: 1,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list commits: %w", err)
	}

	if len(commits) == 0 {
		// No new commits
		return nil, nil
	}

	latestCommit := commits[0]
	commitTime := latestCommit.GetCommit().GetCommitter().GetDate().Time

	// Only return if there's a newer commit than our last sync
	if commitTime.After(lastSync) {
		return &RepositoryChange{
			Type:      "push",
			Commit:    latestCommit.GetSHA(),
			Branch:    branch,
			Timestamp: commitTime,
		}, nil
	}

	return nil, nil
}

func (gc *GitHubClient) checkForPRChanges(owner, repo, branch string, lastSync time.Time) (*RepositoryChange, error) {
	// Get recent PRs that target the specified branch
	prs, _, err := gc.client.PullRequests.List(gc.ctx, owner, repo, &github.PullRequestListOptions{
		State: "closed",
		Base:  branch,
		Sort:  "updated",
		ListOptions: github.ListOptions{
			PerPage: 10,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests: %w", err)
	}

	// Find the most recent merged PR
	for _, pr := range prs {
		if pr.GetMergedAt().After(lastSync) && pr.GetMerged() {
			return &RepositoryChange{
				Type:       "pr",
				Commit:     pr.GetMergeCommitSHA(),
				Branch:     branch,
				Timestamp:  pr.GetMergedAt().Time,
				PullNumber: pr.GetNumber(),
			}, nil
		}
	}

	return nil, nil
}

func (gc *GitHubClient) GetLatestCommit(repoURL, branch string) (string, error) {
	owner, repo, err := gc.parseRepoURL(repoURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse repository URL: %w", err)
	}

	commit, _, err := gc.client.Repositories.GetCommit(gc.ctx, owner, repo, branch, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get latest commit: %w", err)
	}

	return commit.GetSHA(), nil
}

func (gc *GitHubClient) GetRateLimit() (*github.Rate, error) {
	rateLimit, _, err := gc.client.RateLimits(gc.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rate limit: %w", err)
	}

	return rateLimit.Core, nil
}

func (gc *GitHubClient) parseRepoURL(repoURL string) (owner, repo string, err error) {
	// Clean up the URL
	repoURL = strings.TrimSpace(repoURL)

	// Handle different URL formats
	if strings.HasPrefix(repoURL, "git@github.com:") {
		// SSH format: git@github.com:owner/repo.git
		parts := strings.TrimPrefix(repoURL, "git@github.com:")
		parts = strings.TrimSuffix(parts, ".git")
		repoParts := strings.Split(parts, "/")
		if len(repoParts) != 2 {
			return "", "", fmt.Errorf("invalid SSH repository URL format")
		}
		return repoParts[0], repoParts[1], nil
	}

	if strings.HasPrefix(repoURL, "https://github.com/") {
		// HTTPS format: https://github.com/owner/repo.git
		parts := strings.TrimPrefix(repoURL, "https://github.com/")
		parts = strings.TrimSuffix(parts, ".git")
		repoParts := strings.Split(parts, "/")
		if len(repoParts) != 2 {
			return "", "", fmt.Errorf("invalid HTTPS repository URL format")
		}
		return repoParts[0], repoParts[1], nil
	}

	if strings.HasPrefix(repoURL, "github.com/") {
		// Simple format: github.com/owner/repo
		parts := strings.TrimPrefix(repoURL, "github.com/")
		parts = strings.TrimSuffix(parts, ".git")
		repoParts := strings.Split(parts, "/")
		if len(repoParts) != 2 {
			return "", "", fmt.Errorf("invalid repository URL format")
		}
		return repoParts[0], repoParts[1], nil
	}

	// Try to parse as owner/repo format
	repoParts := strings.Split(repoURL, "/")
	if len(repoParts) == 2 {
		return repoParts[0], repoParts[1], nil
	}

	return "", "", fmt.Errorf("unrecognized repository URL format: %s", repoURL)
}

func (gc *GitHubClient) ValidateRepository(repoURL string) error {
	owner, repo, err := gc.parseRepoURL(repoURL)
	if err != nil {
		return err
	}

	_, _, err = gc.client.Repositories.Get(gc.ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("repository not found or not accessible: %w", err)
	}

	return nil
}