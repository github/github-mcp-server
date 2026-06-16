package oauth

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// DefaultAuthTimeout bounds how long a single authorization attempt waits for
// the user to complete the browser or device flow.
const DefaultAuthTimeout = 5 * time.Minute

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

	mu      sync.Mutex
	source  oauth2.TokenSource // refreshing source, set once authorized
	status  flowStatus
	pending *UserAction
	done    chan struct{}
	lastErr error
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
	m.mu.Lock()
	src := m.source
	m.mu.Unlock()
	if src == nil {
		return ""
	}
	tok, err := src.Token()
	if err != nil || !tok.Valid() {
		return ""
	}
	return tok.AccessToken
}

// HasToken reports whether a valid token is currently available.
func (m *Manager) HasToken() bool {
	return m.AccessToken() != ""
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
	if m.AccessToken() != "" {
		return nil, nil
	}

	m.mu.Lock()
	switch m.status {
	case statusAwaitingUser:
		ua := m.pending
		m.mu.Unlock()
		return &Outcome{UserAction: ua}, nil
	case statusStarting:
		m.mu.Unlock()
		return &Outcome{UserAction: &UserAction{
			Message: "GitHub authorization is already in progress. Please retry your request in a few seconds.",
		}}, nil
	case statusInProgress:
		done := m.done
		m.mu.Unlock()
		return m.joinWait(ctx, done)
	}

	// Idle: this call owns the new flow.
	m.status = statusStarting
	m.lastErr = nil
	m.done = make(chan struct{})
	done := m.done
	m.mu.Unlock()

	plan, err := m.begin(prompter)
	if err != nil {
		m.complete(nil, err)
		return nil, err
	}

	m.mu.Lock()
	if plan.userAction != nil {
		m.status = statusAwaitingUser
		m.pending = plan.userAction
	} else {
		m.status = statusInProgress
	}
	m.mu.Unlock()

	bgCtx, cancel := context.WithTimeout(context.Background(), DefaultAuthTimeout)
	go m.runFlow(bgCtx, cancel, plan)

	if plan.userAction != nil {
		return &Outcome{UserAction: plan.userAction}, nil
	}
	return m.joinWait(ctx, done)
}

// runFlow executes a prepared flow in the background and records the result. The
// optional display prompt runs concurrently; if it ends in error or decline it
// cancels the flow.
func (m *Manager) runFlow(ctx context.Context, cancel context.CancelFunc, plan *flowPlan) {
	defer cancel()

	if plan.display != nil {
		go func() {
			if err := plan.display(ctx); err != nil {
				m.logger.Debug("authorization prompt closed", "reason", err)
				cancel()
			}
		}()
	}

	tok, err := plan.run(ctx)
	m.complete(tok, err)
}

// complete records the flow result, installing a refreshing token source on
// success, and wakes any joined callers.
func (m *Manager) complete(tok *oauth2.Token, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.status = statusIdle
	m.pending = nil
	if err != nil {
		m.lastErr = err
		m.logger.Debug("oauth flow failed", "error", err)
	} else {
		m.lastErr = nil
		// Config.TokenSource returns a ReuseTokenSource that refreshes expired
		// tokens using the refresh token — this is what makes GitHub App
		// (expiring) tokens work transparently.
		m.source = m.refreshConfig.TokenSource(context.Background(), tok)
		m.logger.Info("github authorization complete")
	}
	if m.done != nil {
		close(m.done)
		m.done = nil
	}
}

// joinWait blocks until the running flow finishes or ctx is cancelled.
func (m *Manager) joinWait(ctx context.Context, done chan struct{}) (*Outcome, error) {
	select {
	case <-done:
		if m.AccessToken() != "" {
			return nil, nil
		}
		m.mu.Lock()
		err := m.lastErr
		m.mu.Unlock()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("authorization did not complete")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
