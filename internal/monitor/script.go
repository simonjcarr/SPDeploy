package monitor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"gocd/internal/logger"
)

type ScriptExecutor struct {
	timeout time.Duration
}

type ScriptResult struct {
	Success    bool
	Output     string
	Error      string
	ExitCode   int
	Duration   time.Duration
	ScriptPath string
}

func NewScriptExecutor(timeout time.Duration) *ScriptExecutor {
	if timeout == 0 {
		timeout = 5 * time.Minute // Default 5 minute timeout
	}

	return &ScriptExecutor{
		timeout: timeout,
	}
}

func (se *ScriptExecutor) FindScript(repoPath string) (string, error) {
	var scriptNames []string

	// Check for platform-specific scripts first, then fallback to generic
	if runtime.GOOS == "windows" {
		scriptNames = []string{"gocd.bat", "gocd.cmd", "gocd.ps1", "gocd.sh"}
	} else {
		scriptNames = []string{"gocd.sh", "gocd"}
	}

	for _, scriptName := range scriptNames {
		scriptPath := filepath.Join(repoPath, scriptName)
		if info, err := os.Stat(scriptPath); err == nil {
			if info.Mode().IsRegular() {
				// Check if file is executable (Unix-like systems)
				if runtime.GOOS != "windows" {
					if info.Mode()&0111 == 0 {
						// File exists but is not executable, try to make it executable
						if err := os.Chmod(scriptPath, info.Mode()|0755); err != nil {
							logger.Warn("Found script but couldn't make it executable",
								zap.String("script", scriptPath),
								zap.Error(err),
							)
							continue
						}
					}
				}

				logger.Info("Found deployment script", zap.String("script", scriptPath))
				return scriptPath, nil
			}
		}
	}

	return "", nil // No script found, which is fine
}

func (se *ScriptExecutor) ExecuteScript(scriptPath, repoPath string) *ScriptResult {
	start := time.Now()
	result := &ScriptResult{
		ScriptPath: scriptPath,
	}

	logger.Info("Executing deployment script",
		zap.String("script", scriptPath),
		zap.String("repo_path", repoPath),
		zap.Duration("timeout", se.timeout),
	)

	// Prepare the command
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if strings.HasSuffix(scriptPath, ".ps1") {
			cmd = exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
		} else if strings.HasSuffix(scriptPath, ".bat") || strings.HasSuffix(scriptPath, ".cmd") {
			cmd = exec.Command("cmd", "/C", scriptPath)
		} else {
			cmd = exec.Command(scriptPath)
		}
	} else {
		// Unix-like systems
		if strings.HasSuffix(scriptPath, ".sh") {
			cmd = exec.Command("/bin/bash", scriptPath)
		} else {
			cmd = exec.Command(scriptPath)
		}
	}

	// Set working directory to the repository path
	cmd.Dir = repoPath

	// Set up environment variables
	cmd.Env = se.prepareEnvironment(repoPath)

	// Create a channel to signal completion
	done := make(chan error, 1)
	var output []byte
	var err error

	// Run the command in a goroutine
	go func() {
		output, err = cmd.CombinedOutput()
		done <- err
	}()

	// Wait for completion or timeout
	select {
	case err = <-done:
		result.Duration = time.Since(start)
		result.Output = string(output)

		if err != nil {
			result.Success = false
			result.Error = err.Error()

			if exitError, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitError.ExitCode()
			} else {
				result.ExitCode = -1
			}

			logger.LogScriptExecution(repoPath, filepath.Base(scriptPath), "failed", result.Output, err)
		} else {
			result.Success = true
			result.ExitCode = 0
			logger.LogScriptExecution(repoPath, filepath.Base(scriptPath), "success", result.Output, nil)
		}

	case <-time.After(se.timeout):
		// Timeout occurred
		if cmd.Process != nil {
			cmd.Process.Kill()
		}

		result.Duration = se.timeout
		result.Success = false
		result.Error = fmt.Sprintf("script execution timed out after %v", se.timeout)
		result.ExitCode = -1

		logger.LogScriptExecution(repoPath, filepath.Base(scriptPath), "timeout", "",
			fmt.Errorf("script execution timed out after %v", se.timeout))
	}

	return result
}

func (se *ScriptExecutor) prepareEnvironment(repoPath string) []string {
	env := os.Environ()

	// Add GoCD-specific environment variables
	goCDEnv := []string{
		"GOCD_REPO_PATH=" + repoPath,
		"GOCD_TIMESTAMP=" + time.Now().Format(time.RFC3339),
		"GOCD_VERSION=1.0.0",
	}

	// Add Git information if available
	if gitInfo := se.getGitInfo(repoPath); gitInfo != nil {
		goCDEnv = append(goCDEnv,
			"GOCD_GIT_BRANCH="+gitInfo.Branch,
			"GOCD_GIT_COMMIT="+gitInfo.Commit,
			"GOCD_GIT_REMOTE="+gitInfo.Remote,
		)
	}

	return append(env, goCDEnv...)
}

type GitInfo struct {
	Branch string
	Commit string
	Remote string
}

func (se *ScriptExecutor) getGitInfo(repoPath string) *GitInfo {
	// Try to get git information
	gitInfo := &GitInfo{}

	// Get current branch
	if cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"); cmd != nil {
		cmd.Dir = repoPath
		if output, err := cmd.Output(); err == nil {
			gitInfo.Branch = strings.TrimSpace(string(output))
		}
	}

	// Get current commit hash
	if cmd := exec.Command("git", "rev-parse", "HEAD"); cmd != nil {
		cmd.Dir = repoPath
		if output, err := cmd.Output(); err == nil {
			gitInfo.Commit = strings.TrimSpace(string(output))
		}
	}

	// Get remote URL
	if cmd := exec.Command("git", "remote", "get-url", "origin"); cmd != nil {
		cmd.Dir = repoPath
		if output, err := cmd.Output(); err == nil {
			gitInfo.Remote = strings.TrimSpace(string(output))
		}
	}

	// Return nil if we couldn't get any git info
	if gitInfo.Branch == "" && gitInfo.Commit == "" && gitInfo.Remote == "" {
		return nil
	}

	return gitInfo
}

func (se *ScriptExecutor) ValidateScript(scriptPath string) error {
	info, err := os.Stat(scriptPath)
	if err != nil {
		return fmt.Errorf("script file not accessible: %w", err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("script path is not a regular file")
	}

	// On Unix-like systems, check if the file is executable
	if runtime.GOOS != "windows" {
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("script file is not executable")
		}
	}

	return nil
}