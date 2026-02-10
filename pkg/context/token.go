package context

import (
	"context"

	"github.com/github/github-mcp-server/pkg/utils"
)

// tokenCtxKey is a context key for authentication token information
type tokenCtx string

var tokenCtxKey tokenCtx = "tokenctx"

type TokenInfo struct {
	Token     string
	TokenType utils.TokenType
}

// WithTokenInfo adds TokenInfo to the context
func WithTokenInfo(ctx context.Context, tokenInfo *TokenInfo) context.Context {
	return context.WithValue(ctx, tokenCtxKey, tokenInfo)
}

// GetTokenInfo retrieves the authentication token from the context
func GetTokenInfo(ctx context.Context) (*TokenInfo, bool) {
	if tokenInfo, ok := ctx.Value(tokenCtxKey).(*TokenInfo); ok {
		return tokenInfo, true
	}
	return nil, false
}

type TokenScopesKey tokenCtx

var tokenScopesKey TokenScopesKey = "tokenscopesctx"

// WithTokenScopes adds token scopes to the context
func WithTokenScopes(ctx context.Context, scopes []string) context.Context {
	return context.WithValue(ctx, tokenScopesKey, scopes)
}

// GetTokenScopes retrieves token scopes from the context
func GetTokenScopes(ctx context.Context) ([]string, bool) {
	if scopes, ok := ctx.Value(tokenScopesKey).([]string); ok {
		return scopes, true
	}
	return nil, false
}
