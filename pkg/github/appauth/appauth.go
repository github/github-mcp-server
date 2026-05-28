package appauth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Config holds the configuration for GitHub App authentication.
type Config struct {
	// AppID is the GitHub App ID.
	AppID int64

	// PrivateKey is the PEM-encoded RSA private key for the GitHub App.
	PrivateKey []byte

	// InstallationID is the installation ID of the GitHub App.
	InstallationID int64

	// BaseURL is the base URL for the GitHub API (e.g., "https://api.github.com").
	// If empty, defaults to "https://api.github.com".
	BaseURL string
}

// Transport is an http.RoundTripper that authenticates requests using
// a GitHub App installation token. It automatically generates JWTs and
// fetches/refreshes installation tokens as needed.
type Transport struct {
	config Config
	key    *rsa.PrivateKey
	base   http.RoundTripper

	mu    sync.RWMutex
	token string
	exp   time.Time
}

type installationToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewTransport creates a new Transport that authenticates using a GitHub App
// installation token. The transport automatically handles JWT generation and
// installation token refresh.
// The base transport must not inject its own Authorization header, as this
// transport sets it for both installation token requests and API requests.
func NewTransport(base http.RoundTripper, cfg Config) (*Transport, error) {
	key, err := parsePrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	if base == nil {
		base = http.DefaultTransport
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.github.com"
	}
	return &Transport{
		config: cfg,
		key:    key,
		base:   base,
	}, nil
}

// RoundTrip implements http.RoundTripper. It adds the installation token
// to the Authorization header, refreshing it if necessary.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.installationToken(req.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to get installation token: %w", err)
	}
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(req2)
}

// Token returns the current installation token, refreshing if necessary.
func (t *Transport) Token(ctx context.Context) (string, error) {
	return t.installationToken(ctx)
}

func (t *Transport) installationToken(ctx context.Context) (string, error) {
	// Fast path: read lock to check cached token
	t.mu.RLock()
	if t.token != "" && time.Now().Add(5*time.Minute).Before(t.exp) {
		token := t.token
		t.mu.RUnlock()
		return token, nil
	}
	t.mu.RUnlock()

	// Slow path: write lock to refresh
	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring write lock
	if t.token != "" && time.Now().Add(5*time.Minute).Before(t.exp) {
		return t.token, nil
	}

	jwtToken, err := t.generateJWT()
	if err != nil {
		return "", fmt.Errorf("failed to generate JWT: %w", err)
	}

	tok, err := t.fetchInstallationToken(ctx, jwtToken)
	if err != nil {
		return "", err
	}

	t.token = tok.Token
	t.exp = tok.ExpiresAt
	return t.token, nil
}

// generateJWT creates a signed JWT for GitHub App authentication using RS256.
func (t *Transport) generateJWT() (string, error) {
	now := time.Now()

	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}
	payload := map[string]any{
		"iat": now.Add(-30 * time.Second).Unix(), // allow 30s clock drift
		"exp": now.Add(9 * time.Minute).Unix(),   // well within GitHub's 10-minute maximum
		"iss": fmt.Sprintf("%d", t.config.AppID),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWT header: %w", err)
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWT payload: %w", err)
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := headerB64 + "." + payloadB64
	hash := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, t.key, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + sigB64, nil
}

func (t *Transport) fetchInstallationToken(ctx context.Context, jwtToken string) (*installationToken, error) {
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", t.config.BaseURL, t.config.InstallationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create installation token (status %d): %s", resp.StatusCode, body)
	}

	var tok installationToken
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, fmt.Errorf("failed to decode installation token response: %w", err)
	}
	return &tok, nil
}

func parsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("expected RSA private key, got %T", key)
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}
