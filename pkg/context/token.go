package context

import "context"

// tokenCtxKey is a context key for authentication token information
type tokenCtxKey struct{}

// WithTokenInfo adds TokenInfo to the context
func WithTokenInfo(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenCtxKey{}, token)
}

// GetTokenInfo retrieves the authentication token from the context
func GetTokenInfo(ctx context.Context) (string, bool) {
	if token, ok := ctx.Value(tokenCtxKey{}).(string); ok {
		return token, true
	}
	return "", false
}
