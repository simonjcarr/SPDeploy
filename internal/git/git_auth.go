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
	if !strings.Contains(url, "@github.com") {
		return url
	}

	// Find the position of @ and remove everything between https:// and @
	parts := strings.SplitN(url, "@github.com/", 2)
	if len(parts) == 2 {
		return "https://github.com/" + parts[1]
	}

	return url
}