package oauth

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newManager wires a Manager to the fake GitHub server. By default the browser
// auto-opens (driving the callback) and Docker detection is off.
func newManager(t *testing.T, f *fakeGitHub) *Manager {
	t.Helper()
	cfg := Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Scopes:       []string{"repo"},
		Endpoint:     f.endpoint(),
	}
	m := NewManager(cfg, testLogger())
	m.openURL = browserGet
	m.inDocker = func() bool { return false }
	return m
}

func TestAuthenticatePKCEViaBrowser(t *testing.T) {
	f := newFakeGitHub(t)
	m := newManager(t, f)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := m.Authenticate(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, out, "browser flow completes without a user action")
	assert.Equal(t, "gho_access", m.AccessToken())

	// PKCE must have been exercised end to end.
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, "S256", f.codeChallengeMethod)
	assert.NotEmpty(t, f.codeChallenge, "authorize must receive a code_challenge")
	assert.NotEmpty(t, f.codeVerifier, "token exchange must send a code_verifier")
	assert.Equal(t, []string{"authorization_code"}, f.grants)
}

func TestAuthenticateRefreshesExpiringGitHubAppToken(t *testing.T) {
	f := newFakeGitHub(t)
	// GitHub App: the initial token expires immediately and carries a refresh
	// token, so the very next read must refresh transparently.
	f.authToken = "ghu_initial"
	f.authRefresh = "ghr_refresh"
	f.authExpires = 1
	f.refreshToken = "ghu_refreshed"
	f.refreshExpires = 3600

	m := newManager(t, f)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := m.Authenticate(ctx, nil)
	require.NoError(t, err)

	assert.Equal(t, "ghu_refreshed", m.AccessToken(), "expired token must be refreshed")
	assert.Equal(t, []string{"authorization_code", "refresh_token"}, f.recordedGrants())
}

func TestAuthenticateURLElicitation(t *testing.T) {
	f := newFakeGitHub(t)
	m := newManager(t, f)
	m.openURL = func(string) error { return errors.New("no browser") }

	prompter := &fakePrompter{
		urlCapable: true,
		onURL: func(_ context.Context, p Prompt) error {
			return browserGet(p.URL) // user opens the URL and authorizes
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := m.Authenticate(ctx, prompter)
	require.NoError(t, err)
	assert.Nil(t, out)
	assert.Equal(t, "gho_access", m.AccessToken())

	prompter.mu.Lock()
	defer prompter.mu.Unlock()
	require.Len(t, prompter.urlCalls, 1)
	assert.NotEmpty(t, prompter.urlCalls[0].URL)
}

func TestAuthenticateDeclinedPromptFails(t *testing.T) {
	f := newFakeGitHub(t)
	m := newManager(t, f)
	m.openURL = func(string) error { return errors.New("no browser") }

	prompter := &fakePrompter{
		urlCapable: true,
		onURL: func(_ context.Context, _ Prompt) error {
			return ErrPromptDeclined
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := m.Authenticate(ctx, prompter)
	require.Error(t, err, "declining the prompt must abort the flow")
	assert.Empty(t, m.AccessToken())
}

func TestAuthenticateLastDitchUserAction(t *testing.T) {
	f := newFakeGitHub(t)
	m := newManager(t, f)
	m.openURL = func(string) error { return errors.New("no browser") }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// No browser and a nil prompter: the only channel left is a user action
	// returned to the caller.
	out, err := m.Authenticate(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.UserAction)
	assert.NotEmpty(t, out.UserAction.URL)
	assert.Contains(t, out.UserAction.Message, "open this URL")
	assert.Contains(t, out.UserAction.Message, securityAdvisory,
		"missing URL elicitation should trigger the security advisory")

	// A concurrent retry while awaiting the user returns the same action, not a
	// second flow.
	out2, err := m.Authenticate(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, out2.UserAction)
	assert.Equal(t, out.UserAction.URL, out2.UserAction.URL)

	// The user opens the URL out of band; the background flow then completes.
	require.NoError(t, browserGet(out.UserAction.URL))
	assert.Equal(t, "gho_access", waitForToken(t, m))
}

func TestAuthenticateDeviceFlow(t *testing.T) {
	f := newFakeGitHub(t)
	f.deviceToken = "gho_device_token"
	m := newManager(t, f)
	// Inside Docker with a random port, PKCE is impossible, so the device flow
	// is selected.
	m.inDocker = func() bool { return true }
	m.openURL = func(string) error { return errors.New("no browser") }

	prompter := &fakePrompter{urlCapable: true} // shows the code, no action needed

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := m.Authenticate(ctx, prompter)
	require.NoError(t, err)
	assert.Nil(t, out)
	assert.Equal(t, "gho_device_token", m.AccessToken())
	assert.Contains(t, f.recordedGrants(), "urn:ietf:params:oauth:grant-type:device_code")
}

func TestAuthenticateDevicePollingPending(t *testing.T) {
	f := newFakeGitHub(t)
	f.deviceToken = "gho_device_token"
	f.devicePending = 1 // one authorization_pending before success
	m := newManager(t, f)
	m.inDocker = func() bool { return true }
	m.openURL = func(string) error { return errors.New("no browser") }

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	_, err := m.Authenticate(ctx, &fakePrompter{urlCapable: true})
	require.NoError(t, err)
	assert.Equal(t, "gho_device_token", m.AccessToken())
}

func TestAuthenticateHeadlessPrefersDeviceFlow(t *testing.T) {
	f := newFakeGitHub(t)
	f.deviceToken = "gho_device_token"
	m := newManager(t, f)
	// A headless host (no display server) with a random callback port: a PKCE
	// redirect to this machine's localhost is unreachable from a browser on
	// another machine, so device flow must be chosen even though the client can
	// elicit a URL (which would otherwise win over device flow).
	m.openURL = func(string) error { return errNoDisplay }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := m.Authenticate(ctx, &fakePrompter{urlCapable: true})
	require.NoError(t, err)
	assert.Nil(t, out)
	assert.Equal(t, "gho_device_token", m.AccessToken())
	grants := f.recordedGrants()
	assert.Contains(t, grants, "urn:ietf:params:oauth:grant-type:device_code")
	assert.NotContains(t, grants, "authorization_code",
		"headless host must skip the unreachable PKCE authorization-code flow")
}

func TestAuthenticateFixedCallbackPortUnavailableIsFatal(t *testing.T) {
	f := newFakeGitHub(t)
	m := newManager(t, f)

	// Occupy the fixed callback port so the OAuth listener cannot bind it. A held
	// port could belong to another user's process that would receive the redirect,
	// so the flow must fail loudly rather than quietly downgrade to device flow.
	squatter, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer squatter.Close()
	port := squatter.Addr().(*net.TCPAddr).Port
	m.config.CallbackPort = port

	// A browser that would have completed PKCE, proving the abort is caused by the
	// unavailable port and not by a missing display channel.
	m.openURL = browserGet

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := m.Authenticate(ctx, &fakePrompter{urlCapable: true})
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), strconv.Itoa(port))
	assert.Empty(t, m.AccessToken())
	// The decisive check: no device-code grant was attempted, so the flow did not
	// silently fall back when the deliberately chosen port was unavailable.
	assert.Empty(t, f.recordedGrants(), "fixed-port bind failure must not fall back to device flow")
}

func TestAuthenticateNoTokenInitially(t *testing.T) {
	f := newFakeGitHub(t)
	m := newManager(t, f)
	assert.False(t, m.HasToken())
	assert.Empty(t, m.AccessToken())
}

func TestAuthenticateSingleFlight(t *testing.T) {
	f := newFakeGitHub(t)
	m := newManager(t, f)

	// Hold the owner inside begin() (browser open blocks) so a concurrent caller
	// observes the in-progress flow rather than starting its own.
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	m.openURL = func(u string) error {
		once.Do(func() { close(entered) })
		<-release
		return browserGet(u)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ownerDone := make(chan error, 1)
	go func() {
		_, err := m.Authenticate(ctx, nil)
		ownerDone <- err
	}()

	<-entered // owner is now mid-flow with status "starting"

	out, err := m.Authenticate(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, out.UserAction)
	assert.Contains(t, out.UserAction.Message, "already in progress")

	close(release)
	require.NoError(t, <-ownerDone)

	assert.Equal(t, "gho_access", waitForToken(t, m))
	// Exactly one authorization happened despite the concurrent callers.
	assert.Equal(t, []string{"authorization_code"}, f.recordedGrants())
}
