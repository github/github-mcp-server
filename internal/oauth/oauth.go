// Package oauth implements the user-facing OAuth 2.1 login flows the stdio
// server uses to obtain a GitHub token without a pre-provisioned Personal
// Access Token.
//
// It supports both GitHub OAuth Apps and GitHub Apps (user-to-server). The
// only practical difference is that GitHub App user tokens expire and carry a
// refresh token; this package always returns a refreshing [golang.org/x/oauth2.TokenSource]
// so callers never have to special-case the app type.
//
// The package depends only on golang.org/x/oauth2 and the standard library. MCP
// concerns (sessions, elicitation) are abstracted behind the [Prompter]
// interface so the flows can be tested without a live client.
package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
)

// Config describes an OAuth client and the GitHub endpoints it talks to.
type Config struct {
	ClientID     string
	ClientSecret string
	// Scopes requested during authorization. GitHub Apps ignore these (their
	// access is governed by installed permissions); OAuth Apps honor them.
	Scopes []string
	// Endpoint holds the authorization, token, and device endpoints. Build one
	// with [GitHubEndpoint].
	Endpoint oauth2.Endpoint
	// CallbackPort is the fixed local port for the PKCE callback server. Zero
	// requests a random port, which is the secure default for native binaries
	// but cannot be reached through Docker port mapping (see the Manager).
	CallbackPort int
}

// NewGitHubConfig builds a Config for the given GitHub host. An empty host
// targets github.com; otherwise the host may be a GHES or ghe.com hostname,
// with or without a scheme.
func NewGitHubConfig(clientID, clientSecret string, scopes []string, host string, callbackPort int) Config {
	return Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
		Endpoint:     GitHubEndpoint(host),
		CallbackPort: callbackPort,
	}
}

// GitHubEndpoint returns the OAuth authorization, token, and device endpoints
// for a GitHub host. An empty host targets github.com.
func GitHubEndpoint(host string) oauth2.Endpoint {
	base := normalizeHost(host)
	return oauth2.Endpoint{
		AuthURL:       base + "/login/oauth/authorize",
		TokenURL:      base + "/login/oauth/access_token",
		DeviceAuthURL: base + "/login/device/code",
	}
}

// normalizeHost turns a user-supplied host into a scheme+host base URL with no
// trailing slash. The API subdomain is stripped because OAuth endpoints live on
// the web host, not the API host (api.github.com -> github.com).
func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return "https://github.com"
	}

	scheme := "https"
	switch {
	case strings.HasPrefix(host, "https://"):
		host = strings.TrimPrefix(host, "https://")
	case strings.HasPrefix(host, "http://"):
		scheme = "http"
		host = strings.TrimPrefix(host, "http://")
	}

	// Drop any path, query, or fragment; we only need scheme://host.
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}

	host = strings.TrimPrefix(host, "api.")

	return fmt.Sprintf("%s://%s", scheme, host)
}

// randomState returns a cryptographically random URL-safe string used as the
// OAuth state parameter (CSRF protection) and elicitation IDs.
func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
