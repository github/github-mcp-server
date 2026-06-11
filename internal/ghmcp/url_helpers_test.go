package ghmcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAPIHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		host        string
		wantErr     bool
		wantRESTURL string
	}{
		{name: "empty defaults to dotcom", host: "", wantRESTURL: "https://api.github.com/"},
		{name: "github.com routes to dotcom", host: "https://github.com", wantRESTURL: "https://api.github.com/"},
		{name: "ghe.com routes to GHEC", host: "https://tenant.ghe.com", wantRESTURL: "https://api.tenant.ghe.com/"},
		{name: "missing scheme is an error", host: "github.com", wantErr: true},
		{name: "unparseable host is an error", host: "://nope", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			host, err := parseAPIHost(tt.host)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRESTURL, host.baseRESTURL.String())
		})
	}
}

func TestNewDotcomHost(t *testing.T) {
	t.Parallel()

	host, err := newDotcomHost()
	require.NoError(t, err)
	assert.Equal(t, "https://api.github.com/", host.baseRESTURL.String())
	assert.Equal(t, "https://api.github.com/graphql", host.graphqlURL.String())
	assert.Equal(t, "https://uploads.github.com", host.uploadURL.String())
	assert.Equal(t, "https://raw.githubusercontent.com/", host.rawURL.String())
}

func TestNewGHECHost(t *testing.T) {
	t.Parallel()

	t.Run("builds subdomain URLs", func(t *testing.T) {
		t.Parallel()
		host, err := newGHECHost("https://tenant.ghe.com")
		require.NoError(t, err)
		assert.Equal(t, "https://api.tenant.ghe.com/", host.baseRESTURL.String())
		assert.Equal(t, "https://api.tenant.ghe.com/graphql", host.graphqlURL.String())
		assert.Equal(t, "https://uploads.tenant.ghe.com", host.uploadURL.String())
		assert.Equal(t, "https://raw.tenant.ghe.com/", host.rawURL.String())
	})

	t.Run("rejects http scheme", func(t *testing.T) {
		t.Parallel()
		_, err := newGHECHost("http://tenant.ghe.com")
		require.Error(t, err)
	})
}

func TestNewGHESHost(t *testing.T) {
	// Substitute the subdomain-isolation probe to avoid live network calls.
	// (Not parallel: it swaps a package-level seam.)
	original := subdomainIsolationCheck
	t.Cleanup(func() { subdomainIsolationCheck = original })

	t.Run("with subdomain isolation", func(t *testing.T) {
		subdomainIsolationCheck = func(_, _ string) bool { return true }

		host, err := newGHESHost("https://ghes.example.com")
		require.NoError(t, err)
		assert.Equal(t, "https://ghes.example.com/api/v3/", host.baseRESTURL.String())
		assert.Equal(t, "https://ghes.example.com/api/graphql", host.graphqlURL.String())
		assert.Equal(t, "https://uploads.ghes.example.com/", host.uploadURL.String())
		assert.Equal(t, "https://raw.ghes.example.com/", host.rawURL.String())
	})

	t.Run("without subdomain isolation", func(t *testing.T) {
		subdomainIsolationCheck = func(_, _ string) bool { return false }

		host, err := newGHESHost("https://ghes.example.com")
		require.NoError(t, err)
		assert.Equal(t, "https://ghes.example.com/api/v3/", host.baseRESTURL.String())
		assert.Equal(t, "https://ghes.example.com/api/graphql", host.graphqlURL.String())
		assert.Equal(t, "https://ghes.example.com/api/uploads/", host.uploadURL.String())
		assert.Equal(t, "https://ghes.example.com/raw/", host.rawURL.String())
	})
}

// capturingRoundTripper records the request it receives and returns a canned response.
type capturingRoundTripper struct {
	captured *http.Request
}

func (c *capturingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.captured = req
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestUserAgentTransport(t *testing.T) {
	t.Parallel()

	capture := &capturingRoundTripper{}
	rt := &userAgentTransport{transport: capture, agent: "github-mcp-server/test"}

	req := httptest.NewRequest(http.MethodGet, "https://api.github.com/", nil)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "github-mcp-server/test", capture.captured.Header.Get("User-Agent"))
	// The original request must not be mutated (RoundTrip clones it).
	assert.Empty(t, req.Header.Get("User-Agent"))
}

func TestBearerAuthTransport(t *testing.T) {
	t.Parallel()

	capture := &capturingRoundTripper{}
	rt := &bearerAuthTransport{transport: capture, token: "secret-token"}

	req := httptest.NewRequest(http.MethodGet, "https://api.github.com/", nil)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "Bearer secret-token", capture.captured.Header.Get("Authorization"))
	assert.Empty(t, req.Header.Get("Authorization"))
}

func TestAddGitHubAPIErrorToContext(t *testing.T) {
	t.Parallel()

	var capturedCtx context.Context
	next := func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		capturedCtx = ctx
		return nil, nil
	}

	wrapped := addGitHubAPIErrorToContext(next)
	_, err := wrapped(context.Background(), "tools/call", nil)
	require.NoError(t, err)

	// The downstream handler should see a context primed for GitHub API errors.
	_, gErr := ghErrors.GetGitHubAPIErrors(capturedCtx)
	require.NoError(t, gErr)
}

func TestCreateFeatureChecker(t *testing.T) {
	t.Parallel()

	checker := createFeatureChecker([]string{"feature_a", "feature_b"})

	present, err := checker(context.Background(), "feature_a")
	require.NoError(t, err)
	assert.True(t, present)

	absent, err := checker(context.Background(), "feature_c")
	require.NoError(t, err)
	assert.False(t, absent)

	empty := createFeatureChecker(nil)
	got, err := empty(context.Background(), "anything")
	require.NoError(t, err)
	assert.False(t, got)
}
