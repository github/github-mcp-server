package ghmcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/pkg/github"
	"github.com/github/github-mcp-server/v2/pkg/http/headers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdioGitHubClientsUseOAuthRequestContext(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		response string
		call     func(context.Context, *githubClients) error
	}{
		{
			name: "REST", response: `{"login":"octocat"}`,
			call: func(ctx context.Context, clients *githubClients) error {
				_, _, err := clients.rest.Users.Get(ctx, "")
				return err
			},
		},
		{
			name: "GraphQL", response: `{"data":{"viewer":{"login":"octocat"}}}`,
			call: func(ctx context.Context, clients *githubClients) error {
				var query struct{ Viewer struct{ Login string } }
				return clients.gql.Query(ctx, &query, nil)
			},
		},
		{
			name: "raw", response: "file contents",
			call: func(ctx context.Context, clients *githubClients) error {
				resp, err := clients.raw.GetRawContent(ctx, "owner", "repo", "file", nil)
				if err != nil {
					return err
				}
				return resp.Body.Close()
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requestCount atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount.Add(1)
				assert.Equal(t, "Bearer refreshed-token", req.Header.Get(headers.AuthorizationHeader))
				w.Header().Set(headers.ContentTypeHeader, headers.ContentTypeJSON)
				_, _ = io.WriteString(w, tt.response)
			}))
			defer api.Close()

			type waitForCancellationKey struct{}
			entered := make(chan struct{}, 1)
			provider := func(ctx context.Context) (string, error) {
				if ctx.Value(waitForCancellationKey{}) == true {
					entered <- struct{}{}
					<-ctx.Done()
					return "", ctx.Err()
				}
				return "refreshed-token", nil
			}
			clients, err := createGitHubClients(github.MCPServerConfig{Version: "test"}, newStaticAPIHostResolver(t, api.URL), provider)
			require.NoError(t, err)

			ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
			defer stop()
			blockedCtx, cancel := context.WithCancel(context.WithValue(ctx, waitForCancellationKey{}, true))
			defer cancel()
			finished := make(chan error, 1)
			go func() { finished <- tt.call(blockedCtx, clients) }()
			select {
			case <-entered:
			case err := <-finished:
				t.Fatalf("request returned before the OAuth provider saw its context: %v", err)
			case <-ctx.Done():
				t.Fatal("OAuth provider did not see the request context")
			}

			require.NoError(t, tt.call(ctx, clients), "the peer should succeed while the first request waits for a token")
			cancel()
			select {
			case err := <-finished:
				require.ErrorIs(t, err, context.Canceled)
			case <-ctx.Done():
				t.Fatal("canceling the request did not stop its token wait")
			}
			assert.EqualValues(t, 1, requestCount.Load(), "only the healthy peer should reach GitHub")
		})
	}
}

func TestStdioOAuthTransportClosesRequestBodyOnProviderError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	body := &oauthRequestBody{Reader: strings.NewReader("graphql query")}
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "https://github.example/graphql", body)
	base := oauthRoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("the request reached the underlying transport")
		return nil, nil
	})
	transport := stdioBearerTransport(github.MCPServerConfig{}, base, []string{req.URL.Host}, func(ctx context.Context) (string, error) {
		return "", ctx.Err()
	})
	resp, err := transport.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, resp)
	assert.True(t, body.closed)
}

func TestStdioOAuthTransportStripsAuthorizationForUnlistedHost(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://other.example/file", nil)
	req.Header.Set(headers.AuthorizationHeader, "Bearer forwarded-token")
	base := oauthRoundTripFunc(func(outgoing *http.Request) (*http.Response, error) {
		assert.Empty(t, outgoing.Header.Get(headers.AuthorizationHeader))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	transport := stdioBearerTransport(github.MCPServerConfig{}, base, []string{"github.example"}, func(context.Context) (string, error) {
		return "refreshed-token", nil
	})
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, "Bearer forwarded-token", req.Header.Get(headers.AuthorizationHeader), "transport must not modify the caller's request")
}

type oauthRequestBody struct {
	io.Reader
	closed bool
}

func (b *oauthRequestBody) Close() error {
	b.closed = true
	return nil
}

type oauthRoundTripFunc func(*http.Request) (*http.Response, error)

func (f oauthRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
