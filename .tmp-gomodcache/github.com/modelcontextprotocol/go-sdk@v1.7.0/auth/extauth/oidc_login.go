// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file implements OIDC Authorization Code flow for obtaining ID tokens
// as part of Enterprise Managed Authorization (SEP-990).
// See https://openid.net/specs/openid-connect-core-1_0.html

package extauth

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

// OIDCLoginConfig configures the OIDC Authorization Code flow for obtaining
// an ID Token. This is used with [PerformOIDCLogin] to authenticate users
// with an enterprise IdP before calling the Enterprise Managed Authorization flow.
type OIDCLoginConfig struct {
	// IssuerURL is the IdP's issuer URL (e.g., "https://acme.okta.com").
	// REQUIRED.
	IssuerURL string
	// Credentials contains the MCP Client's credentials registered at the IdP.
	// The ClientID field is REQUIRED. The ClientSecret field is OPTIONAL
	// (only required if the client is confidential, not a public client).
	// REQUIRED (struct itself), but ClientSecret field can be empty.
	Credentials *oauthex.ClientCredentials
	// RedirectURL is the OAuth2 redirect URI registered with the IdP.
	// This must match exactly what was registered with the IdP.
	// REQUIRED.
	RedirectURL string
	// Scopes are the OAuth2/OIDC scopes to request.
	// "openid" is REQUIRED for OIDC. Common values: ["openid", "profile", "email"]
	// REQUIRED.
	Scopes []string
	// LoginHint is a hint to the IdP about the user's identity.
	// Some IdPs may require this (e.g., as an email address for routing to SSO providers).
	// Example: "user@example.com"
	// OPTIONAL.
	LoginHint string
	// HTTPClient is the HTTP client for making requests.
	// If nil, http.DefaultClient is used.
	// OPTIONAL.
	HTTPClient *http.Client
}

// PerformOIDCLogin performs the complete OIDC Authorization Code flow with PKCE
// in a single function call. This is the recommended approach for obtaining an
// ID Token for use with [EnterpriseHandler].
//
// Returns an oauth2.Token where:
//   - Extra("id_token") contains the OpenID Connect ID Token (JWT)
//   - AccessToken contains the OAuth2 access token (if issued by IdP)
//   - RefreshToken contains the OAuth2 refresh token (if issued by IdP)
//   - TokenType is the token type (typically "Bearer")
//   - Expiry is when the token expires
func PerformOIDCLogin(
	ctx context.Context,
	config *OIDCLoginConfig,
	authCodeFetcher auth.AuthorizationCodeFetcher,
) (*oauth2.Token, error) {
	if authCodeFetcher == nil {
		return nil, fmt.Errorf("authCodeFetcher is required")
	}

	authReq, oauth2Config, err := initiateOIDCLogin(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate OIDC login: %w", err)
	}

	authResult, err := authCodeFetcher(ctx, &auth.AuthorizationArgs{URL: authReq.authURL})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch authorization code: %w", err)
	}

	if authResult.State != authReq.state {
		return nil, fmt.Errorf("state mismatch: expected %q, got %q", authReq.state, authResult.State)
	}

	tokens, err := completeOIDCLogin(ctx, config, oauth2Config, authResult.Code, authReq.codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("failed to complete OIDC login: %w", err)
	}

	return tokens, nil
}

// oidcAuthorizationRequest holds internal state for OIDC authorization.
type oidcAuthorizationRequest struct {
	authURL      string
	state        string
	codeVerifier string
}

// initiateOIDCLogin initiates an OIDC Authorization Code flow with PKCE.
func initiateOIDCLogin(
	ctx context.Context,
	config *OIDCLoginConfig,
) (*oidcAuthorizationRequest, *oauth2.Config, error) {
	if config == nil {
		return nil, nil, fmt.Errorf("config is required")
	}
	if config.IssuerURL == "" {
		return nil, nil, fmt.Errorf("IssuerURL is required")
	}
	if config.Credentials == nil || config.Credentials.ClientID == "" {
		return nil, nil, fmt.Errorf("Credentials.ClientID is required")
	}
	if config.RedirectURL == "" {
		return nil, nil, fmt.Errorf("RedirectURL is required")
	}
	if len(config.Scopes) == 0 {
		return nil, nil, fmt.Errorf("at least one scope is required (must include 'openid')")
	}

	if !slices.Contains(config.Scopes, "openid") {
		return nil, nil, fmt.Errorf("the 'openid' scope is required for OIDC")
	}

	if err := checkURLScheme(config.IssuerURL); err != nil {
		return nil, nil, fmt.Errorf("invalid IssuerURL: %w", err)
	}
	if err := checkURLScheme(config.RedirectURL); err != nil {
		return nil, nil, fmt.Errorf("invalid RedirectURL: %w", err)
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	meta, err := auth.GetAuthServerMetadata(ctx, config.IssuerURL, httpClient)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to discover OIDC metadata: %w", err)
	}
	if meta == nil {
		return nil, nil, fmt.Errorf("no authorization server metadata found for OIDC issuer: %s", config.IssuerURL)
	}
	if meta.AuthorizationEndpoint == "" {
		return nil, nil, fmt.Errorf("authorization_endpoint not found in OIDC metadata")
	}

	codeVerifier := oauth2.GenerateVerifier()
	state := rand.Text()

	oauth2Config := &oauth2.Config{
		ClientID:    config.Credentials.ClientID,
		RedirectURL: config.RedirectURL,
		Scopes:      config.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  meta.AuthorizationEndpoint,
			TokenURL: meta.TokenEndpoint,
		},
	}
	// Set ClientSecret if ClientSecretAuth is configured
	if config.Credentials.ClientSecretAuth != nil {
		oauth2Config.ClientSecret = config.Credentials.ClientSecretAuth.ClientSecret
	}

	authURLOpts := []oauth2.AuthCodeOption{
		oauth2.S256ChallengeOption(codeVerifier),
	}
	if config.LoginHint != "" {
		authURLOpts = append(authURLOpts, oauth2.SetAuthURLParam("login_hint", config.LoginHint))
	}
	authURL := oauth2Config.AuthCodeURL(state, authURLOpts...)

	return &oidcAuthorizationRequest{
		authURL:      authURL,
		state:        state,
		codeVerifier: codeVerifier,
	}, oauth2Config, nil
}

// completeOIDCLogin completes the OIDC Authorization Code flow by exchanging
// the authorization code for tokens.
func completeOIDCLogin(
	ctx context.Context,
	config *OIDCLoginConfig,
	oauth2Config *oauth2.Config,
	authCode string,
	codeVerifier string,
) (*oauth2.Token, error) {
	if authCode == "" {
		return nil, fmt.Errorf("authCode is required")
	}
	if codeVerifier == "" {
		return nil, fmt.Errorf("codeVerifier is required")
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	ctxWithClient := context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	oauth2Token, err := oauth2Config.Exchange(
		ctxWithClient,
		authCode,
		oauth2.VerifierOption(codeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Validate that id_token is present in the response
	idToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, fmt.Errorf("id_token not found in token response")
	}

	return oauth2Token, nil
}

// checkURLScheme ensures that its argument is a valid URL with a scheme
// that prevents XSS attacks.
// See #526.
// Note: a copy of this function exists in oauthex/oauth2.go; keep these in sync.
func checkURLScheme(u string) error {
	if u == "" {
		return nil
	}
	uu, err := url.Parse(u)
	if err != nil {
		return err
	}
	scheme := strings.ToLower(uu.Scheme)
	if scheme == "javascript" || scheme == "data" || scheme == "vbscript" {
		return fmt.Errorf("URL has disallowed scheme %q", scheme)
	}
	return nil
}
