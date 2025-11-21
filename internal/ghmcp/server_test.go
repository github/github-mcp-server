package ghmcp

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseAPIHost_GitHubDotCom(t *testing.T) {
	tests := []struct {
		name                 string
		host                 string
		expectedRESTURL      string
		expectedGraphQLURL   string
		expectedUploadURL    string
		expectError          bool
	}{
		{
			name:                 "github.com without scheme should error",
			host:                 "github.com",
			expectError:          true,
		},
		{
			name:                 "https://github.com",
			host:                 "https://github.com",
			expectedRESTURL:      "https://api.github.com/",
			expectedGraphQLURL:   "https://api.github.com/graphql",
			expectedUploadURL:    "https://uploads.github.com",
			expectError:          false,
		},
		{
			name:                 "https://www.github.com",
			host:                 "https://www.github.com",
			expectedRESTURL:      "https://api.github.com/",
			expectedGraphQLURL:   "https://api.github.com/graphql",
			expectedUploadURL:    "https://uploads.github.com",
			expectError:          false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseAPIHost(tc.host)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tc.expectedRESTURL, result.baseRESTURL.String())
			assert.Equal(t, tc.expectedGraphQLURL, result.graphqlURL.String())
			assert.Equal(t, tc.expectedUploadURL, result.uploadURL.String())
		})
	}
}

func Test_parseAPIHost_GHEC(t *testing.T) {
	tests := []struct {
		name                 string
		host                 string
		expectedRESTURL      string
		expectedGraphQLURL   string
		expectedUploadURL    string
		expectError          bool
	}{
		{
			name:                 "GHEC without scheme should error",
			host:                 "subdomain.ghe.com",
			expectError:          true,
		},
		{
			name:                 "GHEC with https prefix",
			host:                 "https://subdomain.ghe.com",
			expectedRESTURL:      "https://api.subdomain.ghe.com/",
			expectedGraphQLURL:   "https://api.subdomain.ghe.com/graphql",
			expectedUploadURL:    "https://uploads.subdomain.ghe.com",
			expectError:          false,
		},
		{
			name:                 "GHEC company.ghe.com",
			host:                 "https://company.ghe.com",
			expectedRESTURL:      "https://api.company.ghe.com/",
			expectedGraphQLURL:   "https://api.company.ghe.com/graphql",
			expectedUploadURL:    "https://uploads.company.ghe.com",
			expectError:          false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseAPIHost(tc.host)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tc.expectedRESTURL, result.baseRESTURL.String())
			assert.Equal(t, tc.expectedGraphQLURL, result.graphqlURL.String())
			assert.Equal(t, tc.expectedUploadURL, result.uploadURL.String())
		})
	}
}

func Test_parseAPIHost_GHES(t *testing.T) {
	tests := []struct {
		name                 string
		host                 string
		expectedRESTURL      string
		expectedGraphQLURL   string
		expectedUploadURL    string
		expectError          bool
	}{
		{
			name:                 "GHES with https prefix",
			host:                 "https://github.enterprise.com",
			expectedRESTURL:      "https://github.enterprise.com/api/v3/",
			expectedGraphQLURL:   "https://github.enterprise.com/api/graphql",
			expectedUploadURL:    "https://github.enterprise.com/api/uploads/",
			expectError:          false,
		},
		{
			name:                 "GHES company domain",
			host:                 "https://git.company.com",
			expectedRESTURL:      "https://git.company.com/api/v3/",
			expectedGraphQLURL:   "https://git.company.com/api/graphql",
			expectedUploadURL:    "https://git.company.com/api/uploads/",
			expectError:          false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseAPIHost(tc.host)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tc.expectedRESTURL, result.baseRESTURL.String())
			assert.Equal(t, tc.expectedGraphQLURL, result.graphqlURL.String())
			assert.Equal(t, tc.expectedUploadURL, result.uploadURL.String())
		})
	}
}

func Test_parseAPIHost_EmptyString(t *testing.T) {
	// Empty string should default to github.com
	result, err := parseAPIHost("")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "https://api.github.com/", result.baseRESTURL.String())
	assert.Equal(t, "https://api.github.com/graphql", result.graphqlURL.String())
	assert.Equal(t, "https://uploads.github.com", result.uploadURL.String())
}

func Test_newDotcomHost(t *testing.T) {
	host, err := newDotcomHost()

	require.NoError(t, err)
	require.NotNil(t, host)
	assert.Equal(t, "https://api.github.com/", host.baseRESTURL.String())
	assert.Equal(t, "https://api.github.com/graphql", host.graphqlURL.String())
	assert.Equal(t, "https://uploads.github.com", host.uploadURL.String())
}

func Test_newGHECHost(t *testing.T) {
	tests := []struct {
		name                string
		hostname            string
		expectedRESTURL     string
		expectedGraphQLURL  string
		expectedUploadURL   string
	}{
		{
			name:                "basic GHEC host",
			hostname:            "https://mycompany.ghe.com",
			expectedRESTURL:     "https://api.mycompany.ghe.com/",
			expectedGraphQLURL:  "https://api.mycompany.ghe.com/graphql",
			expectedUploadURL:   "https://uploads.mycompany.ghe.com",
		},
		{
			name:                "another GHEC host",
			hostname:            "https://test.ghe.com",
			expectedRESTURL:     "https://api.test.ghe.com/",
			expectedGraphQLURL:  "https://api.test.ghe.com/graphql",
			expectedUploadURL:   "https://uploads.test.ghe.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, err := newGHECHost(tc.hostname)

			require.NoError(t, err)
			require.NotNil(t, host)
			assert.Equal(t, tc.expectedRESTURL, host.baseRESTURL.String())
			assert.Equal(t, tc.expectedGraphQLURL, host.graphqlURL.String())
			assert.Equal(t, tc.expectedUploadURL, host.uploadURL.String())
		})
	}
}

func Test_newGHESHost(t *testing.T) {
	tests := []struct {
		name                string
		hostname            string
		expectedRESTURL     string
		expectedGraphQLURL  string
		expectedUploadURL   string
	}{
		{
			name:                "basic GHES host",
			hostname:            "https://github.company.com",
			expectedRESTURL:     "https://github.company.com/api/v3/",
			expectedGraphQLURL:  "https://github.company.com/api/graphql",
			expectedUploadURL:   "https://github.company.com/api/uploads/",
		},
		{
			name:                "GHES with subdomain",
			hostname:            "https://git.enterprise.local",
			expectedRESTURL:     "https://git.enterprise.local/api/v3/",
			expectedGraphQLURL:  "https://git.enterprise.local/api/graphql",
			expectedUploadURL:   "https://git.enterprise.local/api/uploads/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, err := newGHESHost(tc.hostname)

			require.NoError(t, err)
			require.NotNil(t, host)
			assert.Equal(t, tc.expectedRESTURL, host.baseRESTURL.String())
			assert.Equal(t, tc.expectedGraphQLURL, host.graphqlURL.String())
			assert.Equal(t, tc.expectedUploadURL, host.uploadURL.String())
		})
	}
}

func Test_parseAPIHost_InvalidURLs(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{
			name: "invalid URL with spaces",
			host: "invalid url with spaces",
		},
		{
			name: "URL with invalid characters",
			host: "https://github.com/\x00",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseAPIHost(tc.host)
			require.Error(t, err)
		})
	}
}

func Test_bearerAuthTransport_RoundTrip(t *testing.T) {
	// Create a mock round tripper that captures the request
	capturedReq := ""
	mockTransport := &mockRoundTripper{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			capturedReq = req.Header.Get("Authorization")
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}

	transport := &bearerAuthTransport{
		transport: mockTransport,
		token:     "test-token-123",
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.NoError(t, err)

	assert.Equal(t, "Bearer test-token-123", capturedReq)
}

func Test_userAgentTransport_RoundTrip(t *testing.T) {
	// Create a mock round tripper that captures the request
	capturedAgent := ""
	mockTransport := &mockRoundTripper{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			capturedAgent = req.Header.Get("User-Agent")
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		},
	}

	transport := &userAgentTransport{
		transport: mockTransport,
		agent:     "test-agent/1.0",
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.NoError(t, err)

	assert.Equal(t, "test-agent/1.0", capturedAgent)
}

// Mock round tripper for testing
type mockRoundTripper struct {
	roundTripFunc func(*http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}
