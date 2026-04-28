package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListRepoSkills(t *testing.T) {
	t.Parallel()

	serverTool := ListRepoSkills(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_repo_skills", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "list_repo_skills must be read-only")

	treeMock := func(entries ...*gogithub.TreeEntry) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			data, _ := json.Marshal(&gogithub.Tree{Entries: entries})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
		}
	}

	tests := []struct {
		name            string
		args            map[string]any
		handlers        map[string]http.HandlerFunc
		expectToolError bool
		expectErrText   string
		expectSkills    []string // names; URLs are checked structurally
	}{
		{
			name: "missing owner",
			args: map[string]any{"repo": "hello-world"},
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: treeMock(),
			},
			expectToolError: true,
			expectErrText:   "owner",
		},
		{
			name: "missing repo",
			args: map[string]any{"owner": "octocat"},
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: treeMock(),
			},
			expectToolError: true,
			expectErrText:   "repo",
		},
		{
			name: "empty repo returns no skills",
			args: map[string]any{"owner": "octocat", "repo": "hello-world"},
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: treeMock(
					&gogithub.TreeEntry{Path: gogithub.Ptr("README.md"), Type: gogithub.Ptr("blob")},
				),
			},
			expectSkills: []string{},
		},
		{
			name: "discovers across all four conventions",
			args: map[string]any{"owner": "octocat", "repo": "hello-world"},
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: treeMock(
					&gogithub.TreeEntry{Path: gogithub.Ptr("skills/code-review/SKILL.md"), Type: gogithub.Ptr("blob")},
					&gogithub.TreeEntry{Path: gogithub.Ptr("skills/acme/data-tool/SKILL.md"), Type: gogithub.Ptr("blob")},
					&gogithub.TreeEntry{Path: gogithub.Ptr("plugins/my-plugin/skills/lint/SKILL.md"), Type: gogithub.Ptr("blob")},
					&gogithub.TreeEntry{Path: gogithub.Ptr("root-level-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
				),
			},
			expectSkills: []string{"code-review", "data-tool", "lint", "root-level-skill"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gogithub.NewClient(MockHTTPClientWithHandlers(tc.handlers))
			deps := BaseDeps{Client: client}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expectToolError {
				assert.True(t, result.IsError, "expected tool error result")
				if tc.expectErrText != "" {
					textContent := getErrorResult(t, result)
					assert.Contains(t, textContent.Text, tc.expectErrText)
				}
				return
			}

			assert.False(t, result.IsError, "unexpected tool error: %+v", result)

			textContent := getTextResult(t, result)
			var payload struct {
				Owner      string `json:"owner"`
				Repo       string `json:"repo"`
				Skills     []struct {
					Name string `json:"name"`
					URL  string `json:"url"`
				} `json:"skills"`
				TotalCount int `json:"totalCount"`
			}
			require.NoError(t, json.Unmarshal([]byte(textContent.Text), &payload))

			assert.Equal(t, tc.args["owner"], payload.Owner)
			assert.Equal(t, tc.args["repo"], payload.Repo)
			assert.Equal(t, len(tc.expectSkills), payload.TotalCount)
			require.Len(t, payload.Skills, len(tc.expectSkills))

			gotNames := make([]string, 0, len(payload.Skills))
			for _, s := range payload.Skills {
				gotNames = append(gotNames, s.Name)
				// Each URL must match the canonical SkillFileURI shape so the
				// model can pass it straight to resources/read.
				expectedURL := SkillFileURI(payload.Owner, payload.Repo, s.Name, "SKILL.md")
				assert.Equal(t, expectedURL, s.URL, "URL must match SkillFileURI(owner, repo, name, SKILL.md)")
			}
			assert.ElementsMatch(t, tc.expectSkills, gotNames)
		})
	}
}
