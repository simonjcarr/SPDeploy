package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TokenStore manages authentication tokens for multiple providers
type TokenStore struct {
	configDir string
	tokenFile string
}

// Tokens represents stored tokens for different providers
type Tokens struct {
	GitHub         string            `json:"github,omitempty"`
	GitLab         string            `json:"gitlab,omitempty"`
	Bitbucket      string            `json:"bitbucket,omitempty"`
	CustomInstances map[string]string `json:"custom_instances,omitempty"`
}

// NewTokenStore creates a new token store
func NewTokenStore() *TokenStore {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".spdeploy")

	return &TokenStore{
		configDir: configDir,
		tokenFile: filepath.Join(configDir, "tokens.json"),
	}
}

// GetToken retrieves a token for a specific provider
func (ts *TokenStore) GetToken(provider string) (string, error) {
	tokens, err := ts.loadTokens()
	if err != nil {
		return "", err
	}

	// Normalize provider name
	provider = strings.ToLower(provider)

	switch provider {
	case "github":
		return tokens.GitHub, nil
	case "gitlab":
		return tokens.GitLab, nil
	case "bitbucket":
		return tokens.Bitbucket, nil
	default:
		// Check custom instances
		if tokens.CustomInstances != nil {
			if token, exists := tokens.CustomInstances[provider]; exists {
				return token, nil
			}
		}
		return "", nil
	}
}

// SetToken stores a token for a specific provider
func (ts *TokenStore) SetToken(provider, token string) error {
	tokens, _ := ts.loadTokens() // Ignore error, start fresh if needed

	// Initialize custom instances map if needed
	if tokens.CustomInstances == nil {
		tokens.CustomInstances = make(map[string]string)
	}

	// Normalize provider name
	provider = strings.ToLower(provider)

	switch provider {
	case "github":
		tokens.GitHub = token
	case "gitlab":
		tokens.GitLab = token
	case "bitbucket":
		tokens.Bitbucket = token
	default:
		// Store as custom instance
		tokens.CustomInstances[provider] = token
	}

	return ts.saveTokens(tokens)
}

// RemoveToken removes a token for a specific provider
func (ts *TokenStore) RemoveToken(provider string) error {
	tokens, err := ts.loadTokens()
	if err != nil {
		return err
	}

	// Normalize provider name
	provider = strings.ToLower(provider)

	switch provider {
	case "github":
		tokens.GitHub = ""
	case "gitlab":
		tokens.GitLab = ""
	case "bitbucket":
		tokens.Bitbucket = ""
	default:
		// Remove from custom instances
		if tokens.CustomInstances != nil {
			delete(tokens.CustomInstances, provider)
		}
	}

	return ts.saveTokens(tokens)
}

// ListTokens returns a list of providers that have tokens stored
func (ts *TokenStore) ListTokens() ([]string, error) {
	tokens, err := ts.loadTokens()
	if err != nil {
		return nil, err
	}

	var providers []string

	if tokens.GitHub != "" {
		providers = append(providers, "github")
	}
	if tokens.GitLab != "" {
		providers = append(providers, "gitlab")
	}
	if tokens.Bitbucket != "" {
		providers = append(providers, "bitbucket")
	}

	for name := range tokens.CustomInstances {
		providers = append(providers, name)
	}

	return providers, nil
}

// loadTokens reads tokens from the JSON file
func (ts *TokenStore) loadTokens() (*Tokens, error) {
	// Ensure config directory exists
	if err := os.MkdirAll(ts.configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(ts.tokenFile); os.IsNotExist(err) {
		// Return empty tokens if file doesn't exist
		return &Tokens{}, nil
	}

	// Read file
	data, err := os.ReadFile(ts.tokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	// Parse JSON
	var tokens Tokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	return &tokens, nil
}

// saveTokens writes tokens to the JSON file
func (ts *TokenStore) saveTokens(tokens *Tokens) error {
	// Ensure config directory exists
	if err := os.MkdirAll(ts.configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	// Write file with restricted permissions
	if err := os.WriteFile(ts.tokenFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// MigrateFromLegacy migrates from old .github_token file to new format
func (ts *TokenStore) MigrateFromLegacy() error {
	legacyFile := filepath.Join(ts.configDir, ".github_token")

	// Check if legacy file exists
	if _, err := os.Stat(legacyFile); os.IsNotExist(err) {
		return nil // Nothing to migrate
	}

	// Read legacy token
	data, err := os.ReadFile(legacyFile)
	if err != nil {
		return fmt.Errorf("failed to read legacy token file: %w", err)
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return nil // Empty token, nothing to migrate
	}

	// Check if new format already has GitHub token
	tokens, _ := ts.loadTokens()
	if tokens.GitHub != "" {
		return nil // Already has GitHub token in new format
	}

	// Migrate token
	if err := ts.SetToken("github", token); err != nil {
		return fmt.Errorf("failed to migrate token: %w", err)
	}

	// Optionally remove legacy file (commented out for safety)
	// os.Remove(legacyFile)

	return nil
}