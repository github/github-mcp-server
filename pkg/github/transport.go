package github

import (
	"net/http"
	"strings"
)

// GraphQLFeaturesTransport is an http.RoundTripper that adds GraphQL-Features
// header based on context values set by withGraphQLFeatures.
//
// This transport should be used in the HTTP client chain for githubv4.Client
// to ensure GraphQL feature flags are properly sent to the GitHub API.
//
// Example usage:
//
//	httpClient := &http.Client{
//		Transport: &github.GraphQLFeaturesTransport{
//			Transport: http.DefaultTransport,
//		},
//	}
//	gqlClient := githubv4.NewClient(httpClient)
type GraphQLFeaturesTransport struct {
	// Transport is the underlying http.RoundTripper. If nil, http.DefaultTransport is used.
	Transport http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
// It adds the GraphQL-Features header if features are present in the request context.
func (t *GraphQLFeaturesTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Clone request to avoid modifying the original
	req = req.Clone(req.Context())

	// Check for GraphQL-Features in context and add header if present
	if features := GetGraphQLFeatures(req.Context()); len(features) > 0 {
		req.Header.Set("GraphQL-Features", strings.Join(features, ", "))
	}

	return transport.RoundTrip(req)
}
