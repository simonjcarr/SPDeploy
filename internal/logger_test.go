package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitLogger(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "spdeploy-logger-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Initialize logger
	err = InitLogger()
	if err != nil {
		t.Errorf("Failed to initialize logger: %v", err)
	}

	// Check that log directory was created
	logDir := filepath.Join(tmpDir, ".local", "share", "spdeploy", "logs")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		// Also check the alternative location
		logDir = filepath.Join(tmpDir, ".spdeploy", "logs")
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			t.Error("Log directory was not created at either expected location")
		}
	}
}

func TestShowContextualLogs(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "spdeploy-logger-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Initialize logger first
	err = InitLogger()
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Test showing logs (non-interactive, should return quickly)
	// These calls should not panic
	t.Run("ShowGlobalLogs", func(t *testing.T) {
		// This will attempt to show logs but won't find any
		// Should handle gracefully
		ShowContextualLogs(true, "", false, "", false)
	})

	t.Run("ShowRepoLogs", func(t *testing.T) {
		// Test with specific repo URL
		ShowContextualLogs(false, "git@github.com:test/repo.git", false, "", false)
	})

	t.Run("ShowAllRepos", func(t *testing.T) {
		// Test listing all repos
		ShowContextualLogs(false, "", true, "", false)
	})
}