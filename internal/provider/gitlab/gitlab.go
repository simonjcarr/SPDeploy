package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GitLabProvider implements the Provider interface for GitLab
type GitLabProvider struct {
	instanceName string
	baseURL      string
	apiURL       string
}

// NewGitLabProvider creates a new GitLab provider for gitlab.com
func NewGitLabProvider() *GitLabProvider {
	return &GitLabProvider{
		instanceName: "gitlab",
		baseURL:      "https://gitlab.com",
		apiURL:       "https://gitlab.com/api/v4",
	}
}

// NewGitLabSelfHostedProvider creates a provider for self-hosted GitLab
func NewGitLabSelfHostedProvider(baseURL, instanceName string) *GitLabProvider {
	return &GitLabProvider{
		instanceName: instanceName,
		baseURL:      baseURL,
		apiURL:       fmt.Sprintf("%s/api/v4", baseURL),
	}
}

// Name returns the provider name
func (g *GitLabProvider) Name() string {
	return "gitlab"
}

// Detect checks if a URL belongs to GitLab
func (g *GitLabProvider) Detect(repoURL string) bool {
	// Check for gitlab.com
	if strings.Contains(repoURL, "gitlab.com") {
		return true
	}

	// Check if it matches our self-hosted instance
	if g.instanceName != "gitlab" && strings.Contains(repoURL, g.baseURL) {
		return true
	}

	return false
}

// ValidateToken validates a GitLab personal access token
func (g *GitLabProvider) ValidateToken(token string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/user", g.apiURL), nil)
	if err != nil {
		return err
	}

	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Also try with Bearer token format
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		req.Header.Del("PRIVATE-TOKEN")

		resp2, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			return fmt.Errorf("invalid token (status: %d)", resp.StatusCode)
		}
	}

	// Optionally decode user info
	var user struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err == nil {
		// Token is valid and we have user info
	}

	return nil
}

// GetAuthenticatedURL returns the URL with embedded authentication
func (g *GitLabProvider) GetAuthenticatedURL(repoURL, token string) string {
	if token == "" {
		return repoURL
	}

	// Handle SSH URLs - convert to HTTPS with token
	if strings.HasPrefix(repoURL, "git@gitlab.com:") {
		path := strings.TrimPrefix(repoURL, "git@gitlab.com:")
		return fmt.Sprintf("https://oauth2:%s@gitlab.com/%s", token, path)
	}

	// Handle self-hosted SSH URLs
	if strings.HasPrefix(repoURL, "git@") && strings.Contains(repoURL, g.baseURL) {
		// Extract domain and path
		parts := strings.Split(repoURL, ":")
		if len(parts) == 2 {
			domain := strings.TrimPrefix(parts[0], "git@")
			path := parts[1]
			return fmt.Sprintf("https://oauth2:%s@%s/%s", token, domain, path)
		}
	}

	// Handle HTTPS URLs - add token
	if strings.HasPrefix(repoURL, "https://") {
		// GitLab prefers oauth2 as the username for token auth
		if strings.Contains(repoURL, "gitlab.com") {
			return strings.Replace(repoURL, "https://", fmt.Sprintf("https://oauth2:%s@", token), 1)
		}
		// For self-hosted, also use oauth2
		if strings.Contains(repoURL, g.baseURL) {
			return strings.Replace(repoURL, "https://", fmt.Sprintf("https://oauth2:%s@", token), 1)
		}
	}

	return repoURL
}

// GetAPIEndpoint returns the API endpoint for GitLab
func (g *GitLabProvider) GetAPIEndpoint() string {
	return g.apiURL
}

// GetTokenEnvVar returns the environment variable name for GitLab tokens
func (g *GitLabProvider) GetTokenEnvVar() string {
	if g.instanceName != "gitlab" {
		return fmt.Sprintf("SPDEPLOY_%s_TOKEN", strings.ToUpper(strings.ReplaceAll(g.instanceName, "-", "_")))
	}
	return "SPDEPLOY_GITLAB_TOKEN"
}

// GetInstanceName returns the instance name
func (g *GitLabProvider) GetInstanceName() string {
	return g.instanceName
}

// GetSetupInstructions returns setup instructions for GitLab
func (g *GitLabProvider) GetSetupInstructions() string {
	if g.instanceName != "gitlab" {
		return fmt.Sprintf(`GitLab Setup (%s):
1. Go to: %s/-/profile/personal_access_tokens
2. Give your token a descriptive name
3. Select expiration date (1 year recommended)
4. Select scopes:
   ✓ read_api
   ✓ read_repository
   ✓ write_repository (if you need push access)
5. Click 'Create personal access token'
6. Set environment variable: export %s=<your-token>`,
			g.instanceName,
			g.baseURL,
			g.GetTokenEnvVar())
	}

	return `GitLab Setup:
1. Go to: https://gitlab.com/-/profile/personal_access_tokens
2. Give your token a descriptive name
3. Select expiration date (1 year recommended)
4. Select scopes:
   ✓ read_api
   ✓ read_repository
   ✓ write_repository (if you need push access)
5. Click 'Create personal access token'
6. Set environment variable: export SPDEPLOY_GITLAB_TOKEN=<your-token>`
}

// CheckAPIAccess verifies API access with optional token
func (g *GitLabProvider) CheckAPIAccess(token string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/version", g.apiURL), nil)
	if err != nil {
		return err
	}

	if token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Version endpoint might require auth on some instances
	if resp.StatusCode == 401 && token == "" {
		// Try a public endpoint
		req, err = http.NewRequest("GET", fmt.Sprintf("%s/projects", g.apiURL), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")

		resp, err = client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 401 {
		return fmt.Errorf("API access check failed (status: %d)", resp.StatusCode)
	}

	return nil
}

// GetVersion retrieves the GitLab version
func (g *GitLabProvider) GetVersion() (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/version", g.apiURL), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get version (status: %d)", resp.StatusCode)
	}

	var versionInfo struct {
		Version  string `json:"version"`
		Revision string `json:"revision"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
		return "", err
	}

	return versionInfo.Version, nil
}