package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnforcePRAuthorAllowlist_NoAllowlist_PermitsAll(t *testing.T) {
	deps := BaseDeps{}

	result, err := enforcePRAuthorAllowlist(context.Background(), deps, "owner", "repo", 1, nil)

	require.NoError(t, err)
	require.Nil(t, result)
}

func TestEnforcePRAuthorAllowlist_AuthorAllowed(t *testing.T) {
	deps := BaseDeps{allowedPRAuthors: buildPRAuthorAllowlist([]string{"renovate[bot]"})}
	pr := &gogithub.PullRequest{User: &gogithub.User{Login: gogithub.Ptr("renovate[bot]")}}

	result, err := enforcePRAuthorAllowlist(context.Background(), deps, "owner", "repo", 1, pr)

	require.NoError(t, err)
	require.Nil(t, result)
}

func TestEnforcePRAuthorAllowlist_AuthorDenied(t *testing.T) {
	deps := BaseDeps{allowedPRAuthors: buildPRAuthorAllowlist([]string{"renovate[bot]"})}
	pr := &gogithub.PullRequest{User: &gogithub.User{Login: gogithub.Ptr("alice")}}

	result, err := enforcePRAuthorAllowlist(context.Background(), deps, "owner", "repo", 1, pr)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	assert.Contains(t, getErrorResult(t, result).Text, `pull request author "alice" is not in --allowed-pr-authors`)
}

func TestEnforcePRAuthorAllowlist_FetchFailure(t *testing.T) {
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposPullsByOwnerByRepoByPullNumber: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		},
	}))
	deps := BaseDeps{
		Client:           client,
		allowedPRAuthors: buildPRAuthorAllowlist([]string{"renovate[bot]"}),
	}

	result, err := enforcePRAuthorAllowlist(context.Background(), deps, "owner", "repo", 1, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	assert.Contains(t, getErrorResult(t, result).Text, "failed to get pull request")
}

func TestEnforcePRAuthorAllowlist_UsesProvidedPR(t *testing.T) {
	calls := 0
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposPullsByOwnerByRepoByPullNumber: func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.WriteHeader(http.StatusInternalServerError)
		},
	}))
	deps := BaseDeps{
		Client:           client,
		allowedPRAuthors: buildPRAuthorAllowlist([]string{"renovate[bot]"}),
	}
	pr := &gogithub.PullRequest{User: &gogithub.User{Login: gogithub.Ptr("renovate[bot]")}}

	result, err := enforcePRAuthorAllowlist(context.Background(), deps, "owner", "repo", 1, pr)

	require.NoError(t, err)
	require.Nil(t, result)
	assert.Zero(t, calls)
}

func TestMergePullRequest_PRAuthorDenied(t *testing.T) {
	serverTool := MergePullRequest(translations.NullTranslationHelper)
	client := gogithub.NewClient(MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, &gogithub.PullRequest{
			User: &gogithub.User{Login: gogithub.Ptr("alice")},
		}),
		PutReposPullsMergeByOwnerByRepoByPullNumber: func(w http.ResponseWriter, _ *http.Request) {
			t.Fatal("merge endpoint should not be called when PR author is denied")
		},
	}))
	deps := BaseDeps{
		Client:           client,
		allowedPRAuthors: buildPRAuthorAllowlist([]string{"renovate[bot]"}),
	}
	handler := serverTool.Handler(deps)
	request := createMCPRequest(map[string]any{
		"owner":      "owner",
		"repo":       "repo",
		"pullNumber": float64(42),
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, getErrorResult(t, result).Text, `pull request author "alice" is not in --allowed-pr-authors`)
}
