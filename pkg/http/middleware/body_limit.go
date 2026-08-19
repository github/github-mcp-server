package middleware

import (
	"errors"
	"net/http"
)

// DefaultMaxRequestBodyBytes bounds the size of HTTP request bodies accepted
// by the MCP endpoints when no explicit limit is configured.
const DefaultMaxRequestBodyBytes int64 = 10 << 20 // 10 MiB

// WithMaxBodySize returns middleware that bounds the size of the request
// body. It must be registered before any middleware that reads or buffers
// the body (e.g. WithMCPParse, WithScopeChallenge) so that an oversized
// payload is rejected before it is ever fully buffered in memory, rather than
// relying on a size guard applied later by the MCP SDK or a downstream
// handler.
//
// When Content-Length is known and already exceeds maxBytes, the request is
// rejected immediately without touching the body. Otherwise the body is
// wrapped with http.MaxBytesReader, so any subsequent read (including
// chunked or unknown-length bodies) fails with a *http.MaxBytesError once
// maxBytes have been consumed.
func WithMaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				writeRequestTooLarge(w)
				return
			}

			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeRequestTooLarge writes the standard "request body too large" response.
// Every middleware that reads the request body should use this so oversized
// requests get a consistent, clear response regardless of which layer
// detects the overflow.
func writeRequestTooLarge(w http.ResponseWriter) {
	http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
}

// isMaxBytesError reports whether err resulted from a body exceeding the
// limit applied by WithMaxBodySize, as opposed to some other read failure.
func isMaxBytesError(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
