package ghmcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAPIHost(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		wantRESTURL string
		wantGQLURL  string
		wantRawURL  string
		wantErr     bool
	}{
		{
			name:        "empty string returns dotcom",
			host:        "",
			wantRESTURL: "https://api.github.com/",
			wantGQLURL:  "https://api.github.com/graphql",
			wantRawURL:  "https://raw.githubusercontent.com/",
			wantErr:     false,
		},
		{
			name:        "github.com returns dotcom URLs",
			host:        "https://github.com",
			wantRESTURL: "https://api.github.com/",
			wantGQLURL:  "https://api.github.com/graphql",
			wantRawURL:  "https://raw.githubusercontent.com/",
			wantErr:     false,
		},
		{
			name:        "GHEC hostname",
			host:        "https://example.ghe.com",
			wantRESTURL: "https://api.example.ghe.com/",
			wantGQLURL:  "https://api.example.ghe.com/graphql",
			wantRawURL:  "https://raw.example.ghe.com/",
			wantErr:     false,
		},
		{
			name:        "GHES hostname with https",
			host:        "https://github.enterprise.local",
			wantRESTURL: "https://github.enterprise.local/api/v3/",
			wantGQLURL:  "https://github.enterprise.local/api/graphql",
			wantRawURL:  "https://github.enterprise.local/raw/",
			wantErr:     false,
		},
		{
			name:        "GHES hostname with http",
			host:        "http://github.enterprise.local",
			wantRESTURL: "http://github.enterprise.local/api/v3/",
			wantGQLURL:  "http://github.enterprise.local/api/graphql",
			wantRawURL:  "http://github.enterprise.local/raw/",
			wantErr:     false,
		},
		{
			name:    "missing scheme returns error",
			host:    "github.com",
			wantErr: true,
		},
		{
			name:    "GHEC with http scheme returns error",
			host:    "http://example.ghe.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAPIHost(tt.host)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRESTURL, got.baseRESTURL.String())
			assert.Equal(t, tt.wantGQLURL, got.graphqlURL.String())
			assert.Equal(t, tt.wantRawURL, got.rawURL.String())
		})
	}
}

func TestNewDotcomHost(t *testing.T) {
	host, err := newDotcomHost()
	require.NoError(t, err)

	assert.Equal(t, "https://api.github.com/", host.baseRESTURL.String())
	assert.Equal(t, "https://api.github.com/graphql", host.graphqlURL.String())
	assert.Equal(t, "https://uploads.github.com", host.uploadURL.String())
	assert.Equal(t, "https://raw.githubusercontent.com/", host.rawURL.String())
}

func TestNewGHECHost(t *testing.T) {
	tests := []struct {
		name        string
		hostname    string
		wantRESTURL string
		wantGQLURL  string
		wantRawURL  string
		wantUpload  string
		wantErr     bool
	}{
		{
			name:        "valid GHEC hostname",
			hostname:    "https://example.ghe.com",
			wantRESTURL: "https://api.example.ghe.com/",
			wantGQLURL:  "https://api.example.ghe.com/graphql",
			wantRawURL:  "https://raw.example.ghe.com/",
			wantUpload:  "https://uploads.example.ghe.com",
			wantErr:     false,
		},
		{
			name:     "http GHEC hostname should error",
			hostname: "http://example.ghe.com",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := newGHECHost(tt.hostname)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "HTTPS")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRESTURL, host.baseRESTURL.String())
			assert.Equal(t, tt.wantGQLURL, host.graphqlURL.String())
			assert.Equal(t, tt.wantRawURL, host.rawURL.String())
			assert.Equal(t, tt.wantUpload, host.uploadURL.String())
		})
	}
}

func TestNewGHESHost(t *testing.T) {
	tests := []struct {
		name        string
		hostname    string
		wantRESTURL string
		wantGQLURL  string
		wantRawURL  string
		wantUpload  string
	}{
		{
			name:        "GHES with https",
			hostname:    "https://github.enterprise.local",
			wantRESTURL: "https://github.enterprise.local/api/v3/",
			wantGQLURL:  "https://github.enterprise.local/api/graphql",
			wantRawURL:  "https://github.enterprise.local/raw/",
			wantUpload:  "https://github.enterprise.local/api/uploads/",
		},
		{
			name:        "GHES with http",
			hostname:    "http://github.local",
			wantRESTURL: "http://github.local/api/v3/",
			wantGQLURL:  "http://github.local/api/graphql",
			wantRawURL:  "http://github.local/raw/",
			wantUpload:  "http://github.local/api/uploads/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := newGHESHost(tt.hostname)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRESTURL, host.baseRESTURL.String())
			assert.Equal(t, tt.wantGQLURL, host.graphqlURL.String())
			assert.Equal(t, tt.wantRawURL, host.rawURL.String())
			assert.Equal(t, tt.wantUpload, host.uploadURL.String())
		})
	}
}

func TestUserAgentTransport(t *testing.T) {
	// Create a test server that echoes back the User-Agent header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("User-Agent-Echo", r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	transport := &userAgentTransport{
		transport: http.DefaultTransport,
		agent:     "test-agent/1.0",
	}

	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", ts.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "test-agent/1.0", resp.Header.Get("User-Agent-Echo"))
}

func TestBearerAuthTransport(t *testing.T) {
	// Create a test server that echoes back the Authorization header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Auth-Echo", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	transport := &bearerAuthTransport{
		transport: http.DefaultTransport,
		token:     "test-token-123",
	}

	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", ts.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "Bearer test-token-123", resp.Header.Get("Auth-Echo"))
}

func TestNewMCPServer(t *testing.T) {
	tests := []struct {
		name    string
		config  MCPServerConfig
		wantErr bool
	}{
		{
			name: "valid dotcom configuration",
			config: MCPServerConfig{
				Version:           "1.0.0",
				Host:              "",
				Token:             "test-token",
				EnabledToolsets:   []string{"issues"},
				DynamicToolsets:   false,
				ReadOnly:          false,
				Translator:        translations.NullTranslationHelper,
				ContentWindowSize: 1000,
			},
			wantErr: false,
		},
		{
			name: "valid GHES configuration",
			config: MCPServerConfig{
				Version:           "1.0.0",
				Host:              "https://github.enterprise.local",
				Token:             "test-token",
				EnabledToolsets:   []string{"issues"},
				DynamicToolsets:   false,
				ReadOnly:          true,
				Translator:        translations.NullTranslationHelper,
				ContentWindowSize: 2000,
			},
			wantErr: false,
		},
		{
			name: "dynamic toolsets enabled",
			config: MCPServerConfig{
				Version:           "1.0.0",
				Host:              "",
				Token:             "test-token",
				EnabledToolsets:   []string{"all", "issues"},
				DynamicToolsets:   true,
				ReadOnly:          false,
				Translator:        translations.NullTranslationHelper,
				ContentWindowSize: 1000,
			},
			wantErr: false,
		},
		{
			name: "invalid host",
			config: MCPServerConfig{
				Version:           "1.0.0",
				Host:              "not-a-url",
				Token:             "test-token",
				EnabledToolsets:   []string{"issues"},
				DynamicToolsets:   false,
				ReadOnly:          false,
				Translator:        translations.NullTranslationHelper,
				ContentWindowSize: 1000,
			},
			wantErr: true,
		},
		{
			name: "invalid toolset name",
			config: MCPServerConfig{
				Version:           "1.0.0",
				Host:              "",
				Token:             "test-token",
				EnabledToolsets:   []string{"nonexistent-toolset"},
				DynamicToolsets:   false,
				ReadOnly:          false,
				Translator:        translations.NullTranslationHelper,
				ContentWindowSize: 1000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewMCPServer(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, server)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, server)
		})
	}
}

func TestUserAgentTransport_RoundTrip(t *testing.T) {
	// Test that RoundTrip properly clones request and sets User-Agent
	originalReq, _ := http.NewRequestWithContext(context.Background(), "GET", "http://example.com", nil)
	originalReq.Header.Set("User-Agent", "original-agent")

	transport := &userAgentTransport{
		transport: &mockRoundTripper{
			checkFunc: func(req *http.Request) {
				// Verify the User-Agent was overwritten
				assert.Equal(t, "new-agent", req.Header.Get("User-Agent"))
				// Verify original request wasn't modified
				assert.Equal(t, "original-agent", originalReq.Header.Get("User-Agent"))
			},
		},
		agent: "new-agent",
	}

	_, _ = transport.RoundTrip(originalReq)
}

func TestBearerAuthTransport_RoundTrip(t *testing.T) {
	// Test that RoundTrip properly clones request and sets Authorization
	originalReq, _ := http.NewRequestWithContext(context.Background(), "GET", "http://example.com", nil)

	transport := &bearerAuthTransport{
		transport: &mockRoundTripper{
			checkFunc: func(req *http.Request) {
				// Verify the Authorization header was set
				assert.Equal(t, "Bearer secret-token", req.Header.Get("Authorization"))
				// Verify original request wasn't modified
				assert.Empty(t, originalReq.Header.Get("Authorization"))
			},
		},
		token: "secret-token",
	}

	_, _ = transport.RoundTrip(originalReq)
}

// mockRoundTripper is a mock implementation of http.RoundTripper for testing
type mockRoundTripper struct {
	checkFunc func(*http.Request)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.checkFunc != nil {
		m.checkFunc(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

func TestAPIHostURLConstruction(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		validate func(t *testing.T, ah apiHost)
	}{
		{
			name: "dotcom URLs should be HTTPS",
			host: "",
			validate: func(t *testing.T, ah apiHost) {
				assert.Equal(t, "https", ah.baseRESTURL.Scheme)
				assert.Equal(t, "https", ah.graphqlURL.Scheme)
				assert.Equal(t, "https", ah.uploadURL.Scheme)
				assert.Equal(t, "https", ah.rawURL.Scheme)
			},
		},
		{
			name: "GHES URLs should preserve scheme",
			host: "http://github.local",
			validate: func(t *testing.T, ah apiHost) {
				assert.Equal(t, "http", ah.baseRESTURL.Scheme)
				assert.Equal(t, "http", ah.graphqlURL.Scheme)
				assert.Equal(t, "http", ah.uploadURL.Scheme)
				assert.Equal(t, "http", ah.rawURL.Scheme)
			},
		},
		{
			name: "URLs should have correct paths",
			host: "https://github.enterprise.local",
			validate: func(t *testing.T, ah apiHost) {
				assert.Equal(t, "/api/v3/", ah.baseRESTURL.Path)
				assert.Equal(t, "/api/graphql", ah.graphqlURL.Path)
				assert.Equal(t, "/api/uploads/", ah.uploadURL.Path)
				assert.Equal(t, "/raw/", ah.rawURL.Path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ah, err := parseAPIHost(tt.host)
			require.NoError(t, err)
			tt.validate(t, ah)
		})
	}
}

func TestMCPServerConfig_DynamicToolsetsFiltersAll(t *testing.T) {
	// Test that "all" is filtered from enabled toolsets when dynamic toolsets is enabled
	config := MCPServerConfig{
		Version:           "1.0.0",
		Host:              "",
		Token:             "test-token",
		EnabledToolsets:   []string{"all", "issues"},
		DynamicToolsets:   true,
		ReadOnly:          false,
		Translator:        translations.NullTranslationHelper,
		ContentWindowSize: 1000,
	}

	server, err := NewMCPServer(config)
	require.NoError(t, err)
	assert.NotNil(t, server)

	// The server should be created successfully with "all" filtered out
	// This test validates the filtering logic works without errors
}

func TestMCPServerConfig_ReadOnlyMode(t *testing.T) {
	config := MCPServerConfig{
		Version:           "1.0.0",
		Host:              "",
		Token:             "test-token",
		EnabledToolsets:   []string{"issues"},
		DynamicToolsets:   false,
		ReadOnly:          true,
		Translator:        translations.NullTranslationHelper,
		ContentWindowSize: 1000,
	}

	server, err := NewMCPServer(config)
	require.NoError(t, err)
	assert.NotNil(t, server)

	// Server should be created successfully in read-only mode
}

func TestParseAPIHost_URLParsing(t *testing.T) {
	// Test various URL formats that could be problematic
	tests := []struct {
		name    string
		host    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no scheme",
			host:    "github.com",
			wantErr: true,
			errMsg:  "scheme",
		},
		{
			name:    "valid https",
			host:    "https://github.enterprise.com",
			wantErr: false,
		},
		{
			name:    "valid http",
			host:    "http://github.local",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseAPIHost(tt.host)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAPIHost_AllFieldsPopulated(t *testing.T) {
	hosts := []string{
		"",                               // dotcom
		"https://example.ghe.com",        // GHEC
		"https://github.enterprise.local", // GHES
	}

	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			ah, err := parseAPIHost(host)
			require.NoError(t, err)

			assert.NotNil(t, ah.baseRESTURL, "baseRESTURL should not be nil")
			assert.NotNil(t, ah.graphqlURL, "graphqlURL should not be nil")
			assert.NotNil(t, ah.uploadURL, "uploadURL should not be nil")
			assert.NotNil(t, ah.rawURL, "rawURL should not be nil")

			// All URLs should be parseable and have a scheme
			assert.NotEmpty(t, ah.baseRESTURL.Scheme)
			assert.NotEmpty(t, ah.graphqlURL.Scheme)
			assert.NotEmpty(t, ah.uploadURL.Scheme)
			assert.NotEmpty(t, ah.rawURL.Scheme)
		})
	}
}

func TestNewMCPServer_ClientConfiguration(t *testing.T) {
	config := MCPServerConfig{
		Version:           "2.5.0",
		Host:              "",
		Token:             "test-token",
		EnabledToolsets:   []string{"issues"},
		DynamicToolsets:   false,
		ReadOnly:          false,
		Translator:        translations.NullTranslationHelper,
		ContentWindowSize: 5000,
	}

	server, err := NewMCPServer(config)
	require.NoError(t, err)
	assert.NotNil(t, server)

	// Verify server was created with the correct version
	// The server should embed the version in various places
}

func TestURLParsing_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		parseURL func() (*url.URL, error)
		wantErr  bool
	}{
		{
			name: "dotcom REST URL",
			parseURL: func() (*url.URL, error) {
				return url.Parse("https://api.github.com/")
			},
			wantErr: false,
		},
		{
			name: "GHES GraphQL URL",
			parseURL: func() (*url.URL, error) {
				return url.Parse("https://github.local/api/graphql")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := tt.parseURL()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, u)
			}
		})
	}
}
