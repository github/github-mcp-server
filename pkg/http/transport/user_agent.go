package transport

import (
	"context"
	"net/http"

	"github.com/github/github-mcp-server/pkg/http/headers"
)

type userAgentKey struct{}

// WithUserAgent supplies a request-scoped identity without mutating a shared transport.
func WithUserAgent(ctx context.Context, agent string) context.Context {
	return context.WithValue(ctx, userAgentKey{}, agent)
}

type UserAgentTransport struct {
	Transport http.RoundTripper
	Agent     string
}

func (t *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	agent := t.Agent
	if scoped, ok := req.Context().Value(userAgentKey{}).(string); ok {
		agent = scoped
	}
	req.Header.Set(headers.UserAgentHeader, agent)
	return t.Transport.RoundTrip(req)
}
