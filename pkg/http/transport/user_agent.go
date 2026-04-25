package transport

import (
	"net/http"

	"github.com/github/github-mcp-server/pkg/http/headers"
)

type UserAgentTransport struct {
	Transport http.RoundTripper
	Agent     string
}

func (t *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set(headers.UserAgentHeader, t.Agent)
	return t.Transport.RoundTrip(req)
}
