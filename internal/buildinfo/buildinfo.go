// Package buildinfo contains variables that are set at build time via ldflags.
// These allow official releases to ship default OAuth credentials so users can
// log in without configuring their own OAuth app. The values are public in
// practice (security relies on PKCE, not on the client secret), but are kept out
// of source and injected at build time.
//
// Example:
//
//	go build -ldflags="-X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientID=xxx"
package buildinfo

// OAuthClientID is the default OAuth client ID, set at build time. Empty in
// local/dev builds.
var OAuthClientID string

// OAuthClientSecret is the default OAuth client secret, set at build time. For
// public OAuth clients it is not truly secret per OAuth 2.1 — PKCE provides the
// security — but it is still injected at build time rather than committed.
var OAuthClientSecret string
