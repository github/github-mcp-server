package github

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func attachedIssueFieldNode(projectNodeID string, projectDatabaseID int, issueFieldNodeID, name, dataType string, options []map[string]any) map[string]any {
	node := map[string]any{
		"id":           projectNodeID,
		"databaseId":   projectDatabaseID,
		"name":         name,
		"dataType":     dataType,
		"isIssueField": true,
		"issueField": map[string]any{
			"id": issueFieldNodeID,
		},
	}
	if dataType == "SINGLE_SELECT" {
		node["options"] = []any{}
		node["issueField"].(map[string]any)["options"] = options
	}
	return node
}

func issueProjectItemFixture(fields []map[string]any) map[string]any {
	return map[string]any{
		"id":           1001,
		"node_id":      "PVTI_1",
		"content_type": "Issue",
		"content": map[string]any{
			"id":             2002,
			"node_id":        "I_123",
			"number":         5,
			"title":          "Track customer",
			"state":          "open",
			"html_url":       "https://github.com/octo-org/roadmap/issues/5",
			"repository_url": "https://api.github.com/repos/octo-org/roadmap",
		},
		"fields": fields,
	}
}

func issueFieldMutationResponse() map[string]any {
	return map[string]any{
		"setIssueFieldValue": map[string]any{
			"issue": map[string]any{
				"id":     "I_123",
				"number": 5,
				"url":    "https://github.com/octo-org/roadmap/issues/5",
			},
			"issueFieldValues": []any{},
		},
	}
}

func Test_ResolveProjectFieldByName_AttachedIssueFields(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				attachedIssueFieldNode("PVTF_customer", 701, "IF_customer", "Customer", "TEXT", nil),
				attachedIssueFieldNode("PVTSSF_risk", 702, "IF_risk", "Risk", "SINGLE_SELECT", []map[string]any{
					{"id": "IFO_low", "name": "Low"},
					{"id": "IFO_high", "name": "High"},
				}),
			})),
		),
	)
	gql := githubv4.NewClient(mocked)

	customer, err := resolveProjectFieldByName(context.Background(), gql, "octo-org", "org", 7, "customer", "")
	require.NoError(t, err)
	assert.Equal(t, "701", customer.ID)
	assert.Equal(t, "PVTF_customer", customer.NodeID)
	assert.True(t, customer.IsIssueField)
	assert.Equal(t, "IF_customer", customer.IssueFieldNodeID)

	risk, err := resolveProjectFieldByName(context.Background(), gql, "octo-org", "org", 7, "RISK", "")
	require.NoError(t, err)
	assert.Equal(t, "702", risk.ID)
	assert.Equal(t, "IF_risk", risk.IssueFieldNodeID)
	assert.Equal(t, []ResolvedFieldOption{
		{ID: "IFO_low", Name: "Low"},
		{ID: "IFO_high", Name: "High"},
	}, risk.Options)
}

func Test_ProjectItemReads_FieldNamesIncludeAttachedIssueFieldValues(t *testing.T) {
	fieldValue := map[string]any{
		"id":             701,
		"issue_field_id": 9001,
		"name":           "Customer",
		"data_type":      "text",
		"value":          "Acme",
	}

	tests := []struct {
		name   string
		tool   func(translations.TranslationHelperFunc) inventory.ServerTool
		method string
		route  string
		body   any
	}{
		{
			name:   "get",
			tool:   ProjectsGet,
			method: projectsMethodGetProjectItem,
			route:  GetOrgsProjectsV2ItemsByProjectByItemID,
			body:   issueProjectItemFixture([]map[string]any{fieldValue}),
		},
		{
			name:   "list",
			tool:   ProjectsList,
			method: projectsMethodListProjectItems,
			route:  GetOrgsProjectsV2ItemsByProject,
			body:   []map[string]any{issueProjectItemFixture([]map[string]any{fieldValue})},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gql := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					projectFieldsTestQuery{},
					fieldsQueryVars("octo-org", 1),
					githubv4mock.DataResponse(fieldsResponse([]map[string]any{
						attachedIssueFieldNode("PVTF_customer", 701, "IF_customer", "Customer", "TEXT", nil),
					})),
				),
			))
			rest := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				tt.route: func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "701", r.URL.Query().Get("fields"))
					w.Header().Set("Content-Type", "application/json")
					require.NoError(t, json.NewEncoder(w).Encode(tt.body))
				},
			})
			serverTool := tt.tool(translations.NullTranslationHelper)
			deps := BaseDeps{Client: mustNewGHClient(t, rest), GQLClient: gql}
			handler := serverTool.Handler(deps)
			args := map[string]any{
				"method":         tt.method,
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"field_names":    []any{"customer"},
			}
			if tt.method == projectsMethodGetProjectItem {
				args["item_id"] = float64(1001)
			}

			request := createMCPRequest(args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, getTextResult(t, result).Text)

			var response map[string]any
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
			item := response
			if tt.method == projectsMethodListProjectItems {
				items := response["items"].([]any)
				item = items[0].(map[string]any)
			}
			fields := item["fields"].([]any)
			require.Len(t, fields, 1)
			assert.Equal(t, map[string]any{
				"id":        float64(701),
				"name":      "Customer",
				"data_type": "text",
				"value":     "Acme",
			}, fields[0])
		})
	}
}

func Test_ProjectsWrite_UpdateProjectItem_AttachedIssueFieldTypes(t *testing.T) {
	number := githubv4.Float(3.5)
	optionID := githubv4.ID("IFO_high")
	deleteValue := githubv4.Boolean(true)

	tests := []struct {
		name      string
		fieldNode map[string]any
		fieldRef  map[string]any
		value     any
		want      IssueFieldCreateOrUpdateInput
	}{
		{
			name:      "text",
			fieldNode: attachedIssueFieldNode("PVTF_customer", 701, "IF_customer", "Customer", "TEXT", nil),
			fieldRef:  map[string]any{"name": "cUsToMeR"},
			value:     "Acme",
			want: IssueFieldCreateOrUpdateInput{
				FieldID:   githubv4.ID("IF_customer"),
				TextValue: githubv4.NewString(githubv4.String("Acme")),
			},
		},
		{
			name:      "text by project field ID",
			fieldNode: attachedIssueFieldNode("PVTF_customer", 701, "IF_customer", "Customer", "TEXT", nil),
			fieldRef:  map[string]any{"id": float64(701)},
			value:     "Acme",
			want: IssueFieldCreateOrUpdateInput{
				FieldID:   githubv4.ID("IF_customer"),
				TextValue: githubv4.NewString(githubv4.String("Acme")),
			},
		},
		{
			name:      "number",
			fieldNode: attachedIssueFieldNode("PVTF_score", 702, "IF_score", "Score", "NUMBER", nil),
			fieldRef:  map[string]any{"name": "Score"},
			value:     3.5,
			want: IssueFieldCreateOrUpdateInput{
				FieldID:     githubv4.ID("IF_score"),
				NumberValue: &number,
			},
		},
		{
			name:      "date",
			fieldNode: attachedIssueFieldNode("PVTF_due", 703, "IF_due", "Due", "DATE", nil),
			fieldRef:  map[string]any{"name": "Due"},
			value:     "2026-08-01",
			want: IssueFieldCreateOrUpdateInput{
				FieldID:   githubv4.ID("IF_due"),
				DateValue: githubv4.NewString(githubv4.String("2026-08-01")),
			},
		},
		{
			name: "single select resolves option case insensitively",
			fieldNode: attachedIssueFieldNode("PVTSSF_risk", 704, "IF_risk", "Risk", "SINGLE_SELECT", []map[string]any{
				{"id": "IFO_low", "name": "Low"},
				{"id": "IFO_high", "name": "High"},
			}),
			fieldRef: map[string]any{"name": "rIsK"},
			value:    "hIgH",
			want: IssueFieldCreateOrUpdateInput{
				FieldID:              githubv4.ID("IF_risk"),
				SingleSelectOptionID: &optionID,
			},
		},
		{
			name: "single select by project field ID",
			fieldNode: attachedIssueFieldNode("PVTSSF_risk", 704, "IF_risk", "Risk", "SINGLE_SELECT", []map[string]any{
				{"id": "IFO_low", "name": "Low"},
				{"id": "IFO_high", "name": "High"},
			}),
			fieldRef: map[string]any{"id": float64(704)},
			value:    "IFO_high",
			want: IssueFieldCreateOrUpdateInput{
				FieldID:              githubv4.ID("IF_risk"),
				SingleSelectOptionID: &optionID,
			},
		},
		{
			name:      "clear",
			fieldNode: attachedIssueFieldNode("PVTF_customer", 701, "IF_customer", "Customer", "TEXT", nil),
			fieldRef:  map[string]any{"name": "Customer"},
			value:     nil,
			want: IssueFieldCreateOrUpdateInput{
				FieldID: githubv4.ID("IF_customer"),
				Delete:  &deleteValue,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gql := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					projectFieldsTestQuery{},
					fieldsQueryVars("octo-org", 1),
					githubv4mock.DataResponse(fieldsResponse([]map[string]any{tt.fieldNode})),
				),
				githubv4mock.NewMutationMatcher(
					setIssueFieldValueMutation{},
					SetIssueFieldValueInput{
						IssueID:     githubv4.ID("I_123"),
						IssueFields: []IssueFieldCreateOrUpdateInput{tt.want},
					},
					nil,
					githubv4mock.DataResponse(issueFieldMutationResponse()),
				),
			))
			rest := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsProjectsV2ItemsByProjectByItemID: mockResponse(t, http.StatusOK, issueProjectItemFixture(nil)),
			})
			deps := BaseDeps{Client: mustNewGHClient(t, rest), GQLClient: gql}
			serverTool := ProjectsWrite(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)
			updatedField := map[string]any{"value": tt.value}
			maps.Copy(updatedField, tt.fieldRef)
			request := createMCPRequest(map[string]any{
				"method":         projectsMethodUpdateProjectItem,
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(1001),
				"updated_field":  updatedField,
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, getTextResult(t, result).Text)
			assert.JSONEq(t, `{"id":"I_123","url":"https://github.com/octo-org/roadmap/issues/5"}`, getTextResult(t, result).Text)
		})
	}
}

func Test_ProjectsWrite_UpdateProjectItem_RejectsIssueFieldForNonIssueItems(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		content     map[string]any
	}{
		{
			name:        "pull request",
			contentType: "PullRequest",
			content: map[string]any{
				"id": 2002, "node_id": "PR_123", "number": 5, "title": "PR",
			},
		},
		{
			name:        "draft issue",
			contentType: "DraftIssue",
			content: map[string]any{
				"id": 2003, "node_id": "DI_123", "title": "Draft",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gql := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					projectFieldsTestQuery{},
					fieldsQueryVars("octo-org", 1),
					githubv4mock.DataResponse(fieldsResponse([]map[string]any{
						attachedIssueFieldNode("PVTF_customer", 701, "IF_customer", "Customer", "TEXT", nil),
					})),
				),
			))
			rest := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsProjectsV2ItemsByProjectByItemID: mockResponse(t, http.StatusOK, map[string]any{
					"id":           1001,
					"node_id":      "PVTI_1",
					"content_type": tt.contentType,
					"content":      tt.content,
				}),
			})
			deps := BaseDeps{Client: mustNewGHClient(t, rest), GQLClient: gql}
			serverTool := ProjectsWrite(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)
			request := createMCPRequest(map[string]any{
				"method":         projectsMethodUpdateProjectItem,
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(1001),
				"updated_field":  map[string]any{"name": "Customer", "value": "Acme"},
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			var response map[string]any
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
			assert.Equal(t, "unsupported_item_type", response["error"])
			assert.Equal(t, tt.contentType, response["name"])
			assert.Contains(t, response["hint"], "only be updated on Issue project items")
		})
	}
}

func Test_ProjectsWrite_UpdateProjectItem_IssueFieldErrorsAreStructured(t *testing.T) {
	tests := []struct {
		name      string
		fieldNode map[string]any
		value     any
		wantKind  string
	}{
		{
			name:      "wrong value type",
			fieldNode: attachedIssueFieldNode("PVTF_customer", 701, "IF_customer", "Customer", "TEXT", nil),
			value:     42.0,
			wantKind:  "invalid_field_value",
		},
		{
			name: "missing underlying metadata",
			fieldNode: map[string]any{
				"id": "PVTF_customer", "databaseId": 701, "name": "Customer", "dataType": "TEXT", "isIssueField": true,
			},
			value:    "Acme",
			wantKind: "issue_field_metadata_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gql := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					projectFieldsTestQuery{},
					fieldsQueryVars("octo-org", 1),
					githubv4mock.DataResponse(fieldsResponse([]map[string]any{tt.fieldNode})),
				),
			))
			deps := BaseDeps{
				Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
				GQLClient: gql,
			}
			serverTool := ProjectsWrite(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)
			request := createMCPRequest(map[string]any{
				"method":         projectsMethodUpdateProjectItem,
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(1001),
				"updated_field":  map[string]any{"name": "Customer", "value": tt.value},
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			var response map[string]any
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
			assert.Equal(t, tt.wantKind, response["error"])
			assert.NotEmpty(t, response["hint"])
		})
	}
}

func Test_ProjectsWrite_UpdateProjectItem_StandardFieldNameStillUsesProjectsREST(t *testing.T) {
	gql := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_status", 101, "Status", []map[string]any{{"id": "OPT_doing", "name": "Doing"}}),
			})),
		),
	))
	rest := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchOrgsProjectsV2ItemsByProjectByItemID: mockResponse(t, http.StatusOK, verbosePullRequestProjectItemFixture()),
	})
	deps := BaseDeps{Client: mustNewGHClient(t, rest), GQLClient: gql}
	serverTool := ProjectsWrite(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         projectsMethodUpdateProjectItem,
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"item_id":        float64(1001),
		"updated_field":  map[string]any{"name": "status", "value": "doing"},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)
}

func Test_UpdateProjectItemsBatch_RejectsAttachedIssueFields(t *testing.T) {
	gql := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				attachedIssueFieldNode("PVTF_customer", 701, "IF_customer", "Customer", "TEXT", nil),
			})),
		),
	))

	result, _, err := updateProjectItemsBatch(context.Background(), nil, gql, "octo-org", "org", 1, map[string]any{
		"updated_field": map[string]any{"name": "Customer", "value": "Acme"},
		"items":         []any{map[string]any{"node_id": "PVTI_1"}},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	text := getTextResult(t, result).Text
	assert.Contains(t, text, "update_project_items does not support Issue Fields")
	assert.Contains(t, text, "Use singular update_project_item")
}
