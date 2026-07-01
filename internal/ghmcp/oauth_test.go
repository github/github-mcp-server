package ghmcp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/internal/oauth"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// probeToolName is the name of the throwaway tool the harness registers; its
// handler runs a probe closure against a sessionPrompter so the adapter can be
// exercised against a real, fully-negotiated server session from the client side.
const probeToolName = "probe"

// runProbe stands up an in-memory MCP client/server pair, registers a tool whose
// handler runs probe against a sessionPrompter wrapping the live server session,
// and returns the text the probe produced. The client is configured with the
// given capabilities and elicitation handler so the adapter sees a real,
// fully-negotiated session rather than a hand-built fake.
func runProbe(
	t *testing.T,
	clientCaps *mcp.ClientCapabilities,
	elicitationHandler func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error),
	probe func(context.Context, *sessionPrompter) string,
) string {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: probeToolName}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		text := probe(ctx, &sessionPrompter{session: req.Session})
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})

	st, ct := mcp.NewInMemoryTransports()

	ss, err := server.Connect(context.Background(), st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, &mcp.ClientOptions{
		Capabilities:       clientCaps,
		ElicitationHandler: elicitationHandler,
	})
	cs, err := client.Connect(context.Background(), ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: probeToolName})
	require.NoError(t, err)
	require.Len(t, res.Content, 1)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "probe result should be text content")
	return text.Text
}

func TestSessionPrompterCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		caps     *mcp.ClientCapabilities
		wantURL  bool
		wantForm bool
	}{
		{
			name:     "no elicitation advertised",
			caps:     &mcp.ClientCapabilities{},
			wantURL:  false,
			wantForm: false,
		},
		{
			name:     "url only",
			caps:     &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}}},
			wantURL:  true,
			wantForm: false,
		},
		{
			name:     "form only",
			caps:     &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{Form: &mcp.FormElicitationCapabilities{}}},
			wantURL:  false,
			wantForm: true,
		},
		{
			name:     "url and form",
			caps:     &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}, Form: &mcp.FormElicitationCapabilities{}}},
			wantURL:  true,
			wantForm: true,
		},
		{
			name:     "empty elicitation capability implies form for backward compatibility",
			caps:     &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{}},
			wantURL:  false,
			wantForm: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := runProbe(t, tc.caps, nil, func(_ context.Context, p *sessionPrompter) string {
				if p.CanPromptURL() {
					if p.CanPromptForm() {
						return "url+form"
					}
					return "url"
				}
				if p.CanPromptForm() {
					return "form"
				}
				return "none"
			})

			want := "none"
			switch {
			case tc.wantURL && tc.wantForm:
				want = "url+form"
			case tc.wantURL:
				want = "url"
			case tc.wantForm:
				want = "form"
			}
			assert.Equal(t, want, got)
		})
	}
}

func TestSessionPrompterPromptActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		action      string
		wantDecline bool
	}{
		{name: "accept", action: "accept", wantDecline: false},
		{name: "decline", action: "decline", wantDecline: true},
		{name: "cancel", action: "cancel", wantDecline: true},
	}

	caps := &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{
		URL:  &mcp.URLElicitationCapabilities{},
		Form: &mcp.FormElicitationCapabilities{},
	}}

	for _, tc := range tests {
		// URL and form modes share the accept/decline mapping; cover both.
		for _, mode := range []string{"url", "form"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				t.Parallel()

				handler := func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
					return &mcp.ElicitResult{Action: tc.action}, nil
				}

				got := runProbe(t, caps, handler, func(ctx context.Context, p *sessionPrompter) string {
					var err error
					if mode == "url" {
						err = p.PromptURL(ctx, oauth.Prompt{Message: "msg", URL: "https://example.com/auth"})
					} else {
						err = p.PromptForm(ctx, oauth.Prompt{Message: "msg"})
					}
					if err == nil {
						return "ok"
					}
					if err == oauth.ErrPromptDeclined {
						return "declined"
					}
					return "error: " + err.Error()
				})

				if tc.wantDecline {
					assert.Equal(t, "declined", got)
				} else {
					assert.Equal(t, "ok", got)
				}
			})
		}
	}
}

// TestSessionPrompterTransportError verifies that a prompt which fails to be
// delivered (the client errors instead of returning an action) is reported as
// ErrPromptUnavailable, not ErrPromptDeclined. The manager relies on this
// distinction to fall back to manual instructions instead of aborting.
func TestSessionPrompterTransportError(t *testing.T) {
	t.Parallel()

	caps := &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{
		URL:  &mcp.URLElicitationCapabilities{},
		Form: &mcp.FormElicitationCapabilities{},
	}}

	for _, mode := range []string{"url", "form"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			handler := func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
				return nil, errors.New("client cannot deliver elicitation")
			}

			got := runProbe(t, caps, handler, func(ctx context.Context, p *sessionPrompter) string {
				var err error
				if mode == "url" {
					err = p.PromptURL(ctx, oauth.Prompt{Message: "msg", URL: "https://example.com/auth"})
				} else {
					err = p.PromptForm(ctx, oauth.Prompt{Message: "msg"})
				}
				switch {
				case err == nil:
					return "ok"
				case errors.Is(err, oauth.ErrPromptDeclined):
					return "declined"
				case errors.Is(err, oauth.ErrPromptUnavailable):
					return "unavailable"
				default:
					return "error: " + err.Error()
				}
			})

			assert.Equal(t, "unavailable", got,
				"a delivery failure must be classified as undeliverable, not a decline")
		})
	}
}

// fakeAuthenticator is a deterministic stand-in for *oauth.Manager that lets the
// middleware be tested at each branch without standing up live GitHub flows.
type fakeAuthenticator struct {
	hasToken     bool
	outcome      *oauth.Outcome
	err          error
	authCalls    int
	lastPrompter oauth.Prompter
}

func (f *fakeAuthenticator) HasToken() bool { return f.hasToken }

func (f *fakeAuthenticator) Authenticate(_ context.Context, prompter oauth.Prompter) (*oauth.Outcome, error) {
	f.authCalls++
	f.lastPrompter = prompter
	return f.outcome, f.err
}

func TestCreateOAuthMiddleware(t *testing.T) {
	t.Parallel()

	const nextText = "handler-ran"
	newNext := func(called *bool) mcp.MethodHandler {
		return func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
			*called = true
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: nextText}}}, nil
		}
	}

	t.Run("non tool call passes through without authenticating", func(t *testing.T) {
		t.Parallel()
		fake := &fakeAuthenticator{hasToken: false}
		var called bool
		mw := createOAuthMiddleware(fake, discardLogger())
		_, err := mw(newNext(&called))(context.Background(), "initialize", &mcp.InitializeRequest{})
		require.NoError(t, err)
		assert.True(t, called, "next should run")
		assert.Zero(t, fake.authCalls, "authentication must not run for non tool calls")
	})

	t.Run("existing token short circuits authentication", func(t *testing.T) {
		t.Parallel()
		fake := &fakeAuthenticator{hasToken: true}
		var called bool
		mw := createOAuthMiddleware(fake, discardLogger())
		_, err := mw(newNext(&called))(context.Background(), "tools/call", &mcp.CallToolRequest{})
		require.NoError(t, err)
		assert.True(t, called, "next should run")
		assert.Zero(t, fake.authCalls, "authentication must be skipped when a token already exists")
	})

	t.Run("successful authentication proceeds to handler", func(t *testing.T) {
		t.Parallel()
		fake := &fakeAuthenticator{hasToken: false, outcome: nil, err: nil}
		var called bool
		mw := createOAuthMiddleware(fake, discardLogger())
		res, err := mw(newNext(&called))(context.Background(), "tools/call", &mcp.CallToolRequest{})
		require.NoError(t, err)
		assert.Equal(t, 1, fake.authCalls)
		assert.True(t, called, "next should run once authorized")
		callRes, ok := res.(*mcp.CallToolResult)
		require.True(t, ok)
		require.Len(t, callRes.Content, 1)
		assert.Equal(t, nextText, callRes.Content[0].(*mcp.TextContent).Text)
	})

	t.Run("pending user action is surfaced as a tool result", func(t *testing.T) {
		t.Parallel()
		const message = "Open https://example.com/auth to authorize, then retry."
		fake := &fakeAuthenticator{hasToken: false, outcome: &oauth.Outcome{UserAction: &oauth.UserAction{Message: message}}}
		var called bool
		mw := createOAuthMiddleware(fake, discardLogger())
		res, err := mw(newNext(&called))(context.Background(), "tools/call", &mcp.CallToolRequest{})
		require.NoError(t, err)
		assert.False(t, called, "next must not run while the user still needs to authorize")
		callRes, ok := res.(*mcp.CallToolResult)
		require.True(t, ok)
		require.Len(t, callRes.Content, 1)
		assert.Equal(t, message, callRes.Content[0].(*mcp.TextContent).Text)
	})

	t.Run("authentication error is returned", func(t *testing.T) {
		t.Parallel()
		fake := &fakeAuthenticator{hasToken: false, err: assert.AnError}
		var called bool
		mw := createOAuthMiddleware(fake, discardLogger())
		_, err := mw(newNext(&called))(context.Background(), "tools/call", &mcp.CallToolRequest{})
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.False(t, called, "next must not run when authentication fails")
	})
}

// TestRunStdioServerRejectsTokenAndOAuth verifies the mutually-exclusive guard:
// supplying both a static token and an OAuth manager is rejected before the
// server starts, rather than silently preferring one for auth and the other for
// scope filtering.
func TestRunStdioServerRejectsTokenAndOAuth(t *testing.T) {
	t.Parallel()

	mgr := oauth.NewManager(oauth.NewGitHubConfig("client-id", "", nil, "", 0), discardLogger())
	err := RunStdioServer(StdioServerConfig{
		Token:        "ghp_static",
		OAuthManager: mgr,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestRunStdioServerRejectsAppAuthAndOAuth(t *testing.T) {
	t.Parallel()

	mgr := oauth.NewManager(oauth.NewGitHubConfig("client-id", "", nil, "", 0), discardLogger())
	err := RunStdioServer(StdioServerConfig{
		AppID:          1,
		PrivateKey:     []byte("not-a-real-key"),
		InstallationID: 2,
		OAuthManager:   mgr,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

// TestCreateGitHubClientsTokenProvider proves the OAuth wiring: when a
// TokenProvider is configured the REST client authenticates with the provider's
// current token on every request (and never pins a stale one), which is what the
// lazy, refreshing OAuth token depends on.
func TestCreateGitHubClientsTokenProvider(t *testing.T) {
	t.Parallel()

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get(headers.AuthorizationHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	current := ""
	apiHost, err := utils.NewAPIHost(server.URL)
	require.NoError(t, err)

	clients, err := createGitHubClients(github.MCPServerConfig{
		Version:       "test",
		TokenProvider: func() string { return current },
	}, apiHost, nil)
	require.NoError(t, err)

	do := func() {
		resp, err := clients.rest.Client().Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
	}

	do()
	assert.Equal(t, "", gotAuth, "no auth header before authorization")

	current = "oauth-token"
	do()
	assert.Equal(t, "Bearer oauth-token", gotAuth, "provider token used once available")

	current = "refreshed-token"
	do()
	assert.Equal(t, "Bearer refreshed-token", gotAuth, "refreshed provider token used")
}
