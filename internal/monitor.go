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

type Monitor struct {
	config *Config
	logger *Logger  // Keep simple logger for backward compatibility
}

func NewMonitor(config *Config) *Monitor {
	// Initialize the advanced logger system
	if err := logger.InitLogger(); err != nil {
		fmt.Printf("Warning: Failed to initialize advanced logger: %v\n", err)
	}

	return &Monitor{
		config: config,
		logger: NewLogger(),
	}
}

func (m *Monitor) Run() {
	for {
		for _, repo := range m.config.Repositories {
			m.checkRepository(repo)
		}
		time.Sleep(time.Duration(m.config.CheckInterval) * time.Second)
	}
}

func (m *Monitor) checkRepository(repo Repository) {
	m.logger.Info("Checking repository: %s", repo.URL)

	// Also log to the advanced logger
	logger.Info("Checking repository", zap.String("repo", repo.URL), zap.String("branch", repo.Branch))

	// Check if there are changes
	hasChanges, err := HasChanges(repo)
	if err != nil {
		m.logger.Error("Failed to check %s: %v", repo.URL, err)
		logger.Error("Failed to check repository", zap.String("repo", repo.URL), zap.Error(err))
		return
	}

	if !hasChanges {
		m.logger.Debug("No changes in %s", repo.URL)
		logger.Debug("No changes in repository", zap.String("repo", repo.URL))
		return
	}

	m.logger.Info("Changes detected in %s, pulling...", repo.URL)
	logger.Info("Changes detected, pulling", zap.String("repo", repo.URL), zap.String("branch", repo.Branch))

	// Pull changes
	if err := PullChanges(repo); err != nil {
		m.logger.Error("Failed to pull %s: %v", repo.URL, err)
		logger.Error("Failed to pull changes", zap.String("repo", repo.URL), zap.Error(err))
		return
	}

	m.logger.Info("Successfully pulled changes for %s", repo.URL)
	logger.Info("Successfully pulled changes", zap.String("repo", repo.URL))

	// Run post-pull script if configured
	if repo.PostPullScript != "" {
		m.runPostPullScript(repo)
	}
}

func (m *Monitor) runPostPullScript(repo Repository) {
	scriptPath := filepath.Join(repo.Path, repo.PostPullScript)
	m.logger.Info("Running post-pull script: %s", scriptPath)
	logger.Info("Running post-pull script", zap.String("repo", repo.URL), zap.String("script", scriptPath))

	cmd := exec.Command("/bin/sh", "-c", scriptPath)
	cmd.Dir = repo.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		m.logger.Error("Post-pull script failed for %s: %v\nOutput: %s",
			repo.URL, err, string(output))
		logger.Error("Post-pull script failed",
			zap.String("repo", repo.URL),
			zap.Error(err),
			zap.String("output", string(output)))
		return
	}

	m.logger.Info("Post-pull script completed successfully for %s", repo.URL)
	logger.Info("Post-pull script completed", zap.String("repo", repo.URL))
	if len(output) > 0 {
		m.logger.Debug("Script output: %s", string(output))
		logger.Debug("Script output", zap.String("repo", repo.URL), zap.String("output", string(output)))
	}
}

func ValidateRepository(repo Repository) error {
	// Check if the path exists or can be created
	if err := ensureDirectoryExists(repo.Path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if it's a git repository
	gitDir := filepath.Join(repo.Path, ".git")
	if !fileExists(gitDir) {
		// Try to clone the repository
		fmt.Printf("Cloning repository %s to %s\n", repo.URL, repo.Path)
		cmd := exec.Command("git", "clone", "-b", repo.Branch, repo.URL, repo.Path)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to clone: %w\nOutput: %s", err, string(output))
		}
	} else {
		// Verify it's the correct repository
		cmd := exec.Command("git", "remote", "get-url", "origin")
		cmd.Dir = repo.Path
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get remote URL: %w", err)
		}

		remoteURL := strings.TrimSpace(string(output))

		if remoteURL != repo.URL {
			return fmt.Errorf("path contains different repository: %s", remoteURL)
		}
	}

	return nil
}