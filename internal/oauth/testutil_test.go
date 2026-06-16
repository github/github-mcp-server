package oauth

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// fakeGitHub is an httptest-backed stand-in for GitHub's OAuth endpoints. It
// implements the authorize redirect, the token endpoint (authorization_code,
// refresh_token, and device_code grants), and the device-code endpoint, while
// recording what it received so tests can assert on real protocol behavior
// (PKCE challenge/verifier, grant sequence) rather than re-implementing it.
type fakeGitHub struct {
	*httptest.Server

	mu                  sync.Mutex
	grants              []string // grant_type of each token request, in order
	codeChallenge       string
	codeChallengeMethod string
	codeVerifier        string
	devicePending       int // number of authorization_pending responses before success

	// Token values returned per grant. A positive expires field is sent as
	// expires_in (and makes the token expiring/refreshable).
	authToken      string
	authRefresh    string
	authExpires    int
	refreshToken   string
	refreshExpires int
	deviceToken    string
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{
		authToken:   "gho_access",
		deviceToken: "gho_device",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/authorize", f.handleAuthorize)
	mux.HandleFunc("/login/oauth/access_token", f.handleToken)
	mux.HandleFunc("/login/device/code", f.handleDeviceCode)

	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Server.Close)
	return f
}

func (f *fakeGitHub) endpoint() oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:       f.URL + "/login/oauth/authorize",
		TokenURL:      f.URL + "/login/oauth/access_token",
		DeviceAuthURL: f.URL + "/login/device/code",
	}
}

func (f *fakeGitHub) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f.mu.Lock()
	f.codeChallenge = q.Get("code_challenge")
	f.codeChallengeMethod = q.Get("code_challenge_method")
	f.mu.Unlock()

	redirect := q.Get("redirect_uri") + "?code=authcode&state=" + url.QueryEscape(q.Get("state"))
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (f *fakeGitHub) handleToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	grant := r.Form.Get("grant_type")

	f.mu.Lock()
	f.grants = append(f.grants, grant)
	switch grant {
	case "authorization_code":
		f.codeVerifier = r.Form.Get("code_verifier")
		f.mu.Unlock()
		writeToken(w, f.authToken, f.authRefresh, f.authExpires)
	case "refresh_token":
		f.mu.Unlock()
		writeToken(w, f.refreshToken, "", f.refreshExpires)
	case "urn:ietf:params:oauth:grant-type:device_code":
		pending := f.devicePending
		if pending > 0 {
			f.devicePending--
		}
		f.mu.Unlock()
		if pending > 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "authorization_pending"})
			return
		}
		writeToken(w, f.deviceToken, "", 0)
	default:
		f.mu.Unlock()
		http.Error(w, "unsupported grant_type", http.StatusBadRequest)
	}
}

func (f *fakeGitHub) handleDeviceCode(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":      "devicecode",
		"user_code":        "ABCD-1234",
		"verification_uri": f.URL + "/device",
		"expires_in":       900,
		"interval":         1,
	})
}

func (f *fakeGitHub) recordedGrants() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.grants...)
}

func writeToken(w http.ResponseWriter, access, refresh string, expiresIn int) {
	body := map[string]any{
		"access_token": access,
		"token_type":   "bearer",
	}
	if refresh != "" {
		body["refresh_token"] = refresh
	}
	if expiresIn != 0 {
		body["expires_in"] = expiresIn
	}
	writeJSON(w, http.StatusOK, body)
}

func writeJSON(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// fakePrompter is a configurable Prompter. The on* hooks simulate the user
// acting on the prompt; a nil hook means the prompt is shown and acknowledged.
type fakePrompter struct {
	urlCapable  bool
	formCapable bool
	onURL       func(context.Context, Prompt) error
	onForm      func(context.Context, Prompt) error

	mu       sync.Mutex
	urlCalls []Prompt
}

func (p *fakePrompter) CanPromptURL() bool { return p.urlCapable }

func (p *fakePrompter) PromptURL(ctx context.Context, prompt Prompt) error {
	p.mu.Lock()
	p.urlCalls = append(p.urlCalls, prompt)
	p.mu.Unlock()
	if p.onURL != nil {
		return p.onURL(ctx, prompt)
	}
	return nil
}

func (p *fakePrompter) CanPromptForm() bool { return p.formCapable }

func (p *fakePrompter) PromptForm(ctx context.Context, prompt Prompt) error {
	if p.onForm != nil {
		return p.onForm(ctx, prompt)
	}
	return nil
}

// browserGet simulates a user completing the authorization-code flow by opening
// the URL: it follows the authorize redirect to the local callback, delivering
// the code to the manager's callback server. Used both as an openURL seam and
// inside prompter hooks.
func browserGet(rawurl string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawurl, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Body.Close()
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// waitForToken polls until the manager has a token or the deadline passes. The
// authorization-code flow completes asynchronously after the callback fires, so
// tests wait for the resulting token rather than sleeping a fixed duration.
func waitForToken(t *testing.T, m *Manager) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if tok := m.AccessToken(); tok != "" {
			return tok
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for access token")
	return ""
}
