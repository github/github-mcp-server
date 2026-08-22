package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicRepoContributionToolScopes(t *testing.T) {
	t.Parallel()

	tools := []struct {
		name string
		tool inventory.ServerTool
	}{
		{name: "fork_repository", tool: ForkRepository(translations.NullTranslationHelper)},
		{name: "create_branch", tool: CreateBranch(translations.NullTranslationHelper)},
		{name: "push_files", tool: PushFiles(translations.NullTranslationHelper)},
		{name: "create_pull_request", tool: CreatePullRequest(translations.NullTranslationHelper)},
		{name: "issue_write", tool: IssueWrite(translations.NullTranslationHelper)},
		{name: "add_issue_comment", tool: AddIssueComment(translations.NullTranslationHelper)},
	}

	for _, tt := range tools {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.tool.ScopeAccess.Visible([]string{string(scopes.PublicRepo)}))
			assert.True(t, tt.tool.ScopeAccess.Visible([]string{string(scopes.Repo)}))

			for _, activeScopes := range [][]string{
				{string(scopes.PublicRepo)},
				{string(scopes.Repo)},
			} {
				assert.Empty(t, tt.tool.ScopeAccess.Challenge(nil, activeScopes))
			}

			if tt.name != "push_files" {
				assert.Equal(t, []string{string(scopes.PublicRepo)}, tt.tool.ScopeAccess.Scopes)
				assert.Equal(t, []string{string(scopes.PublicRepo)}, tt.tool.ScopeAccess.Challenge(nil, nil))
			}
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

	for _, tokenScope := range []scopes.Scope{scopes.PublicRepo, scopes.Repo} {
		t.Run(string(tokenScope), func(t *testing.T) {
			filter := CreateToolScopeFilter([]string{string(tokenScope)})
			for i := range tools {
				included, err := filter(context.Background(), &tools[i])
				require.NoError(t, err)
				assert.True(t, included, "%s should be visible", tools[i].Tool.Name)
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

	assert.Equal(t, []string{string(scopes.PublicRepo), string(scopes.Workflow)}, tool.ScopeAccess.Scopes)
	assert.Equal(t, []string{string(scopes.PublicRepo)}, tool.ScopeAccess.Challenge(regularFiles, nil))
	assert.Equal(t, []string{string(scopes.PublicRepo), string(scopes.Workflow)}, tool.ScopeAccess.Challenge(workflowFiles, nil))
	assert.Equal(t, []string{string(scopes.PublicRepo), string(scopes.Workflow)}, tool.ScopeAccess.Challenge(workflowFiles, []string{string(scopes.PublicRepo)}))
	assert.Empty(t, tool.ScopeAccess.Challenge(regularFiles, []string{string(scopes.PublicRepo)}))
	assert.Empty(t, tool.ScopeAccess.Challenge(regularFiles, []string{string(scopes.Repo)}))
	assert.Empty(t, tool.ScopeAccess.Challenge(workflowFiles, []string{string(scopes.PublicRepo), string(scopes.Workflow)}))
	assert.Empty(t, tool.ScopeAccess.Challenge(workflowFiles, []string{string(scopes.Repo), string(scopes.Workflow)}))
}
