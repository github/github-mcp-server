package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareScopes(t *testing.T) {
	tests := []struct {
		name           string
		tokenScopes    []string
		requiredScopes []string
		wantMissing    []string
		wantExtra      []string
		wantHasAll     bool
	}{
		{
			name:           "exact match",
			tokenScopes:    []string{"repo", "user"},
			requiredScopes: []string{"repo", "user"},
			wantMissing:    []string{},
			wantExtra:      []string{},
			wantHasAll:     true,
		},
		{
			name:           "missing scopes",
			tokenScopes:    []string{"user"},
			requiredScopes: []string{"repo", "user"},
			wantMissing:    []string{"repo"},
			wantExtra:      []string{},
			wantHasAll:     false,
		},
		{
			name:           "extra scopes",
			tokenScopes:    []string{"repo", "user", "gist"},
			requiredScopes: []string{"repo", "user"},
			wantMissing:    []string{},
			wantExtra:      []string{"gist"},
			wantHasAll:     true,
		},
		{
			name:           "missing and extra scopes",
			tokenScopes:    []string{"user", "gist"},
			requiredScopes: []string{"repo", "user"},
			wantMissing:    []string{"repo"},
			wantExtra:      []string{"gist"},
			wantHasAll:     false,
		},
		{
			name:           "empty token scopes",
			tokenScopes:    []string{},
			requiredScopes: []string{"repo", "user"},
			wantMissing:    []string{"repo", "user"},
			wantExtra:      []string{},
			wantHasAll:     false,
		},
		{
			name:           "empty required scopes",
			tokenScopes:    []string{"repo", "user"},
			requiredScopes: []string{},
			wantMissing:    []string{},
			wantExtra:      []string{"repo", "user"},
			wantHasAll:     true,
		},
		{
			name:           "both empty",
			tokenScopes:    []string{},
			requiredScopes: []string{},
			wantMissing:    []string{},
			wantExtra:      []string{},
			wantHasAll:     true,
		},
		{
			name:           "parent scope covers child scope",
			tokenScopes:    []string{"repo"},
			requiredScopes: []string{"public_repo"},
			wantMissing:    []string{},
			wantExtra:      []string{},
			wantHasAll:     true,
		},
		{
			name:           "admin:org covers read:org",
			tokenScopes:    []string{"admin:org"},
			requiredScopes: []string{"read:org"},
			wantMissing:    []string{},
			wantExtra:      []string{},
			wantHasAll:     true,
		},
		{
			name:           "child scope doesn't cover parent",
			tokenScopes:    []string{"public_repo"},
			requiredScopes: []string{"repo"},
			wantMissing:    []string{"repo"},
			wantExtra:      []string{},
			wantHasAll:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareScopes(tt.tokenScopes, tt.requiredScopes)

			assert.Equal(t, tt.tokenScopes, result.TokenScopes)
			assert.Equal(t, tt.requiredScopes, result.RequiredScopes)
			assert.Equal(t, tt.wantHasAll, result.HasAllRequired, "HasAllRequired mismatch")

			// Check missing scopes
			if len(tt.wantMissing) == 0 {
				assert.Empty(t, result.MissingScopes, "expected no missing scopes")
			} else {
				require.Equal(t, tt.wantMissing, result.MissingScopes, "missing scopes mismatch")
			}

			// Check extra scopes
			if len(tt.wantExtra) == 0 {
				assert.Empty(t, result.ExtraScopes, "expected no extra scopes")
			} else {
				require.Equal(t, tt.wantExtra, result.ExtraScopes, "extra scopes mismatch")
			}
		})
	}
}

func TestCompareScopes_Sorting(t *testing.T) {
	// Test that missing and extra scopes are sorted
	result := compareScopes(
		[]string{"zebra", "alpha", "beta"},
		[]string{"delta", "charlie", "alpha"},
	)

	// Missing scopes should be sorted
	assert.Equal(t, []string{"charlie", "delta"}, result.MissingScopes)

	// Extra scopes should be sorted
	assert.Equal(t, []string{"beta", "zebra"}, result.ExtraScopes)
}
