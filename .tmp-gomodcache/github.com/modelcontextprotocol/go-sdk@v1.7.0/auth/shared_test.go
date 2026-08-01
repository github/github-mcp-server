// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/internal/oauthtest"
)

func TestGetAuthServerMetadata(t *testing.T) {
	tests := []struct {
		name           string
		issuerPath     string
		endpointConfig *oauthtest.MetadataEndpointConfig
		wantNil        bool
	}{
		{
			name:       "OAuthEndpoint_Root",
			issuerPath: "",
			endpointConfig: &oauthtest.MetadataEndpointConfig{
				ServeOAuthInsertedEndpoint: true,
			},
		},
		{
			name:       "OpenIDEndpoint_Root",
			issuerPath: "",
			endpointConfig: &oauthtest.MetadataEndpointConfig{
				ServeOpenIDInsertedEndpoint: true,
			},
		},
		{
			name:       "OAuthEndpoint_Path",
			issuerPath: "/oauth",
			endpointConfig: &oauthtest.MetadataEndpointConfig{
				ServeOAuthInsertedEndpoint: true,
			},
		},
		{
			name:       "OpenIDEndpoint_Path",
			issuerPath: "/openid",
			endpointConfig: &oauthtest.MetadataEndpointConfig{
				ServeOpenIDInsertedEndpoint: true,
			},
		},
		{
			name:       "OpenIDAppendedEndpoint_Path",
			issuerPath: "/openid",
			endpointConfig: &oauthtest.MetadataEndpointConfig{
				ServeOpenIDAppendedEndpoint: true,
			},
		},
		{
			name:       "NoMetadata",
			issuerPath: "",
			endpointConfig: &oauthtest.MetadataEndpointConfig{
				// All metadata endpoints disabled.
				ServeOAuthInsertedEndpoint:  false,
				ServeOpenIDInsertedEndpoint: false,
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := oauthtest.NewFakeAuthorizationServer(oauthtest.Config{
				IssuerPath:             tt.issuerPath,
				MetadataEndpointConfig: tt.endpointConfig,
			})
			s.Start(t)
			issuerURL := s.URL() + tt.issuerPath

			got, err := GetAuthServerMetadata(t.Context(), issuerURL, http.DefaultClient)
			if tt.wantNil {
				// When no metadata is found, GetAuthServerMetadata returns (nil, nil).
				if err != nil {
					t.Fatalf("GetAuthServerMetadata() unexpected error = %v, want nil", err)
				}
				if got != nil {
					t.Fatal("GetAuthServerMetadata() expected nil for no metadata, got metadata")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetAuthServerMetadata() error = %v, want nil", err)
			}
			if got == nil {
				t.Fatal("GetAuthServerMetadata() got nil, want metadata")
			}
			if got.Issuer != issuerURL {
				t.Errorf("GetAuthServerMetadata() issuer = %q, want %q", got.Issuer, issuerURL)
			}
		})
	}
}
