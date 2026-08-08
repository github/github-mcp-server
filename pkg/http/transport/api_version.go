package transport

import (
	"net/http"

	"github.com/github/github-mcp-server/pkg/http/headers"
)

// APIVersionTransport sets the GitHub REST API version on every request.
type APIVersionTransport struct {
	Transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *APIVersionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	underlying := t.Transport
	if underlying == nil {
		underlying = http.DefaultTransport
	}

	req = req.Clone(req.Context())
	req.Header.Set(headers.GitHubAPIVersionHeader, headers.GitHubAPIVersion)
	return underlying.RoundTrip(req)
}
