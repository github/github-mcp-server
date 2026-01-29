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
func WithTokenInfo(ctx context.Context, token string, tokenType utils.TokenType) context.Context {
	return context.WithValue(ctx, tokenCtxKey, TokenInfo{Token: token, TokenType: tokenType})
}

// GetTokenInfo retrieves the authentication token from the context
func GetTokenInfo(ctx context.Context) (TokenInfo, bool) {
	if tokenInfo, ok := ctx.Value(tokenCtxKey).(TokenInfo); ok {
		return tokenInfo, true
	}
	return TokenInfo{}, false
}
