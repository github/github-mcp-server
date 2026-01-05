package scopes

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthScopesHeader is the HTTP response header containing the token's OAuth scopes.
const OAuthScopesHeader = "X-OAuth-Scopes"

// DefaultFetchTimeout is the default timeout for scope fetching requests.
const DefaultFetchTimeout = 10 * time.Second

// FetcherOptions configures the scope fetcher.
type FetcherOptions struct {
	// HTTPClient is the HTTP client to use for requests.
	// If nil, a default client with DefaultFetchTimeout is used.
	HTTPClient *http.Client

	// APIHost is the GitHub API host (e.g., "https://api.github.com").
	// Defaults to "https://api.github.com" if empty.
	APIHost string
}

// Fetcher retrieves token scopes from GitHub's API.
// It uses an HTTP HEAD request to minimize bandwidth since we only need headers.
type Fetcher struct {
	client  *http.Client
	apiHost string
}

// NewFetcher creates a new scope fetcher with the given options.
func NewFetcher(opts FetcherOptions) *Fetcher {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: DefaultFetchTimeout}
	}

	apiHost := opts.APIHost
	if apiHost == "" {
		apiHost = "https://api.github.com"
	}

	return &Fetcher{
		client:  client,
		apiHost: apiHost,
	}
}

// FetchTokenScopes retrieves the OAuth scopes for a token by making an HTTP HEAD
// request to the GitHub API and parsing the X-OAuth-Scopes header.
//
// Returns:
//   - []string: List of scopes (empty if no scopes or fine-grained PAT)
//   - error: Any HTTP or parsing error
//
// Note: Fine-grained PATs don't return the X-OAuth-Scopes header, so an empty
// slice is returned for those tokens.
func (f *Fetcher) FetchTokenScopes(ctx context.Context, token string) ([]string, error) {
	// Use a lightweight endpoint that requires authentication
	endpoint, err := url.JoinPath(f.apiHost, "/")
	if err != nil {
		return nil, fmt.Errorf("failed to construct API URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch scopes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("invalid or expired token")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return ParseScopeHeader(resp.Header.Get(OAuthScopesHeader)), nil
}

// ParseScopeHeader parses the X-OAuth-Scopes header value into a list of scopes.
// The header contains comma-separated scope names.
// Returns an empty slice for empty or missing header.
func ParseScopeHeader(header string) []string {
	if header == "" {
		return []string{}
	}

	parts := strings.Split(header, ",")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		scope := strings.TrimSpace(part)
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

// FetchTokenScopes is a convenience function that creates a default fetcher
// and fetches the token scopes.
func FetchTokenScopes(ctx context.Context, token string) ([]string, error) {
	return NewFetcher(FetcherOptions{}).FetchTokenScopes(ctx, token)
}

// FetchTokenScopesWithHost is a convenience function that creates a fetcher
// for a specific API host and fetches the token scopes.
func FetchTokenScopesWithHost(ctx context.Context, token, apiHost string) ([]string, error) {
	return NewFetcher(FetcherOptions{APIHost: apiHost}).FetchTokenScopes(ctx, token)
}
