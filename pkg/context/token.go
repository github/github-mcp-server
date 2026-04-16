package context

import (
	"context"

	"github.com/github/github-mcp-server/pkg/utils"
)

type tokenCtxKey struct{}

type TokenInfo struct {
	Token     string
	TokenType utils.TokenType
}

// WithTokenInfo adds TokenInfo to the context
func WithTokenInfo(ctx context.Context, tokenInfo *TokenInfo) context.Context {
	return context.WithValue(ctx, tokenCtxKey{}, tokenInfo)
}

// GetTokenInfo retrieves the authentication token from the context
func GetTokenInfo(ctx context.Context) (*TokenInfo, bool) {
	if tokenInfo, ok := ctx.Value(tokenCtxKey{}).(*TokenInfo); ok {
		return tokenInfo, true
	}
	return nil, false
}

type tokenScopesKey struct{}

type tokenScopesValue struct {
	Token  string
	Scopes []string
}

// WithTokenScopes adds token scopes to the context
func WithTokenScopes(ctx context.Context, scopes []string) context.Context {
	return context.WithValue(ctx, tokenScopesKey{}, scopes)
}

// WithTokenScopesForToken adds token scopes and the associated token to the context.
func WithTokenScopesForToken(ctx context.Context, token string, scopes []string) context.Context {
	return context.WithValue(ctx, tokenScopesKey{}, tokenScopesValue{
		Token:  token,
		Scopes: scopes,
	})
}

// GetTokenScopes retrieves token scopes from the context
func GetTokenScopes(ctx context.Context) ([]string, bool) {
	if scoped, ok := ctx.Value(tokenScopesKey{}).(tokenScopesValue); ok {
		return scoped.Scopes, true
	}
	if scopes, ok := ctx.Value(tokenScopesKey{}).([]string); ok {
		return scopes, true
	}
	return nil, false
}

// GetTokenScopesForToken retrieves token scopes only when they are bound to the active token.
func GetTokenScopesForToken(ctx context.Context, token string) ([]string, bool) {
	if scoped, ok := ctx.Value(tokenScopesKey{}).(tokenScopesValue); ok {
		if scoped.Token == token {
			return scoped.Scopes, true
		}
		return nil, false
	}
	if token == "" {
		if scopes, ok := ctx.Value(tokenScopesKey{}).([]string); ok {
			return scopes, true
		}
	}
	return nil, false
}
