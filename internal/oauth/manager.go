package oauth

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

// Manager handles OAuth authentication state and flow orchestration.
//
// Flow priority (security-ordered):
//  1. PKCE + browser auto-open (native binary — no elicitation needed)
//  2. PKCE + URL elicitation (Docker with bound port, or native when browser fails)
//  3. Device flow (fallback — more phishable, used only when PKCE is unavailable)
type Manager struct {
	config         Config
	logger         *slog.Logger
	mu             sync.RWMutex
	token          *Result
	authInProgress bool
	authDone       chan struct{}
}

// NewManager creates a new OAuth manager with the given configuration.
func NewManager(cfg Config, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return &Manager{
		config: cfg,
		logger: logger,
	}
}

// HasToken returns true if a valid token is available.
func (m *Manager) HasToken() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token != nil && m.token.AccessToken != ""
}

// GetAccessToken returns the current access token, or empty string if none.
func (m *Manager) GetAccessToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.token == nil {
		return ""
	}
	return m.token.AccessToken
}

// RequestAuthentication triggers the OAuth flow.
// If authentication is already in progress from another goroutine, this waits
// for it to complete rather than starting a duplicate flow.
func (m *Manager) RequestAuthentication(ctx context.Context, session *mcp.ServerSession) error {
	m.mu.Lock()
	if m.authInProgress {
		authDone := m.authDone
		m.mu.Unlock()

		select {
		case <-authDone:
			if m.HasToken() {
				return nil
			}
			return fmt.Errorf("authentication failed")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.authInProgress = true
	m.authDone = make(chan struct{})
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.authInProgress = false
		close(m.authDone)
		m.mu.Unlock()
	}()

	// Always attempt PKCE first — it's more secure than device flow.
	// Skip PKCE only when it cannot work: random port inside Docker
	// (random ports can't be mapped, and the browser can't auto-open).
	if m.config.CallbackPort == 0 && IsRunningInDocker() {
		m.logger.Info("Docker detected with no callback port configured, using device flow")
		return m.startDeviceFlow(ctx, session)
	}

	err := m.startPKCEFlow(ctx, session)
	if err == nil {
		return nil
	}

	m.logger.Info("PKCE flow unavailable, falling back to device flow", "reason", err)

	// Device flow fallback — used when PKCE callback server cannot start
	// (e.g., Docker without port binding). Device flow is more phishable
	// than PKCE, so it's only a fallback.
	return m.startDeviceFlow(ctx, session)
}

// startPKCEFlow runs the PKCE authorization code flow.
//
// Steps:
//  1. Start local callback server (127.0.0.1 only)
//  2. Try to open the auth URL in the user's browser
//  3. If browser fails, use URL elicitation to show the URL securely
//  4. Wait for the callback with the authorization code
//  5. Exchange the code for a token using the PKCE verifier
func (m *Manager) startPKCEFlow(ctx context.Context, session *mcp.ServerSession) error {
	verifier, err := generatePKCEVerifier()
	if err != nil {
		return fmt.Errorf("PKCE setup failed: %w", err)
	}

	state, err := generateRandomToken()
	if err != nil {
		return fmt.Errorf("state generation failed: %w", err)
	}

	listener, port, err := startLocalServer(m.config.CallbackPort)
	if err != nil {
		return fmt.Errorf("callback server failed: %w", err)
	}

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

	authURL := oauth2Cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)
	server := createCallbackServer(state, codeChan, errChan, listener)

	cleanup := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		_ = listener.Close()
	}

	// Try browser auto-open first (works on native, fails in Docker)
	browserErr := openBrowser(authURL)
	if browserErr != nil {
		m.logger.Debug("browser auto-open failed, trying URL elicitation", "error", browserErr)
	}

	// If browser didn't open, use URL elicitation to show the auth URL.
	// URL mode elicitation is secure: the MCP client shows the URL to the
	// user without exposing it to the LLM context.
	elicitCancelChan := make(chan struct{}, 1)
	elicitCtx, cancelElicit := context.WithCancel(ctx)
	defer cancelElicit()

	if browserErr != nil {
		if !m.tryURLElicitation(elicitCtx, session, authURL, elicitCancelChan) {
			// No browser, no URL elicitation — PKCE cannot proceed.
			// Caller will fall back to device flow.
			cleanup()
			return fmt.Errorf("no browser available and client does not support URL elicitation")
		}
	}

	select {
	case code := <-codeChan:
		cancelElicit()
		token, exchangeErr := oauth2Cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		cleanup()
		if exchangeErr != nil {
			return fmt.Errorf("failed to exchange code for token: %w", exchangeErr)
		}

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
		return fmt.Errorf("OAuth timeout after %v — please try again", DefaultAuthTimeout)
	}
}

// tryURLElicitation attempts to show the auth URL via MCP URL-mode elicitation.
// Returns true if elicitation was started, false if unavailable.
func (m *Manager) tryURLElicitation(ctx context.Context, session *mcp.ServerSession, authURL string, cancelChan chan<- struct{}) bool {
	if session == nil {
		return false
	}

	// Check if client supports URL elicitation
	params := session.InitializeParams()
	if params == nil || params.Capabilities == nil ||
		params.Capabilities.Elicitation == nil ||
		params.Capabilities.Elicitation.URL == nil {
		return false
	}

	go func() {
		elicitID, _ := generateRandomToken()
		result, err := session.Elicit(ctx, &mcp.ElicitParams{
			Mode:          "url",
			URL:           authURL,
			ElicitationID: elicitID,
			Message:       "Please visit the URL to authorize GitHub MCP Server.",
		})
		if err != nil || result == nil || result.Action == "cancel" || result.Action == "decline" {
			select {
			case cancelChan <- struct{}{}:
			default:
			}
		}
	}()

	return true
}

// startDeviceFlow runs the device authorization flow.
// This is the fallback when PKCE is unavailable (no port binding).
// Device flow is inherently more phishable than PKCE because the device
// code could be socially engineered — it should only be used as a fallback.
func (m *Manager) startDeviceFlow(ctx context.Context, session *mcp.ServerSession) error {
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

	deviceAuth, err := oauth2Cfg.DeviceAuth(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device authorization: %w", err)
	}

	pollCtx, cancelPoll := context.WithCancel(ctx)
	defer cancelPoll()

	m.showDeviceCode(pollCtx, session, deviceAuth, cancelPoll)

	token, err := oauth2Cfg.DeviceAccessToken(pollCtx, deviceAuth)
	if err != nil {
		if pollCtx.Err() != nil {
			return fmt.Errorf("OAuth authorization was cancelled by user")
		}
		return fmt.Errorf("failed to get device access token: %w", err)
	}

	m.setToken(&Result{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	})

	return nil
}

// showDeviceCode displays the device code to the user via the best available channel.
// Priority: URL elicitation → form elicitation → stderr.
func (m *Manager) showDeviceCode(ctx context.Context, session *mcp.ServerSession, deviceAuth *oauth2.DeviceAuthResponse, cancelPoll context.CancelFunc) {
	message := fmt.Sprintf("Visit %s and enter code: %s", deviceAuth.VerificationURI, deviceAuth.UserCode)

	if session == nil {
		m.logger.Info(message)
		fmt.Fprintf(os.Stderr, "\n%s\n\n", message)
		return
	}

	// Try URL elicitation first (most secure display)
	params := session.InitializeParams()
	supportsURL := params != nil && params.Capabilities != nil &&
		params.Capabilities.Elicitation != nil &&
		params.Capabilities.Elicitation.URL != nil

	if supportsURL {
		go func() {
			elicitID, _ := generateRandomToken()
			result, err := session.Elicit(ctx, &mcp.ElicitParams{
				Mode:          "url",
				URL:           deviceAuth.VerificationURI,
				ElicitationID: elicitID,
				Message:       fmt.Sprintf("Enter the code: %s", deviceAuth.UserCode),
			})
			if err != nil || result == nil || result.Action == "cancel" || result.Action == "decline" {
				cancelPoll()
			}
		}()
		return
	}

	// Try form elicitation — device codes are safe to display via form mode
	// (they are short-lived, require user action, and are designed for display)
	supportsForm := params != nil && params.Capabilities != nil &&
		params.Capabilities.Elicitation != nil

	if supportsForm {
		go func() {
			result, err := session.Elicit(ctx, &mcp.ElicitParams{
				Mode:    "form",
				Message: message,
				RequestedSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"acknowledged": map[string]any{
							"type":        "boolean",
							"title":       "I have entered the code",
							"description": message,
							"default":     false,
						},
					},
				},
			})
			if err != nil || result == nil || result.Action == "cancel" || result.Action == "decline" {
				cancelPoll()
			}
		}()
		return
	}

	// Last resort: stderr (no elicitation available)
	m.logger.Info(message)
	fmt.Fprintf(os.Stderr, "\n%s\n\n", message)
}

func (m *Manager) setToken(token *Result) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
}
