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
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_UIGet(t *testing.T) {
	// Verify tool definition
	serverTool := UIGet(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "ui_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner"})
	assert.True(t, tool.Annotations.ReadOnlyHint, "ui_get should be read-only")
	assert.Equal(t, MCPAppsFeatureFlag, serverTool.FeatureFlagEnable, "ui_get should be gated on the MCP Apps feature flag")

	// ui_get must be app-only so the host hides it from the agent's tool list
	// while keeping it callable by the views (MCP Apps 2026-01-26 spec).
	ui, ok := tool.Meta["ui"].(map[string]any)
	require.True(t, ok, "ui_get should declare _meta.ui")
	assert.Equal(t, []string{"app"}, ui["visibility"], "ui_get should be app-only")

	// Setup mock data
	mockAssignees := []*github.User{
		{Login: github.Ptr("user1"), AvatarURL: github.Ptr("https://avatars.githubusercontent.com/u/1")},
		{Login: github.Ptr("user2"), AvatarURL: github.Ptr("https://avatars.githubusercontent.com/u/2")},
	}

	mockBranches := []*github.Branch{
		{Name: github.Ptr("main"), Protected: github.Ptr(true)},
		{Name: github.Ptr("feature"), Protected: github.Ptr(false)},
	}

	dueDate := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	mockMilestones := []*github.Milestone{
		{Number: github.Ptr(1), Title: github.Ptr("with due date"), DueOn: &github.Timestamp{Time: dueDate}},
		{Number: github.Ptr(2), Title: github.Ptr("no due date")},
	}

	mockIssueTypes := []*github.IssueType{
		{Name: github.Ptr("Bug")},
		{Name: github.Ptr("Feature")},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		mockedGQLClient *http.Client
		requestArgs     map[string]any
		expectError     bool
		expectedErrMsg  string
		validateResult  func(t *testing.T, responseText string)
	}{
		{
			name: "successful assignees fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/owner/repo/assignees": mockResponse(t, http.StatusOK, mockAssignees),
			}),
			requestArgs: map[string]any{
				"method": "assignees",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, responseText string) {
				var response map[string]any
				require.NoError(t, json.Unmarshal([]byte(responseText), &response))
				assert.Contains(t, response, "assignees")
				assert.Contains(t, response, "totalCount")
			},
		},
		{
			name: "successful branches fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/owner/repo/branches": mockResponse(t, http.StatusOK, mockBranches),
			}),
			requestArgs: map[string]any{
				"method": "branches",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, responseText string) {
				var response map[string]any
				require.NoError(t, json.Unmarshal([]byte(responseText), &response))
				assert.Contains(t, response, "branches")
				assert.Contains(t, response, "totalCount")
			},
		},
		{
			name: "successful milestones fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/owner/repo/milestones": mockResponse(t, http.StatusOK, mockMilestones),
			}),
			requestArgs: map[string]any{
				"method": "milestones",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, responseText string) {
				var response map[string]any
				require.NoError(t, json.Unmarshal([]byte(responseText), &response))
				milestones, ok := response["milestones"].([]any)
				require.True(t, ok, "milestones should be a list")
				require.Len(t, milestones, 2)
				first := milestones[0].(map[string]any)
				assert.Equal(t, "2026-01-31", first["due_on"], "milestone with a due date should be formatted")
				second := milestones[1].(map[string]any)
				assert.Equal(t, "", second["due_on"], "milestone without a due date should be empty, not zero time")
			},
		},
		{
			name: "successful issue_types fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /orgs/owner/issue-types": mockResponse(t, http.StatusOK, mockIssueTypes),
			}),
			requestArgs: map[string]any{
				"method": "issue_types",
				"owner":  "owner",
			},
			expectError: false,
			validateResult: func(t *testing.T, responseText string) {
				var issueTypes []map[string]any
				require.NoError(t, json.Unmarshal([]byte(responseText), &issueTypes))
				require.Len(t, issueTypes, 2)
				assert.Equal(t, "Bug", issueTypes[0]["name"])
			},
		},
		{
			name: "issue_types API error returns response context",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /orgs/owner/issue-types": mockResponse(t, http.StatusForbidden, map[string]string{"message": "Forbidden"}),
			}),
			requestArgs: map[string]any{
				"method": "issue_types",
				"owner":  "owner",
			},
			expectError:    true,
			expectedErrMsg: "failed to list issue types",
		},
		{
			name: "successful labels fetch",
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Labels struct {
								Nodes []struct {
									ID          githubv4.ID
									Name        githubv4.String
									Color       githubv4.String
									Description githubv4.String
								}
								TotalCount githubv4.Int
								PageInfo   struct {
									HasNextPage githubv4.Boolean
									EndCursor   githubv4.String
								}
							} `graphql:"labels(first: 100, after: $cursor)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner":  githubv4.String("owner"),
						"repo":   githubv4.String("repo"),
						"cursor": (*githubv4.String)(nil),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"labels": map[string]any{
								"nodes": []any{
									map[string]any{
										"id":          githubv4.ID("label-1"),
										"name":        githubv4.String("bug"),
										"color":       githubv4.String("d73a4a"),
										"description": githubv4.String("Something isn't working"),
									},
								},
								"totalCount": githubv4.Int(1),
								"pageInfo": map[string]any{
									"hasNextPage": githubv4.Boolean(false),
									"endCursor":   githubv4.String(""),
								},
							},
						},
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "labels",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, responseText string) {
				var response map[string]any
				require.NoError(t, json.Unmarshal([]byte(responseText), &response))
				labels, ok := response["labels"].([]any)
				require.True(t, ok, "labels should be a list")
				require.Len(t, labels, 1)
				assert.Equal(t, "bug", labels[0].(map[string]any)["name"])
				assert.Equal(t, float64(1), response["totalCount"])
			},
		},
		{
			name:         "missing method parameter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: method",
		},
		{
			name:         "missing owner parameter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method": "assignees",
				"repo":   "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing repo parameter for assignees",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method": "assignees",
				"owner":  "owner",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: repo",
		},
		{
			name:         "unknown method",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method": "unknown",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError:    true,
			expectedErrMsg: "unknown method: unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup deps with REST and/or GraphQL mocks
			deps := BaseDeps{}
			if tc.mockedClient != nil {
				client, err := github.NewClient(github.WithHTTPClient(tc.mockedClient))
				require.NoError(t, err)
				deps.Client = client
			}
			if tc.mockedGQLClient != nil {
				deps.GQLClient = githubv4.NewClient(tc.mockedGQLClient)
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
					return
				}
				require.NotNil(t, result)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			if tc.validateResult != nil {
				tc.validateResult(t, textContent.Text)
			}
		})
	}
}
