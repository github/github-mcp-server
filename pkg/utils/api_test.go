package utils //nolint:revive //TODO: figure out a better name for this package

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthURL(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		host           string
		expectedOAuth  string
		expectError    bool
		errorSubstring string
	}{
		{
			name:          "dotcom (empty host)",
			host:          "",
			expectedOAuth: "https://github.com/login/oauth",
		},
		{
			name:          "dotcom (explicit github.com)",
			host:          "https://github.com",
			expectedOAuth: "https://github.com/login/oauth",
		},
		{
			name:          "GHEC with HTTPS",
			host:          "https://acme.ghe.com",
			expectedOAuth: "https://acme.ghe.com/login/oauth",
		},
		{
			name:           "GHEC with HTTP (should error)",
			host:           "http://acme.ghe.com",
			expectError:    true,
			errorSubstring: "GHEC URL must be HTTPS",
		},
		{
			name:          "GHES with HTTPS",
			host:          "https://ghes.example.com",
			expectedOAuth: "https://ghes.example.com/login/oauth",
		},
		{
			name:          "GHES with HTTP",
			host:          "http://ghes.example.com",
			expectedOAuth: "http://ghes.example.com/login/oauth",
		},
		{
			name:          "GHES with HTTP and custom port (port stripped - not supported yet)",
			host:          "http://ghes.local:8080",
			expectedOAuth: "http://ghes.local/login/oauth", // Port is stripped ref: ln222 api.go comment
		},
		{
			name:          "GHES with HTTPS and custom port (port stripped - not supported yet)",
			host:          "https://ghes.local:8443",
			expectedOAuth: "https://ghes.local/login/oauth", // Port is stripped ref: ln222 api.go comment
		},
		{
			name:           "host without scheme (should error)",
			host:           "ghes.example.com",
			expectError:    true,
			errorSubstring: "host must have a scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiHost, err := NewAPIHost(tt.host)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorSubstring != "" {
					assert.Contains(t, err.Error(), tt.errorSubstring)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, apiHost)

			oauthURL, err := apiHost.OAuthURL(ctx)
			require.NoError(t, err)
			require.NotNil(t, oauthURL)

			assert.Equal(t, tt.expectedOAuth, oauthURL.String())
		})
	}
}

func TestAPIHost_AllURLsHaveConsistentScheme(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		host           string
		expectedScheme string
	}{
		{
			name:           "GHES with HTTPS",
			host:           "https://ghes.example.com",
			expectedScheme: "https",
		},
		{
			name:           "GHES with HTTP",
			host:           "http://ghes.example.com",
			expectedScheme: "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiHost, err := NewAPIHost(tt.host)
			require.NoError(t, err)

			restURL, err := apiHost.BaseRESTURL(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedScheme, restURL.Scheme, "REST URL scheme should match")

			gqlURL, err := apiHost.GraphqlURL(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedScheme, gqlURL.Scheme, "GraphQL URL scheme should match")

			uploadURL, err := apiHost.UploadURL(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedScheme, uploadURL.Scheme, "Upload URL scheme should match")

			rawURL, err := apiHost.RawURL(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedScheme, rawURL.Scheme, "Raw URL scheme should match")

			oauthURL, err := apiHost.OAuthURL(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedScheme, oauthURL.Scheme, "OAuth URL scheme should match")
		})
	}
}
