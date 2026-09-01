package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MergePullRequestSHAPinning(t *testing.T) {
	serverTool := MergePullRequest(translations.NullTranslationHelper)

	const pinnedSHA = "0123456789abcdef0123456789abcdef01234567"

	mockMergeResult := &github.PullRequestMergeResult{
		SHA:     github.Ptr("merge-result-sha"),
		Merged:  github.Ptr(true),
		Message: github.Ptr("Pull Request successfully merged"),
	}

	t.Run("expected head SHA is sent as REST sha", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PutReposPullsMergeByOwnerByRepoByPullNumber: expectRequestBody(t, map[string]any{
				"merge_method": "merge",
				"sha":          pinnedSHA,
			}).andThen(
				mockResponse(t, http.StatusOK, mockMergeResult),
			),
		}))

		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":           "owner",
			"repo":            "repo",
			"pullNumber":      float64(42),
			"merge_method":    "merge",
			"expectedHeadSha": pinnedSHA,
		})

		result, err := handler(
			ContextWithDeps(context.Background(), deps),
			&request,
		)

		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("head mismatch fails closed without retry", func(t *testing.T) {
		calls := 0

		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PutReposPullsMergeByOwnerByRepoByPullNumber: expectRequestBody(t, map[string]any{
				"merge_method": "merge",
				"sha":          pinnedSHA,
			}).andThen(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(
					`{"message":"Head branch was modified. Review and try the merge again."}`,
				))
			}),
		}))

		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":           "owner",
			"repo":            "repo",
			"pullNumber":      float64(42),
			"merge_method":    "merge",
			"expectedHeadSha": pinnedSHA,
		})

		result, err := handler(
			ContextWithDeps(context.Background(), deps),
			&request,
		)

		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "Head branch was modified")
		assert.Equal(t, 1, calls, "merge must not retry after a head-SHA mismatch")
	})
}
