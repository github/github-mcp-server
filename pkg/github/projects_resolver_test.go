package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/http/headers"
	transportpkg "github.com/github/github-mcp-server/pkg/http/transport"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectFieldsTestQuery is the ungated GraphQL shape used by read and general field resolution.
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

// projectFieldsWithIssueFieldsTestQuery includes the issue_fields-gated attachment bridge.
type projectFieldsWithIssueFieldsTestQuery struct {
	Organization struct {
		ProjectV2 struct {
			Fields projectFieldsWithIssueFieldsConnection `graphql:"fields(first: $first, after: $after)"`
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

// iterationFieldNode is an iteration field response node for use in mock data.
func iterationFieldNode(nodeID string, databaseID int, name string) map[string]any {
	return map[string]any{
		"id":         nodeID,
		"databaseId": databaseID,
		"name":       name,
		"dataType":   "ITERATION",
	}
}

// genericFieldNode is a plain field response node (neither single-select nor
// iteration, e.g. TEXT or NUMBER) for use in mock data.
func genericFieldNode(nodeID string, databaseID int, name, dataType string) map[string]any {
	return map[string]any{
		"id":         nodeID,
		"databaseId": databaseID,
		"name":       name,
		"dataType":   dataType,
	}
}

func attachedIssueFieldNode(projectNodeID string, projectDatabaseID int, issueFieldNodeID, name, dataType string, options []map[string]any) map[string]any {
	node := map[string]any{
		"id":           projectNodeID,
		"databaseId":   projectDatabaseID,
		"name":         name,
		"dataType":     dataType,
		"isIssueField": true,
		"issueField":   map[string]any{"id": issueFieldNodeID},
	}
	if dataType == "SINGLE_SELECT" {
		node["options"] = []any{}
		node["issueField"].(map[string]any)["options"] = options
	}
	return node
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

func Test_ResolveProjectFieldByID_AttachedIssueFieldMetadata(t *testing.T) {
	queryTransport := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				attachedIssueFieldNode("PVTSSF_risk", 702, "IF_risk", "Risk", "SINGLE_SELECT", []map[string]any{
					{"id": "IFO_low", "name": "Low"}, {"id": "IFO_high", "name": "High"},
				}),
			})),
		),
	)
	transport := &mutationAwareTransport{t: t, queries: queryTransport.Transport}
	gql := githubv4.NewClient(&http.Client{
		Transport: &transportpkg.GraphQLFeaturesTransport{Transport: transport},
	})

	field, err := resolveProjectFieldByID(t.Context(), gql, "octo-org", "org", 7, 702)
	require.NoError(t, err)
	require.Len(t, transport.queryCalls, 1)
	assert.Equal(t, "issue_fields", transport.queryCalls[0].Headers.Get(headers.GraphQLFeaturesHeader))
	assert.Equal(t, "702", field.ID)
	assert.Equal(t, "PVTSSF_risk", field.NodeID)
	assert.Equal(t, "IF_risk", field.IssueFieldNodeID)
	assert.True(t, field.IsIssueField)
	assert.Equal(t, []ResolvedFieldOption{
		{ID: "IFO_low", Name: "Low"}, {ID: "IFO_high", Name: "High"},
	}, field.Options)
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
	assert.Equal(t, "PVTSSF_lADOBBcDeFg123", field.NodeID)
	assert.Equal(t, "SINGLE_SELECT", field.DataType)
	assert.Len(t, field.Options, 3)

	optionID, err := resolveSingleSelectOptionByName(field, "In Progress")
	require.NoError(t, err)
	assert.Equal(t, "OPT_b", optionID)
}

func Test_ResolveProjectFieldByName_NodeIDsForAllVariants(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_single1", 111, "Status", []map[string]any{
					{"id": "OPT_a", "name": "Todo"},
				}),
				iterationFieldNode("PVTIF_iteration1", 222, "Sprint"),
				genericFieldNode("PVTF_text1", 333, "Notes", "TEXT"),
			})),
		),
	)
	gql := githubv4.NewClient(mocked)

	variants := []struct {
		fieldName    string
		expectedType string
		wantNodeID   string
	}{
		{"Status", "SINGLE_SELECT", "PVTSSF_single1"},
		{"Sprint", "ITERATION", "PVTIF_iteration1"},
		{"Notes", "TEXT", "PVTF_text1"},
	}
	for _, v := range variants {
		t.Run(v.fieldName, func(t *testing.T) {
			field, err := resolveProjectFieldByName(context.Background(), gql, "octo-org", "org", 7, v.fieldName, v.expectedType)
			require.NoError(t, err)
			require.NotNil(t, field)
			assert.Equal(t, v.wantNodeID, field.NodeID)
			assert.Equal(t, v.expectedType, field.DataType)
		})
	}
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

func Test_ResolveProjectFieldForUpdateByName_IssueFieldSchemaUnavailable(t *testing.T) {
	queryTransport := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.ErrorResponse("Field 'isIssueField' doesn't exist on type 'ProjectV2Field'"),
		),
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				genericFieldNode("PVTF_notes", 333, "Notes", "TEXT"),
			})),
		),
	)
	transport := &mutationAwareTransport{t: t, queries: queryTransport.Transport}
	gql := githubv4.NewClient(&http.Client{
		Transport: &transportpkg.GraphQLFeaturesTransport{Transport: transport},
	})

	field, err := resolveProjectFieldForUpdateByName(t.Context(), gql, "octo-org", "org", 7, "Notes", "")
	require.NoError(t, err)
	assert.Equal(t, "333", field.ID)
	require.Len(t, transport.queryCalls, 2)
	assert.Equal(t, "issue_fields", transport.queryCalls[0].Headers.Get(headers.GraphQLFeaturesHeader))
	assert.Empty(t, transport.queryCalls[1].Headers.Get(headers.GraphQLFeaturesHeader))
	assert.NotContains(t, transport.queryCalls[1].Query, "isIssueField")
	assert.NotContains(t, transport.queryCalls[1].Query, "issueField")
}

func Test_ResolveProjectFieldForUpdateByName_DoesNotHideGraphQLErrors(t *testing.T) {
	queryTransport := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.ErrorResponse("Something went wrong while resolving project fields"),
		),
	)
	transport := &mutationAwareTransport{t: t, queries: queryTransport.Transport}
	gql := githubv4.NewClient(&http.Client{
		Transport: &transportpkg.GraphQLFeaturesTransport{Transport: transport},
	})

	_, err := resolveProjectFieldForUpdateByName(t.Context(), gql, "octo-org", "org", 7, "Notes", "")
	require.ErrorContains(t, err, "Something went wrong")
	assert.Len(t, transport.queryCalls, 1)
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
			ID           githubv4.ID
			ProjectItems struct {
				Nodes []struct {
					ID             githubv4.ID
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

type resolveItemByIssuePageQuery struct {
	Repository struct {
		Issue struct {
			ID           githubv4.ID
			ProjectItems struct {
				Nodes []struct {
					ID             githubv4.ID
					FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
					Project        struct {
						ID githubv4.ID
					}
				}
				PageInfo PageInfoFragment
			} `graphql:"projectItems(first: 50, after: $after, includeArchived: true)"`
		} `graphql:"issue(number: $issueNumber)"`
	} `graphql:"repository(owner: $issueOwner, name: $issueRepo)"`
}

type requestCountingTransport struct {
	inner http.RoundTripper
	count int
}

func (t *requestCountingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.count++
	return t.inner.RoundTrip(req)
}

func Test_ResolveProjectItemByIssueNumber_Success(t *testing.T) {
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
									"id":             "PVTI_target",
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

	nodeID, itemID, err := resolveProjectItemByIssueNumber(context.Background(), gql, "octo-org", "org", 1, "octo-issue-owner", "repo", 123)
	require.NoError(t, err)
	assert.Equal(t, "PVTI_target", nodeID)
	assert.Equal(t, int64(4242), itemID)
}

func Test_ResolveProjectItemByIssueNumber_TargetOnSecondPage(t *testing.T) {
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
					"projectV2": map[string]any{"id": "PVT_project1"},
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
								"hasNextPage":     true,
								"hasPreviousPage": false,
								"startCursor":     "first",
								"endCursor":       "page-one",
							},
						},
					},
				},
			}),
		),
		githubv4mock.NewQueryMatcher(
			resolveItemByIssuePageQuery{},
			map[string]any{
				"issueOwner":  githubv4.String("octo-issue-owner"),
				"issueRepo":   githubv4.String("repo"),
				"issueNumber": githubv4.Int(123),
				"after":       githubv4.String("page-one"),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issue": map[string]any{
						"projectItems": map[string]any{
							"nodes": []any{
								map[string]any{
									"id":             "PVTI_target",
									"fullDatabaseId": "4242",
									"project":        map[string]any{"id": "PVT_project1"},
								},
							},
							"pageInfo": map[string]any{
								"hasNextPage":     false,
								"hasPreviousPage": true,
								"startCursor":     "page-two",
								"endCursor":       "page-two",
							},
						},
					},
				},
			}),
		),
	)
	gql := githubv4.NewClient(mocked)

	nodeID, itemID, err := resolveProjectItemByIssueNumber(context.Background(), gql, "octo-org", "org", 1, "octo-issue-owner", "repo", 123)
	require.NoError(t, err)
	assert.Equal(t, "PVTI_target", nodeID)
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

func Test_ResolveProjectItemIDByIssueNumber_NotInProjectAfterMultiplePages(t *testing.T) {
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
					"projectV2": map[string]any{"id": "PVT_project1"},
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
								"hasNextPage":     true,
								"hasPreviousPage": false,
								"startCursor":     "first",
								"endCursor":       "page-one",
							},
						},
					},
				},
			}),
		),
		githubv4mock.NewQueryMatcher(
			resolveItemByIssuePageQuery{},
			map[string]any{
				"issueOwner":  githubv4.String("octo-issue-owner"),
				"issueRepo":   githubv4.String("repo"),
				"issueNumber": githubv4.Int(123),
				"after":       githubv4.String("page-one"),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issue": map[string]any{
						"projectItems": map[string]any{
							"nodes": []any{
								map[string]any{
									"fullDatabaseId": "8888",
									"project":        map[string]any{"id": "PVT_another"},
								},
							},
							"pageInfo": map[string]any{
								"hasNextPage":     false,
								"hasPreviousPage": true,
								"startCursor":     "page-two",
								"endCursor":       "page-two",
							},
						},
					},
				},
			}),
		),
	)
	countingTransport := &requestCountingTransport{inner: mocked.Transport}
	mocked.Transport = countingTransport
	gql := githubv4.NewClient(mocked)

	_, err := resolveProjectItemIDByIssueNumber(context.Background(), gql, "octo-org", "org", 1, "octo-issue-owner", "repo", 123)
	require.Error(t, err)
	assert.Equal(t, 3, countingTransport.count)

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
			projectFieldsWithIssueFieldsTestQuery{},
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
			projectFieldsWithIssueFieldsTestQuery{},
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

func Test_BuildUpdateProjectItem_SingleSelectIDPrecedesName(t *testing.T) {
	gql := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_status", 101, "Status", []map[string]any{
					{"id": "OPT_other", "name": "OPT_target"},
					{"id": "OPT_target", "name": "Target"},
				}),
			})),
		),
	))

	update, err := buildUpdateProjectItem(t.Context(), gql, "octo-org", "org", 1, map[string]any{
		"name": "Status", "value": "OPT_target",
	})
	require.NoError(t, err)
	require.NotNil(t, update.Project)
	require.Len(t, update.Project.Fields, 1)
	assert.Equal(t, "OPT_target", update.Project.Fields[0].Value)
}
