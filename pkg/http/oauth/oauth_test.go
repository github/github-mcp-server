package oauth

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuthHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		cfg                  *Config
		expectedAuthServer   string
		expectedResourcePath string
	}{
		{
			name:                 "nil config uses defaults",
			cfg:                  nil,
			expectedAuthServer:   DefaultAuthorizationServer,
			expectedResourcePath: "",
		},
		{
			name:                 "empty config uses defaults",
			cfg:                  &Config{},
			expectedAuthServer:   DefaultAuthorizationServer,
			expectedResourcePath: "",
		},
		{
			name: "custom authorization server",
			cfg: &Config{
				AuthorizationServer: "https://custom.example.com/oauth",
			},
			expectedAuthServer:   "https://custom.example.com/oauth",
			expectedResourcePath: "",
		},
		{
			name: "custom base URL and resource path",
			cfg: &Config{
				BaseURL:      "https://example.com",
				ResourcePath: "/mcp",
			},
			expectedAuthServer:   DefaultAuthorizationServer,
			expectedResourcePath: "/mcp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, err := NewAuthHandler(tc.cfg)
			require.NoError(t, err)
			require.NotNil(t, handler)

			assert.Equal(t, tc.expectedAuthServer, handler.cfg.AuthorizationServer)
		})
	}
}

func TestGetEffectiveHostAndScheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		cfg            *Config
		expectedHost   string
		expectedScheme string
	}{
		{
			name: "basic request without forwarding headers",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Host = "example.com"
				return req
			},
			cfg:            &Config{},
			expectedHost:   "example.com",
			expectedScheme: "https", // defaults to https
		},
		{
			name: "request with X-Forwarded-Host header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Host = "internal.example.com"
				req.Header.Set(headers.ForwardedHostHeader, "public.example.com")
				return req
			},
			cfg:            &Config{},
			expectedHost:   "public.example.com",
			expectedScheme: "https",
		},
		{
			name: "request with X-Forwarded-Proto header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Host = "example.com"
				req.Header.Set(headers.ForwardedProtoHeader, "http")
				return req
			},
			cfg:            &Config{},
			expectedHost:   "example.com",
			expectedScheme: "http",
		},
		{
			name: "request with both forwarding headers",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Host = "internal.example.com"
				req.Header.Set(headers.ForwardedHostHeader, "public.example.com")
				req.Header.Set(headers.ForwardedProtoHeader, "https")
				return req
			},
			cfg:            &Config{},
			expectedHost:   "public.example.com",
			expectedScheme: "https",
		},
		{
			name: "request with TLS",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Host = "example.com"
				req.TLS = &tls.ConnectionState{}
				return req
			},
			cfg:            &Config{},
			expectedHost:   "example.com",
			expectedScheme: "https",
		},
		{
			name: "X-Forwarded-Proto takes precedence over TLS",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Host = "example.com"
				req.TLS = &tls.ConnectionState{}
				req.Header.Set(headers.ForwardedProtoHeader, "http")
				return req
			},
			cfg:            &Config{},
			expectedHost:   "example.com",
			expectedScheme: "http",
		},
		{
			name: "scheme is lowercased",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Host = "example.com"
				req.Header.Set(headers.ForwardedProtoHeader, "HTTPS")
				return req
			},
			cfg:            &Config{},
			expectedHost:   "example.com",
			expectedScheme: "https",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := tc.setupRequest()
			host, scheme := GetEffectiveHostAndScheme(req, tc.cfg)

			assert.Equal(t, tc.expectedHost, host)
			assert.Equal(t, tc.expectedScheme, scheme)
		})
	}
}

func TestGetEffectiveResourcePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupRequest func() *http.Request
		expectedPath string
	}{
		{
			name: "root path",
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedPath: "/",
		},
		{
			name: "mcp path",
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/mcp", nil)
			},
			expectedPath: "/mcp",
		},
		{
			name: "readonly path",
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/readonly", nil)
			},
			expectedPath: "/readonly",
		},
		{
			name: "nested path",
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/mcp/x/repos", nil)
			},
			expectedPath: "/mcp/x/repos",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := tc.setupRequest()
			path := GetEffectiveResourcePath(req)

			assert.Equal(t, tc.expectedPath, path)
		})
	}
}

func TestGetProtectedResourceData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		cfg                 *Config
		setupRequest        func() *http.Request
		resourcePath        string
		expectedResourceURL string
		expectedAuthServer  string
		expectError         bool
	}{
		{
			name: "basic request with root resource path",
			cfg:  &Config{},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Host = "api.example.com"
				return req
			},
			resourcePath:        "/",
			expectedResourceURL: "https://api.example.com/",
			expectedAuthServer:  DefaultAuthorizationServer,
		},
		{
			name: "basic request with custom resource path",
			cfg:  &Config{},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
				req.Host = "api.example.com"
				return req
			},
			resourcePath:        "/mcp",
			expectedResourceURL: "https://api.example.com/mcp",
			expectedAuthServer:  DefaultAuthorizationServer,
		},
		{
			name: "with custom base URL",
			cfg: &Config{
				BaseURL: "https://custom.example.com",
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
				req.Host = "api.example.com"
				return req
			},
			resourcePath:        "/mcp",
			expectedResourceURL: "https://custom.example.com/mcp",
			expectedAuthServer:  DefaultAuthorizationServer,
		},
		{
			name: "with custom authorization server",
			cfg: &Config{
				AuthorizationServer: "https://auth.example.com/oauth",
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
				req.Host = "api.example.com"
				return req
			},
			resourcePath:        "/mcp",
			expectedResourceURL: "https://api.example.com/mcp",
			expectedAuthServer:  "https://auth.example.com/oauth",
		},
		{
			name: "base URL with trailing slash is trimmed",
			cfg: &Config{
				BaseURL: "https://custom.example.com/",
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
				req.Host = "api.example.com"
				return req
			},
			resourcePath:        "/mcp",
			expectedResourceURL: "https://custom.example.com/mcp",
			expectedAuthServer:  DefaultAuthorizationServer,
		},
		{
			name: "nested resource path",
			cfg:  &Config{},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/mcp/x/repos", nil)
				req.Host = "api.example.com"
				return req
			},
			resourcePath:        "/mcp/x/repos",
			expectedResourceURL: "https://api.example.com/mcp/x/repos",
			expectedAuthServer:  DefaultAuthorizationServer,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, err := NewAuthHandler(tc.cfg)
			require.NoError(t, err)

			req := tc.setupRequest()
			data, err := handler.GetProtectedResourceData(req, tc.resourcePath)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedResourceURL, data.ResourceURL)
			assert.Equal(t, tc.expectedAuthServer, data.AuthorizationServer)
		})
	}
}

func TestBuildResourceMetadataURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cfg          *Config
		setupRequest func() *http.Request
		resourcePath string
		expectedURL  string
	}{
		{
			name: "root path",
			cfg:  &Config{},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Host = "api.example.com"
				return req
			},
			resourcePath: "/",
			expectedURL:  "https://api.example.com/.well-known/oauth-protected-resource",
		},
		{
			name: "with custom resource path",
			cfg:  &Config{},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
				req.Host = "api.example.com"
				return req
			},
			resourcePath: "/mcp",
			expectedURL:  "https://api.example.com/.well-known/oauth-protected-resource/mcp",
		},
		{
			name: "with base URL config",
			cfg: &Config{
				BaseURL: "https://custom.example.com",
			},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
				req.Host = "api.example.com"
				return req
			},
			resourcePath: "/mcp",
			expectedURL:  "https://custom.example.com/.well-known/oauth-protected-resource/mcp",
		},
		{
			name: "with forwarded headers",
			cfg:  &Config{},
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
				req.Host = "internal.example.com"
				req.Header.Set(headers.ForwardedHostHeader, "public.example.com")
				req.Header.Set(headers.ForwardedProtoHeader, "https")
				return req
			},
			resourcePath: "/mcp",
			expectedURL:  "https://public.example.com/.well-known/oauth-protected-resource/mcp",
		},
		{
			name: "nil config uses request host",
			cfg:  nil,
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Host = "api.example.com"
				return req
			},
			resourcePath: "",
			expectedURL:  "https://api.example.com/.well-known/oauth-protected-resource",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := tc.setupRequest()
			url := BuildResourceMetadataURL(req, tc.cfg, tc.resourcePath)

			assert.Equal(t, tc.expectedURL, url)
		})
	}
}

func TestHandleProtectedResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		cfg                *Config
		path               string
		host               string
		method             string
		expectedStatusCode int
		expectedScopes     []string
		validateResponse   func(t *testing.T, body map[string]any)
	}{
		{
			name: "GET request returns protected resource metadata",
			cfg: &Config{
				BaseURL: "https://api.example.com",
			},
			path:               OAuthProtectedResourcePrefix,
			host:               "api.example.com",
			method:             http.MethodGet,
			expectedStatusCode: http.StatusOK,
			expectedScopes:     SupportedScopes,
			validateResponse: func(t *testing.T, body map[string]any) {
				t.Helper()
				assert.Equal(t, "GitHub MCP Server", body["resource_name"])
				assert.Equal(t, "https://api.example.com", body["resource"])

				authServers, ok := body["authorization_servers"].([]any)
				require.True(t, ok)
				require.Len(t, authServers, 1)
				assert.Equal(t, DefaultAuthorizationServer, authServers[0])
			},
		},
		{
			name: "OPTIONS request for CORS preflight",
			cfg: &Config{
				BaseURL: "https://api.example.com",
			},
			path:               OAuthProtectedResourcePrefix,
			host:               "api.example.com",
			method:             http.MethodOptions,
			expectedStatusCode: http.StatusNoContent,
		},
		{
			name: "path with /mcp suffix",
			cfg: &Config{
				BaseURL: "https://api.example.com",
			},
			path:               OAuthProtectedResourcePrefix + "/mcp",
			host:               "api.example.com",
			method:             http.MethodGet,
			expectedStatusCode: http.StatusOK,
			validateResponse: func(t *testing.T, body map[string]any) {
				t.Helper()
				assert.Equal(t, "https://api.example.com/mcp", body["resource"])
			},
		},
		{
			name: "path with /readonly suffix",
			cfg: &Config{
				BaseURL: "https://api.example.com",
			},
			path:               OAuthProtectedResourcePrefix + "/readonly",
			host:               "api.example.com",
			method:             http.MethodGet,
			expectedStatusCode: http.StatusOK,
			validateResponse: func(t *testing.T, body map[string]any) {
				t.Helper()
				assert.Equal(t, "https://api.example.com/readonly", body["resource"])
			},
		},
		{
			name: "custom authorization server in response",
			cfg: &Config{
				BaseURL:             "https://api.example.com",
				AuthorizationServer: "https://custom.auth.example.com/oauth",
			},
			path:               OAuthProtectedResourcePrefix,
			host:               "api.example.com",
			method:             http.MethodGet,
			expectedStatusCode: http.StatusOK,
			validateResponse: func(t *testing.T, body map[string]any) {
				t.Helper()
				authServers, ok := body["authorization_servers"].([]any)
				require.True(t, ok)
				require.Len(t, authServers, 1)
				assert.Equal(t, "https://custom.auth.example.com/oauth", authServers[0])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, err := NewAuthHandler(tc.cfg)
			require.NoError(t, err)

			router := chi.NewRouter()
			handler.RegisterRoutes(router)

			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Host = tc.host

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedStatusCode, rec.Code)

			// Check CORS headers
			assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
			assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")
			assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "OPTIONS")

			if tc.method == http.MethodGet && tc.validateResponse != nil {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

				var body map[string]any
				err := json.Unmarshal(rec.Body.Bytes(), &body)
				require.NoError(t, err)

				tc.validateResponse(t, body)

				// Verify scopes if expected
				if tc.expectedScopes != nil {
					scopes, ok := body["scopes_supported"].([]any)
					require.True(t, ok)
					assert.Len(t, scopes, len(tc.expectedScopes))
				}
			}
		})
	}
}

func TestRegisterRoutes(t *testing.T) {
	t.Parallel()

	handler, err := NewAuthHandler(&Config{
		BaseURL: "https://api.example.com",
	})
	require.NoError(t, err)

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// List of expected routes that should be registered
	expectedRoutes := []string{
		OAuthProtectedResourcePrefix,
		OAuthProtectedResourcePrefix + "/",
		OAuthProtectedResourcePrefix + "/mcp",
		OAuthProtectedResourcePrefix + "/mcp/",
		OAuthProtectedResourcePrefix + "/readonly",
		OAuthProtectedResourcePrefix + "/readonly/",
		OAuthProtectedResourcePrefix + "/mcp/readonly",
		OAuthProtectedResourcePrefix + "/mcp/readonly/",
		OAuthProtectedResourcePrefix + "/x/repos",
		OAuthProtectedResourcePrefix + "/mcp/x/repos",
	}

	for _, route := range expectedRoutes {
		t.Run("route:"+route, func(t *testing.T) {
			// Test GET
			req := httptest.NewRequest(http.MethodGet, route, nil)
			req.Host = "api.example.com"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "GET %s should return 200", route)

			// Test OPTIONS (CORS preflight)
			req = httptest.NewRequest(http.MethodOptions, route, nil)
			req.Host = "api.example.com"
			rec = httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusNoContent, rec.Code, "OPTIONS %s should return 204", route)
		})
	}
}

func TestSupportedScopes(t *testing.T) {
	t.Parallel()

	// Verify all expected scopes are present
	expectedScopes := []string{
		"repo",
		"read:org",
		"read:user",
		"user:email",
		"read:packages",
		"write:packages",
		"read:project",
		"project",
		"gist",
		"notifications",
		"workflow",
		"codespace",
	}

	assert.Equal(t, expectedScopes, SupportedScopes)
}

func TestProtectedResourceResponseFormat(t *testing.T) {
	t.Parallel()

	handler, err := NewAuthHandler(&Config{
		BaseURL: "https://api.example.com",
	})
	require.NoError(t, err)

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, OAuthProtectedResourcePrefix, nil)
	req.Host = "api.example.com"

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify all required RFC 9728 fields are present
	assert.Contains(t, response, "resource")
	assert.Contains(t, response, "authorization_servers")
	assert.Contains(t, response, "bearer_methods_supported")
	assert.Contains(t, response, "scopes_supported")

	// Verify resource name (optional but we include it)
	assert.Contains(t, response, "resource_name")
	assert.Equal(t, "GitHub MCP Server", response["resource_name"])

	// Verify bearer_methods_supported contains "header"
	bearerMethods, ok := response["bearer_methods_supported"].([]any)
	require.True(t, ok)
	assert.Contains(t, bearerMethods, "header")

	// Verify authorization_servers is an array with GitHub OAuth
	authServers, ok := response["authorization_servers"].([]any)
	require.True(t, ok)
	assert.Len(t, authServers, 1)
	assert.Equal(t, DefaultAuthorizationServer, authServers[0])
}

func TestOAuthProtectedResourcePrefix(t *testing.T) {
	t.Parallel()

	// RFC 9728 specifies this well-known path
	assert.Equal(t, "/.well-known/oauth-protected-resource", OAuthProtectedResourcePrefix)
}

func TestDefaultAuthorizationServer(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://github.com/login/oauth", DefaultAuthorizationServer)
}
