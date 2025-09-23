package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"spdeploy/internal"
)

var (
	Version   = "v3.0.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "spdeploy",
	Short: "Simple SSH-based git repository deployment monitor",
	Long: `SPDeploy monitors git repositories via SSH and automatically pulls changes.
It uses SSH keys for authentication and runs as a simple process.`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildDate),
}

var addCmd = &cobra.Command{
	Use:   "add <ssh-url> <path>",
	Short: "Add a repository to monitor",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sshURL := args[0]
		localPath := args[1]
		branch, _ := cmd.Flags().GetString("branch")

		// Validate SSH URL
		if !strings.HasPrefix(sshURL, "git@") {
			fmt.Fprintf(os.Stderr, "Error: Only SSH URLs are supported (must start with git@)\n")
			os.Exit(1)
		}

		cfg := internal.LoadConfig()

		// Check if repository already exists
		for _, repo := range cfg.Repositories {
			if repo.URL == sshURL && repo.Path == localPath {
				fmt.Fprintf(os.Stderr, "Error: Repository already exists\n")
				os.Exit(1)
			}
		}

		// Add repository
		repo := internal.Repository{
			URL:    sshURL,
			Branch: branch,
			Path:   localPath,
		}

		// Validate repository can be accessed
		if err := internal.ValidateRepository(repo); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to validate repository: %v\n", err)
			os.Exit(1)
		}

		cfg.Repositories = append(cfg.Repositories, repo)

		if err := internal.SaveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to save config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Added repository: %s → %s (branch: %s)\n", sshURL, localPath, branch)
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <ssh-url>",
	Short: "Remove a repository from monitoring",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sshURL := args[0]

		cfg := internal.LoadConfig()

		found := false
		newRepos := []internal.Repository{}
		for _, repo := range cfg.Repositories {
			if repo.URL == sshURL {
				found = true
				continue
			}
			newRepos = append(newRepos, repo)
		}

		if !found {
			fmt.Fprintf(os.Stderr, "Error: Repository not found\n")
			os.Exit(1)
		}

		cfg.Repositories = newRepos

		if err := internal.SaveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to save config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Removed repository: %s\n", sshURL)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all monitored repositories",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := internal.LoadConfig()

		if len(cfg.Repositories) == 0 {
			fmt.Println("No repositories configured")
			return
		}

		fmt.Printf("Monitored repositories (check every %d seconds):\n", cfg.CheckInterval)
		for i, repo := range cfg.Repositories {
			fmt.Printf("%d. %s\n", i+1, repo.URL)
			fmt.Printf("   Branch: %s\n", repo.Branch)
			fmt.Printf("   Path: %s\n", repo.Path)
			if repo.PostPullScript != "" {
				fmt.Printf("   Script: %s\n", repo.PostPullScript)
			}
		}
	},
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start monitoring repositories",
	Run: func(cmd *cobra.Command, args []string) {
		daemon, _ := cmd.Flags().GetBool("daemon")

		if daemon {
			// Start in background using exec.Command
			exe, err := os.Executable()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to get executable: %v\n", err)
				os.Exit(1)
			}

			bgCmd := exec.Command(exe, "run")
			bgCmd.Stdout = nil
			bgCmd.Stderr = nil
			bgCmd.Stdin = nil

			if err := bgCmd.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to start daemon: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✓ Started SPDeploy daemon (PID: %d)\n", bgCmd.Process.Pid)
			fmt.Printf("  Check logs at: ~/.spdeploy/logs/spdeploy.log\n")
			fmt.Printf("  To stop: kill %d\n", bgCmd.Process.Pid)
			os.Exit(0)
		}

		// Run monitor
		cfg := internal.LoadConfig()
		if len(cfg.Repositories) == 0 {
			fmt.Fprintf(os.Stderr, "Error: No repositories configured\n")
			os.Exit(1)
		}

		monitor := internal.NewMonitor(cfg)

		// Setup signal handlers
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			fmt.Println("\n✓ Shutting down...")
			os.Exit(0)
		}()

		fmt.Printf("Starting monitor for %d repositories (interval: %d seconds)\n",
			len(cfg.Repositories), cfg.CheckInterval)

		monitor.Run()
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the monitoring daemon",
	Run: func(cmd *cobra.Command, args []string) {
		// For now, users need to manually kill the process
		fmt.Println("To stop SPDeploy, find and kill the process:")
		fmt.Println("  ps aux | grep spdeploy")
		fmt.Println("  kill <pid>")
	},
}

func init() {
	addCmd.Flags().String("branch", "main", "Branch to monitor")
	addCmd.Flags().String("script", "", "Post-pull script to execute")

	runCmd.Flags().BoolP("daemon", "d", false, "Run in background")

	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(stopCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}