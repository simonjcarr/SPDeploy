package internal

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"spdeploy/internal/logger"
	"go.uber.org/zap"
)

// MonitorV2 is an improved monitor that uses repo-specific logging
type MonitorV2 struct {
	config *Config
	logger *Logger  // Keep simple logger for backward compatibility
}

func NewMonitorV2(config *Config) *MonitorV2 {
	// Initialize the advanced logger system
	if err := logger.InitLogger(); err != nil {
		fmt.Printf("Warning: Failed to initialize advanced logger: %v\n", err)
	}

	return &MonitorV2{
		config: config,
		logger: NewLogger(),
	}
}

func (m *MonitorV2) Run() {
	logger.Info("Starting spdeploy monitor",
		zap.Int("repositories", len(m.config.Repositories)),
		zap.Int("check_interval", m.config.CheckInterval))

	for {
		for _, repo := range m.config.Repositories {
			m.checkRepository(repo)
		}
		time.Sleep(time.Duration(m.config.CheckInterval) * time.Second)
	}
}

func (m *MonitorV2) checkRepository(repo Repository) {
	// Create a repository-specific logger
	repoLogger, err := logger.NewRepoLogger(repo.URL, repo.Path)
	if err != nil {
		logger.Error("Failed to create repository logger",
			zap.String("repo", repo.URL),
			zap.Error(err))
		// Continue with global logger if repo logger fails
	}
	defer func() {
		if repoLogger != nil {
			repoLogger.Close()
		}
	}()

	// Log to both repo-specific and global loggers
	if repoLogger != nil {
		repoLogger.Info("Checking repository",
			zap.String("branch", repo.Branch))
	}
	logger.Info("Checking repository",
		zap.String("repo", repo.URL),
		zap.String("branch", repo.Branch))

	// Check if repository exists and is valid
	gitDir := filepath.Join(repo.Path, ".git")
	if !fileExists(gitDir) {
		errMsg := fmt.Sprintf("Repository path does not exist or is not a git repository: %s", repo.Path)
		if repoLogger != nil {
			repoLogger.Error(errMsg)
		}
		logger.Error(errMsg, zap.String("repo", repo.URL))
		return
	}

	// Get current branch
	cmdBranch := exec.Command("git", "branch", "--show-current")
	cmdBranch.Dir = repo.Path
	currentBranchOutput, err := cmdBranch.Output()
	if err != nil {
		errMsg := "Failed to get current branch"
		if repoLogger != nil {
			repoLogger.Error(errMsg, zap.Error(err))
		}
		logger.Error(errMsg, zap.String("repo", repo.URL), zap.Error(err))
		return
	}
	currentBranch := strings.TrimSpace(string(currentBranchOutput))

	// Check if we're on the correct branch
	if currentBranch != repo.Branch {
		// Try to checkout the correct branch
		cmdCheckout := exec.Command("git", "checkout", repo.Branch)
		cmdCheckout.Dir = repo.Path
		if err := cmdCheckout.Run(); err != nil {
			errMsg := fmt.Sprintf("Failed to checkout branch %s", repo.Branch)
			if repoLogger != nil {
				repoLogger.Error(errMsg, zap.Error(err))
			}
			logger.Error(errMsg, zap.String("repo", repo.URL), zap.Error(err))
			return
		}
		if repoLogger != nil {
			repoLogger.Info("Switched to branch", zap.String("branch", repo.Branch))
		}
	}

	// Fetch latest changes
	cmdFetch := exec.Command("git", "fetch", "origin")
	cmdFetch.Dir = repo.Path
	if err := cmdFetch.Run(); err != nil {
		errMsg := "Failed to fetch from origin"
		if repoLogger != nil {
			repoLogger.Error(errMsg, zap.Error(err))
		}
		logger.Error(errMsg, zap.String("repo", repo.URL), zap.Error(err))
		return
	}

	// Check if there are new changes
	cmdStatus := exec.Command("git", "rev-list", "HEAD..origin/"+repo.Branch, "--count")
	cmdStatus.Dir = repo.Path
	statusOutput, err := cmdStatus.Output()
	if err != nil {
		errMsg := "Failed to check for updates"
		if repoLogger != nil {
			repoLogger.Error(errMsg, zap.Error(err))
		}
		logger.Error(errMsg, zap.String("repo", repo.URL), zap.Error(err))
		return
	}

	commitCount := strings.TrimSpace(string(statusOutput))
	if commitCount == "0" {
		// No new commits
		return
	}

	// New commits available, pull them
	if repoLogger != nil {
		repoLogger.Info("New commits detected", zap.String("count", commitCount))
	}
	logger.Info("New commits detected",
		zap.String("repo", repo.URL),
		zap.String("count", commitCount))

	cmdPull := exec.Command("git", "pull", "origin", repo.Branch)
	cmdPull.Dir = repo.Path
	pullOutput, err := cmdPull.CombinedOutput()
	if err != nil {
		errMsg := "Failed to pull changes"
		if repoLogger != nil {
			repoLogger.Error(errMsg, zap.Error(err), zap.String("output", string(pullOutput)))
		}
		logger.Error(errMsg,
			zap.String("repo", repo.URL),
			zap.Error(err),
			zap.String("output", string(pullOutput)))
		return
	}

	// Log successful deployment
	if repoLogger != nil {
		repoLogger.Info("Successfully pulled changes",
			zap.String("output", strings.TrimSpace(string(pullOutput))))
	}
	logger.Info("Successfully pulled changes",
		zap.String("repo", repo.URL),
		zap.String("output", strings.TrimSpace(string(pullOutput))))

	// Execute post-pull script if configured
	if repo.PostPullScript != "" {
		m.executePostPullScript(repo, repoLogger)
	}
}

func (m *MonitorV2) executePostPullScript(repo Repository, repoLogger *logger.RepoLogger) {
	scriptPath := filepath.Join(repo.Path, repo.PostPullScript)
	if !fileExists(scriptPath) {
		errMsg := fmt.Sprintf("Post-pull script not found: %s", scriptPath)
		if repoLogger != nil {
			repoLogger.Warn(errMsg)
		}
		logger.Warn(errMsg, zap.String("repo", repo.URL))
		return
	}

	cmd := exec.Command("/bin/sh", scriptPath)
	cmd.Dir = repo.Path
	output, err := cmd.CombinedOutput()

	if err != nil {
		errMsg := "Post-pull script failed"
		if repoLogger != nil {
			repoLogger.Error(errMsg,
				zap.Error(err),
				zap.String("script", repo.PostPullScript),
				zap.String("output", string(output)))
		}
		logger.Error(errMsg,
			zap.String("repo", repo.URL),
			zap.Error(err),
			zap.String("script", repo.PostPullScript),
			zap.String("output", string(output)))
		return
	}

	if repoLogger != nil {
		repoLogger.Info("Post-pull script executed successfully",
			zap.String("script", repo.PostPullScript),
			zap.String("output", strings.TrimSpace(string(output))))
	}
	logger.Info("Post-pull script executed successfully",
		zap.String("repo", repo.URL),
		zap.String("script", repo.PostPullScript),
		zap.String("output", strings.TrimSpace(string(output))))
}

// fileExists is defined in git.go