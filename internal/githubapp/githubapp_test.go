package githubapp

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func pkcs1PEM(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func pkcs8PEM(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func TestParsePrivateKey(t *testing.T) {
	key := newTestKey(t)

	t.Run("PKCS1", func(t *testing.T) {
		got, err := ParsePrivateKey(pkcs1PEM(t, key))
		require.NoError(t, err)
		assert.Equal(t, key.N, got.N)
	})

	t.Run("PKCS8", func(t *testing.T) {
		got, err := ParsePrivateKey(pkcs8PEM(t, key))
		require.NoError(t, err)
		assert.Equal(t, key.N, got.N)
	})

	t.Run("not PEM", func(t *testing.T) {
		_, err := ParsePrivateKey([]byte("not a pem"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no PEM block")
	})

	t.Run("non-RSA key", func(t *testing.T) {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		der, err := x509.MarshalPKCS8PrivateKey(priv)
		require.NoError(t, err)
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

		_, err = ParsePrivateKey(keyPEM)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "want an RSA key")
	})
}

func TestConfigValidate(t *testing.T) {
	key := newTestKey(t)
	base := Config{AppID: "123", InstallationID: "456", PrivateKey: key, BaseRESTURL: "https://api.github.com/"}
	require.NoError(t, base.Validate())

	tests := []struct {
		name   string
		mutate func(c *Config)
		want   string
	}{
		{"missing app id", func(c *Config) { c.AppID = "" }, "App ID is required"},
		{"missing installation id", func(c *Config) { c.InstallationID = "" }, "installation ID is required"},
		{"missing private key", func(c *Config) { c.PrivateKey = nil }, "private key is required"},
		{"missing base url", func(c *Config) { c.BaseRESTURL = "" }, "REST base URL is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base
			tt.mutate(&c)
			err := c.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// verifyJWT parses and verifies an app JWT against the public key and returns
// its claims, asserting the structural requirements GitHub enforces.
func verifyJWT(t *testing.T, token string, pub *rsa.PublicKey) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "JWT must have three segments")

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var header map[string]string
	require.NoError(t, json.Unmarshal(headerJSON, &header))
	assert.Equal(t, "RS256", header["alg"])
	assert.Equal(t, "JWT", header["typ"])

	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	require.NoError(t, rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], signature), "signature must verify")

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(claimsJSON, &claims))
	return claims
}

func TestMintJWT(t *testing.T) {
	key := newTestKey(t)
	cfg := Config{AppID: "my-app-id", PrivateKey: key}

	now := time.Now()
	token, err := cfg.mintJWT(now)
	require.NoError(t, err)

	claims := verifyJWT(t, token, &key.PublicKey)
	assert.Equal(t, "my-app-id", claims["iss"])

	iat := int64(claims["iat"].(float64))
	exp := int64(claims["exp"].(float64))
	assert.Equal(t, now.Add(-clockSkew).Unix(), iat, "iat should be backdated by the clock skew")
	assert.Equal(t, now.Add(jwtLifetime).Unix(), exp)
	assert.LessOrEqual(t, exp-iat, int64((10 * time.Minute).Seconds()), "JWT must live no longer than GitHub's 10 minute cap")
}

// installationServer is a fake installation token endpoint that verifies the
// app JWT and returns a token expiring at expiresAt. It counts mint requests.
func installationServer(t *testing.T, pub *rsa.PublicKey, token string, expiresAt time.Time) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/app/installations/456/access_tokens", r.URL.Path)

		authz := r.Header.Get("Authorization")
		require.True(t, strings.HasPrefix(authz, "Bearer "), "must send the app JWT as a bearer token")
		verifyJWT(t, strings.TrimPrefix(authz, "Bearer "), pub)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      token,
			"expires_at": expiresAt.UTC().Format(time.RFC3339),
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func newTestConfig(key *rsa.PrivateKey, baseURL string) Config {
	return Config{AppID: "123", InstallationID: "456", PrivateKey: key, BaseRESTURL: baseURL + "/"}
}

func TestProviderFetchesToken(t *testing.T) {
	key := newTestKey(t)
	srv, calls := installationServer(t, &key.PublicKey, "ghs_fresh", time.Now().Add(time.Hour))

	provider, err := NewProvider(newTestConfig(key, srv.URL), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	require.NoError(t, err)

	assert.Equal(t, "ghs_fresh", provider.AccessToken())
	assert.True(t, provider.HasToken())
	assert.Equal(t, int32(1), calls.Load())
}

func TestProviderCachesToken(t *testing.T) {
	key := newTestKey(t)
	srv, calls := installationServer(t, &key.PublicKey, "ghs_cached", time.Now().Add(time.Hour))

	provider, err := NewProvider(newTestConfig(key, srv.URL), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	require.NoError(t, err)

	for range 3 {
		assert.Equal(t, "ghs_cached", provider.AccessToken())
	}
	assert.Equal(t, int32(1), calls.Load(), "a token valid for an hour should be minted only once")
}

func TestProviderRefreshesNearExpiry(t *testing.T) {
	key := newTestKey(t)
	// expires within the refresh buffer, so the stored expiry is already in the
	// past and every call re-mints.
	srv, calls := installationServer(t, &key.PublicKey, "ghs_short", time.Now().Add(refreshBuffer-time.Minute))

	provider, err := NewProvider(newTestConfig(key, srv.URL), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	require.NoError(t, err)

	assert.Equal(t, "ghs_short", provider.AccessToken())
	assert.Equal(t, "ghs_short", provider.AccessToken())
	assert.Equal(t, int32(2), calls.Load(), "a token expiring within the refresh buffer should re-mint each call")
}

func TestProviderErrorLoggedOnce(t *testing.T) {
	key := newTestKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"A JSON web token could not be decoded"}`))
	}))
	t.Cleanup(srv.Close)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	provider, err := NewProvider(newTestConfig(key, srv.URL), logger)
	require.NoError(t, err)

	assert.Empty(t, provider.AccessToken())
	assert.Empty(t, provider.AccessToken())
	assert.Equal(t, 1, strings.Count(logBuf.String(), "failed to obtain GitHub App installation token"),
		"a repeated fetch failure should only be logged once")
}

func TestProviderErrorIncludesStatus(t *testing.T) {
	key := newTestKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	t.Cleanup(srv.Close)

	source := newInstallationTokenSource(newTestConfig(key, srv.URL), srv.Client())
	_, err := source.Token()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Not Found")
}

func TestNewProviderValidates(t *testing.T) {
	_, err := NewProvider(Config{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "App ID is required")
}

// Ensure the source returns an error rather than panicking on a token-less 201.
func TestSourceRejectsEmptyToken(t *testing.T) {
	key := newTestKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"expires_at":"2099-01-01T00:00:00Z"}`)
	}))
	t.Cleanup(srv.Close)

	source := newInstallationTokenSource(newTestConfig(key, srv.URL), srv.Client())
	_, err := source.Token()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not contain a token")
}
