package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"gocd/internal/logger"
)

const (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	defaultCallbackPort = 8765
)

type GitHubAuthenticator struct {
	clientID     string
	clientSecret string
	callbackPort int
	tokenFile    string
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type DeviceAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
}

func NewGitHubAuthenticator() *GitHubAuthenticator {
	configDir := getConfigDir()
	tokenFile := filepath.Join(configDir, ".github_token")

	return &GitHubAuthenticator{
		clientID:     getClientID(),
		clientSecret: getClientSecret(),
		callbackPort: defaultCallbackPort,
		tokenFile:    tokenFile,
	}
}

func getConfigDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "gocd")
	}

	// Check if running as root/sudo
	if os.Geteuid() == 0 {
		return "/etc/gocd"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ".gocd"
	}
	return filepath.Join(home, ".gocd")
}

func getClientID() string {
	// For production, this would be registered with GitHub
	// For now, we'll use device flow which doesn't require client secret
	return "gocd-cli"
}

func getClientSecret() string {
	// Not needed for device flow
	return ""
}

func (g *GitHubAuthenticator) AuthenticateWithDeviceFlow() (string, error) {
	logger.Info("Starting GitHub device flow authentication")

	// Request device code
	deviceCode, userCode, verificationURI, interval, err := g.requestDeviceCode()
	if err != nil {
		return "", fmt.Errorf("failed to request device code: %w", err)
	}

	// Display instructions to user
	fmt.Println("\n🔐 GitHub Authentication Required")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("1. Visit: %s\n", verificationURI)
	fmt.Printf("2. Enter code: %s\n", userCode)
	fmt.Println("3. Authorize 'gocd' to access your repositories")
	fmt.Println("\nWaiting for authorization...")

	// Open browser automatically
	g.openBrowser(verificationURI)

	// Poll for access token
	token, err := g.pollForAccessToken(deviceCode, interval)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Save token
	if err := g.saveToken(token); err != nil {
		return "", fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println("✅ Authentication successful! Token saved.")
	return token, nil
}

func (g *GitHubAuthenticator) requestDeviceCode() (string, string, string, int, error) {
	data := url.Values{}
	data.Set("client_id", g.clientID)
	data.Set("scope", "repo read:user")

	resp, err := http.Post(
		"https://github.com/login/device/code",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", "", "", 0, err
	}
	defer resp.Body.Close()

	var deviceResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return "", "", "", 0, err
	}

	return deviceResp.DeviceCode, deviceResp.UserCode, deviceResp.VerificationURI, deviceResp.Interval, nil
}

func (g *GitHubAuthenticator) pollForAccessToken(deviceCode string, interval int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("authentication timeout")
		case <-ticker.C:
			token, err := g.checkDeviceAuthorization(deviceCode)
			if err != nil {
				if strings.Contains(err.Error(), "authorization_pending") {
					continue
				}
				return "", err
			}
			if token != "" {
				return token, nil
			}
		}
	}
}

func (g *GitHubAuthenticator) checkDeviceAuthorization(deviceCode string) (string, error) {
	data := url.Values{}
	data.Set("client_id", g.clientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp DeviceAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("%s", tokenResp.Error)
	}

	return tokenResp.AccessToken, nil
}

func (g *GitHubAuthenticator) AuthenticateWithBrowser() (string, error) {
	logger.Info("Starting GitHub OAuth authentication")

	// Generate state for CSRF protection
	state := g.generateState()

	// Start local callback server
	tokenChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", g.callbackPort),
	}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		returnedState := r.URL.Query().Get("state")

		if returnedState != state {
			errorChan <- fmt.Errorf("invalid state parameter")
			fmt.Fprintf(w, "<h1>Authentication Failed</h1><p>Invalid state parameter</p>")
			return
		}

		if code == "" {
			errorChan <- fmt.Errorf("no authorization code received")
			fmt.Fprintf(w, "<h1>Authentication Failed</h1><p>No authorization code received</p>")
			return
		}

		// Exchange code for token
		token, err := g.exchangeCodeForToken(code)
		if err != nil {
			errorChan <- err
			fmt.Fprintf(w, "<h1>Authentication Failed</h1><p>%s</p>", err.Error())
			return
		}

		tokenChan <- token

		// Success response
		fmt.Fprintf(w, `
			<html>
			<head>
				<title>Authentication Successful</title>
				<style>
					body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
					h1 { color: #28a745; }
				</style>
			</head>
			<body>
				<h1>✅ Authentication Successful!</h1>
				<p>You can close this window and return to the terminal.</p>
				<script>window.setTimeout(function(){window.close();}, 2000);</script>
			</body>
			</html>
		`)

		// Shutdown server after response
		go func() {
			time.Sleep(1 * time.Second)
			server.Shutdown(context.Background())
		}()
	})

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errorChan <- err
		}
	}()

	// Build authorization URL
	authURL := g.buildAuthURL(state)

	// Open browser
	fmt.Printf("\n🔐 Opening browser for GitHub authentication...\n")
	fmt.Printf("If the browser doesn't open, visit:\n%s\n\n", authURL)

	if err := g.openBrowser(authURL); err != nil {
		logger.Warn("Failed to open browser", zap.Error(err))
	}

	// Wait for callback
	select {
	case token := <-tokenChan:
		// Save token
		if err := g.saveToken(token); err != nil {
			return "", fmt.Errorf("failed to save token: %w", err)
		}
		fmt.Println("✅ Authentication successful! Token saved.")
		return token, nil
	case err := <-errorChan:
		return "", err
	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
		return "", fmt.Errorf("authentication timeout")
	}
}

func (g *GitHubAuthenticator) buildAuthURL(state string) string {
	params := url.Values{}
	params.Add("client_id", g.clientID)
	params.Add("redirect_uri", fmt.Sprintf("http://localhost:%d/callback", g.callbackPort))
	params.Add("scope", "repo read:user")
	params.Add("state", state)

	return fmt.Sprintf("%s?%s", githubAuthorizeURL, params.Encode())
}

func (g *GitHubAuthenticator) exchangeCodeForToken(code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", g.clientID)
	data.Set("client_secret", g.clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", fmt.Sprintf("http://localhost:%d/callback", g.callbackPort))

	req, err := http.NewRequest("POST", githubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return tokenResp.AccessToken, nil
}

func (g *GitHubAuthenticator) generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (g *GitHubAuthenticator) openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}

func (g *GitHubAuthenticator) saveToken(token string) error {
	// Ensure config directory exists
	configDir := filepath.Dir(g.tokenFile)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save token with restricted permissions
	return os.WriteFile(g.tokenFile, []byte(token), 0600)
}

func (g *GitHubAuthenticator) GetStoredToken() (string, error) {
	data, err := os.ReadFile(g.tokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

func (g *GitHubAuthenticator) ClearToken() error {
	if err := os.Remove(g.tokenFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (g *GitHubAuthenticator) ValidateToken(token string) error {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid token (status: %d)", resp.StatusCode)
	}

	return nil
}