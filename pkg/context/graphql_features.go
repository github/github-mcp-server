package context

import "context"

// graphQLFeaturesKey is a context key for GraphQL feature flags
type graphQLFeaturesKey struct{}

// withGraphQLFeatures adds GraphQL feature flags to the context
func WithGraphQLFeatures(ctx context.Context, features ...string) context.Context {
	return context.WithValue(ctx, graphQLFeaturesKey{}, features)
}

// GetGraphQLFeatures retrieves GraphQL feature flags from the context
func GetGraphQLFeatures(ctx context.Context) []string {
	if features, ok := ctx.Value(graphQLFeaturesKey{}).([]string); ok {
		return features
	}
	return nil
}
