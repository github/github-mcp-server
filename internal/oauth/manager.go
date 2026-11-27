package oauth

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// DefaultAuthTimeout bounds how long a single authorization attempt waits for
// the user to complete the browser or device flow.
const DefaultAuthTimeout = 5 * time.Minute

// tokenRefreshTimeout bounds HTTP calls made by the shared token source.
const tokenRefreshTimeout = 30 * time.Second

// ErrStaleAuthorizationFlow indicates that a prompt response belongs to an
// authorization flow that is no longer current.
var ErrStaleAuthorizationFlow = errors.New("authorization prompt has expired")

// flowStatus tracks the manager's single-flight authorization state.
type flowStatus int

const (
	statusIdle         flowStatus = iota // no flow running
	statusStarting                       // a flow is being prepared (brief)
	statusInProgress                     // a flow is running on a secure channel; callers may join
	statusAwaitingUser                   // a flow is running but the user must act out-of-band
)

// Outcome reports the result of an authorization attempt that did not
// immediately yield a token.
type Outcome struct {
	// UserAction, when non-nil, must be surfaced to the user. The authorization
	// flow continues in the background; the user should retry once they have
	// completed it.
	UserAction *UserAction

	// FlowID correlates a user action with the authorization flow that produced
	// it. Callers must pass it back to AwaitToken or Cancel so a delayed response
	// cannot affect a newer flow.
	FlowID string
}

// UserAction is an instruction for the user to complete authorization out of
// band (the last-resort channel, used when neither a browser nor URL
// elicitation is available).
type UserAction struct {
	// Message is ready to display to the user.
	Message string
	// URL is the authorization URL or device verification URI.
	URL string
	// UserCode is the device-flow code to enter, if any.
	UserCode string
}

type tokenRefresh struct {
	done        chan struct{}
	accessToken string
}

// Manager owns the OAuth login flows and the resulting (refreshing) token for a
// single stdio session. It is safe for concurrent use; only one authorization
// flow runs at a time.
type Manager struct {
	config        Config
	refreshConfig *oauth2.Config
	logger        *slog.Logger

	// Test seams, set by NewManager to real implementations.
	openURL  func(string) error
	inDocker func() bool

	mu               sync.Mutex
	source           oauth2.TokenSource // refreshing source, set once authorized
	token            *oauth2.Token
	refresh          *tokenRefresh
	tokenGeneration  uint64 // increments whenever source is replaced
	status           flowStatus
	flowID           string
	pending          *UserAction
	done             chan struct{}
	cancelFlow       context.CancelFunc // cancels the in-flight flow, if any
	lastErr          error
	refreshErrLogged bool // true once a refresh failure has been logged, reset on re-auth
}

// NewManager builds a Manager for the given configuration. A nil logger logs to
// stderr.
func NewManager(cfg Config, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	m := &Manager{
		config:   cfg,
		logger:   logger,
		openURL:  openBrowser,
		inDocker: isRunningInDocker,
	}
	m.refreshConfig = m.oauth2Config("")
	return m
}

// AccessToken returns a currently valid access token, refreshing it if needed,
// or "" if the session is not authorized (or a refresh has failed and
// re-authorization is required). It is cheap to call repeatedly: the underlying
// token source caches and only refreshes when the token has expired.
func (m *Manager) AccessToken() string {
	token, _ := m.AccessTokenContext(context.Background())
	return token
}

// AccessTokenContext allows callers to stop waiting for a shared token refresh.
func (m *Manager) AccessTokenContext(ctx context.Context) (string, error) {
	token, _, err := m.accessToken(ctx)
	return token, err
}

// accessToken returns the token together with the generation of the source it
// checked. Authenticate uses the generation to detect a source installed while
// token validation was in progress, without repeating a potentially blocking
// refresh request.
func (m *Manager) accessToken(ctx context.Context) (string, uint64, error) {
	for {
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		m.mu.Lock()
		generation := m.tokenGeneration
		if m.source == nil {
			m.mu.Unlock()
			return "", generation, nil
		}
		if m.token.Valid() {
			token := m.token.AccessToken
			m.mu.Unlock()
			return token, generation, nil
		}
		if m.refresh == nil {
			m.refresh = &tokenRefresh{done: make(chan struct{})}
			go m.refreshToken(m.source, generation, m.refresh)
		}
		refresh := m.refresh
		m.mu.Unlock()

		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-refresh.done:
		}
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		m.mu.Lock()
		current := m.tokenGeneration == generation
		m.mu.Unlock()
		if current {
			return refresh.accessToken, generation, nil
		}
	}
}

// refreshToken runs the token source outside the manager lock. Its bounded
// exchange owns refresh-token rotation even if every waiting caller cancels.
func (m *Manager) refreshToken(src oauth2.TokenSource, generation uint64, refresh *tokenRefresh) {
	tok, err := src.Token()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tokenGeneration != generation {
		return
	}
	m.refresh = nil
	defer close(refresh.done)
	if err != nil {
		// A refresh failure (expired GitHub App refresh token, revoked grant, or a
		// network blip) leaves the session unauthorized and forces a re-login.
		// Surface it once, otherwise it only manifests as a surprise re-authorization
		// prompt. The oauth2 error carries the token endpoint's response, not the
		// access or refresh token.
		if !m.refreshErrLogged {
			m.refreshErrLogged = true
			m.logger.Warn("OAuth token refresh failed; re-authorization required", "error", err)
		}
		return
	}
	m.token = tok
	if tok.Valid() {
		refresh.accessToken = tok.AccessToken
	}
}

// HasToken reports whether a valid token is currently available.
func (m *Manager) HasToken() bool {
	return m.AccessToken() != ""
}

// HasTokenContext checks for an active session while respecting caller cancellation.
func (m *Manager) HasTokenContext(ctx context.Context) (bool, error) {
	token, err := m.AccessTokenContext(ctx)
	return token != "", err
}

// Authenticate ensures the session is authorized.
//
// It returns (nil, nil) once a token is available, so the caller may proceed.
// It returns (&Outcome{UserAction}, nil) when the user must complete the flow
// out of band; the flow continues in the background and the caller should show
// the action and have the user retry. It returns (nil, err) on failure.
//
// Only one flow runs at a time. Concurrent callers either join a running secure
// flow, receive the pending user action, or are told to retry shortly.
func (m *Manager) Authenticate(ctx context.Context, prompter Prompter) (*Outcome, error) {
	var flowID string
	var done chan struct{}
	for {
		token, checkedTokenGeneration, err := m.accessToken(ctx)
		if err != nil {
			return nil, err
		}
		if token != "" {
			return nil, nil
		}

		m.mu.Lock()
		switch m.status {
		case statusAwaitingUser:
			ua := m.pending
			flowID := m.flowID
			m.mu.Unlock()
			return &Outcome{UserAction: ua, FlowID: flowID}, nil
		case statusStarting:
			flowID := m.flowID
			m.mu.Unlock()
			return &Outcome{UserAction: &UserAction{
				Message: "GitHub authorization is already in progress. Please retry your request in a few seconds.",
			}, FlowID: flowID}, nil
		case statusInProgress:
			done := m.done
			flowID := m.flowID
			m.mu.Unlock()
			return m.joinWait(ctx, done, flowID)
		}

		// A flow may have installed a token while the source above was being
		// checked. Retry if the source changed before claiming the idle state.
		if m.tokenGeneration != checkedTokenGeneration {
			m.mu.Unlock()
			continue
		}

		// Idle: this call owns the new flow.
		m.status = statusStarting
		m.flowID = rand.Text()
		flowID = m.flowID
		m.lastErr = nil
		m.done = make(chan struct{})
		done = m.done
		m.mu.Unlock()
		break
	}

	plan, err := m.begin(prompter)
	if err != nil {
		m.complete(flowID, nil, err)
		return nil, err
	}

	bgCtx, cancel := context.WithTimeout(context.Background(), DefaultAuthTimeout)
	m.mu.Lock()
	if m.flowID != flowID {
		m.mu.Unlock()
		cancel()
		return nil, ErrStaleAuthorizationFlow
	}
	if plan.userAction != nil {
		m.status = statusAwaitingUser
		m.pending = plan.userAction
	} else {
		m.status = statusInProgress
	}
	m.cancelFlow = cancel
	m.mu.Unlock()
	go m.runFlow(bgCtx, cancel, flowID, plan)

	if plan.userAction != nil {
		return &Outcome{UserAction: plan.userAction, FlowID: flowID}, nil
	}
	return m.joinWait(ctx, done, flowID)
}

// AwaitToken blocks until the in-flight authorization flow yields a token, the
// flow ends without one, or ctx is done. It is the resume half of the
// multi-round-trip flow: a transport that presented the authorization prompt
// itself (via elicitation returned from a tool call) calls this once the user
// has acted, to wait for the background token acquisition to finish.
//
// It returns (nil, nil) once a token is available (proceed), (&Outcome{UserAction},
// nil) when the user must still act out of band, or (nil, err) on failure.
func (m *Manager) AwaitToken(ctx context.Context, flowID string) (*Outcome, error) {
	m.mu.Lock()
	if flowID == "" || flowID != m.flowID {
		m.mu.Unlock()
		return nil, ErrStaleAuthorizationFlow
	}
	done := m.done
	m.mu.Unlock()
	token, err := m.AccessTokenContext(ctx)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	current := m.flowID == flowID
	m.mu.Unlock()
	if !current {
		return nil, ErrStaleAuthorizationFlow
	}
	if token != "" {
		return nil, nil
	}
	if done == nil {
		// No flow is in flight; report whatever terminal state it left behind.
		return m.outcomeAfterFlow(ctx, flowID)
	}
	select {
	case <-done:
		return m.outcomeAfterFlow(ctx, flowID)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Cancel retires the matching authorization flow and aborts its background
// callback listener or device poll. It returns false if flowID is stale.
func (m *Manager) Cancel(flowID string) bool {
	m.mu.Lock()
	if flowID == "" || flowID != m.flowID {
		m.mu.Unlock()
		return false
	}
	cancel := m.cancelFlow
	m.status = statusIdle
	m.flowID = ""
	m.pending = nil
	m.cancelFlow = nil
	m.lastErr = context.Canceled
	if m.done != nil {
		close(m.done)
		m.done = nil
	}
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return true
}

// runFlow executes a prepared flow in the background and records the result. The
// optional display prompt runs concurrently: a decline (or other failure) aborts
// the flow, while an undeliverable prompt degrades to the manual fallback without
// tearing the flow down, so the user can still authorize out of band.
func (m *Manager) runFlow(ctx context.Context, cancel context.CancelFunc, flowID string, plan *flowPlan) {
	defer cancel()

	if plan.display != nil {
		go func() {
			err := plan.display(ctx)
			switch {
			case err == nil:
				// Prompt shown; the flow completes when the token arrives.
			case ctx.Err() != nil:
				// The flow is already ending (timed out or cancelled elsewhere),
				// so there is nothing to fall back to. Checking this before the
				// fallback also prevents misreading a context-cancelled prompt as
				// a transport failure.
			case errors.Is(err, ErrPromptUnavailable) && plan.fallback != nil:
				// The client advertised the capability but could not deliver the
				// prompt. Surface the manual instructions instead of failing, and
				// keep the background flow alive so the user can still authorize.
				m.logger.Debug("authorization prompt undeliverable; falling back to manual instructions", "reason", err)
				m.fallBackToUserAction(flowID, plan.fallback)
			default:
				// A user decline (ErrPromptDeclined) or any other prompt failure
				// ends the flow.
				m.logger.Debug("authorization prompt closed", "reason", err)
				cancel()
			}
		}()
	}

	tok, err := plan.run(ctx)
	m.complete(flowID, tok, err)
}

// fallBackToUserAction promotes a running secure flow to the manual user-action
// channel after its prompt could not be delivered. The background flow keeps
// running, so the user can complete authorization out of band and retry. It is a
// no-op if the flow has already resolved.
func (m *Manager) fallBackToUserAction(flowID string, ua *UserAction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.flowID != flowID || m.status != statusInProgress {
		return
	}
	m.status = statusAwaitingUser
	m.pending = ua
	// Wake any callers joined on this flow so they receive the action, and clear
	// done so complete() does not double-close it when run() later finishes.
	if m.done != nil {
		close(m.done)
		m.done = nil
	}
}

// complete records the flow result, installing a refreshing token source on
// success, and wakes any joined callers.
func (m *Manager) complete(flowID string, tok *oauth2.Token, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.flowID != flowID {
		return
	}

	m.status = statusIdle
	m.pending = nil
	m.cancelFlow = nil
	if err != nil {
		m.lastErr = err
		m.logger.Debug("oauth flow failed", "error", err)
	} else {
		m.lastErr = nil
		// Config.TokenSource returns a ReuseTokenSource that refreshes expired
		// tokens using the refresh token — this is what makes GitHub App
		// (expiring) tokens work transparently. Keep the shared exchange bounded
		// independently of any individual request's context.
		refreshCtx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: tokenRefreshTimeout})
		if m.refresh != nil {
			close(m.refresh.done)
			m.refresh = nil
		}
		m.source = m.refreshConfig.TokenSource(refreshCtx, tok)
		m.token = tok
		m.tokenGeneration++
		m.refreshErrLogged = false
		m.logger.Info("github authorization complete")
	}
	if m.done != nil {
		close(m.done)
		m.done = nil
	}
}

// joinWait blocks until the running flow finishes or ctx is cancelled. If the
// flow was promoted to the manual channel while waiting (its prompt could not be
// delivered), it returns that user action rather than an error.
func (m *Manager) joinWait(ctx context.Context, done chan struct{}, flowID string) (*Outcome, error) {
	select {
	case <-done:
		return m.outcomeAfterFlow(ctx, flowID)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// outcomeAfterFlow reports the result once the flow's done channel has closed
// (or when there is no flow in flight): a token to proceed (nil, nil), a pending
// user action to surface, or the flow's error.
func (m *Manager) outcomeAfterFlow(ctx context.Context, flowID string) (*Outcome, error) {
	m.mu.Lock()
	if flowID == "" || flowID != m.flowID {
		m.mu.Unlock()
		return nil, ErrStaleAuthorizationFlow
	}
	m.mu.Unlock()
	token, err := m.AccessTokenContext(ctx)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if flowID != m.flowID {
		return nil, ErrStaleAuthorizationFlow
	}
	if token != "" {
		return nil, nil
	}
	if m.pending != nil {
		return &Outcome{UserAction: m.pending, FlowID: flowID}, nil
	}
	if m.lastErr != nil {
		return nil, m.lastErr
	}
	return nil, errors.New("authorization did not complete")
}

func (m *Manager) oauth2Config(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     m.config.ClientID,
		ClientSecret: m.config.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       m.config.Scopes,
		Endpoint:     m.config.Endpoint,
	}
}
