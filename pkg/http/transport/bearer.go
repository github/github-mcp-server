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

	// TokenProvider, if set, is called on each request to get the current token.
	// Takes precedence over the static Token field. This supports OAuth flows
	// where the token is obtained lazily after server startup.
	TokenProvider func() string
}

func (t *BearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	token := t.Token
	if t.TokenProvider != nil {
		token = t.TokenProvider()
	}
	req.Header.Set(headers.AuthorizationHeader, "Bearer "+token)

	// Check for GraphQL-Features in context and add header if present
	if features := ghcontext.GetGraphQLFeatures(req.Context()); len(features) > 0 {
		req.Header.Set(headers.GraphQLFeaturesHeader, strings.Join(features, ", "))
	}

	return t.Transport.RoundTrip(req)
}
