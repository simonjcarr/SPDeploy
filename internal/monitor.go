package internal

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Monitor struct {
	config *Config
	logger *Logger
}

func NewMonitor(config *Config) *Monitor {
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

	// Check if there are changes
	hasChanges, err := HasChanges(repo)
	if err != nil {
		m.logger.Error("Failed to check %s: %v", repo.URL, err)
		return
	}

	if !hasChanges {
		m.logger.Debug("No changes in %s", repo.URL)
		return
	}

	m.logger.Info("Changes detected in %s, pulling...", repo.URL)

	// Pull changes
	if err := PullChanges(repo); err != nil {
		m.logger.Error("Failed to pull %s: %v", repo.URL, err)
		return
	}

	m.logger.Info("Successfully pulled changes for %s", repo.URL)

	// Run post-pull script if configured
	if repo.PostPullScript != "" {
		m.runPostPullScript(repo)
	}
}

func (m *Monitor) runPostPullScript(repo Repository) {
	scriptPath := filepath.Join(repo.Path, repo.PostPullScript)
	m.logger.Info("Running post-pull script: %s", scriptPath)

	cmd := exec.Command("/bin/sh", "-c", scriptPath)
	cmd.Dir = repo.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		m.logger.Error("Post-pull script failed for %s: %v\nOutput: %s",
			repo.URL, err, string(output))
		return
	}

	m.logger.Info("Post-pull script completed successfully for %s", repo.URL)
	if len(output) > 0 {
		m.logger.Debug("Script output: %s", string(output))
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