package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	CheckInterval int          `json:"check_interval"`
	Repositories  []Repository `json:"repositories"`
}

type Repository struct {
	URL            string `json:"url"`
	Branch         string `json:"branch"`
	Path           string `json:"path"`
	PostPullScript string `json:"post_pull_script,omitempty"`
}

func getConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".config", "spdeploy")
	os.MkdirAll(configDir, 0755)
	return filepath.Join(configDir, "config.json")
}

func LoadConfig() *Config {
	configPath := getConfigPath()

	// Default config
	config := &Config{
		CheckInterval: 60,
		Repositories:  []Repository{},
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config doesn't exist yet, return default
		return config
	}

	if err := json.Unmarshal(data, config); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to parse config: %v\n", err)
		return config
	}

	// Ensure we have a valid check interval
	if config.CheckInterval < 10 {
		config.CheckInterval = 60
	}

	return config
}

func SaveConfig(config *Config) error {
	configPath := getConfigPath()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}