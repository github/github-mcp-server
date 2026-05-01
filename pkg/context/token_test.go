package context

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTokenScopesForToken_MatchesBoundToken(t *testing.T) {
	ctx := WithTokenScopesForToken(context.Background(), "token-a", []string{"repo"})

	scopes, ok := GetTokenScopesForToken(ctx, "token-a")
	assert.True(t, ok)
	assert.Equal(t, []string{"repo"}, scopes)

	scopes, ok = GetTokenScopesForToken(ctx, "token-b")
	assert.False(t, ok)
	assert.Nil(t, scopes)
}

func TestGetTokenScopesForToken_DoesNotReuseLegacyScopesForNonEmptyToken(t *testing.T) {
	ctx := WithTokenScopes(context.Background(), []string{"repo"})

	scopes, ok := GetTokenScopesForToken(ctx, "token-a")
	assert.False(t, ok)
	assert.Nil(t, scopes)

	legacyScopes, legacyOK := GetTokenScopes(ctx)
	assert.True(t, legacyOK)
	assert.Equal(t, []string{"repo"}, legacyScopes)
}
