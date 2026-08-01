package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorderTransport routes HTTP 请求s through 一个in-process 处理器, mirroring
// internal/githubv4模拟's own transport. We need it 因为githubv4模拟 keys its
// matchers by query string, so it can不model 一个multi-页 labels query: every
// 页 议题 identical query 和differs 仅由$curs或variable. 此
// transport lets 一个单个 处理器 answer 每个页 dynami调用y.
type recorderTransport struct{ handler http.Handler }

func (rt recorderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	rt.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

// alwaysHasNextPageLabelsClient 返回 一个GraphQL 客户端 whose labels query always
// reports another 页, advancing curs或on 每个调用. It exercises uiGetLabels'
// 页 cap: loop fetches one label per 页 until it stops at uiGetMaxPages with
// has_more=真. totalCount is reported as 一个large 服务器-side count so test can
// confirm it stays full repo count even when 结果 are truncated.
func alwaysHasNextPageLabelsClient(t *testing.T) *http.Client {
	t.Helper()
	var calls int
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, _ *http.Request) {
		calls++
		resp := map[string]any{
			"data": map[string]any{
				"repository": map[string]any{
					"labels": map[string]any{
						"nodes": []any{
							map[string]any{
								"id":          fmt.Sprintf("label-%d", calls),
								"name":        fmt.Sprintf("label-%d", calls),
								"color":       "ededed",
								"description": "",
							},
						},
						"totalCount": 9999,
						"pageInfo": map[string]any{
							"hasNextPage": true,
							"endCursor":   fmt.Sprintf("cursor-%d", calls),
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	return &http.Client{Transport: recorderTransport{handler: mux}}
}

// alwaysNextPageHandler 返回 一个REST 处理器 that 始终advertises another 页
// via Link header, regardless 的页 请求ed. It drives 一个pagination loop
// purely off 页 cap so tests can assert ui_获取 stops at uiGetMaxPages 和sets
// has_more=真. 相同 body is 返回ed f或every 页, so number of items
// collected equals number of 页s fetched.
func alwaysNextPageHandler(t *testing.T, body any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			if parsed, err := strconv.Atoi(p); err == nil {
				page = parsed
			}
		}
		w.Header().Set("Link", fmt.Sprintf(`<https://api.github.com/next?page=%d>; rel="next"`, page+1))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(body)
	}
}

func Test_UIGet(t *testing.T) {
	// Verify 工具定义
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

	// ui_获取 必须是 app-仅so host hides it 来自agent's 工具 列出
	// 当keeping it 调用able 由views (MCP Apps 2026-01-26 spec).
	ui, ok := tool.Meta["ui"].(map[string]any)
	require.True(t, ok, "ui_get should declare _meta.ui")
	assert.Equal(t, []string{"app"}, ui["visibility"], "ui_get should be app-only")

	// Setup 模拟 数据
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

	mockReviewers := []*github.User{
		{Login: github.Ptr("octocat"), AvatarURL: github.Ptr("https://avatars.githubusercontent.com/u/583231")},
		{Login: github.Ptr("dependabot[bot]"), AvatarURL: github.Ptr("https://avatars.githubusercontent.com/in/29110")},
		{Login: github.Ptr("github-actions"), Type: github.Ptr("Bot")},
	}

	mockReviewerTeams := []*github.Team{
		{Slug: github.Ptr("docs"), Name: github.Ptr("Docs")},
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
				assert.Equal(t, false, response["has_more"], "results within the page cap should not be truncated")
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
				assert.Equal(t, false, response["has_more"], "results within the page cap should not be truncated")
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
				assert.Equal(t, false, response["has_more"], "results within the page cap should not be truncated")
			},
		},
		{
			name: "successful issue_fields fetch",
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					issueFieldsRepoQuery{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"name":  githubv4.String("repo"),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"issueFields": map[string]any{
								"nodes": []any{
									map[string]any{
										"__typename":     "IssueFieldText",
										"id":             "IFT_1",
										"fullDatabaseId": "42",
										"name":           "DRI",
										"description":    "Directly responsible individual",
										"dataType":       "text",
										"visibility":     "ORG_ONLY",
									},
								},
							},
						},
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "issue_fields",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, responseText string) {
				var response map[string]any
				require.NoError(t, json.Unmarshal([]byte(responseText), &response))
				fields, ok := response["fields"].([]any)
				require.True(t, ok, "fields should be a list")
				require.Len(t, fields, 1)
				assert.Equal(t, "DRI", fields[0].(map[string]any)["name"])
				assert.Equal(t, "text", fields[0].(map[string]any)["data_type"])
				assert.Equal(t, float64(1), response["totalCount"])
			},
		},
		{
			name: "successful reviewers fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/owner/repo/collaborators": mockResponse(t, http.StatusOK, mockReviewers),
				"GET /repos/owner/repo/teams":         mockResponse(t, http.StatusOK, mockReviewerTeams),
			}),
			requestArgs: map[string]any{
				"method": "reviewers",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, responseText string) {
				var response map[string]any
				require.NoError(t, json.Unmarshal([]byte(responseText), &response))
				users, ok := response["users"].([]any)
				require.True(t, ok, "users should be a list")
				require.Len(t, users, 1)
				assert.Equal(t, "octocat", users[0].(map[string]any)["login"])
				teams, ok := response["teams"].([]any)
				require.True(t, ok, "teams should be a list")
				require.Len(t, teams, 1)
				assert.Equal(t, "docs", teams[0].(map[string]any)["slug"])
				assert.Equal(t, "owner", teams[0].(map[string]any)["org"])
				assert.Equal(t, float64(2), response["totalCount"])
				assert.Equal(t, false, response["has_more"], "results within the page cap should not be truncated")
			},
		},
		{
			name: "branches pagination stops at the page cap",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/owner/repo/branches": alwaysNextPageHandler(t, []*github.Branch{{Name: github.Ptr("feature")}}),
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
				branches, ok := response["branches"].([]any)
				require.True(t, ok, "branches should be a list")
				assert.Len(t, branches, uiGetMaxPages, "loop should stop at the page cap")
				assert.Equal(t, float64(uiGetMaxPages), response["totalCount"], "totalCount should be the bounded count")
				assert.Equal(t, true, response["has_more"], "truncated results should set has_more")
			},
		},
		{
			name:            "labels pagination stops at the page cap",
			mockedGQLClient: alwaysHasNextPageLabelsClient(t),
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
				assert.Len(t, labels, uiGetMaxPages, "loop should stop at the page cap")
				assert.Equal(t, true, response["has_more"], "truncated results should set has_more")
				// totalCount stays 服务器-reported full count, so it can exceed
				// number of labels 返回ed once 结果 are truncated.
				assert.Equal(t, float64(9999), response["totalCount"])
			},
		},
		{
			name: "reviewers pagination stops at the page cap",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/owner/repo/collaborators": alwaysNextPageHandler(t, []*github.User{{Login: github.Ptr("octocat")}}),
				"GET /repos/owner/repo/teams":         mockResponse(t, http.StatusOK, mockReviewerTeams),
			}),
			requestArgs: map[string]any{
				"method": "reviewers",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, responseText string) {
				var response map[string]any
				require.NoError(t, json.Unmarshal([]byte(responseText), &response))
				users, ok := response["users"].([]any)
				require.True(t, ok, "users should be a list")
				assert.Len(t, users, uiGetMaxPages, "collaborators loop should stop at the page cap")
				assert.Equal(t, true, response["has_more"], "truncating either loop should set has_more")
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
			// Setup deps with REST and/或GraphQL 模拟s
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

			// Create c所有请求
			request := createMCPRequest(tc.requestArgs)

			// C所有处理器
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify 结果
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

func Test_marshalUIGetIssueFields_TrimsForUI(t *testing.T) {
	priorityLow := 1
	priorityHigh := 2
	result, _, err := marshalUIGetIssueFields([]IssueField{
		{
			ID:          "field-1",
			DatabaseID:  123,
			Name:        "Priority",
			Description: "How urgent this is",
			DataType:    "single_select",
			Visibility:  "public",
			Options: []IssueSingleSelectFieldOption{
				{ID: "option-2", Name: "High", Description: "High priority", Color: "red", Priority: &priorityHigh},
				{ID: "option-1", Name: "Low", Description: "Low priority", Color: "blue", Priority: &priorityLow},
				{ID: "option-3", Name: "No priority", Description: "No priority set", Color: "gray"},
			},
		},
		{
			ID:       "field-2",
			Name:     "Unsupported",
			DataType: "iteration",
		},
		{
			ID:       "field-3",
			Name:     "Notes",
			DataType: "text",
		},
	})
	require.NoError(t, err)

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	fields := response["fields"].([]any)
	require.Len(t, fields, 2)
	assert.Equal(t, float64(2), response["totalCount"])

	singleSelectField := fields[0].(map[string]any)
	assert.NotContains(t, singleSelectField, "full_database_id")
	assert.NotContains(t, singleSelectField, "visibility")
	options := singleSelectField["options"].([]any)
	require.Len(t, options, 3)
	assert.Equal(t, "Low", options[0].(map[string]any)["name"])
	assert.Equal(t, "High", options[1].(map[string]any)["name"])
	assert.Equal(t, "No priority", options[2].(map[string]any)["name"])
	assert.NotContains(t, options[0].(map[string]any), "id")
	assert.NotContains(t, options[0].(map[string]any), "priority")

	textField := fields[1].(map[string]any)
	assert.NotContains(t, textField, "options")
}
