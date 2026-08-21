package scopes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuthScopeCatalog(t *testing.T) {
	supported := SupportedOAuthScopes()
	defaults := DefaultOAuthScopes()

	assert.Subset(t, supported, defaults)
	assert.Contains(t, supported, string(DeleteRepo))
	assert.NotContains(t, defaults, string(DeleteRepo))
	assert.Contains(t, supported, string(Workflow))
	assert.NotContains(t, defaults, string(Workflow))
	assert.Contains(t, supported, string(Codespace))
	assert.NotContains(t, defaults, string(Codespace))
}

func TestScopeChecks(t *testing.T) {
	assert.True(t, HasAll([]string{"repo", "workflow"}, Repo, Workflow))
	assert.False(t, HasAll([]string{"repo"}, Repo, Workflow))
	assert.True(t, HasAll([]string{"admin:org"}, ReadOrg))
	assert.Nil(t, ChallengeAll([]string{"repo", "workflow"}, Repo, Workflow))
	assert.Equal(t, []string{"repo", "workflow"}, ChallengeAll([]string{"repo"}, Repo, Workflow))
}

func TestScopeHierarchy(t *testing.T) {
	// Verify the hierarchy is correctly defined
	assert.Contains(t, ScopeHierarchy[Repo], PublicRepo)
	assert.Contains(t, ScopeHierarchy[Repo], SecurityEvents)
	assert.Contains(t, ScopeHierarchy[AdminOrg], WriteOrg)
	assert.Contains(t, ScopeHierarchy[AdminOrg], ReadOrg)
	assert.Contains(t, ScopeHierarchy[WriteOrg], ReadOrg)
	assert.Contains(t, ScopeHierarchy[Project], ReadProject)
	assert.Contains(t, ScopeHierarchy[WritePackages], ReadPackages)
	assert.Contains(t, ScopeHierarchy[User], ReadUser)
	assert.Contains(t, ScopeHierarchy[User], UserEmail)
}

func TestExpandScopeSet(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		expected map[string]bool
	}{
		{
			name:     "empty scopes",
			scopes:   []string{},
			expected: map[string]bool{},
		},
		{
			name:   "repo expands to include public_repo and security_events",
			scopes: []string{"repo"},
			expected: map[string]bool{
				"repo":            true,
				"public_repo":     true,
				"security_events": true,
			},
		},
		{
			name:   "admin:org expands to include write:org and read:org",
			scopes: []string{"admin:org"},
			expected: map[string]bool{
				"admin:org": true,
				"write:org": true,
				"read:org":  true,
			},
		},
		{
			name:   "write:org expands to include read:org",
			scopes: []string{"write:org"},
			expected: map[string]bool{
				"write:org": true,
				"read:org":  true,
			},
		},
		{
			name:   "user expands to include read:user and user:email",
			scopes: []string{"user"},
			expected: map[string]bool{
				"user":       true,
				"read:user":  true,
				"user:email": true,
			},
		},
		{
			name:   "scope without children stays as-is",
			scopes: []string{"gist"},
			expected: map[string]bool{
				"gist": true,
			},
		},
		{
			name:   "multiple scopes combine correctly",
			scopes: []string{"repo", "gist"},
			expected: map[string]bool{
				"repo":            true,
				"public_repo":     true,
				"security_events": true,
				"gist":            true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandScopeSet(tt.scopes)
			assert.Equal(t, tt.expected, result)
		})
	}
}
