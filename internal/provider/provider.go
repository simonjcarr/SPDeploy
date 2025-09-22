package provider

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Provider represents a git hosting provider
type Provider interface {
	// Name returns the provider name (e.g., "github", "gitlab")
	Name() string

	// Detect checks if a URL belongs to this provider
	Detect(repoURL string) bool

	// ValidateToken validates an authentication token
	ValidateToken(token string) error

	// GetAuthenticatedURL returns the URL with embedded authentication
	GetAuthenticatedURL(repoURL, token string) string

	// GetAPIEndpoint returns the API endpoint for this provider
	GetAPIEndpoint() string

	// GetTokenEnvVar returns the environment variable name for this provider's token
	GetTokenEnvVar() string

	// GetInstanceName returns the instance name (for self-hosted instances)
	GetInstanceName() string

	// GetSetupInstructions returns instructions for setting up authentication
	GetSetupInstructions() string
}

// Registry manages all registered providers
type Registry struct {
	providers []Provider
	instances map[string]ProviderInstance
}

// ProviderInstance represents a configured instance of a provider
type ProviderInstance struct {
	Name     string
	Type     string // github, gitlab, bitbucket, etc.
	BaseURL  string
	APIURL   string
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: []Provider{},
		instances: make(map[string]ProviderInstance),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(p Provider) {
	r.providers = append(r.providers, p)
}

// RegisterInstance adds a configured instance (e.g., self-hosted GitLab)
func (r *Registry) RegisterInstance(instance ProviderInstance) {
	domain := extractDomain(instance.BaseURL)
	r.instances[domain] = instance
}

// DetectProvider detects which provider a repository URL belongs to
func (r *Registry) DetectProvider(repoURL string) (Provider, error) {
	// First check if it's a known instance
	domain := extractDomain(repoURL)
	if instance, ok := r.instances[domain]; ok {
		// Find the provider implementation for this instance type
		for _, p := range r.providers {
			if p.Name() == instance.Type {
				return p, nil
			}
		}
	}

	// Then check each provider's detection logic
	for _, p := range r.providers {
		if p.Detect(repoURL) {
			return p, nil
		}
	}

	// If no provider detected, try to auto-detect
	detectedType, err := autoDetectProvider(repoURL)
	if err == nil && detectedType != "" {
		// Register this instance for future use
		instance := ProviderInstance{
			Name:    domain,
			Type:    detectedType,
			BaseURL: fmt.Sprintf("https://%s", domain),
		}
		r.RegisterInstance(instance)

		// Return the appropriate provider
		for _, p := range r.providers {
			if p.Name() == detectedType {
				return p, nil
			}
		}
	}

	return nil, fmt.Errorf("unable to detect provider for URL: %s", repoURL)
}

// GetProvider returns a provider by name
func (r *Registry) GetProvider(name string) (Provider, error) {
	for _, p := range r.providers {
		if p.Name() == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("provider not found: %s", name)
}

// ListProviders returns all registered providers
func (r *Registry) ListProviders() []Provider {
	return r.providers
}

// ListInstances returns all registered instances
func (r *Registry) ListInstances() map[string]ProviderInstance {
	return r.instances
}

// Helper functions

func extractDomain(repoURL string) string {
	// Handle both HTTPS and SSH URLs
	if strings.HasPrefix(repoURL, "git@") {
		// SSH URL: git@domain.com:user/repo.git
		parts := strings.Split(repoURL, ":")
		if len(parts) >= 1 {
			domain := strings.TrimPrefix(parts[0], "git@")
			return domain
		}
	}

	// Try to parse as URL
	u, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}

	return u.Host
}

// autoDetectProvider attempts to detect the provider type by probing the server
func autoDetectProvider(repoURL string) (string, error) {
	// This will be implemented with the actual detection logic
	// For now, return a placeholder
	return "", fmt.Errorf("auto-detection not yet implemented")
}

// TokenResolver handles token resolution from environment variables
type TokenResolver struct {
	registry *Registry
}

// NewTokenResolver creates a new token resolver
func NewTokenResolver(registry *Registry) *TokenResolver {
	return &TokenResolver{
		registry: registry,
	}
}

// ResolveToken resolves the token for a repository
func (tr *TokenResolver) ResolveToken(repoURL string, repoID string) (string, error) {
	provider, err := tr.registry.DetectProvider(repoURL)
	if err != nil {
		return "", err
	}

	// Try different token sources in priority order
	tokens := []string{
		getEnvToken(fmt.Sprintf("SPDEPLOY_REPO_%s_TOKEN", strings.ToUpper(repoID))),
		getEnvToken(fmt.Sprintf("SPDEPLOY_%s_TOKEN", strings.ToUpper(provider.GetInstanceName()))),
		getEnvToken(provider.GetTokenEnvVar()),
		getEnvToken(fmt.Sprintf("SPDEPLOY_%s_TOKEN", strings.ToUpper(provider.Name()))),
	}

	for _, token := range tokens {
		if token != "" {
			return token, nil
		}
	}

	return "", fmt.Errorf("no token found for %s repository", provider.Name())
}

// getEnvToken retrieves a token from environment variable
func getEnvToken(envVar string) string {
	return strings.TrimSpace(os.Getenv(envVar))
}