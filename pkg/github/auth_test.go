package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOAuthHostFromAPIHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		apiHost          string
		expectedHostname string
		expectedDevice   string
		expectedToken    string
	}{
		{
			name:             "github.com (empty host)",
			apiHost:          "",
			expectedHostname: "github.com",
			expectedDevice:   "https://github.com/login/device/code",
			expectedToken:    "https://github.com/login/oauth/access_token",
		},
		{
			name:             "github.com (explicit)",
			apiHost:          "github.com",
			expectedHostname: "github.com",
			expectedDevice:   "https://github.com/login/device/code",
			expectedToken:    "https://github.com/login/oauth/access_token",
		},
		{
			name:             "GHES without scheme",
			apiHost:          "github.enterprise.com",
			expectedHostname: "github.enterprise.com",
			expectedDevice:   "https://github.enterprise.com/login/device/code",
			expectedToken:    "https://github.enterprise.com/login/oauth/access_token",
		},
		{
			name:             "GHES with https scheme",
			apiHost:          "https://github.enterprise.com",
			expectedHostname: "github.enterprise.com",
			expectedDevice:   "https://github.enterprise.com/login/device/code",
			expectedToken:    "https://github.enterprise.com/login/oauth/access_token",
		},
		{
			name:             "GHEC tenant",
			apiHost:          "company.ghe.com",
			expectedHostname: "company.ghe.com",
			expectedDevice:   "https://company.ghe.com/login/device/code",
			expectedToken:    "https://company.ghe.com/login/oauth/access_token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			host := NewOAuthHostFromAPIHost(tc.apiHost)
			assert.Equal(t, tc.expectedHostname, host.Hostname)
			assert.Equal(t, tc.expectedDevice, host.DeviceCodeURL)
			assert.Equal(t, tc.expectedToken, host.TokenURL)
		})
	}
}

func TestAuthManager_StateTransitions(t *testing.T) {
	t.Parallel()

	host := NewOAuthHostFromAPIHost("")
	authMgr := NewAuthManager(host, "test-client-id", "", nil)

	// Initial state should be unauthenticated
	assert.Equal(t, AuthStateUnauthenticated, authMgr.State())
	assert.False(t, authMgr.IsAuthenticated())
	assert.Empty(t, authMgr.Token())

	// Cannot call Reset when not pending
	authMgr.Reset()
	assert.Equal(t, AuthStateUnauthenticated, authMgr.State())
}

func TestAuthManager_StartDeviceFlow(t *testing.T) {
	t.Parallel()

	// Create a mock OAuth server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device/code" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]interface{}{
				"device_code":      "test-device-code",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900,
				"interval":         5,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	host := OAuthHost{
		Hostname:      "test.example.com",
		DeviceCodeURL: server.URL + "/login/device/code",
		TokenURL:      server.URL + "/login/oauth/access_token",
	}

	authMgr := NewAuthManager(host, "test-client-id", "", nil)

	// Start the device flow
	deviceResp, err := authMgr.StartDeviceFlow(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "test-device-code", deviceResp.DeviceCode)
	assert.Equal(t, "ABCD-1234", deviceResp.UserCode)
	assert.Equal(t, "https://github.com/login/device", deviceResp.VerificationURI)

	// State should now be pending
	assert.Equal(t, AuthStatePending, authMgr.State())
}

func TestAuthManager_CompleteDeviceFlow_Success(t *testing.T) {
	t.Parallel()

	// Track poll attempts
	pollCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device/code" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]interface{}{
				"device_code":      "test-device-code",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900,
				"interval":         1, // Short interval for test
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/login/oauth/access_token" {
			pollCount++
			w.Header().Set("Content-Type", "application/json")
			if pollCount < 2 {
				// First poll returns pending
				resp := map[string]interface{}{
					"error": "authorization_pending",
				}
				_ = json.NewEncoder(w).Encode(resp)
			} else {
				// Second poll returns token
				resp := map[string]interface{}{
					"access_token": "gho_test_token_12345",
					"token_type":   "bearer",
					"scope":        "repo,read:org",
				}
				_ = json.NewEncoder(w).Encode(resp)
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	host := OAuthHost{
		Hostname:      "test.example.com",
		DeviceCodeURL: server.URL + "/login/device/code",
		TokenURL:      server.URL + "/login/oauth/access_token",
	}

	authMgr := NewAuthManager(host, "test-client-id", "", nil)

	// Start the device flow
	_, err := authMgr.StartDeviceFlow(context.Background())
	require.NoError(t, err)

	// Complete the flow
	err = authMgr.CompleteDeviceFlow(context.Background())
	require.NoError(t, err)

	// Should now be authenticated
	assert.Equal(t, AuthStateAuthenticated, authMgr.State())
	assert.True(t, authMgr.IsAuthenticated())
	assert.Equal(t, "gho_test_token_12345", authMgr.Token())
}

func TestAuthManager_CompleteDeviceFlow_AccessDenied(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device/code" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]interface{}{
				"device_code":      "test-device-code",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900,
				"interval":         1,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/login/oauth/access_token" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]interface{}{
				"error":             "access_denied",
				"error_description": "The user has denied your request.",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	host := OAuthHost{
		Hostname:      "test.example.com",
		DeviceCodeURL: server.URL + "/login/device/code",
		TokenURL:      server.URL + "/login/oauth/access_token",
	}

	authMgr := NewAuthManager(host, "test-client-id", "", nil)

	// Start the device flow
	_, err := authMgr.StartDeviceFlow(context.Background())
	require.NoError(t, err)

	// Complete the flow - should fail with access denied
	err = authMgr.CompleteDeviceFlow(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")

	// Should be back to unauthenticated
	assert.Equal(t, AuthStateUnauthenticated, authMgr.State())
}

func TestAuthManager_Reset(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device/code" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]interface{}{
				"device_code":      "test-device-code",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://github.com/login/device",
				"expires_in":       900,
				"interval":         1,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	host := OAuthHost{
		Hostname:      "test.example.com",
		DeviceCodeURL: server.URL + "/login/device/code",
		TokenURL:      server.URL + "/login/oauth/access_token",
	}

	authMgr := NewAuthManager(host, "test-client-id", "", nil)

	// Start the device flow
	_, err := authMgr.StartDeviceFlow(context.Background())
	require.NoError(t, err)
	assert.Equal(t, AuthStatePending, authMgr.State())

	// Reset should clear the pending state
	authMgr.Reset()
	assert.Equal(t, AuthStateUnauthenticated, authMgr.State())
}
