package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

// deviceAuthTimeout bounds the synchronous device-code request made while
// preparing the device flow (before any waiting on the user).
const deviceAuthTimeout = 30 * time.Second

// errCallbackBind marks a failure to bind the local OAuth callback listener, so
// begin can treat a busy fixed port as fatal without mislabeling unrelated
// errors (e.g. a failure to generate the state parameter) as a port conflict.
var errCallbackBind = errors.New("OAuth callback listener could not bind")

// flowPlan is a prepared authorization flow ready to run in the background.
type flowPlan struct {
	// run performs the blocking part of the flow (await callback + exchange, or
	// poll the device endpoint) and returns the token.
	run func(context.Context) (*oauth2.Token, error)
	// display, if set, presents the prompt to the user via the Prompter and
	// blocks until they act. A non-nil error (including ErrPromptDeclined)
	// aborts the flow.
	display func(context.Context) error
	// userAction, if set, indicates the last-resort channel: the caller must
	// surface it and the user retries after authorizing out of band.
	userAction *UserAction
}

// begin selects and prepares the appropriate flow. PKCE is preferred for its
// stronger security; device flow is the fallback. A random callback port inside
// Docker cannot be reached from the host browser, so that combination goes
// straight to device flow.
func (m *Manager) begin(prompter Prompter) (*flowPlan, error) {
	canPKCE := m.config.CallbackPort != 0 || !m.inDocker()
	if canPKCE {
		plan, err := m.beginPKCE(prompter)
		if err == nil {
			return plan, nil
		}
		// A fixed callback port that won't bind is fatal, not a cue to downgrade.
		// The port was chosen deliberately (and registered with the OAuth app), so
		// a bind failure means another process holds it — possibly one positioned
		// to intercept the authorization redirect. Silently switching to device
		// flow would mask that, so stop and make the user resolve it. Only genuine
		// bind failures qualify; other errors fall through to device flow.
		if m.config.CallbackPort != 0 && errors.Is(err, errCallbackBind) {
			return nil, fmt.Errorf("OAuth callback port %d is not available; another process may be using it — free the port or set a different --oauth-callback-port: %w", m.config.CallbackPort, err)
		}
		m.logger.Info("PKCE flow unavailable, falling back to device flow", "reason", err)
	} else {
		m.logger.Info("no callback port inside container; using device flow")
	}
	return m.beginDevice(prompter)
}

// beginPKCE prepares the authorization-code + PKCE flow. It binds the callback
// server and selects the most secure available display channel: browser
// auto-open, then URL elicitation, then a tool-response message. On a headless
// host with a random callback port it diverts to device flow, whose redirect
// does not depend on reaching this machine's localhost.
func (m *Manager) beginPKCE(prompter Prompter) (*flowPlan, error) {
	state, err := randomState()
	if err != nil {
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	// Bind to all interfaces only inside a container, where the published port
	// is delivered via eth0 rather than loopback. Native runs stay on loopback.
	listener, err := listenCallback(m.config.CallbackPort, m.inDocker())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCallbackBind, err)
	}
	if m.inDocker() {
		// Inside a container the callback binds all interfaces so the published
		// port is reachable, which also exposes it to the container network.
		// Publishing to loopback only (e.g. -p 127.0.0.1:%d:%d) keeps the
		// authorization code off the network.
		m.logger.Warn(fmt.Sprintf("OAuth callback is listening on all container interfaces; publish it to loopback only (e.g. -p 127.0.0.1:%d:%d) so the authorization code is not exposed on your network", m.config.CallbackPort, m.config.CallbackPort))
	}
	cs := newCallbackServer(listener, state)

	oc := m.oauth2Config(cs.redirect)
	authURL := oc.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))

	run := func(ctx context.Context) (*oauth2.Token, error) {
		code, err := cs.wait(ctx)
		if err != nil {
			return nil, err
		}
		tok, err := oc.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("exchanging authorization code: %w", err)
		}
		return tok, nil
	}

	browserErr := m.openURL(authURL)
	switch {
	case browserErr == nil:
		m.logger.Info("opened browser for GitHub authorization")
		return &flowPlan{run: run}, nil
	case errors.Is(browserErr, errNoDisplay) && m.config.CallbackPort == 0:
		// Headless host with a random callback port: every PKCE channel ends in a
		// redirect to this machine's localhost, which a browser on another machine
		// (e.g. a remote SSH client) cannot reach — so even URL elicitation would
		// dead-end. Device flow is the only channel reachable from elsewhere, so
		// prefer it when the app supports it; otherwise fall through to the manual
		// authorization URL below for a same-machine browser.
		plan, deviceErr := m.beginDevice(prompter)
		if deviceErr == nil {
			cs.close()
			m.logger.Info("no display server; using device flow")
			return plan, nil
		}
		m.logger.Debug("device flow unavailable on headless host; offering manual authorization URL", "reason", deviceErr)
	default:
		m.logger.Debug("browser auto-open unavailable", "reason", browserErr)
	}

	if canPromptURL(prompter) {
		display := func(ctx context.Context) error {
			return prompter.PromptURL(ctx, Prompt{
				Message: "Authorize the GitHub MCP Server in your browser to continue.",
				URL:     authURL,
			})
		}
		return &flowPlan{run: run, display: display}, nil
	}

	return &flowPlan{run: run, userAction: &UserAction{
		URL: authURL,
		Message: fmt.Sprintf(
			"To authorize the GitHub MCP Server, open this URL in your browser:\n\n%s\n\nAfter authorizing, retry your request.\n\n%s",
			authURL, securityAdvisory,
		),
	}}, nil
}

// beginDevice prepares the device authorization flow. It requests a device code
// up front (so the code can be displayed) and selects a display channel:
// URL elicitation, then form elicitation, then a tool-response message.
func (m *Manager) beginDevice(prompter Prompter) (*flowPlan, error) {
	oc := m.oauth2Config("")

	ctx, cancel := context.WithTimeout(context.Background(), deviceAuthTimeout)
	defer cancel()
	da, err := oc.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}

	run := func(ctx context.Context) (*oauth2.Token, error) {
		tok, err := oc.DeviceAccessToken(ctx, da)
		if err != nil {
			return nil, fmt.Errorf("awaiting device authorization: %w", err)
		}
		return tok, nil
	}

	if canPromptURL(prompter) {
		display := func(ctx context.Context) error {
			return prompter.PromptURL(ctx, Prompt{
				Message:  fmt.Sprintf("Enter code %s to authorize the GitHub MCP Server.", da.UserCode),
				URL:      da.VerificationURI,
				UserCode: da.UserCode,
			})
		}
		return &flowPlan{run: run, display: display}, nil
	}

	if canPromptForm(prompter) {
		display := func(ctx context.Context) error {
			return prompter.PromptForm(ctx, Prompt{
				Message:  deviceInstruction(da),
				URL:      da.VerificationURI,
				UserCode: da.UserCode,
			})
		}
		return &flowPlan{run: run, display: display}, nil
	}

	return &flowPlan{run: run, userAction: &UserAction{
		URL:      da.VerificationURI,
		UserCode: da.UserCode,
		Message: fmt.Sprintf(
			"%s\n\nAfter authorizing, retry your request.\n\n%s",
			deviceInstruction(da), securityAdvisory,
		),
	}}, nil
}

// securityAdvisory nudges users on clients without URL elicitation to ask their
// vendor for it, since it keeps the authorization URL out of the model context.
const securityAdvisory = "Note: your MCP client does not appear to support secure URL elicitation. " +
	"For improved security, consider asking your agent, CLI, or IDE to add it (for example, by opening an issue)."

func deviceInstruction(da *oauth2.DeviceAuthResponse) string {
	return fmt.Sprintf("Visit %s and enter the code %s to authorize the GitHub MCP Server.", da.VerificationURI, da.UserCode)
}
