// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/internal/authutil"
	"github.com/modelcontextprotocol/go-sdk/internal/util"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

// ClientIDMetadataDocumentConfig is used to configure the Client ID Metadata Document
// based client registration per
// https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization#client-id-metadata-documents.
// See https://client.dev/ for more information.
type ClientIDMetadataDocumentConfig struct {
	// URL is the client identifier URL as per
	// https://datatracker.ietf.org/doc/html/draft-ietf-oauth-client-id-metadata-document-00#section-3.
	URL string
}

// DynamicClientRegistrationConfig is used to configure dynamic client registration per
// https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization#dynamic-client-registration.
type DynamicClientRegistrationConfig struct {
	// Metadata to be used in dynamic client registration request as per
	// https://datatracker.ietf.org/doc/html/rfc7591#section-2.
	//
	// If Metadata.ApplicationType is empty, it will be inferred from
	// Metadata.RedirectURIs. When set, it will be validated against the inferred type
	// and an error will be returned if they conflict.
	Metadata *oauthex.ClientRegistrationMetadata
}

// AuthorizationResult is the result of an authorization flow.
// It is returned by [AuthorizationCodeHandler].AuthorizationCodeFetcher implementations.
type AuthorizationResult struct {
	// Code is the authorization code obtained from the authorization server.
	Code string
	// State string returned by the authorization server.
	State string
	// Iss is the issuer identifier returned by the authorization server in the
	// authorization response per [RFC 9207]. The AuthorizationCodeFetcher should
	// populate this from the "iss" query parameter in the redirect URI if present.
	//
	// [RFC 9207]: https://www.rfc-editor.org/rfc/rfc9207
	Iss string
}

// AuthorizationArgs is the input to [AuthorizationCodeFetcher].
type AuthorizationArgs struct {
	// Authorization URL to be opened in a browser for the user to start the authorization process.
	URL string
}

// AuthorizationCodeFetcher is called to initiate the OAuth authorization flow.
// It is responsible for directing the user to the authorization URL (e.g., opening
// in a browser) and returning the authorization code and state once the Authorization
// Server redirects back to the configured RedirectURL.
type AuthorizationCodeFetcher func(ctx context.Context, args *AuthorizationArgs) (*AuthorizationResult, error)

// AuthorizationCodeHandlerConfig is the configuration for [AuthorizationCodeHandler].
type AuthorizationCodeHandlerConfig struct {
	// Client registration configuration.
	// It is attempted in the following order:
	//  1. Client ID Metadata Document
	//  2. Preregistration
	//  3. Dynamic Client Registration
	// At least one method must be configured.
	ClientIDMetadataDocumentConfig  *ClientIDMetadataDocumentConfig
	PreregisteredClient             *oauthex.ClientCredentials
	DynamicClientRegistrationConfig *DynamicClientRegistrationConfig

	// RedirectURL is a required URL to redirect to after authorization.
	// The caller is responsible for handling the redirect out of band.
	//
	// If Dynamic Client Registration is used:
	//  - this field is permitted to be empty, in which case it will be set
	//    to the first redirect URI from
	//    DynamicClientRegistrationConfig.Metadata.RedirectURIs.
	//  - if the field is not empty, it must be one of the redirect URIs in
	//    DynamicClientRegistrationConfig.Metadata.RedirectURIs.
	RedirectURL string

	// AuthorizationCodeFetcher is a required function called to initiate the authorization flow.
	// See [AuthorizationCodeFetcher] for details.
	AuthorizationCodeFetcher AuthorizationCodeFetcher

	// RequestRefreshToken indicates that the client intends to use refresh
	// tokens and is capable of storing them securely.
	//
	// When true and the Authorization Server metadata contains "offline_access"
	// in its scopes_supported, the client adds "offline_access" to the
	// requested scopes.
	//
	// When using Dynamic Client Registration, callers should include
	// "refresh_token" in [DynamicClientRegistrationConfig].Metadata.GrantTypes
	// directly to advertise refresh token support to the Authorization Server.
	//
	// When using Client ID Metadata Document, the document hosted at the
	// Client ID URL should include "refresh_token" in its grant_types.
	//
	// See https://modelcontextprotocol.io/seps/2207-oidc-refresh-token-guidance.
	RequestRefreshToken bool

	// Client is an optional HTTP client to use for HTTP requests.
	// It is used for the following requests:
	//  - Fetching Protected Resource Metadata
	//  - Fetching Authorization Server Metadata
	//  - Registering a client dynamically
	//  - Exchanging an authorization code for an access token
	//  - Refreshing an access token
	// Custom clients can include additional security configurations,
	// such as SSRF protections, see
	// https://modelcontextprotocol.io/docs/tutorials/security/security_best_practices#server-side-request-forgery-ssrf
	// If not provided, http.DefaultClient will be used.
	Client *http.Client

	// NewTokenSource is an optional function that can be set to construct the
	// token source that will be used by the [AuthorizationCodeHandler]. If
	// non-nil, it is called after the authorization code is successfully
	// exchanged for a token in [AuthorizationCodeHandler.Authorize]
	// to obtain the [oauth2.TokenSource] returned by
	// [AuthorizationCodeHandler.TokenSource]. Implementations must use the
	// provided context, which is properly configured for constructing a
	// TokenSource. The default is to call [oauth2.Config.TokenSource].
	NewTokenSource func(context.Context, *oauth2.Config, *oauth2.Token) (oauth2.TokenSource, error)

	// InitialTokenSource is an optional field that can be set to inject the
	// token source that will be used by the [AuthorizationCodeHandler]. If
	// non-nil, it is set as the token source that will be returned by
	// [AuthorizationCodeHandler.TokenSource] during handler initialization.
	// The default is nil, which means no token source has been set initially,
	// and will trigger a call to [AuthorizationCodeHandler.Authorize].
	InitialTokenSource oauth2.TokenSource
}

// AuthorizationCodeHandler is an implementation of [OAuthHandler] that uses
// the authorization code flow to obtain access tokens.
type AuthorizationCodeHandler struct {
	config *AuthorizationCodeHandlerConfig

	// mu protects concurrent access to tokenSource and grantedScopes.
	mu sync.RWMutex

	// tokenSource is the token source to use for authorization.
	tokenSource oauth2.TokenSource

	// grantedScopes maps authorization server issuer to the list of scopes granted by that issuer.
	grantedScopes map[string][]string
}

var _ OAuthHandler = (*AuthorizationCodeHandler)(nil)

func (h *AuthorizationCodeHandler) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.tokenSource, nil
}

// NewAuthorizationCodeHandler creates a new AuthorizationCodeHandler.
// It performs validation of the configuration and returns an error if it is invalid.
// The passed config is consumed by the handler and should not be modified after.
func NewAuthorizationCodeHandler(config *AuthorizationCodeHandlerConfig) (*AuthorizationCodeHandler, error) {
	if config == nil {
		return nil, errors.New("config must be provided")
	}
	if config.ClientIDMetadataDocumentConfig == nil &&
		config.PreregisteredClient == nil &&
		config.DynamicClientRegistrationConfig == nil {
		return nil, errors.New("at least one client registration configuration must be provided")
	}
	if config.AuthorizationCodeFetcher == nil {
		return nil, errors.New("AuthorizationCodeFetcher is required")
	}
	if config.ClientIDMetadataDocumentConfig != nil && !isNonRootHTTPSURL(config.ClientIDMetadataDocumentConfig.URL) {
		return nil, fmt.Errorf("client ID metadata document URL must be a non-root HTTPS URL")
	}
	if config.PreregisteredClient != nil {
		if err := config.PreregisteredClient.Validate(); err != nil {
			return nil, fmt.Errorf("invalid PreregisteredClient configuration: %w", err)
		}
	}
	dCfg := config.DynamicClientRegistrationConfig
	if dCfg != nil {
		if dCfg.Metadata == nil {
			return nil, errors.New("dynamic client registration requires non-nil Metadata")
		}
		if len(dCfg.Metadata.RedirectURIs) == 0 {
			return nil, errors.New("Metadata.RedirectURIs is required for dynamic client registration")
		}
		if config.RedirectURL == "" {
			config.RedirectURL = dCfg.Metadata.RedirectURIs[0]
		} else if !slices.Contains(dCfg.Metadata.RedirectURIs, config.RedirectURL) {
			return nil, fmt.Errorf("RedirectURL %q is not in the list of allowed redirect URIs for dynamic client registration", config.RedirectURL)
		}
		applicationType := inferApplicationType(dCfg.Metadata.RedirectURIs)
		if dCfg.Metadata.ApplicationType == "" {
			dCfg.Metadata.ApplicationType = applicationType
		} else if dCfg.Metadata.ApplicationType != applicationType {
			return nil, fmt.Errorf("application type %q conflicts with the application type inferred from redirect URIs", dCfg.Metadata.ApplicationType)
		}
	}
	if config.RedirectURL == "" {
		// If the RedirectURL was supposed to be set by the dynamic client registration,
		// it should have been set by now. Otherwise, it is required.
		return nil, errors.New("RedirectURL is required")
	}
	if config.Client == nil {
		config.Client = http.DefaultClient
	}
	return &AuthorizationCodeHandler{
		config:        config,
		tokenSource:   config.InitialTokenSource,
		grantedScopes: make(map[string][]string),
	}, nil
}

func isNonRootHTTPSURL(u string) bool {
	pu, err := url.Parse(u)
	if err != nil {
		return false
	}
	return pu.Scheme == "https" && pu.Path != ""
}

// inferApplicationType returns an application type based on the redirect URIs.
func inferApplicationType(redirectURIs []string) string {
	hasNative := false
	hasWeb := false
	for _, uri := range redirectURIs {
		u, err := url.Parse(uri)
		if err != nil {
			return ""
		}
		switch u.Scheme {
		case "http", "https":
			if util.IsLoopback(u.Hostname()) {
				hasNative = true
			} else {
				hasWeb = true
			}
		default:
			hasNative = true
		}
	}

	if hasNative && hasWeb {
		return ""
	}
	if hasNative {
		return "native"
	}
	return "web"
}

// Authorize performs the authorization flow.
// It is designed to perform the whole Authorization Code Grant flow.
// On success, [AuthorizationCodeHandler.TokenSource] will return a token source with the fetched token.
func (h *AuthorizationCodeHandler) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)

	wwwChallenges, err := oauthex.ParseWWWAuthenticate(resp.Header[http.CanonicalHeaderKey("WWW-Authenticate")])
	if err != nil {
		return fmt.Errorf("failed to parse WWW-Authenticate header: %v", err)
	}

	if resp.StatusCode == http.StatusForbidden && errorFromChallenges(wwwChallenges) != "insufficient_scope" {
		// We only want to perform step-up authorization for insufficient_scope errors.
		// Returning nil, so that the call is retried immediately and the response
		// is handled appropriately by the connection.
		// Step-up authorization is defined at
		// https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization#step-up-authorization-flow
		return nil
	}

	prm, err := h.getProtectedResourceMetadata(ctx, wwwChallenges, req.URL.String())
	if err != nil {
		return err
	}

	asm, err := GetAuthServerMetadata(ctx, prm.AuthorizationServers[0], h.config.Client)
	if err != nil {
		return fmt.Errorf("failed to get authorization server metadata: %w", err)
	}
	if asm == nil {
		// Fallback to 2025-03-26 spec: predefined endpoints.
		// https://modelcontextprotocol.io/specification/2025-03-26/basic/authorization#fallbacks-for-servers-without-metadata-discovery
		authServerURL := prm.AuthorizationServers[0]
		asm = &oauthex.AuthServerMeta{
			Issuer:                authServerURL,
			AuthorizationEndpoint: authServerURL + "/authorize",
			TokenEndpoint:         authServerURL + "/token",
			RegistrationEndpoint:  authServerURL + "/register",
		}
	}

	resolvedClientConfig, err := h.handleRegistration(ctx, asm)
	if err != nil {
		return err
	}

	requestedScopes := scopesFromChallenges(wwwChallenges)
	if len(requestedScopes) == 0 && len(prm.ScopesSupported) > 0 {
		requestedScopes = prm.ScopesSupported
	}

	// SEP-2207: when the client desires refresh tokens and the Authorization
	// Server advertises offline_access support, add it to the requested scopes.
	if h.config.RequestRefreshToken &&
		slices.Contains(asm.ScopesSupported, "offline_access") &&
		!slices.Contains(requestedScopes, "offline_access") {
		requestedScopes = append(requestedScopes, "offline_access")
	}

	// Accumulate scopes: union previously granted scopes with the newly
	// challenged scopes so that step-up authorization does not lose
	// permissions granted in earlier rounds (SEP-2350).
	h.mu.RLock()
	granted := h.grantedScopes[asm.Issuer]
	h.mu.RUnlock()
	requestedScopes = authutil.UnionScopes(granted, requestedScopes)

	cfg := &oauth2.Config{
		ClientID:     resolvedClientConfig.clientID,
		ClientSecret: resolvedClientConfig.clientSecret,

		Endpoint: oauth2.Endpoint{
			AuthURL:   asm.AuthorizationEndpoint,
			TokenURL:  asm.TokenEndpoint,
			AuthStyle: resolvedClientConfig.authStyle,
		},
		RedirectURL: h.config.RedirectURL,
		Scopes:      requestedScopes,
	}

	authRes, err := h.getAuthorizationCode(ctx, cfg, prm.Resource)
	if err != nil {
		// Purposefully leaving the error unwrappable so it can be handled by the caller.
		return err
	}
	if err := validateIssuerResponse(authRes.Iss, asm.Issuer, asm.AuthorizationResponseIssParameterSupported); err != nil {
		return err
	}

	err = h.exchangeAuthorizationCode(ctx, cfg, authRes, prm.Resource)
	if err != nil {
		return err
	}

	return h.updateGrantedScopes(asm.Issuer, requestedScopes)
}

// resourceMetadataURLFromChallenges returns a resource metadata URL from the given "WWW-Authenticate" header challenges,
// or the empty string if there is none.
func resourceMetadataURLFromChallenges(cs []oauthex.Challenge) string {
	for _, c := range cs {
		if u := c.Params["resource_metadata"]; u != "" {
			return u
		}
	}
	return ""
}

// scopesFromChallenges returns the scopes from the given "WWW-Authenticate" header challenges.
// It only looks at challenges with the "Bearer" scheme.
func scopesFromChallenges(cs []oauthex.Challenge) []string {
	for _, c := range cs {
		if c.Scheme == "bearer" && c.Params["scope"] != "" {
			return strings.Fields(c.Params["scope"])
		}
	}
	return nil
}

// errorFromChallenges returns the error from the given "WWW-Authenticate" header challenges.
// It only looks at challenges with the "Bearer" scheme.
func errorFromChallenges(cs []oauthex.Challenge) string {
	for _, c := range cs {
		if c.Scheme == "bearer" && c.Params["error"] != "" {
			return c.Params["error"]
		}
	}
	return ""
}

// getProtectedResourceMetadata returns the protected resource metadata.
// If no metadata was found or the fetched metadata fails security checks,
// it returns an error.
func (h *AuthorizationCodeHandler) getProtectedResourceMetadata(ctx context.Context, wwwChallenges []oauthex.Challenge, mcpServerURL string) (*oauthex.ProtectedResourceMetadata, error) {
	// Use MCP server URL as the resource URI per
	// https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization#canonical-server-uri.
	for _, url := range protectedResourceMetadataURLs(resourceMetadataURLFromChallenges(wwwChallenges), mcpServerURL) {
		prm, err := oauthex.GetProtectedResourceMetadata(ctx, url.URL, url.Resource, h.config.Client)
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
	// Fallback to 2025-03-26 spec MCP server root is the Authorization Server:
	// https://modelcontextprotocol.io/specification/2025-03-26/basic/authorization#server-metadata-discovery
	u, err := url.Parse(mcpServerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MCP server URL: %v", err)
	}
	u.Path = ""
	prm := &oauthex.ProtectedResourceMetadata{
		AuthorizationServers: []string{u.String()},
		Resource:             mcpServerURL,
	}
	return prm, nil
}

type prmURL struct {
	// URL represents a URL where Protected Resource Metadata may be retrieved.
	URL string
	// Resource represents the corresponding resource URL for [URL].
	// It is required to perform validation described in RFC 9728, section 3.3.
	Resource string
}

// protectedResourceMetadataURLs returns a list of URLs to try when looking for
// protected resource metadata as mandated by the MCP specification:
// https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization#protected-resource-metadata-discovery-requirements
func protectedResourceMetadataURLs(metadataURL, resourceURL string) []prmURL {
	var urls []prmURL
	if metadataURL != "" {
		urls = append(urls, prmURL{
			URL:      metadataURL,
			Resource: resourceURL,
		})
	}
	ru, err := url.Parse(resourceURL)
	if err != nil {
		return urls
	}
	mu := *ru
	// "At the path of the server's MCP endpoint".
	mu.Path = "/.well-known/oauth-protected-resource/" + strings.TrimLeft(ru.Path, "/")
	urls = append(urls, prmURL{
		URL:      mu.String(),
		Resource: resourceURL,
	})
	// "At the root".
	mu.Path = "/.well-known/oauth-protected-resource"
	ru.Path = ""
	urls = append(urls, prmURL{
		URL:      mu.String(),
		Resource: ru.String(),
	})
	return urls
}

type registrationType int

const (
	registrationTypeClientIDMetadataDocument registrationType = iota
	registrationTypePreregistered
	registrationTypeDynamic
)

type resolvedClientConfig struct {
	registrationType registrationType
	clientID         string
	clientSecret     string
	authStyle        oauth2.AuthStyle
}

func selectTokenAuthMethod(supported []string) oauth2.AuthStyle {
	prefOrder := []string{
		// Preferred in OAuth 2.1 draft: https://www.ietf.org/archive/id/draft-ietf-oauth-v2-1-14.html#name-client-secret.
		"client_secret_post",
		"client_secret_basic",
	}
	for _, method := range prefOrder {
		if slices.Contains(supported, method) {
			return authMethodToStyle(method)
		}
	}
	return oauth2.AuthStyleAutoDetect
}

func authMethodToStyle(method string) oauth2.AuthStyle {
	switch method {
	case "client_secret_post":
		return oauth2.AuthStyleInParams
	case "client_secret_basic":
		return oauth2.AuthStyleInHeader
	case "none":
		// "none" is equivalent to "client_secret_post" but without sending client secret.
		return oauth2.AuthStyleInParams
	default:
		// "client_secret_basic" is the default per https://datatracker.ietf.org/doc/html/rfc7591#section-2.
		return oauth2.AuthStyleInHeader
	}
}

// handleRegistration handles client registration.
// The provided authorization server metadata must be non-nil.
// Support for different registration methods is defined as follows:
//   - Client ID Metadata Document: metadata must have
//     `ClientIDMetadataDocumentSupported` set to true.
//   - Pre-registered client: assumed to be supported.
//   - Dynamic client registration: metadata must have
//     `RegistrationEndpoint` set to a non-empty value.
func (h *AuthorizationCodeHandler) handleRegistration(ctx context.Context, asm *oauthex.AuthServerMeta) (*resolvedClientConfig, error) {
	// 1. Attempt to use Client ID Metadata Document (SEP-991).
	cimdCfg := h.config.ClientIDMetadataDocumentConfig
	if cimdCfg != nil && asm.ClientIDMetadataDocumentSupported {
		return &resolvedClientConfig{
			registrationType: registrationTypeClientIDMetadataDocument,
			clientID:         cimdCfg.URL,
		}, nil
	}
	// 2. Attempt to use pre-registered client configuration.
	preCfg := h.config.PreregisteredClient
	if preCfg != nil {
		if preCfg.Issuer != "" && !authutil.IssuersEqual(preCfg.Issuer, asm.Issuer) {
			return nil, fmt.Errorf("authorization server issuer %q does not match pre-registered credentials issuer %q", asm.Issuer, preCfg.Issuer)
		}
		authStyle := selectTokenAuthMethod(asm.TokenEndpointAuthMethodsSupported)
		clientSecret := ""
		if preCfg.ClientSecretAuth != nil {
			clientSecret = preCfg.ClientSecretAuth.ClientSecret
		}
		return &resolvedClientConfig{
			registrationType: registrationTypePreregistered,
			clientID:         preCfg.ClientID,
			clientSecret:     clientSecret,
			authStyle:        authStyle,
		}, nil
	}
	// 3. Attempt to use dynamic client registration.
	dcrCfg := h.config.DynamicClientRegistrationConfig
	if dcrCfg != nil && asm.RegistrationEndpoint != "" {
		regResp, err := oauthex.RegisterClient(ctx, asm.RegistrationEndpoint, dcrCfg.Metadata, h.config.Client)
		if err != nil {
			return nil, fmt.Errorf("failed to register client: %w", err)
		}
		cfg := &resolvedClientConfig{
			registrationType: registrationTypeDynamic,
			clientID:         regResp.ClientID,
			clientSecret:     regResp.ClientSecret,
			authStyle:        authMethodToStyle(regResp.TokenEndpointAuthMethod),
		}
		return cfg, nil
	}
	return nil, fmt.Errorf("no configured client registration methods are supported by the authorization server")
}

type authResult struct {
	*AuthorizationResult
	// usedCodeVerifier is the PKCE code verifier used to obtain the authorization code.
	// It is preserved for the token exchange step.
	usedCodeVerifier string
}

// getAuthorizationCode uses the [AuthorizationCodeHandler.AuthorizationCodeFetcher]
// to obtain an authorization code.
func (h *AuthorizationCodeHandler) getAuthorizationCode(ctx context.Context, cfg *oauth2.Config, resourceURL string) (*authResult, error) {
	codeVerifier := oauth2.GenerateVerifier()
	state := rand.Text()

	authURL := cfg.AuthCodeURL(state,
		oauth2.S256ChallengeOption(codeVerifier),
		oauth2.SetAuthURLParam("resource", resourceURL),
	)

	authRes, err := h.config.AuthorizationCodeFetcher(ctx, &AuthorizationArgs{URL: authURL})
	if err != nil {
		// Purposefully leaving the error unwrappable so it can be handled by the caller.
		return nil, err
	}
	if authRes.State != state {
		return nil, fmt.Errorf("state mismatch")
	}
	return &authResult{
		AuthorizationResult: authRes,
		usedCodeVerifier:    codeVerifier,
	}, nil
}

// validateIssuerResponse validates the "iss" parameter in an authorization response
// per [RFC 9207].
//
// [RFC 9207]: https://www.rfc-editor.org/rfc/rfc9207
func validateIssuerResponse(iss, expectedIssuer string, issParameterSupported bool) error {
	if issParameterSupported {
		if iss == "" {
			return fmt.Errorf("authorization server advertises RFC 9207 iss parameter support but none was received in the authorization response")
		}
		if iss != expectedIssuer {
			return fmt.Errorf("authorization response issuer %q does not match expected issuer %q", iss, expectedIssuer)
		}
	} else {
		if iss != "" {
			return fmt.Errorf("authorization server does not advertise RFC 9207 iss parameter support but iss was received in the authorization response")
		}
	}

	return nil
}

// exchangeAuthorizationCode exchanges the authorization code for a token
// and stores it in a token source.
func (h *AuthorizationCodeHandler) exchangeAuthorizationCode(ctx context.Context, cfg *oauth2.Config, authResult *authResult, resourceURL string) error {
	opts := []oauth2.AuthCodeOption{
		oauth2.VerifierOption(authResult.usedCodeVerifier),
		oauth2.SetAuthURLParam("resource", resourceURL),
	}
	clientCtx := context.WithValue(ctx, oauth2.HTTPClient, h.config.Client)
	token, err := cfg.Exchange(clientCtx, authResult.Code, opts...)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}
	// The token source outlives this authorization request: it is stored on the
	// handler and used by the transport for the lifetime of the connection. The
	// oauth2 library captures the context passed to TokenSource and reuses it for
	// every subsequent token refresh (see golang.org/x/oauth2: tokenRefresher
	// retains the context and passes it to each refresh round-trip). Binding it to
	// the per-request ctx makes all later refreshes fail with "context canceled"
	// once that request (or the connect operation that triggered authorization)
	// completes. Use a background context that still carries the configured HTTP
	// client so refreshes keep working for the life of the token source.
	refreshCtx := context.WithValue(context.Background(), oauth2.HTTPClient, h.config.Client)
	var ts oauth2.TokenSource
	if h.config.NewTokenSource == nil {
		ts = cfg.TokenSource(refreshCtx, token)
	} else {
		var err error
		ts, err = h.config.NewTokenSource(refreshCtx, cfg, token)
		if err != nil {
			return fmt.Errorf("constructing token source failed: %w", err)
		}
	}
	h.mu.Lock()
	h.tokenSource = ts
	h.mu.Unlock()
	return nil
}

// updateGrantedScopes updates the granted scopes based on the token source and requested scopes.
func (h *AuthorizationCodeHandler) updateGrantedScopes(issuer string, requestedScopes []string) error {
	h.mu.RLock()
	ts := h.tokenSource
	h.mu.RUnlock()

	if ts == nil {
		return nil
	}
	tok, err := ts.Token()
	if err != nil {
		return err
	}

	h.mu.Lock()
	if tokenScopes := authutil.ScopesFromToken(tok); tokenScopes == nil {
		h.grantedScopes[issuer] = requestedScopes
	} else {
		h.grantedScopes[issuer] = tokenScopes
	}
	h.mu.Unlock()
	return nil
}
