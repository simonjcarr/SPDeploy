package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestNewMonitorV2(t *testing.T) {
	config := &Config{
		CheckInterval: 30,
		Repositories:  []Repository{},
	}

	monitor := NewMonitorV2(config)
	if monitor == nil {
		t.Fatal("NewMonitorV2 returned nil")
	}

	if monitor.config != config {
		t.Error("Monitor config not set correctly")
	}
}

func TestMonitorV2CheckRepository(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "spdeploy-monitor-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test git repository
	testRepoDir := filepath.Join(tmpDir, "test-repo")
	if err := os.MkdirAll(testRepoDir, 0755); err != nil {
		t.Fatalf("Failed to create test repo dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = testRepoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = testRepoDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = testRepoDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(testRepoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
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

	// Create main branch
	cmd = exec.Command("git", "branch", "-M", "main")
	cmd.Dir = testRepoDir
	cmd.Run()

	// Create test repository config
	repo := Repository{
		URL:    "git@github.com:test/repo.git",
		Branch: "main",
		Path:   testRepoDir,
	}

	config := &Config{
		CheckInterval: 30,
		Repositories:  []Repository{repo},
	}

	monitor := NewMonitorV2(config)

	// Test checkRepository - should handle the case where there's no remote
	// This will log errors but shouldn't panic
	monitor.checkRepository(repo)
}

func TestMonitorV2ExecutePostPullScript(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "spdeploy-script-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple script
	scriptPath := filepath.Join(tmpDir, "post-pull.sh")
	scriptContent := "#!/bin/sh\necho 'Script executed'\nexit 0"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	repo := Repository{
		URL:            "git@github.com:test/repo.git",
		Branch:         "main",
		Path:           tmpDir,
		PostPullScript: "post-pull.sh",
	}

	config := &Config{
		CheckInterval: 30,
		Repositories:  []Repository{repo},
	}

	monitor := NewMonitorV2(config)

	// Execute script
	monitor.executePostPullScript(repo, nil)

	// Test with non-existent script
	repo.PostPullScript = "non-existent.sh"
	monitor.executePostPullScript(repo, nil) // Should log warning but not panic

	// Test with failing script
	failScriptPath := filepath.Join(tmpDir, "fail.sh")
	failScriptContent := "#!/bin/sh\nexit 1"
	if err := os.WriteFile(failScriptPath, []byte(failScriptContent), 0755); err != nil {
		t.Fatalf("Failed to create fail script: %v", err)
	}

	repo.PostPullScript = "fail.sh"
	monitor.executePostPullScript(repo, nil) // Should log error but not panic
}

func TestMonitorV2RunStop(t *testing.T) {
	config := &Config{
		CheckInterval: 1, // Short interval for testing
		Repositories:  []Repository{},
	}

	monitor := NewMonitorV2(config)

	// Run monitor in goroutine
	done := make(chan bool)
	go func() {
		// Run for a short time then signal done
		go monitor.Run()
		time.Sleep(100 * time.Millisecond)
		done <- true
	}()

	select {
	case <-done:
		// Test passed - monitor ran without panicking
	case <-time.After(2 * time.Second):
		t.Fatal("Monitor run timed out")
	}
}