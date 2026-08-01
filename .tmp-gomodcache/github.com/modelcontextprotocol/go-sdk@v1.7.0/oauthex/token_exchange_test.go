// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package oauthex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestExchangeToken tests the basic token exchange flow.
func TestExchangeToken(t *testing.T) {
	// Create a test IdP server that implements token exchange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type
		if r.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/x-www-form-urlencoded" {
			t.Errorf("expected application/x-www-form-urlencoded, got %s", contentType)
			http.Error(w, "invalid content type", http.StatusBadRequest)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "failed to parse form", http.StatusBadRequest)
			return
		}

		// Verify required parameters per SEP-990 Section 4
		grantType := r.FormValue("grant_type")
		if grantType != GrantTypeTokenExchange {
			t.Errorf("expected grant_type %s, got %s", GrantTypeTokenExchange, grantType)
			writeErrorResponse(w, "invalid_grant", "invalid grant_type")
			return
		}

		requestedTokenType := r.FormValue("requested_token_type")
		if requestedTokenType != TokenTypeIDJAG {
			t.Errorf("expected requested_token_type %s, got %s", TokenTypeIDJAG, requestedTokenType)
			writeErrorResponse(w, "invalid_request", "invalid requested_token_type")
			return
		}

		audience := r.FormValue("audience")
		if audience == "" {
			t.Error("audience is required")
			writeErrorResponse(w, "invalid_request", "missing audience")
			return
		}

		resource := r.FormValue("resource")
		if resource == "" {
			t.Error("resource is required")
			writeErrorResponse(w, "invalid_request", "missing resource")
			return
		}

		subjectToken := r.FormValue("subject_token")
		if subjectToken == "" {
			t.Error("subject_token is required")
			writeErrorResponse(w, "invalid_request", "missing subject_token")
			return
		}

		subjectTokenType := r.FormValue("subject_token_type")
		if subjectTokenType != TokenTypeIDToken {
			t.Errorf("expected subject_token_type %s, got %s", TokenTypeIDToken, subjectTokenType)
			writeErrorResponse(w, "invalid_request", "invalid subject_token_type")
			return
		}

		// Verify client authentication
		clientID := r.FormValue("client_id")
		clientSecret := r.FormValue("client_secret")
		if clientID == "" || clientSecret == "" {
			t.Error("client authentication required")
			writeErrorResponse(w, "invalid_client", "client authentication failed")
			return
		}

		if clientID != "test-client-id" || clientSecret != "test-client-secret" {
			t.Error("invalid client credentials")
			writeErrorResponse(w, "invalid_client", "invalid credentials")
			return
		}

		// Return successful token exchange response per SEP-990 Section 4.2
		resp := map[string]any{
			"issued_token_type": TokenTypeIDJAG,
			"access_token":      "fake-id-jag-token",
			"token_type":        "N_A",
			"scope":             r.FormValue("scope"),
			"expires_in":        300,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Test successful token exchange
	t.Run("successful exchange", func(t *testing.T) {
		req := &TokenExchangeRequest{
			RequestedTokenType: TokenTypeIDJAG,
			Audience:           "https://auth.mcpserver.example/",
			Resource:           "https://mcp.mcpserver.example/",
			Scope:              []string{"read", "write"},
			SubjectToken:       "fake-id-token",
			SubjectTokenType:   TokenTypeIDToken,
		}

		token, err := ExchangeToken(
			context.Background(),
			server.URL,
			req,
			&ClientCredentials{
				ClientID: "test-client-id",
				ClientSecretAuth: &ClientSecretAuth{
					ClientSecret: "test-client-secret",
				},
			},
			server.Client(),
		)

		if err != nil {
			t.Fatalf("ExchangeToken failed: %v", err)
		}

		issuedTokenType, ok := token.Extra("issued_token_type").(string)
		if !ok || issuedTokenType != TokenTypeIDJAG {
			t.Errorf("expected issued_token_type %s, got %s", TokenTypeIDJAG, issuedTokenType)
		}

		if token.AccessToken != "fake-id-jag-token" {
			t.Errorf("expected access_token 'fake-id-jag-token', got %s", token.AccessToken)
		}

		if token.TokenType != "N_A" {
			t.Errorf("expected token_type 'N_A', got %s", token.TokenType)
		}

		scope, ok := token.Extra("scope").(string)
		if !ok || scope != "read write" {
			t.Errorf("expected scope 'read write', got %s", scope)
		}

		// expires_in should be available in Extra
		expiresIn, ok := token.Extra("expires_in").(float64)
		if !ok || int(expiresIn) != 300 {
			t.Errorf("expected expires_in 300, got %v", expiresIn)
		}
	})

	// Test missing required fields
	t.Run("missing audience", func(t *testing.T) {
		req := &TokenExchangeRequest{
			RequestedTokenType: TokenTypeIDJAG,
			Resource:           "https://mcp.mcpserver.example/",
			SubjectToken:       "fake-id-token",
			SubjectTokenType:   TokenTypeIDToken,
		}

		_, err := ExchangeToken(
			context.Background(),
			server.URL,
			req,
			&ClientCredentials{
				ClientID: "test-client-id",
				ClientSecretAuth: &ClientSecretAuth{
					ClientSecret: "test-client-secret",
				},
			},
			server.Client(),
		)

		if err == nil {
			t.Error("expected error for missing audience, got nil")
		}
	})

	// Test invalid URL schemes
	t.Run("invalid audience URL scheme", func(t *testing.T) {
		req := &TokenExchangeRequest{
			RequestedTokenType: TokenTypeIDJAG,
			Audience:           "javascript:alert(1)",
			Resource:           "https://mcp.mcpserver.example/",
			SubjectToken:       "fake-id-token",
			SubjectTokenType:   TokenTypeIDToken,
		}

		_, err := ExchangeToken(
			context.Background(),
			server.URL,
			req,
			&ClientCredentials{
				ClientID: "test-client-id",
				ClientSecretAuth: &ClientSecretAuth{
					ClientSecret: "test-client-secret",
				},
			},
			server.Client(),
		)

		if err == nil {
			t.Error("expected error for invalid audience URL scheme, got nil")
		}
	})
}

// writeErrorResponse writes an OAuth 2.0 error response per RFC 6749 Section 5.2.
func writeErrorResponse(w http.ResponseWriter, errorCode, errorDescription string) {
	errResp := struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description,omitempty"`
	}{
		Error:            errorCode,
		ErrorDescription: errorDescription,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(errResp)
}
