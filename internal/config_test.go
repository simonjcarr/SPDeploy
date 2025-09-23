package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "spdeploy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Test loading default config when no config exists
	cfg := LoadConfig()
	if cfg == nil {
		t.Fatal("LoadConfig returned nil")
	}

	// Check default values
	if cfg.CheckInterval != 60 {
		t.Errorf("Expected default CheckInterval to be 60, got %d", cfg.CheckInterval)
	}

	if len(cfg.Repositories) != 0 {
		t.Errorf("Expected no repositories by default, got %d", len(cfg.Repositories))
	}
}

func TestSaveConfig(t *testing.T) {
	// Create a temporary directory for test config
	tmpDir, err := os.MkdirTemp("", "spdeploy-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create test config
	cfg := &Config{
		CheckInterval: 60,
		Repositories: []Repository{
			{
				URL:    "git@github.com:test/repo.git",
				Branch: "main",
				Path:   "/test/path",
			},
		},
	}

	// Save config
	err = SaveConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify config file was created
	configPath := filepath.Join(tmpDir, ".config", "spdeploy", "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load config and verify
	loadedCfg := LoadConfig()
	if loadedCfg.CheckInterval != 60 {
		t.Errorf("Expected CheckInterval to be 60, got %d", loadedCfg.CheckInterval)
	}

	if len(loadedCfg.Repositories) != 1 {
		t.Fatalf("Expected 1 repository, got %d", len(loadedCfg.Repositories))
	}

	repo := loadedCfg.Repositories[0]
	if repo.URL != "git@github.com:test/repo.git" {
		t.Errorf("Expected URL to be 'git@github.com:test/repo.git', got '%s'", repo.URL)
	}
	if repo.Branch != "main" {
		t.Errorf("Expected Branch to be 'main', got '%s'", repo.Branch)
	}
	if repo.Path != "/test/path" {
		t.Errorf("Expected Path to be '/test/path', got '%s'", repo.Path)
	}
}

func TestGetConfigPath(t *testing.T) {
	// Set HOME to a known value
	oldHome := os.Getenv("HOME")
	testHome := "/test/home"
	os.Setenv("HOME", testHome)
	defer os.Setenv("HOME", oldHome)

	expectedPath := filepath.Join(testHome, ".config", "spdeploy", "config.json")
	actualPath := getConfigPath()

	if actualPath != expectedPath {
		t.Errorf("Expected config path to be '%s', got '%s'", expectedPath, actualPath)
	}
}