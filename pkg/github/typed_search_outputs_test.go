package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedProjectedReadToolOutputSchemas(t *testing.T) {
	tools := []inventory.ServerTool{
		SearchCode(translations.NullTranslationHelper),
		SearchIssues(translations.NullTranslationHelper, WithHost(utils.HostTypeGHES)),
		SearchPullRequests(translations.NullTranslationHelper),
		ListIssues(translations.NullTranslationHelper),
		ListPullRequests(translations.NullTranslationHelper),
		ListBranches(translations.NullTranslationHelper),
		ListTags(translations.NullTranslationHelper),
	}
	for _, tool := range tools {
		assert.Equal(t, []string{"repo"}, tool.ScopeAccess.Scopes, "%s must retain repository read scope", tool.Tool.Name)
	}
	deps := BaseDeps{
		featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetSearchCode: mockResponse(t, http.StatusOK, &github.CodeSearchResult{
				Total:             new(1),
				IncompleteResults: new(false),
				CodeResults: []*github.CodeResult{{
					Name: new("main.go"),
					Path: new("cmd/main.go"),
					SHA:  new("abc123"),
					Repository: &github.Repository{
						FullName: new("owner/repo"),
						Private:  new(false),
					},
					TextMatches: []*github.TextMatch{{
						ObjectType: new("FileContent"),
						Property:   new("content"),
						Fragment:   new("func main()"),
						Matches: []*github.Match{{
							Text:    new("main"),
							Indices: []int{5, 9},
						}},
					}},
				}},
			}),
			GetSearchIssues: mockResponse(t, http.StatusOK, &github.IssuesSearchResult{
				Total:             new(1),
				IncompleteResults: new(false),
				Issues: []*github.Issue{{
					ID:     new(int64(42)),
					URL:    new("https://api.github.com/repos/owner/repo/issues/42"),
					Number: new(42),
					Title:  new("A title"),
					Body:   new("A body"),
					State:  new("open"),
				}},
			}),
			GetReposBranchesByOwnerByRepo: mockResponse(t, http.StatusOK, []*github.Branch{}),
			GetReposTagsByOwnerByRepo: mockResponse(t, http.StatusOK, []*github.RepositoryTag{
				{Name: new("v1.0.0"), Commit: &github.Commit{SHA: new("abc123")}},
			}),
			GetReposByOwnerByRepo: mockResponse(t, http.StatusOK, &github.Repository{Private: new(false)}),
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
			outputSchemas := make(map[string]*jsonschema.Resolved, len(list.Tools))
			for _, tool := range list.Tools {
				if protocolVersion == "2025-11-25" {
					assert.Nil(t, tool.OutputSchema, "legacy clients must not see outputSchema for %s", tool.Name)
					continue
				}

				require.NotNil(t, tool.OutputSchema, "%s must publish its typed output schema", tool.Name)
				require.NoError(t, toolsnaps.Test(tool.Name+"_output", mcp.Tool{
					Name:         tool.Name,
					OutputSchema: tool.OutputSchema,
				}))
				schemaJSON, err := json.Marshal(tool.OutputSchema)
				require.NoError(t, err)
				var schema jsonschema.Schema
				require.NoError(t, json.Unmarshal(schemaJSON, &schema))
				resolved, err := schema.Resolve(nil)
				require.NoError(t, err)
				require.NoError(t, resolved.Validate(projectedReadOutputSample(tool.Name)), "%s output must conform to its schema", tool.Name)
				require.NoError(t, resolved.Validate(projectedReadNullOutputSample(tool.Name)), "%s output schema must accept nullable optional fields", tool.Name)
				outputSchemas[tool.Name] = resolved
			}

			branches, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "list_branches",
				Arguments: map[string]any{"owner": "owner", "repo": "repo"},
			})
			require.NoError(t, err)
			require.False(t, branches.IsError, branches)
			assert.Equal(t, "[]", getTextResult(t, branches).Text)
			assert.JSONEq(t, mustMarshalJSON(t, ifc.LabelRepoMetadata(false)), mustMarshalJSON(t, branches.Meta["ifc"]))
			if protocolVersion == inventory.ProtocolVersionMultiRoundTrip {
				assert.Equal(t, "[]", mustMarshalJSON(t, branches.StructuredContent))
				require.NoError(t, outputSchemas["list_branches"].Validate(branches.StructuredContent))
			} else {
				assert.Nil(t, branches.StructuredContent)
			}

			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "list_tags",
				Arguments: map[string]any{"owner": "owner", "repo": "repo"},
			})
			require.NoError(t, err)
			require.False(t, result.IsError, result)
			require.Equal(t, `[{"name":"v1.0.0","sha":"abc123"}]`, getTextResult(t, result).Text)
			assert.JSONEq(t, mustMarshalJSON(t, ifc.LabelRepoMetadata(false)), mustMarshalJSON(t, result.Meta["ifc"]))
			if protocolVersion == "2025-11-25" {
				assert.Nil(t, result.StructuredContent)
			} else {
				structured, err := json.Marshal(result.StructuredContent)
				require.NoError(t, err)
				assert.JSONEq(t, getTextResult(t, result).Text, string(structured))
				require.NoError(t, outputSchemas["list_tags"].Validate(result.StructuredContent))
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
			if protocolVersion == "2025-11-25" {
				assert.Equal(t, `{"incomplete_results":false,"items":[{"name":"main.go"}],"total_count":1}`, getTextResult(t, result).Text)
			}
			assert.JSONEq(t, mustMarshalJSON(t, ifc.LabelSearchIssues([]bool{false})), mustMarshalJSON(t, result.Meta["ifc"]))
			if protocolVersion == "2025-11-25" {
				assert.Nil(t, result.StructuredContent)
			} else {
				assert.JSONEq(t, `{"total_count":1,"incomplete_results":false,"items":[{"name":"main.go"}]}`, getTextResult(t, result).Text)
				structured, err := json.Marshal(result.StructuredContent)
				require.NoError(t, err)
				assert.JSONEq(t, getTextResult(t, result).Text, string(structured))
				require.NoError(t, outputSchemas["search_code"].Validate(result.StructuredContent))
			}

			result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "search_code",
				Arguments: map[string]any{"query": "main.go"},
			})
			require.NoError(t, err)
			require.False(t, result.IsError, result)
			if protocolVersion == "2025-11-25" {
				assert.Equal(t, `{"total_count":1,"incomplete_results":false,"items":[{"name":"main.go","path":"cmd/main.go","sha":"abc123","repository":"owner/repo","text_matches":[{"object_type":"FileContent","property":"content","fragment":"func main()","matches":[{"text":"main","indices":[5,9]}]}]}]}`, getTextResult(t, result).Text)
			}
			if protocolVersion == inventory.ProtocolVersionMultiRoundTrip {
				assert.JSONEq(t, `{"total_count":1,"incomplete_results":false,"items":[{"name":"main.go","path":"cmd/main.go","sha":"abc123","repository":"owner/repo","text_matches":[{"object_type":"FileContent","property":"content","fragment":"func main()","matches":[{"text":"main","indices":[5,9]}]}]}]}`, getTextResult(t, result).Text)
				require.NoError(t, outputSchemas["search_code"].Validate(result.StructuredContent))
				assert.JSONEq(t, getTextResult(t, result).Text, mustMarshalJSON(t, result.StructuredContent))
			}

			result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "search_issues",
				Arguments: map[string]any{"query": "bug"},
			})
			require.NoError(t, err)
			require.False(t, result.IsError, result)
			if protocolVersion == "2025-11-25" {
				assert.Equal(t, `{"total_count":1,"incomplete_results":false,"items":[{"body":"A body","id":42,"number":42,"state":"open","title":"A title","url":"https://api.github.com/repos/owner/repo/issues/42"}]}`, getTextResult(t, result).Text)
				assert.Nil(t, result.StructuredContent)
			} else {
				assert.JSONEq(t, `{"total_count":1,"incomplete_results":false,"items":[{"number":42,"title":"A title","body":"A body","state":"open"}]}`, getTextResult(t, result).Text)
				assert.JSONEq(t, getTextResult(t, result).Text, mustMarshalJSON(t, result.StructuredContent))
				require.NoError(t, outputSchemas["search_issues"].Validate(result.StructuredContent))
			}

			failure, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "search_code",
				Arguments: map[string]any{"query": ""},
			})
			require.NoError(t, err)
			require.True(t, failure.IsError)
			assert.Nil(t, failure.StructuredContent, "error results must not expose typed output")
			assert.Equal(t, "missing required parameter: query", getErrorResult(t, failure).Text)
		})
	}
}

func projectedReadOutputSample(name string) any {
	switch name {
	case "search_code":
		return map[string]any{
			"total_count": 1, "incomplete_results": false,
			"items": []any{map[string]any{
				"name": "main.go", "path": "cmd/main.go", "sha": "abc123", "repository": "owner/repo",
				"text_matches": []any{map[string]any{
					"object_url":  "https://api.github.com/repos/owner/repo",
					"object_type": "FileContent", "property": "content", "fragment": "func main()",
					"matches": []any{map[string]any{"text": "main", "indices": []any{5, 9}}},
				}},
			}},
		}
	case "search_issues", "search_pull_requests":
		return map[string]any{
			"total_count": 1, "incomplete_results": false,
			"items": []any{map[string]any{
				"number": 42, "title": "A title", "body": "A body", "state": "open",
				"state_reason": "completed", "draft": false, "locked": false,
				"html_url":           "https://github.com/owner/repo/issues/42",
				"user":               map[string]any{"login": "octocat", "profile_url": "https://github.com/octocat"},
				"author_association": "OWNER", "labels": []any{"bug"},
				"assignee": "octocat", "assignees": []any{"octocat"}, "milestone": "v1",
				"comments": 3,
				"reactions": map[string]any{
					"total_count": 1, "+1": 1, "-1": 0, "laugh": 0, "confused": 0,
					"heart": 0, "hooray": 0, "rocket": 0, "eyes": 0,
				},
				"created_at": "2026-01-02T03:04:05Z", "updated_at": "2026-01-03T03:04:05Z",
				"closed_at": "2026-01-04T03:04:05Z", "closed_by": "maintainer",
				"type": "Bug", "repository_url": "https://api.github.com/repos/owner/repo",
				"pull_request": map[string]any{
					"url":       "https://api.github.com/repos/owner/repo/pulls/42",
					"html_url":  "https://github.com/owner/repo/pull/42",
					"diff_url":  "https://github.com/owner/repo/pull/42.diff",
					"patch_url": "https://github.com/owner/repo/pull/42.patch",
				},
				"field_values": []any{map[string]any{"field": "Priority", "value": "P1"}},
			}},
		}
	case "list_issues":
		return map[string]any{
			"issues": []any{map[string]any{
				"number": 42, "title": "A title", "body": "A body", "state": "OPEN",
				"user": map[string]any{"login": "octocat"}, "labels": []any{"bug"},
				"assignees": []any{"octocat"}, "comments": 3,
				"created_at": "2026-01-02T03:04:05Z", "updated_at": "2026-01-03T03:04:05Z",
				"field_values": []any{map[string]any{"field": "Priority", "value": "P1"}},
			}},
			"totalCount": 1,
			"pageInfo": map[string]any{
				"hasNextPage": false, "hasPreviousPage": false,
				"startCursor": "start", "endCursor": "end",
			},
		}
	case "list_pull_requests":
		return []any{map[string]any{
			"number": 42, "title": "A title", "body": "A body", "state": "open",
			"draft": false, "merged": false, "mergeable_state": "clean",
			"html_url": "https://github.com/owner/repo/pull/42",
			"user":     map[string]any{"login": "octocat"},
			"labels":   []any{"bug"}, "assignees": []any{"octocat"},
			"requested_reviewers": []any{"reviewer"}, "merged_by": "maintainer",
			"head":      map[string]any{"ref": "feature", "sha": "abc123"},
			"base":      map[string]any{"ref": "main", "sha": "def456"},
			"additions": 5, "deletions": 2, "changed_files": 1, "commits": 1,
			"comments": 3, "created_at": "2026-01-02T03:04:05Z",
			"updated_at": "2026-01-03T03:04:05Z", "closed_at": "2026-01-04T03:04:05Z",
			"merged_at": "2026-01-04T03:04:05Z", "milestone": "v1",
		}}
	case "list_branches":
		return []any{map[string]any{"name": "main", "sha": "abc123", "protected": true}}
	case "list_tags":
		return []any{map[string]any{"name": "v1.0.0", "sha": "abc123"}}
	default:
		panic("unexpected projected read tool: " + name)
	}
}

func projectedReadNullOutputSample(name string) any {
	switch name {
	case "search_code":
		return map[string]any{
			"total_count": 0, "incomplete_results": false,
			"items": []any{map[string]any{"name": nil}},
		}
	case "search_issues", "search_pull_requests":
		return map[string]any{
			"total_count": nil, "incomplete_results": nil,
			"items": []any{map[string]any{"title": nil}},
		}
	case "list_issues":
		return map[string]any{
			"issues": []any{map[string]any{"state": nil}}, "totalCount": 0,
			"pageInfo": map[string]any{"hasNextPage": false, "hasPreviousPage": false},
		}
	case "list_pull_requests":
		return []any{map[string]any{"state": nil}}
	case "list_branches", "list_tags":
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
			Issue:       &github.Issue{Number: new(42), Title: new("A title"), Body: new("A body")},
			FieldValues: []MinimalFieldValue{},
		}},
	}, []string{"title"})
	assert.JSONEq(t, `{"total_count":1,"incomplete_results":false,"items":[{"title":"A title"}]}`, mustMarshalJSON(t, searchIssues))

	emptyFieldValues := structuredSearchIssuesOutput(SearchIssuesResponse{
		Items: []SearchIssueResult{{Issue: &github.Issue{}, FieldValues: []MinimalFieldValue{}}},
	}, []string{"field_values"})
	assert.Equal(t, `{"items":[{"field_values":[]}]}`, mustMarshalJSON(t, emptyFieldValues))

	unavailableFieldValues := structuredSearchIssuesOutput(SearchIssuesResponse{
		Items: []SearchIssueResult{{Issue: &github.Issue{}}},
	}, []string{"field_values"})
	assert.Equal(t, `{"items":[{}]}`, mustMarshalJSON(t, unavailableFieldValues))

	codeMatches := structuredSearchCodeOutput(MinimalCodeSearchResult{
		Items: []MinimalCodeResult{{
			TextMatches: []*github.TextMatch{{
				ObjectURL:  new("https://api.github.com/repos/owner/repo"),
				ObjectType: new("FileContent"),
				Property:   new("content"),
				Fragment:   new("func main()"),
				Matches: []*github.Match{{
					Text:    new("main"),
					Indices: []int{5, 9},
				}},
			}},
		}},
	}, []string{"text_matches"})
	assert.Equal(t, `{"total_count":0,"incomplete_results":false,"items":[{"text_matches":[{"object_url":"https://api.github.com/repos/owner/repo","object_type":"FileContent","property":"content","fragment":"func main()","matches":[{"text":"main","indices":[5,9]}]}]}]}`, mustMarshalJSON(t, codeMatches))

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
