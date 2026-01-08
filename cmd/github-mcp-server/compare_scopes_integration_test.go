package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompareScopesIntegration tests the full compare-scopes flow with a mock server
func TestCompareScopesIntegration(t *testing.T) {
	// Create a mock GitHub API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Return some test scopes
		w.Header().Set(scopes.OAuthScopesHeader, "repo, read:org, gist")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Configure viper with test values
	viper.Set("personal_access_token", "test_token")
	viper.Set("host", server.URL)

	// Capture output by temporarily redirecting stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Run the compareScopes function
	err := compareScopes()

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	outputBytes, _ := io.ReadAll(r)
	output := string(outputBytes)

	// Verify output contains expected sections
	assert.Contains(t, output, "Fetching token scopes from")

	// The function may return an error if some scopes are missing
	// We're mainly testing that it runs without panicking
	if err != nil {
		// It's ok if there are missing scopes - that's expected
		assert.Contains(t, err.Error(), "missing")
	}
}

// TestCompareScopesWithoutToken tests that the command fails gracefully without a token
func TestCompareScopesWithoutToken(t *testing.T) {
	// Clear the token
	viper.Set("personal_access_token", "")

	// Run the compareScopes function
	err := compareScopes()

	// Should return an error about missing token
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GITHUB_PERSONAL_ACCESS_TOKEN")
}

// TestCompareScopesWithInvalidToken tests error handling with invalid token
func TestCompareScopesWithInvalidToken(t *testing.T) {
	// Create a mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	// Configure viper
	viper.Set("personal_access_token", "invalid_token")
	viper.Set("host", server.URL)

	// Suppress stderr output during this test
	oldStderr := os.Stderr
	os.Stderr = nil
	defer func() { os.Stderr = oldStderr }()

	// Run the compareScopes function
	err := compareScopes()

	// Should return an error about invalid token
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired token")
}
