package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/translations"
	gh "github.com/google/go-github/v82/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ProjectsWrite_CreateProject(t *testing.T) {
	t.Parallel()

	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	t.Run("success user project", func(t *testing.T) {
		t.Parallel()

		mockedClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				struct {
					User struct {
						ID string
					} `graphql:"user(login: $login)"`
				}{},
				map[string]any{
					"login": githubv4.String("octocat"),
				},
				githubv4mock.DataResponse(map[string]any{
					"user": map[string]any{
						"id": "U_octocat",
					},
				}),
			),
			githubv4mock.NewMutationMatcher(
				struct {
					CreateProjectV2 struct {
						ProjectV2 struct {
							ID     string
							Number int
							Title  string
							URL    string
						}
					} `graphql:"createProjectV2(input: $input)"`
				}{},
				githubv4.CreateProjectV2Input{
					OwnerID: githubv4.ID("U_octocat"),
					Title:   githubv4.String("New Project"),
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"createProjectV2": map[string]any{
						"projectV2": map[string]any{
							"id":     "PVT_project123",
							"number": 1,
							"title":  "New Project",
							"url":    "https://github.com/users/octocat/projects/1",
						},
					},
				}),
			),
		)

		deps := BaseDeps{
			GQLClient: githubv4.NewClient(mockedClient),
			Obsv:      stubExporters(),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":     "create_project",
			"owner":      "octocat",
			"owner_type": "user",
			"title":      "New Project",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "PVT_project123", response["id"])
		assert.Equal(t, float64(1), response["number"])
		assert.Equal(t, "New Project", response["title"])
		assert.Equal(t, "https://github.com/users/octocat/projects/1", response["url"])
	})

	t.Run("missing owner_type returns error", func(t *testing.T) {
		t.Parallel()

		deps := BaseDeps{
			GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
			Obsv:      stubExporters(),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method": "create_project",
			"owner":  "octocat",
			"title":  "New Project",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "owner_type is required")
	})

	t.Run("invalid owner_type returns error", func(t *testing.T) {
		t.Parallel()

		deps := BaseDeps{
			GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
			Obsv:      stubExporters(),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":     "create_project",
			"owner":      "octocat",
			"owner_type": "invalid",
			"title":      "New Project",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "invalid owner_type")
		assert.Contains(t, textContent.Text, "must be")
	})
}

// resolveProjectNodeIDOrgMatcher returns a GraphQL query matcher for resolving
// an org project node ID via resolveProjectNodeID.
func resolveProjectNodeIDOrgMatcher(owner string, projectNumber int, nodeID string) githubv4mock.Matcher {
	return githubv4mock.NewQueryMatcher(
		struct {
			Organization struct {
				ProjectV2 struct {
					ID githubv4.ID
				} `graphql:"projectV2(number: $projectNumber)"`
			} `graphql:"organization(login: $owner)"`
		}{},
		map[string]any{
			"owner":         githubv4.String(owner),
			"projectNumber": githubv4.Int(int32(projectNumber)), //nolint:gosec // test constant
		},
		githubv4mock.DataResponse(map[string]any{
			"organization": map[string]any{
				"projectV2": map[string]any{
					"id": nodeID,
				},
			},
		}),
	)
}

func createFieldMatcher() githubv4mock.Matcher {
	return githubv4mock.NewMutationMatcher(
		struct {
			CreateProjectV2Field struct {
				ProjectV2Field struct {
					ProjectV2IterationField struct {
						ID   string
						Name string
					} `graphql:"... on ProjectV2IterationField"`
				}
			} `graphql:"createProjectV2Field(input: $input)"`
		}{},
		githubv4.CreateProjectV2FieldInput{
			ProjectID: githubv4.ID("PVT_project1"),
			DataType:  githubv4.ProjectV2CustomFieldType("ITERATION"),
			Name:      githubv4.String("Sprint"),
		},
		nil,
		githubv4mock.DataResponse(map[string]any{
			"createProjectV2Field": map[string]any{
				"projectV2Field": map[string]any{
					"id":   "PVTIF_field1",
					"name": "Sprint",
				},
			},
		}),
	)
}

func updateFieldIterationResponse() githubv4mock.GQLResponse {
	return githubv4mock.DataResponse(map[string]any{
		"updateProjectV2Field": map[string]any{
			"projectV2Field": map[string]any{
				"id":   "PVTIF_field1",
				"name": "Sprint",
				"configuration": map[string]any{
					"iterations": []any{
						map[string]any{
							"id":        "PVTI_iter1",
							"title":     "Sprint 1",
							"startDate": "2025-01-20",
							"duration":  7,
						},
					},
				},
			},
		},
	})
}

func Test_ProjectsWrite_CreateIterationField(t *testing.T) {
	t.Parallel()

	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	t.Run("success with iterations", func(t *testing.T) {
		t.Parallel()

		mockGQLClient := githubv4mock.NewMockedHTTPClient(
			resolveProjectNodeIDOrgMatcher("octo-org", 1, "PVT_project1"),
			createFieldMatcher(),
			githubv4mock.NewMutationMatcher(
				struct {
					UpdateProjectV2Field struct {
						ProjectV2Field struct {
							ProjectV2IterationField struct {
								ID            string
								Name          string
								Configuration struct {
									Iterations []struct {
										ID        string
										Title     string
										StartDate string
										Duration  int
									}
								}
							} `graphql:"... on ProjectV2IterationField"`
						}
					} `graphql:"updateProjectV2Field(input: $input)"`
				}{},
				UpdateProjectV2FieldInput{
					ProjectID: githubv4.ID("PVT_project1"),
					FieldID:   githubv4.ID("PVTIF_field1"),
					IterationConfiguration: &ProjectV2IterationFieldConfigurationInput{
						Duration:  githubv4.Int(7),
						StartDate: githubv4.Date{Time: time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)},
						Iterations: &[]ProjectV2IterationFieldIterationInput{
							{
								Title:     githubv4.String("Sprint 1"),
								StartDate: githubv4.Date{Time: time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)},
								Duration:  githubv4.Int(7),
							},
						},
					},
				},
				nil,
				updateFieldIterationResponse(),
			),
		)

		deps := BaseDeps{
			GQLClient: githubv4.NewClient(mockGQLClient),
			Obsv:      stubExporters(),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "create_iteration_field",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"field_name":     "Sprint",
			"duration":       float64(7),
			"start_date":     "2025-01-20",
			"iterations": []any{
				map[string]any{
					"title":      "Sprint 1",
					"start_date": "2025-01-20",
					"duration":   float64(7),
				},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "PVTIF_field1", response["id"])
	})

	t.Run("success without iterations", func(t *testing.T) {
		t.Parallel()

		mockGQLClient := githubv4mock.NewMockedHTTPClient(
			resolveProjectNodeIDOrgMatcher("octo-org", 1, "PVT_project1"),
			createFieldMatcher(),
			githubv4mock.NewMutationMatcher(
				struct {
					UpdateProjectV2Field struct {
						ProjectV2Field struct {
							ProjectV2IterationField struct {
								ID            string
								Name          string
								Configuration struct {
									Iterations []struct {
										ID        string
										Title     string
										StartDate string
										Duration  int
									}
								}
							} `graphql:"... on ProjectV2IterationField"`
						}
					} `graphql:"updateProjectV2Field(input: $input)"`
				}{},
				UpdateProjectV2FieldInput{
					ProjectID: githubv4.ID("PVT_project1"),
					FieldID:   githubv4.ID("PVTIF_field1"),
					IterationConfiguration: &ProjectV2IterationFieldConfigurationInput{
						Duration:  githubv4.Int(7),
						StartDate: githubv4.Date{Time: time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)},
					},
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"updateProjectV2Field": map[string]any{
						"projectV2Field": map[string]any{
							"id":   "PVTIF_field1",
							"name": "Sprint",
							"configuration": map[string]any{
								"iterations": []any{},
							},
						},
					},
				}),
			),
		)

		deps := BaseDeps{
			GQLClient: githubv4.NewClient(mockGQLClient),
			Obsv:      stubExporters(),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "create_iteration_field",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(1),
			"field_name":     "Sprint",
			"duration":       float64(7),
			"start_date":     "2025-01-20",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "PVTIF_field1", response["id"])
	})

	t.Run("success with auto-detected owner_type", func(t *testing.T) {
		t.Parallel()

		// detectOwnerType uses REST to probe user first, then org
		mockRESTClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetUsersProjectsV2ByUsernameByProject: mockResponse(t, http.StatusNotFound, nil),
			GetOrgsProjectsV2ByProject: mockResponse(t, http.StatusOK, map[string]any{
				"id":      1,
				"node_id": "PVT_project1",
				"title":   "Org Project",
			}),
		})

		mockGQLClient := githubv4mock.NewMockedHTTPClient(
			resolveProjectNodeIDOrgMatcher("octo-org", 1, "PVT_project1"),
			createFieldMatcher(),
			githubv4mock.NewMutationMatcher(
				struct {
					UpdateProjectV2Field struct {
						ProjectV2Field struct {
							ProjectV2IterationField struct {
								ID            string
								Name          string
								Configuration struct {
									Iterations []struct {
										ID        string
										Title     string
										StartDate string
										Duration  int
									}
								}
							} `graphql:"... on ProjectV2IterationField"`
						}
					} `graphql:"updateProjectV2Field(input: $input)"`
				}{},
				UpdateProjectV2FieldInput{
					ProjectID: githubv4.ID("PVT_project1"),
					FieldID:   githubv4.ID("PVTIF_field1"),
					IterationConfiguration: &ProjectV2IterationFieldConfigurationInput{
						Duration:  githubv4.Int(14),
						StartDate: githubv4.Date{Time: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)},
					},
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"updateProjectV2Field": map[string]any{
						"projectV2Field": map[string]any{
							"id":   "PVTIF_field1",
							"name": "Sprint",
							"configuration": map[string]any{
								"iterations": []any{},
							},
						},
					},
				}),
			),
		)

		deps := BaseDeps{
			Client:    gh.NewClient(mockRESTClient),
			GQLClient: githubv4.NewClient(mockGQLClient),
			Obsv:      stubExporters(),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "create_iteration_field",
			"owner":          "octo-org",
			"project_number": float64(1),
			"field_name":     "Sprint",
			"duration":       float64(14),
			"start_date":     "2025-02-01",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "PVTIF_field1", response["id"])
	})
}
