package utils //nolint:revive // utils is the established package name in this repository

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

type githubAppTokenSource struct {
	appID          int64
	installationID int64
	privateKey     *rsa.PrivateKey
	baseURL        string
	client         *resty.Client
	mu             sync.Mutex
	cachedToken    *oauth2.Token
}

// NewGitHubAppTokenSource creates a new token source for GitHub App installation access tokens.
// It parses the privateKey PEM bytes or reads them from privateKeyPath, and normalizes the host for Enterprise Server.
func NewGitHubAppTokenSource(appID, installationID int64, privateKey []byte, privateKeyPath string, host string) (oauth2.TokenSource, error) {
	var keyBytes []byte
	var err error
	switch {
	case len(privateKey) > 0:
		keyBytes = privateKey
	case privateKeyPath != "":
		keyBytes, err = os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
	default:
		return nil, fmt.Errorf("either app-private-key or app-private-key-path must be provided")
	}

	parsedKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid app-private-key: %w", err)
	}

	// Normalize host to baseURL (independent from internal/oauth to avoid circular dependency)
	baseURL := "https://api.github.com"
	if host != "" && host != "github.com" && host != "api.github.com" && host != "https://github.com" && host != "https://api.github.com" {
		baseURL = host
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = "https://" + baseURL
		}
		baseURL = strings.TrimSuffix(baseURL, "/")

		// Strip 'api.' prefix if present to normalize domain (e.g. api.github.example.com -> github.example.com)
		baseURL = strings.Replace(baseURL, "https://api.", "https://", 1)
		baseURL = strings.Replace(baseURL, "http://api.", "http://", 1)

		// Normalize Enterprise Server API endpoint to end with /api/v3
		if !strings.HasSuffix(baseURL, "/api/v3") {
			baseURL += "/api/v3"
		}
	}

	// Initialize resty client with 15s timeout and automatic 3-retry for 5xx/429/connection issues
	restyClient := resty.New().
		SetTimeout(15 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(3 * time.Second).
		AddRetryCondition(
			func(r *resty.Response, err error) bool {
				// Retry on network errors, HTTP 5xx Server Errors, or HTTP 429 Too Many Requests
				return err != nil || r.StatusCode() >= 500 || r.StatusCode() == http.StatusTooManyRequests
			},
		)

	return &githubAppTokenSource{
		appID:          appID,
		installationID: installationID,
		privateKey:     parsedKey,
		baseURL:        baseURL,
		client:         restyClient,
	}, nil
}

// Token returns a cached token if valid, otherwise requests a new one from GitHub.
func (ts *githubAppTokenSource) Token() (*oauth2.Token, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// If token is still valid (with 1 minute buffer to be safe), return it
	if ts.cachedToken != nil && ts.cachedToken.Expiry.After(time.Now().Add(1*time.Minute)) {
		return ts.cachedToken, nil
	}

	jwtToken, err := ts.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("generating app JWT: %w", err)
	}

	token, err := ts.requestInstallationToken(jwtToken)
	if err != nil {
		return nil, fmt.Errorf("requesting installation token: %w", err)
	}

	ts.cachedToken = token
	return token, nil
}

func (ts *githubAppTokenSource) generateJWT() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    strconv.FormatInt(ts.appID, 10),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)), // clock drift skew
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(ts.privateKey)
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}

	return signedToken, nil
}

func (ts *githubAppTokenSource) requestInstallationToken(jwtToken string) (*oauth2.Token, error) {
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", ts.baseURL, ts.installationID)

	var result struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}

	resp, err := ts.client.R().
		SetContext(context.Background()).
		SetHeader("Authorization", "Bearer "+jwtToken).
		SetHeader("Accept", "application/vnd.github+json").
		SetHeader("User-Agent", "github-mcp-server").
		SetBody(map[string]any{}).
		SetResult(&result).
		Post(url)

	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode(), resp.String())
	}

	expiry, err := time.Parse(time.RFC3339, result.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("parsing token expiry: %w", err)
	}

	return &oauth2.Token{
		AccessToken: result.Token,
		TokenType:   "Bearer",
		Expiry:      expiry,
	}, nil
}
