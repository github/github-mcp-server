// Package githubapp implements non-interactive GitHub App server-to-server
// (s2s) authentication for the stdio server.
//
// Unlike the user-to-server OAuth flows in internal/oauth, this requires no
// human: no browser, no device code, no elicitation. It signs a short-lived
// JWT with the app's private key, exchanges it for an installation access
// token, and transparently refreshes that token before it expires. That makes
// it suitable for headless deployments — CI, Kubernetes, background agents.
//
// It only depends on the standard library and golang.org/x/oauth2.
//
// # Security
//
// This mode injects a long-lived, high-privilege credential (the app private
// key) into an environment shared with an AI agent, and the installation
// tokens it mints can act across every repository the app is installed on. It
// was added by popular demand for non-interactive deployments, but exposing
// credentials to agents — especially in the cloud — is dangerous and is not
// recommended without an independent security review. See
// docs/github-app-auth.md for the full guidance and least-privilege advice.
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
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

const (
	// jwtLifetime is how long minted app JWTs are valid. GitHub rejects app JWTs
	// whose exp is more than 10 minutes in the future; 9 minutes leaves headroom.
	jwtLifetime = 9 * time.Minute

	// clockSkew backdates the JWT iat to tolerate small clock differences
	// between this host and GitHub, which would otherwise reject the JWT.
	clockSkew = 60 * time.Second

	// refreshBuffer refreshes installation tokens this long before their real
	// expiry so an in-flight request never races the expiry boundary.
	refreshBuffer = 5 * time.Minute

	// httpTimeout bounds each call to the installation token endpoint so a
	// stalled GitHub API cannot block a tool call indefinitely.
	httpTimeout = 30 * time.Second
)

// Config describes a GitHub App installation used for server-to-server auth.
type Config struct {
	// AppID is the GitHub App's App ID or client ID; it becomes the JWT issuer
	// (iss). Both forms are accepted by GitHub.
	AppID string

	// InstallationID identifies the installation whose access token is minted.
	InstallationID string

	// PrivateKey signs the app JWT (RS256). Parse one with ParsePrivateKey.
	PrivateKey *rsa.PrivateKey

	// BaseRESTURL is the REST API base, e.g. https://api.github.com/ for
	// github.com or https://HOST/api/v3/ for GitHub Enterprise Server.
	BaseRESTURL string
}

// Validate reports whether the configuration is complete enough to mint tokens.
func (c Config) Validate() error {
	switch {
	case c.AppID == "":
		return errors.New("GitHub App ID is required (GITHUB_APP_ID)")
	case c.InstallationID == "":
		return errors.New("GitHub App installation ID is required (GITHUB_APP_INSTALLATION_ID)")
	case c.PrivateKey == nil:
		return errors.New("GitHub App private key is required (GITHUB_APP_PRIVATE_KEY_PATH or GITHUB_APP_PRIVATE_KEY)")
	case c.BaseRESTURL == "":
		return errors.New("GitHub App REST base URL is required")
	}
	return nil
}

// ParsePrivateKey parses a PEM-encoded RSA private key in PKCS#1 ("RSA PRIVATE
// KEY") or PKCS#8 ("PRIVATE KEY") form — the two formats GitHub issues for app
// keys.
func ParsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
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

// mintJWT builds and signs a short-lived app JWT (RS256) for the configured
// app, as required by the installation token endpoint.
func (c Config) mintJWT(now time.Time) (string, error) {
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]any{
		"iat": now.Add(-clockSkew).Unix(),
		"exp": now.Add(jwtLifetime).Unix(),
		"iss": c.AppID,
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
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.PrivateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

// installationTokenSource is an oauth2.TokenSource that mints GitHub App
// installation access tokens. It performs no caching itself; wrap it in
// oauth2.ReuseTokenSource (see NewProvider) for that.
type installationTokenSource struct {
	cfg        Config
	httpClient *http.Client
}

func newInstallationTokenSource(cfg Config, httpClient *http.Client) *installationTokenSource {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: httpTimeout}
	}
	return &installationTokenSource{cfg: cfg, httpClient: httpClient}
}

// Token mints a fresh installation access token. The returned token's Expiry is
// set refreshBuffer before the real expiry so callers refresh early.
func (s *installationTokenSource) Token() (*oauth2.Token, error) {
	jwt, err := s.cfg.mintJWT(time.Now())
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
		// The error body is GitHub's JSON message (never the token); include a
		// bounded snippet to make misconfiguration diagnosable.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
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

	expiry := body.ExpiresAt
	if !expiry.IsZero() {
		expiry = expiry.Add(-refreshBuffer)
	}
	return &oauth2.Token{
		AccessToken: body.Token,
		TokenType:   "token",
		Expiry:      expiry,
	}, nil
}

// Provider supplies GitHub App installation access tokens, caching and
// refreshing them transparently. Its AccessToken method mirrors
// oauth.Manager.AccessToken so it can back BearerAuthTransport.TokenProvider.
type Provider struct {
	source oauth2.TokenSource
	logger *slog.Logger

	mu        sync.Mutex
	errLogged bool
}

// NewProvider validates cfg and returns a Provider that mints and refreshes
// installation tokens. A nil logger logs to stderr.
func NewProvider(cfg Config, logger *slog.Logger) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	// ReuseTokenSource caches the token and only calls the underlying source
	// once the cached token is expired. Because Token() backdates Expiry by
	// refreshBuffer, that refresh happens ~5 minutes before the real expiry.
	source := oauth2.ReuseTokenSource(nil, newInstallationTokenSource(cfg, nil))
	return &Provider{source: source, logger: logger}, nil
}

// AccessToken returns a currently valid installation access token, refreshing
// it if needed, or "" if a token could not be obtained. A fetch failure is
// logged once (until the next success) so a misconfiguration is visible without
// flooding the log on every tool call.
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

// HasToken reports whether a valid token can currently be obtained.
func (p *Provider) HasToken() bool {
	return p.AccessToken() != ""
}
