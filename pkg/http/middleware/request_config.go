package middleware

import (
	"net/http"
	"slices"
	"strings"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/headers"
)

// WithRequestConfig is a middleware that extracts MCP-related headers and sets them in the request context.
// This includes readonly mode, toolsets, tools, lockdown mode, insiders mode, and feature flags.
func WithRequestConfig(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Readonly mode
		if relaxedParseBool(r.Header.Get(headers.MCPReadOnlyHeader)) {
			ctx = ghcontext.WithReadonly(ctx, true)
		}

		// Toolsets
		if toolsets := headers.ParseCommaSeparated(r.Header.Get(headers.MCPToolsetsHeader)); len(toolsets) > 0 {
			ctx = ghcontext.WithToolsets(ctx, toolsets)
		}

		// Tools
		if tools := headers.ParseCommaSeparated(r.Header.Get(headers.MCPToolsHeader)); len(tools) > 0 {
			ctx = ghcontext.WithTools(ctx, tools)
		}

		// Lockdown mode
		if relaxedParseBool(r.Header.Get(headers.MCPLockdownHeader)) {
			ctx = ghcontext.WithLockdownMode(ctx, true)
		}

		// Disallowed tools
		if disallowedTools := headers.ParseCommaSeparated(r.Header.Get(headers.MCPDisallowedToolsHeader)); len(disallowedTools) > 0 {
			ctx = ghcontext.WithDisallowedTools(ctx, disallowedTools)
		}

		// Insiders mode
		if relaxedParseBool(r.Header.Get(headers.MCPInsidersHeader)) {
			ctx = ghcontext.WithInsidersMode(ctx, true)
		}

		// Feature flags
		if features := headers.ParseCommaSeparated(r.Header.Get(headers.MCPFeaturesHeader)); len(features) > 0 {
			ctx = ghcontext.WithHeaderFeatures(ctx, features)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// relaxedParseBool parses a string into a boolean value, treating various
// common false values or empty strings as false, and everything else as true.
// It is case-insensitive and trims whitespace.
func relaxedParseBool(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	falseValues := []string{"", "false", "0", "no", "off", "n", "f"}
	return !slices.Contains(falseValues, s)
}
