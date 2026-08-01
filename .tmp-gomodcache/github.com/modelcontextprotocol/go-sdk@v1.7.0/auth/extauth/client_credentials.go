// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package extauth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/internal/authutil"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// ClientCredentialsHandlerConfig is the configuration for [ClientCredentialsHandler].
type ClientCredentialsHandlerConfig struct {
	// Credentials contains the pre-registered client ID and secret.
	// REQUIRED. Both ClientID and ClientSecretAuth must be set, since the
	// client credentials grant requires a confidential client.
	Credentials *oauthex.ClientCredentials

	// HTTPClient is an optional HTTP client for customization.
	// If nil, http.DefaultClient is used.
	// OPTIONAL.
	HTTPClient *http.Client
}

// ClientCredentialsHandler is an implementation of [auth.OAuthHandler] that
// uses the OAuth 2.0 Client Credentials grant (RFC 6749 Section 4.4) to
// obtain access tokens.
//
// This handler is intended for service-to-service authentication where the
// client has pre-registered credentials (client ID and secret) and does not
// require user interaction. It bypasses both dynamic client registration and
// the authorization code flow.
//
// The token endpoint and scopes are discovered automatically via Protected
// Resource Metadata (RFC 9728) and Authorization Server Metadata (RFC 8414),
// following the ext-auth specification SEP-1046.
type ClientCredentialsHandler struct {
	config      *ClientCredentialsHandlerConfig
	tokenSource oauth2.TokenSource

	// grantedScopes maps authorization server issuer to the list of scopes granted by that issuer.
	grantedScopes map[string][]string
}

// Compile-time check that ClientCredentialsHandler implements auth.OAuthHandler.
var _ auth.OAuthHandler = (*ClientCredentialsHandler)(nil)

// NewClientCredentialsHandler creates a new ClientCredentialsHandler.
// It validates the configuration and returns an error if invalid.
func NewClientCredentialsHandler(config *ClientCredentialsHandlerConfig) (*ClientCredentialsHandler, error) {
	if config == nil {
		return nil, fmt.Errorf("config must be provided")
	}
	if config.Credentials == nil {
		return nil, fmt.Errorf("credentials are required")
	}
	if err := config.Credentials.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}
	if config.Credentials.ClientSecretAuth == nil {
		return nil, fmt.Errorf("clientSecretAuth is required for client credentials grant")
	}
	return &ClientCredentialsHandler{
		config:        config,
		grantedScopes: make(map[string][]string),
	}, nil
}

// TokenSource returns the token source for outgoing requests.
// Returns nil if authorization has not been performed yet.
func (h *ClientCredentialsHandler) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	return h.tokenSource, nil
}

// Authorize performs the Client Credentials grant to obtain an access token.
// It is called when a request fails with 401 or 403.
//
// The flow follows the ext-auth specification SEP-1046:
//  1. Discover Protected Resource Metadata from the request URL
//  2. Discover Authorization Server Metadata from PRM
//  3. Exchange client credentials for an access token at the token endpoint
func (h *ClientCredentialsHandler) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)

	httpClient := h.config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Step 1: Discover Protected Resource Metadata.
	wwwChallenges, err := oauthex.ParseWWWAuthenticate(resp.Header[http.CanonicalHeaderKey("WWW-Authenticate")])
	if err != nil {
		return fmt.Errorf("failed to parse WWW-Authenticate header: %v", err)
	}

	prm, err := getProtectedResourceMetadata(ctx, wwwChallenges, req.URL.String(), httpClient)
	if err != nil {
		return err
	}

	if len(prm.AuthorizationServers) == 0 {
		return fmt.Errorf("protected resource metadata has no authorization servers specified")
	}

	// Step 2: Discover Authorization Server Metadata.
	asm, err := auth.GetAuthServerMetadata(ctx, prm.AuthorizationServers[0], httpClient)
	if err != nil {
		return fmt.Errorf("failed to get authorization server metadata: %w", err)
	}
	if asm == nil {
		// Fallback to 2025-03-26 spec: predefined endpoints.
		authServerURL := prm.AuthorizationServers[0]
		asm = &oauthex.AuthServerMeta{
			Issuer:        authServerURL,
			TokenEndpoint: authServerURL + "/token",
		}
	}

	creds := h.config.Credentials
	if creds.Issuer != "" && !authutil.IssuersEqual(creds.Issuer, asm.Issuer) {
		return fmt.Errorf("authorization server issuer %q does not match pre-registered credentials issuer %q", asm.Issuer, creds.Issuer)
	}

	// Determine requestedScopes: use PRM's scopes_supported if available.
	requestedScopes := scopesFromChallenges(wwwChallenges)
	if len(requestedScopes) == 0 && len(prm.ScopesSupported) > 0 {
		requestedScopes = prm.ScopesSupported
	}

	// Accumulate scopes: union previously granted scopes with the newly
	// challenged scopes so that step-up authorization does not lose
	// permissions granted in earlier rounds (SEP-2350).
	requestedScopes = authutil.UnionScopes(h.grantedScopes[asm.Issuer], requestedScopes)

	// Step 3: Exchange client credentials for an access token.
	cfg := &clientcredentials.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecretAuth.ClientSecret,
		TokenURL:     asm.TokenEndpoint,
		Scopes:       requestedScopes,
		AuthStyle:    selectTokenAuthMethod(asm.TokenEndpointAuthMethodsSupported),
	}

	ctxWithClient := context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	h.tokenSource = cfg.TokenSource(ctxWithClient)

	return h.updateGrantedScopes(asm.Issuer, requestedScopes)
}

func (h *ClientCredentialsHandler) updateGrantedScopes(issuer string, requestedScopes []string) error {
	if h.tokenSource == nil {
		return nil
	}
	tok, err := h.tokenSource.Token()
	if err != nil {
		h.tokenSource = nil
		return fmt.Errorf("client credentials token request failed: %w", err)
	}
	if tokenScopes := authutil.ScopesFromToken(tok); tokenScopes == nil {
		h.grantedScopes[issuer] = requestedScopes
	} else {
		h.grantedScopes[issuer] = tokenScopes
	}
	return nil
}

// getProtectedResourceMetadata discovers Protected Resource Metadata (RFC 9728)
// from the request URL. This mirrors the logic in AuthorizationCodeHandler.
func getProtectedResourceMetadata(ctx context.Context, wwwChallenges []oauthex.Challenge, mcpServerURL string, httpClient *http.Client) (*oauthex.ProtectedResourceMetadata, error) {
	// Use MCP server URL as the resource URI per
	// https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization#canonical-server-uri.
	for _, u := range protectedResourceMetadataURLs(resourceMetadataURLFromChallenges(wwwChallenges), mcpServerURL) {
		prm, err := oauthex.GetProtectedResourceMetadata(ctx, u.url, u.resource, httpClient)
		if err != nil {
			continue
		}
		if prm == nil {
			continue
		}
		if len(prm.AuthorizationServers) == 0 {
			// If we found PRM, we enforce the 2025-11-25 spec and not search further.
			return nil, fmt.Errorf("protected resource metadata has no authorization servers specified")
		}
		return prm, nil
	}
	// Fallback to 2025-03-26 spec: MCP server root is the Authorization Server.
	// https://modelcontextprotocol.io/specification/2025-03-26/basic/authorization#server-metadata-discovery
	u, err := url.Parse(mcpServerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MCP server URL: %v", err)
	}
	u.Path = ""
	return &oauthex.ProtectedResourceMetadata{
		AuthorizationServers: []string{u.String()},
		Resource:             mcpServerURL,
	}, nil
}

type prmURL struct {
	url      string
	resource string
}

// protectedResourceMetadataURLs returns URLs to try for PRM discovery.
// This mirrors the logic in AuthorizationCodeHandler.
func protectedResourceMetadataURLs(metadataURL, resourceURL string) []prmURL {
	var urls []prmURL
	if metadataURL != "" {
		urls = append(urls, prmURL{url: metadataURL, resource: resourceURL})
	}
	ru, err := url.Parse(resourceURL)
	if err != nil {
		return urls
	}
	mu := *ru
	// At the path of the server's MCP endpoint.
	mu.Path = "/.well-known/oauth-protected-resource/" + strings.TrimLeft(ru.Path, "/")
	urls = append(urls, prmURL{url: mu.String(), resource: resourceURL})
	// At the root.
	mu.Path = "/.well-known/oauth-protected-resource"
	ru.Path = ""
	urls = append(urls, prmURL{url: mu.String(), resource: ru.String()})
	return urls
}

// resourceMetadataURLFromChallenges returns a resource metadata URL from
// WWW-Authenticate challenges, or the empty string if there is none.
func resourceMetadataURLFromChallenges(cs []oauthex.Challenge) string {
	for _, c := range cs {
		if u := c.Params["resource_metadata"]; u != "" {
			return u
		}
	}
	return ""
}

// scopesFromChallenges returns scopes from WWW-Authenticate challenges.
// It only looks at challenges with the "Bearer" scheme.
func scopesFromChallenges(cs []oauthex.Challenge) []string {
	for _, c := range cs {
		if c.Scheme == "bearer" && c.Params["scope"] != "" {
			return strings.Fields(c.Params["scope"])
		}
	}
	return nil
}

// selectTokenAuthMethod selects the preferred token endpoint auth method based on
// the authorization server's supported methods. Prefers client_secret_post over
// client_secret_basic per the OAuth 2.1 draft.
func selectTokenAuthMethod(supported []string) oauth2.AuthStyle {
	prefOrder := []string{
		"client_secret_post",
		"client_secret_basic",
	}
	for _, method := range prefOrder {
		if slices.Contains(supported, method) {
			switch method {
			case "client_secret_post":
				return oauth2.AuthStyleInParams
			case "client_secret_basic":
				return oauth2.AuthStyleInHeader
			}
		}
	}
	return oauth2.AuthStyleAutoDetect
}
