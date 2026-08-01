// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package oauthtest

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"

	internaljson "github.com/modelcontextprotocol/go-sdk/internal/json"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

type ClientInfo struct {
	Secret       string
	RedirectURIs []string
}

type MetadataEndpointConfig struct {
	// Whether to serve the OAuth Authorization Server Metadata at
	// /.well-known/oauth-authorization-server + issuerPath.
	ServeOAuthInsertedEndpoint bool
	// Whether to serve the OAuth Authorization Server Metadata at
	// /.well-known/openid-configuration + issuerPath.
	ServeOpenIDInsertedEndpoint bool
	// Whether to serve the OAuth Authorization Server Metadata at
	// issuerPath + /.well-known/openid-configuration.
	// Should be used when issuerPath is not empty.
	ServeOpenIDAppendedEndpoint bool
}

type RegistrationConfig struct {
	// Whether the client ID metadata document is supported.
	ClientIDMetadataDocumentSupported bool
	// PreregisteredClients is a map of valid ClientIDs to ClientSecrets.
	PreregisteredClients map[string]ClientInfo
	// Whether dynamic client registration is enabled.
	DynamicClientRegistrationEnabled bool
}

// JWTBearerConfig configures support for the JWT Bearer grant type (RFC 7523)
// on a [FakeAuthorizationServer].
type JWTBearerConfig struct {
	// ValidAssertions is the set of assertion values that are accepted.
	// If empty, any non-empty assertion is accepted.
	ValidAssertions []string
}

// ClientCredentialsConfig configures support for the client_credentials
// grant type (RFC 6749 Section 4.4) on a [FakeAuthorizationServer].
type ClientCredentialsConfig struct {
	// Enabled controls whether the /token endpoint accepts
	// grant_type=client_credentials and returns an access token
	// if client authentication succeeds.
	Enabled bool
}

// Config holds configuration for FakeAuthorizationServer.
type Config struct {
	// The optional path component of the issuer URL.
	// If non-empty, it should start with a "/". It should not end with a "/".
	// It affects the paths of the server endpoints.
	IssuerPath string
	// Configuration of the metadata endpoint.
	MetadataEndpointConfig *MetadataEndpointConfig
	// Configuration for client registration.
	RegistrationConfig *RegistrationConfig
	// JWTBearerConfig enables RFC 7523 JWT Bearer grant at the /token endpoint.
	// If non-nil, the server accepts grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer.
	JWTBearerConfig *JWTBearerConfig
	// ClientCredentialsConfig enables RFC 6749 Section 4.4 client credentials
	// grant at the /token endpoint.
	ClientCredentialsConfig *ClientCredentialsConfig
	// ScopesSupported is an optional list of scopes to advertise in the
	// authorization server metadata.
	ScopesSupported []string
	// TokenScopeFunc, if set, is called with the scope from the authorization
	// request and returns the scope string to include in the token response.
	TokenScopeFunc func(requestedScope string) string
	// AccessTokenTTL, if non-zero, is the expires_in (in seconds) returned by the
	// /token endpoint for both the authorization_code and refresh_token grants.
	// When zero a default of 3600 is used. Set it small to force a client's reuse
	// token source to treat the access token as expired and refresh it.
	AccessTokenTTL int
	// IssueRefreshToken, if true, includes a refresh_token in token responses and
	// enables grant_type=refresh_token at the /token endpoint.
	IssueRefreshToken bool
}

// testRefreshToken is the refresh token issued and accepted by the fake server
// when Config.IssueRefreshToken is set.
const testRefreshToken = "test_refresh_token"

// accessTokenExpiresIn returns the expires_in value to use in token responses.
func (s *FakeAuthorizationServer) accessTokenExpiresIn() int {
	if s.config.AccessTokenTTL != 0 {
		return s.config.AccessTokenTTL
	}
	return 3600
}

// FakeAuthorizationServer is a fake OAuth 2.0 Authorization Server for testing.
type FakeAuthorizationServer struct {
	server  *httptest.Server
	Mux     *http.ServeMux
	config  Config
	clients map[string]ClientInfo
	codes   map[string]codeInfo
}

type codeInfo struct {
	CodeChallenge string
	Scope         string
}

// NewFakeAuthorizationServer creates a new FakeAuthorizationServer.
// The server is simple and should not be used outside of testing.
// It supports:
// - Only the authorization Code Grant
// - PKCE verification
// - Client tracking & dynamic registration
// - Client authentication
func NewFakeAuthorizationServer(config Config) *FakeAuthorizationServer {
	s := &FakeAuthorizationServer{
		Mux:    http.NewServeMux(),
		config: config,
		codes:  make(map[string]codeInfo),
	}
	if config.RegistrationConfig != nil {
		s.clients = maps.Clone(config.RegistrationConfig.PreregisteredClients)
	}
	if s.clients == nil {
		s.clients = make(map[string]ClientInfo)
	}

	s.Mux.HandleFunc(s.config.IssuerPath+"/authorize", s.handleAuthorize)
	s.Mux.HandleFunc(s.config.IssuerPath+"/token", s.handleToken)
	if config.MetadataEndpointConfig != nil {
		if config.MetadataEndpointConfig.ServeOAuthInsertedEndpoint {
			s.Mux.HandleFunc("/.well-known/oauth-authorization-server"+s.config.IssuerPath, s.handleMetadata)
		}
		if config.MetadataEndpointConfig.ServeOpenIDInsertedEndpoint {
			s.Mux.HandleFunc("/.well-known/openid-configuration"+s.config.IssuerPath, s.handleMetadata)
		}
		if config.MetadataEndpointConfig.ServeOpenIDAppendedEndpoint && s.config.IssuerPath != "" {
			s.Mux.HandleFunc(s.config.IssuerPath+"/.well-known/openid-configuration", s.handleMetadata)
		}
	} else {
		// Serve the default OAuth endpoint.
		s.Mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleMetadata)
	}
	if config.RegistrationConfig != nil && config.RegistrationConfig.DynamicClientRegistrationEnabled {
		s.Mux.HandleFunc(s.config.IssuerPath+"/register", s.handleRegister)
	}
	s.server = httptest.NewUnstartedServer(s.Mux)

	return s
}

// Start starts the HTTP server and registers a cleanup function on t to close the server.
func (s *FakeAuthorizationServer) Start(t testing.TB) {
	s.server.Start()
	t.Cleanup(s.server.Close)
}

// URL returns the base URL of the server (Issuer).
func (s *FakeAuthorizationServer) URL() string {
	return s.server.URL
}

func (s *FakeAuthorizationServer) handleMetadata(w http.ResponseWriter, r *http.Request) {
	cimdSupported := false
	var registrationEndpoint string
	if s.config.RegistrationConfig != nil {
		cimdSupported = s.config.RegistrationConfig.ClientIDMetadataDocumentSupported
		if s.config.RegistrationConfig.DynamicClientRegistrationEnabled {
			registrationEndpoint = s.URL() + s.config.IssuerPath + "/register"
		}
	}
	meta := &oauthex.AuthServerMeta{
		Issuer:                            s.URL() + s.config.IssuerPath,
		AuthorizationEndpoint:             s.URL() + s.config.IssuerPath + "/authorize",
		TokenEndpoint:                     s.URL() + s.config.IssuerPath + "/token",
		RegistrationEndpoint:              registrationEndpoint,
		ScopesSupported:                   s.config.ScopesSupported,
		ResponseTypesSupported:            []string{"code"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		ClientIDMetadataDocumentSupported: cimdSupported,
		TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic"},
		// Advertise RFC 9207 support: the authorize endpoint includes "iss" in responses.
		AuthorizationResponseIssParameterSupported: true,
	}
	// Set CORS headers for cross-origin client discovery.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle CORS preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Only GET allowed for metadata retrieval
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(meta); err != nil {
		http.Error(w, "Failed to encode metadata", http.StatusInternalServerError)
		return
	}
}

func (s *FakeAuthorizationServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var metadata oauthex.ClientRegistrationMetadata
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if err := internaljson.Unmarshal(body, &metadata); err != nil {
		http.Error(w, "failed to parse request", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	clientID := rand.Text()
	ci := ClientInfo{
		Secret:       rand.Text(),
		RedirectURIs: metadata.RedirectURIs,
	}
	s.clients[clientID] = ci
	metadata.TokenEndpointAuthMethod = "client_secret_basic"
	json.NewEncoder(w).Encode(&oauthex.ClientRegistrationResponse{
		ClientID:                   clientID,
		ClientSecret:               ci.Secret,
		ClientRegistrationMetadata: metadata,
	})
}

func (s *FakeAuthorizationServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	clientID := r.URL.Query().Get("client_id")
	clientInfo, ok := s.clients[clientID]
	if !ok {
		http.Error(w, "unknown client_id", http.StatusBadRequest)
		return
	}

	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		http.Error(w, "missing redirect_uri", http.StatusBadRequest)
		return
	}
	if !slices.Contains(clientInfo.RedirectURIs, redirectURI) {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	codeChallenge := r.URL.Query().Get("code_challenge")
	if codeChallenge == "" {
		http.Error(w, "missing code_challenge", http.StatusBadRequest)
		return
	}
	code := rand.Text()
	s.codes[code] = codeInfo{
		CodeChallenge: codeChallenge,
		Scope:         r.URL.Query().Get("scope"),
	}

	state := r.URL.Query().Get("state")
	issuer := s.URL() + s.config.IssuerPath

	redirectURL := fmt.Sprintf("%s?code=%s&state=%s&iss=%s", redirectURI, code, state, url.QueryEscape(issuer))
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *FakeAuthorizationServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.authenticateClient(r); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	grantType := r.Form.Get("grant_type")
	switch grantType {
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, r)
	case "urn:ietf:params:oauth:grant-type:jwt-bearer":
		s.handleJWTBearerGrant(w, r)
	case "client_credentials":
		s.handleClientCredentialsGrant(w, r)
	case "refresh_token":
		s.handleRefreshTokenGrant(w, r)
	default:
		http.Error(w, fmt.Sprintf("unsupported grant_type: %s", grantType), http.StatusBadRequest)
	}
}

func (s *FakeAuthorizationServer) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.Form.Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	codeInfo, ok := s.codes[code]
	if !ok {
		http.Error(w, "unknown authorization code", http.StatusBadRequest)
		return
	}
	verifier := r.Form.Get("code_verifier")
	if verifier == "" {
		http.Error(w, "missing code_verifier", http.StatusBadRequest)
		return
	}
	sha := sha256.Sum256([]byte(verifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(sha[:])
	if expectedChallenge != codeInfo.CodeChallenge {
		http.Error(w, "PKCE verification failed", http.StatusBadRequest)
		return
	}

	resp := map[string]any{
		"access_token": "test_access_token",
		"token_type":   "Bearer",
		"expires_in":   s.accessTokenExpiresIn(),
	}
	if s.config.IssueRefreshToken {
		resp["refresh_token"] = testRefreshToken
	}
	if s.config.TokenScopeFunc != nil {
		if scope := s.config.TokenScopeFunc(codeInfo.Scope); scope != "" {
			resp["scope"] = scope
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleRefreshTokenGrant implements grant_type=refresh_token (RFC 6749 Section
// 6) when Config.IssueRefreshToken is set, returning a distinct access token so
// callers can observe that a refresh occurred.
func (s *FakeAuthorizationServer) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) {
	if !s.config.IssueRefreshToken {
		http.Error(w, "refresh_token grant not supported", http.StatusBadRequest)
		return
	}
	if r.Form.Get("refresh_token") != testRefreshToken {
		http.Error(w, "invalid refresh_token", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":  "test_access_token_refreshed",
		"token_type":    "Bearer",
		"expires_in":    s.accessTokenExpiresIn(),
		"refresh_token": testRefreshToken,
	})
}

func (s *FakeAuthorizationServer) handleJWTBearerGrant(w http.ResponseWriter, r *http.Request) {
	if s.config.JWTBearerConfig == nil {
		http.Error(w, "JWT bearer grant not supported", http.StatusBadRequest)
		return
	}
	assertion := r.Form.Get("assertion")
	if assertion == "" {
		http.Error(w, "missing assertion", http.StatusBadRequest)
		return
	}
	if len(s.config.JWTBearerConfig.ValidAssertions) > 0 {
		if !slices.Contains(s.config.JWTBearerConfig.ValidAssertions, assertion) {
			http.Error(w, "invalid assertion", http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token": "test_access_token",
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
}

func (s *FakeAuthorizationServer) handleClientCredentialsGrant(w http.ResponseWriter, r *http.Request) {
	if s.config.ClientCredentialsConfig == nil || !s.config.ClientCredentialsConfig.Enabled {
		http.Error(w, "client_credentials grant not supported", http.StatusBadRequest)
		return
	}
	resp := map[string]any{
		"access_token": "test_access_token",
		"token_type":   "Bearer",
		"expires_in":   3600,
	}
	if s.config.TokenScopeFunc != nil {
		if scope := s.config.TokenScopeFunc(r.Form.Get("scope")); scope != "" {
			resp["scope"] = scope
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *FakeAuthorizationServer) authenticateClient(r *http.Request) error {
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID = r.Form.Get("client_id")
		clientSecret = r.Form.Get("client_secret")
	}

	clientInfo, ok := s.clients[clientID]
	if !ok || clientInfo.Secret != clientSecret {
		return errors.New("client not found")
	}
	return nil
}
