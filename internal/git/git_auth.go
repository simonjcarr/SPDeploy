package git

import (
	"fmt"
	"strings"
)

// ConvertSSHToHTTPS converts a GitHub SSH URL to HTTPS format with token authentication
func ConvertSSHToHTTPS(sshURL string, token string) string {
	if token == "" {
		return sshURL // Return original if no token
	}

	// Check if it's a GitHub SSH URL
	if !strings.HasPrefix(sshURL, "git@github.com:") {
		return sshURL // Return original if not GitHub SSH
	}

	// Convert git@github.com:owner/repo.git to https://token@github.com/owner/repo.git
	path := strings.TrimPrefix(sshURL, "git@github.com:")

	// Build HTTPS URL with token
	httpsURL := fmt.Sprintf("https://%s@github.com/%s", token, path)

	return httpsURL
}

// StripTokenFromURL removes the token from an HTTPS URL for logging
func StripTokenFromURL(url string) string {
	// Handle URLs with embedded tokens (GitHub, GitLab, etc.)
	if strings.Contains(url, "@") && strings.HasPrefix(url, "http") {
		// Find the last @ before the domain (to handle oauth2:token@domain format)
		idx := strings.LastIndex(url, "@")
		if idx > 0 {
			// Extract protocol
			protocol := "https://"
			if strings.HasPrefix(url, "http://") {
				protocol = "http://"
			}
			// Remove everything between protocol and @ (including the @)
			afterAt := url[idx+1:]
			return protocol + afterAt
		}
	}

	return url
}