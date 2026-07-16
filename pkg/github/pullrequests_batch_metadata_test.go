package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v87/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetPullRequestMetadataBatch(t *testing.T) {
	serverTool := GetPullRequestMetadataBatch(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_pull_request_metadata_batch", tool.Name)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumbers")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumbers"})

	pr42 := &github.PullRequest{
		Number:   github.Ptr(42),
		Title:    github.Ptr("Release prep"),
		State:    github.Ptr("closed"),
		Merged:   github.Ptr(true),
		HTMLURL:  github.Ptr("https://github.com/owner/repo/pull/42"),
		MergedAt: &github.Timestamp{Time: time.Date(2026, time.June, 12, 12, 0, 0, 0, time.UTC)},
		User: &github.User{
			Login: github.Ptr("octocat"),
		},
		Labels: []*github.Label{{Name: github.Ptr("release")}},
	}
	pr18 := &github.PullRequest{
		Number:   github.Ptr(18),
		Title:    github.Ptr("Changelog fix"),
		State:    github.Ptr("closed"),
		Merged:   github.Ptr(true),
		HTMLURL:  github.Ptr("https://github.com/owner/repo/pull/18"),
		MergedAt: &github.Timestamp{Time: time.Date(2026, time.June, 10, 12, 0, 0, 0, time.UTC)},
		User: &github.User{
			Login: github.Ptr("hubot"),
		},
		Labels: []*github.Label{{Name: github.Ptr("docs")}},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]any
		expectError     bool
		expectedErrMsg  string
		lockdownEnabled bool
		validateResult  func(t *testing.T, textContent string)
	}{
		{
			name: "successful metadata batch preserves input order",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/repos/owner/repo/pulls/42":
						mockResponse(t, http.StatusOK, pr42).ServeHTTP(w, r)
					case "/repos/owner/repo/pulls/18":
						mockResponse(t, http.StatusOK, pr18).ServeHTTP(w, r)
					default:
						http.NotFound(w, r)
					}
				},
			}),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": []any{float64(42), float64(18)},
			},
			validateResult: func(t *testing.T, textContent string) {
				var result batchPullRequestMetadataResponse
				require.NoError(t, json.Unmarshal([]byte(textContent), &result))
				assert.Len(t, result.PullRequests, 2)
				assert.Empty(t, result.Errors)
				assert.Equal(t, 42, result.PullRequests[0].Number)
				assert.Equal(t, "Release prep", result.PullRequests[0].Title)
				assert.Equal(t, 18, result.PullRequests[1].Number)
				assert.Equal(t, "Changelog fix", result.PullRequests[1].Title)
			},
		},
		{
			name: "partial failures are returned without failing the batch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/repos/owner/repo/pulls/42":
						mockResponse(t, http.StatusOK, pr42).ServeHTTP(w, r)
					case "/repos/owner/repo/pulls/999":
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message":"Not Found"}`))
					default:
						http.NotFound(w, r)
					}
				},
			}),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": []any{float64(42), float64(999)},
			},
			validateResult: func(t *testing.T, textContent string) {
				var result batchPullRequestMetadataResponse
				require.NoError(t, json.Unmarshal([]byte(textContent), &result))
				assert.Len(t, result.PullRequests, 1)
				assert.Equal(t, 42, result.PullRequests[0].Number)
				assert.Len(t, result.Errors, 1)
				assert.Equal(t, 999, result.Errors[0].PullNumber)
				assert.Contains(t, result.Errors[0].Message, "failed to get pull request")
			},
		},
		{
			name: "duplicate pull numbers are deduplicated before hydration",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: func(w http.ResponseWriter, r *http.Request) {
					mockResponse(t, http.StatusOK, pr42).ServeHTTP(w, r)
				},
			}),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": []any{float64(42), float64(42), float64(42)},
			},
			validateResult: func(t *testing.T, textContent string) {
				var result batchPullRequestMetadataResponse
				require.NoError(t, json.Unmarshal([]byte(textContent), &result))
				assert.Len(t, result.PullRequests, 1)
				assert.Empty(t, result.Errors)
				assert.Equal(t, 42, result.PullRequests[0].Number)
			},
		},
		{
			name: "lockdown enabled still allows collaborator-authored pull requests",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: func(w http.ResponseWriter, r *http.Request) {
					mockResponse(t, http.StatusOK, &github.PullRequest{
						Number:  github.Ptr(7),
						Title:   github.Ptr("Trusted PR"),
						State:   github.Ptr("open"),
						HTMLURL: github.Ptr("https://github.com/owner/repo/pull/7"),
						User:    &github.User{Login: github.Ptr("maintainer")},
					}).ServeHTTP(w, r)
				},
			}),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": []any{float64(7)},
			},
			lockdownEnabled: true,
			validateResult: func(t *testing.T, textContent string) {
				var result batchPullRequestMetadataResponse
				require.NoError(t, json.Unmarshal([]byte(textContent), &result))
				assert.Len(t, result.PullRequests, 1)
				assert.Empty(t, result.Errors)
				assert.Equal(t, 7, result.PullRequests[0].Number)
			},
		},
		{
			name: "lockdown enabled reports restricted pull requests as per-item errors",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: func(w http.ResponseWriter, r *http.Request) {
					mockResponse(t, http.StatusOK, &github.PullRequest{
						Number:  github.Ptr(8),
						Title:   github.Ptr("Untrusted PR"),
						State:   github.Ptr("open"),
						HTMLURL: github.Ptr("https://github.com/owner/repo/pull/8"),
						User:    &github.User{Login: github.Ptr("external-user")},
					}).ServeHTTP(w, r)
				},
			}),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": []any{float64(8)},
			},
			lockdownEnabled: true,
			validateResult: func(t *testing.T, textContent string) {
				var result batchPullRequestMetadataResponse
				require.NoError(t, json.Unmarshal([]byte(textContent), &result))
				assert.Empty(t, result.PullRequests)
				assert.Len(t, result.Errors, 1)
				assert.Equal(t, 8, result.Errors[0].PullNumber)
				assert.Contains(t, result.Errors[0].Message, "restricted by lockdown mode")
			},
		},
		{
			name:         "empty pullNumbers fails validation",
			mockedClient: githubv4mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": []any{},
			},
			expectError:    true,
			expectedErrMsg: "must contain at least one pull request number",
		},
		{
			name:         "oversized pullNumbers fails validation",
			mockedClient: githubv4mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": oversizedPullRequestArgs(maxPullRequestMetadataBatchSize + 1),
			},
			expectError:    true,
			expectedErrMsg: "exceeds the maximum batch size",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			var repoAccessClient *github.Client
			if tc.lockdownEnabled {
				repoAccessClient = mockRESTPermissionServer(t, "read", map[string]string{
					"maintainer":    "write",
					"external-user": "read",
				})
			}
			deps := BaseDeps{
				Client:          client,
				RepoAccessCache: stubRepoAccessCache(repoAccessClient, 5*time.Minute),
				Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": tc.lockdownEnabled}),
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectError {
				require.True(t, result.IsError)
				text := getErrorResult(t, result)
				assert.Contains(t, text.Text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError)
			text := getTextResult(t, result)
			tc.validateResult(t, text.Text)
		})
	}
}

func oversizedPullRequestArgs(count int) []any {
	values := make([]any, 0, count)
	for i := range count {
		values = append(values, float64(i+1))
	}
	return values
}
