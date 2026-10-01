package github

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type projectedOutputContractCase struct {
	name       string
	tool       inventory.ServerTool
	deps       func() ToolDependencies
	args       map[string]any
	itemsField string
	fullFields []string
	projection []string
}

func TestProjectedOutputsMatchDeclaredSchemas(t *testing.T) {
	for _, tc := range projectedOutputContractCases(t, false) {
		t.Run(tc.name+"/full", func(t *testing.T) {
			result := callRegisteredTool(t, tc.tool, tc.deps(), tc.args)
			requireStructuredJSONAgreement(t, tc.tool, result)

			items := projectedStructuredItems(t, result.StructuredContent, tc.itemsField)
			require.Len(t, items, 1)
			item, ok := items[0].(map[string]any)
			require.True(t, ok)
			for _, field := range tc.fullFields {
				assert.Contains(t, item, field)
			}
			assert.NotContains(t, item, "node_id")
			assert.NotContains(t, item, "issue_field_values")
		})

		t.Run(tc.name+"/projection", func(t *testing.T) {
			args := cloneArguments(tc.args)
			fields := make([]any, len(tc.projection))
			for i, field := range tc.projection {
				fields[i] = field
			}
			args["fields"] = fields

			tool := projectedOutputToolByName(tc.name)
			result := callRegisteredTool(t, tool, tc.deps(), args)
			requireStructuredJSONAgreement(t, tool, result)

			items := projectedStructuredItems(t, result.StructuredContent, tc.itemsField)
			require.Len(t, items, 1)
			item, ok := items[0].(map[string]any)
			require.True(t, ok)
			require.Len(t, item, len(tc.projection))
			for _, field := range tc.projection {
				assert.Contains(t, item, field)
			}
		})
	}
}

func TestProjectedOutputsValidateEmptyResults(t *testing.T) {
	for _, tc := range projectedOutputContractCases(t, true) {
		t.Run(tc.name, func(t *testing.T) {
			result := callRegisteredTool(t, tc.tool, tc.deps(), tc.args)
			requireStructuredJSONAgreement(t, tc.tool, result)
			assert.Empty(t, projectedStructuredItems(t, result.StructuredContent, tc.itemsField))
		})
	}
}

func TestProjectedOutputSchemasMatchSelectableFields(t *testing.T) {
	tests := []struct {
		name       string
		tool       inventory.ServerTool
		itemsField string
		fields     []any
	}{
		{name: "list_issues", tool: ListIssues(translations.NullTranslationHelper), itemsField: "issues", fields: listIssuesItemFieldEnum},
		{name: "list_pull_requests", tool: ListPullRequests(translations.NullTranslationHelper), fields: listPullRequestsItemFieldEnum},
		{name: "search_issues", tool: SearchIssues(translations.NullTranslationHelper), itemsField: "items", fields: searchIssuesItemFieldEnum},
		{name: "search_pull_requests", tool: SearchPullRequests(translations.NullTranslationHelper), itemsField: "items", fields: searchPullRequestsItemFieldEnum},
		{name: "search_code", tool: SearchCode(translations.NullTranslationHelper), itemsField: "items", fields: codeSearchItemFieldEnum},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schema := tc.tool.Tool.OutputSchema.(*jsonschema.Schema)
			itemSchema := schema.Items
			if tc.itemsField != "" {
				itemSchema = schema.Properties[tc.itemsField].Items
			}
			require.NotNil(t, itemSchema)

			actual := make([]string, 0, len(itemSchema.Properties))
			for field := range itemSchema.Properties {
				actual = append(actual, field)
			}
			expected := make([]string, 0, len(tc.fields))
			for _, field := range tc.fields {
				expected = append(expected, field.(string))
			}
			assert.ElementsMatch(t, expected, actual)
		})
	}
}

func TestListIssuesStructuredPagination(t *testing.T) {
	tool := ListIssues(translations.NullTranslationHelper)
	deps := BaseDeps{
		GQLClient: githubv4.NewClient(projectedListIssuesHTTPClient(t, false, 1, "cursor-1", map[string]any{
			"hasNextPage":     true,
			"hasPreviousPage": true,
			"startCursor":     "cursor-2",
			"endCursor":       "cursor-3",
		})),
		Obsv: stubExporters(),
	}
	result := callRegisteredTool(t, tool, deps, map[string]any{
		"owner":   "owner",
		"repo":    "repo",
		"perPage": 1,
		"after":   "cursor-1",
	})
	requireStructuredJSONAgreement(t, tool, result)

	output, ok := result.StructuredContent.(map[string]any)
	require.True(t, ok)
	pageInfo, ok := output["pageInfo"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, pageInfo["hasNextPage"])
	assert.Equal(t, true, pageInfo["hasPreviousPage"])
	assert.Equal(t, "cursor-3", pageInfo["endCursor"])
}

func TestProjectedOutputToolErrorsHaveNoStructuredContent(t *testing.T) {
	clientErrorDeps := func() ToolDependencies {
		return stubDeps{
			clientFn: stubClientFnErr("client unavailable"),
			obsv:     stubExporters(),
		}
	}
	tests := []struct {
		name string
		tool inventory.ServerTool
		deps ToolDependencies
		args map[string]any
	}{
		{
			name: "list_issues",
			tool: ListIssues(translations.NullTranslationHelper),
			deps: stubDeps{gqlClientFn: stubGQLClientFnErr("client unavailable"), obsv: stubExporters()},
			args: map[string]any{"owner": "owner", "repo": "repo"},
		},
		{
			name: "list_pull_requests",
			tool: ListPullRequests(translations.NullTranslationHelper),
			deps: clientErrorDeps(),
			args: map[string]any{"owner": "owner", "repo": "repo"},
		},
		{
			name: "search_issues",
			tool: SearchIssues(translations.NullTranslationHelper),
			deps: clientErrorDeps(),
			args: map[string]any{"query": "bug"},
		},
		{
			name: "search_pull_requests",
			tool: SearchPullRequests(translations.NullTranslationHelper),
			deps: clientErrorDeps(),
			args: map[string]any{"query": "bug"},
		},
		{
			name: "search_code",
			tool: SearchCode(translations.NullTranslationHelper),
			deps: clientErrorDeps(),
			args: map[string]any{"query": "symbol"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := callRegisteredTool(t, tc.tool, tc.deps, tc.args)
			require.True(t, result.IsError)
			assert.Nil(t, result.StructuredContent)
			assert.Contains(t, getErrorResult(t, result).Text, "client unavailable")
		})
	}
}

func TestProjectedListOutputsCSVAgreement(t *testing.T) {
	tests := []struct {
		name string
		tool inventory.ServerTool
		deps ToolDependencies
		args map[string]any
	}{
		{
			name: "list_issues",
			tool: withCSVOutput([]inventory.ServerTool{ListIssues(translations.NullTranslationHelper)})[0],
			deps: projectedCSVOutputDeps{BaseDeps: BaseDeps{
				GQLClient: githubv4.NewClient(projectedListIssuesHTTPClient(t, false, 30, "", nil)),
				Obsv:      stubExporters(),
			}},
			args: map[string]any{"owner": "owner", "repo": "repo", "fields": []any{"number", "title"}},
		},
		{
			name: "list_pull_requests",
			tool: withCSVOutput([]inventory.ServerTool{ListPullRequests(translations.NullTranslationHelper)})[0],
			deps: projectedCSVOutputDeps{BaseDeps: BaseDeps{
				Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
					GetReposPullsByOwnerByRepo: mockResponse(t, http.StatusOK, projectedPullRequests(false)),
				})),
				Obsv: stubExporters(),
			}},
			args: map[string]any{"owner": "owner", "repo": "repo", "fields": []any{"number", "title"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := callRegisteredTool(t, tc.tool, tc.deps, tc.args)
			require.False(t, result.IsError)
			require.NotNil(t, result.StructuredContent)

			schema := tc.tool.Tool.OutputSchema.(*jsonschema.Schema)
			resolved, err := schema.Resolve(nil)
			require.NoError(t, err)
			require.NoError(t, resolved.Validate(result.StructuredContent))

			structuredJSON, err := json.Marshal(result.StructuredContent)
			require.NoError(t, err)
			expectedCSV, err := jsonTextToCSV(string(structuredJSON))
			require.NoError(t, err)
			csvContent, ok := result.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.Equal(t, expectedCSV, csvContent.Text)
		})
	}
}

type projectedCSVOutputDeps struct {
	BaseDeps
}

func (projectedCSVOutputDeps) IsFeatureEnabled(_ context.Context, flag string) bool {
	return flag == FeatureFlagCSVOutput
}

func projectedOutputContractCases(t *testing.T, empty bool) []projectedOutputContractCase {
	t.Helper()
	return []projectedOutputContractCase{
		{
			name: "list_issues",
			tool: ListIssues(translations.NullTranslationHelper),
			deps: func() ToolDependencies {
				return BaseDeps{
					GQLClient: githubv4.NewClient(projectedListIssuesHTTPClient(t, empty, 30, "", nil)),
					Obsv:      stubExporters(),
				}
			},
			args:       map[string]any{"owner": "owner", "repo": "repo"},
			itemsField: "issues",
			fullFields: []string{"number", "title"},
			projection: []string{"number", "title"},
		},
		{
			name: "list_pull_requests",
			tool: ListPullRequests(translations.NullTranslationHelper),
			deps: func() ToolDependencies {
				return BaseDeps{
					Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
						GetReposPullsByOwnerByRepo: mockResponse(t, http.StatusOK, projectedPullRequests(empty)),
					})),
					Obsv: stubExporters(),
				}
			},
			args:       map[string]any{"owner": "owner", "repo": "repo"},
			fullFields: []string{"number", "title"},
			projection: []string{"number", "draft"},
		},
		{
			name: "search_issues",
			tool: SearchIssues(translations.NullTranslationHelper),
			deps: func() ToolDependencies {
				return BaseDeps{
					Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
						GetSearchIssues: mockResponse(t, http.StatusOK, projectedIssueSearchResult(empty)),
					})),
					Obsv: stubExporters(),
				}
			},
			args:       map[string]any{"query": "bug"},
			itemsField: "items",
			fullFields: []string{"number", "title"},
			projection: []string{"number", "draft"},
		},
		{
			name: "search_pull_requests",
			tool: SearchPullRequests(translations.NullTranslationHelper),
			deps: func() ToolDependencies {
				return BaseDeps{
					Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
						GetSearchIssues: mockResponse(t, http.StatusOK, projectedIssueSearchResult(empty)),
					})),
					Obsv: stubExporters(),
				}
			},
			args:       map[string]any{"query": "bug"},
			itemsField: "items",
			fullFields: []string{"number", "title"},
			projection: []string{"number", "locked"},
		},
		{
			name: "search_code",
			tool: SearchCode(translations.NullTranslationHelper),
			deps: func() ToolDependencies {
				return BaseDeps{
					Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
						GetSearchCode: mockResponse(t, http.StatusOK, projectedCodeSearchResult(empty)),
					})),
					Obsv: stubExporters(),
				}
			},
			args:       map[string]any{"query": "symbol"},
			itemsField: "items",
			fullFields: []string{"name", "path"},
			projection: []string{"name", "path"},
		},
	}
}

func projectedOutputToolByName(name string) inventory.ServerTool {
	switch name {
	case "list_issues":
		return ListIssues(translations.NullTranslationHelper)
	case "list_pull_requests":
		return ListPullRequests(translations.NullTranslationHelper)
	case "search_issues":
		return SearchIssues(translations.NullTranslationHelper)
	case "search_pull_requests":
		return SearchPullRequests(translations.NullTranslationHelper)
	case "search_code":
		return SearchCode(translations.NullTranslationHelper)
	default:
		panic("unknown projected output tool: " + name)
	}
}

func callRegisteredTool(t *testing.T, serverTool inventory.ServerTool, deps ToolDependencies, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	serverTool.RegisterFunc(server, deps)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSession.Close() })

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      serverTool.Tool.Name,
		Arguments: args,
	})
	require.NoError(t, err)
	return result
}

func requireStructuredJSONAgreement(t *testing.T, serverTool inventory.ServerTool, result *mcp.CallToolResult) {
	t.Helper()
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", getErrorResult(t, result).Text)
	}
	require.NotNil(t, result.StructuredContent)

	schema := serverTool.Tool.OutputSchema.(*jsonschema.Schema)
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	require.NoError(t, resolved.Validate(result.StructuredContent))

	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}
		var decoded any
		if json.Unmarshal([]byte(text.Text), &decoded) == nil && reflect.DeepEqual(decoded, result.StructuredContent) {
			return
		}
	}
	t.Fatal("no JSON text content matched structuredContent")
}

func projectedStructuredItems(t *testing.T, structured any, field string) []any {
	t.Helper()
	if field == "" {
		items, ok := structured.([]any)
		require.True(t, ok)
		return items
	}
	wrapper, ok := structured.(map[string]any)
	require.True(t, ok)
	items, ok := wrapper[field].([]any)
	require.True(t, ok)
	return items
}

func cloneArguments(args map[string]any) map[string]any {
	cloned := make(map[string]any, len(args)+1)
	maps.Copy(cloned, args)
	return cloned
}

func projectedPullRequests(empty bool) []*github.PullRequest {
	if empty {
		return []*github.PullRequest{}
	}
	return []*github.PullRequest{
		{
			Number:  github.Ptr(42),
			Title:   github.Ptr("First PR"),
			Body:    github.Ptr("PR body"),
			State:   github.Ptr("open"),
			Draft:   github.Ptr(false),
			Merged:  github.Ptr(false),
			HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42"),
			User:    &github.User{Login: github.Ptr("octocat")},
		},
	}
}

func projectedIssueSearchResult(empty bool) *github.IssuesSearchResult {
	result := &github.IssuesSearchResult{
		Total:             github.Ptr(0),
		IncompleteResults: github.Ptr(false),
		Issues:            []*github.Issue{},
	}
	if empty {
		return result
	}
	result.Total = github.Ptr(1)
	result.Issues = []*github.Issue{
		{
			Number:  github.Ptr(42),
			Title:   github.Ptr("A result"),
			Body:    github.Ptr("Result body"),
			State:   github.Ptr("open"),
			Draft:   github.Ptr(false),
			Locked:  github.Ptr(false),
			HTMLURL: github.Ptr("https://github.com/owner/repo/issues/42"),
			User:    &github.User{Login: github.Ptr("octocat")},
			Labels:  []*github.Label{{Name: "bug", Color: "ff0000"}},
		},
	}
	return result
}

func projectedCodeSearchResult(empty bool) *github.CodeSearchResult {
	result := &github.CodeSearchResult{
		Total:             github.Ptr(0),
		IncompleteResults: github.Ptr(false),
		CodeResults:       []*github.CodeResult{},
	}
	if empty {
		return result
	}
	result.Total = github.Ptr(1)
	result.CodeResults = []*github.CodeResult{
		{
			Name: github.Ptr("main.go"),
			Path: github.Ptr("cmd/main.go"),
			SHA:  github.Ptr("abc123"),
			Repository: &github.Repository{
				FullName: github.Ptr("owner/repo"),
			},
			TextMatches: []*github.TextMatch{
				{
					Fragment: github.Ptr("func main() {}"),
					Matches: []*github.Match{
						{Text: github.Ptr("main"), Indices: []int{5, 9}},
					},
				},
			},
		},
	}
	return result
}

func projectedListIssuesHTTPClient(t *testing.T, empty bool, first int, after string, pageInfo map[string]any) *http.Client {
	t.Helper()

	nodes := []map[string]any{}
	totalCount := 0
	if !empty {
		nodes = append(nodes, map[string]any{
			"number":           123,
			"title":            "First Issue",
			"body":             "Issue body",
			"state":            "OPEN",
			"databaseId":       1001,
			"createdAt":        "2023-01-01T00:00:00Z",
			"updatedAt":        "2023-01-01T00:00:00Z",
			"author":           map[string]any{"login": "octocat"},
			"labels":           map[string]any{"nodes": []map[string]any{{"name": "bug", "id": "LA_1", "description": "Bug"}}},
			"assignees":        map[string]any{"nodes": []map[string]any{}},
			"comments":         map[string]any{"totalCount": 0},
			"issueFieldValues": map[string]any{"nodes": []map[string]any{}},
		})
		totalCount = 1
	}
	if pageInfo == nil {
		pageInfo = map[string]any{
			"hasNextPage":     false,
			"hasPreviousPage": false,
			"startCursor":     "",
			"endCursor":       "",
		}
	}

	var afterValue any = (*string)(nil)
	if after != "" {
		afterValue = after
	}
	vars := map[string]any{
		"owner":            "owner",
		"repo":             "repo",
		"states":           []any{"OPEN", "CLOSED"},
		"orderBy":          "CREATED_AT",
		"direction":        "DESC",
		"first":            float64(first),
		"after":            afterValue,
		"issueFieldValues": []any{},
	}
	response := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issues": map[string]any{
				"nodes":      nodes,
				"pageInfo":   pageInfo,
				"totalCount": totalCount,
			},
			"isPrivate": false,
		},
	})
	query := listIssuesFieldsQuery
	if after != "" {
		query = strings.Replace(query, "$after:String", "$after:String!", 1)
	}
	return githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher(query, vars, response))
}
