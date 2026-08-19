package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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
				Transport:     newIsolatedTransport(t),
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
		Transport:     newIsolatedTransport(t),
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
		Transport: newIsolatedTransport(t),
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
		Transport: newIsolatedTransport(t),
		Token:     "token",
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Empty(t, req.Header.Get(headers.AuthorizationHeader), "original request must not be mutated")
}

// hostRecordingTransport records the Authorization header seen for each request
// host, so a test can assert what the token would be attached to without a live
// network. It stands in for the real transport at the bottom of the chain.
type hostRecordingTransport struct {
	authByHost map[string]string
}

func (h *hostRecordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	h.authByHost[req.URL.Host] = req.Header.Get(headers.AuthorizationHeader)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

// TestBearerAuthTransport_HostScoping verifies that when AllowedHosts is set,
// the token is attached to a request on an allowed host but withheld from a
// request to any other host. A redirect off the configured GitHub hosts arrives
// here as a RoundTrip to a different host, so this is the property that keeps
// the token from following such a redirect. net/http's own cross-host stripping
// does not cover it, because this transport re-adds the header on every hop.
//
// The hosts are distinct hostnames, matching the real case of api.github.com
// versus objects.githubusercontent.com.
func TestBearerAuthTransport_HostScoping(t *testing.T) {
	t.Parallel()

	rec := &hostRecordingTransport{authByHost: map[string]string{}}
	rt := &BearerAuthTransport{
		Transport:    rec,
		Token:        "secret-token",
		AllowedHosts: []string{"api.github.com", "raw.githubusercontent.com"},
	}

	for _, target := range []string{
		"https://api.github.com/repos/o/r",
		"https://raw.githubusercontent.com/o/r/main/f", // allowed, different host
		"https://objects.githubusercontent.com/evil",   // redirect target, not allowed
		"https://attacker.example.com/steal",           // arbitrary host, not allowed
	} {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
		require.NoError(t, err)
		resp, err := rt.RoundTrip(req)
		require.NoError(t, err)
		resp.Body.Close()
	}

	assert.Equal(t, "Bearer secret-token", rec.authByHost["api.github.com"],
		"token must be sent to an allowed host")
	assert.Equal(t, "Bearer secret-token", rec.authByHost["raw.githubusercontent.com"],
		"token must be sent to every allowed host")
	assert.Empty(t, rec.authByHost["objects.githubusercontent.com"],
		"token must not be sent to a non-allowed host (a redirect target)")
	assert.Empty(t, rec.authByHost["attacker.example.com"],
		"token must not be sent to an arbitrary non-allowed host")
}

// TestBearerAuthTransport_EmptyAllowedHostsPreservesBehavior verifies the
// backward-compatible default: with no AllowedHosts, the token is attached to
// every host, exactly as before this change.
func TestBearerAuthTransport_EmptyAllowedHostsPreservesBehavior(t *testing.T) {
	t.Parallel()

	rec := &hostRecordingTransport{authByHost: map[string]string{}}
	rt := &BearerAuthTransport{Transport: rec, Token: "secret-token"}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://anywhere.example.com/x", nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, "Bearer secret-token", rec.authByHost["anywhere.example.com"],
		"with no AllowedHosts, token attaches to every host as before")
}

func TestBearerAuthTransport_ExactHostScoping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		allowedHosts []string
		target       string
		wantAuth     bool
	}{
		{
			name:         "host comparison is case insensitive",
			allowedHosts: []string{"API.GITHUB.COM"},
			target:       "https://api.github.com/repos/o/r",
			wantAuth:     true,
		},
		{
			name:         "configured port",
			allowedHosts: []string{"github.example.com:8443"},
			target:       "https://GITHUB.EXAMPLE.COM:8443/api/v3/repos/o/r",
			wantAuth:     true,
		},
		{
			name:         "different port",
			allowedHosts: []string{"github.example.com:8443"},
			target:       "https://github.example.com:9443/api/v3/repos/o/r",
		},
		{
			name:         "explicit default port is not implicitly configured",
			allowedHosts: []string{"api.github.com"},
			target:       "https://api.github.com:443/repos/o/r",
		},
		{
			name:         "subdomain lookalike",
			allowedHosts: []string{"api.github.com"},
			target:       "https://evil.api.github.com/steal",
		},
		{
			name:         "suffix lookalike",
			allowedHosts: []string{"api.github.com"},
			target:       "https://api.github.com.attacker.example/steal",
		},
		{
			name:         "userinfo lookalike",
			allowedHosts: []string{"api.github.com"},
			target:       "https://api.github.com@attacker.example/steal",
		},
		{
			name:         "trailing dot is not an exact host",
			allowedHosts: []string{"api.github.com"},
			target:       "https://api.github.com./steal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := &hostRecordingTransport{authByHost: map[string]string{}}
			rt := &BearerAuthTransport{
				Transport:    rec,
				Token:        "secret-token",
				AllowedHosts: tt.allowedHosts,
			}

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, tt.target, nil)
			require.NoError(t, err)
			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			resp.Body.Close()

			if tt.wantAuth {
				assert.NotEmpty(t, rec.authByHost[req.URL.Host])
			} else {
				assert.Empty(t, rec.authByHost[req.URL.Host])
			}
		})
	}
}

func TestBearerAuthTransport_RedirectHostScoping(t *testing.T) {
	t.Parallel()

	var allowedAuth string
	allowedTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedAuth = r.Header.Get(headers.AuthorizationHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer allowedTarget.Close()

	var foreignAuth string
	foreignTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignAuth = r.Header.Get(headers.AuthorizationHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer foreignTarget.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/allowed":
			http.Redirect(w, r, allowedTarget.URL, http.StatusFound)
		case "/foreign":
			http.Redirect(w, r, foreignTarget.URL, http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer source.Close()

	sourceURL, err := url.Parse(source.URL)
	require.NoError(t, err)
	allowedTargetURL, err := url.Parse(allowedTarget.URL)
	require.NoError(t, err)

	client := &http.Client{Transport: &BearerAuthTransport{
		Transport:    newIsolatedTransport(t),
		Token:        "secret-token",
		AllowedHosts: []string{sourceURL.Host, allowedTargetURL.Host},
	}}

	resp, err := client.Get(source.URL + "/allowed")
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEmpty(t, allowedAuth, "token must be sent to a configured redirect host")

	resp, err = client.Get(source.URL + "/foreign")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Empty(t, foreignAuth, "token must not be sent to an unconfigured redirect host")
}

func TestBearerAuthTransport_RemovesAuthorizationFromDisallowedHost(t *testing.T) {
	t.Parallel()

	rec := &hostRecordingTransport{authByHost: map[string]string{}}
	rt := &BearerAuthTransport{
		Transport:    rec,
		Token:        "secret-token",
		AllowedHosts: []string{"api.github.com"},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://attacker.example/steal", nil)
	require.NoError(t, err)
	req.Header.Set(headers.AuthorizationHeader, "caller-supplied-token")

	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Empty(t, rec.authByHost[req.URL.Host])
	assert.NotEmpty(t, req.Header.Get(headers.AuthorizationHeader), "original request must not be mutated")
}
