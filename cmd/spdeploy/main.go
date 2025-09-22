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
}

// addRepoCmd represents the add repository command
var addRepoCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a GitHub repository to monitor",
	Long: `Add a GitHub repository to monitor for changes.
When changes are detected, the repository will be automatically synced to the specified local path.`,
	Run: func(cmd *cobra.Command, args []string) {
		repoURL, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		localPath, _ := cmd.Flags().GetString("path")
		trigger, _ := cmd.Flags().GetString("trigger")
		withToken, _ := cmd.Flags().GetBool("with-token")

		if repoURL == "" || localPath == "" {
			fmt.Println("Error: Both --repo and --path are required")
			cmd.Help()
			return
		}

		// Validate trigger type
		if trigger != "push" && trigger != "pr" && trigger != "both" {
			fmt.Println("Error: --trigger must be one of: push, pr, both")
			return
		}

		// Expand the path to absolute
		absPath, err := filepath.Abs(localPath)
		if err != nil {
			fmt.Printf("Error resolving path: %v\n", err)
			return
		}

		// Create directory if it doesn't exist
		if err := os.MkdirAll(absPath, 0755); err != nil {
			fmt.Printf("Error creating directory: %v\n", err)
			return
		}

		cfg := config.NewConfig()
		token := ""

		if withToken {
			// Try to read token from PAT file
			patAuth := auth.NewPATAuthenticator()
			token, _ = patAuth.GetStoredToken()
		}

		if token != "" {
			if err := cfg.AddRepositoryWithToken(repoURL, branch, absPath, trigger, token); err != nil {
				fmt.Printf("Error adding repository: %v\n", err)
				return
			}
		} else {
			if err := cfg.AddRepository(repoURL, branch, absPath, trigger); err != nil {
				fmt.Printf("Error adding repository: %v\n", err)
				return
			}
		}

		fmt.Printf("✅ Repository added successfully!\n")
		fmt.Printf("   URL: %s\n", repoURL)
		fmt.Printf("   Branch: %s\n", branch)
		fmt.Printf("   Path: %s\n", absPath)
		fmt.Printf("   Trigger: %s\n", trigger)
		if withToken && token != "" {
			fmt.Printf("   Token: Using stored GitHub token\n")
		}
		fmt.Printf("\n📝 Remember to start the service with: spdeploy start\n")
	},
}

// listRepoCmd represents the list repositories command
var listRepoCmd = &cobra.Command{
	Use:   "list",
	Short: "List all monitored repositories",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.NewConfig()
		repos, err := cfg.ListRepositories()
		if err != nil {
			fmt.Printf("Error listing repositories: %v\n", err)
			return
		}

		if len(repos) == 0 {
			fmt.Println("No repositories are being monitored.")
			return
		}

		fmt.Printf("📦 Monitored Repositories (%d):\n", len(repos))
		fmt.Println(strings.Repeat("-", 80))

		for i, repo := range repos {
			fmt.Printf("%d. %s\n", i+1, repo.URL)
			fmt.Printf("   Branch: %s\n", repo.Branch)
			fmt.Printf("   Path: %s\n", repo.Path)
			fmt.Printf("   Trigger: %s\n", repo.Trigger)
			fmt.Printf("   Active: %v\n", repo.Active)
			if !repo.LastSync.IsZero() {
				fmt.Printf("   Last Sync: %s\n", repo.LastSync.Format("2006-01-02 15:04:05"))
			}
			fmt.Println()
		}
	},
}

// removeRepoCmd represents the remove repository command
var removeRepoCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a repository from monitoring",
	Run: func(cmd *cobra.Command, args []string) {
		repoURL, _ := cmd.Flags().GetString("repo")
		branch, _ := cmd.Flags().GetString("branch")
		path, _ := cmd.Flags().GetString("path")
		all, _ := cmd.Flags().GetBool("all")

		if repoURL == "" {
			fmt.Println("Error: --repo is required")
			cmd.Help()
			return
		}

		cfg := config.NewConfig()

		if all {
			count, err := cfg.RemoveAllRepository(repoURL, branch)
			if err != nil {
				fmt.Printf("Error removing repositories: %v\n", err)
				return
			}
			fmt.Printf("✅ Removed %d instances of %s\n", count, repoURL)
		} else {
			var err error
			if path != "" {
				err = cfg.RemoveRepositoryByPath(repoURL, branch, path)
			} else {
				err = cfg.RemoveRepository(repoURL, branch)
			}
			if err != nil {
				fmt.Printf("Error removing repository: %v\n", err)
				return
			}
			fmt.Printf("✅ Repository removed successfully\n")
		}
	},
}

// authRepoCmd represents the repository authentication command
var authRepoCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with GitHub for private repositories",
	Long: `Set up authentication for accessing private GitHub repositories.
For SSH URLs, shows instructions for setting up SSH keys.
For HTTPS URLs, helps set up a Personal Access Token.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get the repository URL from argument or flag
		var repoURL string
		if len(args) > 0 {
			repoURL = args[0]
		}

		logout, _ := cmd.Flags().GetBool("logout")

		if logout {
			// Clear stored token
			patAuth := auth.NewPATAuthenticator()
			if err := patAuth.ClearToken(); err != nil {
				fmt.Printf("Error clearing token: %v\n", err)
				return
			}
			fmt.Println("✅ GitHub token cleared successfully")
			return
		}

		if repoURL == "" {
			// No URL provided, do interactive PAT setup
			patAuth := auth.NewPATAuthenticator()
			_, err := patAuth.AuthenticateInteractive()
			if err != nil {
				fmt.Printf("Error during authentication: %v\n", err)
				return
			}
			return
		}

		// Check if it's an SSH URL
		if strings.HasPrefix(repoURL, "git@") || strings.Contains(repoURL, "git@") {
			showSSHInstructions()
		} else {
			// HTTPS URL - set up PAT
			patAuth := auth.NewPATAuthenticator()
			_, err := patAuth.AuthenticateInteractive()
			if err != nil {
				fmt.Printf("Error during authentication: %v\n", err)
				return
			}
		}
	},
}

func showSSHInstructions() {
	fmt.Println("\n🔐 SSH Key Setup Instructions")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("To use SSH URLs with GitHub, you need to set up SSH keys.")
	fmt.Println()
	fmt.Println("1. Check if you have an SSH key:")
	fmt.Println("   ls -la ~/.ssh")
	fmt.Println()
	fmt.Println("2. If no key exists, generate one:")
	fmt.Println("   ssh-keygen -t ed25519 -C \"your_email@example.com\"")
	fmt.Println()
	fmt.Println("3. Start the SSH agent:")
	if runtime.GOOS == "windows" {
		fmt.Println("   # In PowerShell (Admin):")
		fmt.Println("   Get-Service -Name ssh-agent | Set-Service -StartupType Manual")
		fmt.Println("   Start-Service ssh-agent")
	} else {
		fmt.Println("   eval \"$(ssh-agent -s)\"")
	}
	fmt.Println()
	fmt.Println("4. Add your SSH private key to the agent:")
	fmt.Println("   ssh-add ~/.ssh/id_ed25519")
	fmt.Println()
	fmt.Println("5. Copy your public key:")
	if runtime.GOOS == "darwin" {
		fmt.Println("   pbcopy < ~/.ssh/id_ed25519.pub")
	} else if runtime.GOOS == "linux" {
		fmt.Println("   cat ~/.ssh/id_ed25519.pub")
		fmt.Println("   # Then copy the output")
	} else {
		fmt.Println("   type %%USERPROFILE%%\\.ssh\\id_ed25519.pub")
		fmt.Println("   # Then copy the output")
	}
	fmt.Println()
	fmt.Println("6. Add the key to GitHub:")
	fmt.Println("   • Go to: https://github.com/settings/keys")
	fmt.Println("   • Click 'New SSH key'")
	fmt.Println("   • Paste your public key")
	fmt.Println("   • Click 'Add SSH key'")
	fmt.Println()
	fmt.Println("7. Test your connection:")
	fmt.Println("   ssh -T git@github.com")
	fmt.Println()
	fmt.Println("You should see: \"Hi username! You've successfully authenticated...\"")
	fmt.Println()
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the SPDeploy daemon",
	Run: func(cmd *cobra.Command, args []string) {
		if err := daemon.Start(); err != nil {
			fmt.Printf("Error starting daemon: %v\n", err)
			return
		}
		fmt.Println("✅ SPDeploy daemon started successfully")
	},
}

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the SPDeploy daemon",
	Run: func(cmd *cobra.Command, args []string) {
		if err := daemon.Stop(); err != nil {
			fmt.Printf("Error stopping daemon: %v\n", err)
			return
		}
		fmt.Println("✅ SPDeploy daemon stopped successfully")
	},
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of SPDeploy daemon",
	Run: func(cmd *cobra.Command, args []string) {
		if daemon.IsRunning() {
			fmt.Println("✅ SPDeploy daemon is running")

			// Show number of monitored repositories
			cfg := config.NewConfig()
			repos, err := cfg.ListRepositories()
			if err == nil {
				activeCount := 0
				for _, repo := range repos {
					if repo.Active {
						activeCount++
					}
				}
				fmt.Printf("📦 Monitoring %d repositories (%d active)\n", len(repos), activeCount)
			}
		} else {
			fmt.Println("❌ SPDeploy daemon is not running")
		}
	},
}

// logCmd represents the log viewing command
var logCmd = &cobra.Command{
	Use:   "log",
	Short: "View deployment logs",
	Long: `View deployment logs for repositories or the global service.
By default, shows logs for the current repository if you're in a git directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		follow, _ := cmd.Flags().GetBool("follow")
		global, _ := cmd.Flags().GetBool("global")
		repo, _ := cmd.Flags().GetString("repo")
		all, _ := cmd.Flags().GetBool("all")
		user, _ := cmd.Flags().GetString("user")

		logger.ShowContextualLogs(global, repo, all, user, follow)
	},
}

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install SPDeploy as a user service",
	Long: `Install SPDeploy as a user service that starts automatically.
This sets up platform-specific service configurations:
- Linux: systemd user service
- macOS: LaunchAgent
- Windows: Scheduled Task`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := installService(); err != nil {
			fmt.Printf("Error installing service: %v\n", err)
			return
		}
		fmt.Println("✅ SPDeploy installed successfully as a user service")
		fmt.Println("📝 The service will start automatically on login")
		fmt.Println("   You can also start it manually with: spdeploy start")
	},
}

// uninstallCmd represents the uninstall command
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall SPDeploy service",
	Long:  `Remove SPDeploy service configuration and optionally remove all data.`,
	Run: func(cmd *cobra.Command, args []string) {
		// First stop the daemon if running
		if daemon.IsRunning() {
			fmt.Println("Stopping SPDeploy daemon...")
			daemon.Stop()
		}

		if err := uninstallService(); err != nil {
			fmt.Printf("Error uninstalling service: %v\n", err)
			return
		}

		// Ask if user wants to remove configuration and logs
		fmt.Print("\n❓ Remove configuration and logs? (y/N): ")
		var response string
		fmt.Scanln(&response)

		if strings.ToLower(response) == "y" {
			removeData()
		}

		fmt.Println("\n✅ SPDeploy uninstalled successfully")
	},
}

func removeData() {
	fmt.Println("Removing configuration and logs...")

	cfg := config.NewConfig()
	configPath := cfg.GetConfigPath()

	// Remove config file
	os.Remove(configPath)

	// Remove config directory if empty
	configDir := filepath.Dir(configPath)
	os.Remove(configDir)

	// Remove log directories
	homeDir, _ := os.UserHomeDir()
	logDir := filepath.Join(homeDir, ".spdeploy")
	os.RemoveAll(logDir)

	fmt.Println("✅ Configuration and logs removed")
}

func installService() error {
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get the actual binary path
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	switch runtime.GOOS {
	case "darwin":
		return installMacOSService(execPath)
	case "linux":
		return installLinuxService(execPath)
	case "windows":
		return installWindowsService(execPath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func installMacOSService(execPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.spdeploy.agent.plist")

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.spdeploy.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>--service</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>%s/.spdeploy/logs/error.log</string>
    <key>StandardOutPath</key>
    <string>%s/.spdeploy/logs/output.log</string>
</dict>
</plist>`, execPath, homeDir, homeDir)

	// Create LaunchAgents directory if it doesn't exist
	launchAgentsDir := filepath.Dir(plistPath)
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return err
	}

	// Write the plist file
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return err
	}

	// Load the service
	cmd := exec.Command("launchctl", "load", plistPath)
	if err := cmd.Run(); err != nil {
		// Try to unload first in case it's already loaded
		exec.Command("launchctl", "unload", plistPath).Run()
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to load service: %w", err)
		}
	}

	return nil
}

func installLinuxService(execPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Create systemd user directory
	systemdDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(systemdDir, 0755); err != nil {
		return err
	}

	servicePath := filepath.Join(systemdDir, "spdeploy.service")

	serviceContent := fmt.Sprintf(`[Unit]
Description=SPDeploy Continuous Deployment Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s --service
Restart=always
RestartSec=10

[Install]
WantedBy=default.target`, execPath)

	// Write the service file
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return err
	}

	// Reload systemd user daemon
	exec.Command("systemctl", "--user", "daemon-reload").Run()

	// Enable the service
	if err := exec.Command("systemctl", "--user", "enable", "spdeploy.service").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	// Start the service
	if err := exec.Command("systemctl", "--user", "start", "spdeploy.service").Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func installWindowsService(execPath string) error {
	// Create a scheduled task that runs at logon
	taskName := "SPDeploy"

	// Delete existing task if it exists
	exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()

	// Create the task
	cmd := exec.Command("schtasks", "/create",
		"/tn", taskName,
		"/tr", fmt.Sprintf(`"%s" --service`, execPath),
		"/sc", "onlogon",
		"/rl", "highest",
		"/f")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create scheduled task: %w", err)
	}

	// Start the task
	if err := exec.Command("schtasks", "/run", "/tn", taskName).Run(); err != nil {
		return fmt.Errorf("failed to start scheduled task: %w", err)
	}

	return nil
}

func uninstallService() error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallMacOSService()
	case "linux":
		return uninstallLinuxService()
	case "windows":
		return uninstallWindowsService()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func uninstallMacOSService() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.spdeploy.agent.plist")

	// Unload the service
	exec.Command("launchctl", "unload", plistPath).Run()

	// Remove the plist file
	os.Remove(plistPath)

	return nil
}

func uninstallLinuxService() error {
	// Stop the service
	exec.Command("systemctl", "--user", "stop", "spdeploy.service").Run()

	// Disable the service
	exec.Command("systemctl", "--user", "disable", "spdeploy.service").Run()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	servicePath := filepath.Join(homeDir, ".config", "systemd", "user", "spdeploy.service")
	os.Remove(servicePath)

	// Reload systemd
	exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

func uninstallWindowsService() error {
	taskName := "SPDeploy"

	// Stop the task
	exec.Command("schtasks", "/end", "/tn", taskName).Run()

	// Delete the task
	if err := exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run(); err != nil {
		return fmt.Errorf("failed to delete scheduled task: %w", err)
	}

	return nil
}

func runAsService() {
	// Initialize logger first
	if err := logger.InitLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Starting SPDeploy service")

	// Get the GitHub token
	patAuth := auth.NewPATAuthenticator()
	githubToken, _ := patAuth.GetStoredToken()

	// Create monitor
	m := monitor.NewMonitor(githubToken)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the monitor
	if err := m.Start(); err != nil {
		logger.Error("Failed to start monitor", zap.Error(err))
		os.Exit(1)
	}

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Received shutdown signal")

	// Stop the monitor
	m.Stop()
}

func init() {
	// Root command flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is platform specific)")
	rootCmd.Flags().BoolVar(&serviceFlag, "service", false, "Run as service (internal use)")
	rootCmd.Flags().MarkHidden("service")

	// Repository commands
	rootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(addRepoCmd, listRepoCmd, removeRepoCmd, authRepoCmd)

	// Add repo flags
	addRepoCmd.Flags().String("repo", "", "GitHub repository URL (required)")
	addRepoCmd.Flags().String("branch", "main", "Branch to monitor")
	addRepoCmd.Flags().String("path", "", "Local path where repository should be synced (required)")
	addRepoCmd.Flags().String("trigger", "push", "Trigger type: push, pr, or both")
	addRepoCmd.Flags().Bool("with-token", false, "Use stored GitHub token for this repository")
	addRepoCmd.MarkFlagRequired("repo")
	addRepoCmd.MarkFlagRequired("path")

	// Remove repo flags
	removeRepoCmd.Flags().String("repo", "", "Repository URL to remove (required)")
	removeRepoCmd.Flags().String("branch", "main", "Specific branch to remove")
	removeRepoCmd.Flags().String("path", "", "Specific path to remove")
	removeRepoCmd.Flags().Bool("all", false, "Remove all instances of this repository")
	removeRepoCmd.MarkFlagRequired("repo")

	// Auth repo flags
	authRepoCmd.Flags().Bool("logout", false, "Clear stored GitHub token")

	// Service control commands
	rootCmd.AddCommand(startCmd, stopCmd, statusCmd)

	// Log command
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logCmd.Flags().BoolP("global", "g", false, "Show global service logs")
	logCmd.Flags().String("repo", "", "Show logs for specific repository")
	logCmd.Flags().BoolP("all", "a", false, "List all repositories being monitored")
	logCmd.Flags().String("user", "", "View logs for specific user (requires permissions)")

	// Installation commands
	rootCmd.AddCommand(installCmd, uninstallCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}