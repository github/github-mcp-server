// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package extauth provides OAuth handler implementations for MCP authorization extensions.
// This package implements Enterprise Managed Authorization as defined in SEP-990.

package extauth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

// grantTypeJWTBearer is the grant type for RFC 7523 JWT Bearer authorization grant.
const grantTypeJWTBearer = "urn:ietf:params:oauth:grant-type:jwt-bearer"

// IDTokenFetcher is called to obtain an ID Token from the enterprise IdP.
// This is typically done via OIDC login flow where the user authenticates
// with their enterprise identity provider.
//
// Returns an oauth2.Token where Extra("id_token") contains the OpenID Connect ID Token (JWT).
type IDTokenFetcher func(ctx context.Context) (*oauth2.Token, error)

// EnterpriseHandlerConfig is the configuration for [EnterpriseHandler].
type EnterpriseHandlerConfig struct {
	// IdP configuration (where the user authenticates)

	// IdPIssuerURL is the enterprise IdP's issuer URL (e.g., "https://acme.okta.com").
	// Used for OIDC discovery to find the token endpoint.
	// REQUIRED.
	IdPIssuerURL string

	// IdPCredentials contains the MCP Client's credentials registered at the IdP.
	// REQUIRED. These credentials are used for token exchange at the IdP.
	// The ClientID is always required. ClientSecretAuth is optional and only needed
	// if the IdP requires client authentication (confidential clients).
	IdPCredentials *oauthex.ClientCredentials

	// MCP Server configuration (the resource being accessed)

	// MCPAuthServerURL is the MCP Server's authorization server issuer URL.
	// Used as the audience for token exchange and for metadata discovery.
	// REQUIRED.
	MCPAuthServerURL string

	// MCPResourceURI is the MCP Server's resource identifier (RFC 9728).
	// Used as the resource parameter in token exchange.
	// REQUIRED.
	MCPResourceURI string

	// MCPCredentials contains the MCP Client's credentials registered at the MCP Server.
	// REQUIRED. These credentials are used for JWT Bearer grant at the MCP Server.
	// The ClientID is always required. ClientSecretAuth is optional and only needed
	// if the MCP Server requires client authentication.
	MCPCredentials *oauthex.ClientCredentials

	// MCPScopes is the list of scopes to request at the MCP Server.
	// OPTIONAL.
	MCPScopes []string

	// IDTokenFetcher is called to obtain an ID Token when authorization is needed.
	// The implementation should handle the OIDC login flow (e.g., browser redirect,
	// callback handling) and return the ID token.
	// REQUIRED.
	IDTokenFetcher IDTokenFetcher

	// HTTPClient is an optional HTTP client for customization.
	// If nil, http.DefaultClient is used.
	// OPTIONAL.
	HTTPClient *http.Client
}

// EnterpriseHandler is an implementation of [auth.OAuthHandler] that uses
// Enterprise Managed Authorization (SEP-990) to obtain access tokens.
//
// The flow consists of:
//  1. OIDC Login: User authenticates with enterprise IdP → ID Token
//  2. Token Exchange (RFC 8693): ID Token → ID-JAG at IdP
//  3. JWT Bearer Grant (RFC 7523): ID-JAG → Access Token at MCP Server
type EnterpriseHandler struct {
	config *EnterpriseHandlerConfig

	// tokenSource is the token source obtained after authorization.
	tokenSource oauth2.TokenSource
}

// Compile-time check that EnterpriseHandler implements auth.OAuthHandler.
var _ auth.OAuthHandler = (*EnterpriseHandler)(nil)

// NewEnterpriseHandler creates a new EnterpriseHandler.
// It performs validation of the configuration and returns an error if invalid.
func NewEnterpriseHandler(config *EnterpriseHandlerConfig) (*EnterpriseHandler, error) {
	if config == nil {
		return nil, errors.New("config must be provided")
	}
	if config.IdPIssuerURL == "" {
		return nil, errors.New("IdPIssuerURL is required")
	}
	if config.IdPCredentials == nil {
		return nil, errors.New("IdPCredentials is required")
	}
	if err := config.IdPCredentials.Validate(); err != nil {
		return nil, fmt.Errorf("invalid IdPCredentials: %w", err)
	}
	if config.MCPAuthServerURL == "" {
		return nil, errors.New("MCPAuthServerURL is required")
	}
	if config.MCPResourceURI == "" {
		return nil, errors.New("MCPResourceURI is required")
	}
	if config.MCPCredentials == nil {
		return nil, errors.New("MCPCredentials is required")
	}
	if err := config.MCPCredentials.Validate(); err != nil {
		return nil, fmt.Errorf("invalid MCPCredentials: %w", err)
	}
	if config.IDTokenFetcher == nil {
		return nil, errors.New("IDTokenFetcher is required")
	}
	return &EnterpriseHandler{config: config}, nil
}

// TokenSource returns the token source for outgoing requests.
// Returns nil if authorization has not been performed yet.
func (h *EnterpriseHandler) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	return h.tokenSource, nil
}

// Authorize performs the Enterprise Managed Authorization flow.
// It is called when a request fails with 401 or 403.
func (h *EnterpriseHandler) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)

	httpClient := h.config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Step 1: Get ID Token via the configured fetcher (e.g., OIDC login)
	oidcToken, err := h.config.IDTokenFetcher(ctx)
	if err != nil {
		return fmt.Errorf("failed to obtain ID token: %w", err)
	}

	// Extract ID token from the oauth2.Token
	idToken, ok := oidcToken.Extra("id_token").(string)
	if !ok || idToken == "" {
		return fmt.Errorf("id_token not found in OIDC token response")
	}

	// Step 2: Discover IdP token endpoint via OIDC discovery
	idpMeta, err := auth.GetAuthServerMetadata(ctx, h.config.IdPIssuerURL, httpClient)
	if err != nil {
		return fmt.Errorf("failed to discover IdP metadata: %w", err)
	}
	if idpMeta == nil {
		return fmt.Errorf("no authorization server metadata found for IdP: %s", h.config.IdPIssuerURL)
	}

	// Step 3: Token Exchange (ID Token → ID-JAG)
	tokenExchangeReq := &oauthex.TokenExchangeRequest{
		RequestedTokenType: oauthex.TokenTypeIDJAG,
		Audience:           h.config.MCPAuthServerURL,
		Resource:           h.config.MCPResourceURI,
		Scope:              h.config.MCPScopes,
		SubjectToken:       idToken,
		SubjectTokenType:   oauthex.TokenTypeIDToken,
	}

	idJAGToken, err := oauthex.ExchangeToken(
		ctx,
		idpMeta.TokenEndpoint,
		tokenExchangeReq,
		h.config.IdPCredentials,
		httpClient,
	)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	// Step 4: Discover MCP Server token endpoint
	mcpMeta, err := auth.GetAuthServerMetadata(ctx, h.config.MCPAuthServerURL, httpClient)
	if err != nil {
		return fmt.Errorf("failed to discover MCP auth server metadata: %w", err)
	}
	if mcpMeta == nil {
		return fmt.Errorf("no authorization server metadata found for MCP server: %s", h.config.MCPAuthServerURL)
	}

	// Step 5: JWT Bearer Grant (ID-JAG → Access Token)
	// The ID-JAG is in the AccessToken field of the token (despite the name)
	accessToken, err := exchangeJWTBearer(
		ctx,
		mcpMeta.TokenEndpoint,
		idJAGToken.AccessToken,
		h.config.MCPCredentials,
		httpClient,
	)
	if err != nil {
		return fmt.Errorf("JWT bearer grant failed: %w", err)
	}

	// Store the token source for subsequent requests
	h.tokenSource = oauth2.StaticTokenSource(accessToken)
	return nil
}

// exchangeJWTBearer exchanges an Identity Assertion JWT Authorization Grant (ID-JAG)
// for an access token using JWT Bearer Grant per RFC 7523.
func exchangeJWTBearer(
	ctx context.Context,
	tokenEndpoint string,
	assertion string,
	clientCreds *oauthex.ClientCredentials,
	httpClient *http.Client,
) (*oauth2.Token, error) {
	cfg := &oauth2.Config{
		ClientID: clientCreds.ClientID,
		Endpoint: oauth2.Endpoint{
			TokenURL:  tokenEndpoint,
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
	// Set ClientSecret if ClientSecretAuth is configured
	if clientCreds.ClientSecretAuth != nil {
		cfg.ClientSecret = clientCreds.ClientSecretAuth.ClientSecret
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	ctxWithClient := context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	token, err := cfg.Exchange(
		ctxWithClient,
		"",
		oauth2.SetAuthURLParam("grant_type", grantTypeJWTBearer),
		oauth2.SetAuthURLParam("assertion", assertion),
	)
	if err != nil {
		return nil, fmt.Errorf("JWT bearer grant request failed: %w", err)
	}

	return token, nil
}
