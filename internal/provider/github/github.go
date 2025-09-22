package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GitHubProvider implements the Provider interface for GitHub
type GitHubProvider struct {
	instanceName string
	apiURL       string
}

// NewGitHubProvider creates a new GitHub provider
func NewGitHubProvider() *GitHubProvider {
	return &GitHubProvider{
		instanceName: "github",
		apiURL:       "https://api.github.com",
	}
}

// NewGitHubEnterpriseProvider creates a provider for GitHub Enterprise
func NewGitHubEnterpriseProvider(baseURL, instanceName string) *GitHubProvider {
	return &GitHubProvider{
		instanceName: instanceName,
		apiURL:       fmt.Sprintf("%s/api/v3", baseURL),
	}
}

// Name returns the provider name
func (g *GitHubProvider) Name() string {
	return "github"
}

// Detect checks if a URL belongs to GitHub
func (g *GitHubProvider) Detect(repoURL string) bool {
	// Check for github.com
	if strings.Contains(repoURL, "github.com") {
		return true
	}

	// Check if it matches our GitHub Enterprise instance
	if g.instanceName != "github" && strings.Contains(repoURL, g.instanceName) {
		return true
	}

	return false
}

// ValidateToken validates a GitHub personal access token
func (g *GitHubProvider) ValidateToken(token string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/user", g.apiURL), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid token (status: %d)", resp.StatusCode)
	}

	// Optionally decode user info
	var user struct {
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err == nil {
		// Token is valid and we have user info
	}

	return nil
}

// GetAuthenticatedURL returns the URL with embedded authentication
func (g *GitHubProvider) GetAuthenticatedURL(repoURL, token string) string {
	if token == "" {
		return repoURL
	}

	// Handle SSH URLs - convert to HTTPS with token
	if strings.HasPrefix(repoURL, "git@github.com:") {
		path := strings.TrimPrefix(repoURL, "git@github.com:")
		return fmt.Sprintf("https://%s@github.com/%s", token, path)
	}

	// Handle HTTPS URLs - add token
	if strings.HasPrefix(repoURL, "https://github.com/") {
		return strings.Replace(repoURL, "https://", fmt.Sprintf("https://%s@", token), 1)
	}

	// For GitHub Enterprise
	if strings.Contains(repoURL, g.instanceName) && g.instanceName != "github" {
		if strings.HasPrefix(repoURL, "https://") {
			return strings.Replace(repoURL, "https://", fmt.Sprintf("https://%s@", token), 1)
		}
	}

	return repoURL
}

// GetAPIEndpoint returns the API endpoint for GitHub
func (g *GitHubProvider) GetAPIEndpoint() string {
	return g.apiURL
}

// GetTokenEnvVar returns the environment variable name for GitHub tokens
func (g *GitHubProvider) GetTokenEnvVar() string {
	if g.instanceName != "github" {
		return fmt.Sprintf("SPDEPLOY_%s_TOKEN", strings.ToUpper(g.instanceName))
	}
	return "SPDEPLOY_GITHUB_TOKEN"
}

// GetInstanceName returns the instance name
func (g *GitHubProvider) GetInstanceName() string {
	return g.instanceName
}

// GetSetupInstructions returns setup instructions for GitHub
func (g *GitHubProvider) GetSetupInstructions() string {
	if g.instanceName != "github" {
		return fmt.Sprintf(`GitHub Enterprise Setup:
1. Go to: %s/settings/tokens/new
2. Give your token a descriptive name
3. Select expiration (90 days recommended)
4. Select scopes:
   ✓ repo (Full control of repositories)
5. Click 'Generate token'
6. Set environment variable: export %s=<your-token>`,
			strings.TrimSuffix(g.apiURL, "/api/v3"),
			g.GetTokenEnvVar())
	}

	return `GitHub Setup:
1. Go to: https://github.com/settings/tokens/new
2. Give your token a descriptive name
3. Select expiration (90 days recommended)
4. Select scopes:
   ✓ repo (Full control of repositories)
5. Click 'Generate token'
6. Set environment variable: export SPDEPLOY_GITHUB_TOKEN=<your-token>`
}

// CheckAPIAccess verifies API access with optional token
func (g *GitHubProvider) CheckAPIAccess(token string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/rate_limit", g.apiURL), nil)
	if err != nil {
		return err
	}

	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API access check failed (status: %d)", resp.StatusCode)
	}

	return nil
}