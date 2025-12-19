package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AuthState represents the current authentication state of the server.
type AuthState int

const (
	// AuthStateUnauthenticated means no token is available.
	AuthStateUnauthenticated AuthState = iota
	// AuthStatePending means device flow has been initiated, waiting for user.
	AuthStatePending
	// AuthStateAuthenticated means a valid token is available.
	AuthStateAuthenticated
)

// DeviceCodeResponse represents the response from GitHub's device code endpoint.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse represents the response from GitHub's token endpoint.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

// AuthManager manages authentication state for the MCP server.
// It handles the OAuth device flow and token storage.
type AuthManager struct {
	mu sync.RWMutex

	state        AuthState
	token        string
	deviceCode   *DeviceCodeResponse
	expiresAt    time.Time
	clientID     string
	clientSecret string
	scopes       []string

	// Host configuration for deriving OAuth endpoints
	host OAuthHost
}

// OAuthHost contains the OAuth endpoints for a GitHub host.
type OAuthHost struct {
	DeviceCodeURL string
	TokenURL      string
	Hostname      string
}

// NewOAuthHostFromAPIHost creates OAuth endpoints from the API host configuration.
func NewOAuthHostFromAPIHost(hostname string) OAuthHost {
	if hostname == "" || hostname == "github.com" || hostname == "https://github.com" || hostname == "https://api.github.com" {
		return OAuthHost{
			DeviceCodeURL: "https://github.com/login/device/code",
			TokenURL:      "https://github.com/login/oauth/access_token",
			Hostname:      "github.com",
		}
	}

	// If the hostname doesn't have a scheme, add https://
	if !strings.HasPrefix(hostname, "http://") && !strings.HasPrefix(hostname, "https://") {
		hostname = "https://" + hostname
	}

	// Parse the hostname to extract the base
	u, err := url.Parse(hostname)
	var host string
	if err != nil || u.Hostname() == "" {
		// Fallback: strip scheme if it was added, use original hostname
		host = strings.TrimPrefix(hostname, "https://")
		host = strings.TrimPrefix(host, "http://")
		return OAuthHost{
			DeviceCodeURL: fmt.Sprintf("https://%s/login/device/code", host),
			TokenURL:      fmt.Sprintf("https://%s/login/oauth/access_token", host),
			Hostname:      host,
		}
	}

	// For GHEC (ghe.com) and GHES, OAuth endpoints are on the main host
	host = u.Hostname()
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}

	return OAuthHost{
		DeviceCodeURL: fmt.Sprintf("%s://%s/login/device/code", scheme, host),
		TokenURL:      fmt.Sprintf("%s://%s/login/oauth/access_token", scheme, host),
		Hostname:      host,
	}
}

// DefaultOAuthClientID is the OAuth App client ID for the GitHub MCP Server.
// This OAuth App is registered and managed by GitHub for use with this server.
// The client ID is safe to embed in source code per OAuth 2.0 spec for public clients.
// Users can override this with --oauth-client-id for enterprise scenarios.
const DefaultOAuthClientID = "Ov23ctTMsnT9LTRdBYYM"

// DefaultOAuthScopes are the standard scopes needed for complete MCP functionality.
var DefaultOAuthScopes = []string{
	"gist",
	"notifications",
	"public_repo",
	"repo",
	"repo:status",
	"repo_deployment",
	"user",
	"user:email",
	"user:follow",
	"read:gpg_key",
	"read:org",
	"project",
}

// NewAuthManager creates a new AuthManager.
func NewAuthManager(host OAuthHost, clientID, clientSecret string, scopes []string) *AuthManager {
	if clientID == "" {
		clientID = DefaultOAuthClientID
	}
	if len(scopes) == 0 {
		scopes = DefaultOAuthScopes
	}

	return &AuthManager{
		state:        AuthStateUnauthenticated,
		host:         host,
		clientID:     clientID,
		clientSecret: clientSecret,
		scopes:       scopes,
	}
}

// NewAuthManagerWithToken creates an AuthManager that is already authenticated.
func NewAuthManagerWithToken(token string) *AuthManager {
	return &AuthManager{
		state: AuthStateAuthenticated,
		token: token,
	}
}

// State returns the current authentication state.
func (a *AuthManager) State() AuthState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// Token returns the current access token, or empty string if not authenticated.
func (a *AuthManager) Token() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.token
}

// IsAuthenticated returns true if a valid token is available.
func (a *AuthManager) IsAuthenticated() bool {
	return a.State() == AuthStateAuthenticated
}

// StartDeviceFlow initiates the OAuth device authorization flow.
// Returns the device code response containing the user code and verification URL.
func (a *AuthManager) StartDeviceFlow(ctx context.Context) (*DeviceCodeResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state == AuthStateAuthenticated {
		return nil, fmt.Errorf("already authenticated")
	}

	// Build the request
	data := url.Values{}
	data.Set("client_id", a.clientID)
	data.Set("scope", joinScopes(a.scopes))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.host.DeviceCodeURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read device code response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var deviceResp DeviceCodeResponse
	if err := json.Unmarshal(body, &deviceResp); err != nil {
		return nil, fmt.Errorf("failed to parse device code response: %w", err)
	}

	// Store the device code and update state
	a.deviceCode = &deviceResp
	a.expiresAt = time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)
	a.state = AuthStatePending

	return &deviceResp, nil
}

// CompleteDeviceFlow polls for the access token after the user has authorized.
// This should be called after StartDeviceFlow and after the user has entered the code.
func (a *AuthManager) CompleteDeviceFlow(ctx context.Context) error {
	return a.CompleteDeviceFlowWithProgress(ctx, nil)
}

// ProgressCallback is called during polling to report progress.
// elapsed is seconds since polling started, total is the expiry time in seconds.
type ProgressCallback func(elapsed, total int, message string)

// CompleteDeviceFlowWithProgress polls for the access token with progress updates.
// The onProgress callback is called periodically during polling.
func (a *AuthManager) CompleteDeviceFlowWithProgress(ctx context.Context, onProgress ProgressCallback) error {
	a.mu.Lock()
	deviceCode := a.deviceCode
	expiresAt := a.expiresAt
	a.mu.Unlock()

	if deviceCode == nil {
		return fmt.Errorf("no pending device flow - call StartDeviceFlow first")
	}

	if time.Now().After(expiresAt) {
		a.mu.Lock()
		a.state = AuthStateUnauthenticated
		a.deviceCode = nil
		a.mu.Unlock()
		return fmt.Errorf("device code expired - please start a new login flow")
	}

	// Poll for the token
	interval := time.Duration(deviceCode.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second // Minimum poll interval per RFC 8628
	}

	startTime := time.Now()
	totalSeconds := deviceCode.ExpiresIn

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Report progress before polling
			if onProgress != nil {
				elapsed := int(time.Since(startTime).Seconds())
				onProgress(elapsed, totalSeconds, "⏳ Waiting for authorization...")
			}

			token, err := a.pollForToken(ctx, deviceCode.DeviceCode)
			if err != nil {
				// Check for specific error types
				if err.Error() == "authorization_pending" {
					continue // Keep polling
				}
				if err.Error() == "slow_down" {
					// Increase interval by 5 seconds per RFC 8628
					interval += 5 * time.Second
					ticker.Reset(interval)
					continue
				}
				// Other errors are terminal
				a.mu.Lock()
				a.state = AuthStateUnauthenticated
				a.deviceCode = nil
				a.mu.Unlock()
				return err
			}

			// Success! Store the token
			a.mu.Lock()
			a.token = token
			a.state = AuthStateAuthenticated
			a.deviceCode = nil
			a.mu.Unlock()

			return nil
		}
	}
}

// pollForToken makes a single request to the token endpoint.
func (a *AuthManager) pollForToken(ctx context.Context, deviceCode string) (string, error) {
	data := url.Values{}
	data.Set("client_id", a.clientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	// Add client secret if provided (for confidential clients)
	if a.clientSecret != "" {
		data.Set("client_secret", a.clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.host.TokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	// Check for OAuth errors
	if tokenResp.Error != "" {
		switch tokenResp.Error {
		case "authorization_pending":
			return "", fmt.Errorf("authorization_pending")
		case "slow_down":
			return "", fmt.Errorf("slow_down")
		case "expired_token":
			return "", fmt.Errorf("device code expired - please start a new login flow")
		case "access_denied":
			return "", fmt.Errorf("authorization was denied by the user")
		default:
			return "", fmt.Errorf("OAuth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
		}
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return tokenResp.AccessToken, nil
}

// Reset clears any pending authentication state.
func (a *AuthManager) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state == AuthStatePending {
		a.state = AuthStateUnauthenticated
		a.deviceCode = nil
	}
}

// SetToken directly sets the authentication token (for testing or migration).
func (a *AuthManager) SetToken(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token = token
	a.state = AuthStateAuthenticated
	a.deviceCode = nil
}

func joinScopes(scopes []string) string {
	return strings.Join(scopes, " ")
}
