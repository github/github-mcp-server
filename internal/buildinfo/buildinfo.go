// Package buildinfo contains build-time injected values.
//
// These values are set via -ldflags during the build process.
// For example:
//
//	go build -ldflags="-X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientID=xxx"
//
// The OAuth credentials are used as default values for stdio mode when no
// PAT or explicit OAuth configuration is provided. This enables a "just works"
// experience for most users while still allowing developer overrides.
//
// Note: These credentials are intentionally baked into the binary. While they
// can be reverse-engineered, this provides a barrier against trivial cloning
// and establishes clear provenance for the official GitHub MCP Server.
package buildinfo

// OAuthClientID is the default GitHub OAuth App Client ID.
// Set at build time via -ldflags for official releases.
// Empty string means no default OAuth credentials are available.
var OAuthClientID string

// OAuthClientSecret is the default GitHub OAuth App Client Secret.
// Set at build time via -ldflags for official releases.
// While called a "secret", OAuth client secrets in native apps cannot truly
// be kept secret and are considered public per RFC 8252.
var OAuthClientSecret string

// HasOAuthCredentials returns true if build-time OAuth credentials are available.
func HasOAuthCredentials() bool {
	return OAuthClientID != ""
}
