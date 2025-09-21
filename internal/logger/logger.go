package logger

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.Logger

func init() {
	// Initialize with a basic logger
	config := zap.NewDevelopmentConfig()
	config.OutputPaths = []string{"stdout"}
	logger, _ := config.Build()
	globalLogger = logger
}

func InitLogger() error {
	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		currentUser = &user.User{Username: "system"}
	}

	// Always use ~/.gocd/logs for consistency
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create global log directory for this user
	logDir := filepath.Join(homeDir, ".gocd", "logs", "global", currentUser.Username)

	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file with timestamp
	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", time.Now().Format("2006-01-02")))

	// Configure zap logger
	zapConfig := zap.NewProductionConfig()
	zapConfig.OutputPaths = []string{
		"stdout",
		logFile,
	}
	zapConfig.ErrorOutputPaths = []string{
		"stderr",
		logFile,
	}

	// Custom encoder config for better readability
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
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	globalLogger = logger
	return nil
}

func GetLogger() *zap.Logger {
	return globalLogger
}

func Info(msg string, fields ...zap.Field) {
	globalLogger.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	globalLogger.Error(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	globalLogger.Debug(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	globalLogger.Warn(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	globalLogger.Fatal(msg, fields...)
}

// ShowLogs shows logs (deprecated - use ShowContextualLogs in log_viewer.go)
func ShowLogs(follow bool) {
	// This function is kept for backward compatibility
	// but now just calls the new context-aware log viewer
	ShowContextualLogs(false, "", false, "", follow)
}


func LogRepoEvent(repoURL, branch, event, details string) {
	Info("Repository event",
		zap.String("repo", repoURL),
		zap.String("branch", branch),
		zap.String("event", event),
		zap.String("details", details),
	)
}

func LogDeployment(repoURL, branch, status string, err error) {
	fields := []zap.Field{
		zap.String("repo", repoURL),
		zap.String("branch", branch),
		zap.String("status", status),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		Error("Deployment failed", fields...)
	} else {
		Info("Deployment completed", fields...)
	}
}

func LogScriptExecution(repoPath, scriptName, status string, output string, err error) {
	fields := []zap.Field{
		zap.String("repo_path", repoPath),
		zap.String("script", scriptName),
		zap.String("status", status),
		zap.String("output", output),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		Error("Script execution failed", fields...)
	} else {
		Info("Script executed successfully", fields...)
	}
}