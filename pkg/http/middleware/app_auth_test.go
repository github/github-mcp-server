package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github/appauth"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateAppKey(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

func newAppTransport(t *testing.T, baseURL string) *appauth.Transport {
	t.Helper()
	tr, err := appauth.NewTransport(http.DefaultTransport, appauth.Config{
		AppID:          12345,
		PrivateKey:     generateAppKey(t),
		InstallationID: 67890,
		BaseURL:        baseURL,
	})
	require.NoError(t, err)
	return tr
}

func TestWithGitHubAppToken_InjectsToken(t *testing.T) {
	var hits atomic.Int32
	ghAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "ghs_injected_token",
			"expires_at": time.Now().Add(1 * time.Hour),
		})
	}))
	defer ghAPI.Close()

	tr := newAppTransport(t, ghAPI.URL)

	var captured *ghcontext.TokenInfo
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		info, _ := ghcontext.GetTokenInfo(r.Context())
		captured = info
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := WithGitHubAppToken(tr, logger)(next)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, captured)
	assert.Equal(t, "ghs_injected_token", captured.Token)
	assert.Equal(t, utils.TokenTypeServerToServerGitHubAppToken, captured.TokenType)
	assert.Equal(t, int32(1), hits.Load())
}

func TestWithGitHubAppToken_PreservesExistingTokenInfo(t *testing.T) {
	var hits atomic.Int32
	ghAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "ghs_should_not_be_used",
			"expires_at": time.Now().Add(1 * time.Hour),
		})
	}))
	defer ghAPI.Close()

	tr := newAppTransport(t, ghAPI.URL)

	pre := &ghcontext.TokenInfo{
		Token:     "ghp_explicit",
		TokenType: utils.TokenTypePersonalAccessToken,
	}

	var captured *ghcontext.TokenInfo
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		info, _ := ghcontext.GetTokenInfo(r.Context())
		captured = info
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := WithGitHubAppToken(tr, logger)(next)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req = req.WithContext(ghcontext.WithTokenInfo(req.Context(), pre))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, captured)
	assert.Equal(t, "ghp_explicit", captured.Token)
	assert.Equal(t, int32(0), hits.Load(), "installation token should not have been fetched")
}

func TestWithGitHubAppToken_PropagatesFetchError(t *testing.T) {
	ghAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer ghAPI.Close()

	tr := newAppTransport(t, ghAPI.URL)

	nextCalled := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := WithGitHubAppToken(tr, logger)(next)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.False(t, nextCalled, "next handler must not run when token fetch fails")
}
