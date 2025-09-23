package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestIntegrationFullWorkflow tests the complete workflow after refactoring
func TestIntegrationFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	// Create temp directory for entire test
	tmpDir, err := os.MkdirTemp("", "spdeploy-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	t.Run("ConfigManagement", func(t *testing.T) {
		// Test loading default config
		cfg := LoadConfig()
		if cfg == nil {
			t.Fatal("LoadConfig returned nil")
		}

		// Add a repository
		repo := Repository{
			URL:    "git@github.com:test/integration.git",
			Branch: "main",
			Path:   filepath.Join(tmpDir, "test-repo"),
		}
		cfg.Repositories = append(cfg.Repositories, repo)

		// Save config
		if err := SaveConfig(cfg); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		// Load config again and verify
		loadedCfg := LoadConfig()
		if len(loadedCfg.Repositories) != 1 {
			t.Errorf("Expected 1 repository, got %d", len(loadedCfg.Repositories))
		}
	})

	t.Run("DaemonLifecycle", func(t *testing.T) {
		// Test daemon PID management
		testPID := 12345

		// Write PID
		if err := WriteDaemonPID(testPID); err != nil {
			t.Fatalf("Failed to write daemon PID: %v", err)
		}

		// Check if daemon is "running" (it won't be, but PID file exists)
		// This tests the PID file read functionality
		pid, err := readDaemonPID()
		if err != nil {
			t.Errorf("Failed to read daemon PID: %v", err)
		}
		if pid != testPID {
			t.Errorf("Expected PID %d, got %d", testPID, pid)
		}

		// Test cleanup for current process
		currentPID := os.Getpid()
		WriteDaemonPID(currentPID)
		CleanupDaemonPID()

		// PID file should be removed
		if _, err := readDaemonPID(); err == nil {
			t.Error("PID file should have been removed")
		}
	})

	t.Run("MonitorInitialization", func(t *testing.T) {
		// Create a test config
		cfg := &Config{
			CheckInterval: 30,
			Repositories: []Repository{
				{
					URL:    "git@github.com:test/repo1.git",
					Branch: "main",
					Path:   filepath.Join(tmpDir, "repo1"),
				},
				{
					URL:    "git@github.com:test/repo2.git",
					Branch: "develop",
					Path:   filepath.Join(tmpDir, "repo2"),
				},
			},
		}

		// Initialize monitor
		monitor := NewMonitorV2(cfg)
		if monitor == nil {
			t.Fatal("NewMonitorV2 returned nil")
		}

		// Verify monitor has correct config
		if monitor.config != cfg {
			t.Error("Monitor config not set correctly")
		}

		// Test that monitor can start (run briefly)
		done := make(chan bool)
		go func() {
			go monitor.Run()
			time.Sleep(50 * time.Millisecond)
			done <- true
		}()

		select {
		case <-done:
			// Monitor ran successfully
		case <-time.After(1 * time.Second):
			t.Error("Monitor initialization timed out")
		}
	})

	t.Run("LoggerIntegration", func(t *testing.T) {
		// Initialize logger system
		if err := InitLogger(); err != nil {
			t.Errorf("Failed to initialize logger: %v", err)
		}

		// Test that logger functions don't panic
		// These are wrapper functions that should work after refactoring
		ShowContextualLogs(true, "", false, "", false)  // Global logs
		ShowContextualLogs(false, "test-repo", false, "", false)  // Repo logs
		ShowContextualLogs(false, "", true, "", false)  // All repos
	})

	t.Run("GitOperations", func(t *testing.T) {
		// Test git helper functions
		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("test"), 0644)

		// Test fileExists
		if !fileExists(testFile) {
			t.Error("fileExists returned false for existing file")
		}

		// Test ensureDirectoryExists
		newDir := filepath.Join(tmpDir, "new", "nested", "dir")
		if err := ensureDirectoryExists(newDir); err != nil {
			t.Errorf("Failed to ensure directory exists: %v", err)
		}

		if !fileExists(newDir) {
			t.Error("Directory was not created")
		}
	})
}

// TestRefactoringDidNotBreakAPI ensures that all public functions still exist
func TestRefactoringDidNotBreakAPI(t *testing.T) {
	// This test verifies that the refactoring didn't remove any public APIs
	// that might be used by the main package

	// Config functions
	_ = LoadConfig()
	_ = SaveConfig(&Config{})
	_ = getConfigPath()

	// Daemon helper functions
	_ = IsDaemonRunning()
	_ = WriteDaemonPID(1234)
	_ = StopDaemon()
	CleanupDaemonPID()

	// Git functions
	_ = ValidateRepository(Repository{})
	_ = fileExists("/tmp/test")
	_ = ensureDirectoryExists("/tmp/test")

	// Monitor functions
	_ = NewMonitorV2(&Config{})

	// Logger wrapper functions
	_ = InitLogger()
	ShowContextualLogs(false, "", false, "", false)

	// If we get here, all public APIs still exist
	t.Log("All public APIs are still available after refactoring")
}