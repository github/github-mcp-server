// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package extauth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/internal/oauthtest"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

func validEnterpriseHandlerConfig() *EnterpriseHandlerConfig {
	return &EnterpriseHandlerConfig{
		IdPIssuerURL:     "https://idp.example.com",
		IdPCredentials:   &oauthex.ClientCredentials{ClientID: "idp_client_id"},
		MCPAuthServerURL: "https://mcp-auth.example.com",
		MCPResourceURI:   "https://mcp.example.com",
		MCPCredentials:   &oauthex.ClientCredentials{ClientID: "mcp_client_id"},
		IDTokenFetcher: func(ctx context.Context) (*oauth2.Token, error) {
			token := &oauth2.Token{AccessToken: "mock", TokenType: "Bearer"}
			return token.WithExtra(map[string]any{"id_token": "mock_id_token"}), nil
		},
	}
}

func TestNewEnterpriseHandler_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    *EnterpriseHandlerConfig
		wantError string
	}{
		{
			name:      "nil config",
			config:    nil,
			wantError: "config must be provided",
		},
		{
			name: "missing IdPIssuerURL",
			config: func() *EnterpriseHandlerConfig {
				c := validEnterpriseHandlerConfig()
				c.IdPIssuerURL = ""
				return c
			}(),
			wantError: "IdPIssuerURL is required",
		},
		{
			name: "nil IdPCredentials",
			config: func() *EnterpriseHandlerConfig {
				c := validEnterpriseHandlerConfig()
				c.IdPCredentials = nil
				return c
			}(),
			wantError: "IdPCredentials is required",
		},
		{
			name: "invalid IdPCredentials - empty ClientID",
			config: func() *EnterpriseHandlerConfig {
				c := validEnterpriseHandlerConfig()
				c.IdPCredentials = &oauthex.ClientCredentials{ClientID: ""}
				return c
			}(),
			wantError: "invalid IdPCredentials",
		},
		{
			name: "invalid IdPCredentials - empty ClientSecret in ClientSecretAuth",
			config: func() *EnterpriseHandlerConfig {
				c := validEnterpriseHandlerConfig()
				c.IdPCredentials = &oauthex.ClientCredentials{
					ClientID:         "idp_client_id",
					ClientSecretAuth: &oauthex.ClientSecretAuth{ClientSecret: ""},
				}
				return c
			}(),
			wantError: "invalid IdPCredentials",
		},
		{
			name: "missing MCPAuthServerURL",
			config: func() *EnterpriseHandlerConfig {
				c := validEnterpriseHandlerConfig()
				c.MCPAuthServerURL = ""
				return c
			}(),
			wantError: "MCPAuthServerURL is required",
		},
		{
			name: "missing MCPResourceURI",
			config: func() *EnterpriseHandlerConfig {
				c := validEnterpriseHandlerConfig()
				c.MCPResourceURI = ""
				return c
			}(),
			wantError: "MCPResourceURI is required",
		},
		{
			name: "nil MCPCredentials",
			config: func() *EnterpriseHandlerConfig {
				c := validEnterpriseHandlerConfig()
				c.MCPCredentials = nil
				return c
			}(),
			wantError: "MCPCredentials is required",
		},
		{
			name: "invalid MCPCredentials - empty ClientID",
			config: func() *EnterpriseHandlerConfig {
				c := validEnterpriseHandlerConfig()
				c.MCPCredentials = &oauthex.ClientCredentials{ClientID: ""}
				return c
			}(),
			wantError: "invalid MCPCredentials",
		},
		{
			name: "missing IDTokenFetcher",
			config: func() *EnterpriseHandlerConfig {
				c := validEnterpriseHandlerConfig()
				c.IDTokenFetcher = nil
				return c
			}(),
			wantError: "IDTokenFetcher is required",
		},
		{
			name:      "valid public clients",
			config:    validEnterpriseHandlerConfig(),
			wantError: "",
		},
		{
			name: "valid confidential clients",
			config: func() *EnterpriseHandlerConfig {
				c := validEnterpriseHandlerConfig()
				c.IdPCredentials.ClientSecretAuth = &oauthex.ClientSecretAuth{ClientSecret: "idp_secret"}
				c.MCPCredentials.ClientSecretAuth = &oauthex.ClientSecretAuth{ClientSecret: "mcp_secret"}
				return c
			}(),
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewEnterpriseHandler(tt.config)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantError)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if handler == nil {
					t.Fatal("expected handler to be non-nil")
				}
			}
		})
	}
}

func TestEnterpriseHandler_Authorize_E2E(t *testing.T) {
	const idJAGToken = "id-jag-token-from-idp"

	idpServer := oauthtest.NewFakeIdPServer(oauthtest.IdPConfig{
		PreregisteredClients: map[string]oauthtest.ClientInfo{
			"idp_client_id": {Secret: "idp_secret"},
		},
		TokenExchangeConfig: &oauthtest.TokenExchangeConfig{
			IDJAGToken: idJAGToken,
		},
	})
	idpServer.Start(t)

	mcpAuthServer := oauthtest.NewFakeAuthorizationServer(oauthtest.Config{
		RegistrationConfig: &oauthtest.RegistrationConfig{
			PreregisteredClients: map[string]oauthtest.ClientInfo{
				"mcp_client_id": {Secret: "mcp_secret"},
			},
		},
		JWTBearerConfig: &oauthtest.JWTBearerConfig{
			ValidAssertions: []string{idJAGToken},
		},
	})
	mcpAuthServer.Start(t)

	config := validEnterpriseHandlerConfig()
	config.IdPIssuerURL = idpServer.URL()
	config.IdPCredentials.ClientSecretAuth = &oauthex.ClientSecretAuth{ClientSecret: "idp_secret"}
	config.MCPAuthServerURL = mcpAuthServer.URL()
	config.MCPCredentials.ClientSecretAuth = &oauthex.ClientSecretAuth{ClientSecret: "mcp_secret"}
	config.MCPScopes = []string{"read", "write"}
	config.IDTokenFetcher = func(ctx context.Context) (*oauth2.Token, error) {
		token := &oauth2.Token{AccessToken: "mock_access_token", TokenType: "Bearer"}
		return token.WithExtra(map[string]any{"id_token": "mock_id_token_from_user_login"}), nil
	}

	handler, err := NewEnterpriseHandler(config)
	if err != nil {
		t.Fatalf("NewEnterpriseHandler failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api", nil)
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    req,
	}

	if err := handler.Authorize(context.Background(), req, resp); err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenSource, err := handler.TokenSource(context.Background())
	if err != nil {
		t.Fatalf("TokenSource failed: %v", err)
	}
	if tokenSource == nil {
		t.Fatal("expected token source to be set after authorization")
	}

	token, err := tokenSource.Token()
	if err != nil {
		t.Fatalf("Token() failed: %v", err)
	}
	if token.AccessToken != "test_access_token" {
		t.Errorf("AccessToken = %q, want %q", token.AccessToken, "test_access_token")
	}
}

func TestEnterpriseHandler_Authorize_IDTokenFetcherError(t *testing.T) {
	config := validEnterpriseHandlerConfig()
	config.IDTokenFetcher = func(ctx context.Context) (*oauth2.Token, error) {
		return nil, fmt.Errorf("user cancelled login")
	}

	handler, err := NewEnterpriseHandler(config)
	if err != nil {
		t.Fatalf("NewEnterpriseHandler failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "https://mcp.example.com/api", nil)
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    req,
	}

	err = handler.Authorize(context.Background(), req, resp)
	if err == nil {
		t.Fatal("expected error from Authorize, got nil")
	}
	if !strings.Contains(err.Error(), "failed to obtain ID token") {
		t.Errorf("error = %v, want error containing %q", err, "failed to obtain ID token")
	}
}

func TestEnterpriseHandler_TokenSource_BeforeAuthorization(t *testing.T) {
	handler, err := NewEnterpriseHandler(validEnterpriseHandlerConfig())
	if err != nil {
		t.Fatalf("NewEnterpriseHandler failed: %v", err)
	}

	tokenSource, err := handler.TokenSource(context.Background())
	if err != nil {
		t.Fatalf("TokenSource failed: %v", err)
	}
	if tokenSource != nil {
		t.Errorf("expected nil token source before authorization, got %v", tokenSource)
	}
}
