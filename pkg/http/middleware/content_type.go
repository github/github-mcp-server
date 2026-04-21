package middleware

import (
	"mime"
	"net/http"
)

// NormalizeContentType is a middleware that normalizes the Content-Type header
// by stripping optional parameters (e.g. charset=utf-8) when the media type
// is "application/json". This works around strict Content-Type matching in
// the Go MCP SDK's StreamableHTTP handler which rejects valid JSON media
// types that include parameters.
//
// Per RFC 8259, JSON text exchanged between systems that are not part of a
// closed ecosystem MUST be encoded using UTF-8, so the charset parameter is
// redundant but MUST be accepted per HTTP semantics.
//
// See: https://github.com/github/github-mcp-server/issues/2333
func NormalizeContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "" {
			mediaType, _, err := mime.ParseMediaType(ct)
			if err == nil && mediaType == "application/json" {
				r.Header.Set("Content-Type", "application/json")
			}
		}
		next.ServeHTTP(w, r)
	})
}
