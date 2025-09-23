package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ValidateRepository checks if a repository can be accessed
func ValidateRepository(repo Repository) error {
	// Ensure the directory exists
	if err := ensureDirectoryExists(repo.Path); err != nil {
		return fmt.Errorf("failed to ensure directory exists: %w", err)
	}

	// Check if it's already a git repository
	gitDir := filepath.Join(repo.Path, ".git")
	if fileExists(gitDir) {
		// Verify it's the correct repository
		cmd := exec.Command("git", "remote", "get-url", "origin")
		cmd.Dir = repo.Path
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get remote URL: %w", err)
		}

		remoteURL := strings.TrimSpace(string(output))
		if remoteURL != repo.URL {
			return fmt.Errorf("directory already contains a different repository: %s", remoteURL)
		}
		return nil
	}

	// Try to clone the repository
	cmd := exec.Command("git", "clone", "-b", repo.Branch, repo.URL, repo.Path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Helper functions

func ensureDirectoryExists(path string) error {
	// Expand home directory if needed
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// Check if directory exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		// Create the directory
		return os.MkdirAll(path, 0755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", path)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}