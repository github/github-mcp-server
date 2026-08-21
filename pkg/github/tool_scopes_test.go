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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.tool.ScopeAccess.Challenge)
			assert.Empty(t, tt.tool.ScopeAccess.Challenge(tt.arguments, tt.allowed))
			assert.NotEmpty(t, tt.tool.ScopeAccess.Challenge(tt.arguments, tt.disallowed))
			assert.True(t, tt.tool.ScopeAccess.Visible(nil))
		})
	}
}
