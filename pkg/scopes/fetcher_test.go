package scopes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScopeHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected []string
	}{
		{
			name:     "empty header",
			header:   "",
			expected: []string{},
		},
		{
			name:     "single scope",
			header:   "repo",
			expected: []string{"repo"},
		},
		{
			name:     "multiple scopes",
			header:   "repo, user, gist",
			expected: []string{"repo", "user", "gist"},
		},
		{
			name:     "scopes with extra whitespace",
			header:   "  repo  ,  user  ,  gist  ",
			expected: []string{"repo", "user", "gist"},
		},
		{
			name:     "scopes without spaces",
			header:   "repo,user,gist",
			expected: []string{"repo", "user", "gist"},
		},
		{
			name:     "scopes with colons",
			header:   "read:org, write:org, admin:org",
			expected: []string{"read:org", "write:org", "admin:org"},
		},
		{
			name:     "empty parts are filtered",
			header:   "repo,,gist",
			expected: []string{"repo", "gist"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseScopeHeader(tt.header)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFetcher_FetchTokenScopes(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectedScopes []string
		expectError    bool
		errorContains  string
	}{
		{
			name: "successful fetch with multiple scopes",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-OAuth-Scopes", "repo, user, gist")
				w.WriteHeader(http.StatusOK)
			},
			expectedScopes: []string{"repo", "user", "gist"},
			expectError:    false,
		},
		{
			name: "successful fetch with single scope",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-OAuth-Scopes", "repo")
				w.WriteHeader(http.StatusOK)
			},
			expectedScopes: []string{"repo"},
			expectError:    false,
		},
		{
			name: "fine-grained PAT returns empty scopes",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				// Fine-grained PATs don't return X-OAuth-Scopes
				w.WriteHeader(http.StatusOK)
			},
			expectedScopes: []string{},
			expectError:    false,
		},
		{
			name: "unauthorized token",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			expectError:   true,
			errorContains: "invalid or expired token",
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectError:   true,
			errorContains: "unexpected status code: 500",
		},
		{
			name: "verifies authorization header is set",
			handler: func(w http.ResponseWriter, r *http.Request) {
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.Header().Set("X-OAuth-Scopes", "repo")
				w.WriteHeader(http.StatusOK)
			},
			expectedScopes: []string{"repo"},
			expectError:    false,
		},
		{
			name: "verifies request method is HEAD",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodHead {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.Header().Set("X-OAuth-Scopes", "repo")
				w.WriteHeader(http.StatusOK)
			},
			expectedScopes: []string{"repo"},
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			fetcher := NewFetcher(FetcherOptions{
				APIHost: server.URL,
			})

			scopes, err := fetcher.FetchTokenScopes(context.Background(), "test-token")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedScopes, scopes)
			}
		})
	}
}

func TestFetcher_DefaultOptions(t *testing.T) {
	fetcher := NewFetcher(FetcherOptions{})

	// Verify default API host is set
	assert.Equal(t, "https://api.github.com", fetcher.apiHost)

	// Verify default HTTP client is set with timeout
	assert.NotNil(t, fetcher.client)
	assert.Equal(t, DefaultFetchTimeout, fetcher.client.Timeout)
}

func TestFetcher_CustomHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 5 * time.Second}

	fetcher := NewFetcher(FetcherOptions{
		HTTPClient: customClient,
	})

	assert.Equal(t, customClient, fetcher.client)
}

func TestFetcher_CustomAPIHost(t *testing.T) {
	fetcher := NewFetcher(FetcherOptions{
		APIHost: "https://api.github.enterprise.com",
	})

	assert.Equal(t, "https://api.github.enterprise.com", fetcher.apiHost)
}

func TestFetcher_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fetcher := NewFetcher(FetcherOptions{
		APIHost: server.URL,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := fetcher.FetchTokenScopes(ctx, "test-token")
	require.Error(t, err)
}
