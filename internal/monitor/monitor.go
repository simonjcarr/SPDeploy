package monitor

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"spdeploy/internal/config"
	"spdeploy/internal/git"
	"spdeploy/internal/github"
	"spdeploy/internal/logger"
)

type Monitor struct {
	config          *config.ConfigManager
	gitManager      *git.GitManager
	providerManager *git.GitManagerWithProvider
	githubClient    *github.GitHubClient
	scriptExec      *ScriptExecutor
	pollInterval    time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	repositories    map[string]*RepositoryWatcher
	mu              sync.RWMutex
}

type RepositoryWatcher struct {
	Config       config.Repository
	LastSync     time.Time
	IsProcessing bool
	LastError    error
	GitManager   *git.GitManager
	RepoLogger   *logger.RepoLogger
}

func NewMonitor(githubToken string) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())

	// Create provider-aware git manager
	providerManager := git.NewGitManagerWithProvider()

	return &Monitor{
		config:          config.NewConfig(),
		gitManager:      git.NewGitManagerWithToken(githubToken), // Keep for backward compatibility
		providerManager: providerManager,
		githubClient:    github.NewGitHubClient(githubToken),
		scriptExec:      NewScriptExecutor(5 * time.Minute),
		pollInterval:    60 * time.Second,
		ctx:             ctx,
		cancel:          cancel,
		repositories:    make(map[string]*RepositoryWatcher),
	}
}

func (m *Monitor) Start() error {
	logger.Info("Starting spdeploy monitor service")

	// Initialize logger
	if err := logger.InitLogger(); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Load repositories from config
	if err := m.loadRepositories(); err != nil {
		return fmt.Errorf("failed to load repositories: %w", err)
	}

	// Start monitoring loop
	m.wg.Add(1)
	go m.monitorLoop()

	// Start config file watcher
	m.wg.Add(1)
	go m.watchConfigFile()

	logger.Info("spdeploy monitor service started successfully")
	return nil
}

func (m *Monitor) Stop() {
	logger.Info("Stopping spdeploy monitor service")
	m.cancel()
	m.wg.Wait()
	logger.Info("spdeploy monitor service stopped")
}

func (m *Monitor) loadRepositories() error {
	repos, err := m.config.ListRepositories()
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Load provider instances from config
	configData, _ := m.config.Load()
	if configData != nil && len(configData.Providers) > 0 {
		var providerInstances []git.ProviderInstance
		for _, p := range configData.Providers {
			providerInstances = append(providerInstances, git.ProviderInstance{
				Name:    p.Name,
				Type:    p.Type,
				BaseURL: p.BaseURL,
				APIURL:  p.APIURL,
			})
		}
		m.providerManager.LoadProviderInstances(providerInstances)
	}

	for _, repo := range repos {
		if !repo.Active {
			continue
		}

		// Get authenticated URL for this repository
		authURL, err := m.providerManager.GetAuthenticatedURL(repo.URL, repo.ID)
		if err != nil {
			logger.Warn("Failed to get authenticated URL",
				zap.String("repo", repo.URL),
				zap.Error(err))
			authURL = repo.URL // Fallback to original URL
		}

		// Create a GitManager for this repository
		// If we have an authenticated URL with token, use it
		repoGitManager := git.NewGitManager()
		if authURL != repo.URL {
			// Extract token from authenticated URL if present
			// For now, just use the git manager as-is
			repoGitManager = git.NewGitManager()
		}

		// Create a repository-specific logger
		repoLogger, err := logger.NewRepoLogger(repo.URL, repo.Path)
		if err != nil {
			logger.Error("Failed to create repository logger",
				zap.String("repo", repo.URL),
				zap.Error(err),
			)
			// Continue with global logger if repo logger fails
		}

		watcher := &RepositoryWatcher{
			Config:     repo,
			LastSync:   repo.LastSync,
			GitManager: repoGitManager,
			RepoLogger: repoLogger,
		}

		// Validate or setup the repository using authenticated URL
		err = repoGitManager.ValidateOrSetupRepo(authURL, repo.Branch, repo.Path)
		if err != nil {
			logger.Error("Failed to validate repository",
				zap.String("repo", repo.URL),
				zap.Error(err),
			)
			watcher.LastError = err
		}

		m.repositories[repo.ID] = watcher
		logger.Info("Loaded repository for monitoring",
			zap.String("repo", repo.URL),
			zap.String("branch", repo.Branch),
			zap.String("path", repo.Path),
		)
	}

	return nil
}

// ReloadRepositories dynamically reloads the repository configuration
func (m *Monitor) ReloadRepositories() error {
	logger.Info("Reloading repository configuration")

	repos, err := m.config.ListRepositories()
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Load provider instances from config
	configData, _ := m.config.Load()
	if configData != nil && len(configData.Providers) > 0 {
		var providerInstances []git.ProviderInstance
		for _, p := range configData.Providers {
			providerInstances = append(providerInstances, git.ProviderInstance{
				Name:    p.Name,
				Type:    p.Type,
				BaseURL: p.BaseURL,
				APIURL:  p.APIURL,
			})
		}
		m.providerManager.LoadProviderInstances(providerInstances)
	}

	// Track which repositories are in the new config
	currentRepoIDs := make(map[string]bool)

	for _, repo := range repos {
		if !repo.Active {
			continue
		}

		currentRepoIDs[repo.ID] = true

		// Check if repository already exists
		if existing, exists := m.repositories[repo.ID]; exists {
			// Update configuration but preserve runtime state
			existing.Config = repo
			logger.Debug("Updated existing repository configuration",
				zap.String("repo", repo.URL),
				zap.String("id", repo.ID))
			continue
		}

		// New repository - add it
		logger.Info("Adding new repository to monitoring",
			zap.String("repo", repo.URL),
			zap.String("branch", repo.Branch),
			zap.String("path", repo.Path))

		// Get authenticated URL for this repository
		authURL, err := m.providerManager.GetAuthenticatedURL(repo.URL, repo.ID)
		if err != nil {
			logger.Warn("Failed to get authenticated URL",
				zap.String("repo", repo.URL),
				zap.Error(err))
			authURL = repo.URL // Fallback to original URL
		}

		// Create a GitManager for this repository
		repoGitManager := git.NewGitManager()

		// Create a repository-specific logger
		repoLogger, err := logger.NewRepoLogger(repo.URL, repo.Path)
		if err != nil {
			logger.Error("Failed to create repository logger",
				zap.String("repo", repo.URL),
				zap.Error(err))
		}

		watcher := &RepositoryWatcher{
			Config:     repo,
			LastSync:   repo.LastSync,
			GitManager: repoGitManager,
			RepoLogger: repoLogger,
		}

		// Validate or setup the repository using authenticated URL
		err = repoGitManager.ValidateOrSetupRepo(authURL, repo.Branch, repo.Path)
		if err != nil {
			logger.Error("Failed to validate repository",
				zap.String("repo", repo.URL),
				zap.Error(err))
			watcher.LastError = err
		}

		m.repositories[repo.ID] = watcher
	}

	// Remove repositories that are no longer in config
	for id, watcher := range m.repositories {
		if !currentRepoIDs[id] {
			logger.Info("Removing repository from monitoring",
				zap.String("repo", watcher.Config.URL),
				zap.String("id", id))
			delete(m.repositories, id)
		}
	}

	logger.Info("Repository configuration reload completed",
		zap.Int("total_repos", len(m.repositories)))

	return nil
}

func (m *Monitor) monitorLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// Create a ticker for config reload - check every 30 seconds
	reloadTicker := time.NewTicker(30 * time.Second)
	defer reloadTicker.Stop()

	logger.Info("Starting monitoring loop",
		zap.Duration("interval", m.pollInterval),
		zap.Int("repositories", len(m.repositories)),
	)

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAllRepositories()
		case <-reloadTicker.C:
			// Reload configuration periodically
			if err := m.ReloadRepositories(); err != nil {
				logger.Error("Failed to reload repositories", zap.Error(err))
			}
		}
	}
}

// watchConfigFile watches the configuration file for changes
func (m *Monitor) watchConfigFile() {
	defer m.wg.Done()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error("Failed to create file watcher", zap.Error(err))
		return
	}
	defer watcher.Close()

	configPath := m.config.GetConfigPath()

	// Watch the config file
	err = watcher.Add(configPath)
	if err != nil {
		logger.Error("Failed to watch config file",
			zap.String("path", configPath),
			zap.Error(err))
		return
	}

	logger.Info("Watching configuration file for changes",
		zap.String("path", configPath))

	// Also watch the config directory for file replacements
	configDir := filepath.Dir(configPath)
	if err := watcher.Add(configDir); err != nil {
		logger.Warn("Failed to watch config directory",
			zap.String("dir", configDir),
			zap.Error(err))
	}

	// Debounce timer to avoid multiple reloads for rapid file changes
	var debounceTimer *time.Timer
	debounceDelay := 2 * time.Second

	for {
		select {
		case <-m.ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our config file
			if filepath.Base(event.Name) == filepath.Base(configPath) {
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					logger.Debug("Config file change detected",
						zap.String("event", event.Op.String()),
						zap.String("file", event.Name))

					// Cancel existing timer
					if debounceTimer != nil {
						debounceTimer.Stop()
					}

					// Start new timer
					debounceTimer = time.AfterFunc(debounceDelay, func() {
						logger.Info("Config file changed, reloading repositories")
						if err := m.ReloadRepositories(); err != nil {
							logger.Error("Failed to reload repositories after config change",
								zap.Error(err))
						}
					})
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Error("File watcher error", zap.Error(err))
		}
	}
}

func (m *Monitor) checkAllRepositories() {
	m.mu.RLock()
	repos := make([]*RepositoryWatcher, 0, len(m.repositories))
	for _, repo := range m.repositories {
		repos = append(repos, repo)
	}
	m.mu.RUnlock()

	// Check repositories concurrently
	for _, repo := range repos {
		if repo.IsProcessing {
			continue // Skip if already processing
		}

		m.wg.Add(1)
		go func(r *RepositoryWatcher) {
			defer m.wg.Done()
			m.checkRepository(r)
		}(repo)
	}
}

// logWithFallback logs to both repo-specific and global loggers
func (m *Monitor) logInfo(watcher *RepositoryWatcher, msg string, fields ...zap.Field) {
	if watcher.RepoLogger != nil {
		watcher.RepoLogger.Info(msg, fields...)
	}
	logger.Info(msg, fields...)
}

func (m *Monitor) logError(watcher *RepositoryWatcher, msg string, fields ...zap.Field) {
	if watcher.RepoLogger != nil {
		watcher.RepoLogger.Error(msg, fields...)
	}
	logger.Error(msg, fields...)
}

func (m *Monitor) logDebug(watcher *RepositoryWatcher, msg string, fields ...zap.Field) {
	if watcher.RepoLogger != nil {
		watcher.RepoLogger.Debug(msg, fields...)
	}
	logger.Debug(msg, fields...)
}

func (m *Monitor) checkRepository(watcher *RepositoryWatcher) {
	repo := watcher.Config

	// Mark as processing
	m.mu.Lock()
	watcher.IsProcessing = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		watcher.IsProcessing = false
		m.mu.Unlock()
	}()

	m.logDebug(watcher, "Checking repository for changes",
		zap.String("repo", repo.URL),
		zap.String("branch", repo.Branch),
	)

	// Determine if this is a GitHub repository
	isGitHub := strings.Contains(repo.URL, "github.com")

	var change *github.RepositoryChange
	var err error

	if isGitHub {
		// Check for changes via GitHub API
		change, err = m.githubClient.CheckForChanges(repo.URL, repo.Branch, repo.Trigger, watcher.LastSync)
		if err != nil {
			m.logError(watcher, "Failed to check for repository changes",
				zap.String("repo", repo.URL),
				zap.Error(err),
			)
			watcher.LastError = err
			return
		}
	} else {
		// For non-GitHub repositories, use git fetch to check for changes
		m.logDebug(watcher, "Using git-based change detection for non-GitHub repository",
			zap.String("repo", repo.URL),
		)

		// Perform a git fetch to get the latest remote information
		hasChanges, err := m.checkRepositoryViaGit(watcher, repo)
		if err != nil {
			m.logError(watcher, "Failed to check for repository changes via git",
				zap.String("repo", repo.URL),
				zap.Error(err),
			)
			watcher.LastError = err
			return
		}

		if hasChanges {
			// Create a synthetic change object for consistency
			change = &github.RepositoryChange{
				Type:      "push",
				Commit:    "latest",
				Branch:    repo.Branch,
				Timestamp: time.Now(),
			}
		}
	}

	if change == nil {
		// No changes detected
		return
	}

	m.logInfo(watcher, "Changes detected in repository",
		zap.String("repo", repo.URL),
		zap.String("branch", repo.Branch),
		zap.String("type", change.Type),
		zap.String("commit", change.Commit),
		zap.Time("timestamp", change.Timestamp),
	)

	// Pull the latest changes
	// For non-GitHub repos, we need to use authenticated pull
	if !isGitHub {
		err = m.pullWithAuthentication(watcher, repo)
	} else {
		err = watcher.GitManager.PullLatestChanges(repo.Path)
	}

	if err != nil {
		m.logError(watcher, "Failed to pull latest changes",
			zap.String("repo", repo.URL),
			zap.String("path", repo.Path),
			zap.Error(err),
		)
		watcher.LastError = err
		return
	}

	// Log deployment success to both repo and global logs
	if watcher.RepoLogger != nil {
		watcher.RepoLogger.Info("Deployment pull successful",
			zap.String("branch", repo.Branch))
	}
	logger.LogDeployment(repo.URL, repo.Branch, "pull_success", nil)

	// Look for and execute deployment script
	scriptPath, err := m.scriptExec.FindScript(repo.Path)
	if err != nil {
		m.logError(watcher, "Error finding deployment script",
			zap.String("repo_path", repo.Path),
			zap.Error(err))
	} else if scriptPath != "" {
		// Execute the script
		result := m.scriptExec.ExecuteScript(scriptPath, repo.Path)

		if !result.Success {
			m.logError(watcher, "Deployment script failed",
				zap.String("repo", repo.URL),
				zap.String("script", result.ScriptPath),
				zap.String("error", result.Error),
				zap.String("output", result.Output),
				zap.Int("exit_code", result.ExitCode),
				zap.Duration("duration", result.Duration))
		} else {
			m.logInfo(watcher, "Deployment script executed successfully",
				zap.String("repo", repo.URL),
				zap.String("script", result.ScriptPath),
				zap.Duration("duration", result.Duration))
		}
	}

	// Update last sync time
	watcher.LastSync = change.Timestamp
	watcher.LastError = nil

	// Persist the update
	err = m.config.UpdateRepositorySync(repo.ID, change.Timestamp)
	if err != nil {
		m.logError(watcher, "Failed to update repository sync time",
			zap.String("repo", repo.URL),
			zap.Error(err))
	}

	// Log deployment completion to both repo and global logs
	if watcher.RepoLogger != nil {
		watcher.RepoLogger.Info("Deployment complete",
			zap.String("branch", repo.Branch),
			zap.String("commit", change.Commit[:8]))
	}
	logger.LogRepoEvent(repo.URL, repo.Branch, "deployment_complete",
		fmt.Sprintf("Successfully deployed %s", change.Commit[:8]))
}

func (m *Monitor) AddRepository(repoURL, branch, path, trigger string) error {
	// Add to config first
	err := m.config.AddRepository(repoURL, branch, path, trigger)
	if err != nil {
		return err
	}

	// Reload repositories to include the new one
	return m.loadRepositories()
}

func (m *Monitor) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]interface{}{
		"running":      true,
		"repositories": len(m.repositories),
		"poll_interval": m.pollInterval.String(),
	}

	repos := make([]map[string]interface{}, 0, len(m.repositories))
	for _, watcher := range m.repositories {
		repoStatus := map[string]interface{}{
			"url":           watcher.Config.URL,
			"branch":        watcher.Config.Branch,
			"path":          watcher.Config.Path,
			"trigger":       watcher.Config.Trigger,
			"last_sync":     watcher.LastSync,
			"is_processing": watcher.IsProcessing,
		}

		if watcher.LastError != nil {
			repoStatus["last_error"] = watcher.LastError.Error()
		}

		repos = append(repos, repoStatus)
	}

	status["repository_details"] = repos
	return status
}

// pullWithAuthentication performs a git pull with authentication for non-GitHub repos
func (m *Monitor) pullWithAuthentication(watcher *RepositoryWatcher, repo config.Repository) error {
	// Get authenticated URL for pulling
	authURL, err := m.providerManager.GetAuthenticatedURL(repo.URL, repo.ID)
	if err != nil {
		m.logDebug(watcher, "Failed to get authenticated URL, using original",
			zap.String("repo", repo.URL),
			zap.Error(err))
		authURL = repo.URL
	}

	// Perform a pull using the authenticated URL
	pullCmd := exec.Command("git", "pull", authURL, repo.Branch)
	pullCmd.Dir = repo.Path
	output, err := pullCmd.CombinedOutput()

	if err != nil {
		// Check if the output indicates we're already up to date
		outputStr := string(output)
		if strings.Contains(outputStr, "Already up to date") || strings.Contains(outputStr, "Already up-to-date") {
			m.logInfo(watcher, "Repository already up to date",
				zap.String("path", repo.Path))
			return nil
		}
		return fmt.Errorf("failed to pull changes: %w, output: %s", err, outputStr)
	}

	m.logInfo(watcher, "Changes pulled successfully",
		zap.String("path", repo.Path),
		zap.String("output", string(output)))

	return nil
}

// checkRepositoryViaGit uses git commands to check if there are remote changes
func (m *Monitor) checkRepositoryViaGit(watcher *RepositoryWatcher, repo config.Repository) (bool, error) {
	// Get authenticated URL for fetching
	authURL, err := m.providerManager.GetAuthenticatedURL(repo.URL, repo.ID)
	if err != nil {
		m.logDebug(watcher, "Failed to get authenticated URL, using original",
			zap.String("repo", repo.URL),
			zap.Error(err))
		authURL = repo.URL
	}

	// First, get the current local commit hash
	localCmd := exec.Command("git", "rev-parse", fmt.Sprintf("refs/heads/%s", repo.Branch))
	localCmd.Dir = repo.Path
	localOutput, err := localCmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to get local commit: %w", err)
	}
	localCommit := strings.TrimSpace(string(localOutput))

	// Perform a fetch to update remote refs
	fetchCmd := exec.Command("git", "fetch", authURL, fmt.Sprintf("%s:refs/remotes/origin/%s", repo.Branch, repo.Branch))
	fetchCmd.Dir = repo.Path
	fetchOutput, err := fetchCmd.CombinedOutput()
	if err != nil {
		// Log the error but try to continue with the existing remote ref
		m.logDebug(watcher, "Git fetch failed, checking existing remote ref",
			zap.String("repo", repo.URL),
			zap.String("output", string(fetchOutput)),
			zap.Error(err))
	}

	// Get the remote commit hash
	remoteCmd := exec.Command("git", "rev-parse", fmt.Sprintf("refs/remotes/origin/%s", repo.Branch))
	remoteCmd.Dir = repo.Path
	remoteOutput, err := remoteCmd.Output()
	if err != nil {
		// If we can't get remote ref, try fetching without auth
		if authURL != repo.URL {
			fetchCmd = exec.Command("git", "fetch", "origin", repo.Branch)
			fetchCmd.Dir = repo.Path
			fetchCmd.CombinedOutput()

			remoteOutput, err = remoteCmd.Output()
			if err != nil {
				return false, fmt.Errorf("failed to get remote commit: %w", err)
			}
		} else {
			return false, fmt.Errorf("failed to get remote commit: %w", err)
		}
	}
	remoteCommit := strings.TrimSpace(string(remoteOutput))

	// Compare commits
	hasChanges := localCommit != remoteCommit

	if hasChanges {
		m.logInfo(watcher, "Git-based check found changes",
			zap.String("repo", repo.URL),
			zap.String("local_commit", localCommit[:8]),
			zap.String("remote_commit", remoteCommit[:8]))
	}

	return hasChanges, nil
}