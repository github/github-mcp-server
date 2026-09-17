package transport

import (
	"net/http"
)

// RateLimitTransport wraps an HTTP transport to intercept and handle
// GitHub API rate limit responses.
type RateLimitTransport struct {
	Transport http.RoundTripper
}

func (t *RateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Transport == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return t.Transport.RoundTrip(req)
}
