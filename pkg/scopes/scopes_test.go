package scopes

import (
	"sort"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/stretchr/testify/assert"
)

func TestExpandScopes(t *testing.T) {
	tests := []struct {
		name     string
		required []Scope
		expected []string
	}{
		{
			name:     "nil returns nil",
			required: nil,
			expected: nil,
		},
		{
			name:     "empty returns nil",
			required: []Scope{},
			expected: nil,
		},
		{
			name:     "repo scope returns just repo",
			required: []Scope{Repo},
			expected: []string{"repo"},
		},
		{
			name:     "public_repo also accepts repo (parent)",
			required: []Scope{PublicRepo},
			expected: []string{"public_repo", "repo"},
		},
		{
			name:     "delete_repo returns just delete_repo",
			required: []Scope{DeleteRepo},
			expected: []string{"delete_repo"},
		},
		{
			name:     "security_events also accepts repo (parent)",
			required: []Scope{SecurityEvents},
			expected: []string{"repo", "security_events"},
		},
		{
			name:     "read:org also accepts write:org and admin:org (parents)",
			required: []Scope{ReadOrg},
			expected: []string{"admin:org", "read:org", "write:org"},
		},
		{
			name:     "write:org also accepts admin:org (parent)",
			required: []Scope{WriteOrg},
			expected: []string{"admin:org", "write:org"},
		},
		{
			name:     "admin:org returns just admin:org (no parent)",
			required: []Scope{AdminOrg},
			expected: []string{"admin:org"},
		},
		{
			name:     "read:project also accepts project (parent)",
			required: []Scope{ReadProject},
			expected: []string{"project", "read:project"},
		},
		{
			name:     "project returns just project (no parent)",
			required: []Scope{Project},
			expected: []string{"project"},
		},
		{
			name:     "gist returns just gist (no parent)",
			required: []Scope{Gist},
			expected: []string{"gist"},
		},
		{
			name:     "notifications returns just notifications (no parent)",
			required: []Scope{Notifications},
			expected: []string{"notifications"},
		},
		{
			name:     "read:packages also accepts write:packages (parent)",
			required: []Scope{ReadPackages},
			expected: []string{"read:packages", "write:packages"},
		},
		{
			name:     "read:user also accepts user (parent)",
			required: []Scope{ReadUser},
			expected: []string{"read:user", "user"},
		},
		{
			name:     "multiple scopes combine correctly",
			required: []Scope{PublicRepo, ReadOrg},
			expected: []string{"admin:org", "public_repo", "read:org", "repo", "write:org"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandScopes(tt.required...)

			// Sort both for consistent comparison
			if result != nil {
				sort.Strings(result)
			}
			if tt.expected != nil {
				sort.Strings(tt.expected)
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

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

func TestScopePolicy(t *testing.T) {
	t.Run("any authorization path is sufficient", func(t *testing.T) {
		policy := AnyOfScopePolicy(Repo, ReadOrg)

		assert.True(t, ScopePolicySatisfied([]string{"repo"}, policy))
		assert.True(t, ScopePolicySatisfied([]string{"admin:org"}, policy))
		assert.Nil(t, ChallengeScopesForPolicy([]string{"admin:org"}, policy))
		assert.False(t, ScopePolicySatisfied([]string{"gist"}, policy))
		assert.Equal(t, []string{"repo"}, ChallengeScopesForPolicy([]string{"gist"}, policy))
	})

	t.Run("every requirement in a path must be satisfied", func(t *testing.T) {
		policy := AllOfScopePolicy(Repo, Workflow)

		assert.True(t, ScopePolicySatisfied([]string{"repo", "workflow"}, policy))
		assert.False(t, ScopePolicySatisfied([]string{"repo"}, policy))
		assert.Equal(t, []string{"workflow"}, ChallengeScopesForPolicy([]string{"repo"}, policy))
	})

	t.Run("challenge uses declared path preference", func(t *testing.T) {
		policy := inventory.ScopePolicy{
			AnyOf: []inventory.ScopePath{
				{AllOf: []inventory.ScopeRequirement{
					NewScopeRequirement(Repo),
					NewScopeRequirement(Workflow),
				}},
				{AllOf: []inventory.ScopeRequirement{NewScopeRequirement(ReadOrg)}},
			},
		}

		assert.Equal(t, []string{"repo", "workflow"}, ChallengeScopesForPolicy([]string{"gist"}, policy))
	})

	t.Run("canonical challenge scope always satisfies its requirement", func(t *testing.T) {
		policy := inventory.ScopePolicy{
			AnyOf: []inventory.ScopePath{{
				AllOf: []inventory.ScopeRequirement{{
					ChallengeScope: "narrow",
					AnyOf:          []string{"broad"},
				}},
			}},
		}

		assert.True(t, ScopePolicySatisfied([]string{"narrow"}, policy))
	})

	t.Run("accepted alternatives apply to one requirement only", func(t *testing.T) {
		policy := inventory.ScopePolicy{
			AnyOf: []inventory.ScopePath{{
				AllOf: []inventory.ScopeRequirement{
					NewScopeRequirement(PublicRepo),
					NewScopeRequirement(Workflow),
				},
			}},
		}

		assert.True(t, ScopePolicySatisfied([]string{"repo", "workflow"}, policy))
		assert.False(t, ScopePolicySatisfied([]string{"repo"}, policy))
		assert.Equal(t, []string{"workflow"}, ChallengeScopesForPolicy([]string{"repo"}, policy))
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
