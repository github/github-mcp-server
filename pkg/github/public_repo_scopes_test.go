package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicRepoContributionToolScopeAccess(t *testing.T) {
	t.Parallel()

	tools := []struct {
		name string
		tool inventory.ServerTool
	}{
		{name: "fork_repository", tool: ForkRepository(translations.NullTranslationHelper)},
		{name: "create_branch", tool: CreateBranch(translations.NullTranslationHelper)},
		{name: "create_pull_request", tool: CreatePullRequest(translations.NullTranslationHelper)},
		{name: "issue_write", tool: IssueWrite(translations.NullTranslationHelper)},
		{name: "add_issue_comment", tool: AddIssueComment(translations.NullTranslationHelper)},
	}

	for _, tt := range tools {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, []string{string(scopes.Repo)}, tt.tool.ScopeAccess.Scopes)
			require.NotNil(t, tt.tool.ScopeAccess.Visible)
			assert.False(t, tt.tool.ScopeAccess.Visible(nil))
			assert.True(t, tt.tool.ScopeAccess.Visible([]string{string(scopes.PublicRepo)}))
			assert.True(t, tt.tool.ScopeAccess.Visible([]string{string(scopes.Repo)}))

			require.NotNil(t, tt.tool.ScopeAccess.Challenge)
			assert.Equal(t, []string{string(scopes.Repo)}, tt.tool.ScopeAccess.Challenge(nil, nil))
			assert.Equal(t, []string{string(scopes.Repo)}, tt.tool.ScopeAccess.Challenge(nil, []string{string(scopes.PublicRepo)}))
			assert.Empty(t, tt.tool.ScopeAccess.Challenge(nil, []string{string(scopes.Repo)}))
		})
	}
}

func TestPublicRepoContributionToolsVisibleToPATs(t *testing.T) {
	t.Parallel()

	tools := []inventory.ServerTool{
		ForkRepository(translations.NullTranslationHelper),
		CreateBranch(translations.NullTranslationHelper),
		PushFiles(translations.NullTranslationHelper),
		CreatePullRequest(translations.NullTranslationHelper),
		IssueWrite(translations.NullTranslationHelper),
		AddIssueComment(translations.NullTranslationHelper),
	}

	tests := []struct {
		name        string
		tokenScopes []string
		wantVisible bool
	}{
		{name: "no scopes"},
		{name: "public_repo", tokenScopes: []string{string(scopes.PublicRepo)}, wantVisible: true},
		{name: "repo", tokenScopes: []string{string(scopes.Repo)}, wantVisible: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := CreateToolScopeFilter(tt.tokenScopes)
			for i := range tools {
				included, err := filter(t.Context(), &tools[i])
				require.NoError(t, err)
				assert.Equal(t, tt.wantVisible, included, tools[i].Tool.Name)
			}
		})
	}
}

func TestPushFilesOAuthScopeChallenges(t *testing.T) {
	t.Parallel()

	tool := PushFiles(translations.NullTranslationHelper)
	regularFiles := map[string]any{
		"files": []any{map[string]any{"path": "README.md"}},
	}
	workflowFiles := map[string]any{
		"files": []any{map[string]any{"path": ".github/workflows/ci.yml"}},
	}

	assert.Equal(t, []string{string(scopes.Repo), string(scopes.Workflow)}, tool.ScopeAccess.Scopes)
	require.NotNil(t, tool.ScopeAccess.Visible)
	assert.True(t, tool.ScopeAccess.Visible([]string{string(scopes.PublicRepo)}))
	assert.True(t, tool.ScopeAccess.Visible([]string{string(scopes.Repo)}))

	assert.Equal(t, []string{string(scopes.Repo)}, tool.ScopeAccess.Challenge(regularFiles, nil))
	assert.Equal(t, []string{string(scopes.Repo)}, tool.ScopeAccess.Challenge(regularFiles, []string{string(scopes.PublicRepo)}))
	assert.Empty(t, tool.ScopeAccess.Challenge(regularFiles, []string{string(scopes.Repo)}))

	assert.Equal(t, []string{string(scopes.Repo), string(scopes.Workflow)}, tool.ScopeAccess.Challenge(workflowFiles, nil))
	assert.Equal(t, []string{string(scopes.Repo), string(scopes.Workflow)}, tool.ScopeAccess.Challenge(workflowFiles, []string{string(scopes.PublicRepo)}))
	assert.Equal(t, []string{string(scopes.Workflow)}, tool.ScopeAccess.Challenge(workflowFiles, []string{string(scopes.Repo)}))
	assert.Equal(t, []string{string(scopes.Repo)}, tool.ScopeAccess.Challenge(workflowFiles, []string{string(scopes.Workflow)}))
	assert.Equal(t, []string{string(scopes.Repo)}, tool.ScopeAccess.Challenge(workflowFiles, []string{string(scopes.PublicRepo), string(scopes.Workflow)}))
	assert.Empty(t, tool.ScopeAccess.Challenge(workflowFiles, []string{string(scopes.Repo), string(scopes.Workflow)}))
}
