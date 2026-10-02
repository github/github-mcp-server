package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedProjectedReadToolOutputSchemas(t *testing.T) {
	tools := []inventory.ServerTool{
		SearchCode(translations.NullTranslationHelper),
		SearchIssues(translations.NullTranslationHelper),
		SearchPullRequests(translations.NullTranslationHelper),
		ListIssues(translations.NullTranslationHelper),
		ListPullRequests(translations.NullTranslationHelper),
		ListBranches(translations.NullTranslationHelper),
		ListTags(translations.NullTranslationHelper),
	}
	deps := BaseDeps{
		Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetSearchCode: mockResponse(t, http.StatusOK, &github.CodeSearchResult{
				Total:             new(1),
				IncompleteResults: new(false),
				CodeResults: []*github.CodeResult{{
					Name: new("main.go"),
					Path: new("cmd/main.go"),
					SHA:  new("abc123"),
				}},
			}),
			GetReposTagsByOwnerByRepo: mockResponse(t, http.StatusOK, []*github.RepositoryTag{
				{Name: new("v1.0.0"), Commit: &github.Commit{SHA: new("abc123")}},
			}),
		})),
	}

	for _, protocolVersion := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocolVersion, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{Name: "typed-projected-read-test", Version: "v1"}, nil)
			server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
			for _, tool := range tools {
				tool.RegisterFunc(server, deps)
			}

			session := connectCommentVisibilityClient(t, server, protocolVersion)
			list, err := session.ListTools(context.Background(), nil)
			require.NoError(t, err)
			require.Len(t, list.Tools, len(tools))
			for _, tool := range list.Tools {
				if protocolVersion == "2025-11-25" {
					assert.Nil(t, tool.OutputSchema, "legacy clients must not see outputSchema for %s", tool.Name)
					continue
				}

				require.NotNil(t, tool.OutputSchema, "%s must publish its typed output schema", tool.Name)
				schemaJSON, err := json.Marshal(tool.OutputSchema)
				require.NoError(t, err)
				var schema jsonschema.Schema
				require.NoError(t, json.Unmarshal(schemaJSON, &schema))
				resolved, err := schema.Resolve(nil)
				require.NoError(t, err)
				require.NoError(t, resolved.Validate(projectedReadOutputSample(tool.Name)), "%s output must conform to its schema", tool.Name)
			}

			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "list_tags",
				Arguments: map[string]any{"owner": "owner", "repo": "repo"},
			})
			require.NoError(t, err)
			require.False(t, result.IsError, result)
			require.Equal(t, `[{"name":"v1.0.0","sha":"abc123"}]`, getTextResult(t, result).Text)
			if protocolVersion == "2025-11-25" {
				assert.Nil(t, result.StructuredContent)
			} else {
				structured, err := json.Marshal(result.StructuredContent)
				require.NoError(t, err)
				assert.JSONEq(t, getTextResult(t, result).Text, string(structured))
			}

			result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "search_code",
				Arguments: map[string]any{
					"query":  "main.go",
					"fields": []any{"name"},
				},
			})
			require.NoError(t, err)
			require.False(t, result.IsError, result)
			assert.JSONEq(t, `{"total_count":1,"incomplete_results":false,"items":[{"name":"main.go"}]}`, getTextResult(t, result).Text)
			if protocolVersion == "2025-11-25" {
				assert.Nil(t, result.StructuredContent)
			} else {
				structured, err := json.Marshal(result.StructuredContent)
				require.NoError(t, err)
				assert.JSONEq(t, getTextResult(t, result).Text, string(structured))
			}
		})
	}
}

func projectedReadOutputSample(name string) any {
	switch name {
	case "search_code", "search_issues", "search_pull_requests":
		return map[string]any{"total_count": 0, "incomplete_results": false, "items": []any{}}
	case "list_issues":
		return map[string]any{
			"issues": []any{}, "totalCount": 0,
			"pageInfo": map[string]any{"hasNextPage": false, "hasPreviousPage": false},
		}
	case "list_pull_requests", "list_branches", "list_tags":
		return []any{}
	default:
		panic("unexpected projected read tool: " + name)
	}
}

func TestTypedProjectedReadOutputsRespectFieldSelection(t *testing.T) {
	code := structuredSearchCodeOutput(MinimalCodeSearchResult{
		TotalCount:        1,
		IncompleteResults: false,
		Items:             []MinimalCodeResult{{Name: "main.go", Path: "cmd/main.go", SHA: "abc123", Repository: "owner/repo"}},
	}, []string{"name"})
	assert.JSONEq(t, `{"total_count":1,"incomplete_results":false,"items":[{"name":"main.go"}]}`, mustMarshalJSON(t, code))

	searchIssues := structuredSearchIssuesOutput(SearchIssuesResponse{
		Total:             new(1),
		IncompleteResults: new(false),
		Items: []SearchIssueResult{{
			Issue: &github.Issue{Number: new(42), Title: new("A title"), Body: new("A body")},
		}},
	}, []string{"title"})
	assert.JSONEq(t, `{"total_count":1,"incomplete_results":false,"items":[{"title":"A title"}]}`, mustMarshalJSON(t, searchIssues))

	issues := structuredListIssuesOutput(MinimalIssuesResponse{
		Issues:     []MinimalIssue{{Number: 42, Title: "A title", Body: "A body", State: "OPEN", Assignees: []string{}}},
		TotalCount: 1,
	}, []string{"title"})
	assert.JSONEq(t, `{"issues":[{"title":"A title"}],"totalCount":1,"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}`, mustMarshalJSON(t, issues))

	pullRequests := structuredListPullRequestsOutput([]MinimalPullRequest{{
		Number: 42, Title: "A title", Body: "A body", State: "open", HTMLURL: "https://example.test/pr/42",
	}}, []string{"title"})
	assert.JSONEq(t, `[{"title":"A title"}]`, mustMarshalJSON(t, pullRequests))
}

func TestNormalizeTypedReadArgumentsPreservesLegacyValues(t *testing.T) {
	normalize := normalizeTypedReadArguments([]string{"state", "orderBy", "direction"}, true)
	normalized, err := normalize(json.RawMessage(`{"state":"open","orderBy":"updated_at","direction":"asc","perPage":"25","page":null}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"state":"OPEN","orderBy":"UPDATED_AT","direction":"ASC","perPage":25,"page":null}`, string(normalized))
}

func mustMarshalJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}
