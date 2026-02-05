package context

import (
	"context"

	"github.com/github/github-mcp-server/pkg/utils"
)

// tokenCtxKey is a context key for authentication token information
type tokenCtx string

var tokenCtxKey tokenCtx = "tokenctx"

type TokenInfo struct {
	Token         string
	TokenType     utils.TokenType
	ScopesFetched bool
	Scopes        []string
}

// WithTokenInfo adds TokenInfo to the context
func WithTokenInfo(ctx context.Context, tokenInfo *TokenInfo) context.Context {
	return context.WithValue(ctx, tokenCtxKey, tokenInfo)
}

func SetTokenScopes(ctx context.Context, scopes []string) {
	if tokenInfo, ok := GetTokenInfo(ctx); ok {
		tokenInfo.Scopes = scopes
		tokenInfo.ScopesFetched = true
	}
}

// GetTokenInfo retrieves the authentication token from the context
func GetTokenInfo(ctx context.Context) (*TokenInfo, bool) {
	if tokenInfo, ok := ctx.Value(tokenCtxKey).(*TokenInfo); ok {
		return tokenInfo, true
	}
	return nil, false
}
