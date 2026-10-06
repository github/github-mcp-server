package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestReadinessFields(t *testing.T) {
	cases := []struct {
		name, tool string
		fields     []string
		response   string
		graphql    bool
		wantError  string
	}{
		{"list batch", "list", []string{"number", "review_decision", "status_check_rollup"}, `{"data":{"nodes":[{"id":"PR_one","reviewDecision":"APPROVED","commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"SUCCESS"}}}]}},{"id":"PR_two","reviewDecision":null,"commits":{"nodes":[{"commit":{"statusCheckRollup":null}}]}}]}}`, true, ""},
		{"search batch across repos", "search", []string{"number", "review_decision", "status_check_rollup"}, `{"data":{"nodes":[{"id":"PR_one","reviewDecision":"CHANGES_REQUESTED","commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"FAILURE"}}}]}},{"id":"PR_two","reviewDecision":"REVIEW_REQUIRED","commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"PENDING"}}}]}}]}}`, true, ""},
		{"list default", "list", nil, "", false, ""},
		{"search other fields", "search", []string{"number"}, "", false, ""},
		{"list GraphQL error", "list", []string{"review_decision"}, `{"errors":[{"message":"readiness unavailable"}]}`, true, "readiness"},
		{"search missing node", "search", []string{"status_check_rollup"}, `{"data":{"nodes":[{"id":"PR_one","reviewDecision":null,"commits":{"nodes":[{"commit":{"statusCheckRollup":null}}]}},null]}}`, true, "readiness"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gql := &sequencedGraphQLTransport{t: t}
			if tc.graphql {
				gql.responses = []func(capturedGraphQLRequest) (int, string){func(req capturedGraphQLRequest) (int, string) {
					assert.Contains(t, req.Query, "nodes(ids:")
					assert.Contains(t, req.Query, "commits(last: 1)")
					assert.NotContains(t, req.Query, "headRef")
					assert.Equal(t, []any{"PR_one", "PR_two"}, req.Variables["ids"])
					return http.StatusOK, tc.response
				}}
			}
			deps := BaseDeps{GQLClient: githubv4.NewClient(&http.Client{Transport: gql})}
			args := map[string]any{"page": float64(2), "perPage": float64(2)}
			if tc.fields != nil {
				args["fields"] = tc.fields
			}
			var content string
			var isError bool
			if tc.tool == "list" {
				prs := []*github.PullRequest{
					{NodeID: github.Ptr("PR_one"), Number: github.Ptr(1), Title: github.Ptr("First"), State: github.Ptr("open")},
					{NodeID: github.Ptr("PR_two"), Number: github.Ptr(2), Title: github.Ptr("Second"), State: github.Ptr("open")},
				}
				deps.Client = mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
					GetReposPullsByOwnerByRepo: expectQueryParams(t, map[string]string{"page": "2", "per_page": "2"}).andThen(mockResponse(t, http.StatusOK, prs)),
				}))
				args["owner"] = "owner"
				args["repo"] = "repo"
				req := createMCPRequest(args)
				tool := ListPullRequests(translations.NullTranslationHelper)
				result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &req)
				require.NoError(t, err)
				isError = result.IsError
				content = getTextResult(t, result).Text
			} else {
				issues := &github.IssuesSearchResult{Total: github.Ptr(2), IncompleteResults: github.Ptr(false), Issues: []*github.Issue{
					{NodeID: github.Ptr("PR_one"), Number: github.Ptr(1), Title: github.Ptr("First"), State: github.Ptr("open"), RepositoryURL: github.Ptr("https://api.github.com/repos/a/one")},
					{NodeID: github.Ptr("PR_two"), Number: github.Ptr(2), Title: github.Ptr("Second"), State: github.Ptr("open"), RepositoryURL: github.Ptr("https://api.github.com/repos/b/two")},
				}}
				deps.Client = mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
					GetSearchIssues: expectQueryParams(t, map[string]string{"q": "is:pr review", "page": "2", "per_page": "2"}).andThen(mockResponse(t, http.StatusOK, issues)),
				}))
				args["query"] = "review"
				req := createMCPRequest(args)
				tool := SearchPullRequests(translations.NullTranslationHelper)
				result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &req)
				require.NoError(t, err)
				isError = result.IsError
				content = getTextResult(t, result).Text
			}
			if tc.graphql {
				assert.Len(t, gql.calls, 1)
			} else {
				assert.Empty(t, gql.calls)
			}
			if tc.wantError != "" {
				require.True(t, isError)
				assert.Contains(t, strings.ToLower(content), tc.wantError)
				return
			}
			require.False(t, isError)
			var payload any
			require.NoError(t, json.Unmarshal([]byte(content), &payload))
			var items []any
			if tc.tool == "list" {
				items = payload.([]any)
			} else {
				items = payload.(map[string]any)["items"].([]any)
			}
			require.Len(t, items, 2)
			if !tc.graphql {
				return
			}
			first := items[0].(map[string]any)
			second := items[1].(map[string]any)
			assert.Equal(t, float64(1), first["number"])
			assert.Equal(t, float64(2), second["number"])
			if tc.tool == "list" {
				assert.Equal(t, "APPROVED", first["review_decision"])
				assert.Equal(t, "SUCCESS", first["status_check_rollup"])
				assert.Nil(t, second["review_decision"])
				assert.Nil(t, second["status_check_rollup"])
			} else {
				assert.Equal(t, "CHANGES_REQUESTED", first["review_decision"])
				assert.Equal(t, "FAILURE", first["status_check_rollup"])
				assert.Equal(t, "REVIEW_REQUIRED", second["review_decision"])
				assert.Equal(t, "PENDING", second["status_check_rollup"])
			}
		})
	}
}

func TestPullRequestReadinessFieldSchema(t *testing.T) {
	for _, tool := range []struct {
		name   string
		schema *jsonschema.Schema
	}{
		{"list_pull_requests", ListPullRequests(translations.NullTranslationHelper).Tool.InputSchema.(*jsonschema.Schema)},
		{"search_pull_requests", SearchPullRequests(translations.NullTranslationHelper).Tool.InputSchema.(*jsonschema.Schema)},
	} {
		field := tool.schema.Properties["fields"]
		require.NotNil(t, field)
		require.NotNil(t, field.Items)
		assert.Contains(t, field.Items.Enum, "review_decision", tool.name)
		assert.Contains(t, field.Items.Enum, "status_check_rollup", tool.name)
	}
}

func TestFetchPullRequestReadinessSelectedFields(t *testing.T) {
	cases := []struct {
		name          string
		fields        []string
		response      string
		review        any
		checks        any
		includeReview bool
		includeChecks bool
	}{
		{
			name:          "review only",
			fields:        []string{"review_decision"},
			response:      `{"data":{"nodes":[{"id":"PR_one","reviewDecision":"APPROVED"}]}}`,
			review:        "APPROVED",
			includeReview: true,
		},
		{
			name:          "checks only from latest commit",
			fields:        []string{"status_check_rollup"},
			response:      `{"data":{"nodes":[{"id":"PR_one","commits":{"nodes":[{"commit":{"statusCheckRollup":{"state":"SUCCESS"}}}]}}]}}`,
			checks:        "SUCCESS",
			includeChecks: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			transport := &sequencedGraphQLTransport{t: t, responses: []func(capturedGraphQLRequest) (int, string){
				func(req capturedGraphQLRequest) (int, string) {
					assert.Equal(t, tc.includeReview, req.Variables["includeReview"])
					assert.Equal(t, tc.includeChecks, req.Variables["includeChecks"])
					assert.Contains(t, req.Query, "@include(if: $includeReview)")
					assert.Contains(t, req.Query, "@include(if: $includeChecks)")
					return http.StatusOK, tc.response
				},
			}}
			deps := BaseDeps{GQLClient: githubv4.NewClient(&http.Client{Transport: transport})}
			values, err := fetchPullRequestReadiness(context.Background(), deps, []string{"PR_one"}, tc.fields)
			require.NoError(t, err)
			require.Len(t, transport.calls, 1)
			require.Contains(t, values, "PR_one")
			if tc.review == nil {
				assert.Nil(t, values["PR_one"].ReviewDecision)
			} else {
				assert.Equal(t, tc.review, *values["PR_one"].ReviewDecision)
			}
			if tc.checks == nil {
				assert.Nil(t, values["PR_one"].StatusCheckRollup)
			} else {
				assert.Equal(t, tc.checks, *values["PR_one"].StatusCheckRollup)
			}
		})
	}
}

func TestFetchPullRequestReadinessNoResultsAndMissingID(t *testing.T) {
	deps := BaseDeps{}
	values, err := fetchPullRequestReadiness(context.Background(), deps, nil, []string{"review_decision"})
	require.NoError(t, err)
	assert.Empty(t, values)
	_, err = fetchPullRequestReadiness(context.Background(), deps, []string{""}, []string{"review_decision"})
	require.ErrorContains(t, err, "node ID")
}
