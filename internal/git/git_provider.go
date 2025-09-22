package git

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"spdeploy/internal/logger"
	"spdeploy/internal/provider"
	"spdeploy/internal/provider/github"
	"spdeploy/internal/provider/gitlab"
	"spdeploy/internal/provider/generic"
)

// GitManagerWithProvider manages git operations using provider abstraction
type GitManagerWithProvider struct {
	registry      *provider.Registry
	tokenResolver *provider.TokenResolver
}

// NewGitManagerWithProvider creates a new provider-aware git manager
func NewGitManagerWithProvider() *GitManagerWithProvider {
	registry := provider.NewRegistry()

	// Register built-in providers
	registry.Register(github.NewGitHubProvider())
	registry.Register(gitlab.NewGitLabProvider())

	// TODO: Load configured instances from config

	return &GitManagerWithProvider{
		registry:      registry,
		tokenResolver: provider.NewTokenResolver(registry),
	}
}

// GetAuthenticatedURL returns an authenticated URL for the repository
func (gm *GitManagerWithProvider) GetAuthenticatedURL(repoURL, repoID string) (string, error) {
	// Detect provider
	p, err := gm.registry.DetectProvider(repoURL)
	if err != nil {
		logger.Warn("Unable to detect provider, using URL as-is",
			zap.String("url", repoURL),
			zap.Error(err))
		return repoURL, nil
	}

	// Resolve token from environment
	token, err := gm.tokenResolver.ResolveToken(repoURL, repoID)
	if err != nil {
		logger.Debug("No token found for repository",
			zap.String("url", repoURL),
			zap.String("provider", p.Name()),
			zap.Error(err))
		// Continue without token (might be public repo or using SSH)
		return repoURL, nil
	}

	// Get authenticated URL from provider
	authURL := p.GetAuthenticatedURL(repoURL, token)

	// Log success (without exposing token)
	logger.Info("Using authenticated URL for repository",
		zap.String("provider", p.Name()),
		zap.String("repo_id", repoID))

	return authURL, nil
}

// ValidateToken validates a token for a given repository
func (gm *GitManagerWithProvider) ValidateToken(repoURL, token string) error {
	p, err := gm.registry.DetectProvider(repoURL)
	if err != nil {
		return fmt.Errorf("unable to detect provider: %w", err)
	}

	return p.ValidateToken(token)
}

// GetProviderInstructions returns setup instructions for a repository
func (gm *GitManagerWithProvider) GetProviderInstructions(repoURL string) string {
	p, err := gm.registry.DetectProvider(repoURL)
	if err != nil {
		return "Unable to detect provider. Please configure authentication manually."
	}

	return p.GetSetupInstructions()
}

// ResolveToken attempts to find a token for the given repository
func (gm *GitManagerWithProvider) ResolveToken(repoURL, repoID string) (string, error) {
	return gm.tokenResolver.ResolveToken(repoURL, repoID)
}

// LoadProviderInstances loads configured provider instances from config
func (gm *GitManagerWithProvider) LoadProviderInstances(instances []ProviderInstance) {
	for _, inst := range instances {
		// Convert config instance to provider instance
		providerInst := provider.ProviderInstance{
			Name:    inst.Name,
			Type:    inst.Type,
			BaseURL: inst.BaseURL,
			APIURL:  inst.APIURL,
		}
		gm.registry.RegisterInstance(providerInst)

		// Also register the actual provider implementation if needed
		switch inst.Type {
		case "gitlab":
			gm.registry.Register(gitlab.NewGitLabSelfHostedProvider(inst.BaseURL, inst.Name))
		case "github":
			gm.registry.Register(github.NewGitHubEnterpriseProvider(inst.BaseURL, inst.Name))
		case "generic":
			gm.registry.Register(generic.NewGenericProvider(inst.BaseURL, inst.Name))
		}
	}
}

// ProviderInstance represents a provider instance from config
type ProviderInstance struct {
	Name    string
	Type    string
	BaseURL string
	APIURL  string
}

// StripAuthFromURL removes authentication tokens from URLs for logging
func StripAuthFromURL(url string) string {
	// Handle URLs with embedded tokens
	if strings.Contains(url, "@") {
		// Find the position of @ and remove everything between :// and @
		if idx := strings.Index(url, "://"); idx != -1 {
			protocol := url[:idx+3]
			remaining := url[idx+3:]
			if atIdx := strings.Index(remaining, "@"); atIdx != -1 {
				// Remove auth portion
				return protocol + remaining[atIdx+1:]
			}
		}
	}
	return url
}