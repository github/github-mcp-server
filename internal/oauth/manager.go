package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

// Manager handles OAuth authentication state with URL elicitation support
type Manager struct {
	config         Config
	mu             sync.RWMutex
	token          *Result
	authInProgress bool
	authDone       chan struct{} // closed when auth completes
}

// NewManager creates a new OAuth manager with the given configuration
func NewManager(cfg Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// HasToken returns true if a valid token is available
func (m *Manager) HasToken() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token != nil && m.token.AccessToken != ""
}

// GetAccessToken returns the access token if available
func (m *Manager) GetAccessToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.token == nil {
		return ""
	}
	return m.token.AccessToken
}

// RequestAuthentication triggers the OAuth flow using URL elicitation
// Uses session.Elicit() for synchronous blocking auth if session is provided
// Falls back to URLElicitationRequiredError if session is not available
// If auth is already in progress, waits for it to complete instead of starting a new flow
func (m *Manager) RequestAuthentication(ctx context.Context, session *mcp.ServerSession) error {
	// Check if auth is already in progress
	m.mu.Lock()
	if m.authInProgress {
		// Wait for the existing auth to complete
		authDone := m.authDone
		m.mu.Unlock()

		select {
		case <-authDone:
			// Auth completed, check if we have a token now
			if m.HasToken() {
				return nil
			}
			// Auth failed, but don't start a new one - let the next request retry
			return fmt.Errorf("authentication failed")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Mark auth as in progress
	m.authInProgress = true
	m.authDone = make(chan struct{})
	m.mu.Unlock()

	// Ensure we clean up the in-progress state when done
	defer func() {
		m.mu.Lock()
		m.authInProgress = false
		close(m.authDone)
		m.mu.Unlock()
	}()

	// Determine which flow to use based on environment
	useDeviceFlow := isRunningInDocker() && m.config.CallbackPort == 0

	if useDeviceFlow {
		return m.startDeviceFlowWithElicitation(ctx, session)
	}

	return m.startPKCEFlowWithElicitation(ctx, session)
}

// startDeviceFlowWithElicitation initiates device flow and uses session elicitation.
// Device flow is used when a callback server cannot be started (e.g., in Docker containers).
// It displays a code that the user must enter at the verification URL.
func (m *Manager) startDeviceFlowWithElicitation(ctx context.Context, session *mcp.ServerSession) error {
	oauth2Cfg := &oauth2.Config{
		ClientID:     m.config.ClientID,
		ClientSecret: m.config.ClientSecret,
		Scopes:       m.config.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:       m.config.AuthURL,
			TokenURL:      m.config.TokenURL,
			DeviceAuthURL: m.config.DeviceAuthURL,
		},
	}

	// Request device authorization
	deviceAuth, err := oauth2Cfg.DeviceAuth(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device authorization: %w", err)
	}

	// Create cancellable context for polling
	pollCtx, cancelPoll := context.WithCancel(ctx)
	defer cancelPoll()

	// Use session elicitation if available to show the user the verification URL and code
	if session != nil {
		// Run elicitation in goroutine - if cancelled, abort the device flow
		go func() {
			elicitID, err := generateRandomToken()
			if err != nil {
				// Non-critical: use fallback ID if generation fails
				elicitID = "fallback-id"
			}
			// Use pollCtx so elicitation is cancelled when polling completes or is cancelled
			result, err := session.Elicit(pollCtx, &mcp.ElicitParams{
				Mode:          "url",
				URL:           deviceAuth.VerificationURI,
				ElicitationID: elicitID,
				Message:       fmt.Sprintf("GitHub OAuth Device Authorization\n\nYour code: %s\n\nVisit the URL and enter this code to authenticate.", deviceAuth.UserCode),
			})
			// If elicitation was cancelled or declined, abort the polling
			if err != nil || result == nil || result.Action == "cancel" || result.Action == "decline" {
				cancelPoll()
			}
		}()
	}

	// Poll for the token (blocking, but respects context cancellation)
	token, err := oauth2Cfg.DeviceAccessToken(pollCtx, deviceAuth)
	if err != nil {
		if pollCtx.Err() != nil {
			return fmt.Errorf("OAuth authorization was cancelled by user")
		}
		return fmt.Errorf("failed to get device access token: %w", err)
	}

	// Store the token
	m.setToken(&Result{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	})

	return nil
}

// startPKCEFlowWithElicitation initiates PKCE flow with browser and session elicitation
// Uses session.Elicit() for synchronous blocking auth - the request waits until auth completes
func (m *Manager) startPKCEFlowWithElicitation(ctx context.Context, session *mcp.ServerSession) error {
	// Generate PKCE verifier
	verifier, err := generatePKCEVerifier()
	if err != nil {
		// Fall back to device flow if PKCE setup fails
		return m.startDeviceFlowWithElicitation(ctx, session)
	}

	// Generate state for CSRF protection
	state, err := generateRandomToken()
	if err != nil {
		return m.startDeviceFlowWithElicitation(ctx, session)
	}

	// Start local callback server
	listener, port, err := startLocalServer(m.config.CallbackPort)
	if err != nil {
		// Cannot start callback server - fall back to device flow
		return m.startDeviceFlowWithElicitation(ctx, session)
	}

	// Create OAuth2 config
	oauth2Cfg := &oauth2.Config{
		ClientID:     m.config.ClientID,
		ClientSecret: m.config.ClientSecret,
		RedirectURL:  fmt.Sprintf("http://localhost:%d/callback", port),
		Scopes:       m.config.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  m.config.AuthURL,
			TokenURL: m.config.TokenURL,
		},
	}

	// Build authorization URL with PKCE
	authURL := oauth2Cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))

	// Setup callback handling
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Create and start callback server
	server := createCallbackServer(state, codeChan, errChan, listener)

	// Cleanup function
	cleanup := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		_ = listener.Close() // Error intentionally ignored in cleanup
	}

	// Try to open browser - if it works, no elicitation needed
	browserErr := openBrowser(authURL)

	// Channel to signal elicitation cancellation
	elicitCancelChan := make(chan struct{}, 1)

	// Create cancellable context for elicitation
	elicitCtx, cancelElicit := context.WithCancel(ctx)
	defer cancelElicit()

	// Only elicit if browser failed to open (e.g., headless environment)
	// and we need to show the user the URL manually
	if browserErr != nil && session != nil {
		// Run elicitation in goroutine so we can monitor callback in parallel
		go func() {
			elicitID, _ := generateRandomToken() // Non-critical: empty ID is acceptable
			// Use elicitCtx so elicitation is cancelled when auth completes
			result, err := session.Elicit(elicitCtx, &mcp.ElicitParams{
				Mode:          "url",
				URL:           authURL,
				ElicitationID: elicitID,
				Message:       "GitHub OAuth Authorization\n\nPlease visit the URL to authorize access.",
			})
			// If elicitation was cancelled or declined, signal to abort
			if err != nil || result == nil || result.Action == "cancel" || result.Action == "decline" {
				select {
				case elicitCancelChan <- struct{}{}:
				default:
				}
			}
		}()
	}

	// Wait for callback with timeout
	select {
	case code := <-codeChan:
		// Exchange code for token
		token, err := oauth2Cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		cleanup()
		if err != nil {
			return fmt.Errorf("failed to exchange code for token: %w", err)
		}

		// Store token
		m.setToken(&Result{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			TokenType:    token.TokenType,
			Expiry:       token.Expiry,
		})

		return nil

	case err := <-errChan:
		cleanup()
		return fmt.Errorf("OAuth callback error: %w", err)

	case <-elicitCancelChan:
		cleanup()
		return fmt.Errorf("OAuth authorization was cancelled by user")

	case <-ctx.Done():
		cleanup()
		return ctx.Err()

	case <-time.After(DefaultAuthTimeout):
		cleanup()
		return fmt.Errorf("OAuth timeout after %v - please try again", DefaultAuthTimeout)
	}
}

// setToken stores the OAuth token
func (m *Manager) setToken(token *Result) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
}

// Helper functions

// generateRandomToken generates a cryptographically random URL-safe token.
// Used for CSRF state and elicitation IDs.
func generateRandomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
