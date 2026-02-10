package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	gh "github.com/google/go-github/v82/github"
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
			projects, ok := response["projects"].([]any)
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
		fieldsList, ok := response["fields"].([]any)
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
		itemsList, ok := response["items"].([]any)
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

func Test_ListProjectStatusUpdates(t *testing.T) {
	serverTool := ListProjectStatusUpdates(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_project_status_updates", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.Contains(t, schema.Properties, "project_number")
	assert.Contains(t, schema.Properties, "per_page")
	assert.Contains(t, schema.Properties, "after")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "owner_type", "project_number"})

	t.Run("success user project", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdatesUserQuery{},
				map[string]any{
					"owner":         githubv4.String("octocat"),
					"projectNumber": githubv4.Int(1),
					"first":         githubv4.Int(50),
					"after":         (*githubv4.String)(nil),
				},
				githubv4mock.DataResponse(map[string]any{
					"user": map[string]any{
						"projectV2": map[string]any{
							"statusUpdates": map[string]any{
								"nodes": []map[string]any{
									{
										"id":         "SU_1",
										"body":       "On track",
										"status":     "ON_TRACK",
										"createdAt":  "2026-01-15T10:00:00Z",
										"startDate":  "2026-01-01",
										"targetDate": "2026-03-01",
										"creator":    map[string]any{"login": "octocat"},
									},
									{
										"id":         "SU_2",
										"body":       "At risk",
										"status":     "AT_RISK",
										"createdAt":  "2026-01-10T10:00:00Z",
										"startDate":  "2026-01-01",
										"targetDate": "2026-03-01",
										"creator":    map[string]any{"login": "octocat"},
									},
								},
								"pageInfo": map[string]any{
									"hasNextPage":     false,
									"hasPreviousPage": false,
									"startCursor":     "cursor1",
									"endCursor":       "cursor2",
								},
							},
						},
					},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		updates, ok := response["statusUpdates"].([]any)
		require.True(t, ok)
		assert.Len(t, updates, 2)
		_, hasPageInfo := response["pageInfo"].(map[string]any)
		assert.True(t, hasPageInfo)

		// Verify actual content of returned status updates
		firstUpdate, ok := updates[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "SU_1", firstUpdate["id"])
		assert.Equal(t, "On track", firstUpdate["body"])
		assert.Equal(t, "ON_TRACK", firstUpdate["status"])
		assert.Equal(t, "2026-01-01", firstUpdate["start_date"])
		assert.Equal(t, "2026-03-01", firstUpdate["target_date"])
		creator, ok := firstUpdate["creator"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "octocat", creator["login"])
	})

	t.Run("success org project", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdatesOrgQuery{},
				map[string]any{
					"owner":         githubv4.String("octo-org"),
					"projectNumber": githubv4.Int(5),
					"first":         githubv4.Int(50),
					"after":         (*githubv4.String)(nil),
				},
				githubv4mock.DataResponse(map[string]any{
					"organization": map[string]any{
						"projectV2": map[string]any{
							"statusUpdates": map[string]any{
								"nodes": []map[string]any{
									{
										"id":         "SU_3",
										"body":       "Off track",
										"status":     "OFF_TRACK",
										"createdAt":  "2026-02-01T10:00:00Z",
										"startDate":  "2026-01-01",
										"targetDate": "2026-04-01",
										"creator":    map[string]any{"login": "admin"},
									},
								},
								"pageInfo": map[string]any{
									"hasNextPage":     false,
									"hasPreviousPage": false,
									"startCursor":     "",
									"endCursor":       "",
								},
							},
						},
					},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(5),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		updates, ok := response["statusUpdates"].([]any)
		require.True(t, ok)
		assert.Len(t, updates, 1)
	})

	t.Run("success with pagination cursor", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdatesUserQuery{},
				map[string]any{
					"owner":         githubv4.String("octocat"),
					"projectNumber": githubv4.Int(1),
					"first":         githubv4.Int(10),
					"after":         githubv4.String("cursor_abc"),
				},
				githubv4mock.DataResponse(map[string]any{
					"user": map[string]any{
						"projectV2": map[string]any{
							"statusUpdates": map[string]any{
								"nodes": []map[string]any{},
								"pageInfo": map[string]any{
									"hasNextPage":     false,
									"hasPreviousPage": true,
									"startCursor":     "cursor_abc",
									"endCursor":       "cursor_def",
								},
							},
						},
					},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(1),
			"per_page":       float64(10),
			"after":          "cursor_abc",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		updates, ok := response["statusUpdates"].([]any)
		require.True(t, ok)
		assert.Len(t, updates, 0)
		pageInfo, ok := response["pageInfo"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, pageInfo["hasPreviousPage"])
		assert.Equal(t, "cursor_def", pageInfo["nextCursor"])
		assert.Equal(t, "cursor_abc", pageInfo["prevCursor"])
		// Verify old field names are NOT present
		_, hasEndCursor := pageInfo["endCursor"]
		assert.False(t, hasEndCursor, "should use nextCursor, not endCursor")
		_, hasStartCursor := pageInfo["startCursor"]
		assert.False(t, hasStartCursor, "should use prevCursor, not startCursor")
	})

	t.Run("per_page exceeding max is capped", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdatesUserQuery{},
				map[string]any{
					"owner":         githubv4.String("octocat"),
					"projectNumber": githubv4.Int(1),
					"first":         githubv4.Int(50), // Should be capped to MaxProjectsPerPage (50)
					"after":         (*githubv4.String)(nil),
				},
				githubv4mock.DataResponse(map[string]any{
					"user": map[string]any{
						"projectV2": map[string]any{
							"statusUpdates": map[string]any{
								"nodes":    []map[string]any{},
								"pageInfo": map[string]any{"hasNextPage": false, "hasPreviousPage": false, "startCursor": "", "endCursor": ""},
							},
						},
					},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(1),
			"per_page":       float64(999), // Exceeds max, should be capped
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("graphql error", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdatesUserQuery{},
				map[string]any{
					"owner":         githubv4.String("octocat"),
					"projectNumber": githubv4.Int(1),
					"first":         githubv4.Int(50),
					"after":         (*githubv4.String)(nil),
				},
				githubv4mock.ErrorResponse("something went wrong"),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "failed to list project status updates")
	})

	t.Run("missing required params", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient()
		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)

		// Missing owner
		request := createMCPRequest(map[string]any{
			"owner_type":     "user",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "missing required parameter: owner")

		// Missing owner_type
		request = createMCPRequest(map[string]any{
			"owner":          "octocat",
			"project_number": float64(1),
		})
		result, err = handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "missing required parameter: owner_type")

		// Missing project_number
		request = createMCPRequest(map[string]any{
			"owner":      "octocat",
			"owner_type": "user",
		})
		result, err = handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "missing required parameter: project_number")
	})
}

func Test_GetProjectStatusUpdate(t *testing.T) {
	serverTool := GetProjectStatusUpdate(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_project_status_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "status_update_id")
	assert.ElementsMatch(t, schema.Required, []string{"status_update_id"})

	t.Run("success", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdateNodeQuery{},
				map[string]any{
					"id": githubv4.ID("SU_abc123"),
				},
				githubv4mock.DataResponse(map[string]any{
					"node": map[string]any{
						"id":         "SU_abc123",
						"body":       "Making progress",
						"status":     "ON_TRACK",
						"createdAt":  "2026-01-15T10:00:00Z",
						"startDate":  "2026-01-01",
						"targetDate": "2026-03-01",
						"creator":    map[string]any{"login": "octocat"},
					},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"status_update_id": "SU_abc123",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "SU_abc123", response["id"])
		assert.Equal(t, "Making progress", response["body"])
		assert.Equal(t, "ON_TRACK", response["status"])
		assert.Equal(t, "2026-01-01", response["start_date"])
		assert.Equal(t, "2026-03-01", response["target_date"])
		assert.Contains(t, response["created_at"], "2026-01-15")
		creator, ok := response["creator"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "octocat", creator["login"])
	})

	t.Run("graphql error", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdateNodeQuery{},
				map[string]any{
					"id": githubv4.ID("SU_bad"),
				},
				githubv4mock.ErrorResponse("node not found"),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"status_update_id": "SU_bad",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "failed to get project status update")
	})

	t.Run("missing status_update_id", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient()
		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "missing required parameter: status_update_id")
	})

	t.Run("not a status update node", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdateNodeQuery{},
				map[string]any{
					"id": githubv4.ID("ISSUE_abc123"),
				},
				githubv4mock.DataResponse(map[string]any{
					"node": map[string]any{},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"status_update_id": "ISSUE_abc123",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "node is not a ProjectV2StatusUpdate")
	})
}

func Test_ProjectsList_ListProjectStatusUpdates(t *testing.T) {
	toolDef := ProjectsList(translations.NullTranslationHelper)

	t.Run("success via consolidated tool", func(t *testing.T) {
		// REST mock for detectOwnerType (when owner_type is omitted)
		restClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetUsersProjectsV2ByUsernameByProject: mockResponse(t, http.StatusOK, map[string]any{"id": 1}),
		})

		// GQL mock for listProjectStatusUpdates
		gqlMockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdatesUserQuery{},
				map[string]any{
					"owner":         githubv4.String("octocat"),
					"projectNumber": githubv4.Int(1),
					"first":         githubv4.Int(50),
					"after":         (*githubv4.String)(nil),
				},
				githubv4mock.DataResponse(map[string]any{
					"user": map[string]any{
						"projectV2": map[string]any{
							"statusUpdates": map[string]any{
								"nodes": []map[string]any{
									{
										"id":         "SU_1",
										"body":       "On track",
										"status":     "ON_TRACK",
										"createdAt":  "2026-01-15T10:00:00Z",
										"startDate":  "2026-01-01",
										"targetDate": "2026-03-01",
										"creator":    map[string]any{"login": "octocat"},
									},
								},
								"pageInfo": map[string]any{
									"hasNextPage":     false,
									"hasPreviousPage": false,
									"startCursor":     "",
									"endCursor":       "",
								},
							},
						},
					},
				}),
			),
		)

		gqlClient := githubv4.NewClient(gqlMockedClient)
		deps := BaseDeps{
			Client:    gh.NewClient(restClient),
			GQLClient: gqlClient,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "list_project_status_updates",
			"owner":          "octocat",
			"project_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		updates, ok := response["statusUpdates"].([]any)
		require.True(t, ok)
		assert.Len(t, updates, 1)
	})
}

func Test_ProjectsGet_GetProjectStatusUpdate(t *testing.T) {
	toolDef := ProjectsGet(translations.NullTranslationHelper)

	t.Run("success via consolidated tool", func(t *testing.T) {
		gqlMockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdateNodeQuery{},
				map[string]any{
					"id": githubv4.ID("SU_abc123"),
				},
				githubv4mock.DataResponse(map[string]any{
					"node": map[string]any{
						"id":         "SU_abc123",
						"body":       "On track",
						"status":     "ON_TRACK",
						"createdAt":  "2026-01-15T10:00:00Z",
						"startDate":  "2026-01-01",
						"targetDate": "2026-03-01",
						"creator":    map[string]any{"login": "octocat"},
					},
				}),
			),
		)

		gqlClient := githubv4.NewClient(gqlMockedClient)
		deps := BaseDeps{
			GQLClient: gqlClient,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":           "get_project_status_update",
			"owner":            "octocat",
			"project_number":   float64(1),
			"status_update_id": "SU_abc123",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "SU_abc123", response["id"])
		assert.Equal(t, "On track", response["body"])
	})
}

func Test_CreateProjectStatusUpdate(t *testing.T) {
	serverTool := CreateProjectStatusUpdate(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "create_project_status_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.Contains(t, schema.Properties, "project_number")
	assert.Contains(t, schema.Properties, "body")
	assert.Contains(t, schema.Properties, "status")
	assert.Contains(t, schema.Properties, "start_date")
	assert.Contains(t, schema.Properties, "target_date")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "owner_type", "project_number"})

	t.Run("success with all fields", func(t *testing.T) {
		bodyStr := githubv4.String("Project is going well")
		statusStr := githubv4.String("ON_TRACK")
		startDateStr := githubv4.String("2026-01-01")
		targetDateStr := githubv4.String("2026-06-30")

		mockedClient := githubv4mock.NewMockedHTTPClient(
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
					"owner":         githubv4.String("octocat"),
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
			// Mock createProjectV2StatusUpdate mutation
			githubv4mock.NewMutationMatcher(
				struct {
					CreateProjectV2StatusUpdate struct {
						StatusUpdate statusUpdateNode
					} `graphql:"createProjectV2StatusUpdate(input: $input)"`
				}{},
				CreateProjectV2StatusUpdateInput{
					ProjectID:  githubv4.ID("PVT_project2"),
					Body:       &bodyStr,
					Status:     &statusStr,
					StartDate:  &startDateStr,
					TargetDate: &targetDateStr,
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"createProjectV2StatusUpdate": map[string]any{
						"statusUpdate": map[string]any{
							"id":         "PVTSU_su001",
							"body":       "Project is going well",
							"status":     "ON_TRACK",
							"createdAt":  "2026-02-09T12:00:00Z",
							"startDate":  "2026-01-01",
							"targetDate": "2026-06-30",
							"creator":    map[string]any{"login": "octocat"},
						},
					},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(2),
			"body":           "Project is going well",
			"status":         "ON_TRACK",
			"start_date":     "2026-01-01",
			"target_date":    "2026-06-30",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "PVTSU_su001", response["id"])
		assert.Equal(t, "Project is going well", response["body"])
		assert.Equal(t, "ON_TRACK", response["status"])
		assert.Equal(t, "2026-01-01", response["start_date"])
		assert.Equal(t, "2026-06-30", response["target_date"])
		creator, ok := response["creator"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "octocat", creator["login"])
	})

	t.Run("success with minimal fields", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
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
					"owner":         githubv4.String("octocat"),
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
			// Mock createProjectV2StatusUpdate mutation with minimal input
			githubv4mock.NewMutationMatcher(
				struct {
					CreateProjectV2StatusUpdate struct {
						StatusUpdate statusUpdateNode
					} `graphql:"createProjectV2StatusUpdate(input: $input)"`
				}{},
				CreateProjectV2StatusUpdateInput{
					ProjectID: githubv4.ID("PVT_project2"),
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"createProjectV2StatusUpdate": map[string]any{
						"statusUpdate": map[string]any{
							"id":        "PVTSU_su002",
							"createdAt": "2026-02-09T12:00:00Z",
						},
					},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(2),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "PVTSU_su002", response["id"])
	})

	t.Run("invalid status", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient()
		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(2),
			"status":         "INVALID_STATUS",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "invalid status")
	})

	t.Run("success org owner_type", func(t *testing.T) {
		bodyStr := githubv4.String("Org project update")
		statusStr := githubv4.String("ON_TRACK")

		mockedClient := githubv4mock.NewMockedHTTPClient(
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
					"projectNumber": githubv4.Int(5),
				},
				githubv4mock.DataResponse(map[string]any{
					"organization": map[string]any{
						"projectV2": map[string]any{
							"id": "PVT_org_project5",
						},
					},
				}),
			),
			// Mock createProjectV2StatusUpdate mutation
			githubv4mock.NewMutationMatcher(
				struct {
					CreateProjectV2StatusUpdate struct {
						StatusUpdate statusUpdateNode
					} `graphql:"createProjectV2StatusUpdate(input: $input)"`
				}{},
				CreateProjectV2StatusUpdateInput{
					ProjectID: githubv4.ID("PVT_org_project5"),
					Body:      &bodyStr,
					Status:    &statusStr,
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"createProjectV2StatusUpdate": map[string]any{
						"statusUpdate": map[string]any{
							"id":        "PVTSU_org_su001",
							"body":      "Org project update",
							"status":    "ON_TRACK",
							"createdAt": "2026-02-09T12:00:00Z",
							"creator":   map[string]any{"login": "admin"},
						},
					},
				}),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(5),
			"body":           "Org project update",
			"status":         "ON_TRACK",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "PVTSU_org_su001", response["id"])
		assert.Equal(t, "Org project update", response["body"])
		assert.Equal(t, "ON_TRACK", response["status"])
	})

	t.Run("graphql mutation error", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			// Mock project ID query for user - succeeds
			githubv4mock.NewQueryMatcher(
				struct {
					User struct {
						ProjectV2 struct {
							ID githubv4.ID
						} `graphql:"projectV2(number: $projectNumber)"`
					} `graphql:"user(login: $owner)"`
				}{},
				map[string]any{
					"owner":         githubv4.String("octocat"),
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
			// Mock createProjectV2StatusUpdate mutation - fails
			githubv4mock.NewMutationMatcher(
				struct {
					CreateProjectV2StatusUpdate struct {
						StatusUpdate statusUpdateNode
					} `graphql:"createProjectV2StatusUpdate(input: $input)"`
				}{},
				CreateProjectV2StatusUpdateInput{
					ProjectID: githubv4.ID("PVT_project2"),
				},
				nil,
				githubv4mock.ErrorResponse("mutation failed: insufficient permissions"),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(2),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, ProjectStatusUpdateCreateFailedError)
	})

	t.Run("project ID resolution failure", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient(
			// Mock project ID query for user - fails
			githubv4mock.NewQueryMatcher(
				struct {
					User struct {
						ProjectV2 struct {
							ID githubv4.ID
						} `graphql:"projectV2(number: $projectNumber)"`
					} `graphql:"user(login: $owner)"`
				}{},
				map[string]any{
					"owner":         githubv4.String("nonexistent"),
					"projectNumber": githubv4.Int(999),
				},
				githubv4mock.ErrorResponse("Could not resolve to a User with the login of 'nonexistent'"),
			),
		)

		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "nonexistent",
			"owner_type":     "user",
			"project_number": float64(999),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, ProjectResolveIDFailedError)
	})

	t.Run("invalid start_date format", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient()
		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(2),
			"start_date":     "not-a-date",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "invalid start_date")
		assert.Contains(t, textContent.Text, "YYYY-MM-DD")
	})

	t.Run("invalid target_date format", func(t *testing.T) {
		mockedClient := githubv4mock.NewMockedHTTPClient()
		client := githubv4.NewClient(mockedClient)
		deps := BaseDeps{
			GQLClient: client,
		}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(2),
			"target_date":    "01/15/2026",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "invalid target_date")
		assert.Contains(t, textContent.Text, "YYYY-MM-DD")
	})
}

func Test_ProjectsWrite_CreateProjectStatusUpdate(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	t.Run("success via consolidated tool", func(t *testing.T) {
		bodyStr := githubv4.String("Consolidated test")
		statusStr := githubv4.String("AT_RISK")

		gqlMockedClient := githubv4mock.NewMockedHTTPClient(
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
					"owner":         githubv4.String("octocat"),
					"projectNumber": githubv4.Int(3),
				},
				githubv4mock.DataResponse(map[string]any{
					"user": map[string]any{
						"projectV2": map[string]any{
							"id": "PVT_project3",
						},
					},
				}),
			),
			// Mock createProjectV2StatusUpdate mutation
			githubv4mock.NewMutationMatcher(
				struct {
					CreateProjectV2StatusUpdate struct {
						StatusUpdate statusUpdateNode
					} `graphql:"createProjectV2StatusUpdate(input: $input)"`
				}{},
				CreateProjectV2StatusUpdateInput{
					ProjectID: githubv4.ID("PVT_project3"),
					Body:      &bodyStr,
					Status:    &statusStr,
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"createProjectV2StatusUpdate": map[string]any{
						"statusUpdate": map[string]any{
							"id":        "PVTSU_su003",
							"body":      "Consolidated test",
							"status":    "AT_RISK",
							"createdAt": "2026-02-09T12:00:00Z",
							"creator":   map[string]any{"login": "octocat"},
						},
					},
				}),
			),
		)

		gqlClient := githubv4.NewClient(gqlMockedClient)
		deps := BaseDeps{
			GQLClient: gqlClient,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "create_project_status_update",
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(3),
			"body":           "Consolidated test",
			"status":         "AT_RISK",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "PVTSU_su003", response["id"])
		assert.Equal(t, "Consolidated test", response["body"])
		assert.Equal(t, "AT_RISK", response["status"])
	})
}
