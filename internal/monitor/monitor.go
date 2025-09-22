package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"spdeploy/internal/config"
	"spdeploy/internal/git"
	"spdeploy/internal/github"
	"spdeploy/internal/logger"
)

type Monitor struct {
	config        *config.ConfigManager
	gitManager    *git.GitManager
	githubClient  *github.GitHubClient
	scriptExec    *ScriptExecutor
	pollInterval  time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	repositories  map[string]*RepositoryWatcher
	mu            sync.RWMutex
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

	return &Monitor{
		config:       config.NewConfig(),
		gitManager:   git.NewGitManagerWithToken(githubToken),
		githubClient: github.NewGitHubClient(githubToken),
		scriptExec:   NewScriptExecutor(5 * time.Minute),
		pollInterval: 60 * time.Second,
		ctx:          ctx,
		cancel:       cancel,
		repositories: make(map[string]*RepositoryWatcher),
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

	for _, repo := range repos {
		if !repo.Active {
			continue
		}

		// Create a GitManager for this repository with its specific token
		repoGitManager := git.NewGitManagerWithToken(repo.Token)

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

		// Validate or setup the repository
		err = repoGitManager.ValidateOrSetupRepo(repo.URL, repo.Branch, repo.Path)
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

func (m *Monitor) monitorLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

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

	// Check for changes via GitHub API
	change, err := m.githubClient.CheckForChanges(repo.URL, repo.Branch, repo.Trigger, watcher.LastSync)
	if err != nil {
		m.logError(watcher, "Failed to check for repository changes",
			zap.String("repo", repo.URL),
			zap.Error(err),
		)
		watcher.LastError = err
		return
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

	// Pull the latest changes using the repository's specific GitManager
	err = watcher.GitManager.PullLatestChanges(repo.Path)
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