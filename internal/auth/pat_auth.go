package auth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/term"
	"spdeploy/internal/logger"
)

type PATAuthenticator struct {
	tokenFile string
}

type GitHubUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func NewPATAuthenticator() *PATAuthenticator {
	configDir := getConfigDir()
	tokenFile := filepath.Join(configDir, ".github_token")

	return &PATAuthenticator{
		tokenFile: tokenFile,
	}
}

func (p *PATAuthenticator) AuthenticateInteractive() (string, error) {
	fmt.Println("\n🔐 GitHub Personal Access Token Setup")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("To access private repositories, you need a GitHub Personal Access Token.")
	fmt.Println()
	fmt.Println("📋 Follow these steps:")
	fmt.Println("1. Go to: https://github.com/settings/tokens/new")
	fmt.Println("2. Give your token a descriptive name (e.g., 'spdeploy-deployment')")
	fmt.Println("3. Select expiration (90 days recommended)")
	fmt.Println("4. Select scopes:")
	fmt.Println("   ✓ repo (Full control of private repositories)")
	fmt.Println("5. Click 'Generate token' at the bottom")
	fmt.Println("6. Copy the generated token")
	fmt.Println()

	// Try to open the browser
	if err := openBrowser("https://github.com/settings/tokens/new"); err != nil {
		logger.Debug("Could not open browser", zap.Error(err))
	} else {
		fmt.Println("🌐 Opening GitHub in your browser...")
		fmt.Println()
	}

	fmt.Print("Enter your Personal Access Token: ")

	// Read token securely (hidden input)
	var token string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", fmt.Errorf("failed to read token: %w", err)
		}
		token = strings.TrimSpace(string(tokenBytes))
		fmt.Println() // Add newline after hidden input
	} else {
		// Fallback for non-terminal environments
		reader := bufio.NewReader(os.Stdin)
		tokenInput, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read token: %w", err)
		}
		token = strings.TrimSpace(tokenInput)
	}

	if token == "" {
		return "", fmt.Errorf("token cannot be empty")
	}

	// Validate token
	fmt.Println("\n🔍 Validating token...")
	user, err := p.validateAndGetUser(token)
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	// Save token
	if err := p.saveToken(token); err != nil {
		return "", fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Printf("\n✅ Authentication successful!\n")
	fmt.Printf("👤 Authenticated as: %s", user.Login)
	if user.Name != "" {
		fmt.Printf(" (%s)", user.Name)
	}
	fmt.Println()
	fmt.Println("\n🎯 You can now add private repositories with 'spdeploy add')")

	return token, nil
}

func (p *PATAuthenticator) validateAndGetUser(token string) (*GitHubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (p *PATAuthenticator) saveToken(token string) error {
	// Ensure config directory exists
	configDir := filepath.Dir(p.tokenFile)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save token with restricted permissions
	return os.WriteFile(p.tokenFile, []byte(token), 0600)
}

func (p *PATAuthenticator) GetStoredToken() (string, error) {
	// Try primary location first
	data, err := os.ReadFile(p.tokenFile)
	if err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	// If running as root/sudo, also check user's home directory
	if os.Geteuid() == 0 {
		// Try to get the original user's home directory when using sudo
		sudoUser := os.Getenv("SUDO_USER")
		if sudoUser != "" {
			userTokenFile := fmt.Sprintf("/Users/%s/.spdeploy/.github_token", sudoUser)
			// Also try /home for Linux systems
			if _, err := os.Stat(userTokenFile); os.IsNotExist(err) {
				userTokenFile = fmt.Sprintf("/home/%s/.spdeploy/.github_token", sudoUser)
			}

			data, err := os.ReadFile(userTokenFile)
			if err == nil {
				return strings.TrimSpace(string(data)), nil
			}
		}
	}

	if os.IsNotExist(err) {
		return "", nil
	}
	return "", err
}

func (p *PATAuthenticator) ClearToken() error {
	if err := os.Remove(p.tokenFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (p *PATAuthenticator) ValidateToken(token string) error {
	_, err := p.validateAndGetUser(token)
	return err
}

func openBrowser(url string) error {
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