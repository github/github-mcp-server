// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package oauthex

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenExpiry = time.Hour
)

var jwtSigningKey = []byte("fake-secret-key")

type authCodeInfo struct {
	codeChallenge string
	redirectURI   string
}

type state struct {
	mu        sync.Mutex
	authCodes map[string]authCodeInfo
}

// NewFakeMCPServerMux constructs a ServeMux that implements an MCP server
// with an integrated OAuth 2.1 authentication server. It should be used with
// [httptest.NewTLSServer].
func NewFakeMCPServerMux() *http.ServeMux {
	s := &state{authCodes: make(map[string]authCodeInfo)}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCP)
	mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleProtectedResourceMetadata)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleServerMetadata)
	mux.HandleFunc("/register", s.handleDynamicClientRegistration)
	mux.HandleFunc("/authorize", s.handleAuthorize)
	mux.HandleFunc("/token", s.handleToken)
	return mux
}

// handleMCP is the protected resource endpoint. It requires a valid Bearer token.
// If the token is missing or invalid, it returns a 401 Unauthorized response
// with a WWW-Authenticate header pointing to the resource metadata.
func (s *state) handleMCP(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSigningKey, nil
	})

	if err != nil || !token.Valid {
		metadataURL := getBaseURL(r) + "/.well-known/oauth-protected-resource"
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata="%s", scope="openid profile email"`, metadataURL))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Token is valid, serve the resource.
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Hello from a protected MCP server!")
}

// handleProtectedResourceMetadata serves the OAuth 2.0 Protected Resource Metadata document,
// as defined in RFC 9728.
func (s *state) handleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	// Construct the authorization server issuer URL dynamically.
	authServerIssuer := getBaseURL(r)

	metadata := ProtectedResourceMetadata{
		ScopesSupported:      []string{"openid", "profile", "email"},
		AuthorizationServers: []string{authServerIssuer},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(metadata); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func (s *state) handleServerMetadata(w http.ResponseWriter, r *http.Request) {
	issuer := getBaseURL(r)
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
		CodeChallengeMethodsSupported:     []string{"S256"},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(metadata); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(buf.Bytes())
}

// handleDynamicClientRegistration handles dynamic client registration requests,
// as defined in RFC 7591.
func (s *state) handleDynamicClientRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var crm ClientRegistrationMetadata
	if err := json.NewDecoder(r.Body).Decode(&crm); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON in request body: %v", err), http.StatusBadRequest)
		return
	}
	clientID := fmt.Sprintf("fake-client-%d", time.Now().UnixNano())

	response := ClientRegistrationResponse{
		ClientRegistrationMetadata: crm,
		ClientID:                   clientID,
		ClientIDIssuedAt:           time.Now(),
		ClientSecret:               "fake-registration-access-secret",
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(buf.Bytes())
}

func (s *state) handleToken(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	grantType := r.Form.Get("grant_type")
	code := r.Form.Get("code")
	codeVerifier := r.Form.Get("code_verifier")
	// Ignore redirect_uri; it is not required in 2.1.
	// https://www.ietf.org/archive/id/draft-ietf-oauth-v2-1-13.html#redirect-uri-in-token-request

	if grantType != "authorization_code" {
		http.Error(w, "unsupported_grant_type", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	authCodeInfo, ok := s.authCodes[code]
	if !ok {
		http.Error(w, "invalid_grant", http.StatusBadRequest)
		return
	}
	delete(s.authCodes, code)
	s.mu.Unlock()

	// PKCE verification.
	hasher := sha256.New()
	hasher.Write([]byte(codeVerifier))
	calculatedChallenge := base64.RawURLEncoding.EncodeToString(hasher.Sum(nil))
	if calculatedChallenge != authCodeInfo.codeChallenge {
		http.Error(w, "invalid_grant", http.StatusBadRequest)
		return
	}

	// Issue JWT.
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": getBaseURL(r),
		"sub": "fake-user-id",
		"aud": "fake-client-id",
		"exp": now.Add(tokenExpiry).Unix(),
		"iat": now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(jwtSigningKey)
	if err != nil {
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	tokenResponse := map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(tokenExpiry.Seconds()),
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(tokenResponse); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(buf.Bytes())
}

func (s *state) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	responseType := query.Get("response_type")
	redirectURI := query.Get("redirect_uri")
	codeChallenge := query.Get("code_challenge")
	codeChallengeMethod := query.Get("code_challenge_method")

	if responseType != "code" {
		http.Error(w, "unsupported_response_type", http.StatusBadRequest)
		return
	}
	if redirectURI == "" {
		http.Error(w, "invalid_request (no redirect_uri)", http.StatusBadRequest)
		return
	}
	if codeChallenge == "" || codeChallengeMethod != "S256" {
		http.Error(w, "invalid_request (code challenge is not S256)", http.StatusBadRequest)
		return
	}
	if query.Get("client_id") == "" {
		http.Error(w, "invalid_request (missing client_id)", http.StatusBadRequest)
		return
	}

	authCode := "fake-auth-code-" + fmt.Sprintf("%d", time.Now().UnixNano())
	s.mu.Lock()
	s.authCodes[authCode] = authCodeInfo{
		codeChallenge: codeChallenge,
		redirectURI:   redirectURI,
	}
	s.mu.Unlock()

	redirectURL := fmt.Sprintf("%s?code=%s&state=%s", redirectURI, authCode, query.Get("state"))
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// getBaseURL constructs the base URL (scheme://host) from the request.
func getBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
