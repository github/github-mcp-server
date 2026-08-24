package middleware

import (
	"net/http"
	"slices"
	"strings"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/headers"
)

// queryParamFeatures is the URL query parameter that carries feature flags,
// mirroring the X-MCP-Features header. It exists so clients that cannot set
// custom headers on the MCP connection — hosted IDEs, agent platforms, or
// harnesses that compose the server URL on the user's behalf (see #3145) —
// can still opt into flagged tools.
const queryParamFeatures = "features"

// WithRequestConfig is a middleware that extracts MCP-related headers and sets them in the request context.
// This includes readonly mode, toolsets, tools, lockdown mode, insiders mode, and feature flags.
// Feature flags may also arrive via the `features` URL query parameter; when
// both are present the query parameter wins, matching how the toolset path
// segments take precedence over their headers.
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

		// Excluded tools
		if excludeTools := headers.ParseCommaSeparated(r.Header.Get(headers.MCPExcludeToolsHeader)); len(excludeTools) > 0 {
			ctx = ghcontext.WithExcludeTools(ctx, excludeTools)
		}

		// Insiders mode
		if relaxedParseBool(r.Header.Get(headers.MCPInsidersHeader)) {
			ctx = ghcontext.WithInsidersMode(ctx, true)
		}

		// Feature flags: presence-based selection. The URL query parameter and
		// the X-MCP-Features header are separate channels — whichever is
		// present is used as-is, and they are never combined. When both are
		// present the query parameter wins, so a client composing the server
		// URL can always express its intent even when it cannot control
		// headers. Unknown flags are dropped later by ResolveFeatureFlags
		// against AllowedFeatureFlags, so neither channel is privileged.
		queryFeatures, hasQuery := r.URL.Query()[queryParamFeatures]
		headerFeatures := r.Header.Get(headers.MCPFeaturesHeader)
		switch {
		case hasQuery && strings.TrimSpace(queryFeatures[0]) != "":
			ctx = ghcontext.WithHeaderFeatures(ctx, headers.ParseCommaSeparated(queryFeatures[0]))
		case headerFeatures != "":
			ctx = ghcontext.WithHeaderFeatures(ctx, headers.ParseCommaSeparated(headerFeatures))
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
