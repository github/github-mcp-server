package oauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePKCEVerifier(t *testing.T) {
	verifier, err := generatePKCEVerifier()
	require.NoError(t, err)

	// Base64URL encoding of 32 bytes = 43 characters
	assert.GreaterOrEqual(t, len(verifier), 43)

	verifier2, err := generatePKCEVerifier()
	require.NoError(t, err)
	assert.NotEqual(t, verifier, verifier2)
}

func TestGenerateRandomToken(t *testing.T) {
	token1, err := generateRandomToken()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(token1), 20)

	token2, err := generateRandomToken()
	require.NoError(t, err)
	assert.NotEqual(t, token1, token2)
}

func TestGetGitHubOAuthConfig(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		callbackPort  int
		wantAuthURL   string
		wantTokenURL  string
		wantDeviceURL string
	}{
		{
			name:          "default github.com",
			host:          "",
			wantAuthURL:   "https://github.com/login/oauth/authorize",
			wantTokenURL:  "https://github.com/login/oauth/access_token",
			wantDeviceURL: "https://github.com/login/device/code",
		},
		{
			name:          "GHES host with scheme",
			host:          "https://github.enterprise.com",
			callbackPort:  8085,
			wantAuthURL:   "https://github.enterprise.com/login/oauth/authorize",
			wantTokenURL:  "https://github.enterprise.com/login/oauth/access_token",
			wantDeviceURL: "https://github.enterprise.com/login/device/code",
		},
		{
			name:          "GHEC host (ghe.com)",
			host:          "https://mycompany.ghe.com",
			wantAuthURL:   "https://mycompany.ghe.com/login/oauth/authorize",
			wantTokenURL:  "https://mycompany.ghe.com/login/oauth/access_token",
			wantDeviceURL: "https://mycompany.ghe.com/login/device/code",
		},
		{
			name:          "host without scheme defaults to https",
			host:          "github.enterprise.com",
			wantAuthURL:   "https://github.enterprise.com/login/oauth/authorize",
			wantTokenURL:  "https://github.enterprise.com/login/oauth/access_token",
			wantDeviceURL: "https://github.enterprise.com/login/device/code",
		},
		{
			name:          "api.github.com strips api subdomain",
			host:          "api.github.com",
			wantAuthURL:   "https://github.com/login/oauth/authorize",
			wantTokenURL:  "https://github.com/login/oauth/access_token",
			wantDeviceURL: "https://github.com/login/device/code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GetGitHubOAuthConfig("cid", "csecret", []string{"repo"}, tt.host, tt.callbackPort)

			assert.Equal(t, "cid", cfg.ClientID)
			assert.Equal(t, "csecret", cfg.ClientSecret)
			assert.Equal(t, []string{"repo"}, cfg.Scopes)
			assert.Equal(t, tt.wantAuthURL, cfg.AuthURL)
			assert.Equal(t, tt.wantTokenURL, cfg.TokenURL)
			assert.Equal(t, tt.wantDeviceURL, cfg.DeviceAuthURL)
			assert.Equal(t, tt.callbackPort, cfg.CallbackPort)
		})
	}
}

func TestStartLocalServer(t *testing.T) {
	t.Run("random port binds to localhost", func(t *testing.T) {
		listener, port, err := startLocalServer(0)
		require.NoError(t, err)
		defer listener.Close()

		assert.Greater(t, port, 0)
		// Random port binds to 127.0.0.1 (secure, native only)
		assert.Contains(t, listener.Addr().String(), "127.0.0.1:")
	})

	t.Run("fixed port binds to all interfaces", func(t *testing.T) {
		fixedPort := 54321
		listener, port, err := startLocalServer(fixedPort)
		require.NoError(t, err)
		defer listener.Close()

		assert.Equal(t, fixedPort, port)
		// Fixed port binds to all interfaces (0.0.0.0 or [::]) for Docker port mapping
		addr := listener.Addr().String()
		assert.True(t, strings.Contains(addr, "0.0.0.0:") || strings.Contains(addr, "[::]:"),
			"expected all-interface bind, got %s", addr)
	})
}

func TestCallbackHandler(t *testing.T) {
	expectedState := "test-state-12345"

	t.Run("successful callback", func(t *testing.T) {
		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)
		handler := createCallbackHandler(expectedState, codeChan, errChan)

		req := httptest.NewRequest("GET", "/callback?code=test-code&state=test-state-12345", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
		assert.Contains(t, w.Body.String(), "Authorization Successful")

		select {
		case code := <-codeChan:
			assert.Equal(t, "test-code", code)
		default:
			t.Fatal("expected code on channel")
		}
	})

	t.Run("state mismatch", func(t *testing.T) {
		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)
		handler := createCallbackHandler(expectedState, codeChan, errChan)

		req := httptest.NewRequest("GET", "/callback?code=test-code&state=wrong-state", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		select {
		case err := <-errChan:
			assert.Contains(t, err.Error(), "state mismatch")
		default:
			t.Fatal("expected error on channel")
		}
	})

	t.Run("missing code", func(t *testing.T) {
		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)
		handler := createCallbackHandler(expectedState, codeChan, errChan)

		req := httptest.NewRequest("GET", "/callback?state=test-state-12345", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		select {
		case err := <-errChan:
			assert.Contains(t, err.Error(), "no authorization code")
		default:
			t.Fatal("expected error on channel")
		}
	})

	t.Run("OAuth error response", func(t *testing.T) {
		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)
		handler := createCallbackHandler(expectedState, codeChan, errChan)

		req := httptest.NewRequest("GET", "/callback?error=access_denied&error_description=User+denied+access", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code) // Error template renders with 200
		assert.Contains(t, w.Body.String(), "Authorization Failed")

		select {
		case err := <-errChan:
			assert.Contains(t, err.Error(), "access_denied")
			assert.Contains(t, err.Error(), "User denied access")
		default:
			t.Fatal("expected error on channel")
		}
	})

	t.Run("XSS prevention in error messages", func(t *testing.T) {
		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)
		handler := createCallbackHandler(expectedState, codeChan, errChan)

		// Attempt XSS via error parameter — html/template auto-escapes
		req := httptest.NewRequest("GET", `/callback?error=<script>alert('xss')</script>`, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		body := w.Body.String()
		assert.NotContains(t, body, "<script>")
		assert.Contains(t, body, "&lt;script&gt;")

		// Drain error channel
		<-errChan
	})
}

func TestManagerTokenOperations(t *testing.T) {
	mgr := NewManager(Config{}, nil)

	t.Run("no token initially", func(t *testing.T) {
		assert.False(t, mgr.HasToken())
		assert.Empty(t, mgr.GetAccessToken())
	})

	t.Run("has token after setting", func(t *testing.T) {
		mgr.setToken(&Result{
			AccessToken: "gho_test123456",
			TokenType:   "Bearer",
		})
		assert.True(t, mgr.HasToken())
		assert.Equal(t, "gho_test123456", mgr.GetAccessToken())
	})

	t.Run("no token if empty access token", func(t *testing.T) {
		mgr.setToken(&Result{AccessToken: "", TokenType: "Bearer"})
		assert.False(t, mgr.HasToken())
	})

	t.Run("full result stored correctly", func(t *testing.T) {
		expiry := time.Now().Add(time.Hour)
		mgr.setToken(&Result{
			AccessToken:  "access-123",
			RefreshToken: "refresh-456",
			TokenType:    "Bearer",
			Expiry:       expiry,
		})
		assert.Equal(t, "access-123", mgr.GetAccessToken())
		assert.True(t, mgr.HasToken())
	})
}

func TestIsRunningInDocker(_ *testing.T) {
	// This test validates the function doesn't panic.
	// The actual result depends on the test environment.
	result := IsRunningInDocker()
	_ = result // Just ensure it doesn't panic
}
