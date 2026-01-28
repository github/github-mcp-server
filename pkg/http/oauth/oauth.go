// Package oauth provides OAuth 2.0 Protected Resource Metadata (RFC 9728) support
// for the GitHub MCP Server HTTP mode.
package oauth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const (
	// OAuthProtectedResourcePrefix is the well-known path prefix for OAuth protected resource metadata.
	OAuthProtectedResourcePrefix = "/.well-known/oauth-protected-resource"

	// DefaultAuthorizationServer is GitHub's OAuth authorization server.
	DefaultAuthorizationServer = "https://github.com/login/oauth"
)

// SupportedScopes lists all OAuth scopes that may be required by MCP tools.
var SupportedScopes = []string{
	"repo",
	"read:org",
	"read:user",
	"user:email",
	"read:packages",
	"write:packages",
	"read:project",
	"project",
	"gist",
	"notifications",
	"workflow",
	"codespace",
}

// Config holds the OAuth configuration for the MCP server.
type Config struct {
	// BaseURL is the publicly accessible URL where this server is hosted.
	// This is used to construct the OAuth resource URL.
	BaseURL string

	// AuthorizationServer is the OAuth authorization server URL.
	// Defaults to GitHub's OAuth server if not specified.
	AuthorizationServer string

	// ResourcePath is the resource path suffix (e.g., "/mcp").
	// If empty, defaults to "/"
	ResourcePath string
}

// ProtectedResourceData contains the data needed to build an OAuth protected resource response.
type ProtectedResourceData struct {
	ResourceURL         string
	AuthorizationServer string
}

// AuthHandler handles OAuth-related HTTP endpoints.
type AuthHandler struct {
	cfg *Config
}

// NewAuthHandler creates a new OAuth auth handler.
func NewAuthHandler(cfg *Config) (*AuthHandler, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	// Default authorization server to GitHub
	if cfg.AuthorizationServer == "" {
		cfg.AuthorizationServer = DefaultAuthorizationServer
	}

	return &AuthHandler{
		cfg: cfg,
	}, nil
}

// routePatterns defines the route patterns for OAuth protected resource metadata.
var routePatterns = []string{
	"",          // Root: /.well-known/oauth-protected-resource
	"/readonly", // Read-only mode
	"/insiders", // Insiders mode
	"/x/{toolset}",
	"/x/{toolset}/readonly",
}

// RegisterRoutes registers the OAuth protected resource metadata routes.
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	for _, pattern := range routePatterns {
		for _, route := range h.routesForPattern(pattern) {
			path := OAuthProtectedResourcePrefix + route

			// Build metadata for this specific resource path
			metadata := h.buildMetadata(route)
			r.Handle(path, auth.ProtectedResourceMetadataHandler(metadata))
		}
	}
}

func (h *AuthHandler) buildMetadata(resourcePath string) *oauthex.ProtectedResourceMetadata {
	baseURL := strings.TrimSuffix(h.cfg.BaseURL, "/")
	resourceURL := baseURL
	if resourcePath != "" && resourcePath != "/" {
		resourceURL = baseURL + resourcePath
	}

	return &oauthex.ProtectedResourceMetadata{
		Resource:               resourceURL,
		AuthorizationServers:   []string{h.cfg.AuthorizationServer},
		ResourceName:           "GitHub MCP Server",
		ScopesSupported:        SupportedScopes,
		BearerMethodsSupported: []string{"header"},
	}
}

// routesForPattern generates route variants for a given pattern.
// GitHub strips the /mcp prefix before forwarding, so we register both variants:
// - With /mcp prefix: for direct access or when GitHub doesn't strip
// - Without /mcp prefix: for when GitHub has stripped the prefix
func (h *AuthHandler) routesForPattern(pattern string) []string {
	return []string{
		pattern,
		"/mcp" + pattern,
		pattern + "/",
		"/mcp" + pattern + "/",
	}
}

// GetEffectiveResourcePath returns the resource path for OAuth protected resource URLs.
// It checks for the X-GitHub-Original-Path header set by GitHub, which contains
// the exact path the client requested before the /mcp prefix was stripped.
// If the header is not present, it falls back to
// restoring the /mcp prefix.
func GetEffectiveResourcePath(r *http.Request) string {
	// Check for the original path header from GitHub (preferred method)
	if originalPath := r.Header.Get(headers.OriginalPathHeader); originalPath != "" {
		return originalPath
	}

	// Fallback: GitHub strips /mcp prefix, so we need to restore it for the external URL
	if r.URL.Path == "/" {
		return "/mcp"
	}
	return "/mcp" + r.URL.Path
}

// GetProtectedResourceData builds the OAuth protected resource data for a request.
func (h *AuthHandler) GetProtectedResourceData(r *http.Request, resourcePath string) (*ProtectedResourceData, error) {
	host, scheme := GetEffectiveHostAndScheme(r, h.cfg)

	// Build the base URL
	baseURL := fmt.Sprintf("%s://%s", scheme, host)
	if h.cfg.BaseURL != "" {
		baseURL = strings.TrimSuffix(h.cfg.BaseURL, "/")
	}

	// Build the resource URL using url.JoinPath for proper path handling
	var resourceURL string
	var err error
	if resourcePath == "/" {
		resourceURL = baseURL + "/"
	} else {
		resourceURL, err = url.JoinPath(baseURL, resourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to build resource URL: %w", err)
		}
	}

	return &ProtectedResourceData{
		ResourceURL:         resourceURL,
		AuthorizationServer: h.cfg.AuthorizationServer,
	}, nil
}

// GetEffectiveHostAndScheme returns the effective host and scheme for a request.
// It checks X-Forwarded-Host and X-Forwarded-Proto headers first (set by proxies),
// then falls back to the request's Host and TLS state.
func GetEffectiveHostAndScheme(r *http.Request, cfg *Config) (host, scheme string) { //nolint:revive // parameters are required by http.oauth.BuildResourceMetadataURL signature
	// Check for forwarded headers first (typically set by reverse proxies)
	if forwardedHost := r.Header.Get(headers.ForwardedHostHeader); forwardedHost != "" {
		host = forwardedHost
	} else {
		host = r.Host
	}

	// Determine scheme
	switch {
	case r.Header.Get(headers.ForwardedProtoHeader) != "":
		scheme = strings.ToLower(r.Header.Get(headers.ForwardedProtoHeader))
	case r.TLS != nil:
		scheme = "https"
	default:
		// Default to HTTPS in production scenarios
		scheme = "https"
	}

	return host, scheme
}

// BuildResourceMetadataURL constructs the full URL to the OAuth protected resource metadata endpoint.
func BuildResourceMetadataURL(r *http.Request, cfg *Config, resourcePath string) string {
	host, scheme := GetEffectiveHostAndScheme(r, cfg)

	if cfg != nil && cfg.BaseURL != "" {
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
		return baseURL + OAuthProtectedResourcePrefix + "/" + strings.TrimPrefix(resourcePath, "/")
	}

	path := OAuthProtectedResourcePrefix
	if resourcePath != "" && resourcePath != "/" {
		path = path + "/" + strings.TrimPrefix(resourcePath, "/")
	}

	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}
