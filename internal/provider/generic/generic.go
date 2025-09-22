package generic

import (
	"fmt"
	"strings"
)

// GenericProvider implements the Provider interface for generic git servers
type GenericProvider struct {
	instanceName string
	baseURL      string
}

// NewGenericProvider creates a new generic git provider
func NewGenericProvider(baseURL, instanceName string) *GenericProvider {
	if instanceName == "" {
		instanceName = "generic"
	}
	return &GenericProvider{
		instanceName: instanceName,
		baseURL:      baseURL,
	}
}

// Name returns the provider name
func (g *GenericProvider) Name() string {
	return "generic"
}

// Detect checks if a URL belongs to this provider
func (g *GenericProvider) Detect(repoURL string) bool {
	// Generic provider doesn't auto-detect, it's used as a fallback
	// or when explicitly configured
	if g.baseURL != "" && strings.Contains(repoURL, g.baseURL) {
		return true
	}
	return false
}

// ValidateToken for generic providers (no-op as we can't validate without knowing the API)
func (g *GenericProvider) ValidateToken(token string) error {
	// Generic git servers might not have an API to validate tokens
	// We'll assume the token is valid and let git operations fail if it's not
	if token == "" {
		return fmt.Errorf("token is empty")
	}
	return nil
}

// GetAuthenticatedURL returns the URL with embedded authentication
func (g *GenericProvider) GetAuthenticatedURL(repoURL, token string) string {
	if token == "" {
		return repoURL
	}

	// For generic providers, we'll use basic auth format
	// Handle SSH URLs - leave as is (requires SSH keys)
	if strings.HasPrefix(repoURL, "git@") {
		// SSH URLs should use SSH keys, not tokens
		return repoURL
	}

	// Handle HTTPS URLs - add token as username with 'x-token-auth' as password
	// This is a common pattern for git servers
	if strings.HasPrefix(repoURL, "https://") {
		// Try token as password with 'token' as username (common pattern)
		return strings.Replace(repoURL, "https://", fmt.Sprintf("https://token:%s@", token), 1)
	}

	if strings.HasPrefix(repoURL, "http://") {
		// Not recommended but support it
		return strings.Replace(repoURL, "http://", fmt.Sprintf("http://token:%s@", token), 1)
	}

	return repoURL
}

// GetAPIEndpoint returns empty for generic providers
func (g *GenericProvider) GetAPIEndpoint() string {
	// Generic providers might not have a standard API
	return ""
}

// GetTokenEnvVar returns the environment variable name for tokens
func (g *GenericProvider) GetTokenEnvVar() string {
	if g.instanceName != "" && g.instanceName != "generic" {
		// Use instance-specific env var
		sanitized := strings.ToUpper(strings.ReplaceAll(g.instanceName, "-", "_"))
		sanitized = strings.ReplaceAll(sanitized, ".", "_")
		return fmt.Sprintf("SPDEPLOY_%s_TOKEN", sanitized)
	}
	return "SPDEPLOY_GIT_TOKEN"
}

// GetInstanceName returns the instance name
func (g *GenericProvider) GetInstanceName() string {
	return g.instanceName
}

// GetSetupInstructions returns setup instructions for generic git servers
func (g *GenericProvider) GetSetupInstructions() string {
	envVar := g.GetTokenEnvVar()

	if g.instanceName != "" && g.instanceName != "generic" {
		return fmt.Sprintf(`Generic Git Server Setup (%s):

This appears to be a custom git server. Authentication methods vary by server.

Common authentication methods:
1. Personal Access Tokens
2. API Tokens
3. Username/Password
4. SSH Keys

For token-based authentication:
1. Generate a token in your git server's settings
2. Set environment variable: export %s=<your-token>

For SSH authentication:
1. Generate SSH key: ssh-keygen -t ed25519 -C "your-email@example.com"
2. Add public key to your git server
3. Use SSH URLs (git@%s:user/repo.git)

Note: Token format varies by server. The token will be used as the password
with 'token' as the username for HTTPS authentication.`,
			g.instanceName,
			envVar,
			g.baseURL)
	}

	return fmt.Sprintf(`Generic Git Server Setup:

For token-based authentication:
1. Generate a token in your git server's settings
2. Set environment variable: export %s=<your-token>

For SSH authentication:
1. Generate SSH key: ssh-keygen -t ed25519 -C "your-email@example.com"
2. Add public key to your git server
3. Use SSH URLs for your repositories

Note: Authentication methods vary by git server implementation.`,
		envVar)
}