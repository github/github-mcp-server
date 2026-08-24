package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPublicRepoWriteToolsDeclareLeastPrivilegeScopes asserts that the public
// repository write tools advertise `public_repo` as their required scope while
// still accepting a full `repo` token (via the scope hierarchy). This enables
// least-privilege, public-only OAuth deployments instead of forcing the broad
// `repo` scope, which also grants private-repository access.
// See https://github.com/github/github-mcp-server/issues/3136
func TestPublicRepoWriteToolsDeclareLeastPrivilegeScopes(t *testing.T) {
	t.Parallel()

	builders := map[string]func(translations.TranslationHelperFunc) *inventory.ServerTool{
		"add_issue_comment":   wrapToolBuilder(AddIssueComment),
		"issue_write":         wrapToolBuilder(IssueWrite),
		"create_branch":       wrapToolBuilder(CreateBranch),
		"push_files":          wrapToolBuilder(PushFiles),
		"create_pull_request": wrapToolBuilder(CreatePullRequest),
		"fork_repository":     wrapToolBuilder(ForkRepository),
	}

	for name, build := range builders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			st := build(translations.NullTranslationHelper)

			require.NotNil(t, st.ScopeAccess.Challenge)
			args := map[string]any(nil)
			wantScopes := []string{string(scopes.PublicRepo)}
			if name == "push_files" {
				args = map[string]any{"files": []any{map[string]any{"path": "README.md"}}}
				wantScopes = append(wantScopes, string(scopes.Workflow))
			}
			assert.Equal(t, wantScopes, st.ScopeAccess.Scopes,
				"%s should expose the least-privilege scope upper bound", name)
			assert.Empty(t, st.ScopeAccess.Challenge(args, []string{string(scopes.PublicRepo)}),
				"%s should accept a public_repo token", name)
			assert.Empty(t, st.ScopeAccess.Challenge(args, []string{string(scopes.Repo)}),
				"%s should accept a parent repo token", name)
			if name == "push_files" {
				workflowArgs := map[string]any{"files": []any{map[string]any{"path": ".github/workflows/ci.yml"}}}
				assert.Equal(t, []string{string(scopes.PublicRepo), string(scopes.Workflow)},
					st.ScopeAccess.Challenge(workflowArgs, []string{string(scopes.PublicRepo)}),
					"push_files should additionally challenge for workflow when needed")
			}
		})
	}
}

// TestPublicRepoWriteToolsVisibleToPublicRepoToken asserts the PAT tool filter
// shows these tools to a public_repo-only token and keeps them visible for a
// full repo token.
func TestPublicRepoWriteToolsVisibleToPublicRepoToken(t *testing.T) {
	t.Parallel()

	publicRepoToken := []string{string(scopes.PublicRepo)}
	repoToken := []string{string(scopes.Repo)}

	filterForPublicToken := CreateToolScopeFilter(publicRepoToken)
	filterForRepoToken := CreateToolScopeFilter(repoToken)

	builders := map[string]func(translations.TranslationHelperFunc) *inventory.ServerTool{
		"add_issue_comment":   wrapToolBuilder(AddIssueComment),
		"issue_write":         wrapToolBuilder(IssueWrite),
		"create_branch":       wrapToolBuilder(CreateBranch),
		"push_files":          wrapToolBuilder(PushFiles),
		"create_pull_request": wrapToolBuilder(CreatePullRequest),
		"fork_repository":     wrapToolBuilder(ForkRepository),
	}

	for name, build := range builders {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			st := build(translations.NullTranslationHelper)

			allowed, err := filterForPublicToken(t.Context(), st)
			require.NoError(t, err)
			assert.True(t, allowed, "%s should be visible with a public_repo-only token", name)

			allowed, err = filterForRepoToken(t.Context(), st)
			require.NoError(t, err)
			assert.True(t, allowed, "%s should remain visible with a full repo token", name)
		})
	}
}

func wrapToolBuilder(fn func(translations.TranslationHelperFunc) inventory.ServerTool) func(translations.TranslationHelperFunc) *inventory.ServerTool {
	return func(t translations.TranslationHelperFunc) *inventory.ServerTool {
		st := fn(t)
		return &st
	}
}
