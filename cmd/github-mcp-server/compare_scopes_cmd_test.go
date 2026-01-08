package main

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompareScopesLogic tests the core logic of scope comparison
func TestCompareScopesLogic(t *testing.T) {
	// Get all tools from inventory
	tr, _ := translations.TranslationHelper()
	inventory := github.NewInventory(tr).WithToolsets([]string{"all"}).Build()
	allTools := inventory.AllTools()

	// Collect unique required scopes
	requiredScopesSet := make(map[string]bool)
	for _, tool := range allTools {
		for _, scope := range tool.RequiredScopes {
			requiredScopesSet[scope] = true
		}
	}

	// Should have some required scopes
	require.NotEmpty(t, requiredScopesSet, "Expected some tools to require scopes")

	// Test with token that has all scopes
	allScopes := make([]string, 0, len(requiredScopesSet))
	for scope := range requiredScopesSet {
		allScopes = append(allScopes, scope)
	}

	// Check that each tool either requires no scopes or has its requirements met
	missingCount := 0
	for _, tool := range allTools {
		if len(tool.AcceptedScopes) == 0 {
			continue
		}
		if !scopes.HasRequiredScopes(allScopes, tool.AcceptedScopes) {
			missingCount++
		}
	}

	// When we have all required scopes, no tools should be missing access
	assert.Equal(t, 0, missingCount, "Expected no tools to be missing when all scopes present")

	// Test with empty token (no scopes)
	emptyScopes := []string{}
	missingWithEmpty := 0
	for _, tool := range allTools {
		if len(tool.AcceptedScopes) == 0 {
			continue
		}
		if !scopes.HasRequiredScopes(emptyScopes, tool.AcceptedScopes) {
			missingWithEmpty++
		}
	}

	// With empty scopes, some tools requiring scopes should be inaccessible
	assert.Greater(t, missingWithEmpty, 0, "Expected some tools to be missing with empty scopes")
}

// TestScopeHierarchyInComparison tests that scope hierarchy is respected
func TestScopeHierarchyInComparison(t *testing.T) {
	// If token has "repo", it should grant access to tools requiring "public_repo"
	tokenWithRepo := []string{"repo"}
	acceptedScopes := []string{"public_repo", "repo"} // Tool accepts either

	hasAccess := scopes.HasRequiredScopes(tokenWithRepo, acceptedScopes)
	assert.True(t, hasAccess, "Token with 'repo' should grant access to tools accepting 'public_repo'")

	// If token has "public_repo", it should NOT grant access to tools requiring full "repo"
	tokenWithPublicRepo := []string{"public_repo"}
	acceptedScopesFullRepo := []string{"repo"}

	hasAccess = scopes.HasRequiredScopes(tokenWithPublicRepo, acceptedScopesFullRepo)
	assert.False(t, hasAccess, "Token with 'public_repo' should NOT grant access to tools requiring full 'repo'")
}

// TestInventoryHasToolsWithScopes verifies the inventory contains tools with scope requirements
func TestInventoryHasToolsWithScopes(t *testing.T) {
	tr, _ := translations.TranslationHelper()
	inventory := github.NewInventory(tr).WithToolsets([]string{"all"}).Build()
	allTools := inventory.AllTools()

	// Count tools with scope requirements
	toolsWithScopes := 0
	toolsWithoutScopes := 0

	for _, tool := range allTools {
		if len(tool.RequiredScopes) > 0 {
			toolsWithScopes++
		} else {
			toolsWithoutScopes++
		}
	}

	// We should have both tools with and without scope requirements
	assert.Greater(t, toolsWithScopes, 0, "Expected some tools to require scopes")
	assert.Greater(t, toolsWithoutScopes, 0, "Expected some tools to not require scopes")

	t.Logf("Tools with scopes: %d, Tools without scopes: %d", toolsWithScopes, toolsWithoutScopes)
}
