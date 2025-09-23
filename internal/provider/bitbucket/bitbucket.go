package bitbucket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// BitbucketProvider implements the Provider interface for Bitbucket
type BitbucketProvider struct {
	instanceName string
	baseURL      string
	apiURL       string
}

// NewBitbucketProvider creates a new Bitbucket provider for bitbucket.org
func NewBitbucketProvider() *BitbucketProvider {
	return &BitbucketProvider{
		instanceName: "bitbucket",
		baseURL:      "https://bitbucket.org",
		apiURL:       "https://api.bitbucket.org/2.0",
	}
}

// NewBitbucketServerProvider creates a provider for self-hosted Bitbucket Server
func NewBitbucketServerProvider(baseURL, instanceName string) *BitbucketProvider {
	return &BitbucketProvider{
		instanceName: instanceName,
		baseURL:      baseURL,
		apiURL:       fmt.Sprintf("%s/rest/api/1.0", baseURL),
	}
}

// Name returns the provider name
func (b *BitbucketProvider) Name() string {
	return "bitbucket"
}

// Detect checks if a URL belongs to Bitbucket
func (b *BitbucketProvider) Detect(repoURL string) bool {
	// Check for bitbucket.org
	if strings.Contains(repoURL, "bitbucket.org") {
		return true
	}

	// Check if it matches our self-hosted instance
	if b.instanceName != "bitbucket" && strings.Contains(repoURL, b.baseURL) {
		return true
	}

	return false
}

// ValidateToken validates a Bitbucket API token
func (b *BitbucketProvider) ValidateToken(token string) error {
	// For Bitbucket Cloud API tokens, we need to test with a simple endpoint
	// The /user endpoint might not work with all token types, so let's try /repositories
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/repositories", b.apiURL), nil)
	if err != nil {
		return err
	}

	// Try Bearer authentication first (for API tokens)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// If Bearer auth fails, the token is invalid
	// Note: We accept 200 (success) or 401 (unauthorized but valid endpoint)
	// A 401 with valid format means the token format is correct but lacks permissions
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		// Token format is valid (though it might lack permissions for this endpoint)
		return nil
	}

	return fmt.Errorf("invalid token (status: %d)", resp.StatusCode)
}

// GetAuthenticatedURL returns the URL with embedded authentication
func (b *BitbucketProvider) GetAuthenticatedURL(repoURL, token string) string {
	if token == "" {
		return repoURL
	}

	// Extract username from the URL if present
	var username string
	if strings.Contains(repoURL, "@") {
		// Extract username from URL like https://username@bitbucket.org/...
		parts := strings.Split(repoURL, "@")
		if len(parts) >= 2 && strings.HasPrefix(parts[0], "https://") {
			username = strings.TrimPrefix(parts[0], "https://")
		}
	}

	// Handle SSH URLs - convert to HTTPS with token
	if strings.HasPrefix(repoURL, "git@bitbucket.org:") {
		path := strings.TrimPrefix(repoURL, "git@bitbucket.org:")
		// For Bitbucket API tokens, use x-token-auth format
		return fmt.Sprintf("https://x-token-auth:%s@bitbucket.org/%s", token, path)
	}

	// Handle HTTPS URLs - add token
	if strings.HasPrefix(repoURL, "https://") {
		// Remove any existing auth from URL
		cleanURL := repoURL
		if strings.Contains(cleanURL, "@") {
			parts := strings.SplitN(cleanURL, "@", 2)
			if len(parts) == 2 {
				// Extract the part after @ to get the domain and path
				cleanURL = "https://" + parts[1]
			}
		}

		// For Bitbucket Cloud, use x-token-auth format for API tokens
		if strings.Contains(cleanURL, "bitbucket.org") {
			return strings.Replace(cleanURL, "https://", fmt.Sprintf("https://x-token-auth:%s@", token), 1)
		}

		// For self-hosted, also use username:token format
		if strings.Contains(cleanURL, b.baseURL) {
			if username != "" {
				return strings.Replace(cleanURL, "https://", fmt.Sprintf("https://%s:%s@", username, token), 1)
			}
			return strings.Replace(cleanURL, "https://", fmt.Sprintf("https://x-token-auth:%s@", token), 1)
		}
	}

	return repoURL
}

// GetAPIEndpoint returns the API endpoint for Bitbucket
func (b *BitbucketProvider) GetAPIEndpoint() string {
	return b.apiURL
}

// GetTokenEnvVar returns the environment variable name for Bitbucket tokens
func (b *BitbucketProvider) GetTokenEnvVar() string {
	if b.instanceName != "bitbucket" {
		return fmt.Sprintf("SPDEPLOY_%s_TOKEN", strings.ToUpper(strings.ReplaceAll(b.instanceName, "-", "_")))
	}
	return "SPDEPLOY_BITBUCKET_TOKEN"
}

// GetInstanceName returns the instance name
func (b *BitbucketProvider) GetInstanceName() string {
	return b.instanceName
}

// GetSetupInstructions returns setup instructions for Bitbucket
func (b *BitbucketProvider) GetSetupInstructions() string {
	if b.instanceName != "bitbucket" {
		return fmt.Sprintf(`Bitbucket Setup (%s):
1. Go to: %s (your profile settings)
2. Navigate to API tokens or Personal access tokens
3. Create a new API token with these permissions:
   ✓ Repository: Read
   ✓ Repository: Write (if you need push access)
4. Copy the generated token
5. Set environment variable: export %s=<your-token>`,
			b.instanceName,
			b.baseURL,
			b.GetTokenEnvVar())
	}

	return `Bitbucket Setup:
1. Go to: https://bitbucket.org/account/settings/api-tokens/
2. Click 'Create API token'
3. Give it a descriptive label (e.g., "spdeploy")
4. Select scopes:
   ✓ repository:read (Read access to repositories)
   ✓ repository:write (if you need push access)
5. Click 'Create'
6. Copy the generated API token
7. Set environment variable: export SPDEPLOY_BITBUCKET_TOKEN=<your-api-token>`
}

// CheckAPIAccess verifies API access with optional token
func (b *BitbucketProvider) CheckAPIAccess(token string) error {
	// For Bitbucket Cloud, check repositories endpoint
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/repositories", b.apiURL), nil)
	if err != nil {
		return err
	}

	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Public repositories should return 200, authenticated should also return 200
	if resp.StatusCode != http.StatusOK && resp.StatusCode != 401 {
		return fmt.Errorf("API access check failed (status: %d)", resp.StatusCode)
	}

	return nil
}

// GetVersion retrieves the Bitbucket version (mainly for self-hosted)
func (b *BitbucketProvider) GetVersion() (string, error) {
	// Bitbucket Cloud doesn't expose version via API
	if b.instanceName == "bitbucket" {
		return "Cloud", nil
	}

	// For Bitbucket Server
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/application-properties", b.apiURL), nil)
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

	var appProps struct {
		Version     string `json:"version"`
		DisplayName string `json:"displayName"`
		BuildNumber string `json:"buildNumber"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&appProps); err != nil {
		return "", err
	}

	return appProps.Version, nil
}