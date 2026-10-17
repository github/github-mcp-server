package middleware

import (
	"errors"
	"net/http"
)

// DefaultMaxRequestBodyBytes bounds the total HTTP request, not just the tool
// payload within it. It sits modestly above the MCP SDK's own default to leave
// room for JSON-RPC and tool-call envelope overhead; because it is the larger
// of the two, callers must also pass it to the SDK or the SDK would cap it.
const DefaultMaxRequestBodyBytes int64 = 5 << 20 // 5 MiB

// WithMaxBodySize bounds the size of the request body. It must be registered
// before any middleware that reads or buffers the body (WithMCPParse,
// WithScopeChallenge), so an oversized payload is rejected before it is
// buffered in memory rather than by a later guard in the MCP SDK.
//
// A body of unknown length (chunked, HTTP/2) cannot be rejected upfront, so
// the limit is instead enforced on read and surfaces as a *http.MaxBytesError
// to whichever middleware reads the body first.
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

func writeRequestTooLarge(w http.ResponseWriter) {
	http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
}

func isMaxBytesError(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
