package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/headers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBearerAuthTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		token         string
		tokenProvider func() string
		wantAuth      string
	}{
		{
			name:     "static token",
			token:    "static-token",
			wantAuth: "Bearer static-token",
		},
		{
			name:          "token provider takes precedence over static token",
			token:         "static-token",
			tokenProvider: func() string { return "provided-token" },
			wantAuth:      "Bearer provided-token",
		},
		{
			name:          "token provider with empty static token",
			tokenProvider: func() string { return "provided-token" },
			wantAuth:      "Bearer provided-token",
		},
		{
			name:          "token provider may return empty before authorization",
			tokenProvider: func() string { return "" },
			wantAuth:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotAuth string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get(headers.AuthorizationHeader)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			rt := &BearerAuthTransport{
				Transport:     http.DefaultTransport,
				Token:         tc.token,
				TokenProvider: tc.tokenProvider,
			}

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
			require.NoError(t, err)

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.wantAuth, gotAuth)
		})
	}
}

// TestBearerAuthTransport_TokenProviderResolvedPerRequest verifies that the
// token provider is consulted on every request, so a token that arrives (or is
// refreshed) after the transport is constructed takes effect without rebuilding
// the client. This is the property OAuth relies on.
func TestBearerAuthTransport_TokenProviderResolvedPerRequest(t *testing.T) {
	t.Parallel()

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get(headers.AuthorizationHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	current := ""
	rt := &BearerAuthTransport{
		Transport:     http.DefaultTransport,
		TokenProvider: func() string { return current },
	}

	do := func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
	}

	do()
	assert.Equal(t, "", gotAuth, "no auth header before authorization")

	current = "first-token"
	do()
	assert.Equal(t, "Bearer first-token", gotAuth, "token picked up once available")

	current = "refreshed-token"
	do()
	assert.Equal(t, "Bearer refreshed-token", gotAuth, "refreshed token picked up")
}

func TestBearerAuthTransport_PassesGraphQLFeaturesHeader(t *testing.T) {
	t.Parallel()

	var gotFeatures string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFeatures = r.Header.Get(headers.GraphQLFeaturesHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rt := &BearerAuthTransport{
		Transport: http.DefaultTransport,
		Token:     "token",
	}

	ctx := ghcontext.WithGraphQLFeatures(context.Background(), "feature1", "feature2")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, nil)
	require.NoError(t, err)

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "feature1, feature2", gotFeatures)
}

func TestBearerAuthTransport_DoesNotMutateOriginalRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rt := &BearerAuthTransport{
		Transport: http.DefaultTransport,
		Token:     "token",
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Empty(t, req.Header.Get(headers.AuthorizationHeader), "original request must not be mutated")
}
