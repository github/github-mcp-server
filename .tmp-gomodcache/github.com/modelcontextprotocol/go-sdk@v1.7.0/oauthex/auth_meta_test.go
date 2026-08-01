// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package oauthex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthMetaParse(t *testing.T) {
	// Verify that we parse Google's auth server metadata.
	data, err := os.ReadFile(filepath.FromSlash("testdata/google-auth-meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var a AuthServerMeta
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatal(err)
	}
	// Spot check.
	if g, w := a.Issuer, "https://accounts.google.com"; g != w {
		t.Errorf("got %q, want %q", g, w)
	}
}

func TestGetAuthServerMetaPKCESupport(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name                    string
		hasPKCESupport          bool
		wantError               string
		issuerWithTrailingSlash bool
	}{
		{
			name:           "server_with_pkce_support",
			hasPKCESupport: true,
		},
		{
			name:           "server_without_pkce_support",
			hasPKCESupport: false,
			wantError:      "does not implement PKCE",
		},
		{
			// ProtectedResourceMetadata may contain AuthorizationServers with a trailing slash (see Issue #953)
			name:                    "issuer_with_trailing_slash",
			hasPKCESupport:          true,
			issuerWithTrailingSlash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start a fake OAuth 2.1 auth server
			wrapper := http.NewServeMux()
			wrapper.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
				u, _ := url.Parse("https://" + r.Host)
				issuer := "https://localhost:" + u.Port()
				metadata := AuthServerMeta{
					Issuer:                            issuer,
					AuthorizationEndpoint:             issuer + "/authorize",
					TokenEndpoint:                     issuer + "/token",
					RegistrationEndpoint:              issuer + "/register",
					JWKSURI:                           issuer + "/.well-known/jwks.json",
					ScopesSupported:                   []string{"openid", "profile", "email"},
					ResponseTypesSupported:            []string{"code"},
					GrantTypesSupported:               []string{"authorization_code"},
					TokenEndpointAuthMethodsSupported: []string{"none"},
				}

				// Add PKCE support based on test case
				if tt.hasPKCESupport {
					metadata.CodeChallengeMethodsSupported = []string{"S256"}
				}
				// If hasPKCESupport is false, CodeChallengeMethodsSupported remains empty

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(metadata)
			})
			ts := httptest.NewTLSServer(wrapper)
			defer ts.Close()

			// The fake server sets issuer to https://localhost:<port>, so compute that issuer.
			u, _ := url.Parse(ts.URL)
			issuer := "https://localhost:" + u.Port()
			metadataURL := issuer + "/.well-known/oauth-authorization-server"
			if tt.issuerWithTrailingSlash {
				issuer += "/"
			}

			// The fake server presents a cert for example.com; set ServerName accordingly.
			httpClient := ts.Client()
			if tr, ok := httpClient.Transport.(*http.Transport); ok {
				clone := tr.Clone()
				clone.TLSClientConfig.ServerName = "example.com"
				httpClient.Transport = clone
			}

			meta, err := GetAuthServerMeta(ctx, metadataURL, issuer, httpClient)
			if tt.wantError != "" {
				if err == nil {
					t.Fatal("wanted error but got none")
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Errorf("wanted error to contain %q, but got: %v", tt.wantError, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unwanted error: %v", err)
				}
				if meta == nil {
					t.Fatal("wanted metadata but got nil")
				}
				// Verify PKCE support is present
				if len(meta.CodeChallengeMethodsSupported) == 0 {
					t.Error("wanted PKCE support but CodeChallengeMethodsSupported is empty")
				}
			}
		})
	}
}
