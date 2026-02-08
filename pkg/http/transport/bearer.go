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
}

func (t *BearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set(headers.AuthorizationHeader, "Bearer "+t.Token)

	// Check for GraphQL-Features in context and add header if present
	if features := ghcontext.GetGraphQLFeatures(req.Context()); len(features) > 0 {
		req.Header.Set(headers.GraphQLFeaturesHeader, strings.Join(features, ", "))
	}

	return t.Transport.RoundTrip(req)
}
