package scopes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopeString(t *testing.T) {
	tests := []struct {
		scope    Scope
		expected string
	}{
		{Repo, "repo"},
		{PublicRepo, "public_repo"},
		{Notifications, "notifications"},
		{Gist, "gist"},
		{NoScope, ""},
		{SecurityEvents, "security_events"},
		{ReadOrg, "read:org"},
		{Project, "project"},
		{ReadProject, "read:project"},
	}

	for _, tt := range tests {
		t.Run(string(tt.scope), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.scope.String())
		})
	}
}

func TestScopeIncludes(t *testing.T) {
	tests := []struct {
		name     string
		have     Scope
		need     Scope
		expected bool
	}{
		{"same scope", Repo, Repo, true},
		{"repo includes public_repo", Repo, PublicRepo, true},
		{"repo includes repo:status", Repo, RepoStatus, true},
		{"repo includes security_events", Repo, SecurityEvents, true},
		{"public_repo does not include repo", PublicRepo, Repo, false},
		{"user includes read:user", User, ReadUser, true},
		{"user includes user:email", User, UserEmail, true},
		{"project includes read:project", Project, ReadProject, true},
		{"read:project does not include project", ReadProject, Project, false},
		{"admin:org includes write:org", AdminOrg, WriteOrg, true},
		{"admin:org includes read:org", AdminOrg, ReadOrg, true},
		{"write:org includes read:org", WriteOrg, ReadOrg, true},
		{"unrelated scopes", Gist, Notifications, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ScopeIncludes(tt.have, tt.need)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasRequiredScopes(t *testing.T) {
	tests := []struct {
		name     string
		have     []Scope
		required []Scope
		expected bool
	}{
		{
			name:     "exact match single scope",
			have:     []Scope{Repo},
			required: []Scope{Repo},
			expected: true,
		},
		{
			name:     "parent scope satisfies child",
			have:     []Scope{Repo},
			required: []Scope{PublicRepo},
			expected: true,
		},
		{
			name:     "multiple required all satisfied",
			have:     []Scope{Repo, Notifications},
			required: []Scope{PublicRepo, Notifications},
			expected: true,
		},
		{
			name:     "missing required scope",
			have:     []Scope{Repo},
			required: []Scope{Notifications},
			expected: false,
		},
		{
			name:     "empty required",
			have:     []Scope{Repo},
			required: []Scope{},
			expected: true,
		},
		{
			name:     "empty have with required",
			have:     []Scope{},
			required: []Scope{Repo},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasRequiredScopes(tt.have, tt.required)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWithScopes(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []Scope
		expected []string
	}{
		{
			name:     "single scope",
			scopes:   []Scope{Repo},
			expected: []string{"repo"},
		},
		{
			name:     "multiple scopes",
			scopes:   []Scope{Repo, Notifications},
			expected: []string{"repo", "notifications"},
		},
		{
			name:     "no scope",
			scopes:   []Scope{NoScope},
			expected: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := WithScopes(tt.scopes...)
			require.NotNil(t, meta)

			scopeVal, ok := meta[MetaKey]
			require.True(t, ok)

			scopeStrings, ok := scopeVal.([]string)
			require.True(t, ok)
			assert.Equal(t, tt.expected, scopeStrings)
		})
	}
}

func TestGetScopesFromMeta(t *testing.T) {
	tests := []struct {
		name     string
		meta     map[string]any
		expected []Scope
	}{
		{
			name:     "nil meta",
			meta:     nil,
			expected: nil,
		},
		{
			name:     "empty meta",
			meta:     map[string]any{},
			expected: nil,
		},
		{
			name: "string slice",
			meta: map[string]any{
				MetaKey: []string{"repo", "notifications"},
			},
			expected: []Scope{Repo, Notifications},
		},
		{
			name: "any slice (from JSON)",
			meta: map[string]any{
				MetaKey: []any{"repo", "gist"},
			},
			expected: []Scope{Repo, Gist},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetScopesFromMeta(tt.meta)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAcceptedScopes(t *testing.T) {
	tests := []struct {
		name     string
		required Scope
		contains []Scope
	}{
		{
			name:     "repo is accepted by itself",
			required: Repo,
			contains: []Scope{Repo},
		},
		{
			name:     "public_repo accepted by repo",
			required: PublicRepo,
			contains: []Scope{PublicRepo, Repo},
		},
		{
			name:     "read:org accepted by admin:org and write:org",
			required: ReadOrg,
			contains: []Scope{ReadOrg, WriteOrg, AdminOrg},
		},
		{
			name:     "read:project accepted by project",
			required: ReadProject,
			contains: []Scope{ReadProject, Project},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAcceptedScopes(tt.required)
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "expected %s to be in accepted scopes", expected)
			}
		})
	}
}

func TestScopeStringsAndParseScopes(t *testing.T) {
	original := []Scope{Repo, Notifications, Gist}
	strings := ScopeStrings(original)

	assert.Equal(t, []string{"repo", "notifications", "gist"}, strings)

	parsed := ParseScopes(strings)
	assert.Equal(t, original, parsed)
}
