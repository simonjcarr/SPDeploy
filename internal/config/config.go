package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Repository struct {
	ID       string    `yaml:"id"`
	URL      string    `yaml:"url"`
	Branch   string    `yaml:"branch"`
	Path     string    `yaml:"path"`
	Trigger  string    `yaml:"trigger"`
	LastSync time.Time `yaml:"last_sync"`
	Active   bool      `yaml:"active"`
	Token    string    `yaml:"token,omitempty"` // Per-repository token (encrypted in future)
}

type Config struct {
	Repositories []Repository `yaml:"repositories"`
	DaemonPID    int          `yaml:"daemon_pid,omitempty"`
	LogLevel     string       `yaml:"log_level"`
	PollInterval int          `yaml:"poll_interval"`
	GitHubToken  string       `yaml:"github_token,omitempty"`
}

type ConfigManager struct {
	configPath string
}

func NewConfig() *ConfigManager {
	configPath := getConfigPath()
	configDir := filepath.Dir(configPath)

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		panic(fmt.Sprintf("cannot create config directory: %v", err))
	}

	return &ConfigManager{
		configPath: configPath,
	}
}

// getConfigPath returns the OS-appropriate system-level configuration file path
func getConfigPath() string {
	switch runtime.GOOS {
	case "windows":
		// Windows: C:\ProgramData\SPDeploy\config.yaml
		programData := os.Getenv("PROGRAMDATA")
		if programData == "" {
			programData = "C:\\ProgramData"
		}
		return filepath.Join(programData, "SPDeploy", "config.yaml")

	case "darwin":
		// macOS: /etc/spdeploy/config.yaml (or ~/.config/spdeploy/config.yaml for non-root)
		if os.Geteuid() == 0 {
			return "/etc/spdeploy/config.yaml"
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = os.Getenv("HOME")
		}
		return filepath.Join(homeDir, ".config", "spdeploy", "config.yaml")

	default:
		// Linux/Unix: /etc/spdeploy/config.yaml
		return "/etc/spdeploy/config.yaml"
	}
}

func (cm *ConfigManager) Load() (*Config, error) {
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		return &Config{
			Repositories: []Repository{},
			LogLevel:     "info",
			PollInterval: 60,
		}, nil
	}

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func (cm *ConfigManager) Save(config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (cm *ConfigManager) AddRepository(repoURL, branch, path, trigger string) error {
	return cm.AddRepositoryWithToken(repoURL, branch, path, trigger, "")
}

func (cm *ConfigManager) AddRepositoryWithToken(repoURL, branch, path, trigger, token string) error {
	config, err := cm.Load()
	if err != nil {
		return err
	}

	// Clean up the repository URL
	repoURL = strings.TrimSpace(repoURL)
	if !strings.HasPrefix(repoURL, "https://") && !strings.HasPrefix(repoURL, "git@") {
		if !strings.HasPrefix(repoURL, "github.com/") {
			repoURL = "github.com/" + repoURL
		}
		repoURL = "https://" + repoURL
	}

	// Default branch if not specified
	if branch == "" {
		branch = "main"
	}

	// Default path to current directory if not specified
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		path = cwd
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	path = absPath

	// Validate trigger
	if trigger != "push" && trigger != "pr" && trigger != "both" {
		return fmt.Errorf("invalid trigger type: %s (must be push, pr, or both)", trigger)
	}

	// Generate unique ID
	id := fmt.Sprintf("%x", time.Now().UnixNano())

	// Check if repository already exists
	for _, repo := range config.Repositories {
		if repo.URL == repoURL && repo.Branch == branch && repo.Path == path {
			return fmt.Errorf("repository already configured")
		}
	}

	// Add new repository
	newRepo := Repository{
		ID:      id,
		URL:     repoURL,
		Branch:  branch,
		Path:    path,
		Trigger: trigger,
		Active:  true,
		Token:   token,
	}

	config.Repositories = append(config.Repositories, newRepo)

	return cm.Save(config)
}

func (cm *ConfigManager) RemoveRepository(repoURL, branch string) error {
	config, err := cm.Load()
	if err != nil {
		return err
	}

	var updatedRepos []Repository
	found := false

	for _, repo := range config.Repositories {
		if repo.URL == repoURL && repo.Branch == branch {
			found = true
			continue
		}
		updatedRepos = append(updatedRepos, repo)
	}

	if !found {
		return fmt.Errorf("repository not found")
	}

	config.Repositories = updatedRepos
	return cm.Save(config)
}

func (cm *ConfigManager) RemoveRepositoryByPath(repoURL, branch, path string) error {
	config, err := cm.Load()
	if err != nil {
		return err
	}

	// Clean up the repository URL to match how it's stored
	repoURL = strings.TrimSpace(repoURL)
	if !strings.HasPrefix(repoURL, "https://") && !strings.HasPrefix(repoURL, "git@") {
		if !strings.HasPrefix(repoURL, "github.com/") {
			repoURL = "github.com/" + repoURL
		}
		repoURL = "https://" + repoURL
	}

	// Default branch if not specified
	if branch == "" {
		branch = "main"
	}

	// Convert path to absolute if specified
	if path != "" {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		path = absPath
	}

	var updatedRepos []Repository
	found := false

	for _, repo := range config.Repositories {
		// If path is specified, match all three: URL, branch, and path
		if path != "" {
			if repo.URL == repoURL && repo.Branch == branch && repo.Path == path {
				found = true
				continue
			}
		} else {
			// If no path specified, just match URL and branch (original behavior)
			if repo.URL == repoURL && repo.Branch == branch {
				found = true
				continue
			}
		}
		updatedRepos = append(updatedRepos, repo)
	}

	if !found {
		if path != "" {
			return fmt.Errorf("repository not found at specified path: %s", path)
		}
		return fmt.Errorf("repository not found")
	}

	config.Repositories = updatedRepos
	return cm.Save(config)
}

func (cm *ConfigManager) RemoveAllRepository(repoURL, branch string) (int, error) {
	config, err := cm.Load()
	if err != nil {
		return 0, err
	}

	// Clean up the repository URL to match how it's stored
	repoURL = strings.TrimSpace(repoURL)
	if !strings.HasPrefix(repoURL, "https://") && !strings.HasPrefix(repoURL, "git@") {
		if !strings.HasPrefix(repoURL, "github.com/") {
			repoURL = "github.com/" + repoURL
		}
		repoURL = "https://" + repoURL
	}

	// Default branch if not specified
	if branch == "" {
		branch = "main"
	}

	var updatedRepos []Repository
	removedCount := 0

	for _, repo := range config.Repositories {
		if repo.URL == repoURL && repo.Branch == branch {
			removedCount++
			continue
		}
		updatedRepos = append(updatedRepos, repo)
	}

	if removedCount == 0 {
		return 0, fmt.Errorf("no repositories found matching: %s (branch: %s)", repoURL, branch)
	}

	config.Repositories = updatedRepos
	if err := cm.Save(config); err != nil {
		return 0, err
	}

	return removedCount, nil
}

func (cm *ConfigManager) ListRepositories() ([]Repository, error) {
	config, err := cm.Load()
	if err != nil {
		return nil, err
	}

	return config.Repositories, nil
}

func (cm *ConfigManager) UpdateRepositorySync(repoID string, lastSync time.Time) error {
	config, err := cm.Load()
	if err != nil {
		return err
	}

	for i, repo := range config.Repositories {
		if repo.ID == repoID {
			config.Repositories[i].LastSync = lastSync
			return cm.Save(config)
		}
	}

	return fmt.Errorf("repository not found")
}

func (cm *ConfigManager) SetDaemonPID(pid int) error {
	config, err := cm.Load()
	if err != nil {
		return err
	}

	config.DaemonPID = pid
	return cm.Save(config)
}

func (cm *ConfigManager) GetDaemonPID() (int, error) {
	config, err := cm.Load()
	if err != nil {
		return 0, err
	}

	return config.DaemonPID, nil
}

func (cm *ConfigManager) GetConfigPath() string {
	return cm.configPath
}

func (cm *ConfigManager) GetLogDirectory() string {
	switch runtime.GOOS {
	case "windows":
		// Windows: C:\ProgramData\SPDeploy\logs
		programData := os.Getenv("PROGRAMDATA")
		if programData == "" {
			programData = "C:\\ProgramData"
		}
		return filepath.Join(programData, "SPDeploy", "logs")

	case "darwin":
		// macOS: /var/log/spdeploy (or ~/.local/share/spdeploy/logs for non-root)
		if os.Geteuid() == 0 {
			return "/var/log/spdeploy"
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = os.Getenv("HOME")
		}
		return filepath.Join(homeDir, ".local", "share", "spdeploy", "logs")

	default:
		// Linux/Unix: /var/log/spdeploy
		return "/var/log/spdeploy"
	}
}