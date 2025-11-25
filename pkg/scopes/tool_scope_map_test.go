package scopes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopeSetContains(t *testing.T) {
	set := NewScopeSet([]string{"repo", "user", "gist"})

	assert.True(t, set.Contains("repo"))
	assert.True(t, set.Contains("user"))
	assert.True(t, set.Contains("gist"))
	assert.False(t, set.Contains("admin"))
	assert.False(t, set.Contains(""))
}

func TestScopeSetContainsAny(t *testing.T) {
	set := NewScopeSet([]string{"repo", "user"})

	assert.True(t, set.ContainsAny("repo"))
	assert.True(t, set.ContainsAny("admin", "repo"))
	assert.True(t, set.ContainsAny("user", "gist"))
	assert.False(t, set.ContainsAny("admin", "gist"))
	assert.False(t, set.ContainsAny())
}

func TestScopeSetToSlice(t *testing.T) {
	set := NewScopeSet([]string{"repo", "user"})
	slice := set.ToSlice()

	assert.Len(t, slice, 2)
	assert.Contains(t, slice, "repo")
	assert.Contains(t, slice, "user")
}

func TestNewToolScopeInfo(t *testing.T) {
	tests := []struct {
		name            string
		required        []Scope
		wantRequired    []string
		wantAccepted    []string
		wantNotAccepted []string
	}{
		{
			name:         "no scopes required",
			required:     []Scope{},
			wantRequired: []string{},
			wantAccepted: []string{},
		},
		{
			name:         "single scope - repo",
			required:     []Scope{Repo},
			wantRequired: []string{"repo"},
			wantAccepted: []string{"repo"},
			// repo has no parent scopes
		},
		{
			name:         "scope with parent - public_repo",
			required:     []Scope{PublicRepo},
			wantRequired: []string{"public_repo"},
			wantAccepted: []string{"public_repo", "repo"}, // repo includes public_repo
		},
		{
			name:         "scope with parent - read:org",
			required:     []Scope{ReadOrg},
			wantRequired: []string{"read:org"},
			wantAccepted: []string{"read:org", "write:org", "admin:org"},
		},
		{
			name:         "multiple scopes",
			required:     []Scope{Repo, Notifications},
			wantRequired: []string{"repo", "notifications"},
			wantAccepted: []string{"repo", "notifications"},
		},
		{
			name:         "scope with deep hierarchy - read:repo_hook",
			required:     []Scope{ReadRepoHook},
			wantRequired: []string{"read:repo_hook"},
			wantAccepted: []string{"read:repo_hook", "write:repo_hook", "admin:repo_hook"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := NewToolScopeInfo(tt.required)

			// Check required scopes
			for _, scope := range tt.wantRequired {
				assert.True(t, info.RequiredScopes.Contains(scope),
					"expected required scope %s to be present", scope)
			}
			assert.Equal(t, len(tt.wantRequired), len(info.RequiredScopes),
				"unexpected number of required scopes")

			// Check accepted scopes
			for _, scope := range tt.wantAccepted {
				assert.True(t, info.AcceptedScopes.Contains(scope),
					"expected accepted scope %s to be present", scope)
			}

			// Check not accepted scopes
			for _, scope := range tt.wantNotAccepted {
				assert.False(t, info.AcceptedScopes.Contains(scope),
					"expected scope %s to NOT be accepted", scope)
			}
		})
	}
}

func TestToolScopeInfoHasAcceptedScope(t *testing.T) {
	tests := []struct {
		name       string
		required   []Scope
		userScopes []string
		want       bool
	}{
		{
			name:       "no requirements - always passes",
			required:   []Scope{},
			userScopes: []string{},
			want:       true,
		},
		{
			name:       "has exact required scope",
			required:   []Scope{Repo},
			userScopes: []string{"repo"},
			want:       true,
		},
		{
			name:       "missing required scope",
			required:   []Scope{Repo},
			userScopes: []string{"gist"},
			want:       false,
		},
		{
			name:       "has parent scope - repo satisfies public_repo",
			required:   []Scope{PublicRepo},
			userScopes: []string{"repo"},
			want:       true,
		},
		{
			name:       "has parent scope - admin:org satisfies read:org",
			required:   []Scope{ReadOrg},
			userScopes: []string{"admin:org"},
			want:       true,
		},
		{
			name:       "has one of multiple user scopes",
			required:   []Scope{Repo},
			userScopes: []string{"gist", "repo", "user"},
			want:       true,
		},
		{
			name:       "empty user scopes - fails",
			required:   []Scope{Repo},
			userScopes: []string{},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := NewToolScopeInfo(tt.required)
			got := info.HasAcceptedScope(tt.userScopes...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToolScopeInfoMissingScopes(t *testing.T) {
	tests := []struct {
		name       string
		required   []Scope
		userScopes []string
		want       []string
	}{
		{
			name:       "no requirements",
			required:   []Scope{},
			userScopes: []string{},
			want:       nil,
		},
		{
			name:       "all satisfied",
			required:   []Scope{Repo},
			userScopes: []string{"repo"},
			want:       nil,
		},
		{
			name:       "missing repo",
			required:   []Scope{Repo},
			userScopes: []string{"gist"},
			want:       []string{"repo"},
		},
		{
			name:       "satisfied by parent scope",
			required:   []Scope{ReadOrg},
			userScopes: []string{"admin:org"},
			want:       nil,
		},
		{
			name:       "multiple missing",
			required:   []Scope{Repo, Notifications},
			userScopes: []string{"gist"},
			want:       []string{"repo", "notifications"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := NewToolScopeInfo(tt.required)
			got := info.MissingScopes(tt.userScopes...)

			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.ElementsMatch(t, tt.want, got)
			}
		})
	}
}

func TestBuildToolScopeMapFromMeta(t *testing.T) {
	tools := []ToolMeta{
		{
			Name: "get_repo",
			Meta: WithScopes(Repo),
		},
		{
			Name: "create_gist",
			Meta: WithScopes(Gist),
		},
		{
			Name: "get_user",
			Meta: nil, // no scopes
		},
	}

	scopeMap := BuildToolScopeMapFromMeta(tools)

	require.Len(t, scopeMap, 3)

	// Check get_repo
	repoInfo, ok := scopeMap["get_repo"]
	require.True(t, ok)
	assert.True(t, repoInfo.RequiredScopes.Contains("repo"))
	assert.True(t, repoInfo.HasAcceptedScope("repo"))

	// Check create_gist
	gistInfo, ok := scopeMap["create_gist"]
	require.True(t, ok)
	assert.True(t, gistInfo.RequiredScopes.Contains("gist"))
	assert.True(t, gistInfo.HasAcceptedScope("gist"))
	assert.False(t, gistInfo.HasAcceptedScope("repo"))

	// Check get_user (no scopes)
	userInfo, ok := scopeMap["get_user"]
	require.True(t, ok)
	assert.Empty(t, userInfo.RequiredScopes)
	assert.True(t, userInfo.HasAcceptedScope()) // no requirements always passes
}

func TestGetToolScopeInfo(t *testing.T) {
	meta := WithScopes(Repo, Notifications)
	info := GetToolScopeInfo(meta)

	assert.True(t, info.RequiredScopes.Contains("repo"))
	assert.True(t, info.RequiredScopes.Contains("notifications"))
	assert.True(t, info.HasAcceptedScope("repo", "notifications"))
}

func TestToolScopeMapAllRequiredScopes(t *testing.T) {
	tools := []ToolMeta{
		{Name: "tool1", Meta: WithScopes(Repo)},
		{Name: "tool2", Meta: WithScopes(Gist)},
		{Name: "tool3", Meta: WithScopes(Repo, Notifications)},
		{Name: "tool4", Meta: nil}, // no scopes
	}

	scopeMap := BuildToolScopeMapFromMeta(tools)
	allRequired := scopeMap.AllRequiredScopes()

	assert.True(t, allRequired.Contains("repo"))
	assert.True(t, allRequired.Contains("gist"))
	assert.True(t, allRequired.Contains("notifications"))
	assert.False(t, allRequired.Contains("user"))
}

func TestToolScopeMapToolsRequiringScope(t *testing.T) {
	tools := []ToolMeta{
		{Name: "tool1", Meta: WithScopes(Repo)},
		{Name: "tool2", Meta: WithScopes(Gist)},
		{Name: "tool3", Meta: WithScopes(Repo, Notifications)},
	}

	scopeMap := BuildToolScopeMapFromMeta(tools)

	repoTools := scopeMap.ToolsRequiringScope("repo")
	assert.ElementsMatch(t, []string{"tool1", "tool3"}, repoTools)

	gistTools := scopeMap.ToolsRequiringScope("gist")
	assert.ElementsMatch(t, []string{"tool2"}, gistTools)

	userTools := scopeMap.ToolsRequiringScope("user")
	assert.Empty(t, userTools)
}

func TestToolScopeMapToolsAcceptingScope(t *testing.T) {
	tools := []ToolMeta{
		{Name: "tool1", Meta: WithScopes(PublicRepo)}, // accepts repo too
		{Name: "tool2", Meta: WithScopes(ReadOrg)},    // accepts write:org and admin:org too
	}

	scopeMap := BuildToolScopeMapFromMeta(tools)

	// public_repo is required but repo is also accepted
	repoAccepting := scopeMap.ToolsAcceptingScope("repo")
	assert.Contains(t, repoAccepting, "tool1")

	// read:org is required but admin:org is also accepted
	adminOrgAccepting := scopeMap.ToolsAcceptingScope("admin:org")
	assert.Contains(t, adminOrgAccepting, "tool2")
}
