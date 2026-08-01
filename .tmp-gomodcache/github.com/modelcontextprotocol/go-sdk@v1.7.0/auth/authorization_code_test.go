// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/modelcontextprotocol/go-sdk/internal/oauthtest"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

func TestAuthorize(t *testing.T) {
	authServer := oauthtest.NewFakeAuthorizationServer(oauthtest.Config{
		RegistrationConfig: &oauthtest.RegistrationConfig{
			PreregisteredClients: map[string]oauthtest.ClientInfo{
				"test_client_id": {
					Secret:       "test_client_secret",
					RedirectURIs: []string{"http://localhost:12345/callback"},
				},
			},
		},
	})
	authServer.Start(t)

	resourceMux := http.NewServeMux()
	resourceServer := httptest.NewServer(resourceMux)
	t.Cleanup(resourceServer.Close)
	resourceURL := resourceServer.URL + "/resource"

	resourceMux.Handle("/.well-known/oauth-protected-resource/resource", ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:             resourceURL,
		AuthorizationServers: []string{authServer.URL()},
	}))

	handler, err := NewAuthorizationCodeHandler(&AuthorizationCodeHandlerConfig{
		RedirectURL: "http://localhost:12345/callback",
		PreregisteredClient: &oauthex.ClientCredentials{
			ClientID: "test_client_id",
			ClientSecretAuth: &oauthex.ClientSecretAuth{
				ClientSecret: "test_client_secret",
			},
		},
		AuthorizationCodeFetcher: func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
			// The fake authorization server will redirect to an URL with code and state.
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			resp, err := client.Get(args.URL)
			if err != nil {
				return nil, fmt.Errorf("failed to visit auth URL: %v", err)
			}
			defer resp.Body.Close()
			dump, err := httputil.DumpResponse(resp, true)
			if err != nil {
				t.Fatalf("failed to dump response: %v", err)
			}
			t.Log(string(dump))

			location, err := resp.Location()
			if err != nil {
				return nil, fmt.Errorf("failed to get location header: %v", err)
			}
			return &AuthorizationResult{
				Code:  location.Query().Get("code"),
				State: location.Query().Get("state"),
				Iss:   location.Query().Get("iss"),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewAuthorizationCodeHandler failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, resourceURL, nil)
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    req,
	}
	resp.Header.Set(
		"WWW-Authenticate",
		"Bearer resource_metadata="+resourceServer.URL+"/.well-known/oauth-protected-resource/resource",
	)

	if err := handler.Authorize(context.Background(), req, resp); err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenSource, err := handler.TokenSource(t.Context())
	if err != nil {
		t.Fatalf("Failed to get token source: %v", err)
	}
	token, err := tokenSource.Token()
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}
	if token.AccessToken != "test_access_token" {
		t.Errorf("Expected access token 'test_access_token', got '%s'", token.AccessToken)
	}
}

// TestAuthorize_RefreshAfterContextCancel verifies that the token source built
// by Authorize keeps refreshing after the context passed to Authorize is
// cancelled. The token source is stored on the handler and used by the
// transport for the whole connection lifetime, whereas Authorize is typically
// called from a request- or connect-scoped context. golang.org/x/oauth2
// captures the context handed to Config.TokenSource and reuses it for every
// refresh, so binding it to the request context made every refresh after the
// access token expired fail with "context canceled". Regression test for that.
func TestAuthorize_RefreshAfterContextCancel(t *testing.T) {
	authServer := oauthtest.NewFakeAuthorizationServer(oauthtest.Config{
		// expires_in below oauth2's 10s expiry delta, so the reuse token source
		// treats the access token as expired immediately and must refresh.
		AccessTokenTTL:    1,
		IssueRefreshToken: true,
		RegistrationConfig: &oauthtest.RegistrationConfig{
			PreregisteredClients: map[string]oauthtest.ClientInfo{
				"test_client_id": {
					Secret:       "test_client_secret",
					RedirectURIs: []string{"http://localhost:12345/callback"},
				},
			},
		},
	})
	authServer.Start(t)

	resourceMux := http.NewServeMux()
	resourceServer := httptest.NewServer(resourceMux)
	t.Cleanup(resourceServer.Close)
	resourceURL := resourceServer.URL + "/resource"
	resourceMux.Handle("/.well-known/oauth-protected-resource/resource", ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:             resourceURL,
		AuthorizationServers: []string{authServer.URL()},
	}))

	handler, err := NewAuthorizationCodeHandler(&AuthorizationCodeHandlerConfig{
		RedirectURL: "http://localhost:12345/callback",
		PreregisteredClient: &oauthex.ClientCredentials{
			ClientID:         "test_client_id",
			ClientSecretAuth: &oauthex.ClientSecretAuth{ClientSecret: "test_client_secret"},
		},
		AuthorizationCodeFetcher: func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
			client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			resp, err := client.Get(args.URL)
			if err != nil {
				return nil, fmt.Errorf("failed to visit auth URL: %v", err)
			}
			defer resp.Body.Close()
			location, err := resp.Location()
			if err != nil {
				return nil, fmt.Errorf("failed to get location header: %v", err)
			}
			return &AuthorizationResult{
				Code:  location.Query().Get("code"),
				State: location.Query().Get("state"),
				Iss:   location.Query().Get("iss"),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewAuthorizationCodeHandler failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, resourceURL, nil)
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    req,
	}
	resp.Header.Set("WWW-Authenticate", "Bearer resource_metadata="+resourceServer.URL+"/.well-known/oauth-protected-resource/resource")

	// Authorize under a context that we cancel immediately afterwards, mimicking
	// the request/connect context the transport passes and that is already done
	// by the time a later token refresh runs.
	ctx, cancel := context.WithCancel(context.Background())
	if err := handler.Authorize(ctx, req, resp); err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}
	cancel()

	tokenSource, err := handler.TokenSource(context.Background())
	if err != nil {
		t.Fatalf("Failed to get token source: %v", err)
	}
	// The access token is already expired, so this forces a refresh round-trip.
	// It must not fail with "context canceled" from the cancelled Authorize ctx.
	token, err := tokenSource.Token()
	if err != nil {
		t.Fatalf("token refresh after Authorize context cancellation failed: %v", err)
	}
	if token.AccessToken != "test_access_token_refreshed" {
		t.Errorf("expected refreshed access token %q, got %q", "test_access_token_refreshed", token.AccessToken)
	}
}

func TestAuthorize_ScopeAccumulation(t *testing.T) {
	authServer := oauthtest.NewFakeAuthorizationServer(oauthtest.Config{
		RegistrationConfig: &oauthtest.RegistrationConfig{
			PreregisteredClients: map[string]oauthtest.ClientInfo{
				"test_client_id": {
					Secret:       "test_client_secret",
					RedirectURIs: []string{"http://localhost:12345/callback"},
				},
			},
		},
		TokenScopeFunc: func(requestedScope string) string {
			// Simulate a server that never grants "write".
			var granted []string
			for _, s := range strings.Fields(requestedScope) {
				if s != "write" {
					granted = append(granted, s)
				}
			}
			return strings.Join(granted, " ")
		},
	})
	authServer.Start(t)

	resourceMux := http.NewServeMux()
	resourceServer := httptest.NewServer(resourceMux)
	t.Cleanup(resourceServer.Close)
	resourceURL := resourceServer.URL + "/resource"

	resourceMux.Handle("/.well-known/oauth-protected-resource/resource", ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:             resourceURL,
		AuthorizationServers: []string{authServer.URL()},
	}))

	var capturedAuthURLs []string
	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	handler, err := NewAuthorizationCodeHandler(&AuthorizationCodeHandlerConfig{
		RedirectURL: "http://localhost:12345/callback",
		PreregisteredClient: &oauthex.ClientCredentials{
			ClientID: "test_client_id",
			ClientSecretAuth: &oauthex.ClientSecretAuth{
				ClientSecret: "test_client_secret",
			},
		},
		AuthorizationCodeFetcher: func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
			capturedAuthURLs = append(capturedAuthURLs, args.URL)
			resp, err := noRedirectClient.Get(args.URL)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			loc, err := resp.Location()
			if err != nil {
				return nil, err
			}
			return &AuthorizationResult{
				Code:  loc.Query().Get("code"),
				State: loc.Query().Get("state"),
				Iss:   loc.Query().Get("iss"),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewAuthorizationCodeHandler failed: %v", err)
	}

	// First authorization: 401 with scope="read write".
	// The token response will only grant "read" (TokenScopeFunc strips "write").
	req := httptest.NewRequest(http.MethodGet, resourceURL, nil)
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     make(http.Header),
		Body:       http.NoBody,
	}
	resp.Header.Set("WWW-Authenticate",
		fmt.Sprintf(`Bearer scope="read write", resource_metadata="%s/.well-known/oauth-protected-resource/resource"`, resourceServer.URL))
	if err := handler.Authorize(context.Background(), req, resp); err != nil {
		t.Fatalf("First Authorize failed: %v", err)
	}

	// Verify first auth URL requested "read" and "write".
	firstURL, err := url.Parse(capturedAuthURLs[0])
	if err != nil {
		t.Fatalf("Failed to parse first auth URL: %v", err)
	}
	firstScopes := strings.Fields(firstURL.Query().Get("scope"))
	if diff := cmp.Diff([]string{"read", "write"}, firstScopes, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("First auth scopes mismatch (-want +got):\n%s", diff)
	}

	// Verify only "read" was granted (the token omitted "write").
	issuer := authServer.URL()
	if diff := cmp.Diff([]string{"read"}, handler.grantedScopes[issuer], cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("After first Authorize, grantedScopes mismatch (-want +got):\n%s", diff)
	}

	// Second authorization: 403 insufficient_scope with scope="admin".
	// Accumulated scopes should be "read" (previously granted) + "admin" (new).
	req2 := httptest.NewRequest(http.MethodGet, resourceURL, nil)
	resp2 := &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     make(http.Header),
		Body:       http.NoBody,
	}
	resp2.Header.Set("WWW-Authenticate",
		fmt.Sprintf(`Bearer error="insufficient_scope", scope="admin", resource_metadata="%s/.well-known/oauth-protected-resource/resource"`, resourceServer.URL))
	if err := handler.Authorize(context.Background(), req2, resp2); err != nil {
		t.Fatalf("Second Authorize failed: %v", err)
	}

	// Verify second auth URL accumulated "read" (granted) + "admin" (challenged),
	// but NOT "write" (requested but never granted).
	secondURL, err := url.Parse(capturedAuthURLs[1])
	if err != nil {
		t.Fatalf("Failed to parse second auth URL: %v", err)
	}
	secondScopes := strings.Fields(secondURL.Query().Get("scope"))
	if diff := cmp.Diff([]string{"admin", "read"}, secondScopes, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("Second auth scopes mismatch (-want +got):\n%s", diff)
	}
}

func TestAuthorize_ForbiddenUnhandledError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/resource", nil)
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    req,
	}
	resp.Header.Set(
		"WWW-Authenticate",
		"Bearer error=invalid_token",
	)
	handler, err := NewAuthorizationCodeHandler(validConfig())
	if err != nil {
		t.Fatalf("NewAuthorizationCodeHandler failed: %v", err)
	}
	err = handler.Authorize(t.Context(), req, resp)
	if err != nil {
		t.Fatalf("Authorize() failed: %v", err)
	}
}

func TestNewAuthorizationCodeHandler_Success(t *testing.T) {
	simpleHandler := func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
		return nil, nil
	}
	tests := []struct {
		name   string
		config *AuthorizationCodeHandlerConfig
	}{
		{
			name: "ClientIDMetadataDocumentConfig",
			config: &AuthorizationCodeHandlerConfig{
				ClientIDMetadataDocumentConfig: &ClientIDMetadataDocumentConfig{URL: "https://example.com/client"},
				RedirectURL:                    "https://example.com/callback",
				AuthorizationCodeFetcher:       simpleHandler,
			},
		},
		{
			name: "PreregisteredClientConfig",
			config: &AuthorizationCodeHandlerConfig{
				PreregisteredClient: &oauthex.ClientCredentials{
					ClientID: "test_client_id",
					ClientSecretAuth: &oauthex.ClientSecretAuth{
						ClientSecret: "test_client_secret",
					},
				},
				RedirectURL:              "https://example.com/callback",
				AuthorizationCodeFetcher: simpleHandler,
			},
		},
		{
			name: "DynamicClientRegistrationConfig_NoRedirectURL",
			config: &AuthorizationCodeHandlerConfig{
				DynamicClientRegistrationConfig: &DynamicClientRegistrationConfig{
					Metadata: &oauthex.ClientRegistrationMetadata{
						RedirectURIs: []string{
							"https://example.com/callback",
						},
					},
				},
				AuthorizationCodeFetcher: simpleHandler,
			},
		},
		{
			name: "DynamicClientRegistrationConfig_WithRedirectURL",
			config: &AuthorizationCodeHandlerConfig{
				DynamicClientRegistrationConfig: &DynamicClientRegistrationConfig{
					Metadata: &oauthex.ClientRegistrationMetadata{
						RedirectURIs: []string{
							"https://example.com/callback",
						},
					},
				},
				RedirectURL:              "https://example.com/callback",
				AuthorizationCodeFetcher: simpleHandler,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewAuthorizationCodeHandler(tt.config); err != nil {
				t.Fatalf("NewAuthorizationCodeHandler failed: %v", err)
			}
		})
	}
}

func TestNewAuthorizationCodeHandler_Error(t *testing.T) {
	// Ensure the base config is valid.
	if _, err := NewAuthorizationCodeHandler(validConfig()); err != nil {
		t.Fatalf("NewAuthorizationCodeHandler failed: %v", err)
	}

	tests := []struct {
		name   string
		config func() *AuthorizationCodeHandlerConfig
	}{
		{
			name: "NilConfig",
			config: func() *AuthorizationCodeHandlerConfig {
				return nil
			},
		},
		{
			name: "NoRegistrationConfig",
			config: func() *AuthorizationCodeHandlerConfig {
				cfg := validConfig()
				cfg.ClientIDMetadataDocumentConfig = nil
				cfg.PreregisteredClient = nil
				cfg.DynamicClientRegistrationConfig = nil
				return cfg
			},
		},
		{
			name: "MissingRedirectURL",
			config: func() *AuthorizationCodeHandlerConfig {
				cfg := validConfig()
				cfg.RedirectURL = ""
				return cfg
			},
		},
		{
			name: "MissingAuthorizationCodeFetcher",
			config: func() *AuthorizationCodeHandlerConfig {
				cfg := validConfig()
				cfg.AuthorizationCodeFetcher = nil
				return cfg
			},
		},
		{
			name: "InvalidMetadataURL",
			config: func() *AuthorizationCodeHandlerConfig {
				cfg := validConfig()
				cfg.ClientIDMetadataDocumentConfig.URL = "https://example.com"
				return cfg
			},
		},
		{
			name: "InvalidPreregistered_MissingSecretConfig",
			config: func() *AuthorizationCodeHandlerConfig {
				cfg := validConfig()
				cfg.PreregisteredClient = &oauthex.ClientCredentials{}
				return cfg
			},
		},
		{
			name: "InvalidPreregistered_EmptyID",
			config: func() *AuthorizationCodeHandlerConfig {
				cfg := validConfig()
				cfg.PreregisteredClient = &oauthex.ClientCredentials{
					ClientID: "",
					ClientSecretAuth: &oauthex.ClientSecretAuth{
						ClientSecret: "secret",
					},
				}
				return cfg
			},
		},
		{
			name: "InvalidPreregistered_EmptySecret",
			config: func() *AuthorizationCodeHandlerConfig {
				cfg := validConfig()
				cfg.PreregisteredClient = &oauthex.ClientCredentials{
					ClientID: "test_client_id",
					ClientSecretAuth: &oauthex.ClientSecretAuth{
						ClientSecret: "",
					},
				}
				return cfg
			},
		},
		{
			name: "InvalidDynamic_MissingMetadata",
			config: func() *AuthorizationCodeHandlerConfig {
				cfg := validConfig()
				cfg.DynamicClientRegistrationConfig = &DynamicClientRegistrationConfig{}
				return cfg
			},
		},
		{
			name: "InvalidDynamic_InconsistentRedirectURI",
			config: func() *AuthorizationCodeHandlerConfig {
				cfg := validConfig()
				cfg.DynamicClientRegistrationConfig = &DynamicClientRegistrationConfig{
					Metadata: &oauthex.ClientRegistrationMetadata{
						RedirectURIs: []string{"https://example.com/callback1"},
					},
				}
				cfg.RedirectURL = "https://example.com/callback2"
				return cfg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAuthorizationCodeHandler(tt.config())
			if err == nil {
				t.Errorf("NewAuthorizationCodeHandler() = nil, want error")
			}
		})
	}
}

func TestGetProtectedResourceMetadata_Success(t *testing.T) {
	handler, err := NewAuthorizationCodeHandler(validConfig())
	if err != nil {
		t.Fatalf("NewAuthorizationCodeHandler() error = %v", err)
	}

	pathForChallenge := "/protected-resource"

	tests := []struct {
		name               string
		challengesProvided bool
		// Path of the PRM endpoint.
		prmPath string
		// Path of the MCP server that is accessed.
		mcpServerPath string
		// Path for the Resource expected in the returned PRM.
		resourcePath string
	}{
		{
			name:               "FromChallenges",
			challengesProvided: true,
			prmPath:            pathForChallenge,
			mcpServerPath:      "/resource",
			resourcePath:       "/resource",
		},
		{
			name:               "FallbackToEndpoint",
			challengesProvided: false,
			prmPath:            "/.well-known/oauth-protected-resource/resource",
			mcpServerPath:      "/resource",
			resourcePath:       "/resource",
		},
		{
			name:               "FallbackToRoot",
			challengesProvided: false,
			prmPath:            "/.well-known/oauth-protected-resource",
			mcpServerPath:      "/resource",
			resourcePath:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)
			metadata := &oauthex.ProtectedResourceMetadata{
				Resource:             server.URL + tt.resourcePath,
				AuthorizationServers: []string{"https://oauth.example.com"},
				ScopesSupported:      []string{"read", "write"},
			}
			mux.Handle(tt.prmPath, ProtectedResourceMetadataHandler(metadata))
			var challenges []oauthex.Challenge
			if tt.challengesProvided {
				challenges = []oauthex.Challenge{
					{
						Scheme: "Bearer",
						Params: map[string]string{
							"resource_metadata": server.URL + pathForChallenge,
						},
					},
				}
			}

			got, err := handler.getProtectedResourceMetadata(t.Context(), challenges, server.URL+tt.mcpServerPath)
			if err != nil {
				t.Fatalf("getProtectedResourceMetadata() error = %v", err)
			}
			if got == nil {
				t.Fatal("getProtectedResourceMetadata() got nil, want metadata")
			}
			if diff := cmp.Diff(metadata, got); diff != "" {
				t.Errorf("getProtectedResourceMetadata() metadata mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetProtectedResourceMetadata_Backcompat(t *testing.T) {
	handler, err := NewAuthorizationCodeHandler(validConfig())
	if err != nil {
		t.Fatalf("NewAuthorizationCodeHandler() error = %v", err)
	}
	var challenges []oauthex.Challenge
	got, err := handler.getProtectedResourceMetadata(t.Context(), challenges, "http://localhost:1234/resource")
	if err != nil {
		t.Fatalf("getProtectedResourceMetadata() error = %v", err)
	}
	wantPRM := &oauthex.ProtectedResourceMetadata{
		Resource:             "http://localhost:1234/resource",
		AuthorizationServers: []string{"http://localhost:1234"},
	}
	if diff := cmp.Diff(wantPRM, got); diff != "" {
		t.Errorf("getProtectedResourceMetadata() metadata mismatch (-want +got):\n%s", diff)
	}
}

func TestGetProtectedResourceMetadata_Error(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	metadata := &oauthex.ProtectedResourceMetadata{
		Resource:             server.URL + "/resource",
		AuthorizationServers: nil, // Empty list is invalid
		ScopesSupported:      []string{"read", "write"},
	}
	mux.Handle("/.well-known/oauth-protected-resource/resource", ProtectedResourceMetadataHandler(metadata))
	handler, err := NewAuthorizationCodeHandler(validConfig())
	if err != nil {
		t.Fatalf("NewAuthorizationCodeHandler() error = %v", err)
	}
	var challenges []oauthex.Challenge
	got, err := handler.getProtectedResourceMetadata(t.Context(), challenges, server.URL+"/resource")
	if err == nil || !strings.Contains(err.Error(), "authorization servers") {
		t.Errorf("getProtectedResourceMetadata() = %v, want error containing \"authorization servers\"", err)
	}
	if got != nil {
		t.Errorf("getProtectedResourceMetadata() = %+v, want nil", got)
	}
}

func TestSelectTokenAuthMethod(t *testing.T) {
	tests := []struct {
		name      string
		supported []string
		want      oauth2.AuthStyle
	}{
		{
			name:      "PostPreferredOverBasic",
			supported: []string{"client_secret_basic", "client_secret_post"},
			want:      oauth2.AuthStyleInParams,
		},
		{
			name:      "BasicChosenIfPostNotSupported",
			supported: []string{"private_key_jwt", "client_secret_basic"},
			want:      oauth2.AuthStyleInHeader,
		},
		{
			name:      "NoneSupported",
			supported: []string{"private_key_jwt"},
			want:      oauth2.AuthStyleAutoDetect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectTokenAuthMethod(tt.supported)
			if got != tt.want {
				t.Errorf("selectTokenAuthMethod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandleRegistration(t *testing.T) {
	tests := []struct {
		name          string
		serverConfig  *oauthtest.RegistrationConfig
		handlerConfig *AuthorizationCodeHandlerConfig
		asm           *oauthex.AuthServerMeta
		want          *resolvedClientConfig
		wantError     bool
		issuerMatch   bool
		issuerSuffix  string
	}{
		{
			name: "ClientIDMetadataDocument",
			serverConfig: &oauthtest.RegistrationConfig{
				ClientIDMetadataDocumentSupported: true,
			},
			handlerConfig: &AuthorizationCodeHandlerConfig{
				ClientIDMetadataDocumentConfig: &ClientIDMetadataDocumentConfig{URL: "https://client.example.com/metadata.json"},
			},
			want: &resolvedClientConfig{
				registrationType: registrationTypeClientIDMetadataDocument,
				clientID:         "https://client.example.com/metadata.json",
			},
		},
		{
			name: "Preregistered",
			serverConfig: &oauthtest.RegistrationConfig{
				PreregisteredClients: map[string]oauthtest.ClientInfo{
					"pre_client_id": {
						Secret: "pre_client_secret",
					},
				},
			},
			handlerConfig: &AuthorizationCodeHandlerConfig{
				PreregisteredClient: &oauthex.ClientCredentials{
					ClientID: "pre_client_id",
					ClientSecretAuth: &oauthex.ClientSecretAuth{
						ClientSecret: "pre_client_secret",
					},
				},
			},
			want: &resolvedClientConfig{
				registrationType: registrationTypePreregistered,
				clientID:         "pre_client_id",
				clientSecret:     "pre_client_secret",
				authStyle:        oauth2.AuthStyleInParams,
			},
		},
		{
			name: "Preregistered_IssuerMatch",
			serverConfig: &oauthtest.RegistrationConfig{
				PreregisteredClients: map[string]oauthtest.ClientInfo{
					"pre_client_id": {
						Secret: "pre_client_secret",
					},
				},
			},
			handlerConfig: &AuthorizationCodeHandlerConfig{
				PreregisteredClient: &oauthex.ClientCredentials{
					ClientID: "pre_client_id",
					ClientSecretAuth: &oauthex.ClientSecretAuth{
						ClientSecret: "pre_client_secret",
					},
					Issuer: "", // set dynamically in the test
				},
			},
			want: &resolvedClientConfig{
				registrationType: registrationTypePreregistered,
				clientID:         "pre_client_id",
				clientSecret:     "pre_client_secret",
				authStyle:        oauth2.AuthStyleInParams,
			},
			issuerMatch: true,
		},
		{
			name: "Preregistered_IssuerMismatch",
			serverConfig: &oauthtest.RegistrationConfig{
				PreregisteredClients: map[string]oauthtest.ClientInfo{
					"pre_client_id": {
						Secret: "pre_client_secret",
					},
				},
			},
			handlerConfig: &AuthorizationCodeHandlerConfig{
				PreregisteredClient: &oauthex.ClientCredentials{
					ClientID: "pre_client_id",
					ClientSecretAuth: &oauthex.ClientSecretAuth{
						ClientSecret: "pre_client_secret",
					},
					Issuer: "https://other-issuer.example.com",
				},
			},
			wantError: true,
		},
		{
			name: "Preregistered_IssuerMatchTrailingSlash",
			serverConfig: &oauthtest.RegistrationConfig{
				PreregisteredClients: map[string]oauthtest.ClientInfo{
					"pre_client_id": {
						Secret: "pre_client_secret",
					},
				},
			},
			handlerConfig: &AuthorizationCodeHandlerConfig{
				PreregisteredClient: &oauthex.ClientCredentials{
					ClientID: "pre_client_id",
					ClientSecretAuth: &oauthex.ClientSecretAuth{
						ClientSecret: "pre_client_secret",
					},
					Issuer: "", // set dynamically in the test (with trailing slash)
				},
			},
			want: &resolvedClientConfig{
				registrationType: registrationTypePreregistered,
				clientID:         "pre_client_id",
				clientSecret:     "pre_client_secret",
				authStyle:        oauth2.AuthStyleInParams,
			},
			issuerMatch:  true,
			issuerSuffix: "/",
		},
		{
			name: "NoneSupported",
			handlerConfig: &AuthorizationCodeHandlerConfig{
				ClientIDMetadataDocumentConfig: &ClientIDMetadataDocumentConfig{URL: "https://client.example.com/metadata.json"},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oauthtest.NewFakeAuthorizationServer(oauthtest.Config{RegistrationConfig: tt.serverConfig})
			s.Start(t)
			// Set the Issuer dynamically if requested by the test case.
			if tt.issuerMatch {
				tt.handlerConfig.PreregisteredClient.Issuer = s.URL() + tt.issuerSuffix
			}
			tt.handlerConfig.AuthorizationCodeFetcher = func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
				return nil, nil
			}
			tt.handlerConfig.RedirectURL = "https://example.com/callback"
			handler, err := NewAuthorizationCodeHandler(tt.handlerConfig)
			if err != nil {
				t.Fatalf("NewAuthorizationCodeHandler() error = %v, want nil", err)
			}
			asm, err := GetAuthServerMetadata(t.Context(), s.URL(), http.DefaultClient)
			if err != nil {
				t.Fatalf("GetAuthServerMetadata() unexpected error = %v", err)
			}
			got, err := handler.handleRegistration(t.Context(), asm)
			if err != nil {
				if !tt.wantError {
					t.Fatalf("handleRegistration() unexpected error = %v", err)
				}
				return
			}
			if tt.wantError {
				t.Fatal("handleRegistration() expected error, got nil")
			}
			if got.registrationType != tt.want.registrationType {
				t.Errorf("handleRegistration() registrationType = %v, want %v", got.registrationType, tt.want.registrationType)
			}
			if got.clientID != tt.want.clientID {
				t.Errorf("handleRegistration() clientID = %q, want %q", got.clientID, tt.want.clientID)
			}
			if got.clientSecret != tt.want.clientSecret {
				t.Errorf("handleRegistration() clientSecret = %q, want %q", got.clientSecret, tt.want.clientSecret)
			}
			if got.authStyle != tt.want.authStyle {
				t.Errorf("handleRegistration() authStyle = %v, want %v", got.authStyle, tt.want.authStyle)
			}
		})
	}
}

func TestDynamicRegistration(t *testing.T) {
	s := oauthtest.NewFakeAuthorizationServer(oauthtest.Config{
		RegistrationConfig: &oauthtest.RegistrationConfig{
			DynamicClientRegistrationEnabled: true,
		},
	})
	s.Start(t)
	handler, err := NewAuthorizationCodeHandler(&AuthorizationCodeHandlerConfig{
		DynamicClientRegistrationConfig: &DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				RedirectURIs: []string{"https://example.com/callback"},
			},
		},
		RedirectURL: "https://example.com/callback",
		AuthorizationCodeFetcher: func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("NewAuthorizationCodeHandler() error = %v", err)
	}
	asm, err := GetAuthServerMetadata(t.Context(), s.URL(), http.DefaultClient)
	if err != nil {
		t.Fatalf("GetAuthServerMetadata() unexpected error = %v", err)
	}
	got, err := handler.handleRegistration(t.Context(), asm)
	if err != nil {
		t.Fatalf("handleRegistration() error = %v, want nil", err)
	}
	if got.registrationType != registrationTypeDynamic {
		t.Errorf("handleRegistration() registrationType = %v, want %v", got.registrationType, registrationTypeDynamic)
	}
	if got.clientID == "" {
		t.Errorf("handleRegistration() clientID = %q, want non-empty", got.clientID)
	}
	if got.clientSecret == "" {
		t.Errorf("handleRegistration() clientSecret = %q, want non-empty", got.clientSecret)
	}
	if got.authStyle != oauth2.AuthStyleInHeader {
		t.Errorf("handleRegistration() authStyle = %v, want %v", got.authStyle, oauth2.AuthStyleInHeader)
	}
}

func TestValidateIssuerResponse(t *testing.T) {
	const expectedIssuer = "https://auth.example.com"

	tests := []struct {
		name            string
		iss             string
		issSupported    bool
		wantErr         bool
		wantErrContains string
	}{
		{
			name:         "ValidIss",
			iss:          expectedIssuer,
			issSupported: true,
		},
		{
			name:            "WrongIss",
			iss:             "https://attacker.example.com",
			issSupported:    true,
			wantErr:         true,
			wantErrContains: "does not match expected issuer",
		},
		{
			name:            "MissingIssWhenRequired",
			iss:             "",
			issSupported:    true,
			wantErr:         true,
			wantErrContains: "RFC 9207",
		},
		{
			name:         "MissingIssWhenNotRequired",
			iss:          "",
			issSupported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIssuerResponse(tt.iss, expectedIssuer, tt.issSupported)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateIssuerResponse() = nil, want error containing %q", tt.wantErrContains)
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("validateIssuerResponse() error = %q, want it to contain %q", err.Error(), tt.wantErrContains)
				}
			} else if err != nil {
				t.Fatalf("validateIssuerResponse() unexpected error = %v", err)
			}
		})
	}
}

func TestInferApplicationType(t *testing.T) {
	tests := []struct {
		name         string
		redirectURIs []string
		want         string
	}{
		{
			name:         "localhost",
			redirectURIs: []string{"http://localhost:8085/callback"},
			want:         "native",
		},
		{
			name:         "127.0.0.1",
			redirectURIs: []string{"http://127.0.0.1:8085/callback"},
			want:         "native",
		},
		{
			name:         "IPv6 loopback",
			redirectURIs: []string{"http://[::1]:8085/callback"},
			want:         "native",
		},
		{
			name:         "custom scheme",
			redirectURIs: []string{"myapp://callback"},
			want:         "native",
		},
		{
			name:         "HTTPS remote",
			redirectURIs: []string{"https://myapp.example.com/callback"},
			want:         "web",
		},
		{
			name:         "mixed native and web",
			redirectURIs: []string{"https://myapp.example.com/callback", "http://localhost:8085/callback"},
			want:         "",
		},
		{
			name:         "multiple remote",
			redirectURIs: []string{"https://app1.example.com/cb", "https://app2.example.com/cb"},
			want:         "web",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferApplicationType(tt.redirectURIs)
			if got != tt.want {
				t.Errorf("inferApplicationType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplicationTypeInference(t *testing.T) {
	fetcher := func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
		return nil, nil
	}

	tests := []struct {
		name           string
		redirectURIs   []string
		initialAppType string
		wantAppType    string
		wantErr        bool
	}{
		{
			name:         "inferred as native for localhost",
			redirectURIs: []string{"http://localhost:8085/callback"},
			wantAppType:  "native",
		},
		{
			name:         "inferred as web for remote",
			redirectURIs: []string{"https://example.com/callback"},
			wantAppType:  "web",
		},
		{
			name:         "mixed native and web URIs sets empty application type",
			redirectURIs: []string{"https://example.com/callback", "http://localhost:8085/callback"},
			wantAppType:  "",
		},
		{
			name:           "explicit value matching inference is preserved",
			redirectURIs:   []string{"http://localhost:8085/callback"},
			initialAppType: "native",
			wantAppType:    "native",
		},
		{
			name:           "explicit value conflicts with inference returns error",
			redirectURIs:   []string{"http://localhost:8085/callback"},
			initialAppType: "web",
			wantErr:        true,
		},
		{
			name:           "explicit value when inference is ambiguous returns error",
			redirectURIs:   []string{"https://example.com/callback", "http://localhost:8085/callback"},
			initialAppType: "web",
			wantErr:        true,
		},
		{
			name:         "invalid URI returns empty application type",
			redirectURIs: []string{"http://%/"},
			wantAppType:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AuthorizationCodeHandlerConfig{
				DynamicClientRegistrationConfig: &DynamicClientRegistrationConfig{
					Metadata: &oauthex.ClientRegistrationMetadata{
						RedirectURIs:    tt.redirectURIs,
						ApplicationType: tt.initialAppType,
					},
				},
				AuthorizationCodeFetcher: fetcher,
			}
			_, err := NewAuthorizationCodeHandler(cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewAuthorizationCodeHandler() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			got := cfg.DynamicClientRegistrationConfig.Metadata.ApplicationType
			if got != tt.wantAppType {
				t.Errorf("ApplicationType = %q, want %q", got, tt.wantAppType)
			}
		})
	}
}

func TestAuthorize_OfflineAccessScope(t *testing.T) {
	tests := []struct {
		name                string
		requestRefreshToken bool
		asScopesSupported   []string
		challengeScopes     string
		wantOfflineAccess   bool
	}{
		{
			name:                "AddedWhenASSupportsAndClientRequests",
			requestRefreshToken: true,
			asScopesSupported:   []string{"openid", "offline_access"},
			wantOfflineAccess:   true,
		},
		{
			name:                "NotAddedWhenClientDoesNotRequest",
			requestRefreshToken: false,
			asScopesSupported:   []string{"openid", "offline_access"},
			wantOfflineAccess:   false,
		},
		{
			name:                "NotAddedWhenASDoesNotSupport",
			requestRefreshToken: true,
			asScopesSupported:   []string{"openid"},
			wantOfflineAccess:   false,
		},
		{
			name:                "NotDuplicatedWhenAlreadyInScopes",
			requestRefreshToken: true,
			asScopesSupported:   []string{"openid", "offline_access"},
			challengeScopes:     "read offline_access",
			wantOfflineAccess:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authServer := oauthtest.NewFakeAuthorizationServer(oauthtest.Config{
				ScopesSupported: tt.asScopesSupported,
				RegistrationConfig: &oauthtest.RegistrationConfig{
					PreregisteredClients: map[string]oauthtest.ClientInfo{
						"test_client_id": {
							Secret:       "test_client_secret",
							RedirectURIs: []string{"http://localhost:12345/callback"},
						},
					},
				},
			})
			authServer.Start(t)

			resourceMux := http.NewServeMux()
			resourceServer := httptest.NewServer(resourceMux)
			t.Cleanup(resourceServer.Close)
			resourceURL := resourceServer.URL + "/resource"
			resourceMux.Handle("/.well-known/oauth-protected-resource/resource", ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
				Resource:             resourceURL,
				AuthorizationServers: []string{authServer.URL()},
			}))

			var capturedAuthURL string
			handler, err := NewAuthorizationCodeHandler(&AuthorizationCodeHandlerConfig{
				RedirectURL: "http://localhost:12345/callback",
				PreregisteredClient: &oauthex.ClientCredentials{
					ClientID: "test_client_id",
					ClientSecretAuth: &oauthex.ClientSecretAuth{
						ClientSecret: "test_client_secret",
					},
				},
				RequestRefreshToken: tt.requestRefreshToken,
				AuthorizationCodeFetcher: func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
					capturedAuthURL = args.URL
					return nil, fmt.Errorf("stop after capturing URL")
				},
			})
			if err != nil {
				t.Fatalf("NewAuthorizationCodeHandler failed: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, resourceURL, nil)
			resp := &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}
			wwwAuth := "Bearer resource_metadata=" + resourceServer.URL + "/.well-known/oauth-protected-resource/resource"
			if tt.challengeScopes != "" {
				wwwAuth += fmt.Sprintf(", scope=%q", tt.challengeScopes)
			}
			resp.Header.Set("WWW-Authenticate", wwwAuth)

			// Authorize will fail at the fetcher, but we only care about the URL.
			handler.Authorize(context.Background(), req, resp)

			if capturedAuthURL == "" {
				t.Fatal("AuthorizationCodeFetcher was not called")
			}
			u, err := url.Parse(capturedAuthURL)
			if err != nil {
				t.Fatalf("failed to parse captured auth URL: %v", err)
			}
			scopes := strings.Fields(u.Query().Get("scope"))
			hasOfflineAccess := slices.Contains(scopes, "offline_access")
			if hasOfflineAccess != tt.wantOfflineAccess {
				t.Errorf("offline_access in scopes = %v, want %v (scopes: %v)", hasOfflineAccess, tt.wantOfflineAccess, scopes)
			}

			// When offline_access was already present in challenge scopes,
			// verify it appears exactly once.
			if tt.wantOfflineAccess {
				count := 0
				for _, s := range scopes {
					if s == "offline_access" {
						count++
					}
				}
				if count != 1 {
					t.Errorf("offline_access appears %d times in scopes, want 1", count)
				}
			}
		})
	}
}

// validConfig for test to create an AuthorizationCodeHandler using its constructor.
// Values that are relevant to the test should be set explicitly.
func validConfig() *AuthorizationCodeHandlerConfig {
	return &AuthorizationCodeHandlerConfig{
		ClientIDMetadataDocumentConfig: &ClientIDMetadataDocumentConfig{URL: "https://example.com/client"},
		RedirectURL:                    "https://example.com/callback",
		AuthorizationCodeFetcher: func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
			return nil, nil
		},
	}
}

func TestNewTokenSource(t *testing.T) {
	// mock the /token endpoint to successfully return an access token on code exchange
	mockTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token": "test_token", "token_type": "bearer"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockTS.Close()

	// configure the handler and set NewTokenSource
	var called bool
	handler, err := NewAuthorizationCodeHandler(&AuthorizationCodeHandlerConfig{
		RedirectURL: "http://localhost/callback",
		PreregisteredClient: &oauthex.ClientCredentials{
			ClientID: "test_client",
		},
		AuthorizationCodeFetcher: func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
			u, _ := url.Parse(args.URL)
			return &AuthorizationResult{
				Code:  "test_code",
				State: u.Query().Get("state"),
			}, nil
		},
		NewTokenSource: func(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (oauth2.TokenSource, error) {
			called = true
			return oauth2.StaticTokenSource(token), nil
		},
		Client: mockTS.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Simulate a 401 response from a resource server.
	// The WWW-Authenticate: Bearer header triggers the authorization logic.
	req := httptest.NewRequest(http.MethodGet, mockTS.URL, nil)
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    req,
	}
	resp.Header.Set("WWW-Authenticate", "Bearer")

	// Authorize and confirm NewTokenSource was called.
	err = handler.Authorize(t.Context(), req, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected NewTokenSource to be called")
	}
}

func TestInitialTokenSource(t *testing.T) {
	handler, err := NewAuthorizationCodeHandler(&AuthorizationCodeHandlerConfig{
		RedirectURL: "http://localhost:12345/callback",
		PreregisteredClient: &oauthex.ClientCredentials{
			ClientID: "test_client_id",
		},
		AuthorizationCodeFetcher: func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error) {
			return nil, nil
		},
		InitialTokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "set_token"}),
	})
	if err != nil {
		t.Fatalf("NewAuthorizationCodeHandler failed: %v", err)
	}

	ts, err := handler.TokenSource(t.Context())
	if err != nil {
		t.Fatalf("failed to get token source: %v", err)
	}
	if ts == nil {
		t.Fatal("expected token source to be non-nil")
	}

	tok, err := ts.Token()
	if err != nil {
		t.Fatalf("failed to get Token: %v", err)
	}
	if tok.AccessToken != "set_token" {
		t.Errorf("expected access token 'set_token', got '%s'", tok.AccessToken)
	}
}
