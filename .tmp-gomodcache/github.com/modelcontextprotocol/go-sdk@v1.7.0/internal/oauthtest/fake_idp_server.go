// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package oauthtest

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// TokenExchangeConfig configures RFC 8693 token exchange support on a [FakeIdPServer].
type TokenExchangeConfig struct {
	// IDJAGToken is the ID-JAG value returned from token exchange.
	// Defaults to "test-id-jag-token" if empty.
	IDJAGToken string
}

// IdPConfig holds configuration for [FakeIdPServer].
type IdPConfig struct {
	// PreregisteredClients maps client IDs to their info.
	PreregisteredClients map[string]ClientInfo

	// TokenExchangeConfig enables RFC 8693 token exchange at the /token endpoint.
	// If non-nil, the server accepts grant_type=urn:ietf:params:oauth:grant-type:token-exchange.
	TokenExchangeConfig *TokenExchangeConfig
}

// FakeIdPServer is a fake OIDC Identity Provider for testing.
// It supports:
//   - OIDC discovery (/.well-known/openid-configuration)
//   - Authorization Code Grant with PKCE
//   - ID Token issuance (fake JWTs)
//   - RFC 8693 Token Exchange (ID Token → ID-JAG), if configured
type FakeIdPServer struct {
	server  *httptest.Server
	mux     *http.ServeMux
	config  IdPConfig
	clients map[string]ClientInfo
	codes   map[string]codeInfo
}

// NewFakeIdPServer creates a new FakeIdPServer.
func NewFakeIdPServer(config IdPConfig) *FakeIdPServer {
	s := &FakeIdPServer{
		mux:     http.NewServeMux(),
		config:  config,
		clients: make(map[string]ClientInfo),
		codes:   make(map[string]codeInfo),
	}
	maps.Copy(s.clients, config.PreregisteredClients)

	s.mux.HandleFunc("/.well-known/openid-configuration", s.handleMetadata)
	s.mux.HandleFunc("/authorize", s.handleAuthorize)
	s.mux.HandleFunc("/token", s.handleToken)
	s.server = httptest.NewUnstartedServer(s.mux)

	return s
}

// Start starts the HTTP server and registers a cleanup function on t.
func (s *FakeIdPServer) Start(t testing.TB) {
	s.server.Start()
	t.Cleanup(s.server.Close)
}

// URL returns the base URL of the server (issuer).
func (s *FakeIdPServer) URL() string {
	return s.server.URL
}

func (s *FakeIdPServer) handleMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	grantTypes := []string{"authorization_code"}
	if s.config.TokenExchangeConfig != nil {
		grantTypes = append(grantTypes, oauthex.GrantTypeTokenExchange)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"issuer":                           s.URL(),
		"authorization_endpoint":           s.URL() + "/authorize",
		"token_endpoint":                   s.URL() + "/token",
		"jwks_uri":                         s.URL() + "/.well-known/jwks.json",
		"response_types_supported":         []string{"code"},
		"code_challenge_methods_supported": []string{"S256"},
		"grant_types_supported":            grantTypes,
	})
}

func (s *FakeIdPServer) handleAuthorize(w http.ResponseWriter, r *http.Request) {
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
	s.codes[code] = codeInfo{CodeChallenge: codeChallenge}
	state := r.URL.Query().Get("state")
	redirectURL := fmt.Sprintf("%s?code=%s&state=%s", redirectURI, code, state)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *FakeIdPServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
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
	case oauthex.GrantTypeTokenExchange:
		s.handleTokenExchangeGrant(w, r)
	default:
		http.Error(w, fmt.Sprintf("unsupported grant_type: %s", grantType), http.StatusBadRequest)
	}
}

func (s *FakeIdPServer) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.Form.Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	ci, ok := s.codes[code]
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
	if expectedChallenge != ci.CodeChallenge {
		http.Error(w, "PKCE verification failed", http.StatusBadRequest)
		return
	}

	clientID := r.Form.Get("client_id")
	now := time.Now().Unix()
	idToken := fakeIDToken(s.URL(), clientID, now)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":  "test_access_token",
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": "test_refresh_token",
		"id_token":      idToken,
	})
}

func (s *FakeIdPServer) handleTokenExchangeGrant(w http.ResponseWriter, r *http.Request) {
	if s.config.TokenExchangeConfig == nil {
		http.Error(w, "token exchange not supported", http.StatusBadRequest)
		return
	}
	if r.Form.Get("requested_token_type") != oauthex.TokenTypeIDJAG {
		http.Error(w, "invalid requested_token_type", http.StatusBadRequest)
		return
	}
	if r.Form.Get("subject_token_type") != oauthex.TokenTypeIDToken {
		http.Error(w, "invalid subject_token_type", http.StatusBadRequest)
		return
	}
	if r.Form.Get("subject_token") == "" {
		http.Error(w, "missing subject_token", http.StatusBadRequest)
		return
	}

	idJAG := s.config.TokenExchangeConfig.IDJAGToken
	if idJAG == "" {
		idJAG = "test-id-jag-token"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":      idJAG,
		"issued_token_type": oauthex.TokenTypeIDJAG,
		"token_type":        "N_A",
		"expires_in":        300,
	})
}

func (s *FakeIdPServer) authenticateClient(r *http.Request) error {
	clientID := r.Form.Get("client_id")
	clientSecret := r.Form.Get("client_secret")
	clientInfo, ok := s.clients[clientID]
	if !ok {
		return fmt.Errorf("unknown client")
	}
	if clientInfo.Secret != clientSecret {
		return fmt.Errorf("invalid client credentials")
	}
	return nil
}

// fakeIDToken creates a fake JWT ID token for testing.
// The token has a valid structure but is not cryptographically signed.
func fakeIDToken(issuer, audience string, now int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claimsJSON, _ := json.Marshal(map[string]any{
		"iss":   issuer,
		"sub":   "test-user",
		"aud":   audience,
		"exp":   now + 3600,
		"iat":   now,
		"email": "test@example.com",
	})
	claims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	return header + "." + claims + ".mock-signature"
}
