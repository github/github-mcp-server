package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/translations"
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

func resolveProjectNodeIDUserMatcher(owner string, projectNumber int, nodeID string) githubv4mock.Matcher {
	return githubv4mock.NewQueryMatcher(
		struct {
			User struct {
				ProjectV2 struct {
					ID githubv4.ID
				} `graphql:"projectV2(number: $projectNumber)"`
			} `graphql:"user(login: $owner)"`
		}{},
		map[string]any{
			"owner":         githubv4.String(owner),
			"projectNumber": githubv4.Int(int32(projectNumber)), //nolint:gosec // test constant
		},
		githubv4mock.DataResponse(map[string]any{
			"user": map[string]any{
				"projectV2": map[string]any{
					"id": nodeID,
				},
			},
		}),
	)
}

func projectViewParentMatcher(viewID, projectID string) githubv4mock.Matcher {
	return githubv4mock.NewQueryMatcher(
		projectViewParentQuery{},
		map[string]any{"id": githubv4.ID(viewID)},
		githubv4mock.DataResponse(map[string]any{
			"node": map[string]any{
				"id":      viewID,
				"project": map[string]any{"id": projectID},
			},
		}),
	)
}

func projectViewParentErrorMatcher(viewID, message string) githubv4mock.Matcher {
	return githubv4mock.NewQueryMatcher(
		projectViewParentQuery{},
		map[string]any{"id": githubv4.ID(viewID)},
		githubv4mock.ErrorResponse(message),
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
				} `graphql:"projectV2Field"`
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
						} `graphql:"projectV2Field"`
					} `graphql:"updateProjectV2Field(input: $input)"`
				}{},
				UpdateProjectV2FieldInput{
					FieldID: githubv4.ID("PVTIF_field1"),
					IterationConfiguration: &ProjectV2IterationFieldConfigurationInput{
						Duration:  githubv4.Int(7),
						StartDate: githubv4.Date{Time: time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)},
						Iterations: []ProjectV2IterationFieldIterationInput{
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
			"method":             "create_iteration_field",
			"owner":              "octo-org",
			"owner_type":         "org",
			"project_number":     float64(1),
			"field_name":         "Sprint",
			"iteration_duration": float64(7),
			"start_date":         "2025-01-20",
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
						} `graphql:"projectV2Field"`
					} `graphql:"updateProjectV2Field(input: $input)"`
				}{},
				UpdateProjectV2FieldInput{
					FieldID: githubv4.ID("PVTIF_field1"),
					IterationConfiguration: &ProjectV2IterationFieldConfigurationInput{
						Duration:   githubv4.Int(7),
						StartDate:  githubv4.Date{Time: time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)},
						Iterations: []ProjectV2IterationFieldIterationInput{},
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
			"method":             "create_iteration_field",
			"owner":              "octo-org",
			"owner_type":         "org",
			"project_number":     float64(1),
			"field_name":         "Sprint",
			"iteration_duration": float64(7),
			"start_date":         "2025-01-20",
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
						} `graphql:"projectV2Field"`
					} `graphql:"updateProjectV2Field(input: $input)"`
				}{},
				UpdateProjectV2FieldInput{
					FieldID: githubv4.ID("PVTIF_field1"),
					IterationConfiguration: &ProjectV2IterationFieldConfigurationInput{
						Duration:   githubv4.Int(14),
						StartDate:  githubv4.Date{Time: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)},
						Iterations: []ProjectV2IterationFieldIterationInput{},
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
			Client:    mustNewGHClient(t, mockRESTClient),
			GQLClient: githubv4.NewClient(mockGQLClient),
			Obsv:      stubExporters(),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":             "create_iteration_field",
			"owner":              "octo-org",
			"project_number":     float64(1),
			"field_name":         "Sprint",
			"iteration_duration": float64(14),
			"start_date":         "2025-02-01",
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

func Test_ProjectsList_ListProjectViews(t *testing.T) {
	toolDef := ProjectsList(translations.NullTranslationHelper)

	t.Run("lists organization views with forward pagination and IFC", func(t *testing.T) {
		first := githubv4.Int(2)
		after := githubv4.String("after-cursor")
		matcher := githubv4mock.NewQueryMatcher(
			projectViewsOrgQuery{},
			map[string]any{
				"owner":         githubv4.String("octo-org"),
				"projectNumber": githubv4.Int(7),
				"first":         &first,
				"after":         &after,
				"last":          (*githubv4.Int)(nil),
				"before":        (*githubv4.String)(nil),
			},
			githubv4mock.DataResponse(map[string]any{
				"organization": map[string]any{
					"projectV2": map[string]any{
						"id":     "PVT_project7",
						"public": false,
						"views": map[string]any{
							"nodes": []map[string]any{
								{
									"id":     "PVTV_view1",
									"number": 1,
									"name":   "Ready work",
									"layout": "TABLE_LAYOUT",
									"filter": "status:Ready",
								},
							},
							"pageInfo": map[string]any{
								"hasNextPage":     true,
								"hasPreviousPage": false,
								"startCursor":     "start-cursor",
								"endCursor":       "end-cursor",
							},
						},
					},
				},
			}),
		)
		matcher.Variables["first"] = first
		matcher.Variables["after"] = after
		gqlClient := githubv4mock.NewMockedHTTPClient(
			matcher,
		)
		deps := BaseDeps{
			Client:         mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient:      githubv4.NewClient(gqlClient),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "list_project_views",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"per_page":       float64(2),
			"after":          "after-cursor",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, getTextResult(t, result).Text)

		var response struct {
			Views    []MinimalProjectView `json:"views"`
			PageInfo map[string]any       `json:"pageInfo"`
		}
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		require.Len(t, response.Views, 1)
		assert.Equal(t, MinimalProjectView{
			ID:     "PVTV_view1",
			Number: 1,
			Name:   "Ready work",
			Layout: "table",
			Filter: "status:Ready",
		}, response.Views[0])
		assert.Equal(t, "end-cursor", response.PageInfo["nextCursor"])
		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "untrusted", ifcMap["integrity"])
		assert.Equal(t, "private", ifcMap["confidentiality"])
	})

	t.Run("lists user views with backward pagination", func(t *testing.T) {
		last := githubv4.Int(3)
		before := githubv4.String("before-cursor")
		matcher := githubv4mock.NewQueryMatcher(
			projectViewsUserQuery{},
			map[string]any{
				"owner":         githubv4.String("octocat"),
				"projectNumber": githubv4.Int(8),
				"first":         (*githubv4.Int)(nil),
				"after":         (*githubv4.String)(nil),
				"last":          &last,
				"before":        &before,
			},
			githubv4mock.DataResponse(map[string]any{
				"user": map[string]any{
					"projectV2": map[string]any{
						"id":     "PVT_project8",
						"public": true,
						"views": map[string]any{
							"nodes": []map[string]any{},
							"pageInfo": map[string]any{
								"hasNextPage":     false,
								"hasPreviousPage": true,
								"startCursor":     "previous-cursor",
								"endCursor":       "",
							},
						},
					},
				},
			}),
		)
		matcher.Variables["last"] = last
		matcher.Variables["before"] = before
		gqlClient := githubv4mock.NewMockedHTTPClient(
			matcher,
		)
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(gqlClient),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "list_project_views",
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(8),
			"per_page":       float64(3),
			"before":         "before-cursor",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, getTextResult(t, result).Text)

		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
		pageInfo := response["pageInfo"].(map[string]any)
		assert.Equal(t, "previous-cursor", pageInfo["prevCursor"])
	})

	t.Run("rejects conflicting cursors", func(t *testing.T) {
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "list_project_views",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"after":          "a",
			"before":         "b",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "provide either 'after' or 'before'")
	})
}

func Test_ProjectsGet_GetProjectView(t *testing.T) {
	toolDef := ProjectsGet(translations.NullTranslationHelper)

	t.Run("gets a private project view by node ID", func(t *testing.T) {
		gqlClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				projectViewNodeQuery{},
				map[string]any{"id": githubv4.ID("PVTV_view1")},
				githubv4mock.DataResponse(map[string]any{
					"node": map[string]any{
						"id":      "PVTV_view1",
						"number":  1,
						"name":    "Ready work",
						"layout":  "BOARD_LAYOUT",
						"filter":  "status:Ready",
						"project": map[string]any{"public": false},
					},
				}),
			),
		)
		deps := BaseDeps{
			GQLClient:      githubv4.NewClient(gqlClient),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":  "get_project_view",
			"view_id": "PVTV_view1",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var view MinimalProjectView
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &view))
		assert.Equal(t, "PVTV_view1", view.ID)
		assert.Equal(t, "board", view.Layout)
		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "private", ifcMap["confidentiality"])
	})

	t.Run("rejects a missing or wrong node type", func(t *testing.T) {
		gqlClient := githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				projectViewNodeQuery{},
				map[string]any{"id": githubv4.ID("I_issue1")},
				githubv4mock.DataResponse(map[string]any{"node": map[string]any{}}),
			),
		)
		deps := BaseDeps{GQLClient: githubv4.NewClient(gqlClient)}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":  "get_project_view",
			"view_id": "I_issue1",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "node is not a ProjectV2View or was not found")
	})
}

func Test_ProjectsWrite_CreateProjectView(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	t.Run("creates organization view with filter and visible fields", func(t *testing.T) {
		restClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /orgs/{org}/projectsV2/{project}/views": func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/orgs/octo-org/projectsV2/7/views", r.URL.Path)
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, "Ready work", body["name"])
				assert.Equal(t, "table", body["layout"])
				assert.Equal(t, "status:Ready", body["filter"])
				assert.Equal(t, []any{float64(101), float64(202)}, body["visible_fields"])
				mockResponse(t, http.StatusCreated, map[string]any{
					"node_id":        "PVTV_view1",
					"number":         1,
					"name":           "Ready work",
					"layout":         "table",
					"filter":         "status:Ready",
					"visible_fields": []int64{101, 202},
				})(w, r)
			},
		})
		deps := BaseDeps{
			Client:    mustNewGHClient(t, restClient),
			GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "create_project_view",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"name":           "Ready work",
			"layout":         "table",
			"filter":         "status:Ready",
			"visible_fields": []any{"101", "202"},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var view MinimalProjectView
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &view))
		assert.Equal(t, "PVTV_view1", view.ID)
		assert.Equal(t, []int64{101, 202}, view.VisibleFields)
	})

	t.Run("creates a user view by login", func(t *testing.T) {
		restClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /users/{user_id}/projectsV2/{project}/views": func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/users/octocat/projectsV2/8/views", r.URL.Path)
				mockResponse(t, http.StatusCreated, map[string]any{
					"node_id": "PVTV_view2",
					"number":  2,
					"name":    "Board",
					"layout":  "board",
				})(w, r)
			},
		})
		deps := BaseDeps{
			Client:    mustNewGHClient(t, restClient),
			GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "create_project_view",
			"owner":          "octocat",
			"owner_type":     "user",
			"project_number": float64(8),
			"name":           "Board",
			"layout":         "board",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, `"id":"PVTV_view2"`)
	})

	t.Run("auto-detects an organization owner", func(t *testing.T) {
		restClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetUsersByUsername: mockResponse(t, http.StatusOK, map[string]any{
				"id":   99,
				"type": "Organization",
			}),
			"POST /orgs/{org}/projectsV2/{project}/views": mockResponse(t, http.StatusCreated, map[string]any{
				"node_id": "PVTV_view3",
				"number":  3,
				"name":    "Table",
				"layout":  "table",
			}),
		})
		deps := BaseDeps{
			Client:    mustNewGHClient(t, restClient),
			GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "create_project_view",
			"owner":          "octo-org",
			"project_number": float64(9),
			"name":           "Table",
			"layout":         "table",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("rejects visible fields for roadmap layout", func(t *testing.T) {
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "create_project_view",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"name":           "Roadmap",
			"layout":         "roadmap",
			"visible_fields": []any{"101"},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "visible_fields is not supported for roadmap views")
	})
}

func Test_ProjectsWrite_UpdateProjectView(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	t.Run("updates only the supplied name", func(t *testing.T) {
		name := githubv4.String("Renamed")
		gqlClient := githubv4mock.NewMockedHTTPClient(
			resolveProjectNodeIDOrgMatcher("octo-org", 7, "PVT_project7"),
			projectViewParentMatcher("PVTV_view1", "PVT_project7"),
			githubv4mock.NewMutationMatcher(
				struct {
					UpdateProjectV2View struct {
						ProjectV2View projectViewNode `graphql:"projectV2View"`
					} `graphql:"updateProjectV2View(input: $input)"`
				}{},
				UpdateProjectV2ViewInput{
					ViewID: githubv4.ID("PVTV_view1"),
					Name:   &name,
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"updateProjectV2View": map[string]any{
						"projectV2View": map[string]any{
							"id":     "PVTV_view1",
							"number": 1,
							"name":   "Renamed",
							"layout": "TABLE_LAYOUT",
							"filter": "status:Ready",
						},
					},
				}),
			),
		)
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(gqlClient),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "update_project_view",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"view_id":        "PVTV_view1",
			"name":           "Renamed",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, `"name":"Renamed"`)
	})

	t.Run("sends an explicit empty filter to clear it", func(t *testing.T) {
		filter := githubv4.String("")
		gqlClient := githubv4mock.NewMockedHTTPClient(
			resolveProjectNodeIDOrgMatcher("octo-org", 7, "PVT_project7"),
			projectViewParentMatcher("PVTV_view1", "PVT_project7"),
			githubv4mock.NewMutationMatcher(
				struct {
					UpdateProjectV2View struct {
						ProjectV2View projectViewNode `graphql:"projectV2View"`
					} `graphql:"updateProjectV2View(input: $input)"`
				}{},
				UpdateProjectV2ViewInput{
					ViewID: githubv4.ID("PVTV_view1"),
					Filter: &filter,
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"updateProjectV2View": map[string]any{
						"projectV2View": map[string]any{
							"id":     "PVTV_view1",
							"number": 1,
							"name":   "Renamed",
							"layout": "TABLE_LAYOUT",
							"filter": "",
						},
					},
				}),
			),
		)
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(gqlClient),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "update_project_view",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"view_id":        "PVTV_view1",
			"filter":         "",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, `"filter":""`)
	})

	t.Run("normalizes an updated layout to the GraphQL enum", func(t *testing.T) {
		layout := githubv4.ProjectV2ViewLayoutBoardLayout
		gqlClient := githubv4mock.NewMockedHTTPClient(
			resolveProjectNodeIDOrgMatcher("octo-org", 7, "PVT_project7"),
			projectViewParentMatcher("PVTV_view1", "PVT_project7"),
			githubv4mock.NewMutationMatcher(
				struct {
					UpdateProjectV2View struct {
						ProjectV2View projectViewNode `graphql:"projectV2View"`
					} `graphql:"updateProjectV2View(input: $input)"`
				}{},
				UpdateProjectV2ViewInput{
					ViewID: githubv4.ID("PVTV_view1"),
					Layout: &layout,
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"updateProjectV2View": map[string]any{
						"projectV2View": map[string]any{
							"id":     "PVTV_view1",
							"number": 1,
							"name":   "Board",
							"layout": "BOARD_LAYOUT",
							"filter": "",
						},
					},
				}),
			),
		)
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(gqlClient),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "update_project_view",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"view_id":        "PVTV_view1",
			"layout":         "board",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, `"layout":"board"`)
	})

	t.Run("surfaces GraphQL API errors", func(t *testing.T) {
		name := githubv4.String("Renamed")
		gqlClient := githubv4mock.NewMockedHTTPClient(
			resolveProjectNodeIDOrgMatcher("octo-org", 7, "PVT_project7"),
			projectViewParentMatcher("PVTV_view1", "PVT_project7"),
			githubv4mock.NewMutationMatcher(
				struct {
					UpdateProjectV2View struct {
						ProjectV2View projectViewNode `graphql:"projectV2View"`
					} `graphql:"updateProjectV2View(input: $input)"`
				}{},
				UpdateProjectV2ViewInput{
					ViewID: githubv4.ID("PVTV_view1"),
					Name:   &name,
				},
				nil,
				githubv4mock.ErrorResponse("update failed"),
			),
		)
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(gqlClient),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "update_project_view",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"view_id":        "PVTV_view1",
			"name":           "Renamed",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, ProjectViewUpdateFailedError)
		assert.Contains(t, getTextResult(t, result).Text, "update failed")
	})

	for _, tc := range []struct {
		name       string
		owner      string
		ownerType  string
		resolveReq githubv4mock.Matcher
	}{
		{
			name:       "rejects organization project mismatch",
			owner:      "octo-org",
			ownerType:  "org",
			resolveReq: resolveProjectNodeIDOrgMatcher("octo-org", 7, "PVT_org_project"),
		},
		{
			name:       "rejects user project mismatch",
			owner:      "octocat",
			ownerType:  "user",
			resolveReq: resolveProjectNodeIDUserMatcher("octocat", 7, "PVT_user_project"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gqlClient := githubv4mock.NewMockedHTTPClient(
				tc.resolveReq,
				projectViewParentMatcher("PVTV_view1", "PVT_other_project"),
			)
			deps := BaseDeps{
				Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
				GQLClient: githubv4.NewClient(gqlClient),
			}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(map[string]any{
				"method":         "update_project_view",
				"owner":          tc.owner,
				"owner_type":     tc.ownerType,
				"project_number": float64(7),
				"view_id":        "PVTV_view1",
				"name":           "Renamed",
			})

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getTextResult(t, result).Text, ProjectViewUpdateFailedError)
			assert.Contains(t, getTextResult(t, result).Text, "project view does not belong to the requested project")
		})
	}

	t.Run("surfaces parent verification API errors", func(t *testing.T) {
		gqlClient := githubv4mock.NewMockedHTTPClient(
			resolveProjectNodeIDOrgMatcher("octo-org", 7, "PVT_project7"),
			projectViewParentErrorMatcher("PVTV_view1", "lookup failed"),
		)
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(gqlClient),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "update_project_view",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"view_id":        "PVTV_view1",
			"name":           "Renamed",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, ProjectViewUpdateFailedError)
		assert.Contains(t, getTextResult(t, result).Text, "failed to resolve project view: lookup failed")
	})

	t.Run("rejects an empty update", func(t *testing.T) {
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "update_project_view",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"view_id":        "PVTV_view1",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "requires at least one of name, layout, or filter")
	})
}

func Test_ProjectsWrite_DeleteProjectView(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	t.Run("deletes a view from the requested project", func(t *testing.T) {
		gqlClient := githubv4mock.NewMockedHTTPClient(
			resolveProjectNodeIDOrgMatcher("octo-org", 7, "PVT_project7"),
			projectViewParentMatcher("PVTV_view1", "PVT_project7"),
			githubv4mock.NewMutationMatcher(
				struct {
					DeleteProjectV2View struct {
						ProjectV2View struct {
							ID githubv4.ID
						} `graphql:"projectV2View"`
					} `graphql:"deleteProjectV2View(input: $input)"`
				}{},
				DeleteProjectV2ViewInput{ViewID: githubv4.ID("PVTV_view1")},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"deleteProjectV2View": map[string]any{
						"projectV2View": map[string]any{"id": "PVTV_view1"},
					},
				}),
			),
		)
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(gqlClient),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "delete_project_view",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"view_id":        "PVTV_view1",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.JSONEq(t, `{"deleted_view_id":"PVTV_view1"}`, getTextResult(t, result).Text)
	})

	for _, tc := range []struct {
		name       string
		owner      string
		ownerType  string
		resolveReq githubv4mock.Matcher
	}{
		{
			name:       "rejects organization project mismatch",
			owner:      "octo-org",
			ownerType:  "org",
			resolveReq: resolveProjectNodeIDOrgMatcher("octo-org", 7, "PVT_org_project"),
		},
		{
			name:       "rejects user project mismatch",
			owner:      "octocat",
			ownerType:  "user",
			resolveReq: resolveProjectNodeIDUserMatcher("octocat", 7, "PVT_user_project"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gqlClient := githubv4mock.NewMockedHTTPClient(
				tc.resolveReq,
				projectViewParentMatcher("PVTV_view1", "PVT_other_project"),
			)
			deps := BaseDeps{
				Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
				GQLClient: githubv4.NewClient(gqlClient),
			}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(map[string]any{
				"method":         "delete_project_view",
				"owner":          tc.owner,
				"owner_type":     tc.ownerType,
				"project_number": float64(7),
				"view_id":        "PVTV_view1",
			})

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getTextResult(t, result).Text, ProjectViewDeleteFailedError)
			assert.Contains(t, getTextResult(t, result).Text, "project view does not belong to the requested project")
		})
	}

	t.Run("surfaces parent verification API errors", func(t *testing.T) {
		gqlClient := githubv4mock.NewMockedHTTPClient(
			resolveProjectNodeIDOrgMatcher("octo-org", 7, "PVT_project7"),
			projectViewParentErrorMatcher("PVTV_view1", "lookup failed"),
		)
		deps := BaseDeps{
			Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			GQLClient: githubv4.NewClient(gqlClient),
		}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":         "delete_project_view",
			"owner":          "octo-org",
			"owner_type":     "org",
			"project_number": float64(7),
			"view_id":        "PVTV_view1",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, ProjectViewDeleteFailedError)
		assert.Contains(t, getTextResult(t, result).Text, "failed to resolve project view: lookup failed")
	})
}
