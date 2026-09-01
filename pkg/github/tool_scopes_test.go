package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionalToolScopeChecks(t *testing.T) {
	tests := []struct {
		name       string
		tool       inventory.ServerTool
		arguments  map[string]any
		allowed    []string
		disallowed []string
	}{
		{
			name:       "issue fields repository route",
			tool:       ListIssueFields(translations.NullTranslationHelper),
			arguments:  map[string]any{"owner": "octo", "repo": "repo"},
			allowed:    []string{"repo"},
			disallowed: []string{"read:org"},
		},
		{
			name:       "issue fields organization route",
			tool:       ListIssueFields(translations.NullTranslationHelper),
			arguments:  map[string]any{"owner": "octo"},
			allowed:    []string{"read:org"},
			disallowed: []string{"repo"},
		},
		{
			name:       "issue types repository route",
			tool:       ListIssueTypes(translations.NullTranslationHelper),
			arguments:  map[string]any{"owner": "octo", "repo": "repo"},
			allowed:    []string{"repo"},
			disallowed: []string{"read:org"},
		},
		{
			name:       "issue types organization route",
			tool:       ListIssueTypes(translations.NullTranslationHelper),
			arguments:  map[string]any{"owner": "octo"},
			allowed:    []string{"admin:org"},
			disallowed: []string{"repo"},
		},
		{
			name:       "ui repository method",
			tool:       UIGet(translations.NullTranslationHelper),
			arguments:  map[string]any{"method": "labels", "owner": "octo", "repo": "repo"},
			allowed:    []string{"repo"},
			disallowed: []string{"read:org"},
		},
		{
			name:       "ui organization method",
			tool:       UIGet(translations.NullTranslationHelper),
			arguments:  map[string]any{"method": "issue_types", "owner": "octo"},
			allowed:    []string{"write:org"},
			disallowed: []string{"repo"},
		},
		{
			name:       "ui reviewers method",
			tool:       UIGet(translations.NullTranslationHelper),
			arguments:  map[string]any{"method": "reviewers", "owner": "octo", "repo": "repo"},
			allowed:    []string{"repo", "read:org"},
			disallowed: []string{"repo"},
		},
		{
			name:       "ui reviewers method without repository defers to validation",
			tool:       UIGet(translations.NullTranslationHelper),
			arguments:  map[string]any{"method": "reviewers", "owner": "octo"},
			allowed:    nil,
			disallowed: nil,
		},
		{
			name:       "ui reviewers method with malformed repository defers to validation",
			tool:       UIGet(translations.NullTranslationHelper),
			arguments:  map[string]any{"method": "reviewers", "owner": "octo", "repo": 123},
			allowed:    nil,
			disallowed: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.tool.ScopeAccess.Challenge)
			assert.True(t, tt.tool.ScopeAccess.Dynamic)
			assert.Equal(t, []string{"repo", "read:org"}, tt.tool.ScopeAccess.Scopes)
			assert.Empty(t, tt.tool.ScopeAccess.Challenge(tt.arguments, tt.allowed))
			if tt.disallowed == nil {
				assert.Empty(t, tt.tool.ScopeAccess.Challenge(tt.arguments, nil))
			} else {
				assert.NotEmpty(t, tt.tool.ScopeAccess.Challenge(tt.arguments, tt.disallowed))
			}
			assert.True(t, tt.tool.ScopeAccess.Visible(nil))
		})
	}
}

func TestDynamicToolScopeMetadataIsExhaustive(t *testing.T) {
	tests := []struct {
		tool      inventory.ServerTool
		maxScopes []string
	}{
		{tool: CreateOrUpdateFile(translations.NullTranslationHelper), maxScopes: []string{"repo", "workflow"}},
		{tool: DeleteFile(translations.NullTranslationHelper), maxScopes: []string{"repo", "workflow"}},
		{tool: PushFiles(translations.NullTranslationHelper), maxScopes: []string{"repo", "workflow"}},
		{tool: ListIssueFields(translations.NullTranslationHelper), maxScopes: []string{"repo", "read:org"}},
		{tool: ListIssueTypes(translations.NullTranslationHelper), maxScopes: []string{"repo", "read:org"}},
		{tool: UIGet(translations.NullTranslationHelper), maxScopes: []string{"repo", "read:org"}},
	}

	for _, tt := range tests {
		t.Run(tt.tool.Tool.Name, func(t *testing.T) {
			assert.True(t, tt.tool.ScopeAccess.Dynamic)
			assert.Equal(t, tt.maxScopes, tt.tool.ScopeAccess.Scopes)
			assert.NotNil(t, tt.tool.ScopeAccess.Challenge)
		})
	}
}
