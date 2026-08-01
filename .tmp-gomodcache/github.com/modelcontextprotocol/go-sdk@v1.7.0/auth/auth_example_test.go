// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"
)

// savingTokenSource is an oauth2.TokenSource that passes the oauth2 config and
// token to the given saver function each time the access token value changes.
type savingTokenSource struct {
	mu          sync.Mutex
	src         oauth2.TokenSource
	saver       func(*oauth2.Config, *oauth2.Token) error
	config      *oauth2.Config
	accessToken string
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tok, err := s.src.Token()
	if err != nil {
		return nil, err
	}
	if s.accessToken != tok.AccessToken {
		s.accessToken = tok.AccessToken
		// This saver implementation always returns nil.
		_ = s.saver(s.config, tok)
	}
	return tok, nil
}

// NewSavingTokenSource persists OAuth 2.0 sessions by intercepting token
// changes from the wrapped oauth2.TokenSource. When this wrapper detects an
// access token change, it calls the provided session saver with the
// oauth2.Config and the new oauth2.Token.
//
// initial is an optional access token the caller may already hold (such as a
// token loaded from storage). It initialises the wrapper's state so that if
// the first wrapped.Token() call returns the same token, does not trigger a
// redundant call to saver(). Pass nil when there is no existing token, in
// which case the first token produced by wrapped.Token() is saved.
func NewSavingTokenSource(wrapped oauth2.TokenSource, config *oauth2.Config, initial *oauth2.Token, saver func(*oauth2.Config, *oauth2.Token) error) oauth2.TokenSource {
	if wrapped == nil {
		return nil
	}
	if saver == nil {
		return wrapped
	}
	var accessToken string
	if initial != nil {
		accessToken = initial.AccessToken
	}
	return &savingTokenSource{
		src:         wrapped,
		saver:       saver,
		config:      config,
		accessToken: accessToken,
	}
}

// sessionStore is an in-memory OAuth 2.0 session store used to demonstrate
// persistence. A production implementation would persist the session to disk or
// a secret store.
type sessionStore struct {
	config *oauth2.Config
	token  *oauth2.Token
}

// save persists the given OAuth 2.0 config and token.
func (s *sessionStore) save(config *oauth2.Config, token *oauth2.Token) error {
	fmt.Printf("Saving token: %s\n", token.AccessToken)
	s.config = config
	s.token = token
	return nil
}

// restore loads a previously persisted OAuth 2.0 session, if one exists.
func (s *sessionStore) restore() (*oauth2.Config, *oauth2.Token, error) {
	if s.config != nil && s.token != nil {
		fmt.Println("Restoring session.")
	} else {
		fmt.Println("No session found to restore.")
	}
	return s.config, s.token, nil
}

// newMockAuthServer returns an httptest.Server that simulates both an MCP
// resource server requiring authorization and its OAuth authorization server.
func newMockAuthServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"issuer": "http://%s", "authorization_endpoint": "http://%s/auth", "token_endpoint": "http://%s/token", "code_challenge_methods_supported": ["S256"]}`, r.Host, r.Host, r.Host)
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token": "mock-token", "token_type": "bearer"}`))
		default:
			// The mock MCP endpoint returns 401 until the client presents a valid
			// bearer token.
			if r.Header.Get("Authorization") == "Bearer mock-token" {
				// A real server would return a valid MCP message here. The empty
				// response causes Connect to return an error after authorization,
				// which is ignored for this example which demonstrates token
				// persistence only.
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
}

// This example shows how OAuth2 session persistence might be implemented. It
// connects twice using a shared in-memory session store: the first connection
// authorizes and saves the session, and the second restores and reuses it.
func Example_persistence() {
	// Simulate an MCP server that requires authorization and an OAuth server.
	mockServer := newMockAuthServer()
	defer mockServer.Close()

	// store persists the OAuth2 session across both connections.
	store := &sessionStore{}

	// connect performs a single client connection, restoring any saved session
	// beforehand and saving any new session acquired during authorization.
	connect := func() {
		// Load the OAuth2 session if available, and use it.
		var initialTS oauth2.TokenSource
		cfg, tok, err := store.restore()
		if err == nil && cfg != nil && tok != nil {
			initialTS = NewSavingTokenSource(
				cfg.TokenSource(context.Background(), tok),
				cfg, tok, store.save,
			)
		}

		// Configure and initialize the AuthorizationCodeHandler with a
		// NewTokenSource that saves the session when the access token changes,
		// and the InitialTokenSource set to the session loaded via restore().
		config := &auth.AuthorizationCodeHandlerConfig{
			RedirectURL: "http://localhost/callback",
			PreregisteredClient: &oauthex.ClientCredentials{
				ClientID: "example",
			},
			Client: mockServer.Client(),
			AuthorizationCodeFetcher: func(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
				fmt.Println("No token source found. Transport is calling Authorize()...")
				// Extract the generated state from the authorization URL
				u, _ := url.Parse(args.URL)
				state := u.Query().Get("state")
				return &auth.AuthorizationResult{Code: "mock-code", State: state}, nil
			},
			NewTokenSource: func(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (oauth2.TokenSource, error) {
				// This save implementation always returns nil.
				_ = store.save(cfg, token)
				return NewSavingTokenSource(
					cfg.TokenSource(ctx, token), cfg, token, store.save,
				), nil
			},
			InitialTokenSource: initialTS,
		}
		handler, err := auth.NewAuthorizationCodeHandler(config)
		if err != nil {
			fmt.Printf("Error creating handler: %v\n", err)
			return
		}

		// Set the constructed handler on a transport.
		transport := &mcp.StreamableClientTransport{
			Endpoint:     mockServer.URL + "/sse",
			OAuthHandler: handler,
		}

		// Create a client and attempt to connect using the configured
		// transport. The transport will automatically:
		// 1. Call TokenSource() to check for an existing session. This will
		//    return InitialTokenSource, if set.
		// 2. Try the MCP endpoint, and encounter a 401 response from the mock
		//    server.
		// 3. Call Authorize() to perform the OAuth flow, which calls
		//    NewTokenSource. In this example NewTokenSource saves the newly
		//    acquired session.
		// 4. Retry the MCP endpoint with a valid bearer token and get a 200.
		client := mcp.NewClient(
			&mcp.Implementation{Name: "example", Version: "1.0.0"}, nil,
		)
		// Response ignored: this example asserts authorization only.
		_, _ = client.Connect(context.Background(), transport, nil)
	}

	// The first connection has no saved session, so it authorizes and saves.
	fmt.Println("--- First connect ---")
	connect()
	// The second connection restores the saved session and reuses it, so no
	// further authorization or save occurs.
	fmt.Println("--- Second connect ---")
	connect()

	// Output:
	// --- First connect ---
	// No session found to restore.
	// No token source found. Transport is calling Authorize()...
	// Saving token: mock-token
	// --- Second connect ---
	// Restoring session.
}
