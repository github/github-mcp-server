// Package buildinfo contains variables that are set at build time via ldflags.
// These allow official releases to include default OAuth credentials without
// requiring end-user configuration.
//
// Example ldflags usage:
//
//	go build -ldflags="-X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientID=xxx"
package buildinfo

// OAuthClientID is the default OAuth client ID, set at build time.
var OAuthClientID string

// OAuthClientSecret is the default OAuth client secret, set at build time.
// Note: For public OAuth clients (native apps), the client secret is not
// truly secret per OAuth 2.1 — security relies on PKCE, not the secret.
var OAuthClientSecret string
