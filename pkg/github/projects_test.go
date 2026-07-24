package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
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
			client := mustNewGHClient(t, tc.mockedClient)
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

		client := mustNewGHClient(t, mockedClient)
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
		client := mustNewGHClient(t, mockedClient)
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

func verbosePullRequestProjectItemFixture() map[string]any {
	return map[string]any{
		"id":           1001,
		"node_id":      "PVTI_1",
		"content_type": "PullRequest",
		"item_url":     "https://api.github.com/projectsV2/items/1001",
		"project_url":  "https://api.github.com/orgs/octo-org/projectsV2/1",
		"creator": map[string]any{
			"login":         "creator",
			"id":            999,
			"followers_url": "https://api.github.com/users/creator/followers",
		},
		"content": map[string]any{
			"id":         2002,
			"node_id":    "PR_1",
			"number":     42,
			"title":      "Reduce project item output",
			"body":       "Long pull request body that should not be returned from project item tools.",
			"state":      "closed",
			"html_url":   "https://github.com/cli/cli/pull/42",
			"url":        "https://api.github.com/repos/cli/cli/pulls/42",
			"diff_url":   "https://github.com/cli/cli/pull/42.diff",
			"patch_url":  "https://github.com/cli/cli/pull/42.patch",
			"draft":      false,
			"merged":     true,
			"created_at": "2026-05-07T18:41:21Z",
			"updated_at": "2026-05-07T21:21:57Z",
			"closed_at":  "2026-05-07T21:21:55Z",
			"merged_at":  "2026-05-07T21:21:55Z",
			"user": map[string]any{
				"login":         "octocat",
				"id":            123,
				"followers_url": "https://api.github.com/users/octocat/followers",
			},
			"assignees": []map[string]any{
				{
					"login":      "hubot",
					"events_url": "https://api.github.com/users/hubot/events{/privacy}",
				},
			},
			"labels": []map[string]any{
				{
					"name": "bug",
					"url":  "https://api.github.com/repos/cli/cli/labels/bug",
				},
			},
			"milestone": map[string]any{
				"title":       "v1.0",
				"description": "Verbose milestone description",
			},
			"head": map[string]any{
				"ref": "feature",
				"repo": map[string]any{
					"full_name":   "fork/cli",
					"archive_url": "https://api.github.com/repos/fork/cli/{archive_format}{/ref}",
				},
			},
			"base": map[string]any{
				"ref": "trunk",
				"repo": map[string]any{
					"full_name":   "cli/cli",
					"archive_url": "https://api.github.com/repos/cli/cli/{archive_format}{/ref}",
				},
			},
			"_links": map[string]any{
				"self": map[string]any{
					"href": "https://api.github.com/repos/cli/cli/pulls/42",
				},
			},
			"statuses_url": "https://api.github.com/repos/cli/cli/statuses/abc123",
		},
		"fields": []map[string]any{
			{
				"id":        301,
				"name":      "Status",
				"data_type": "single_select",
				"value": map[string]any{
					"id":          "opt1",
					"name":        "Done",
					"color":       "GREEN",
					"description": "Verbose option description",
				},
			},
		},
		"created_at": "2026-05-28T07:39:37Z",
		"updated_at": "2026-05-28T07:40:15Z",
	}
}

func assertMinimalPullRequestProjectItem(t *testing.T, rawJSON string, item map[string]any) {
	t.Helper()

	assert.Equal(t, float64(1001), item["id"])
	assert.Equal(t, "PVTI_1", item["node_id"])
	assert.Equal(t, "PullRequest", item["content_type"])
	assert.Equal(t, "creator", item["creator"])
	assert.Equal(t, "2026-05-28T07:39:37Z", item["created_at"])
	assert.Equal(t, "2026-05-28T07:40:15Z", item["updated_at"])

	content, ok := item["content"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(42), content["number"])
	assert.Equal(t, "Reduce project item output", content["title"])
	assert.Equal(t, "closed", content["state"])
	assert.Equal(t, "https://github.com/cli/cli/pull/42", content["html_url"])
	assert.Equal(t, "cli/cli", content["repository"])
	assert.Equal(t, "octocat", content["author"])
	assert.Equal(t, true, content["merged"])
	assert.Equal(t, "2026-05-07T18:41:21Z", content["created_at"])
	assert.Equal(t, "2026-05-07T21:21:57Z", content["updated_at"])
	assert.Equal(t, "2026-05-07T21:21:55Z", content["closed_at"])
	assert.Equal(t, "2026-05-07T21:21:55Z", content["merged_at"])
	assert.Equal(t, []any{"hubot"}, content["assignees"])
	assert.Equal(t, []any{"bug"}, content["labels"])
	assert.Equal(t, "v1.0", content["milestone"])

	fields, ok := item["fields"].([]any)
	require.True(t, ok)
	require.Len(t, fields, 1)
	field, ok := fields[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(301), field["id"])
	assert.Equal(t, "Status", field["name"])
	assert.Equal(t, "single_select", field["data_type"])
	value, ok := field["value"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "opt1", value["id"])
	assert.Equal(t, "Done", value["name"])
	assert.Equal(t, "GREEN", value["color"])

	assert.NotContains(t, rawJSON, `"body"`)
	assert.NotContains(t, rawJSON, `"archive_url"`)
	assert.NotContains(t, rawJSON, `"followers_url"`)
	assert.NotContains(t, rawJSON, `"events_url"`)
	assert.NotContains(t, rawJSON, `"_links"`)
	assert.NotContains(t, rawJSON, `"head"`)
	assert.NotContains(t, rawJSON, `"base"`)
	assert.NotContains(t, rawJSON, `"url":`)
	assert.NotContains(t, rawJSON, `"statuses_url"`)
	assert.NotContains(t, rawJSON, `"diff_url"`)
}

func Test_ProjectsList_ListProjectItems(t *testing.T) {
	toolDef := ProjectsList(translations.NullTranslationHelper)

	items := []map[string]any{verbosePullRequestProjectItemFixture()}

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2ItemsByProject: mockResponse(t, http.StatusOK, items),
		})

		client := mustNewGHClient(t, mockedClient)
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
		item, ok := itemsList[0].(map[string]any)
		require.True(t, ok)
		assertMinimalPullRequestProjectItem(t, textContent.Text, item)
	})

	t.Run("rejects fields and field_names together", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
		client := mustNewGHClient(t, mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "list_project_items",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"fields":         []any{"100"},
			"field_names":    []any{"Status"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "provide either 'fields' or 'field_names', not both")
	})
}

func Test_detectOwnerType(t *testing.T) {
	t.Run("uses organization account type", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetUsersByUsername: mockResponse(t, http.StatusOK, map[string]any{
				"login": "github",
				"type":  "Organization",
			}),
		})
		client := mustNewGHClient(t, mockedClient)

		ownerType, err := detectOwnerType(context.Background(), client, "github", 1)

		require.NoError(t, err)
		assert.Equal(t, "org", ownerType)
	})

	t.Run("uses user account type", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetUsersByUsername: mockResponse(t, http.StatusOK, map[string]any{
				"login": "octocat",
				"type":  "User",
			}),
		})
		client := mustNewGHClient(t, mockedClient)

		ownerType, err := detectOwnerType(context.Background(), client, "octocat", 1)

		require.NoError(t, err)
		assert.Equal(t, "user", ownerType)
	})

	t.Run("falls back to project probes", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetUsersProjectsV2ByUsernameByProject: mockResponse(t, http.StatusNotFound, nil),
			GetOrgsProjectsV2ByProject:            mockResponse(t, http.StatusOK, map[string]any{"id": 1}),
		})
		client := mustNewGHClient(t, mockedClient)

		ownerType, err := detectOwnerType(context.Background(), client, "octo-org", 1)

		require.NoError(t, err)
		assert.Equal(t, "org", ownerType)
	})
}

func Test_ProjectsList_IFC_InsidersMode(t *testing.T) {
	toolDef := ProjectsList(translations.NullTranslationHelper)

	t.Run("list_projects joins returned project visibilities", func(t *testing.T) {
		projects := []map[string]any{
			{"id": 1, "node_id": "NODE1", "title": "Public Project", "public": true},
			{"id": 2, "node_id": "NODE2", "title": "Private Project", "public": false},
		}
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2: mockResponse(t, http.StatusOK, projects),
		})
		client := mustNewGHClient(t, mockedClient)
		deps := BaseDeps{
			Client:         client,
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":     "list_projects",
			"owner":      "octo-org",
			"owner_type": "org",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "untrusted", ifcMap["integrity"])
		assert.Equal(t, "private", ifcMap["confidentiality"])
	})

	t.Run("list_project_fields uses project metadata label", func(t *testing.T) {
		fields := []map[string]any{{"id": 101, "name": "Status", "data_type": "single_select"}}
		project := map[string]any{"id": 1, "node_id": "NODE1", "title": "Private Project", "public": false}
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2FieldsByProject: mockResponse(t, http.StatusOK, fields),
			GetOrgsProjectsV2ByProject:       mockResponse(t, http.StatusOK, project),
		})
		client := mustNewGHClient(t, mockedClient)
		deps := BaseDeps{
			Client:         client,
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
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

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "trusted", ifcMap["integrity"])
		assert.Equal(t, "private", ifcMap["confidentiality"])
	})

	t.Run("list_project_items uses project content label", func(t *testing.T) {
		items := []map[string]any{verbosePullRequestProjectItemFixture()}
		project := map[string]any{"id": 1, "node_id": "NODE1", "title": "Private Project", "public": false}
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2ItemsByProject: mockResponse(t, http.StatusOK, items),
			GetOrgsProjectsV2ByProject:      mockResponse(t, http.StatusOK, project),
		})
		client := mustNewGHClient(t, mockedClient)
		deps := BaseDeps{
			Client:         client,
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
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

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "untrusted", ifcMap["integrity"])
		assert.Equal(t, "private", ifcMap["confidentiality"])
	})

	t.Run("list_project_status_updates uses GraphQL project visibility", func(t *testing.T) {
		gqlMockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				statusUpdatesOrgQuery{},
				map[string]any{
					"owner":         githubv4.String("octo-org"),
					"projectNumber": githubv4.Int(1),
					"first":         githubv4.Int(50),
					"after":         (*githubv4.String)(nil),
				},
				githubv4mock.DataResponse(map[string]any{
					"organization": map[string]any{
						"projectV2": map[string]any{
							"public": true,
							"statusUpdates": map[string]any{
								"nodes": []map[string]any{},
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
		deps := BaseDeps{
			Client:         mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient:      githubv4.NewClient(gqlMockedClient),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "list_project_status_updates",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "untrusted", ifcMap["integrity"])
		assert.Equal(t, "public", ifcMap["confidentiality"])
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
	assert.ElementsMatch(t, inputSchema.Required, []string{"method"})
}

func Test_ProjectsGet_GetProject(t *testing.T) {
	toolDef := ProjectsGet(translations.NullTranslationHelper)

	project := map[string]any{"id": 123, "title": "Project Title"}

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2ByProject: mockResponse(t, http.StatusOK, project),
		})

		client := mustNewGHClient(t, mockedClient)
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
		client := mustNewGHClient(t, mockedClient)
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

func Test_ProjectsGet_IFC_InsidersMode(t *testing.T) {
	toolDef := ProjectsGet(translations.NullTranslationHelper)

	t.Run("get_project uses project metadata label", func(t *testing.T) {
		project := map[string]any{"id": 123, "node_id": "NODE1", "title": "Private Project", "public": false}
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2ByProject: mockResponse(t, http.StatusOK, project),
		})
		client := mustNewGHClient(t, mockedClient)
		deps := BaseDeps{
			Client:         client,
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
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

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "trusted", ifcMap["integrity"])
		assert.Equal(t, "private", ifcMap["confidentiality"])
	})

	t.Run("get_project_status_update uses GraphQL project visibility", func(t *testing.T) {
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
						"project":    map[string]any{"public": true},
					},
				}),
			),
		)
		deps := BaseDeps{
			GQLClient:      githubv4.NewClient(gqlMockedClient),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":           "get_project_status_update",
			"status_update_id": "SU_abc123",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "untrusted", ifcMap["integrity"])
		assert.Equal(t, "public", ifcMap["confidentiality"])
	})
}

func Test_ProjectsGet_GetProjectField(t *testing.T) {
	toolDef := ProjectsGet(translations.NullTranslationHelper)

	field := map[string]any{"id": 101, "name": "Status", "data_type": "single_select"}

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2FieldsByProjectByFieldID: mockResponse(t, http.StatusOK, field),
		})

		client := mustNewGHClient(t, mockedClient)
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
		client := mustNewGHClient(t, mockedClient)
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

	item := verbosePullRequestProjectItemFixture()

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2ItemsByProjectByItemID: mockResponse(t, http.StatusOK, item),
		})

		client := mustNewGHClient(t, mockedClient)
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
		assertMinimalPullRequestProjectItem(t, textContent.Text, response)
	})

	t.Run("missing item_id", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
		client := mustNewGHClient(t, mockedClient)
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

	t.Run("rejects fields and field_names together", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
		client := mustNewGHClient(t, mockedClient)
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
			"fields":         []any{"100"},
			"field_names":    []any{"Status"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "provide either 'fields' or 'field_names', not both")
	})
}

func Test_ProjectsWrite(t *testing.T) {
	// Verify tool definition once
	toolDef := ProjectsWrite(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "projects_write", toolDef.Tool.Name)
	assert.Contains(t, toolDef.Tool.Description, "bulk-update many items at once")
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
	assert.Contains(t, inputSchema.Properties, "items")
	assert.ElementsMatch(t, inputSchema.Required, []string{"method", "owner"})

	// Verify DestructiveHint is set
	assert.NotNil(t, toolDef.Tool.Annotations)
	assert.NotNil(t, toolDef.Tool.Annotations.DestructiveHint)
	assert.True(t, *toolDef.Tool.Annotations.DestructiveHint)
}

func Test_ProjectsWrite_UpdateProjectItemsSchema(t *testing.T) {
	inputSchema := ProjectsWrite(translations.NullTranslationHelper).Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties["items"].Description, "prefer it over calling 'update_project_item' in a loop")
	itemSchema := inputSchema.Properties["items"].Items

	assert.Equal(t, "object", itemSchema.Type)
	assert.Empty(t, itemSchema.Properties, "item references should be modeled by oneOf, not flattened properties")
	require.Len(t, itemSchema.OneOf, 3)

	expectedRequired := [][]string{
		{"node_id"},
		{"item_id"},
		{"item_owner", "item_repo", "issue_number"},
	}
	expectedProperties := [][]string{
		{"node_id"},
		{"item_id"},
		{"item_owner", "item_repo", "issue_number"},
	}
	for i, variant := range itemSchema.OneOf {
		properties := make([]string, 0, len(variant.Properties))
		for name := range variant.Properties {
			properties = append(properties, name)
		}
		assert.Equal(t, "object", variant.Type)
		assert.ElementsMatch(t, expectedRequired[i], variant.Required)
		assert.ElementsMatch(t, expectedProperties[i], properties)
		for _, property := range variant.Properties {
			assert.NotEmpty(t, property.Type)
			assert.NotEmpty(t, property.Description)
		}
		require.NotNil(t, variant.AdditionalProperties)
		assert.NotNil(t, variant.AdditionalProperties.Not, "variant must reject additional properties")
	}

	fieldSchema := inputSchema.Properties["updated_field"]
	assert.Equal(t, "object", fieldSchema.Type)
	assert.Contains(t, fieldSchema.Description, "one top-level field/value applies to every item")
	require.Len(t, fieldSchema.OneOf, 2)
	for i, variant := range fieldSchema.OneOf {
		reference := "id"
		if i == 1 {
			reference = "name"
		}
		properties := make([]string, 0, len(variant.Properties))
		for name := range variant.Properties {
			properties = append(properties, name)
		}
		assert.ElementsMatch(t, []string{reference, "value"}, variant.Required)
		assert.ElementsMatch(t, []string{reference, "value"}, properties)
		require.NotNil(t, variant.AdditionalProperties)
		assert.NotNil(t, variant.AdditionalProperties.Not)
		assert.Empty(t, variant.Properties["value"].Type, "an unconstrained value schema accepts any JSON value, including null")
		assert.Empty(t, variant.Properties["value"].Types)
		assert.NotEmpty(t, variant.Properties["value"].Description)
	}
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
							ID             githubv4.ID
							FullDatabaseID string `graphql:"fullDatabaseId"`
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
							"id":             "PVTI_item1",
							"fullDatabaseId": "1001",
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
		assert.Equal(t, float64(1001), response["item_id"])
		assert.Equal(t, "1001", response["full_database_id"])
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
							ID             githubv4.ID
							FullDatabaseID string `graphql:"fullDatabaseId"`
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
							"id":             "PVTI_item2",
							"fullDatabaseId": "1002",
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
		assert.Equal(t, float64(1002), response["item_id"])
		assert.Equal(t, "1002", response["full_database_id"])
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

	updatedItem := verbosePullRequestProjectItemFixture()

	t.Run("success organization", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PatchOrgsProjectsV2ItemsByProjectByItemID: mockResponse(t, http.StatusOK, updatedItem),
		})

		client := mustNewGHClient(t, mockedClient)
		deps := BaseDeps{
			Client: client,
			GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					projectFieldsTestQuery{},
					fieldsQueryVars("octo-org", 1),
					githubv4mock.DataResponse(fieldsResponse([]map[string]any{
						genericFieldNode("PVTF_notes", 101, "Notes", "TEXT"),
					})),
				),
			)),
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
		assertMinimalPullRequestProjectItem(t, textContent.Text, response)
	})

	t.Run("missing updated_field", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})
		client := mustNewGHClient(t, mockedClient)
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

		client := mustNewGHClient(t, mockedClient)
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
		client := mustNewGHClient(t, mockedClient)
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

func TestMinimalProjectFieldValue(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  any
	}{
		{
			name: "select option",
			value: map[string]any{
				"id":          "opt1",
				"name":        "Done",
				"color":       "GREEN",
				"description": "verbose",
			},
			want: minimalProjectOptionValue{
				ID:    "opt1",
				Name:  "Done",
				Color: "GREEN",
			},
		},
		{
			name: "iteration",
			value: map[string]any{
				"id":         "iter1",
				"title":      "Sprint 1",
				"start_date": "2026-05-01",
				"duration":   float64(14),
			},
			want: minimalProjectIterationValue{
				ID:        "iter1",
				Title:     "Sprint 1",
				StartDate: "2026-05-01",
				Duration:  14,
			},
		},
		{
			name: "assignees",
			value: []any{
				map[string]any{"login": "octocat", "followers_url": "https://api.github.com/users/octocat/followers"},
				map[string]any{"login": "hubot", "followers_url": "https://api.github.com/users/hubot/followers"},
			},
			want: []string{"octocat", "hubot"},
		},
		{
			name: "labels",
			value: []any{
				map[string]any{"name": "bug", "url": "https://api.github.com/repos/cli/cli/labels/bug"},
				map[string]any{"name": "help wanted", "url": "https://api.github.com/repos/cli/cli/labels/help%20wanted"},
			},
			want: []string{"bug", "help wanted"},
		},
		{
			name: "repository",
			value: map[string]any{
				"full_name":   "cli/cli",
				"archive_url": "https://api.github.com/repos/cli/cli/{archive_format}{/ref}",
			},
			want: "cli/cli",
		},
		{
			name: "linked pull requests",
			value: []any{
				map[string]any{
					"number":   float64(42),
					"title":    "Reduce output",
					"state":    "open",
					"html_url": "https://github.com/cli/cli/pull/42",
					"base": map[string]any{
						"repo": map[string]any{
							"full_name":   "cli/cli",
							"archive_url": "https://api.github.com/repos/cli/cli/{archive_format}{/ref}",
						},
					},
				},
			},
			want: []minimalProjectPullRequestRef{
				{
					Number:     42,
					Title:      "Reduce output",
					State:      "open",
					HTMLURL:    "https://github.com/cli/cli/pull/42",
					Repository: "cli/cli",
				},
			},
		},
		{
			name: "raw text content",
			value: map[string]any{
				"raw":  "plain text",
				"html": "<p>plain text</p>",
			},
			want: "plain text",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, minimalProjectFieldValue(tc.value))
		})
	}
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
							"public": false,
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
			Client:    mustNewGHClient(t, restClient),
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
						"project":    map[string]any{"public": false},
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
