package scopes

import (
	"sort"
	"testing"

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

func TestToStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []Scope
		expected []string
	}{
		{
			name:     "empty returns empty",
			scopes:   []Scope{},
			expected: []string{},
		},
		{
			name:     "single scope",
			scopes:   []Scope{Repo},
			expected: []string{"repo"},
		},
		{
			name:     "multiple scopes",
			scopes:   []Scope{Repo, Gist, ReadOrg},
			expected: []string{"repo", "gist", "read:org"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToStringSlice(tt.scopes...)
			assert.Equal(t, tt.expected, result)
		})
	}
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
