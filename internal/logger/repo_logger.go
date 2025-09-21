package logger

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// RepoLogger represents a logger for a specific repository and user
type RepoLogger struct {
	logger   *zap.Logger
	repoURL  string
	username string
	repoPath string
}

// sanitizeRepoURL converts a repository URL to a safe directory name
func sanitizeRepoURL(repoURL string) string {
	cleaned := repoURL

	// Remove any embedded tokens (format: https://token@github.com/...)
	if strings.Contains(cleaned, "@") && strings.HasPrefix(cleaned, "http") {
		parts := strings.SplitN(cleaned, "@", 2)
		if len(parts) == 2 {
			// Take only the part after the @
			cleaned = parts[1]
		}
	} else {
		// Remove protocol prefixes
		cleaned = strings.TrimPrefix(cleaned, "https://")
		cleaned = strings.TrimPrefix(cleaned, "http://")
	}

	// Handle SSH format
	cleaned = strings.TrimPrefix(cleaned, "git@")

	// Replace special characters with hyphens
	re := regexp.MustCompile(`[^a-zA-Z0-9-._]+`)
	cleaned = re.ReplaceAllString(cleaned, "-")

	// Remove trailing .git
	cleaned = strings.TrimSuffix(cleaned, ".git")

	// Remove any leading/trailing hyphens
	cleaned = strings.Trim(cleaned, "-")

	return cleaned
}

// NewRepoLogger creates a new repository-specific logger
func NewRepoLogger(repoURL, repoPath string) (*RepoLogger, error) {
	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	// Determine log directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create repository-specific log directory
	sanitizedRepo := sanitizeRepoURL(repoURL)
	logDir := filepath.Join(homeDir, ".gocd", "logs", "repos", sanitizedRepo, currentUser.Username)

	// Ensure log directory exists with proper permissions
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file with date
	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", time.Now().Format("2006-01-02")))

	// Configure zap logger
	zapConfig := zap.NewProductionConfig()
	zapConfig.OutputPaths = []string{logFile}
	zapConfig.ErrorOutputPaths = []string{logFile}

	// Add user and repo context to all logs
	zapConfig.InitialFields = map[string]interface{}{
		"user": currentUser.Username,
		"repo": repoURL,
		"path": repoPath,
	}

	// Custom encoder config
	zapConfig.EncoderConfig = zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &RepoLogger{
		logger:   logger,
		repoURL:  repoURL,
		username: currentUser.Username,
		repoPath: repoPath,
	}, nil
}

// Info logs an info message
func (r *RepoLogger) Info(msg string, fields ...zap.Field) {
	r.logger.Info(msg, fields...)
}

// Error logs an error message
func (r *RepoLogger) Error(msg string, fields ...zap.Field) {
	r.logger.Error(msg, fields...)
}

// Debug logs a debug message
func (r *RepoLogger) Debug(msg string, fields ...zap.Field) {
	r.logger.Debug(msg, fields...)
}

// Warn logs a warning message
func (r *RepoLogger) Warn(msg string, fields ...zap.Field) {
	r.logger.Warn(msg, fields...)
}

// Fatal logs a fatal message and exits
func (r *RepoLogger) Fatal(msg string, fields ...zap.Field) {
	r.logger.Fatal(msg, fields...)
}

// Close closes the logger
func (r *RepoLogger) Close() {
	r.logger.Sync()
}