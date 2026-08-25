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
	assert.True(t, HasAllScopeNames([]string{"admin:org"}, []string{"read:org"}))
	assert.False(t, HasAllScopeNames([]string{"repo"}, []string{"repo", "workflow"}))
	assert.Nil(t, ChallengeAll([]string{"repo", "workflow"}, Repo, Workflow))
	assert.Equal(t, []string{"repo", "workflow"}, ChallengeAll([]string{"repo"}, Repo, Workflow))
}

func TestDynamicChallenge(t *testing.T) {
	visible := func([]string) bool { return true }
	challenge := func(map[string]any, []string) []string { return nil }
	access := DynamicChallenge([]Scope{Repo, Workflow}, visible, challenge)

	assert.Equal(t, []string{"repo", "workflow"}, access.Scopes)
	assert.True(t, access.Dynamic)
	assert.NotNil(t, access.Challenge)
	assert.True(t, access.Visible(nil))
	assert.Panics(t, func() {
		DynamicChallenge(nil, visible, challenge)
	})
	assert.Panics(t, func() {
		DynamicChallenge([]Scope{Repo}, visible, nil)
	})
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
