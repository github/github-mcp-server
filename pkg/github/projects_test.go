package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	gh "github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for consolidated project tools

func Test_ProjectsList(t *testing.T) {
	// Verify tool definition once
	toolDef := ProjectsList(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "projects_list", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "method")
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "owner_type")
	assert.Contains(t, inputSchema.Properties, "project_number")
	assert.Contains(t, inputSchema.Properties, "query")
	assert.Contains(t, inputSchema.Properties, "fields")
	assert.ElementsMatch(t, inputSchema.Required, []string{"method", "owner"})
}

func Test_ProjectsList_ListProjects(t *testing.T) {
	toolDef := ProjectsList(translations.NullTranslationHelper)

	orgProjects := []map[string]any{{"id": 1, "node_id": "NODE1", "title": "Org Project"}}
	userProjects := []map[string]any{{"id": 2, "node_id": "NODE2", "title": "User Project"}}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
		expectedLength int
	}{
		{
			name: "success organization",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsProjectsV2: mockResponse(t, http.StatusOK, orgProjects),
			}),
			requestArgs: map[string]any{
				"method":     "list_projects",
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError:    false,
			expectedLength: 1,
		},
		{
			name: "success user",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetUsersProjectsV2ByUsername: mockResponse(t, http.StatusOK, userProjects),
			}),
			requestArgs: map[string]any{
				"method":     "list_projects",
				"owner":      "octocat",
				"owner_type": "user",
			},
			expectError:    false,
			expectedLength: 1,
		},
		{
			name:         "missing required parameter method",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: method",
		},
		{
			name:         "unknown method",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":     "unknown_method",
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError:    true,
			expectedErrMsg: "unknown method: unknown_method",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			textContent := getTextResult(t, result)

			if tc.expectError {
				if tc.expectedErrMsg != "" {
					assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				}
				return
			}

			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			projects, ok := response["projects"].([]interface{})
			require.True(t, ok)
			assert.Equal(t, tc.expectedLength, len(projects))
		})
	}
}

func Test_ProjectsList_ListProjectFields(t *testing.T) {
	toolDef := ProjectsList(translations.NullTranslationHelper)

	fields := []map[string]any{{"id": 101, "name": "Status", "data_type": "single_select"}}

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2FieldsByProject: mockResponse(t, http.StatusOK, fields),
		})

		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "list_project_fields",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		fieldsList, ok := response["fields"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 1, len(fieldsList))
	})

	t.Run("missing project_number", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":     "list_project_fields",
			"owner":      "octo-org",
			"owner_type": "org",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "missing required parameter: project_number")
	})
}

func Test_ProjectsList_ListProjectItems(t *testing.T) {
	toolDef := ProjectsList(translations.NullTranslationHelper)

	items := []map[string]any{{"id": 1001, "archived_at": nil, "content": map[string]any{"title": "Issue 1"}}}

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2ItemsByProject: mockResponse(t, http.StatusOK, items),
		})

		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "list_project_items",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		itemsList, ok := response["items"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, 1, len(itemsList))
	})
}

func Test_ProjectsGet(t *testing.T) {
	// Verify tool definition once
	toolDef := ProjectsGet(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "projects_get", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "method")
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "owner_type")
	assert.Contains(t, inputSchema.Properties, "project_number")
	assert.Contains(t, inputSchema.Properties, "field_id")
	assert.Contains(t, inputSchema.Properties, "item_id")
	assert.ElementsMatch(t, inputSchema.Required, []string{"method", "owner", "project_number"})
}

func Test_ProjectsGet_GetProject(t *testing.T) {
	toolDef := ProjectsGet(translations.NullTranslationHelper)

	project := map[string]any{"id": 123, "title": "Project Title"}

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2ByProject: mockResponse(t, http.StatusOK, project),
		})

		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "get_project",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["id"])
	})

	t.Run("unknown method", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "unknown_method",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "unknown method: unknown_method")
	})
}

func Test_ProjectsGet_GetProjectField(t *testing.T) {
	toolDef := ProjectsGet(translations.NullTranslationHelper)

	field := map[string]any{"id": 101, "name": "Status", "data_type": "single_select"}

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2FieldsByProjectByFieldID: mockResponse(t, http.StatusOK, field),
		})

		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "get_project_field",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"field_id":       float64(101),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["id"])
	})

	t.Run("missing field_id", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "get_project_field",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "missing required parameter: field_id")
	})
}

func Test_ProjectsGet_GetProjectItem(t *testing.T) {
	toolDef := ProjectsGet(translations.NullTranslationHelper)

	item := map[string]any{"id": 1001, "archived_at": nil, "content": map[string]any{"title": "Issue 1"}}

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2ItemsByProjectByItemID: mockResponse(t, http.StatusOK, item),
		})

		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "get_project_item",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"item_id":        float64(1001),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["id"])
	})

	t.Run("missing item_id", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "get_project_item",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "missing required parameter: item_id")
	})
}

func Test_ProjectsWrite(t *testing.T) {
	// Verify tool definition once
	toolDef := ProjectsWrite(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "projects_write", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "method")
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "owner_type")
	assert.Contains(t, inputSchema.Properties, "project_number")
	assert.Contains(t, inputSchema.Properties, "item_id")
	assert.Contains(t, inputSchema.Properties, "item_type")
	assert.Contains(t, inputSchema.Properties, "item_owner")
	assert.Contains(t, inputSchema.Properties, "item_repo")
	assert.Contains(t, inputSchema.Properties, "issue_number")
	assert.Contains(t, inputSchema.Properties, "pull_request_number")
	assert.Contains(t, inputSchema.Properties, "updated_field")
	assert.ElementsMatch(t, inputSchema.Required, []string{"method", "owner", "project_number"})

	// Verify DestructiveHint is set
	assert.NotNil(t, toolDef.Tool.Annotations)
	assert.NotNil(t, toolDef.Tool.Annotations.DestructiveHint)
	assert.True(t, *toolDef.Tool.Annotations.DestructiveHint)
}

func Test_ProjectsWrite_AddProjectItem(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	t.Run("success organization with issue", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			// Mock resolveIssueNodeID query
			githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Issue struct {
							ID githubv4.ID
						} `graphql:"issue(number: $issueNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}{},
				map[string]any{
					"owner":       githubv4.String("item-owner"),
					"repo":        githubv4.String("item-repo"),
					"issueNumber": githubv4.Int(123),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"issue": map[string]any{
							"id": "I_issue123",
						},
					},
				}),
			),
			// Mock project ID query for org
			githubv4mock.NewQueryMatcher(
				struct {
					Organization struct {
						ProjectV2 struct {
							ID githubv4.ID
						} `graphql:"projectV2(number: $projectNumber)"`
					} `graphql:"organization(login: $owner)"`
				}{},
				map[string]any{
					"owner":         githubv4.String("octo-org"),
					"projectNumber": githubv4.Int(1),
				},
				githubv4mock.DataResponse(map[string]any{
					"organization": map[string]any{
						"projectV2": map[string]any{
							"id": "PVT_project1",
						},
					},
				}),
			),
			// Mock addProjectV2ItemById mutation
			githubv4mock.NewMutationMatcher(
				struct {
					AddProjectV2ItemByID struct {
						Item struct {
							ID githubv4.ID
						}
					} `graphql:"addProjectV2ItemById(input: $input)"`
				}{},
				githubv4.AddProjectV2ItemByIdInput{
					ProjectID: githubv4.ID("PVT_project1"),
					ContentID: githubv4.ID("I_issue123"),
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"addProjectV2ItemById": map[string]any{
						"item": map[string]any{
							"id": "PVTI_item1",
						},
					},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "add_project_item",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"item_owner":     "item-owner",
			"item_repo":      "item-repo",
			"issue_number":   float64(123),
			"item_type":      "issue",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["id"])
		assert.Contains(t, response["message"], "Successfully added")
	})

	t.Run("success user with pull request", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			// Mock resolvePullRequestNodeID query
			githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						PullRequest struct {
							ID githubv4.ID
						} `graphql:"pullRequest(number: $prNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}{},
				map[string]any{
					"owner":    githubv4.String("item-owner"),
					"repo":     githubv4.String("item-repo"),
					"prNumber": githubv4.Int(456),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"pullRequest": map[string]any{
							"id": "PR_pr456",
						},
					},
				}),
			),
			// Mock project ID query for user
			githubv4mock.NewQueryMatcher(
				struct {
					User struct {
						ProjectV2 struct {
							ID githubv4.ID
						} `graphql:"projectV2(number: $projectNumber)"`
					} `graphql:"user(login: $owner)"`
				}{},
				map[string]any{
					"owner":         githubv4.String("octo-user"),
					"projectNumber": githubv4.Int(2),
				},
				githubv4mock.DataResponse(map[string]any{
					"user": map[string]any{
						"projectV2": map[string]any{
							"id": "PVT_project2",
						},
					},
				}),
			),
			// Mock addProjectV2ItemById mutation
			githubv4mock.NewMutationMatcher(
				struct {
					AddProjectV2ItemByID struct {
						Item struct {
							ID githubv4.ID
						}
					} `graphql:"addProjectV2ItemById(input: $input)"`
				}{},
				githubv4.AddProjectV2ItemByIdInput{
					ProjectID: githubv4.ID("PVT_project2"),
					ContentID: githubv4.ID("PR_pr456"),
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"addProjectV2ItemById": map[string]any{
						"item": map[string]any{
							"id": "PVTI_item2",
						},
					},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":              "add_project_item",
			"owner":               "octo-user",
			"owner_type":          "user",
			"project_number":      float64(2),
			"item_owner":          "item-owner",
			"item_repo":           "item-repo",
			"pull_request_number": float64(456),
			"item_type":           "pull_request",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["id"])
		assert.Contains(t, response["message"], "Successfully added")
	})

	t.Run("missing item_type", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient()
		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "add_project_item",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"item_owner":     "item-owner",
			"item_repo":      "item-repo",
			"issue_number":   float64(123),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "missing required parameter: item_type")
	})

	t.Run("invalid item_type", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient()
		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "add_project_item",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"item_owner":     "item-owner",
			"item_repo":      "item-repo",
			"issue_number":   float64(123),
			"item_type":      "invalid_type",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "item_type must be either 'issue' or 'pull_request'")
	})

	t.Run("unknown method", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient()
		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "unknown_method",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "unknown method: unknown_method")
	})
}

func Test_ProjectsWrite_UpdateProjectItem(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	updatedItem := map[string]any{"id": 1001, "archived_at": nil}

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PatchOrgsProjectsV2ItemsByProjectByItemID: mockResponse(t, http.StatusOK, updatedItem),
		})

		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "update_project_item",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"item_id":        float64(1001),
			"updated_field": map[string]any{
				"id":    float64(101),
				"value": "In Progress",
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response["id"])
	})

	t.Run("missing updated_field", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "update_project_item",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"item_id":        float64(1001),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "missing required parameter: updated_field")
	})
}

func Test_ProjectsWrite_DeleteProjectItem(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			DeleteOrgsProjectsV2ItemsByProjectByItemID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		})

		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "delete_project_item",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"item_id":        float64(1001),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "project item successfully deleted")
	})

	t.Run("missing item_id", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
		client := gh.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "delete_project_item",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "missing required parameter: item_id")
	})
}
