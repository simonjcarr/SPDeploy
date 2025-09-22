package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"spdeploy/internal/config"
	"spdeploy/internal/provider"
	"spdeploy/internal/provider/github"
	"spdeploy/internal/provider/gitlab"
	"spdeploy/internal/provider/generic"
)

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Manage git provider configurations",
	Long:  `Configure and manage git providers for authentication and repository access`,
}

var providerAddCmd = &cobra.Command{
	Use:   "add [name] [type] [base-url]",
	Short: "Register a git provider instance",
	Long: `Register a git provider instance for self-hosted servers.

Examples:
  # Register a self-hosted GitLab
  spdeploy provider add gitlab-company gitlab https://gitlab.company.com

  # Register a GitHub Enterprise instance
  spdeploy provider add github-enterprise github https://github.company.com

  # Register a generic git server
  spdeploy provider add custom-git generic https://git.internal.net`,
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		providerType := args[1]
		baseURL := args[2]

		apiURL, _ := cmd.Flags().GetString("api-url")

		// Validate provider type
		validTypes := []string{"github", "gitlab", "bitbucket", "gitea", "gogs", "generic"}
		isValid := false
		for _, t := range validTypes {
			if providerType == t {
				isValid = true
				break
			}
		}

		if !isValid {
			fmt.Printf("Error: Invalid provider type '%s'\n", providerType)
			fmt.Printf("Valid types: %s\n", strings.Join(validTypes, ", "))
			os.Exit(1)
		}

		// Set default API URL if not provided
		if apiURL == "" {
			switch providerType {
			case "github":
				apiURL = fmt.Sprintf("%s/api/v3", baseURL)
			case "gitlab":
				apiURL = fmt.Sprintf("%s/api/v4", baseURL)
			case "bitbucket":
				apiURL = fmt.Sprintf("%s/rest/api/1.0", baseURL)
			case "gitea", "gogs":
				apiURL = fmt.Sprintf("%s/api/v1", baseURL)
			}
		}

		// Add provider instance to config
		cfg := config.NewConfig()
		instance := config.ProviderInstance{
			Name:    name,
			Type:    providerType,
			BaseURL: baseURL,
			APIURL:  apiURL,
		}

		if err := cfg.AddProviderInstance(instance); err != nil {
			fmt.Printf("Error adding provider: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Successfully registered %s provider: %s\n", providerType, name)
		fmt.Printf("   Base URL: %s\n", baseURL)
		if apiURL != "" {
			fmt.Printf("   API URL: %s\n", apiURL)
		}

		// Show token setup instructions
		fmt.Println("\n📝 Next steps:")
		fmt.Printf("1. Set authentication token: export SPDEPLOY_%s_TOKEN=<your-token>\n",
			strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
		fmt.Printf("2. Add repositories: spdeploy add %s/user/repo.git main /path/to/deploy\n", baseURL)
	},
}

var providerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured provider instances",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.NewConfig()
		instances, err := cfg.GetProviderInstances()
		if err != nil {
			fmt.Printf("Error loading providers: %v\n", err)
			os.Exit(1)
		}

		// Add built-in providers
		fmt.Println("Built-in Providers:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tURL\tTOKEN ENV")
		fmt.Fprintln(w, "github\tgithub\thttps://github.com\tSPDEPLOY_GITHUB_TOKEN")
		fmt.Fprintln(w, "gitlab\tgitlab\thttps://gitlab.com\tSPDEPLOY_GITLAB_TOKEN")
		fmt.Fprintln(w, "bitbucket\tbitbucket\thttps://bitbucket.org\tSPDEPLOY_BITBUCKET_TOKEN")
		w.Flush()

		if len(instances) > 0 {
			fmt.Println("\nConfigured Instances:")
			w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTYPE\tBASE URL\tAPI URL\tTOKEN ENV")
			for _, instance := range instances {
				tokenEnv := fmt.Sprintf("SPDEPLOY_%s_TOKEN",
					strings.ToUpper(strings.ReplaceAll(instance.Name, "-", "_")))
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					instance.Name, instance.Type, instance.BaseURL, instance.APIURL, tokenEnv)
			}
			w.Flush()
		}
	},
}

var providerDetectCmd = &cobra.Command{
	Use:   "detect [url]",
	Short: "Detect the provider type for a git URL",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repoURL := args[0]

		fmt.Printf("🔍 Detecting provider for: %s\n\n", repoURL)

		detector := provider.NewDetector()
		result, err := detector.DetectProvider(repoURL)
		if err != nil {
			fmt.Printf("❌ Unable to detect provider: %v\n", err)
			fmt.Println("\nYou can manually register this provider:")
			fmt.Println("  spdeploy provider add <name> <type> <base-url>")
			os.Exit(1)
		}

		fmt.Printf("✅ Detected: %s\n", result.Provider)
		if result.Version != "" {
			fmt.Printf("   Version: %s\n", result.Version)
		}
		if result.APIURL != "" {
			fmt.Printf("   API URL: %s\n", result.APIURL)
		}
		fmt.Printf("   Confidence: %.0f%%\n", result.Confidence*100)

		// Suggest next steps
		fmt.Println("\n📝 Next steps:")
		switch result.Provider {
		case "github":
			fmt.Println("1. Create a Personal Access Token:")
			fmt.Println("   https://github.com/settings/tokens/new")
			fmt.Println("2. Set environment variable:")
			fmt.Println("   export SPDEPLOY_GITHUB_TOKEN=<your-token>")
		case "gitlab":
			fmt.Println("1. Create a Personal Access Token:")
			fmt.Printf("   %s/-/profile/personal_access_tokens\n", strings.TrimSuffix(result.APIURL, "/api/v4"))
			fmt.Println("2. Set environment variable:")
			fmt.Println("   export SPDEPLOY_GITLAB_TOKEN=<your-token>")
		default:
			fmt.Printf("1. Generate an access token in your %s instance\n", result.Provider)
			fmt.Println("2. Set environment variable:")
			fmt.Printf("   export SPDEPLOY_%s_TOKEN=<your-token>\n", strings.ToUpper(result.Provider))
		}
	},
}

var providerTestCmd = &cobra.Command{
	Use:   "test [name]",
	Short: "Test provider connectivity and authentication",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		providerName := ""
		if len(args) > 0 {
			providerName = args[0]
		}

		token, _ := cmd.Flags().GetString("token")

		// If no token provided, try to get from environment
		if token == "" {
			if providerName != "" {
				envVar := fmt.Sprintf("SPDEPLOY_%s_TOKEN", strings.ToUpper(providerName))
				token = os.Getenv(envVar)
				if token == "" {
					fmt.Printf("⚠️  No token found in %s\n", envVar)
				}
			}
		}

		// Create provider based on name
		var p provider.Provider
		switch providerName {
		case "github", "":
			p = github.NewGitHubProvider()
			if token == "" {
				token = os.Getenv("SPDEPLOY_GITHUB_TOKEN")
			}
		case "gitlab":
			p = gitlab.NewGitLabProvider()
			if token == "" {
				token = os.Getenv("SPDEPLOY_GITLAB_TOKEN")
			}
		default:
			// Check if it's a configured instance
			cfg := config.NewConfig()
			instances, _ := cfg.GetProviderInstances()
			for _, inst := range instances {
				if inst.Name == providerName {
					switch inst.Type {
					case "github":
						p = github.NewGitHubEnterpriseProvider(inst.BaseURL, inst.Name)
					case "gitlab":
						p = gitlab.NewGitLabSelfHostedProvider(inst.BaseURL, inst.Name)
					case "generic":
						p = generic.NewGenericProvider(inst.BaseURL, inst.Name)
					}
					break
				}
			}
		}

		if p == nil {
			fmt.Printf("❌ Unknown provider: %s\n", providerName)
			os.Exit(1)
		}

		fmt.Printf("🔍 Testing %s provider...\n", p.Name())

		// Test token if provided
		if token != "" {
			fmt.Print("   Validating token... ")
			if err := p.ValidateToken(token); err != nil {
				fmt.Printf("❌ Invalid token: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✅")
		} else {
			fmt.Println("   ⚠️  No token provided - skipping authentication test")
		}

		fmt.Printf("\n✅ Provider %s is configured correctly\n", p.Name())
	},
}

var providerRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a configured provider instance",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		cfg := config.NewConfig()
		if err := cfg.RemoveProviderInstance(name); err != nil {
			fmt.Printf("Error removing provider: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Successfully removed provider instance: %s\n", name)
	},
}

func init() {
	// Add flags to provider add command
	providerAddCmd.Flags().String("api-url", "", "API URL for the provider (auto-detected if not specified)")

	// Add flags to provider test command
	providerTestCmd.Flags().String("token", "", "Token to test (uses environment variable if not specified)")

	// Add subcommands to provider command
	providerCmd.AddCommand(providerAddCmd)
	providerCmd.AddCommand(providerListCmd)
	providerCmd.AddCommand(providerDetectCmd)
	providerCmd.AddCommand(providerTestCmd)
	providerCmd.AddCommand(providerRemoveCmd)
}