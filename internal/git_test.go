package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidateRepository(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "spdeploy-git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("ValidateNewRepository", func(t *testing.T) {
		// Create a test git repo
		testRepoDir := filepath.Join(tmpDir, "test-repo")
		if err := os.MkdirAll(testRepoDir, 0755); err != nil {
			t.Fatalf("Failed to create test repo dir: %v", err)
		}

		// Initialize a git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = testRepoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to init git repo: %v", err)
		}

		// Configure git user for the test repo
		cmd = exec.Command("git", "config", "user.email", "test@example.com")
		cmd.Dir = testRepoDir
		cmd.Run()

		cmd = exec.Command("git", "config", "user.name", "Test User")
		cmd.Dir = testRepoDir
		cmd.Run()

		// Create a test file and commit
		testFile := filepath.Join(testRepoDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = testRepoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to add files: %v", err)
		}

		cmd = exec.Command("git", "commit", "-m", "Initial commit")
		cmd.Dir = testRepoDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to commit: %v", err)
		}

		// Test ValidateRepository with existing git directory
		repo := Repository{
			URL:    "git@github.com:test/repo.git",
			Branch: "main",
			Path:   testRepoDir,
		}

		// Note: This will fail because the remote URL won't match
		// but it tests that the function handles existing repos
		err := ValidateRepository(repo)
		if err == nil {
			t.Error("Expected error for mismatched repository URL")
		}
	})

	t.Run("CreateDirectoryIfNotExists", func(t *testing.T) {
		// Test with non-existent directory
		newDir := filepath.Join(tmpDir, "new-repo")
		repo := Repository{
			URL:    "git@github.com:test/repo.git",
			Branch: "main",
			Path:   newDir,
		}

		// This will fail on clone (no real repo) but should create directory
		ValidateRepository(repo)

		// Check that directory was created (even though clone failed)
		if _, err := os.Stat(newDir); os.IsNotExist(err) {
			t.Error("Directory should have been created")
		}
	})
}

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Test with existing file
	if !fileExists(tmpFile.Name()) {
		t.Error("fileExists returned false for existing file")
	}

	// Test with non-existing file
	if fileExists("/non/existent/file.txt") {
		t.Error("fileExists returned true for non-existent file")
	}
}

func TestEnsureDirectoryExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "spdeploy-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("CreateNewDirectory", func(t *testing.T) {
		newDir := filepath.Join(tmpDir, "new", "nested", "dir")
		err := ensureDirectoryExists(newDir)
		if err != nil {
			t.Errorf("Failed to create directory: %v", err)
		}

		if _, err := os.Stat(newDir); os.IsNotExist(err) {
			t.Error("Directory was not created")
		}
	})

	t.Run("ExistingDirectory", func(t *testing.T) {
		existingDir := filepath.Join(tmpDir, "existing")
		os.MkdirAll(existingDir, 0755)

		err := ensureDirectoryExists(existingDir)
		if err != nil {
			t.Errorf("Error with existing directory: %v", err)
		}
	})

	t.Run("FileInsteadOfDirectory", func(t *testing.T) {
		// Create a file
		filePath := filepath.Join(tmpDir, "file.txt")
		os.WriteFile(filePath, []byte("test"), 0644)

		err := ensureDirectoryExists(filePath)
		if err == nil {
			t.Error("Expected error when path is a file, not a directory")
		}
	})

	t.Run("HomeDirectoryExpansion", func(t *testing.T) {
		// Set HOME to temp directory
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", oldHome)

		err := ensureDirectoryExists("~/test-dir")
		if err != nil {
			t.Errorf("Failed to create directory with ~ expansion: %v", err)
		}

		expectedDir := filepath.Join(tmpDir, "test-dir")
		if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
			t.Error("Directory with ~ expansion was not created")
		}
	})
}