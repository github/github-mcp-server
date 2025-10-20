package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GitHubAppAuthProvider manages GitHub App authentication with automatic token refresh
type GitHubAppAuthProvider struct {
	appID          string
	privateKey     *rsa.PrivateKey
	installationID int64
	currentToken   *InstallationToken
	httpClient     *http.Client
}

// InstallationToken represents a GitHub App installation access token
type InstallationToken struct {
	Token     string
	ExpiresAt time.Time
}

// NewGitHubAppAuthProvider creates a new GitHub App auth provider
// It loads the private key, generates an initial token, and starts background refresh
func NewGitHubAppAuthProvider(appID string, privateKeyPath string, installationID int64) (*GitHubAppAuthProvider, error) {
	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	provider := &GitHubAppAuthProvider{
		appID:          appID,
		privateKey:     privateKey,
		installationID: installationID,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}

	// Generate initial token
	if err := provider.refreshToken(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to generate initial token: %w", err)
	}

	// Start background refresh goroutine
	go provider.tokenRefreshLoop()

	return provider, nil
}

// GetToken returns a valid installation token, refreshing if necessary
func (p *GitHubAppAuthProvider) GetToken() (string, error) {
	if p.currentToken == nil || p.isTokenExpiringSoon() {
		if err := p.refreshToken(context.Background()); err != nil {
			return "", err
		}
	}
	return p.currentToken.Token, nil
}

// generateJWT creates a signed JWT for GitHub App authentication
func (p *GitHubAppAuthProvider) generateJWT() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		Issuer:    p.appID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(p.privateKey)
}

// refreshToken exchanges JWT for installation access token
func (p *GitHubAppAuthProvider) refreshToken(ctx context.Context) error {
	jwtToken, err := p.generateJWT()
	if err != nil {
		return fmt.Errorf("failed to generate JWT: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", p.installationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "github-mcp-server")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to exchange JWT: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to get installation token: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	expiresAt, err := time.Parse(time.RFC3339, result.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to parse expiration time: %w", err)
	}

	p.currentToken = &InstallationToken{
		Token:     result.Token,
		ExpiresAt: expiresAt,
	}

	return nil
}

// isTokenExpiringSoon checks if token expires within 5 minutes
func (p *GitHubAppAuthProvider) isTokenExpiringSoon() bool {
	if p.currentToken == nil {
		return true
	}
	return time.Until(p.currentToken.ExpiresAt) < 5*time.Minute
}

// tokenRefreshLoop runs in background to refresh tokens before they expire
func (p *GitHubAppAuthProvider) tokenRefreshLoop() {
	for {
		if p.currentToken != nil {
			waitDuration := time.Until(p.currentToken.ExpiresAt) - 5*time.Minute
			if waitDuration > 0 {
				time.Sleep(waitDuration)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := p.refreshToken(ctx); err != nil {
			// Log error and retry in 1 minute
			fmt.Fprintf(os.Stderr, "[github-app-auth] Failed to refresh token: %v\n", err)
			cancel()
			time.Sleep(1 * time.Minute)
			continue
		}
		cancel()
	}
}

// loadPrivateKey reads and parses RSA private key from PEM file
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA key: %w", err)
	}

	return key, nil
}

// LoadAuthConfigFromEnv loads authentication configuration from environment variables
// Supports both Personal Access Token and GitHub App authentication
func LoadAuthConfigFromEnv() (authType string, token string, err error) {
	// Check for PAT first (backward compatibility)
	if pat := os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN"); pat != "" {
		return "pat", pat, nil
	}

	// Check for GitHub App
	appID := os.Getenv("GITHUB_APP_ID")
	if appID == "" {
		return "", "", fmt.Errorf("no authentication credentials provided (set GITHUB_PERSONAL_ACCESS_TOKEN or GITHUB_APP_* vars)")
	}

	keyPath := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
	if keyPath == "" {
		return "", "", fmt.Errorf("GITHUB_APP_PRIVATE_KEY_PATH required when using GitHub App")
	}

	installIDStr := os.Getenv("GITHUB_APP_INSTALLATION_ID")
	if installIDStr == "" {
		return "", "", fmt.Errorf("GITHUB_APP_INSTALLATION_ID required when using GitHub App")
	}

	installID, err := strconv.ParseInt(strings.TrimSpace(installIDStr), 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("invalid GITHUB_APP_INSTALLATION_ID: %w", err)
	}

	// Create GitHub App auth provider
	provider, err := NewGitHubAppAuthProvider(appID, keyPath, installID)
	if err != nil {
		return "", "", fmt.Errorf("failed to initialize GitHub App auth: %w", err)
	}

	token, err = provider.GetToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to get GitHub App token: %w", err)
	}

	return "github-app", token, nil
}
