package internal

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"spdeploy/internal/logger"
)

type Logger struct {
	logger *log.Logger
	file   *os.File
	debug  bool
}

func NewLogger() *Logger {
	// Create log directory
	homeDir, _ := os.UserHomeDir()
	logDir := filepath.Join(homeDir, ".spdeploy", "logs")
	os.MkdirAll(logDir, 0755)

	// Create or open log file
	logFile := filepath.Join(logDir, "spdeploy.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fall back to stderr if we can't open log file
		return &Logger{
			logger: log.New(os.Stderr, "", log.LstdFlags),
			debug:  os.Getenv("SPDEPLOY_DEBUG") != "",
		}
	}

	return &Logger{
		logger: log.New(file, "", 0),
		file:   file,
		debug:  os.Getenv("SPDEPLOY_DEBUG") != "",
	}
}

func (l *Logger) formatMessage(level, format string, args ...interface{}) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	return fmt.Sprintf("[%s] %s: %s", timestamp, level, message)
}

func (l *Logger) Info(format string, args ...interface{}) {
	msg := l.formatMessage("INFO", format, args...)
	l.logger.Println(msg)
	if os.Getenv("SPDEPLOY_QUIET") == "" {
		fmt.Println(msg)
	}
}

func (l *Logger) Error(format string, args ...interface{}) {
	msg := l.formatMessage("ERROR", format, args...)
	l.logger.Println(msg)
	fmt.Fprintln(os.Stderr, msg)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if !l.debug {
		return
	}
	msg := l.formatMessage("DEBUG", format, args...)
	l.logger.Println(msg)
	if os.Getenv("SPDEPLOY_QUIET") == "" {
		fmt.Println(msg)
	}
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

// Wrapper functions for the advanced logger package

// InitLogger initializes the global logger
func InitLogger() error {
	return logger.InitLogger()
}

// ShowContextualLogs shows logs based on the current directory and flags
func ShowContextualLogs(showGlobal bool, repoURL string, allRepos bool, username string, follow bool) {
	logger.ShowContextualLogs(showGlobal, repoURL, allRepos, username, follow)
}