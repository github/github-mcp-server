package ghmcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type dummyRequest struct {
	Test string `json:"test"`
}

func TestMultiUserHTTPServer_TokenRequired(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractTokenFromRequest(r)
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing GitHub token in Authorization header"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	ts := httptest.NewServer(h)
	defer ts.Close()

	// No Authorization header
	resp, err := http.Post(ts.URL, "application/json", bytes.NewBufferString(`{"test":"noauth"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}
}

func TestMultiUserHTTPServer_ValidToken(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractTokenFromRequest(r)
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing GitHub token in Authorization header"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"token":"` + token + `"}`))
	})

	ts := httptest.NewServer(h)
	defer ts.Close()

	// With Authorization header
	body, _ := json.Marshal(dummyRequest{Test: "authed"})
	req, _ := http.NewRequest("POST", ts.URL, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer testtoken123")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
	data, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(data, []byte("testtoken123")) {
		t.Errorf("expected token in response, got %s", string(data))
	}
}

func TestExtractTokenFromRequest_StrictValidation(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"valid bearer token", "Bearer ghp_1234567890abcdef", "ghp_1234567890abcdef"},
		{"valid long token", "Bearer github_pat_11AAAAAAA0AAAAAAAAAAAAAAAAA", "github_pat_11AAAAAAA0AAAAAAAAAAAAAAAAA"},
		{"missing header", "", ""},
		{"malformed - no Bearer", "ghp_1234567890abcdef", ""},
		{"malformed - wrong case", "bearer ghp_1234567890abcdef", ""},
		{"too short token", "Bearer abc", ""},
		{"empty token", "Bearer ", ""},
		{"only Bearer", "Bearer", ""},
		{"spaces in token", "Bearer ghp_123 456", "ghp_123 456"}, // This should work
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: http.Header{},
			}
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			
			result := extractTokenFromRequest(req)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMultiUserHandler_ContextInjection(t *testing.T) {
	// Mock MCP server that verifies token is in context
	mockMCP := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := r.Context().Value("github_token").(string)
		if !ok {
			t.Error("token not found in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if token != "test_token_123456" {
			t.Errorf("wrong token in context: %s", token)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	})

	handler := &multiUserHandler{mcpServer: mockMCP}

	req := httptest.NewRequest("POST", "/", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer test_token_123456")
	
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestMultiUserHandler_MissingToken(t *testing.T) {
	// Mock MCP server (should not be called)
	mockMCP := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("MCP server should not be called when token is missing")
	})

	handler := &multiUserHandler{mcpServer: mockMCP}

	req := httptest.NewRequest("POST", "/", strings.NewReader("{}"))
	// No Authorization header
	
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %s", contentType)
	}

	if !strings.Contains(w.Body.String(), "missing GitHub token") {
		t.Errorf("expected error message about missing token, got: %s", w.Body.String())
	}
}
