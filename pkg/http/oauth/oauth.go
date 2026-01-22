// Package oauth provides OAuth 2.0 Protected Resource Metadata (RFC 9728) support
// for the GitHub MCP Server HTTP mode.
package oauth

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"net/http"
	"strings"
	"text/template"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/go-chi/chi/v5"
)

const (
	// OAuthProtectedResourcePrefix is the well-known path prefix for OAuth protected resource metadata.
	OAuthProtectedResourcePrefix = "/.well-known/oauth-protected-resource"

	// DefaultAuthorizationServer is GitHub's OAuth authorization server.
	DefaultAuthorizationServer = "https://github.com/login/oauth"
)

//go:embed protected_resource.json.tmpl
var protectedResourceTemplate []byte

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
	// Example: "https://mcp.example.com"
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
	cfg                       *Config
	protectedResourceTemplate *template.Template
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

	tmpl, err := template.New("protected-resource").Parse(string(protectedResourceTemplate))
	if err != nil {
		return nil, fmt.Errorf("failed to parse protected resource template: %w", err)
	}

	return &AuthHandler{
		cfg:                       cfg,
		protectedResourceTemplate: tmpl,
	}, nil
}

// routePatterns defines the route patterns for OAuth protected resource metadata.
var routePatterns = []string{
	"",          // Root: /.well-known/oauth-protected-resource
	"/readonly", // Read-only mode
	"/x/{toolset}",
	"/x/{toolset}/readonly",
}

// RegisterRoutes registers the OAuth protected resource metadata routes.
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	for _, pattern := range routePatterns {
		for _, route := range h.routesForPattern(pattern) {
			path := OAuthProtectedResourcePrefix + route
			r.Get(path, h.handleProtectedResource)
			r.Options(path, h.handleProtectedResource) // CORS support
		}
	}
}

// routesForPattern generates route variants for a given pattern.
func (h *AuthHandler) routesForPattern(pattern string) []string {
	routes := []string{
		pattern,
		pattern + "/",
		pattern + "/mcp",
		pattern + "/mcp/",
	}
	return routes
}

// handleProtectedResource handles requests for OAuth protected resource metadata.
func (h *AuthHandler) handleProtectedResource(w http.ResponseWriter, r *http.Request) {
	// Extract the resource path from the URL
	resourcePath := strings.TrimPrefix(r.URL.Path, OAuthProtectedResourcePrefix)
	if resourcePath == "" || resourcePath == "/" {
		resourcePath = "/"
	} else {
		resourcePath = strings.TrimPrefix(resourcePath, "/")
	}

	data, err := h.GetProtectedResourceData(r, html.EscapeString(resourcePath))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var buf bytes.Buffer
	if err := h.protectedResourceTemplate.Execute(&buf, data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// GetProtectedResourceData builds the OAuth protected resource data for a request.
func (h *AuthHandler) GetProtectedResourceData(r *http.Request, resourcePath string) (*ProtectedResourceData, error) {
	host, scheme := GetEffectiveHostAndScheme(r, h.cfg)

	// Build the resource URL
	var resourceURL string
	if h.cfg.BaseURL != "" {
		// Use configured base URL
		baseURL := strings.TrimSuffix(h.cfg.BaseURL, "/")
		if resourcePath == "/" {
			resourceURL = baseURL + "/"
		} else {
			resourceURL = baseURL + "/" + resourcePath
		}
	} else {
		// Derive from request
		if resourcePath == "/" {
			resourceURL = fmt.Sprintf("%s://%s/", scheme, host)
		} else {
			resourceURL = fmt.Sprintf("%s://%s/%s", scheme, host, resourcePath)
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
