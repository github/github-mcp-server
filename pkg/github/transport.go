package github

import (
	"context"
	"net/http"
	"strings"
)

// graphQLFeaturesKey is a context key for GraphQL feature flags.
// These flags enable preview or experimental GitHub API features that are not yet GA.
type graphQLFeaturesKey struct{}

// WithGraphQLFeatures adds GraphQL feature flags to the context.
// The flags are read by GraphQLFeaturesTransport and sent as the GraphQL-Features header.
// This is used by tool handlers that require experimental GitHub API features.
// Remote servers can also use this function in tests to simulate feature flag contexts.
func WithGraphQLFeatures(ctx context.Context, features ...string) context.Context {
	return context.WithValue(ctx, graphQLFeaturesKey{}, features)
}

// GetGraphQLFeatures retrieves GraphQL feature flags from the context.
// This function is exported to allow custom HTTP transports (e.g., in remote servers)
// to read feature flags and add them as the "GraphQL-Features" header.
//
// For most use cases, use GraphQLFeaturesTransport instead of calling this directly.
func GetGraphQLFeatures(ctx context.Context) []string {
	if features, ok := ctx.Value(graphQLFeaturesKey{}).([]string); ok {
		return features
	}
	return nil
}

// GraphQLFeaturesTransport is an http.RoundTripper that adds GraphQL-Features
// header based on context values set by WithGraphQLFeatures.
//
// This transport should be used in the HTTP client chain for githubv4.Client
// to ensure GraphQL feature flags are properly sent to the GitHub API.
// Without this transport, certain GitHub API features (like Copilot assignment)
// that require feature flags will fail with schema validation errors.
//
// Example usage for local server (layering with auth):
//
//	httpClient := &http.Client{
//		Transport: &github.GraphQLFeaturesTransport{
//			Transport: &authTransport{
//				Transport: http.DefaultTransport,
//				token:     "ghp_...",
//			},
//		},
//	}
//	gqlClient := githubv4.NewClient(httpClient)
//
// Example usage for remote server (simple case):
//
//	httpClient := &http.Client{
//		Transport: &github.GraphQLFeaturesTransport{
//			Transport: http.DefaultTransport,
//		},
//	}
//	gqlClient := githubv4.NewClient(httpClient)
//
// The transport reads feature flags from request context using GetGraphQLFeatures.
// Feature flags are added to context by the tool handler via WithGraphQLFeatures.
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
