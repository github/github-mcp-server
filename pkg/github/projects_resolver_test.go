package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectFieldsQueryMatcher is the GraphQL shape we use for fields(first:100) resolution.
// Keep this in sync with projectFieldsConnection in projects_resolver.go.
type projectFieldsTestQuery struct {
	Organization struct {
		ProjectV2 struct {
			Fields struct {
				Nodes []struct {
					ProjectV2Field struct {
						ID         githubv4.ID
						DatabaseID githubv4.Int `graphql:"databaseId"`
						Name       githubv4.String
						DataType   githubv4.String
					} `graphql:"... on ProjectV2Field"`
					ProjectV2IterationField struct {
						ID         githubv4.ID
						DatabaseID githubv4.Int `graphql:"databaseId"`
						Name       githubv4.String
						DataType   githubv4.String
					} `graphql:"... on ProjectV2IterationField"`
					ProjectV2SingleSelectField struct {
						ID         githubv4.ID
						DatabaseID githubv4.Int `graphql:"databaseId"`
						Name       githubv4.String
						DataType   githubv4.String
						Options    []struct {
							ID   githubv4.String
							Name githubv4.String
						}
					} `graphql:"... on ProjectV2SingleSelectField"`
				}
				PageInfo PageInfoFragment
			} `graphql:"fields(first: $first, after: $after)"`
		} `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"organization(login: $owner)"`
}

func fieldsQueryVars(owner string, projectNumber int) map[string]any {
	return map[string]any{
		"owner":         githubv4.String(owner),
		"projectNumber": githubv4.Int(int32(projectNumber)), //nolint:gosec
		"first":         githubv4.Int(resolverFieldsPageSize),
		"after":         (*githubv4.String)(nil),
	}
}

// statusFieldNode is a single-select field response node for use in mock data.
// `nodeID` is the global node ID (e.g. "PVTSSF_lADO...") and `databaseID` is
// the numeric database ID the REST API expects.
func statusFieldNode(nodeID string, databaseID int, name string, options []map[string]any) map[string]any {
	return map[string]any{
		"id":         nodeID,
		"databaseId": databaseID,
		"name":       name,
		"dataType":   "SINGLE_SELECT",
		"options":    options,
	}
}

func fieldsResponse(nodes []map[string]any) map[string]any {
	return map[string]any{
		"organization": map[string]any{
			"projectV2": map[string]any{
				"fields": map[string]any{
					"nodes": nodes,
					"pageInfo": map[string]any{
						"hasNextPage":     false,
						"hasPreviousPage": false,
						"startCursor":     "",
						"endCursor":       "",
					},
				},
			},
		},
	}
}

func Test_ResolveProjectFieldByName_Success(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_lADOBBcDeFg123", 12345, "Status", []map[string]any{
					{"id": "OPT_a", "name": "Todo"},
					{"id": "OPT_b", "name": "In Progress"},
					{"id": "OPT_c", "name": "Done"},
				}),
			})),
		),
	)
	gql := githubv4.NewClient(mocked)

	field, err := resolveProjectFieldByName(context.Background(), gql, "octo-org", "org", 7, "Status", "SINGLE_SELECT")
	require.NoError(t, err)
	require.NotNil(t, field)
	assert.Equal(t, "12345", field.ID)
	assert.Equal(t, "SINGLE_SELECT", field.DataType)
	assert.Len(t, field.Options, 3)

	optionID, err := resolveSingleSelectOptionByName(field, "In Progress")
	require.NoError(t, err)
	assert.Equal(t, "OPT_b", optionID)
}

func Test_ResolveProjectFieldByName_NotFound_ReturnsStructuredError(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_lADOBBcDeFg123", 12345, "Status", nil),
			})),
		),
	)
	gql := githubv4.NewClient(mocked)

	_, err := resolveProjectFieldByName(context.Background(), gql, "octo-org", "org", 7, "Priority", "")
	require.Error(t, err)

	var msg map[string]any
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &msg))
	assert.Equal(t, "field_not_found", msg["error"])
	assert.Equal(t, "Priority", msg["name"])
	assert.NotEmpty(t, msg["candidates"])
}

func Test_ResolveProjectFieldByName_Ambiguous_ReturnsStructuredError(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_lADOBBcDeFg123", 12345, "Status", nil),
				statusFieldNode("PVTSSF_lADOBBcDeFg678", 67890, "Status", nil),
			})),
		),
	)
	gql := githubv4.NewClient(mocked)

	_, err := resolveProjectFieldByName(context.Background(), gql, "octo-org", "org", 7, "Status", "")
	require.Error(t, err)

	var msg map[string]any
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &msg))
	assert.Equal(t, "field_ambiguous", msg["error"])
	candidates, _ := msg["candidates"].([]any)
	assert.Len(t, candidates, 2)
}

func Test_ResolveSingleSelectOptionByName_NotFound(t *testing.T) {
	field := &ResolvedField{
		ID:       "12345",
		Name:     "Status",
		DataType: "SINGLE_SELECT",
		Options: []ResolvedFieldOption{
			{ID: "OPT_a", Name: "Todo"},
			{ID: "OPT_b", Name: "Done"},
		},
	}

	_, err := resolveSingleSelectOptionByName(field, "Blocked")
	require.Error(t, err)

	var msg map[string]any
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &msg))
	assert.Equal(t, "option_not_found", msg["error"])
	assert.Equal(t, "Blocked", msg["name"])
}

func Test_ResolveSingleSelectOptionByName_WrongFieldType(t *testing.T) {
	field := &ResolvedField{
		ID:       "12345",
		Name:     "Description",
		DataType: "TEXT",
	}

	_, err := resolveSingleSelectOptionByName(field, "anything")
	require.Error(t, err)

	var msg map[string]any
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &msg))
	assert.Equal(t, "wrong_field_type", msg["error"])
}

// resolveItemByIssueQuery matches the GraphQL shape used by
// resolveProjectItemIDByIssueNumber for the issue.projectItems traversal.
type resolveItemByIssueQuery struct {
	Repository struct {
		Issue struct {
			ProjectItems struct {
				Nodes []struct {
					FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
					Project        struct {
						ID githubv4.ID
					}
				}
				PageInfo PageInfoFragment
			} `graphql:"projectItems(first: 50, includeArchived: true)"`
		} `graphql:"issue(number: $issueNumber)"`
	} `graphql:"repository(owner: $issueOwner, name: $issueRepo)"`
}

func Test_ResolveProjectItemIDByIssueNumber_Success(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
		// project node id lookup (org)
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
		// issue.projectItems lookup
		githubv4mock.NewQueryMatcher(
			resolveItemByIssueQuery{},
			map[string]any{
				"issueOwner":  githubv4.String("octo-issue-owner"),
				"issueRepo":   githubv4.String("repo"),
				"issueNumber": githubv4.Int(123),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issue": map[string]any{
						"projectItems": map[string]any{
							"nodes": []any{
								map[string]any{
									"fullDatabaseId": "9999",
									"project":        map[string]any{"id": "PVT_other"},
								},
								map[string]any{
									"fullDatabaseId": "4242",
									"project":        map[string]any{"id": "PVT_project1"},
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
	gql := githubv4.NewClient(mocked)

	itemID, err := resolveProjectItemIDByIssueNumber(context.Background(), gql, "octo-org", "org", 1, "octo-issue-owner", "repo", 123)
	require.NoError(t, err)
	assert.Equal(t, int64(4242), itemID)
}

func Test_ResolveProjectItemIDByIssueNumber_NotInProject(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
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
		githubv4mock.NewQueryMatcher(
			resolveItemByIssueQuery{},
			map[string]any{
				"issueOwner":  githubv4.String("octo-issue-owner"),
				"issueRepo":   githubv4.String("repo"),
				"issueNumber": githubv4.Int(123),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issue": map[string]any{
						"projectItems": map[string]any{
							"nodes": []any{
								map[string]any{
									"fullDatabaseId": "9999",
									"project":        map[string]any{"id": "PVT_other"},
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
	gql := githubv4.NewClient(mocked)

	_, err := resolveProjectItemIDByIssueNumber(context.Background(), gql, "octo-org", "org", 1, "octo-issue-owner", "repo", 123)
	require.Error(t, err)

	var msg map[string]any
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &msg))
	assert.Equal(t, "item_not_in_project", msg["error"])
}

func Test_ResolveFieldNamesToIDs_Success(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_lADOBBcDeFg100", 100, "Status", nil),
				statusFieldNode("PVTSSF_lADOBBcDeFg200", 200, "Priority", nil),
			})),
		),
	)
	gql := githubv4.NewClient(mocked)

	ids, err := resolveFieldNamesToIDs(context.Background(), gql, "octo-org", "org", 1, []string{"Status", "Priority"})
	require.NoError(t, err)
	assert.Equal(t, []int64{100, 200}, ids)
}

// Field and single-select option name matching is case-insensitive so agents passing lowercase
// names like "status" or "in progress" resolve to "Status" and "In Progress" respectively.
func Test_ResolveProjectFieldByName_CaseInsensitive(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_lADOBBcDeFg123", 12345, "Status", []map[string]any{
					{"id": "OPT_a", "name": "Todo"},
					{"id": "OPT_b", "name": "In Progress"},
				}),
			})),
		),
	)
	gql := githubv4.NewClient(mocked)

	field, err := resolveProjectFieldByName(context.Background(), gql, "octo-org", "org", 7, "status", "")
	require.NoError(t, err)
	require.NotNil(t, field)
	assert.Equal(t, "12345", field.ID)

	optionID, err := resolveSingleSelectOptionByName(field, "in progress")
	require.NoError(t, err)
	assert.Equal(t, "OPT_b", optionID)
}

// Test_ResolveFieldNamesToIDs_CaseInsensitive verifies bulk name resolution
// also matches case-insensitively.
func Test_ResolveFieldNamesToIDs_CaseInsensitive(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_lADOBBcDeFg100", 100, "Status", nil),
				statusFieldNode("PVTSSF_lADOBBcDeFg200", 200, "Priority", nil),
			})),
		),
	)
	gql := githubv4.NewClient(mocked)

	ids, err := resolveFieldNamesToIDs(context.Background(), gql, "octo-org", "org", 1, []string{"status", "PRIORITY"})
	require.NoError(t, err)
	assert.Equal(t, []int64{100, 200}, ids)
}

// Test_ProjectsWrite_UpdateProjectItem_ByName is the acceptance test for the
// write side: set Status = "In Progress" using only names plus an issue number.
func Test_ProjectsWrite_UpdateProjectItem_ByName(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	updatedItem := verbosePullRequestProjectItemFixture()

	mockedREST := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchOrgsProjectsV2ItemsByProjectByItemID: mockResponse(t, http.StatusOK, updatedItem),
	})
	restClient := mustNewGHClient(t, mockedREST)

	mockedGQL := githubv4mock.NewMockedHTTPClient(
		// 1. project node id (used by resolveProjectItemIDByIssueNumber)
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
					"projectV2": map[string]any{"id": "PVT_project1"},
				},
			}),
		),
		// 2. issue -> projectItems lookup
		githubv4mock.NewQueryMatcher(
			resolveItemByIssueQuery{},
			map[string]any{
				"issueOwner":  githubv4.String("github"),
				"issueRepo":   githubv4.String("planning-tracking"),
				"issueNumber": githubv4.Int(123),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issue": map[string]any{
						"projectItems": map[string]any{
							"nodes": []any{
								map[string]any{
									"fullDatabaseId": "1001",
									"project":        map[string]any{"id": "PVT_project1"},
								},
							},
							"pageInfo": map[string]any{
								"hasNextPage": false, "hasPreviousPage": false,
								"startCursor": "", "endCursor": "",
							},
						},
					},
				},
			}),
		),
		// 3. fields(first:100) for name resolution
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_lADOBBcDeFg101", 101, "Status", []map[string]any{
					{"id": "OPT_in_progress", "name": "In Progress"},
				}),
			})),
		),
	)
	gqlClient := githubv4.NewClient(mockedGQL)

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_item",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"item_owner":     "github",
		"item_repo":      "planning-tracking",
		"issue_number":   float64(123),
		"updated_field": map[string]any{
			"name":  "Status",
			"value": "In Progress",
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)
}

func Test_ProjectsWrite_UpdateProjectItem_NameNotFound_StructuredError(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	mockedGQL := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_lADOBBcDeFg101", 101, "Status", nil),
			})),
		),
	)
	gqlClient := githubv4.NewClient(mockedGQL)
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_item",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"item_id":        float64(1001),
		"updated_field": map[string]any{
			"name":  "Doesnt Exist",
			"value": "whatever",
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.True(t, result.IsError)

	textContent := getTextResult(t, result)
	var msg map[string]any
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &msg))
	assert.Equal(t, "field_not_found", msg["error"])
	assert.Equal(t, "Doesnt Exist", msg["name"])
}
