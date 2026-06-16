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
	// and takes precedence over Token. It backs OAuth, where the token is
	// obtained after the client is built and is refreshed over the session's
	// lifetime. It may return an empty string before authorization completes.
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
