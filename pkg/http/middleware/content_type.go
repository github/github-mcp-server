package middleware

import (
	"mime"
	"net/http"

	"github.com/github/github-mcp-server/pkg/http/headers"
)

// NormalizeContentType strips MIME parameters from the Content-Type header so
// "application/json; charset=utf-8" is treated as "application/json".
func NormalizeContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get(headers.ContentTypeHeader); ct != "" {
			mediaType, _, err := mime.ParseMediaType(ct)
			if err == nil && mediaType != ct {
				r = r.Clone(r.Context())
				r.Header.Set(headers.ContentTypeHeader, mediaType)
			}
		}
		next.ServeHTTP(w, r)
	})
}
