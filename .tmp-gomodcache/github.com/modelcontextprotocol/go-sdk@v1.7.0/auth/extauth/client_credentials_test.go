// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package extauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/internal/oauthtest"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

func validClientCredentialsConfig() *ClientCredentialsHandlerConfig {
	return &ClientCredentialsHandlerConfig{
		Credentials: &oauthex.ClientCredentials{
			ClientID:         "test-client",
			ClientSecretAuth: &oauthex.ClientSecretAuth{ClientSecret: "test-secret"},
		},
	}
}

func TestNewClientCredentialsHandler_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    *ClientCredentialsHandlerConfig
		wantError string
	}{
		{
			name:      "nil config",
			config:    nil,
			wantError: "config must be provided",
		},
		{
			name: "nil credentials",
			config: func() *ClientCredentialsHandlerConfig {
				c := validClientCredentialsConfig()
				c.Credentials = nil
				return c
			}(),
			wantError: "credentials are required",
		},
		{
			name: "missing client ID",
			config: func() *ClientCredentialsHandlerConfig {
				c := validClientCredentialsConfig()
				c.Credentials.ClientID = ""
				return c
			}(),
			wantError: "ClientID is required",
		},
		{
			name: "missing client secret auth",
			config: func() *ClientCredentialsHandlerConfig {
				c := validClientCredentialsConfig()
				c.Credentials.ClientSecretAuth = nil
				return c
			}(),
			wantError: "clientSecretAuth is required",
		},
		{
			name: "empty client secret",
			config: func() *ClientCredentialsHandlerConfig {
				c := validClientCredentialsConfig()
				c.Credentials.ClientSecretAuth.ClientSecret = ""
				return c
			}(),
			wantError: "ClientSecret is required",
		},
		{
			name:   "valid config",
			config: validClientCredentialsConfig(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClientCredentialsHandler(tc.config)
			if tc.wantError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tc.wantError)
				} else if !strings.Contains(err.Error(), tc.wantError) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantError)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestClientCredentialsHandler_Authorize(t *testing.T) {
	authServer := oauthtest.NewFakeAuthorizationServer(oauthtest.Config{
		MetadataEndpointConfig: &oauthtest.MetadataEndpointConfig{
			ServeOAuthInsertedEndpoint: true,
		},
		RegistrationConfig: &oauthtest.RegistrationConfig{
			PreregisteredClients: map[string]oauthtest.ClientInfo{
				"test-client": {Secret: "test-secret"},
			},
		},
		ClientCredentialsConfig: &oauthtest.ClientCredentialsConfig{
			Enabled: true,
		},
	})
	authServer.Start(t)

	resourceMux := http.NewServeMux()
	resourceServer := httptest.NewServer(resourceMux)
	t.Cleanup(resourceServer.Close)
	resourceURL := resourceServer.URL + "/resource"

	resourceMux.Handle("/.well-known/oauth-protected-resource/resource", auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:             resourceURL,
		AuthorizationServers: []string{authServer.URL()},
		ScopesSupported:      []string{"mcp:read", "mcp:write"},
	}))

	t.Run("successful authorization", func(t *testing.T) {
		handler, err := NewClientCredentialsHandler(validClientCredentialsConfig())
		if err != nil {
			t.Fatal(err)
		}

		// TokenSource should be nil before Authorize.
		ts, err := handler.TokenSource(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if ts != nil {
			t.Fatal("expected nil TokenSource before Authorize")
		}

		// Simulate a 401 response from the MCP server.
		resp := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{},
			Body:       http.NoBody,
		}
		req := httptest.NewRequest("GET", resourceURL, nil)
		if err := handler.Authorize(t.Context(), req, resp); err != nil {
			t.Fatalf("Authorize failed: %v", err)
		}

		// TokenSource should now return a valid token.
		ts, err = handler.TokenSource(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if ts == nil {
			t.Fatal("expected non-nil TokenSource after Authorize")
		}
		tok, err := ts.Token()
		if err != nil {
			t.Fatalf("Token() failed: %v", err)
		}
		if tok.AccessToken != "test_access_token" {
			t.Errorf("got access token %q, want %q", tok.AccessToken, "test_access_token")
		}
	})

	t.Run("bad credentials", func(t *testing.T) {
		config := validClientCredentialsConfig()
		config.Credentials.ClientSecretAuth.ClientSecret = "wrong-secret"
		handler, err := NewClientCredentialsHandler(config)
		if err != nil {
			t.Fatal(err)
		}

		resp := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{},
			Body:       http.NoBody,
		}
		req := httptest.NewRequest("GET", resourceURL, nil)
		if err := handler.Authorize(t.Context(), req, resp); err == nil {
			t.Error("expected Authorize to fail with bad credentials")
		}

		// TokenSource should still be nil after failed Authorize.
		ts, err := handler.TokenSource(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if ts != nil {
			t.Error("expected nil TokenSource after failed Authorize")
		}
	})

	t.Run("scopes from WWW-Authenticate challenge", func(t *testing.T) {
		handler, err := NewClientCredentialsHandler(validClientCredentialsConfig())
		if err != nil {
			t.Fatal(err)
		}

		resp := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header: http.Header{
				"WWW-Authenticate": []string{`Bearer scope="read write"`},
			},
			Body: http.NoBody,
		}
		req := httptest.NewRequest("GET", resourceURL, nil)
		if err := handler.Authorize(t.Context(), req, resp); err != nil {
			t.Fatalf("Authorize failed: %v", err)
		}

		ts, err := handler.TokenSource(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if ts == nil {
			t.Fatal("expected non-nil TokenSource")
		}
	})

	t.Run("issuer mismatch", func(t *testing.T) {
		config := validClientCredentialsConfig()
		config.Credentials.Issuer = "https://other-issuer.example.com"
		handler, err := NewClientCredentialsHandler(config)
		if err != nil {
			t.Fatal(err)
		}

		resp := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{},
			Body:       http.NoBody,
		}
		req := httptest.NewRequest("GET", resourceURL, nil)
		err = handler.Authorize(t.Context(), req, resp)
		if err == nil {
			t.Fatal("expected Authorize to fail with issuer mismatch")
		}
		if !strings.Contains(err.Error(), "does not match") {
			t.Errorf("error %q does not mention issuer mismatch", err.Error())
		}
	})

	t.Run("issuer match ignoring trailing slash", func(t *testing.T) {
		config := validClientCredentialsConfig()
		// authServer.URL() has no trailing slash; configure with one to
		// verify the comparison tolerates the difference (per RFC 8414 §3.3
		// normalization applied in oauthex.IssuersEqual).
		config.Credentials.Issuer = authServer.URL() + "/"
		handler, err := NewClientCredentialsHandler(config)
		if err != nil {
			t.Fatal(err)
		}

		resp := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{},
			Body:       http.NoBody,
		}
		req := httptest.NewRequest("GET", resourceURL, nil)
		if err := handler.Authorize(t.Context(), req, resp); err != nil {
			t.Fatalf("Authorize() unexpected error = %v", err)
		}
	})

	t.Run("PRM via resource_metadata in challenge", func(t *testing.T) {
		prmMux := http.NewServeMux()
		prmMux.Handle("/custom-prm", auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
			Resource:             resourceURL,
			AuthorizationServers: []string{authServer.URL()},
		}))
		prmServer := httptest.NewServer(prmMux)
		t.Cleanup(prmServer.Close)

		handler, err := NewClientCredentialsHandler(validClientCredentialsConfig())
		if err != nil {
			t.Fatal(err)
		}

		resp := &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header: http.Header{
				"WWW-Authenticate": []string{`Bearer resource_metadata="` + prmServer.URL + `/custom-prm"`},
			},
			Body: http.NoBody,
		}
		req := httptest.NewRequest("GET", resourceURL, nil)
		if err := handler.Authorize(t.Context(), req, resp); err != nil {
			t.Fatalf("Authorize with resource_metadata challenge failed: %v", err)
		}

		ts, err := handler.TokenSource(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if ts == nil {
			t.Fatal("expected non-nil TokenSource")
		}
	})
}

func TestClientCredentialsHandler_ScopeAccumulation(t *testing.T) {
	authServer := oauthtest.NewFakeAuthorizationServer(oauthtest.Config{
		MetadataEndpointConfig: &oauthtest.MetadataEndpointConfig{
			ServeOAuthInsertedEndpoint: true,
		},
		RegistrationConfig: &oauthtest.RegistrationConfig{
			PreregisteredClients: map[string]oauthtest.ClientInfo{
				"test-client": {Secret: "test-secret"},
			},
		},
		ClientCredentialsConfig: &oauthtest.ClientCredentialsConfig{
			Enabled: true,
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

	resourceMux.Handle("/.well-known/oauth-protected-resource/resource", auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:             resourceURL,
		AuthorizationServers: []string{authServer.URL()},
	}))

	handler, err := NewClientCredentialsHandler(validClientCredentialsConfig())
	if err != nil {
		t.Fatal(err)
	}

	// First authorization: 401 with scope="read write".
	// The token response will only grant "read" (TokenScopeFunc strips "write").
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     make(http.Header),
		Body:       http.NoBody,
	}
	resp.Header.Set("WWW-Authenticate", `Bearer scope="read write"`)
	req := httptest.NewRequest("GET", resourceURL, nil)
	if err := handler.Authorize(t.Context(), req, resp); err != nil {
		t.Fatalf("First Authorize failed: %v", err)
	}

	// Verify only "read" was granted (the token omitted "write").
	issuer := authServer.URL()
	if diff := cmp.Diff([]string{"read"}, handler.grantedScopes[issuer], cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("After first Authorize, grantedScopes mismatch (-want +got):\n%s", diff)
	}

	// Second authorization: 401 with scope="admin" (simulating step-up).
	// Accumulated scopes should be "read" (previously granted) + "admin" (new),
	// but NOT "write" (requested but never granted).
	resp2 := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     make(http.Header),
		Body:       http.NoBody,
	}
	resp2.Header.Set("WWW-Authenticate", `Bearer scope="admin"`)
	req2 := httptest.NewRequest("GET", resourceURL, nil)
	if err := handler.Authorize(t.Context(), req2, resp2); err != nil {
		t.Fatalf("Second Authorize failed: %v", err)
	}

	// Verify accumulated scopes: "read" (granted) + "admin" (new), no "write".
	if diff := cmp.Diff([]string{"admin", "read"}, handler.grantedScopes[issuer], cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("After second Authorize, grantedScopes mismatch (-want +got):\n%s", diff)
	}
}

func TestSelectTokenAuthMethod(t *testing.T) {
	tests := []struct {
		name      string
		supported []string
		want      oauth2.AuthStyle
	}{
		{
			name:      "prefers client_secret_post",
			supported: []string{"client_secret_basic", "client_secret_post"},
			want:      oauth2.AuthStyleInParams,
		},
		{
			name:      "falls back to client_secret_basic",
			supported: []string{"client_secret_basic"},
			want:      oauth2.AuthStyleInHeader,
		},
		{
			name:      "auto detect when none supported",
			supported: []string{"private_key_jwt"},
			want:      oauth2.AuthStyleAutoDetect,
		},
		{
			name:      "auto detect when empty",
			supported: nil,
			want:      oauth2.AuthStyleAutoDetect,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := selectTokenAuthMethod(tc.supported)
			if got != tc.want {
				t.Errorf("selectTokenAuthMethod(%v) = %v, want %v", tc.supported, got, tc.want)
			}
		})
	}
}
