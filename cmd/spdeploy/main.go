package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"spdeploy/internal/auth"
	"spdeploy/internal/config"
	"spdeploy/internal/daemon"
	"spdeploy/internal/logger"
	"spdeploy/internal/monitor"
)

var (
	// Version information - set at build time via ldflags
	AppName   = "SPDeploy"
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"

	cfgFile string
	logFlag bool
	followFlag bool
	serviceFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "spdeploy",
	Short: "Continuous deployment service for GitHub repositories",
	Long: `SPDeploy is a lightweight continuous deployment service that automatically
syncs your code from GitHub to your local directories when changes are detected.`,
	Version: fmt.Sprintf("%s\nCommit: %s\nBuilt: %s", Version, GitCommit, BuildDate),
	Run: func(cmd *cobra.Command, args []string) {
		if serviceFlag {
			runAsService()
			return
		}
		if logFlag {
			logger.ShowLogs(followFlag)
			return
		}
		cmd.Help()
	},
}

// repoCmd represents the repository management command group
var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage GitHub repositories",
	Long:  `Manage GitHub repositories for continuous deployment monitoring`,
}

var repoAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a repository to monitor",
	Long:  `Add a GitHub repository to monitor for changes`,
	Run: func(cmd *cobra.Command, args []string) {
		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		path, _ := cmd.Flags().GetString("path")
		trigger, _ := cmd.Flags().GetString("trigger")
		useToken, _ := cmd.Flags().GetBool("with-token")

		if repo == "" {
			fmt.Println("Error: --repo flag is required")
			os.Exit(1)
		}

		if path == "" {
			fmt.Println("Error: --path flag is required")
			fmt.Println("Please specify a deployment path where you have write permissions")
			os.Exit(1)
		}

		// Validate deployment path
		if err := validateDeploymentPath(path); err != nil {
			fmt.Printf("Error: Invalid deployment path: %v\n", err)
			fmt.Println("Please ensure you have write permissions to the specified directory")
			os.Exit(1)
		}

		// Detect URL type and guide authentication
		isSSH := strings.HasPrefix(repo, "git@") || (strings.Contains(repo, ":") && !strings.HasPrefix(repo, "http"))
		isHTTPS := strings.HasPrefix(repo, "https://") || strings.HasPrefix(repo, "http://")

		var token string
		if isSSH {
			// SSH URL - check for SSH keys
			homeDir, _ := os.UserHomeDir()
			sshKeyPaths := []string{
				filepath.Join(homeDir, ".ssh", "id_ed25519"),
				filepath.Join(homeDir, ".ssh", "id_rsa"),
				filepath.Join(homeDir, ".ssh", "id_ecdsa"),
			}
			keyFound := false
			for _, keyPath := range sshKeyPaths {
				if _, err := os.Stat(keyPath); err == nil {
					keyFound = true
					break
				}
			}
			if !keyFound {
				fmt.Printf("⚠️  SSH URL detected but no SSH keys found.\n")
				fmt.Printf("   Run 'spdeploy repo auth %s' for setup instructions.\n", repo)
			}
		} else if isHTTPS && useToken {
			// HTTPS URL with token flag
			authenticator := auth.NewPATAuthenticator()
			storedToken, err := authenticator.GetStoredToken()
			if err != nil || storedToken == "" {
				fmt.Println("No stored GitHub token found.")
				fmt.Printf("Please run 'spdeploy repo auth %s' first to set up GitHub authentication.\n", repo)
				os.Exit(1)
			}
			token = storedToken
			fmt.Println("✓ Using stored GitHub token for this repository")
		} else if isHTTPS && !useToken {
			fmt.Println("ℹ️  HTTPS URL detected. For private repositories, you may want to:")
			fmt.Printf("   1. Run 'spdeploy repo auth %s' to set up a PAT token\n", repo)
			fmt.Println("   2. Then add with: spdeploy repo add --repo", repo, "--with-token")
		}

		cfg := config.NewConfig()
		err := cfg.AddRepositoryWithToken(repo, branch, path, trigger, token)
		if err != nil {
			fmt.Printf("Error adding repository: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Successfully added repository: %s\n", repo)
		if isSSH {
			fmt.Println("   Authentication: SSH keys")
			fmt.Println("   Note: Ensure SSH keys are available when the service runs")
		} else if token != "" {
			fmt.Println("   Authentication: GitHub Personal Access Token")
		} else {
			fmt.Println("   Authentication: None (public repository only)")
		}
		fmt.Printf("   Branch: %s\n", branch)
		if path != "" {
			fmt.Printf("   Path: %s\n", path)
		}
	},
}

// validateDeploymentPath checks if the path exists or can be created, and if we have write permissions
func validateDeploymentPath(path string) error {
	// Expand home directory if path starts with ~
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot expand home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[2:])
	}

	// Make path absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		// Try to create the directory
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("cannot create directory: %w", err)
		}
		fmt.Printf("✓ Created deployment directory: %s\n", absPath)
	} else if err != nil {
		return fmt.Errorf("cannot access path: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("path exists but is not a directory")
	}

	// Test write permission by creating and removing a test file
	testFile := filepath.Join(absPath, ".spdeploy-permission-test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("no write permission in directory: %w", err)
	}

	// Clean up test file
	if err := os.Remove(testFile); err != nil {
		// Not critical if we can't remove it, but log it
		fmt.Printf("Warning: Could not remove test file %s: %v\n", testFile, err)
	}

	fmt.Printf("✓ Validated deployment path: %s\n", absPath)
	return nil
}

// Helper function to show OS-specific SSH setup instructions
func showSSHSetupInstructions(repoURL string) {
	fmt.Printf("You're using an SSH URL (%s).\n", repoURL)
	fmt.Println("SSH URLs authenticate using SSH keys, not PAT tokens.")
	fmt.Println()

	switch runtime.GOOS {
	case "darwin":
		fmt.Println("To set up SSH authentication on macOS:")
		fmt.Println("1. Generate an SSH key:")
		fmt.Println("   ssh-keygen -t ed25519 -C \"your-email@example.com\"")
		fmt.Println()
		fmt.Println("2. Add the key to ssh-agent:")
		fmt.Println("   ssh-add ~/.ssh/id_ed25519")
		fmt.Println()
		fmt.Println("3. Copy your public key:")
		fmt.Println("   pbcopy < ~/.ssh/id_ed25519.pub")
		fmt.Println()
		fmt.Println("4. Add it to GitHub: Settings → SSH and GPG keys → New SSH key")
		fmt.Println("5. Test connection: ssh -T git@github.com")

	case "linux":
		fmt.Println("To set up SSH authentication on Linux:")
		fmt.Println("1. Generate an SSH key:")
		fmt.Println("   ssh-keygen -t ed25519 -C \"your-email@example.com\"")
		fmt.Println()
		fmt.Println("2. Start ssh-agent and add key:")
		fmt.Println("   eval \"$(ssh-agent -s)\"")
		fmt.Println("   ssh-add ~/.ssh/id_ed25519")
		fmt.Println()
		fmt.Println("3. Copy your public key:")
		fmt.Println("   cat ~/.ssh/id_ed25519.pub")
		fmt.Println("   # Then manually copy the output")
		fmt.Println()
		fmt.Println("4. Add it to GitHub: Settings → SSH and GPG keys → New SSH key")
		fmt.Println("5. Test connection: ssh -T git@github.com")

	case "windows":
		fmt.Println("To set up SSH authentication on Windows:")
		fmt.Println("1. Generate an SSH key (in PowerShell or Git Bash):")
		fmt.Println("   ssh-keygen -t ed25519 -C \"your-email@example.com\"")
		fmt.Println()
		fmt.Println("2. Add the key to ssh-agent:")
		fmt.Println("   # In PowerShell (as Administrator):")
		fmt.Println("   Get-Service -Name ssh-agent | Set-Service -StartupType Automatic")
		fmt.Println("   Start-Service ssh-agent")
		fmt.Println("   ssh-add $HOME\\.ssh\\id_ed25519")
		fmt.Println()
		fmt.Println("   # OR in Git Bash:")
		fmt.Println("   eval \"$(ssh-agent -s)\"")
		fmt.Println("   ssh-add ~/.ssh/id_ed25519")
		fmt.Println()
		fmt.Println("3. Copy your public key:")
		fmt.Println("   # PowerShell:")
		fmt.Println("   Get-Content $HOME\\.ssh\\id_ed25519.pub | Set-Clipboard")
		fmt.Println()
		fmt.Println("   # OR Git Bash:")
		fmt.Println("   cat ~/.ssh/id_ed25519.pub")
		fmt.Println("   # Then manually copy the output")
		fmt.Println()
		fmt.Println("4. Add it to GitHub: Settings → SSH and GPG keys → New SSH key")
		fmt.Println("5. Test connection: ssh -T git@github.com")

	default:
		// Generic Unix-like instructions
		fmt.Println("To set up SSH authentication:")
		fmt.Println("1. Generate an SSH key:")
		fmt.Println("   ssh-keygen -t ed25519 -C \"your-email@example.com\"")
		fmt.Println()
		fmt.Println("2. Add the key to ssh-agent:")
		fmt.Println("   ssh-add ~/.ssh/id_ed25519")
		fmt.Println()
		fmt.Println("3. Display your public key:")
		fmt.Println("   cat ~/.ssh/id_ed25519.pub")
		fmt.Println("   # Copy the output")
		fmt.Println()
		fmt.Println("4. Add it to GitHub: Settings → SSH and GPG keys → New SSH key")
		fmt.Println("5. Test connection: ssh -T git@github.com")
	}
}

var repoAuthCmd = &cobra.Command{
	Use:   "auth [url]",
	Short: "Set up authentication for repositories",
	Long:  `Set up authentication for GitHub repositories.
Provide a repository URL to get specific instructions for SSH or HTTPS authentication.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logout, _ := cmd.Flags().GetBool("logout")

		// If a URL is provided, check if it's SSH or HTTPS
		if len(args) > 0 {
			repoURL := args[0]
			if strings.HasPrefix(repoURL, "git@") || strings.Contains(repoURL, ":") && !strings.HasPrefix(repoURL, "http") {
				// SSH URL detected
				showSSHSetupInstructions(repoURL)
				return
			}
			// HTTPS URL - continue with PAT token flow
			fmt.Printf("You're using an HTTPS URL (%s).\n", repoURL)
			fmt.Println("HTTPS URLs can authenticate using Personal Access Tokens (PAT).")
			fmt.Println()
		}

		authenticator := auth.NewPATAuthenticator()

		if logout {
			if err := authenticator.ClearToken(); err != nil {
				fmt.Printf("Error clearing token: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✅ GitHub authentication cleared")
			return
		}

		// Check if token already exists
		existingToken, err := authenticator.GetStoredToken()
		if err != nil {
			fmt.Printf("Error checking existing token: %v\n", err)
		} else if existingToken != "" {
			fmt.Println("🔐 Validating existing GitHub token...")
			if err := authenticator.ValidateToken(existingToken); err == nil {
				fmt.Println("✅ Valid GitHub token already configured")
				fmt.Println("Use 'spdeploy repo auth --logout' to clear and re-authenticate")
				return
			}
			fmt.Println("⚠️  Existing token is invalid, re-authenticating...")
		}

		// Perform authentication using PAT flow
		token, err := authenticator.AuthenticateInteractive()
		if err != nil {
			fmt.Printf("❌ Authentication failed: %v\n", err)
			os.Exit(1)
		}

		// Set environment variable for current session
		os.Setenv("GITHUB_TOKEN", token)
	},
}

var repoRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a repository from monitoring",
	Long:  `Remove a GitHub repository from monitoring. Can remove by repository URL or by specific directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		repo, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		path, _ := cmd.Flags().GetString("path")
		all, _ := cmd.Flags().GetBool("all")

		if repo == "" {
			fmt.Println("Error: --repo flag is required")
			os.Exit(1)
		}

		cfg := config.NewConfig()

		// If --all flag is set, remove all instances of the repo
		if all {
			count, err := cfg.RemoveAllRepository(repo, branch)
			if err != nil {
				fmt.Printf("Error removing repository: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Successfully removed %d instance(s) of repository: %s\n", count, repo)
			return
		}

		// Remove specific instance
		err := cfg.RemoveRepositoryByPath(repo, branch, path)
		if err != nil {
			fmt.Printf("Error removing repository: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully removed repository: %s", repo)
		if path != "" {
			fmt.Printf(" from path: %s", path)
		}
		fmt.Println()
	},
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all monitored repositories",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.NewConfig()
		repos, err := cfg.ListRepositories()
		if err != nil {
			fmt.Printf("Error listing repositories: %v\n", err)
			os.Exit(1)
		}

		if len(repos) == 0 {
			fmt.Println("No repositories configured")
			return
		}

		fmt.Println("Monitored repositories:")
		for _, repo := range repos {
			fmt.Printf("  %s (branch: %s, path: %s, trigger: %s)\n", repo.URL, repo.Branch, repo.Path, repo.Trigger)
		}
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the SPDeploy daemon service",
	Run: func(cmd *cobra.Command, args []string) {
		err := daemon.Start()
		if err != nil {
			fmt.Printf("Error starting daemon: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("SPDeploy daemon started successfully")
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the SPDeploy daemon service",
	Run: func(cmd *cobra.Command, args []string) {
		err := daemon.Stop()
		if err != nil {
			fmt.Printf("Error stopping daemon: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("SPDeploy daemon stopped successfully")
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status and monitored repositories",
	Run: func(cmd *cobra.Command, args []string) {
		running := daemon.IsRunning()
		if running {
			fmt.Println("SPDeploy daemon is running")
		} else {
			fmt.Println("SPDeploy daemon is not running")
		}

		cfg := config.NewConfig()
		repos, err := cfg.ListRepositories()
		if err != nil {
			fmt.Printf("Error getting repositories: %v\n", err)
			return
		}

		fmt.Printf("Monitoring %d repositories\n", len(repos))
	},
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install SPDeploy for the current user",
	Long:  `Install SPDeploy and set up a user service that runs in your user context.
This ensures you have full access to your deployment directories.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := installSPDeploy()
		if err != nil {
			fmt.Printf("Installation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ SPDeploy installed successfully!")
		fmt.Println()
		fmt.Println("Quick start:")
		fmt.Println("  1. Set up authentication: spdeploy repo auth <repository-url>")
		fmt.Println("  2. Add a repository: spdeploy repo add --repo <repository-url>")
		fmt.Println("  3. Start monitoring: spdeploy start")
		fmt.Println("  4. View logs: spdeploy log")
	},
}

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "View deployment logs",
	Long: `View deployment logs for repositories. By default, shows logs for the repository
in the current directory. Use flags to view global service logs or specify a repository.`,
	Run: func(cmd *cobra.Command, args []string) {
		showGlobal, _ := cmd.Flags().GetBool("global")
		repoURL, _ := cmd.Flags().GetString("repo")
		allRepos, _ := cmd.Flags().GetBool("all")
		username, _ := cmd.Flags().GetString("user")
		follow, _ := cmd.Flags().GetBool("follow")

		logger.ShowContextualLogs(showGlobal, repoURL, allRepos, username, follow)
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall SPDeploy from the system",
	Long:  `Remove SPDeploy binaries and stop any running services`,
	Run: func(cmd *cobra.Command, args []string) {
		// Confirm uninstallation
		fmt.Print("Are you sure you want to uninstall SPDeploy? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Uninstallation cancelled")
			return
		}

		err := uninstallSPDeploy()
		if err != nil {
			fmt.Printf("Uninstallation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ SPDeploy uninstalled successfully!")
	},
}

func init() {
	// Set custom version template
	rootCmd.SetVersionTemplate(fmt.Sprintf(`%s {{.Version}}
`, AppName))

	// Add -v shorthand for --version
	rootCmd.Flags().BoolP("version", "v", false, "version for spdeploy")

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is system config location)")
	rootCmd.Flags().BoolVar(&logFlag, "log", false, "show logs")
	rootCmd.Flags().BoolVarP(&followFlag, "follow", "f", false, "follow logs in real-time")
	rootCmd.Flags().BoolVar(&serviceFlag, "service", false, "run as service (internal use)")
	rootCmd.Flags().MarkHidden("service")

	// Set up repo subcommands
	repoAddCmd.Flags().String("repo", "", "GitHub repository URL (required)")
	repoAddCmd.Flags().String("branch", "main", "branch to monitor")
	repoAddCmd.Flags().String("path", "", "deployment path where repository will be synced (required)")
	repoAddCmd.Flags().String("trigger", "push", "trigger type: push, pr, or both")
	repoAddCmd.Flags().Bool("with-token", false, "use stored GitHub token for this repository")
	repoAddCmd.MarkFlagRequired("repo")
	repoAddCmd.MarkFlagRequired("path")

	repoAuthCmd.Flags().Bool("logout", false, "clear stored GitHub token")

	repoRemoveCmd.Flags().String("repo", "", "GitHub repository URL (required)")
	repoRemoveCmd.Flags().String("branch", "main", "branch to remove (defaults to main)")
	repoRemoveCmd.Flags().String("path", "", "specific local path to remove from (optional)")
	repoRemoveCmd.Flags().Bool("all", false, "remove all instances of this repository")

	// Add subcommands to repo command
	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoAuthCmd)
	repoCmd.AddCommand(repoRemoveCmd)
	repoCmd.AddCommand(repoListCmd)

	logCmd.Flags().BoolP("follow", "f", false, "follow logs in real-time")
	logCmd.Flags().BoolP("global", "g", false, "show global service logs instead of repository logs")
	logCmd.Flags().String("repo", "", "show logs for specific repository URL")
	logCmd.Flags().BoolP("all", "a", false, "list all repositories being monitored")
	logCmd.Flags().String("user", "", "view logs for specific user (requires sudo)")

	// Add commands to root
	rootCmd.AddCommand(repoCmd)  // New repo command group
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
}

func installSPDeploy() error {
	// Get current executable path
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Determine install directory based on platform
	var installDir string
	if runtime.GOOS == "windows" {
		// On Windows, install to a location in PATH or create one
		installDir = "C:\\Program Files\\SPDeploy"
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return fmt.Errorf("failed to create install directory: %w", err)
		}
	} else {
		// Unix-like systems
		installDir = "/usr/local/bin"
	}

	// Define target path
	var target string
	if runtime.GOOS == "windows" {
		target = filepath.Join(installDir, "spdeploy.exe")
	} else {
		target = filepath.Join(installDir, "spdeploy")
	}

	// Stop any running user service first
	fmt.Println("🛑 Stopping any running SPDeploy service...")
	daemon.Stop() // Ignore errors if not running

	// Copy binary to system location (requires sudo)
	fmt.Println("📦 Installing SPDeploy binary to system location...")
	fmt.Printf("   This requires sudo access to copy to %s\n", target)

	var copyErr error
	if runtime.GOOS != "windows" {
		// Use sudo to copy on Unix-like systems
		cmd := exec.Command("sudo", "cp", executable, target)
		copyErr = cmd.Run()
		if copyErr == nil {
			// Make executable
			cmd = exec.Command("sudo", "chmod", "755", target)
			copyErr = cmd.Run()
		}
	} else {
		// Windows - try direct copy, may need admin rights
		copyErr = copyFile(executable, target)
	}

	if copyErr != nil {
		return fmt.Errorf("failed to install binary (may need admin/sudo access): %w", copyErr)
	}

	// Create user directories (not system directories)
	fmt.Println("📁 Creating user configuration directories...")
	if err := createUserDirectories(); err != nil {
		fmt.Printf("⚠️  Warning: Could not create user directories: %v\n", err)
	}

	// Create user service (not system service)
	fmt.Println("⚙️  Creating user service...")
	if err := createUserService(target); err != nil {
		fmt.Printf("⚠️  Warning: Could not create user service: %v\n", err)
		fmt.Println("   You can still use 'spdeploy start' to run manually")
	} else {
		fmt.Println("✅ User service created successfully")
		fmt.Println("")
		fmt.Println("Service will run as:", os.Getenv("USER"))
		fmt.Println("To start the service: spdeploy start")
		fmt.Println("To view logs: spdeploy log")
	}

	// On Windows, add to PATH if needed
	if runtime.GOOS == "windows" {
		fmt.Println("💡 Note: You may need to add", installDir, "to your PATH environment variable")
	}

	fmt.Println("🎯 Installation completed to:", installDir)
	return nil
}

func uninstallSPDeploy() error {
	// Stop and remove system service
	fmt.Println("🛑 Stopping and removing system service...")
	if err := daemon.Stop(); err != nil {
		fmt.Printf("⚠️  Warning: Could not stop service: %v\n", err)
	}
	if err := removeSystemService(); err != nil {
		fmt.Printf("⚠️  Warning: Could not remove system service: %v\n", err)
	}

	// Determine install paths
	var installDir string
	if runtime.GOOS == "windows" {
		installDir = "C:\\Program Files\\SPDeploy"
	} else {
		installDir = "/usr/local/bin"
	}

	// Remove binary
	var target string
	if runtime.GOOS == "windows" {
		target = filepath.Join(installDir, "spdeploy.exe")
	} else {
		target = filepath.Join(installDir, "spdeploy")
	}

	fmt.Println("🗑️  Removing binary...")

	// Remove binary
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove binary: %w", err)
	}

	// On Windows, remove install directory if empty
	if runtime.GOOS == "windows" {
		if entries, err := os.ReadDir(installDir); err == nil && len(entries) == 0 {
			os.Remove(installDir)
		}
	}

	// Optionally remove user config (ask user)
	cfg := config.NewConfig()
	configDir := filepath.Dir(cfg.GetConfigPath())
	if _, err := os.Stat(configDir); err == nil {
		fmt.Print("🗂️  Remove configuration and logs? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if response == "y" || response == "Y" {
			if err := os.RemoveAll(configDir); err != nil {
				fmt.Printf("⚠️  Warning: Could not remove config directory: %v\n", err)
			} else {
				fmt.Println("🗑️  Configuration and logs removed")
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	// For system installation, we need sudo on Unix systems
	if runtime.GOOS != "windows" && filepath.Dir(dst) == "/usr/local/bin" {
		// Use sudo to copy
		cmd := exec.Command("sudo", "cp", src, dst)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	// Regular file copy for other cases
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}

func runAsService() {
	// Initialize logging
	if err := logger.InitLogger(); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Starting SPDeploy service")

	// Write our PID
	cfg := config.NewConfig()
	pidFile := filepath.Join(filepath.Dir(cfg.GetConfigPath()), "spdeploy.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		logger.Error("Failed to write PID file", zap.Error(err))
		os.Exit(1)
	}

	// Initialize the monitor
	// Try to get token from environment first, then from stored token
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		authenticator := auth.NewPATAuthenticator()
		storedToken, err := authenticator.GetStoredToken()
		if err != nil {
			logger.Warn("Failed to load stored GitHub token", zap.Error(err))
		} else if storedToken != "" {
			githubToken = storedToken
			logger.Info("Using stored GitHub token")
		}
	}
	mon := monitor.NewMonitor(githubToken)

	// Start monitoring
	if err := mon.Start(); err != nil {
		logger.Error("Failed to start monitor", zap.Error(err))
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Graceful shutdown
	mon.Stop()
	os.Remove(pidFile)

	logger.Info("Service stopped gracefully")
}

func createUserDirectories() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create user-specific directories
	configDir := filepath.Join(homeDir, ".config", "spdeploy")
	logDir := filepath.Join(homeDir, ".spdeploy", "logs")

	// Create config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create log directory
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	fmt.Printf("✓ Created user directories:\n")
	fmt.Printf("  Config: %s\n", configDir)
	fmt.Printf("  Logs: %s\n", logDir)

	return nil
}

// Deprecated - keeping for backward compatibility
func createSystemDirectories() error {
	return createUserDirectories()
}

func createUserService(binaryPath string) error {
	switch runtime.GOOS {
	case "linux":
		return createUserSystemdService(binaryPath)
	case "darwin":
		return createUserLaunchdService(binaryPath)
	case "windows":
		return createUserWindowsService(binaryPath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// Deprecated - keeping for backward compatibility
func createSystemService(binaryPath string) error {
	return createUserService(binaryPath)
}

func createUserSystemdService(binaryPath string) error {
	// User systemd service
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=SPDeploy Continuous Deployment Service (User)
After=network.target

[Service]
Type=simple
ExecStart=%s --service
Restart=always
RestartSec=5
Environment=PATH=/usr/local/bin:/usr/bin:/bin
Environment=HOME=%s

[Install]
WantedBy=default.target
`, binaryPath, homeDir)

	// Create user systemd directory
	systemdDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	serviceFile := filepath.Join(systemdDir, "spdeploy.service")

	// Write service file (no sudo needed for user services)
	if err := os.WriteFile(serviceFile, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd user daemon
	exec.Command("systemctl", "--user", "daemon-reload").Run()

	// Enable service for user
	if err := exec.Command("systemctl", "--user", "enable", "spdeploy.service").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	fmt.Println("✓ User systemd service created")
	fmt.Println("  Service file:", serviceFile)
	fmt.Println("  To start: systemctl --user start spdeploy")
	fmt.Println("  To check status: systemctl --user status spdeploy")

	return nil
}

// Deprecated system-wide version
func createSystemdService(binaryPath string) error {
	serviceContent := fmt.Sprintf(`[Unit]
Description=SPDeploy Continuous Deployment Service
After=network.target

[Service]
Type=simple
User=%s
ExecStart=%s --service
Restart=always
RestartSec=5
Environment=PATH=/usr/local/bin:/usr/bin:/bin
Environment=HOME=%s

[Install]
WantedBy=multi-user.target
`, os.Getenv("USER"), binaryPath, os.Getenv("HOME"))

	serviceFile := "/etc/systemd/system/spdeploy.service"

	// Write service file with sudo
	cmd := exec.Command("sudo", "tee", serviceFile)
	cmd.Stdin = strings.NewReader(serviceContent)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create systemd service file: %w", err)
	}

	// Reload systemd and enable service
	if err := exec.Command("sudo", "systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	if err := exec.Command("sudo", "systemctl", "enable", "spdeploy.service").Run(); err != nil {
		return fmt.Errorf("failed to enable systemd service: %w", err)
	}

	return nil
}

func createUserLaunchdService(binaryPath string) error {
	// User LaunchAgent for macOS
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.spdeploy.user</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>--service</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s/.spdeploy/logs/service.log</string>
    <key>StandardErrorPath</key>
    <string>%s/.spdeploy/logs/service.error.log</string>
    <key>WorkingDirectory</key>
    <string>%s</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
        <key>HOME</key>
        <string>%s</string>
    </dict>
</dict>
</plist>`, binaryPath, homeDir, homeDir, homeDir, homeDir)

	// Create LaunchAgents directory if needed
	launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	plistPath := filepath.Join(launchAgentsDir, "com.spdeploy.user.plist")

	// Write plist file (no sudo needed for user LaunchAgents)
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	// Load the launch agent
	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		// Try to unload first in case it's already loaded
		exec.Command("launchctl", "unload", plistPath).Run()
		if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
			return fmt.Errorf("failed to load launch agent: %w", err)
		}
	}

	fmt.Println("✓ User LaunchAgent created")
	fmt.Println("  Plist file:", plistPath)
	fmt.Println("  To start: launchctl start com.spdeploy.user")
	fmt.Println("  To check: launchctl list | grep spdeploy")

	return nil
}

// Deprecated system-wide version
func createLaunchdService(binaryPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.spdeploy.service</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>--service</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s/Library/Logs/SPDeploy/spdeploy.log</string>
    <key>StandardErrorPath</key>
    <string>%s/Library/Logs/SPDeploy/spdeploy.log</string>
    <key>WorkingDirectory</key>
    <string>%s</string>
</dict>
</plist>
`, binaryPath, homeDir, homeDir, homeDir)

	plistDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	plistFile := filepath.Join(plistDir, "com.spdeploy.service.plist")
	if err := os.WriteFile(plistFile, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}

	// Load the service
	if err := exec.Command("launchctl", "load", plistFile).Run(); err != nil {
		return fmt.Errorf("failed to load launchd service: %w", err)
	}

	return nil
}

func createUserWindowsService(binaryPath string) error {
	// Windows Task Scheduler for user context
	taskName := "SPDeploy_User_Service"

	// Create scheduled task XML
	taskXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>SPDeploy Continuous Deployment Service (User)</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
    </LogonTrigger>
  </Triggers>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>%s</Command>
      <Arguments>--service</Arguments>
    </Exec>
  </Actions>
</Task>`, binaryPath)

	// Write task XML to temp file
	tempFile := filepath.Join(os.TempDir(), "spdeploy-task.xml")
	if err := os.WriteFile(tempFile, []byte(taskXML), 0644); err != nil {
		return fmt.Errorf("failed to write task XML: %w", err)
	}
	defer os.Remove(tempFile)

	// Delete existing task if it exists
	exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()

	// Create new task
	cmd := exec.Command("schtasks", "/create", "/tn", taskName, "/xml", tempFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create scheduled task: %w\nOutput: %s", err, output)
	}

	fmt.Println("✓ Windows scheduled task created")
	fmt.Printf("  Task name: %s\n", taskName)
	fmt.Println("  To start: schtasks /run /tn", taskName)
	fmt.Println("  To check: schtasks /query /tn", taskName)

	return nil
}

// Deprecated system-wide version
func createWindowsService(binaryPath string) error {
	// For Windows, we'll use sc.exe to create the service
	serviceName := "SPDeploy"
	serviceDisplayName := "SPDeploy Continuous Deployment Service"

	cmd := exec.Command("sc.exe", "create", serviceName,
		"binPath=", fmt.Sprintf("\"%s\" --service", binaryPath),
		"DisplayName=", serviceDisplayName,
		"start=", "auto")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create Windows service: %w", err)
	}

	// Set service description
	exec.Command("sc.exe", "description", serviceName,
		"Monitors GitHub repositories and automatically deploys changes").Run()

	return nil
}

func removeSystemService() error {
	switch runtime.GOOS {
	case "linux":
		// Stop and disable systemd service
		exec.Command("sudo", "systemctl", "stop", "spdeploy.service").Run()
		exec.Command("sudo", "systemctl", "disable", "spdeploy.service").Run()
		return exec.Command("sudo", "rm", "-f", "/etc/systemd/system/spdeploy.service").Run()
	case "darwin":
		// Unload and remove launchd service
		homeDir, _ := os.UserHomeDir()
		plistFile := filepath.Join(homeDir, "Library", "LaunchAgents", "com.spdeploy.service.plist")
		exec.Command("launchctl", "unload", plistFile).Run()
		return os.Remove(plistFile)
	case "windows":
		// Remove Windows service
		return exec.Command("sc.exe", "delete", "SPDeploy").Run()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}