package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"go.uber.org/zap"
	"gocd/internal/logger"
)

type GitManager struct{
	githubToken string
}

func NewGitManager() *GitManager {
	return &GitManager{}
}

func NewGitManagerWithToken(token string) *GitManager {
	return &GitManager{
		githubToken: token,
	}
}

func (gm *GitManager) ValidateOrSetupRepo(repoURL, branch, localPath string) error {
	logger.Info("Validating repository setup",
		zap.String("repo", repoURL),
		zap.String("branch", branch),
		zap.String("path", localPath),
	)

	// Check if directory exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		// Directory doesn't exist, create it and clone
		if err := os.MkdirAll(localPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", localPath, err)
		}
		return gm.cloneRepository(repoURL, branch, localPath)
	}

	// Directory exists, check if it's empty
	entries, err := os.ReadDir(localPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", localPath, err)
	}

	if len(entries) == 0 {
		// Directory is empty, clone the repository
		return gm.cloneRepository(repoURL, branch, localPath)
	}

	// Directory has content, check if it's a git repository
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("directory %s exists but is not a git repository: %w", localPath, err)
	}

	// Check if the remote matches
	remotes, err := repo.Remotes()
	if err != nil {
		return fmt.Errorf("failed to get remotes: %w", err)
	}

	if len(remotes) == 0 {
		return fmt.Errorf("repository has no remotes configured")
	}

	// Check origin remote
	origin, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("no origin remote found: %w", err)
	}

	// Compare URLs (normalize them)
	originURL := origin.Config().URLs[0]
	// Skip URL comparison if we're using token authentication
	// because the stored URL will have the token but the configured URL won't
	if gm.githubToken == "" && !gm.compareGitURLs(originURL, repoURL) {
		return fmt.Errorf("repository remote URL %s does not match expected %s", originURL, repoURL)
	}

	// Check if we're on the correct branch
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	currentBranch := head.Name().Short()
	if currentBranch != branch {
		// Try to checkout the correct branch
		worktree, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("failed to get worktree: %w", err)
		}

		branchRef := plumbing.NewBranchReferenceName(branch)
		err = worktree.Checkout(&git.CheckoutOptions{
			Branch: branchRef,
		})
		if err != nil {
			// Branch might not exist locally, try to create it from remote
			err = worktree.Checkout(&git.CheckoutOptions{
				Branch: branchRef,
				Create: true,
			})
			if err != nil {
				return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
			}
		}
	}

	logger.Info("Repository validation successful",
		zap.String("repo", repoURL),
		zap.String("path", localPath),
	)

	return nil
}

func (gm *GitManager) cloneRepository(repoURL, branch, localPath string) error {
	// Use the URL as specified by the user
	// If it's HTTPS and we have a token, add the token
	// If it's SSH, use SSH as-is (requires SSH keys)
	cloneURL := repoURL
	if gm.githubToken != "" && strings.HasPrefix(repoURL, "https://github.com/") {
		// Only add token to HTTPS URLs, not SSH URLs
		cloneURL = strings.Replace(repoURL, "https://", fmt.Sprintf("https://%s@", gm.githubToken), 1)
	}

	// Log with the sanitized URL (no token)
	logger.Info("Cloning repository",
		zap.String("repo", StripTokenFromURL(cloneURL)),
		zap.String("branch", branch),
		zap.String("path", localPath),
	)

	_, err := git.PlainClone(localPath, false, &git.CloneOptions{
		URL:           cloneURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Depth:         1,
	})

	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// If we used authentication, ensure the remote URL includes it
	if gm.githubToken != "" && strings.Contains(cloneURL, gm.githubToken) {
		// The remote URL is already set with authentication from the clone
		// No need to update it
	}

	logger.Info("Repository cloned successfully",
		zap.String("repo", repoURL),
		zap.String("path", localPath),
	)

	return nil
}

func (gm *GitManager) PullLatestChanges(localPath string) error {
	logger.Info("Pulling latest changes", zap.String("path", localPath))

	// Since go-git has issues with embedded tokens in URLs,
	// we'll use the system git command for pull operations
	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = localPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if the output indicates we're already up to date
		outputStr := string(output)
		if strings.Contains(outputStr, "Already up to date") {
			logger.Info("Repository already up to date", zap.String("path", localPath))
			return nil
		}
		return fmt.Errorf("failed to pull changes: %w, output: %s", err, outputStr)
	}

	logger.Info("Changes pulled successfully",
		zap.String("path", localPath),
		zap.String("output", string(output)))

	return nil
}

func (gm *GitManager) GetLatestCommitHash(localPath string) (string, error) {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	return head.Hash().String(), nil
}

func (gm *GitManager) GetRemoteLatestCommitHash(repoURL, branch string) (string, error) {
	// Create a temporary directory for fetching remote info
	tempDir, err := os.MkdirTemp("", "gocd-remote-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone just the specific branch with minimal depth
	_, err = git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Depth:         1,
	})

	if err != nil {
		return "", fmt.Errorf("failed to clone remote repository: %w", err)
	}

	return gm.GetLatestCommitHash(tempDir)
}

func (gm *GitManager) HasChanges(repoURL, branch, localPath string) (bool, error) {
	localHash, err := gm.GetLatestCommitHash(localPath)
	if err != nil {
		return false, fmt.Errorf("failed to get local commit hash: %w", err)
	}

	remoteHash, err := gm.GetRemoteLatestCommitHash(repoURL, branch)
	if err != nil {
		return false, fmt.Errorf("failed to get remote commit hash: %w", err)
	}

	return localHash != remoteHash, nil
}

func (gm *GitManager) compareGitURLs(url1, url2 string) bool {
	// Normalize URLs for comparison
	normalize := func(url string) string {
		// Convert SSH to HTTPS format for comparison
		if strings.HasPrefix(url, "git@github.com:") {
			url = "https://github.com/" + strings.TrimPrefix(url, "git@github.com:")
		}

		// Remove .git suffix
		url = strings.TrimSuffix(url, ".git")

		// Ensure it starts with https://
		if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
			if strings.HasPrefix(url, "github.com/") {
				url = "https://" + url
			}
		}

		return strings.ToLower(url)
	}

	return normalize(url1) == normalize(url2)
}

func (gm *GitManager) GetRepositoryInfo(localPath string) (*RepositoryInfo, error) {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	origin, err := repo.Remote("origin")
	if err != nil {
		return nil, fmt.Errorf("failed to get origin remote: %w", err)
	}

	return &RepositoryInfo{
		Branch:     head.Name().Short(),
		CommitHash: head.Hash().String(),
		RemoteURL:  origin.Config().URLs[0],
	}, nil
}

type RepositoryInfo struct {
	Branch     string
	CommitHash string
	RemoteURL  string
}