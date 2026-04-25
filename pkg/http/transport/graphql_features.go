package transport

import (
	"net/http"
	"strings"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/http/headers"
)

// GraphQLFeaturesTransport is an http.RoundTripper that adds GraphQL-Features
// header to requests based on context values. This is required for using
// non-GA GraphQL API features like the agent assignment API.
//
// This transport is used internally by the MCP server and is also exported
// for library consumers who need to build their own HTTP clients with
// GraphQL feature flag support.
//
// Usage:
//
//	import "github.com/github/github-mcp-server/pkg/http/transport"
//
//	httpClient := &http.Client{
//	    Transport: &transport.GraphQLFeaturesTransport{
//	        Transport: http.DefaultTransport,
//	    },
//	}
//	gqlClient := githubv4.NewClient(httpClient)
//
// Then use ghcontext.WithGraphQLFeatures(ctx, "feature_name") when calling GraphQL operations.
type GraphQLFeaturesTransport struct {
	// Transport is the underlying HTTP transport. If nil, http.DefaultTransport is used.
	Transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t *GraphQLFeaturesTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Clone the request to avoid mutating the original
	req = req.Clone(req.Context())

	// Check for GraphQL-Features in context and add header if present
	if features := ghcontext.GetGraphQLFeatures(req.Context()); len(features) > 0 {
		req.Header.Set(headers.GraphQLFeaturesHeader, strings.Join(features, ", "))
	}

	return transport.RoundTrip(req)
}
