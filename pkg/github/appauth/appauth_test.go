package appauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return key, pemBytes
}

func TestParsePrivateKey_PKCS1(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	key, err := parsePrivateKey(pemBytes)
	require.NoError(t, err)
	assert.NotNil(t, key)
}

func TestParsePrivateKey_PKCS8(t *testing.T) {
	rsaKey, _ := generateTestKey(t)
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	key, err := parsePrivateKey(pemBytes)
	require.NoError(t, err)
	assert.NotNil(t, key)
}

func TestParsePrivateKey_InvalidPEM(t *testing.T) {
	_, err := parsePrivateKey([]byte("not a pem"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block")
}

func TestParsePrivateKey_UnsupportedType(t *testing.T) {
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: []byte("fake"),
	})
	_, err := parsePrivateKey(pemBytes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported PEM block type")
}

func TestNewTransport_InvalidKey(t *testing.T) {
	_, err := NewTransport(nil, Config{
		AppID:          123,
		PrivateKey:     []byte("invalid"),
		InstallationID: 456,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse private key")
}

func TestNewTransport_DefaultBaseURL(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	tr, err := NewTransport(nil, Config{
		AppID:          123,
		PrivateKey:     pemBytes,
		InstallationID: 456,
	})
	require.NoError(t, err)
	assert.Equal(t, "https://api.github.com", tr.config.BaseURL)
}

func TestNewTransport_CustomBaseURL(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	tr, err := NewTransport(nil, Config{
		AppID:          123,
		PrivateKey:     pemBytes,
		InstallationID: 456,
		BaseURL:        "https://github.example.com/api/v3",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://github.example.com/api/v3", tr.config.BaseURL)
}

func TestTransport_GenerateJWT(t *testing.T) {
	key, pemBytes := generateTestKey(t)
	tr, err := NewTransport(nil, Config{
		AppID:          12345,
		PrivateKey:     pemBytes,
		InstallationID: 67890,
	})
	require.NoError(t, err)

	jwtToken, err := tr.generateJWT()
	require.NoError(t, err)

	claims, err := VerifyJWT(jwtToken, &key.PublicKey)
	require.NoError(t, err)

	assert.Equal(t, "12345", claims["iss"])

	iat := int64(claims["iat"].(float64))
	exp := int64(claims["exp"].(float64))
	assert.InDelta(t, time.Now().Unix(), iat, 60)
	assert.InDelta(t, time.Now().Add(10*time.Minute).Unix(), exp, 60)
}

func TestTransport_FetchInstallationToken(t *testing.T) {
	key, pemBytes := generateTestKey(t)
	installationID := int64(67890)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/app/installations/%d/access_tokens", installationID)
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		authHeader := r.Header.Get("Authorization")
		assert.True(t, len(authHeader) > 7)
		jwtToken := authHeader[7:] // strip "Bearer "

		_, err := VerifyJWT(jwtToken, &key.PublicKey)
		assert.NoError(t, err)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(installationToken{
			Token:     "ghs_test_token_123",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		})
	}))
	defer server.Close()

	tr, err := NewTransport(server.Client().Transport, Config{
		AppID:          12345,
		PrivateKey:     pemBytes,
		InstallationID: installationID,
		BaseURL:        server.URL,
	})
	require.NoError(t, err)

	token, err := tr.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghs_test_token_123", token)
}

func TestTransport_TokenCaching(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(installationToken{
			Token:     "ghs_cached_token",
			ExpiresAt: time.Now().Add(1 * time.Hour),
		})
	}))
	defer server.Close()

	tr, err := NewTransport(server.Client().Transport, Config{
		AppID:          12345,
		PrivateKey:     pemBytes,
		InstallationID: 67890,
		BaseURL:        server.URL,
	})
	require.NoError(t, err)

	token1, err := tr.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghs_cached_token", token1)

	token2, err := tr.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghs_cached_token", token2)

	assert.Equal(t, int32(1), callCount.Load())
}

func TestTransport_TokenRefresh(t *testing.T) {
	_, pemBytes := generateTestKey(t)
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := callCount.Add(1)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(installationToken{
			Token:     fmt.Sprintf("ghs_token_%d", count),
			ExpiresAt: time.Now().Add(1 * time.Minute), // expires soon, within 5min refresh window
		})
	}))
	defer server.Close()

	tr, err := NewTransport(server.Client().Transport, Config{
		AppID:          12345,
		PrivateKey:     pemBytes,
		InstallationID: 67890,
		BaseURL:        server.URL,
	})
	require.NoError(t, err)

	token1, err := tr.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghs_token_1", token1)

	// Token expires within 5 minutes, so next call should refresh
	token2, err := tr.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ghs_token_2", token2)
	assert.Equal(t, int32(2), callCount.Load())
}

func TestTransport_RoundTrip(t *testing.T) {
	_, pemBytes := generateTestKey(t)

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/app/installations/67890/access_tokens" {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(installationToken{
				Token:     "ghs_roundtrip_token",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			})
			return
		}
		assert.Equal(t, "Bearer ghs_roundtrip_token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer tokenServer.Close()

	tr, err := NewTransport(tokenServer.Client().Transport, Config{
		AppID:          12345,
		PrivateKey:     pemBytes,
		InstallationID: 67890,
		BaseURL:        tokenServer.URL,
	})
	require.NoError(t, err)

	client := &http.Client{Transport: tr}
	resp, err := client.Get(tokenServer.URL + "/repos/owner/repo")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTransport_FetchError(t *testing.T) {
	_, pemBytes := generateTestKey(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
	}))
	defer server.Close()

	tr, err := NewTransport(server.Client().Transport, Config{
		AppID:          12345,
		PrivateKey:     pemBytes,
		InstallationID: 67890,
		BaseURL:        server.URL,
	})
	require.NoError(t, err)

	_, err = tr.Token(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create installation token")
	assert.Contains(t, err.Error(), "Bad credentials")
}
