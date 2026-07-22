// Package githubapp provides GitHub App installation access tokens.
package githubapp

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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

const (
	jwtLifetime   = 9 * time.Minute
	clockSkew     = time.Minute
	refreshBuffer = 5 * time.Minute
	httpTimeout   = 30 * time.Second
)

// Config describes a GitHub App installation used for server-to-server auth.
type Config struct {
	// AppID is used as the JWT issuer. GitHub accepts an app ID or client ID.
	AppID string

	// InstallationID identifies the installation whose access token is minted.
	InstallationID string

	// PrivateKeyPEM is the RSA key used to sign app JWTs.
	PrivateKeyPEM []byte

	// BaseRESTURL is the REST API base, e.g. https://api.github.com/ for
	// github.com or https://HOST/api/v3/ for GitHub Enterprise Server.
	BaseRESTURL string
}

func (c Config) validate() error {
	switch {
	case c.AppID == "":
		return errors.New("GitHub App ID or client ID is required (GITHUB_APP_ID)")
	case c.InstallationID == "":
		return errors.New("GitHub App installation ID is required (GITHUB_APP_INSTALLATION_ID)")
	case len(c.PrivateKeyPEM) == 0:
		return errors.New("GitHub App private key is required (GITHUB_APP_PRIVATE_KEY_PATH or GITHUB_APP_PRIVATE_KEY)")
	case c.BaseRESTURL == "":
		return errors.New("GitHub App REST base URL is required")
	}
	return nil
}

func parsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found in private key")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key (want PKCS#1 or PKCS#8 RSA): %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is %T, want an RSA key", parsed)
	}
	return key, nil
}

func mintJWT(appID string, privateKey *rsa.PrivateKey, now time.Time) (string, error) {
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{
		"iat": now.Add(-clockSkew).Unix(),
		"exp": now.Add(jwtLifetime).Unix(),
		"iss": appID,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("encoding JWT header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encoding JWT claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)

	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

type installationTokenSource struct {
	cfg        Config
	privateKey *rsa.PrivateKey
	httpClient *http.Client
}

func newInstallationTokenSource(cfg Config, privateKey *rsa.PrivateKey, httpClient *http.Client) *installationTokenSource {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: httpTimeout}
	}
	return &installationTokenSource{cfg: cfg, privateKey: privateKey, httpClient: httpClient}
}

func (s *installationTokenSource) Token() (*oauth2.Token, error) {
	jwt, err := mintJWT(s.cfg.AppID, s.privateKey, time.Now())
	if err != nil {
		return nil, err
	}

	endpoint, err := url.JoinPath(s.cfg.BaseRESTURL, "app", "installations", s.cfg.InstallationID, "access_tokens")
	if err != nil {
		return nil, fmt.Errorf("building installation token URL: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating installation token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting installation token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		snippet, readErr := io.ReadAll(io.LimitReader(resp.Body, 512))
		if readErr != nil {
			return nil, fmt.Errorf("installation token request failed: %s (reading response: %w)", resp.Status, readErr)
		}
		return nil, fmt.Errorf("installation token request failed: %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}

	var body struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding installation token response: %w", err)
	}
	if body.Token == "" {
		return nil, errors.New("installation token response did not contain a token")
	}
	if body.ExpiresAt.IsZero() {
		return nil, errors.New("installation token response did not contain an expiry")
	}
	return &oauth2.Token{
		AccessToken: body.Token,
		TokenType:   "token",
		Expiry:      body.ExpiresAt.Add(-refreshBuffer),
	}, nil
}

// Provider caches and refreshes GitHub App installation access tokens.
type Provider struct {
	source oauth2.TokenSource
	logger *slog.Logger

	mu        sync.Mutex
	errLogged bool
}

func NewProvider(cfg Config, logger *slog.Logger) (*Provider, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	privateKey, err := parsePrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub App private key: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	source := oauth2.ReuseTokenSource(nil, newInstallationTokenSource(cfg, privateKey, nil))
	return &Provider{source: source, logger: logger}, nil
}

// AccessToken returns a cached token or refreshes it before expiry.
func (p *Provider) AccessToken() string {
	tok, err := p.source.Token()
	if err != nil {
		p.mu.Lock()
		if !p.errLogged {
			p.errLogged = true
			p.logger.Error("failed to obtain GitHub App installation token", "error", err)
		}
		p.mu.Unlock()
		return ""
	}
	p.mu.Lock()
	p.errLogged = false
	p.mu.Unlock()
	return tok.AccessToken
}
