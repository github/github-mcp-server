package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestPrivateKeyPEM generates a temporary 2048-bit RSA key for testing
func generateTestPrivateKeyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.EncodeToMemory(pemBlock)
}

func TestNewGitHubAppTokenSource_Errors(t *testing.T) {
	// Test when no key is provided
	_, err := NewGitHubAppTokenSource(12345, 67890, nil, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be provided")

	// Test invalid PEM key
	_, err = NewGitHubAppTokenSource(12345, 67890, []byte("invalid-key"), "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid app-private-key")
}

func TestGitHubAppTokenSource_Token(t *testing.T) {
	keyPEM := generateTestPrivateKeyPEM(t)

	// Create a test HTTP server to mock GitHub App installation token endpoint
	serverCalled := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		assert.Equal(t, "/api/v3/app/installations/67890/access_tokens", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Verify auth header
		authHeader := r.Header.Get("Authorization")
		assert.Contains(t, authHeader, "Bearer ")

		// Verify other headers
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "github-mcp-server", r.Header.Get("User-Agent"))

		serverCalled++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"token": "test-installation-token", "expires_at": "2026-07-19T02:00:00Z"}`)
	}))
	defer ts.Close()

	// Initialize the TokenSource with our mock server URL as host
	tokenSource, err := NewGitHubAppTokenSource(12345, 67890, keyPEM, "", ts.URL)
	require.NoError(t, err)

	// Get Token first time (should trigger HTTP request)
	tok, err := tokenSource.Token()
	require.NoError(t, err)
	assert.Equal(t, "test-installation-token", tok.AccessToken)
	assert.Equal(t, 1, serverCalled)

	// Get Token second time (should use cache, so serverCalled stays 1)
	tok2, err := tokenSource.Token()
	require.NoError(t, err)
	assert.Equal(t, "test-installation-token", tok2.AccessToken)
	assert.Equal(t, 1, serverCalled)
}
