package githubapp

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAppServer serves the two endpoints a MultiProvider uses: the installation
// directory and the per-installation token exchange. installations maps an
// account login to its installation ID.
func newAppServer(t *testing.T, installations map[string]int64, listCalls *atomic.Int64) *httptest.Server {
	t.Helper()

	type account struct {
		Login string `json:"login"`
	}
	type installation struct {
		ID      int64   `json:"id"`
		Account account `json:"account"`
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/app/installations", func(w http.ResponseWriter, r *http.Request) {
		if listCalls != nil {
			listCalls.Add(1)
		}
		if r.URL.Query().Get("page") != "1" {
			require.NoError(t, json.NewEncoder(w).Encode([]installation{}))
			return
		}
		body := make([]installation, 0, len(installations))
		for login, id := range installations {
			body = append(body, installation{ID: id, Account: account{Login: login}})
		}
		require.NoError(t, json.NewEncoder(w).Encode(body))
	})
	mux.HandleFunc("/app/installations/", func(w http.ResponseWriter, r *http.Request) {
		// The token names its installation so tests can assert which one a
		// request was routed to. Parsing the ID rather than echoing the path
		// keeps untrusted input out of the response body.
		raw := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/app/installations/"), "/access_tokens")
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
		body := map[string]string{
			"token":      "token-for-" + strconv.FormatInt(id, 10),
			"expires_at": "2999-01-01T00:00:00Z",
		}
		require.NoError(t, json.NewEncoder(w).Encode(body))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func newTestMultiProvider(t *testing.T, baseURL string) *MultiProvider {
	t.Helper()
	provider, err := NewMultiProvider(MultiConfig{
		AppID:         "123",
		PrivateKeyPEM: pkcs1PEM(t, newTestKey(t)),
		BaseRESTURL:   baseURL,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	return provider
}

func TestMultiConfigValidate(t *testing.T) {
	key := pkcs1PEM(t, newTestKey(t))

	tests := []struct {
		name    string
		cfg     MultiConfig
		wantErr string
	}{
		{
			name: "valid",
			cfg:  MultiConfig{AppID: "1", PrivateKeyPEM: key, BaseRESTURL: "https://api.github.com/"},
		},
		{
			name:    "missing app ID",
			cfg:     MultiConfig{PrivateKeyPEM: key, BaseRESTURL: "https://api.github.com/"},
			wantErr: "GITHUB_APP_ID",
		},
		{
			name:    "missing private key",
			cfg:     MultiConfig{AppID: "1", BaseRESTURL: "https://api.github.com/"},
			wantErr: "private key is required",
		},
		{
			name:    "missing base URL",
			cfg:     MultiConfig{AppID: "1", PrivateKeyPEM: key},
			wantErr: "REST base URL is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestMultiProviderTokenForOwner verifies that each owner is routed to the
// installation that owns it, and that an owner the app is not installed on
// yields no token rather than another installation's.
func TestMultiProviderTokenForOwner(t *testing.T) {
	server := newAppServer(t, map[string]int64{"octo-org": 11, "other-org": 22}, nil)
	provider := newTestMultiProvider(t, server.URL+"/")

	assert.Equal(t, "token-for-11", provider.TokenForOwner("octo-org"))
	assert.Equal(t, "token-for-22", provider.TokenForOwner("other-org"))
	assert.Empty(t, provider.TokenForOwner("unknown-org"))
	assert.Empty(t, provider.TokenForOwner(""))
}

// TestMultiProviderOwnerLookupIsCaseInsensitive covers owners whose casing in a
// tool argument differs from the account login GitHub returns.
func TestMultiProviderOwnerLookupIsCaseInsensitive(t *testing.T) {
	server := newAppServer(t, map[string]int64{"Octo-Org": 11}, nil)
	provider := newTestMultiProvider(t, server.URL+"/")

	assert.Equal(t, "token-for-11", provider.TokenForOwner("octo-ORG"))
}

// TestMultiProviderCachesInstallationDirectory verifies that a resolvable owner
// does not re-list installations on every request, and that an unresolvable one
// does not re-list until the refresh interval elapses.
func TestMultiProviderCachesInstallationDirectory(t *testing.T) {
	var listCalls atomic.Int64
	server := newAppServer(t, map[string]int64{"octo-org": 11}, &listCalls)
	provider := newTestMultiProvider(t, server.URL+"/")

	for range 3 {
		assert.Equal(t, "token-for-11", provider.TokenForOwner("octo-org"))
	}
	assert.Equal(t, int64(1), listCalls.Load())

	for range 3 {
		assert.Empty(t, provider.TokenForOwner("unknown-org"))
	}
	assert.Equal(t, int64(1), listCalls.Load(), "a miss within the refresh interval should not re-list")
}

// TestMultiProviderRefreshesOnMiss verifies that an owner added to the app after
// the directory was cached resolves once the cached copy is stale.
func TestMultiProviderRefreshesOnMiss(t *testing.T) {
	installations := map[string]int64{"octo-org": 11}
	server := newAppServer(t, installations, nil)
	provider := newTestMultiProvider(t, server.URL+"/")

	require.Equal(t, "token-for-11", provider.TokenForOwner("octo-org"))
	require.Empty(t, provider.TokenForOwner("late-org"))

	installations["late-org"] = 33
	// Age the cached directory past the refresh interval.
	provider.mu.Lock()
	provider.listedAt = provider.listedAt.Add(-2 * installationsRefreshInterval)
	provider.mu.Unlock()

	assert.Equal(t, "token-for-33", provider.TokenForOwner("late-org"))
}

// TestMultiProviderTokenForRequest exercises the routing the transport relies
// on, over both REST paths and GraphQL bodies.
func TestMultiProviderTokenForRequest(t *testing.T) {
	server := newAppServer(t, map[string]int64{"octo-org": 11, "other-org": 22}, nil)
	provider := newTestMultiProvider(t, server.URL+"/")

	restReq, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/octo-org/repo/issues", nil)
	require.NoError(t, err)
	assert.Equal(t, "token-for-11", provider.TokenForRequest(restReq))

	body := `{"query":"query($owner:String!){}","variables":{"owner":"other-org"}}`
	gqlReq, err := http.NewRequest(http.MethodPost, "https://api.github.com/graphql", strings.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, "token-for-22", provider.TokenForRequest(gqlReq))

	unscopedReq, err := http.NewRequest(http.MethodGet, "https://api.github.com/rate_limit", nil)
	require.NoError(t, err)
	assert.Empty(t, provider.TokenForRequest(unscopedReq))
}

// TestMultiProviderListError verifies that a failing directory listing degrades
// to an empty token rather than panicking or returning a wrong one.
func TestMultiProviderListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.Copy(w, bytes.NewBufferString(`{"message":"Bad credentials"}`))
	}))
	t.Cleanup(server.Close)

	provider := newTestMultiProvider(t, server.URL+"/")
	assert.Empty(t, provider.TokenForOwner("octo-org"))
}
