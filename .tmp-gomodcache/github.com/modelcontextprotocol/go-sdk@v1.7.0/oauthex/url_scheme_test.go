// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package oauthex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCheckURLScheme tests the checkURLScheme function directly.
func TestCheckURLScheme(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Valid schemes
		{"empty string", "", false},
		{"https url", "https://example.com/path", false},
		{"http url", "http://example.com/path", false},
		{"custom scheme", "myapp://callback", false},

		// Dangerous schemes that should be blocked
		{"javascript scheme", "javascript:alert('xss')", true},
		{"javascript uppercase", "JAVASCRIPT:alert('xss')", true},
		{"javascript mixed case", "JaVaScRiPt:alert('xss')", true},
		{"data scheme", "data:text/html,<script>alert('xss')</script>", true},
		{"data uppercase", "DATA:text/html,test", true},
		{"vbscript scheme", "vbscript:msgbox('xss')", true},
		{"vbscript uppercase", "VBSCRIPT:test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkURLScheme(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkURLScheme(%q): got err %v, want err %v", tt.url, err != nil, tt.wantErr)
			}
		})
	}
}

// TestValidateAuthServerMetaURLs tests validation of AuthServerMeta URL fields.
func TestValidateAuthServerMetaURLs(t *testing.T) {
	validMeta := &AuthServerMeta{
		Issuer:                "https://auth.example.com",
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
		JWKSURI:               "https://auth.example.com/.well-known/jwks.json",
		RegistrationEndpoint:  "https://auth.example.com/register",
		ServiceDocumentation:  "https://docs.example.com",
		OpPolicyURI:           "https://example.com/policy",
		OpTOSURI:              "https://example.com/tos",
		RevocationEndpoint:    "https://auth.example.com/revoke",
		IntrospectionEndpoint: "https://auth.example.com/introspect",
	}

	t.Run("valid metadata", func(t *testing.T) {
		if err := validateAuthServerMetaURLs(validMeta); err != nil {
			t.Errorf("validateAuthServerMetaURLs(): got err %v, want nil", err)
		}
	})

	// Test each URL field with a dangerous scheme
	dangerousFields := []struct {
		name     string
		setField func(*AuthServerMeta)
	}{
		{"authorization_endpoint", func(m *AuthServerMeta) { m.AuthorizationEndpoint = "javascript:alert(1)" }},
		{"token_endpoint", func(m *AuthServerMeta) { m.TokenEndpoint = "javascript:alert(1)" }},
		{"jwks_uri", func(m *AuthServerMeta) { m.JWKSURI = "data:text/html,test" }},
		{"registration_endpoint", func(m *AuthServerMeta) { m.RegistrationEndpoint = "vbscript:test" }},
		{"service_documentation", func(m *AuthServerMeta) { m.ServiceDocumentation = "javascript:void(0)" }},
		{"op_policy_uri", func(m *AuthServerMeta) { m.OpPolicyURI = "javascript:x" }},
		{"op_tos_uri", func(m *AuthServerMeta) { m.OpTOSURI = "data:,test" }},
		{"revocation_endpoint", func(m *AuthServerMeta) { m.RevocationEndpoint = "javascript:1" }},
		{"introspection_endpoint", func(m *AuthServerMeta) { m.IntrospectionEndpoint = "javascript:2" }},
	}

	for _, tt := range dangerousFields {
		t.Run("dangerous "+tt.name, func(t *testing.T) {
			// Copy valid metadata
			meta := *validMeta
			// Set one field to a dangerous value
			tt.setField(&meta)

			err := validateAuthServerMetaURLs(&meta)
			if err == nil {
				t.Errorf("validateAuthServerMetaURLs(): got nil error, want error for dangerous %s", tt.name)
			} else if !strings.Contains(err.Error(), tt.name) {
				t.Errorf("validateAuthServerMetaURLs(): got error %v, want error containing %q", err, tt.name)
			}
		})
	}

	t.Run("empty optional fields are valid", func(t *testing.T) {
		meta := &AuthServerMeta{
			Issuer:                "https://auth.example.com",
			AuthorizationEndpoint: "https://auth.example.com/authorize",
			TokenEndpoint:         "https://auth.example.com/token",
			JWKSURI:               "https://auth.example.com/.well-known/jwks.json",
			// All optional fields left empty
		}
		if err := validateAuthServerMetaURLs(meta); err != nil {
			t.Errorf("validateAuthServerMetaURLs(): got err %v, want nil", err)
		}
	})
}

// TestValidateClientRegistrationURLs tests validation of ClientRegistrationMetadata URL fields.
func TestValidateClientRegistrationURLs(t *testing.T) {
	validMeta := &ClientRegistrationMetadata{
		RedirectURIs: []string{"https://app.example.com/callback", "myapp://callback"},
		ClientURI:    "https://example.com",
		LogoURI:      "https://example.com/logo.png",
		TOSURI:       "https://example.com/tos",
		PolicyURI:    "https://example.com/policy",
		JWKSURI:      "https://example.com/.well-known/jwks.json",
	}

	t.Run("valid metadata", func(t *testing.T) {
		if err := validateClientRegistrationURLs(validMeta); err != nil {
			t.Errorf("validateClientRegistrationURLs(): got err %v, want nil", err)
		}
	})

	t.Run("dangerous redirect_uri", func(t *testing.T) {
		meta := *validMeta
		meta.RedirectURIs = []string{"https://safe.com/cb", "javascript:alert(1)"}

		err := validateClientRegistrationURLs(&meta)
		if err == nil {
			t.Error("validateClientRegistrationURLs(): got nil error, want error for dangerous redirect_uri")
		} else if !strings.Contains(err.Error(), "redirect_uris[1]") {
			t.Errorf("validateClientRegistrationURLs(): got error %v, want error containing \"redirect_uris[1]\"", err)
		}
	})

	// Test each URL field with a dangerous scheme
	dangerousFields := []struct {
		name     string
		setField func(*ClientRegistrationMetadata)
	}{
		{"client_uri", func(m *ClientRegistrationMetadata) { m.ClientURI = "javascript:alert(1)" }},
		{"logo_uri", func(m *ClientRegistrationMetadata) { m.LogoURI = "data:image/svg,<script>alert(1)</script>" }},
		{"tos_uri", func(m *ClientRegistrationMetadata) { m.TOSURI = "vbscript:test" }},
		{"policy_uri", func(m *ClientRegistrationMetadata) { m.PolicyURI = "javascript:void(0)" }},
		{"jwks_uri", func(m *ClientRegistrationMetadata) { m.JWKSURI = "data:application/json,{}" }},
	}

	for _, tt := range dangerousFields {
		t.Run("dangerous "+tt.name, func(t *testing.T) {
			meta := *validMeta
			tt.setField(&meta)

			err := validateClientRegistrationURLs(&meta)
			if err == nil {
				t.Errorf("validateClientRegistrationURLs(): got nil error, want error for dangerous %s", tt.name)
			} else if !strings.Contains(err.Error(), tt.name) {
				t.Errorf("validateClientRegistrationURLs(): got error %v, want error containing %q", err, tt.name)
			}
		})
	}

	t.Run("empty optional fields are valid", func(t *testing.T) {
		meta := &ClientRegistrationMetadata{
			RedirectURIs: []string{"https://app.example.com/callback"},
			// All optional URL fields left empty
		}
		if err := validateClientRegistrationURLs(meta); err != nil {
			t.Errorf("validateClientRegistrationURLs(): got err %v, want nil", err)
		}
	})
}

// TestGetAuthServerMetaRejectsDangerousURLs tests that GetAuthServerMeta rejects
// metadata containing dangerous URL schemes.
func TestGetAuthServerMetaRejectsDangerousURLs(t *testing.T) {
	tests := []struct {
		name        string
		metadata    AuthServerMeta
		wantErrText string
	}{
		{
			name: "javascript authorization_endpoint",
			metadata: AuthServerMeta{
				Issuer:                        "", // Will be set dynamically
				AuthorizationEndpoint:         "javascript:alert('xss')",
				TokenEndpoint:                 "https://auth.example.com/token",
				JWKSURI:                       "https://auth.example.com/.well-known/jwks.json",
				ResponseTypesSupported:        []string{"code"},
				CodeChallengeMethodsSupported: []string{"S256"},
			},
			wantErrText: "authorization_endpoint",
		},
		{
			name: "data token_endpoint",
			metadata: AuthServerMeta{
				Issuer:                        "",
				AuthorizationEndpoint:         "https://auth.example.com/authorize",
				TokenEndpoint:                 "data:text/html,<script>alert(1)</script>",
				JWKSURI:                       "https://auth.example.com/.well-known/jwks.json",
				ResponseTypesSupported:        []string{"code"},
				CodeChallengeMethodsSupported: []string{"S256"},
			},
			wantErrText: "token_endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				meta := tt.metadata
				meta.Issuer = "https://" + r.Host
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(meta)
			}))
			defer server.Close()

			ctx := context.Background()
			issuer := server.URL
			metadataURL := issuer
			_, err := GetAuthServerMeta(ctx, metadataURL, issuer, server.Client())
			if err == nil {
				t.Fatal("GetAuthServerMeta(): got nil error, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Errorf("GetAuthServerMeta(): got error %v, want error containing %q", err, tt.wantErrText)
			}
		})
	}
}

// TestGetProtectedResourceMetadataRejectsDangerousURLs tests that
// GetProtectedResourceMetadata rejects metadata with dangerous authorization server URLs.
func TestGetProtectedResourceMetadataRejectsDangerousURLs(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverURL := "https://" + r.Host
		meta := ProtectedResourceMetadata{
			Resource:             serverURL,
			AuthorizationServers: []string{"javascript:alert('xss')"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meta)
	}))
	defer server.Close()

	ctx := context.Background()
	metadataURL := server.URL + "/.well-known/oauth-protected-resource"
	_, err := GetProtectedResourceMetadata(ctx, metadataURL, server.URL, server.Client())
	if err == nil {
		t.Fatal("GetProtectedResourceMetadata(): got nil error, want error")
	}
	if !strings.Contains(err.Error(), "disallowed scheme") {
		t.Errorf("GetProtectedResourceMetadata(): got error %v, want error containing \"disallowed scheme\"", err)
	}
}

// TestRegisterClientRejectsDangerousURLs tests that RegisterClient rejects
// responses containing dangerous URL schemes.
func TestRegisterClientRejectsDangerousURLs(t *testing.T) {
	tests := []struct {
		name         string
		responseJSON string
		wantErrText  string
	}{
		{
			name: "javascript redirect_uri in response",
			responseJSON: `{
				"client_id": "test-client",
				"redirect_uris": ["javascript:alert(1)"]
			}`,
			wantErrText: "redirect_uris[0]",
		},
		{
			name: "data client_uri",
			responseJSON: `{
				"client_id": "test-client",
				"redirect_uris": ["https://app.example.com/callback"],
				"client_uri": "data:text/html,<script>alert(1)</script>"
			}`,
			wantErrText: "client_uri",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(tt.responseJSON))
			}))
			defer server.Close()

			ctx := context.Background()
			clientMeta := &ClientRegistrationMetadata{
				RedirectURIs: []string{"https://app.example.com/callback"},
			}

			_, err := RegisterClient(ctx, server.URL+"/register", clientMeta, server.Client())
			if err == nil {
				t.Fatal("RegisterClient(): got nil error, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Errorf("RegisterClient(): got error %v, want error containing %q", err, tt.wantErrText)
			}
		})
	}
}
