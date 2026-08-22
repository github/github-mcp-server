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

func TestPublicRepoContributionToolsAcceptPublicRepoScope(t *testing.T) {
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

	filter := CreateToolScopeFilter([]string{string(scopes.PublicRepo)})
	for _, tt := range tools {
		t.Run(tt.name, func(t *testing.T) {
			tool := tt.tool
			assert.Equal(t, []string{string(scopes.PublicRepo)}, tool.RequiredScopes)
			assert.ElementsMatch(t, []string{string(scopes.PublicRepo), string(scopes.Repo)}, tool.AcceptedScopes)

			included, err := filter(context.Background(), &tool)
			require.NoError(t, err)
			assert.True(t, included)
		})
	}
}
