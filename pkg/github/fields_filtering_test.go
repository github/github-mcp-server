package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- list_commits ---------------------------------------------------------

func Test_LegacyListCommits_Definition(t *testing.T) {
	serverTool := LegacyListCommits(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))
	require.Equal(t, []string{FeatureFlagFieldsParam}, serverTool.FeatureFlagDisable)

	assert.Equal(t, "list_commits", tool.Name)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.NotContains(t, schema.Properties, "fields")
}

func mockListCommits() []*github.RepositoryCommit {
	return []*github.RepositoryCommit{
		{
			SHA:     github.Ptr("abc123def456"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/commit/abc123def456"),
			Commit: &github.Commit{
				Message: github.Ptr("First commit with a reasonably long message to add bytes"),
				Author: &github.CommitAuthor{
					Name:  github.Ptr("Test User"),
					Email: github.Ptr("test@example.com"),
				},
			},
			Author: &github.User{Login: github.Ptr("testuser")},
		},
	}
}

func Test_ListCommits_FieldFiltering(t *testing.T) {
	serverTool := ListCommits(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposCommitsByOwnerByRepo: mockResponse(t, http.StatusOK, mockListCommits()),
	}))
	deps := BaseDeps{Client: client}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":  "owner",
		"repo":   "repo",
		"fields": []any{"sha"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &items))
	require.Len(t, items, 1)
	require.Len(t, items[0], 1)
	assert.Contains(t, items[0], "sha")
	assert.NotContains(t, textContent.Text, "html_url")
	assert.NotContains(t, textContent.Text, "commit")
}

func Test_ListCommits_FieldsTelemetry(t *testing.T) {
	serverTool := ListCommits(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposCommitsByOwnerByRepo: mockResponse(t, http.StatusOK, mockListCommits()),
	}))

	assertFieldsTelemetry(t, serverTool, client, "list_commits",
		map[string]any{"owner": "owner", "repo": "repo", "fields": []any{"sha"}},
		map[string]any{"owner": "owner", "repo": "repo"})
}

// --- list_releases --------------------------------------------------------

func Test_LegacyListReleases_Definition(t *testing.T) {
	serverTool := LegacyListReleases(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))
	require.Equal(t, []string{FeatureFlagFieldsParam}, serverTool.FeatureFlagDisable)

	assert.Equal(t, "list_releases", tool.Name)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.NotContains(t, schema.Properties, "fields")
}

func mockListReleases() []*github.RepositoryRelease {
	return []*github.RepositoryRelease{
		{
			ID:      1,
			TagName: "v1.0.0",
			Name:    github.Ptr("First Release"),
			Body:    github.Ptr("Release notes with a reasonably long body to add bytes"),
			HTMLURL: "https://github.com/owner/repo/releases/tag/v1.0.0",
		},
	}
}

func Test_ListReleases_FieldFiltering(t *testing.T) {
	serverTool := ListReleases(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposReleasesByOwnerByRepo: mockResponse(t, http.StatusOK, mockListReleases()),
	}))
	deps := BaseDeps{Client: client}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":  "owner",
		"repo":   "repo",
		"fields": []any{"tag_name", "name"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &items))
	require.Len(t, items, 1)
	require.Len(t, items[0], 2)
	assert.Contains(t, items[0], "tag_name")
	assert.Contains(t, items[0], "name")
	assert.NotContains(t, textContent.Text, "body")
	assert.NotContains(t, textContent.Text, "html_url")
}

func Test_ListReleases_FieldsTelemetry(t *testing.T) {
	serverTool := ListReleases(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposReleasesByOwnerByRepo: mockResponse(t, http.StatusOK, mockListReleases()),
	}))

	assertFieldsTelemetry(t, serverTool, client, "list_releases",
		map[string]any{"owner": "owner", "repo": "repo", "fields": []any{"tag_name"}},
		map[string]any{"owner": "owner", "repo": "repo"})
}

// --- list_pull_requests ---------------------------------------------------

func Test_LegacyListPullRequests_Definition(t *testing.T) {
	serverTool := LegacyListPullRequests(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))
	require.Equal(t, []string{FeatureFlagFieldsParam}, serverTool.FeatureFlagDisable)

	assert.Equal(t, "list_pull_requests", tool.Name)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.NotContains(t, schema.Properties, "fields")
}

func mockListPullRequests() []*github.PullRequest {
	return []*github.PullRequest{
		{
			Number:  github.Ptr(42),
			Title:   github.Ptr("First PR"),
			Body:    github.Ptr("PR body with a reasonably long description to add bytes"),
			State:   github.Ptr("open"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42"),
			User:    &github.User{Login: github.Ptr("user1")},
		},
	}
}

func Test_ListPullRequests_FieldFiltering(t *testing.T) {
	serverTool := ListPullRequests(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposPullsByOwnerByRepo: mockResponse(t, http.StatusOK, mockListPullRequests()),
	}))
	deps := BaseDeps{Client: client}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":  "owner",
		"repo":   "repo",
		"fields": []any{"number", "title"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	var items []map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &items))
	require.Len(t, items, 1)
	require.Len(t, items[0], 2)
	assert.Contains(t, items[0], "number")
	assert.Contains(t, items[0], "title")
	assert.NotContains(t, textContent.Text, "html_url")
	assert.NotContains(t, textContent.Text, "body")
}

func Test_ListPullRequests_FieldsTelemetry(t *testing.T) {
	serverTool := ListPullRequests(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposPullsByOwnerByRepo: mockResponse(t, http.StatusOK, mockListPullRequests()),
	}))

	assertFieldsTelemetry(t, serverTool, client, "list_pull_requests",
		map[string]any{"owner": "owner", "repo": "repo", "fields": []any{"number"}},
		map[string]any{"owner": "owner", "repo": "repo"})
}

// --- search_pull_requests -------------------------------------------------

func Test_LegacySearchPullRequests_Definition(t *testing.T) {
	serverTool := LegacySearchPullRequests(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))
	require.Equal(t, []string{FeatureFlagFieldsParam}, serverTool.FeatureFlagDisable)

	assert.Equal(t, "search_pull_requests", tool.Name)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.NotContains(t, schema.Properties, "fields")
}

// mockIssueSearchResult returns a single-item issues search result. It is used
// for both search_pull_requests and search_issues since both hit the REST
// issues search endpoint. Issues intentionally omit NodeID so search_issues
// does not attempt the follow-up GraphQL field-values enrichment.
func mockIssueSearchResult() *github.IssuesSearchResult {
	return &github.IssuesSearchResult{
		Total:             github.Ptr(1),
		IncompleteResults: github.Ptr(false),
		Issues: []*github.Issue{
			{
				Number:  github.Ptr(42),
				Title:   github.Ptr("A result"),
				Body:    github.Ptr("Body with a reasonably long description to add bytes"),
				State:   github.Ptr("open"),
				HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42"),
				User:    &github.User{Login: github.Ptr("user1")},
			},
		},
	}
}

func Test_SearchPullRequests_FieldFiltering(t *testing.T) {
	serverTool := SearchPullRequests(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetSearchIssues: mockResponse(t, http.StatusOK, mockIssueSearchResult()),
	}))
	deps := BaseDeps{Client: client}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"query":  "fix",
		"fields": []any{"number", "title"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	assertSearchWrapperFiltered(t, textContent.Text)
}

func Test_SearchPullRequests_FieldsTelemetry(t *testing.T) {
	serverTool := SearchPullRequests(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetSearchIssues: mockResponse(t, http.StatusOK, mockIssueSearchResult()),
	}))

	assertFieldsTelemetry(t, serverTool, client, "search_pull_requests",
		map[string]any{"query": "fix", "fields": []any{"number"}},
		map[string]any{"query": "fix"})
}

// --- search_issues --------------------------------------------------------

func Test_LegacySearchIssues_Definition(t *testing.T) {
	serverTool := LegacySearchIssues(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))
	require.Equal(t, []string{FeatureFlagFieldsParam}, serverTool.FeatureFlagDisable)

	assert.Equal(t, "search_issues", tool.Name)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.NotContains(t, schema.Properties, "fields")
}

func Test_SearchIssues_FieldFiltering(t *testing.T) {
	serverTool := SearchIssues(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetSearchIssues: mockResponse(t, http.StatusOK, mockIssueSearchResult()),
	}))
	deps := BaseDeps{Client: client}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"query":  "bug",
		"fields": []any{"number", "title"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	assertSearchWrapperFiltered(t, textContent.Text)
}

func Test_SearchIssues_FieldsTelemetry(t *testing.T) {
	serverTool := SearchIssues(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetSearchIssues: mockResponse(t, http.StatusOK, mockIssueSearchResult()),
	}))

	assertFieldsTelemetry(t, serverTool, client, "search_issues",
		map[string]any{"query": "bug", "fields": []any{"number"}},
		map[string]any{"query": "bug"})
}

// --- list_issues (GraphQL) ------------------------------------------------

func Test_LegacyListIssues_Definition(t *testing.T) {
	serverTool := LegacyListIssues(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))
	require.Equal(t, []string{FeatureFlagFieldsParam}, serverTool.FeatureFlagDisable)

	assert.Equal(t, "list_issues", tool.Name)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.NotContains(t, schema.Properties, "fields")
}

// listIssuesFieldsQuery and listIssuesFieldsVars mirror the exact GraphQL query
// and variables list_issues issues for owner/repo with default parameters (no
// labels, no since). They must stay in sync with the query built in
// getIssueQueryType; see Test_ListIssues for the canonical copies.
const listIssuesFieldsFieldValuesSelection = "issueFieldValues(first: 25){nodes{__typename,... on IssueFieldDateValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldNumberValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},valueNumber: value},... on IssueFieldSingleSelectValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldTextValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value}}}"

const listIssuesFieldsQuery = "query($after:String$direction:OrderDirection!$first:Int!$issueFieldValues:[IssueFieldValueFilter!]!$orderBy:IssueOrderField!$owner:String!$repo:String!$states:[IssueState!]!){repository(owner: $owner, name: $repo){issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {issueFieldValues: $issueFieldValues}){nodes{number,title,body,state,databaseId,author{login},createdAt,updatedAt,labels(first: 100){nodes{name,id,description}},comments{totalCount}," + listIssuesFieldsFieldValuesSelection + "},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount},isPrivate}}"

func listIssuesFieldsMockClient() *http.Client {
	vars := map[string]any{
		"owner":            "owner",
		"repo":             "repo",
		"states":           []any{"OPEN", "CLOSED"},
		"orderBy":          "CREATED_AT",
		"direction":        "DESC",
		"first":            float64(30),
		"after":            (*string)(nil),
		"issueFieldValues": []any{},
	}
	response := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issues": map[string]any{
				"nodes": []map[string]any{
					{
						"number":           123,
						"title":            "First Issue",
						"body":             "This is a reasonably long issue body to add bytes",
						"state":            "OPEN",
						"databaseId":       1001,
						"createdAt":        "2023-01-01T00:00:00Z",
						"updatedAt":        "2023-01-01T00:00:00Z",
						"author":           map[string]any{"login": "user1"},
						"labels":           map[string]any{"nodes": []map[string]any{}},
						"comments":         map[string]any{"totalCount": 1},
						"issueFieldValues": map[string]any{"nodes": []map[string]any{}},
					},
				},
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 1,
			},
			"isPrivate": false,
		},
	})
	matcher := githubv4mock.NewQueryMatcher(listIssuesFieldsQuery, vars, response)
	return githubv4mock.NewMockedHTTPClient(matcher)
}

func Test_ListIssues_FieldFiltering(t *testing.T) {
	serverTool := ListIssues(translations.NullTranslationHelper)
	deps := BaseDeps{GQLClient: githubv4.NewClient(listIssuesFieldsMockClient())}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":  "owner",
		"repo":   "repo",
		"fields": []any{"number", "title"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)

	// The wrapper metadata is preserved while each issue is reduced to the
	// requested fields only.
	var returned struct {
		Issues     []map[string]any `json:"issues"`
		TotalCount int              `json:"totalCount"`
		PageInfo   map[string]any   `json:"pageInfo"`
	}
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &returned))
	assert.Equal(t, 1, returned.TotalCount)
	require.NotNil(t, returned.PageInfo)
	require.Len(t, returned.Issues, 1)
	require.Len(t, returned.Issues[0], 2)
	assert.Contains(t, returned.Issues[0], "number")
	assert.Contains(t, returned.Issues[0], "title")
	assert.NotContains(t, textContent.Text, "\"body\"")
}

func Test_ListIssues_FieldsTelemetry(t *testing.T) {
	serverTool := ListIssues(translations.NullTranslationHelper)

	t.Run("filtered call records savings", func(t *testing.T) {
		deps, rec := depsWithRecordingMetrics(t, BaseDeps{GQLClient: githubv4.NewClient(listIssuesFieldsMockClient())})
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":  "owner",
			"repo":   "repo",
			"fields": []any{"number"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		assertFilteredCounters(t, rec, "list_issues")
	})

	t.Run("unfiltered call records adoption only", func(t *testing.T) {
		deps, rec := depsWithRecordingMetrics(t, BaseDeps{GQLClient: githubv4.NewClient(listIssuesFieldsMockClient())})
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		call, ok := rec.increment(metricFieldsToolCall)
		require.True(t, ok)
		assert.Equal(t, "false", call.tags["filtered"])
		_, ok = rec.counter(metricFieldsBytesFull)
		assert.False(t, ok, "no byte counters when not filtered")
	})
}

// --- shared assertion helpers ---------------------------------------------

// assertSearchWrapperFiltered asserts that a filtered search response preserves
// the total_count / incomplete_results wrapper while reducing each item to the
// requested number/title fields only.
func assertSearchWrapperFiltered(t *testing.T, text string) {
	t.Helper()
	var returned struct {
		TotalCount        int              `json:"total_count"`
		IncompleteResults bool             `json:"incomplete_results"`
		Items             []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &returned))
	assert.Equal(t, 1, returned.TotalCount)
	require.Len(t, returned.Items, 1)
	require.Len(t, returned.Items[0], 2)
	assert.Contains(t, returned.Items[0], "number")
	assert.Contains(t, returned.Items[0], "title")
	assert.NotContains(t, text, "html_url")
	assert.NotContains(t, text, "\"body\"")
}

// assertFilteredCounters asserts the full set of counters emitted for a filtered
// call: an increment tagged filtered=true plus positive byte counters where
// full > sent.
func assertFilteredCounters(t *testing.T, rec *recordingMetrics, tool string) {
	t.Helper()
	call, ok := rec.increment(metricFieldsToolCall)
	require.True(t, ok)
	assert.Equal(t, tool, call.tags["tool"])
	assert.Equal(t, "true", call.tags["filtered"])

	full, ok := rec.counter(metricFieldsBytesFull)
	require.True(t, ok)
	sent, ok := rec.counter(metricFieldsBytesSent)
	require.True(t, ok)
	assert.Greater(t, full.value, sent.value, "filtering should remove bytes")
}

// assertFieldsTelemetry runs a filtered and an unfiltered call against the given
// tool and asserts the expected adoption and savings telemetry for each.
func assertFieldsTelemetry(t *testing.T, serverTool inventory.ServerTool, client *github.Client, tool string, filteredArgs, unfilteredArgs map[string]any) {
	t.Helper()

	t.Run("filtered call records savings", func(t *testing.T) {
		deps, rec := depsWithRecordingMetrics(t, BaseDeps{Client: client})
		handler := serverTool.Handler(deps)

		request := createMCPRequest(filteredArgs)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		assertFilteredCounters(t, rec, tool)
	})

	t.Run("unfiltered call records adoption only", func(t *testing.T) {
		deps, rec := depsWithRecordingMetrics(t, BaseDeps{Client: client})
		handler := serverTool.Handler(deps)

		request := createMCPRequest(unfilteredArgs)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		call, ok := rec.increment(metricFieldsToolCall)
		require.True(t, ok)
		assert.Equal(t, "false", call.tags["filtered"])
		_, ok = rec.counter(metricFieldsBytesFull)
		assert.False(t, ok, "no byte counters when not filtered")
	})
}
