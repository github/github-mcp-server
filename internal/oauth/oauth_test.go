package oauth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// expectedPKCEVerifierMinLength is the expected minimum length of a PKCE verifier
	// Base64URL encoding of 32 bytes = 43 characters (32 * 8 / 6, rounded up)
	expectedPKCEVerifierMinLength = 43
)

func TestGeneratePKCEVerifier(t *testing.T) {
	verifier, err := generatePKCEVerifier()
	require.NoError(t, err)
	require.NotEmpty(t, verifier)

	// Verifier should be at least 43 characters (base64url of 32 bytes)
	assert.GreaterOrEqual(t, len(verifier), expectedPKCEVerifierMinLength)

	// Generate another one to ensure they're different
	verifier2, err := generatePKCEVerifier()
	require.NoError(t, err)
	assert.NotEqual(t, verifier, verifier2)
}

func TestGetGitHubOAuthConfig(t *testing.T) {
	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	scopes := []string{"repo", "user"}

	t.Run("default github.com", func(t *testing.T) {
		cfg := GetGitHubOAuthConfig(clientID, clientSecret, scopes, "", 0)

		assert.Equal(t, clientID, cfg.ClientID)
		assert.Equal(t, clientSecret, cfg.ClientSecret)
		assert.Equal(t, scopes, cfg.Scopes)
		assert.Equal(t, "https://github.com/login/oauth/authorize", cfg.AuthURL)
		assert.Equal(t, "https://github.com/login/oauth/access_token", cfg.TokenURL)
		assert.Equal(t, "https://github.com/login/device/code", cfg.DeviceAuthURL)
		assert.Equal(t, "", cfg.Host)
		assert.Equal(t, 0, cfg.CallbackPort)
	})

	t.Run("GHES host", func(t *testing.T) {
		cfg := GetGitHubOAuthConfig(clientID, clientSecret, scopes, "https://github.enterprise.com", 8080)

		assert.Equal(t, clientID, cfg.ClientID)
		assert.Equal(t, clientSecret, cfg.ClientSecret)
		assert.Equal(t, scopes, cfg.Scopes)
		assert.Equal(t, "https://github.enterprise.com/login/oauth/authorize", cfg.AuthURL)
		assert.Equal(t, "https://github.enterprise.com/login/oauth/access_token", cfg.TokenURL)
		assert.Equal(t, "https://github.enterprise.com/login/device/code", cfg.DeviceAuthURL)
		assert.Equal(t, "https://github.enterprise.com", cfg.Host)
		assert.Equal(t, 8080, cfg.CallbackPort)
	})

	t.Run("GHEC host (ghe.com)", func(t *testing.T) {
		cfg := GetGitHubOAuthConfig(clientID, clientSecret, scopes, "https://mycompany.ghe.com", 0)

		assert.Equal(t, "https://mycompany.ghe.com/login/oauth/authorize", cfg.AuthURL)
		assert.Equal(t, "https://mycompany.ghe.com/login/oauth/access_token", cfg.TokenURL)
		assert.Equal(t, "https://mycompany.ghe.com/login/device/code", cfg.DeviceAuthURL)
	})

	t.Run("host without scheme", func(t *testing.T) {
		cfg := GetGitHubOAuthConfig(clientID, clientSecret, scopes, "github.enterprise.com", 0)

		// Should default to https
		assert.Equal(t, "https://github.enterprise.com/login/oauth/authorize", cfg.AuthURL)
	})
}

func TestStartLocalServer(t *testing.T) {
	t.Run("random port", func(t *testing.T) {
		listener, port, err := startLocalServer(0)
		require.NoError(t, err)
		require.NotNil(t, listener)
		defer listener.Close()

		assert.Greater(t, port, 0)
		assert.Less(t, port, 65536)
	})

	t.Run("fixed port", func(t *testing.T) {
		// Use a high port to avoid conflicts
		fixedPort := 54321
		listener, port, err := startLocalServer(fixedPort)
		require.NoError(t, err)
		require.NotNil(t, listener)
		defer listener.Close()

		assert.Equal(t, fixedPort, port)
	})
}

// Manager tests

func TestNewManager(t *testing.T) {
	cfg := Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		Scopes:       []string{"repo"},
	}

	mgr := NewManager(cfg)

	assert.NotNil(t, mgr)
	// Test observable behavior, not internal state
	assert.False(t, mgr.HasToken())
	assert.Empty(t, mgr.GetAccessToken())
}

func TestManagerHasToken(t *testing.T) {
	mgr := NewManager(Config{})

	t.Run("no token initially", func(t *testing.T) {
		assert.False(t, mgr.HasToken())
	})

	t.Run("has token after setting", func(t *testing.T) {
		mgr.setToken(&Result{
			AccessToken: "test-token",
			TokenType:   "Bearer",
		})

		assert.True(t, mgr.HasToken())
	})

	t.Run("no token if empty access token", func(t *testing.T) {
		mgr.setToken(&Result{
			AccessToken: "",
			TokenType:   "Bearer",
		})

		assert.False(t, mgr.HasToken())
	})
}

func TestManagerGetAccessToken(t *testing.T) {
	mgr := NewManager(Config{})

	t.Run("empty initially", func(t *testing.T) {
		assert.Empty(t, mgr.GetAccessToken())
	})

	t.Run("returns token after setting", func(t *testing.T) {
		expectedToken := "gho_test123456"
		mgr.setToken(&Result{
			AccessToken:  expectedToken,
			TokenType:    "Bearer",
			RefreshToken: "refresh-token",
			Expiry:       time.Now().Add(time.Hour),
		})

		assert.Equal(t, expectedToken, mgr.GetAccessToken())
	})
}

func TestManagerSetToken(t *testing.T) {
	mgr := NewManager(Config{})

	token := &Result{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	mgr.setToken(token)

	// Verify token is stored correctly
	assert.Equal(t, token.AccessToken, mgr.GetAccessToken())
	assert.True(t, mgr.HasToken())
}

func TestGenerateRandomToken(t *testing.T) {
	token1, err := generateRandomToken()
	require.NoError(t, err)
	require.NotEmpty(t, token1)

	// Token should be URL-safe base64 encoded
	// 16 bytes of random data = ~22 chars in base64url
	assert.GreaterOrEqual(t, len(token1), 20)

	// Each call should produce unique token
	token2, err := generateRandomToken()
	require.NoError(t, err)
	assert.NotEqual(t, token1, token2)
}
