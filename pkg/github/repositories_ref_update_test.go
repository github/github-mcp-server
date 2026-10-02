package github

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/stretchr/testify/require"
)

func TestPushFilesRetriesConcurrentRefUpdate(t *testing.T) {
	getRefCalls := 0
	updateRefCalls := 0
	client := mustNewGHClient(t, NewMockedHTTPClient(
		WithRequestMatchHandler(
			GetReposGitRefByOwnerByRepoByRef,
			func(w http.ResponseWriter, _ *http.Request) {
				getRefCalls++
				refSHA := "abc123"
				if getRefCalls >= 3 {
					refSHA = "concurrent999"
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(&github.Reference{
					Ref:    github.Ptr("refs/heads/main"),
					Object: &github.GitObject{SHA: github.Ptr(refSHA)},
				}))
			},
		),
		WithRequestMatchHandler(
			GetReposGitCommitsByOwnerByRepoByCommitSHA,
			func(w http.ResponseWriter, r *http.Request) {
				sha := "abc123"
				treeSHA := "def456"
				if r.URL.Path == "/repos/owner/repo/git/commits/concurrent999" {
					sha = "concurrent999"
					treeSHA = "tree999"
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(&github.Commit{
					SHA:  github.Ptr(sha),
					Tree: &github.Tree{SHA: github.Ptr(treeSHA)},
				}))
			},
		),
		WithRequestMatch(
			PostReposGitTreesByOwnerByRepo,
			&github.Tree{SHA: github.Ptr("ghi789")},
		),
		WithRequestMatch(
			PostReposGitCommitsByOwnerByRepo,
			&github.Commit{SHA: github.Ptr("jkl012")},
		),
		WithRequestMatchHandler(
			PatchReposGitRefsByOwnerByRepoByRef,
			func(w http.ResponseWriter, _ *http.Request) {
				updateRefCalls++
				w.Header().Set("Content-Type", "application/json")
				if updateRefCalls == 1 {
					w.WriteHeader(http.StatusUnprocessableEntity)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]string{
						"message": "Update is not a fast forward",
					}))
					return
				}
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(&github.Reference{
					Ref:    github.Ptr("refs/heads/main"),
					Object: &github.GitObject{SHA: github.Ptr("jkl012")},
				}))
			},
		),
	))

	deps := BaseDeps{Client: client}
	tool := PushFiles(translations.NullTranslationHelper)
	handler := tool.Handler(deps)
	request := createMCPRequest(map[string]any{
		"owner":   "owner",
		"repo":    "repo",
		"branch":  "main",
		"files":   []any{map[string]any{"path": "README.md", "content": "# Hi"}},
		"message": "Update files",
	})
	result, err := handler(ContextWithDeps(t.Context(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Equal(t, 2, updateRefCalls)
	require.Equal(t, 3, getRefCalls)
}
