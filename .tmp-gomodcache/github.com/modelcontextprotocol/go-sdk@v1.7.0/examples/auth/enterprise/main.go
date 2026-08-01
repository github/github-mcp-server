// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/auth/extauth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

var (
	// IdP (Identity Provider) configuration.
	idpIssuerURL    = flag.String("idp_issuer", "https://your-idp.okta.com", "IdP issuer URL (e.g., https://your-company.okta.com)")
	idpClientID     = flag.String("idp_client_id", "", "Client ID registered at the IdP")
	idpClientSecret = flag.String("idp_client_secret", "", "Client secret at the IdP (optional for public clients)")

	// MCP Server configuration.
	mcpServerURL     = flag.String("mcp_server", "http://localhost:8000/mcp", "URL of the MCP server")
	mcpAuthServerURL = flag.String("mcp_auth_server", "https://auth.mcpserver.example", "MCP server's authorization server URL")
	mcpResourceURI   = flag.String("mcp_resource_uri", "https://mcp.mcpserver.example", "MCP server's resource identifier (RFC 9728)")
	mcpClientID      = flag.String("mcp_client_id", "", "Client ID at the MCP server (optional)")
	mcpClientSecret  = flag.String("mcp_client_secret", "", "Client secret at the MCP server (optional)")

	// OAuth callback configuration.
	callbackPort = flag.Int("callback_port", 3142, "Port for the local HTTP server that will receive the OAuth callback")
)

// codeReceiver handles the OAuth callback from the IdP's authorization endpoint.
// It starts a local HTTP server to receive the authorization code after the user
// authenticates with their enterprise IdP.
type codeReceiver struct {
	authChan chan *auth.AuthorizationResult
	errChan  chan error
	server   *http.Server
}

// serveRedirectHandler starts an HTTP server to handle the OAuth redirect callback.
func (r *codeReceiver) serveRedirectHandler(listener net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		// Extract the authorization code and state from the callback URL.
		r.authChan <- &auth.AuthorizationResult{
			Code:  req.URL.Query().Get("code"),
			State: req.URL.Query().Get("state"),
		}
		fmt.Fprint(w, "Authentication successful. You can close this window.")
	})

	r.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", *callbackPort),
		Handler: mux,
	}
	if err := r.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		r.errChan <- err
	}
}

// getAuthorizationCode implements the AuthorizationCodeFetcher interface.
// It displays the authorization URL to the user and waits for the callback.
func (r *codeReceiver) getAuthorizationCode(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
	fmt.Printf("\nPlease open the following URL in your browser to authenticate:\n%s\n\n", args.URL)
	select {
	case authRes := <-r.authChan:
		return authRes, nil
	case err := <-r.errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// close shuts down the HTTP server.
func (r *codeReceiver) close() {
	if r.server != nil {
		r.server.Close()
	}
}

func main() {
	flag.Parse()

	// Validate required configuration.
	if *idpClientID == "" {
		log.Fatal("--idp_client_id is required")
	}

	// Set up the OAuth callback receiver.
	receiver := &codeReceiver{
		authChan: make(chan *auth.AuthorizationResult),
		errChan:  make(chan error),
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *callbackPort))
	if err != nil {
		log.Fatalf("failed to listen on port %d: %v", *callbackPort, err)
	}
	go receiver.serveRedirectHandler(listener)
	defer receiver.close()

	log.Printf("OAuth callback server listening on http://localhost:%d", *callbackPort)

	// Create an ID Token fetcher that performs OIDC login with the enterprise IdP.
	idTokenFetcher := func(ctx context.Context) (*oauth2.Token, error) {
		log.Println("Starting OIDC login flow...")

		creds := &oauthex.ClientCredentials{
			ClientID: *idpClientID,
		}
		if *idpClientSecret != "" {
			creds.ClientSecretAuth = &oauthex.ClientSecretAuth{
				ClientSecret: *idpClientSecret,
			}
		}

		oidcConfig := &extauth.OIDCLoginConfig{
			IssuerURL:   *idpIssuerURL,
			Credentials: creds,
			RedirectURL: fmt.Sprintf("http://localhost:%d", *callbackPort),
			Scopes:      []string{"openid", "profile", "email"},
		}

		// PerformOIDCLogin handles the complete OIDC Authorization Code flow with PKCE.
		tokens, err := extauth.PerformOIDCLogin(ctx, oidcConfig, receiver.getAuthorizationCode)
		if err != nil {
			return nil, fmt.Errorf("OIDC login failed: %w", err)
		}

		log.Println("OIDC login successful, obtained ID token")
		return tokens, nil
	}

	// Create the Enterprise Handler.
	// This handler implements the complete Enterprise Managed Authorization flow:
	// 1. OIDC Login: User authenticates with enterprise IdP → ID Token (via idTokenFetcher).
	// 2. Token Exchange (RFC 8693): ID Token → ID-JAG at IdP.
	// 3. JWT Bearer Grant (RFC 7523): ID-JAG → Access Token at MCP Server.
	log.Println("Creating enterprise authorization handler...")

	// Prepare IdP credentials
	idpCreds := &oauthex.ClientCredentials{
		ClientID: *idpClientID,
	}
	if *idpClientSecret != "" {
		idpCreds.ClientSecretAuth = &oauthex.ClientSecretAuth{
			ClientSecret: *idpClientSecret,
		}
	}

	// Prepare MCP credentials
	var mcpCreds *oauthex.ClientCredentials
	if *mcpClientID != "" {
		mcpCreds = &oauthex.ClientCredentials{
			ClientID: *mcpClientID,
		}
		if *mcpClientSecret != "" {
			mcpCreds.ClientSecretAuth = &oauthex.ClientSecretAuth{
				ClientSecret: *mcpClientSecret,
			}
		}
	}

	enterpriseHandler, err := extauth.NewEnterpriseHandler(&extauth.EnterpriseHandlerConfig{
		// IdP configuration (where the user authenticates).
		IdPIssuerURL:   *idpIssuerURL,
		IdPCredentials: idpCreds,

		// MCP Server configuration (the resource being accessed).
		MCPAuthServerURL: *mcpAuthServerURL,
		MCPResourceURI:   *mcpResourceURI,
		MCPCredentials:   mcpCreds,
		MCPScopes:        []string{"read", "write"},

		// ID Token fetcher (performs OIDC login when needed).
		IDTokenFetcher: idTokenFetcher,
	})
	if err != nil {
		log.Fatalf("failed to create enterprise handler: %v", err)
	}

	// Create the MCP client transport with the enterprise handler.
	transport := &mcp.StreamableClientTransport{
		Endpoint:     *mcpServerURL,
		OAuthHandler: enterpriseHandler,
	}

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "enterprise-client-example",
		Version: "1.0.0",
	}, nil)

	log.Printf("Connecting to MCP server at %s...", *mcpServerURL)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("failed to connect to MCP server: %v", err)
	}
	defer session.Close()

	log.Println("Successfully connected to MCP server!")

	// List available tools as a demonstration.
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("failed to list tools: %v", err)
	}

	log.Println("\nAvailable tools:")
	if len(tools.Tools) == 0 {
		log.Println("  (no tools available)")
	} else {
		for _, tool := range tools.Tools {
			log.Printf("  - %q: %s", tool.Name, tool.Description)
		}
	}
}
