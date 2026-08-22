package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
)

func TestPublicRepositoryWriteToolsUsePublicRepoScope(t *testing.T) {
	tests := []struct {
		name string
		tool inventory.ServerTool
	}{
		{name: "add_issue_comment", tool: AddIssueComment(translations.NullTranslationHelper)},
		{name: "issue_write", tool: IssueWrite(translations.NullTranslationHelper)},
		{name: "create_branch", tool: CreateBranch(translations.NullTranslationHelper)},
		{name: "push_files", tool: PushFiles(translations.NullTranslationHelper)},
		{name: "create_pull_request", tool: CreatePullRequest(translations.NullTranslationHelper)},
		{name: "fork_repository", tool: ForkRepository(translations.NullTranslationHelper)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, []string{string(scopes.PublicRepo)}, tt.tool.RequiredScopes)
			assert.ElementsMatch(t, []string{string(scopes.PublicRepo), string(scopes.Repo)}, tt.tool.AcceptedScopes)
		})
	}
}
