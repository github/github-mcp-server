package transport

import (
	"net/http"
	"strings"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	headers "github.com/github/github-mcp-server/pkg/http/headers"
)

type BearerAuthTransport struct {
	Transport http.RoundTripper
	Token     string

	// TokenProvider, when non-nil, supplies the bearer token for each request
	// and takes precedence over Token.
	TokenProvider func() string

	// AllowedHosts, when non-empty, restricts the hosts the Authorization
	// header is attached to. The token is set only when the request host
	// and port exactly match one of these entries (case-insensitive). This
	// scopes the credential to the configured GitHub hosts, so that if a
	// response redirects off them the token is not carried to the redirect
	// target.
	//
	// net/http strips a cross-host Authorization header when it follows a
	// redirect, but only for headers set on the initial request. This
	// transport re-adds the header on every hop, so that protection does not
	// otherwise apply here.
	//
	// When empty, the token is attached to every request, preserving the
	// prior behavior.
	AllowedHosts []string
}

func (t *BearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	token := t.Token
	if t.TokenProvider != nil {
		token = t.TokenProvider()
	}
	if !t.hostAllowed(req.URL.Host) {
		req.Header.Del(headers.AuthorizationHeader)
	} else if token != "" {
		req.Header.Set(headers.AuthorizationHeader, "Bearer "+token)
	}

	// Check for GraphQL-Features in context and add header if present
	if features := ghcontext.GetGraphQLFeatures(req.Context()); len(features) > 0 {
		req.Header.Set(headers.GraphQLFeaturesHeader, strings.Join(features, ", "))
	}

	return t.Transport.RoundTrip(req)
}

// hostAllowed reports whether the token may be attached to a request bound for
// host. An empty AllowedHosts allows all hosts, preserving prior behavior.
func (t *BearerAuthTransport) hostAllowed(host string) bool {
	if len(t.AllowedHosts) == 0 {
		return true
	}
	for _, h := range t.AllowedHosts {
		if strings.EqualFold(h, host) {
			return true
		}
	}
	return false
}
