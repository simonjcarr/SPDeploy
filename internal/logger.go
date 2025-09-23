package internal

import (
	"spdeploy/internal/logger"
)

// Wrapper functions for the advanced logger package

// InitLogger initializes the global logger
func InitLogger() error {
	return logger.InitLogger()
}

// ShowContextualLogs shows logs based on the current directory and flags
func ShowContextualLogs(showGlobal bool, repoURL string, allRepos bool, username string, follow bool) {
	logger.ShowContextualLogs(showGlobal, repoURL, allRepos, username, follow)
}