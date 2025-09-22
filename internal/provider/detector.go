package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"spdeploy/internal/logger"
)

// DetectionResult contains information about detected provider
type DetectionResult struct {
	Provider string
	Version  string
	APIURL   string
	Confidence float64 // 0.0 to 1.0
}

// Detector handles provider detection
type Detector struct {
	client *http.Client
	cache  map[string]*DetectionResult
}

// NewDetector creates a new provider detector
func NewDetector() *Detector {
	return &Detector{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: make(map[string]*DetectionResult),
	}
}

// DetectProvider attempts to detect the git provider for a given URL
func (d *Detector) DetectProvider(repoURL string) (*DetectionResult, error) {
	baseURL := extractBaseURL(repoURL)

	// Check cache first
	if result, ok := d.cache[baseURL]; ok {
		return result, nil
	}

	// Try known domains first
	if result := d.detectKnownDomain(repoURL); result != nil {
		d.cache[baseURL] = result
		return result, nil
	}

	// Try API detection
	if result := d.detectViaAPI(baseURL); result != nil {
		d.cache[baseURL] = result
		return result, nil
	}

	// Try HTML detection
	if result := d.detectViaHTML(baseURL); result != nil {
		d.cache[baseURL] = result
		return result, nil
	}

	// Try fingerprinting
	if result := d.detectViaFingerprint(baseURL); result != nil {
		d.cache[baseURL] = result
		return result, nil
	}

	return nil, fmt.Errorf("unable to detect provider for %s", repoURL)
}

// detectKnownDomain checks if the URL is from a known public provider
func (d *Detector) detectKnownDomain(repoURL string) *DetectionResult {
	knownDomains := map[string]string{
		"github.com":     "github",
		"gitlab.com":     "gitlab",
		"bitbucket.org":  "bitbucket",
		"codeberg.org":   "gitea",
		"gitea.com":      "gitea",
		"sr.ht":          "sourcehut",
	}

	domain := extractDomain(repoURL)
	if provider, ok := knownDomains[domain]; ok {
		apiURL := ""
		switch provider {
		case "github":
			apiURL = "https://api.github.com"
		case "gitlab":
			apiURL = "https://gitlab.com/api/v4"
		case "bitbucket":
			apiURL = "https://api.bitbucket.org/2.0"
		}

		return &DetectionResult{
			Provider:   provider,
			APIURL:     apiURL,
			Confidence: 1.0,
		}
	}

	return nil
}

// detectViaAPI attempts to detect provider by probing API endpoints
func (d *Detector) detectViaAPI(baseURL string) *DetectionResult {
	// Try GitLab API
	if result := d.tryGitLabAPI(baseURL); result != nil {
		return result
	}

	// Try Gitea API
	if result := d.tryGiteaAPI(baseURL); result != nil {
		return result
	}

	// Try Bitbucket API
	if result := d.tryBitbucketAPI(baseURL); result != nil {
		return result
	}

	// Try Gogs API
	if result := d.tryGogsAPI(baseURL); result != nil {
		return result
	}

	return nil
}

// tryGitLabAPI attempts to detect GitLab via API
func (d *Detector) tryGitLabAPI(baseURL string) *DetectionResult {
	apiURL := fmt.Sprintf("%s/api/v4/version", baseURL)

	resp, err := d.client.Get(apiURL)
	if err != nil {
		logger.Debug("GitLab API detection failed", zap.String("url", apiURL), zap.Error(err))
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var versionInfo struct {
			Version  string `json:"version"`
			Revision string `json:"revision"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err == nil {
			return &DetectionResult{
				Provider:   "gitlab",
				Version:    versionInfo.Version,
				APIURL:     fmt.Sprintf("%s/api/v4", baseURL),
				Confidence: 1.0,
			}
		}
	}

	// Try without /api/v4 (some GitLab instances have different paths)
	resp2, err := d.client.Get(fmt.Sprintf("%s/help", baseURL))
	if err == nil && resp2.StatusCode == 200 {
		defer resp2.Body.Close()
		body, _ := io.ReadAll(resp2.Body)
		if strings.Contains(string(body), "GitLab") {
			return &DetectionResult{
				Provider:   "gitlab",
				APIURL:     fmt.Sprintf("%s/api/v4", baseURL),
				Confidence: 0.8,
			}
		}
	}

	return nil
}

// tryGiteaAPI attempts to detect Gitea via API
func (d *Detector) tryGiteaAPI(baseURL string) *DetectionResult {
	apiURL := fmt.Sprintf("%s/api/v1/version", baseURL)

	resp, err := d.client.Get(apiURL)
	if err != nil {
		logger.Debug("Gitea API detection failed", zap.String("url", apiURL), zap.Error(err))
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var versionInfo struct {
			Version string `json:"version"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err == nil {
			return &DetectionResult{
				Provider:   "gitea",
				Version:    versionInfo.Version,
				APIURL:     fmt.Sprintf("%s/api/v1", baseURL),
				Confidence: 1.0,
			}
		}
	}

	return nil
}

// tryBitbucketAPI attempts to detect Bitbucket via API
func (d *Detector) tryBitbucketAPI(baseURL string) *DetectionResult {
	// For Bitbucket Server (self-hosted)
	apiURL := fmt.Sprintf("%s/rest/api/1.0/application-properties", baseURL)

	resp, err := d.client.Get(apiURL)
	if err != nil {
		logger.Debug("Bitbucket API detection failed", zap.String("url", apiURL), zap.Error(err))
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var appProps struct {
			Version     string `json:"version"`
			DisplayName string `json:"displayName"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&appProps); err == nil {
			if strings.Contains(strings.ToLower(appProps.DisplayName), "bitbucket") {
				return &DetectionResult{
					Provider:   "bitbucket",
					Version:    appProps.Version,
					APIURL:     fmt.Sprintf("%s/rest/api/1.0", baseURL),
					Confidence: 1.0,
				}
			}
		}
	}

	return nil
}

// tryGogsAPI attempts to detect Gogs via API
func (d *Detector) tryGogsAPI(baseURL string) *DetectionResult {
	// Gogs uses similar API to Gitea but different endpoints
	apiURL := fmt.Sprintf("%s/api/v1/repos/search", baseURL)

	resp, err := d.client.Get(apiURL)
	if err != nil {
		logger.Debug("Gogs API detection failed", zap.String("url", apiURL), zap.Error(err))
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		// Check headers for Gogs-specific markers
		if strings.Contains(resp.Header.Get("X-Content-Type-Options"), "gogs") {
			return &DetectionResult{
				Provider:   "gogs",
				APIURL:     fmt.Sprintf("%s/api/v1", baseURL),
				Confidence: 0.9,
			}
		}
	}

	return nil
}

// detectViaHTML attempts to detect provider by analyzing HTML content
func (d *Detector) detectViaHTML(baseURL string) *DetectionResult {
	resp, err := d.client.Get(baseURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // Read max 1MB
	if err != nil {
		return nil
	}

	htmlStr := string(body)

	// Check for GitLab markers
	if strings.Contains(htmlStr, "gitlab-logo") ||
		strings.Contains(htmlStr, "<meta content='GitLab") ||
		strings.Contains(htmlStr, "gon.gitlab_url") {
		return &DetectionResult{
			Provider:   "gitlab",
			APIURL:     fmt.Sprintf("%s/api/v4", baseURL),
			Confidence: 0.8,
		}
	}

	// Check for Gitea markers
	if strings.Contains(htmlStr, "Powered by Gitea") ||
		strings.Contains(htmlStr, "gitea-version") ||
		strings.Contains(htmlStr, "window.config.appName = \"Gitea\"") {
		return &DetectionResult{
			Provider:   "gitea",
			APIURL:     fmt.Sprintf("%s/api/v1", baseURL),
			Confidence: 0.8,
		}
	}

	// Check for Bitbucket markers
	if strings.Contains(htmlStr, "Bitbucket") ||
		strings.Contains(htmlStr, "bitbucket-logo") {
		return &DetectionResult{
			Provider:   "bitbucket",
			APIURL:     fmt.Sprintf("%s/rest/api/1.0", baseURL),
			Confidence: 0.7,
		}
	}

	// Check for Gogs markers
	if strings.Contains(htmlStr, "Powered by Gogs") ||
		strings.Contains(htmlStr, "gogs-version") {
		return &DetectionResult{
			Provider:   "gogs",
			APIURL:     fmt.Sprintf("%s/api/v1", baseURL),
			Confidence: 0.7,
		}
	}

	return nil
}

// detectViaFingerprint attempts to detect provider by checking known endpoints
func (d *Detector) detectViaFingerprint(baseURL string) *DetectionResult {
	fingerprints := map[string][]string{
		"gitlab": {
			"/api/v4/version",
			"/users/sign_in",
			"/-/profile",
		},
		"gitea": {
			"/api/v1/version",
			"/user/login",
			"/explore/repos",
		},
		"bitbucket": {
			"/rest/api/1.0/projects",
			"/login",
		},
		"gogs": {
			"/api/v1/users",
			"/user/login",
			"/explore/repos",
		},
	}

	for provider, endpoints := range fingerprints {
		matchCount := 0
		for _, endpoint := range endpoints {
			resp, err := d.client.Head(baseURL + endpoint)
			if err == nil && resp.StatusCode < 400 {
				matchCount++
				resp.Body.Close()
			}
		}

		// If at least half of the endpoints match, we have a probable match
		if float64(matchCount) >= float64(len(endpoints))/2 {
			apiURL := ""
			switch provider {
			case "gitlab":
				apiURL = fmt.Sprintf("%s/api/v4", baseURL)
			case "gitea", "gogs":
				apiURL = fmt.Sprintf("%s/api/v1", baseURL)
			case "bitbucket":
				apiURL = fmt.Sprintf("%s/rest/api/1.0", baseURL)
			}

			return &DetectionResult{
				Provider:   provider,
				APIURL:     apiURL,
				Confidence: float64(matchCount) / float64(len(endpoints)),
			}
		}
	}

	return nil
}

// extractBaseURL extracts the base URL from a repository URL
func extractBaseURL(repoURL string) string {
	// Handle SSH URLs
	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.Split(repoURL, ":")
		if len(parts) >= 1 {
			domain := strings.TrimPrefix(parts[0], "git@")
			return fmt.Sprintf("https://%s", domain)
		}
	}

	// Handle HTTPS URLs
	if strings.HasPrefix(repoURL, "https://") || strings.HasPrefix(repoURL, "http://") {
		// Remove path to get base URL
		parts := strings.Split(repoURL, "/")
		if len(parts) >= 3 {
			return strings.Join(parts[:3], "/")
		}
	}

	return repoURL
}