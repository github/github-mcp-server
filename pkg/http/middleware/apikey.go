package middleware

import (
	"crypto/subtle"
	"net/http"
)

// WithAPIKey returns middleware that validates the X-API-Key header against the
// configured key. It uses constant-time comparison to prevent timing attacks.
// If the key is empty, the middleware is a no-op (all requests pass through).
func WithAPIKey(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("X-API-Key")
			if provided == "" {
				http.Error(w, "missing API key", http.StatusUnauthorized)
				return
			}
			if subtle.ConstantTimeCompare([]byte(provided), []byte(key)) != 1 {
				http.Error(w, "invalid API key", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
