package logger

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"gocd/internal/config"
)

// ShowContextualLogs shows logs based on the current directory and flags
func ShowContextualLogs(showGlobal bool, repoURL string, allRepos bool, username string, follow bool) {
	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		fmt.Printf("Error getting current user: %v\n", err)
		return
	}

	// If running as root and no user specified
	if currentUser.Uid == "0" && username == "" && !showGlobal {
		fmt.Println("Running as root. Please specify --user <username> to view user logs, or use --global for service logs")
		return
	}

	// Use specified username or current user
	targetUser := currentUser.Username
	if username != "" {
		targetUser = username
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting home directory: %v\n", err)
		return
	}

	var logFile string

	if showGlobal {
		// Show global service logs
		logFile = filepath.Join(homeDir, ".gocd", "logs", "global", targetUser, fmt.Sprintf("%s.log", time.Now().Format("2006-01-02")))
	} else if repoURL != "" {
		// Show logs for specific repository
		// Use the provided URL directly for consistency
		sanitizedRepo := sanitizeRepoURL(repoURL)
		logFile = filepath.Join(homeDir, ".gocd", "logs", "repos", sanitizedRepo, targetUser, fmt.Sprintf("%s.log", time.Now().Format("2006-01-02")))
	} else if allRepos {
		// List all repositories being monitored by the user
		listUserRepos(homeDir, targetUser)
		return
	} else {
		// Try to detect repository from current directory
		repoInfo, err := detectCurrentRepo()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			fmt.Println("You are not in a git repository. Use --repo <url> to specify a repository, or --global for service logs")
			return
		}

		// Check if this repo is monitored by the user
		cfg := config.NewConfig()
		repos, _ := cfg.ListRepositories()
		monitored := false
		for _, repo := range repos {
			if normalizeRepoURL(repo.URL) == normalizeRepoURL(repoInfo.URL) {
				monitored = true
				break
			}
		}

		if !monitored {
			fmt.Printf("This repository (%s) is not monitored by gocd\n", repoInfo.URL)
			fmt.Println("Add it with: gocd add --repo " + repoInfo.URL + " --path .")
			return
		}

		// Use the actual monitored repo URL for the log path
		sanitizedRepo := ""
		for _, repo := range repos {
			if normalizeRepoURL(repo.URL) == normalizeRepoURL(repoInfo.URL) {
				// Use the exact URL from the config for consistent log paths
				sanitizedRepo = sanitizeRepoURL(repo.URL)
				break
			}
		}
		logFile = filepath.Join(homeDir, ".gocd", "logs", "repos", sanitizedRepo, targetUser, fmt.Sprintf("%s.log", time.Now().Format("2006-01-02")))
	}

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Printf("No log file found at: %s\n", logFile)
		return
	}

	// Check permissions if viewing another user's logs
	if username != "" && currentUser.Uid != "0" {
		fmt.Println("Permission denied: Only root can view other users' logs")
		return
	}

	// Display the logs
	displayLogFile(logFile, follow)
}

// RepoInfo holds basic repository information
type RepoInfo struct {
	URL    string
	Branch string
	Path   string
}

// detectCurrentRepo detects if the current directory is a git repository
func detectCurrentRepo() (*RepoInfo, error) {
	// Check if we're in a git repository
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository")
	}

	repoPath := strings.TrimSpace(string(output))

	// Get the remote URL
	cmd = exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("no origin remote found")
	}

	remoteURL := strings.TrimSpace(string(output))

	// Get current branch
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoPath
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch")
	}

	branch := strings.TrimSpace(string(output))

	return &RepoInfo{
		URL:    remoteURL,
		Branch: branch,
		Path:   repoPath,
	}, nil
}

// normalizeRepoURL normalizes repository URLs for comparison
func normalizeRepoURL(url string) string {
	normalized := url

	// Remove any embedded tokens (format: https://token@github.com/...)
	if strings.Contains(normalized, "@") && strings.HasPrefix(normalized, "http") {
		parts := strings.SplitN(normalized, "@", 2)
		if len(parts) == 2 {
			// Reconstruct without token
			protocol := "https://"
			if strings.HasPrefix(normalized, "http://") {
				protocol = "http://"
			}
			normalized = protocol + parts[1]
		}
	}

	// Convert SSH to HTTPS format for comparison
	if strings.HasPrefix(normalized, "git@github.com:") {
		normalized = "https://github.com/" + strings.TrimPrefix(normalized, "git@github.com:")
	}

	// Remove .git suffix
	normalized = strings.TrimSuffix(normalized, ".git")

	// Remove protocol
	normalized = strings.TrimPrefix(normalized, "https://")
	normalized = strings.TrimPrefix(normalized, "http://")

	// Extract just the host and path
	normalized = strings.ToLower(normalized)

	return normalized
}

// listUserRepos lists all repositories being monitored by a user
func listUserRepos(homeDir, username string) {
	reposDir := filepath.Join(homeDir, ".gocd", "logs", "repos")

	entries, err := os.ReadDir(reposDir)
	if err != nil {
		fmt.Println("No repositories are being monitored")
		return
	}

	fmt.Printf("Repositories monitored by %s:\n", username)
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if user has logs in this repo
			userLogDir := filepath.Join(reposDir, entry.Name(), username)
			if _, err := os.Stat(userLogDir); err == nil {
				fmt.Printf("  - %s\n", entry.Name())
			}
		}
	}
}

// displayLogFile displays the contents of a log file
func displayLogFile(logFile string, follow bool) {
	file, err := os.Open(logFile)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}
	defer file.Close()

	if follow {
		// Follow mode - tail the file
		followLogFile(file)
	} else {
		// Show existing logs
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading log file: %v\n", err)
		}
	}
}

// followLogFile tails a log file and displays new content as it arrives
func followLogFile(file *os.File) {
	// Seek to end of file
	file.Seek(0, 2)

	scanner := bufio.NewScanner(file)
	for {
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading log file: %v\n", err)
			return
		}

		// Wait a bit before checking for new content
		time.Sleep(500 * time.Millisecond)
	}
}