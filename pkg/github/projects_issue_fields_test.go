package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/http/headers"
	transportpkg "github.com/github/github-mcp-server/pkg/http/transport"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
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
		"issueField":   map[string]any{"id": issueFieldNodeID},
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
			"id": 2002, "node_id": "I_123", "number": 5, "title": "Track customer",
		},
		"fields": fields,
	}
}

func issueProjectItemMatcher(issueNodeID, itemNodeID string, itemID int) githubv4mock.Matcher {
	return githubv4mock.NewQueryMatcher(
		resolveItemByIssueQuery{},
		map[string]any{
			"issueOwner": githubv4.String("octo-org"), "issueRepo": githubv4.String("roadmap"),
			"issueNumber": githubv4.Int(5),
		},
		githubv4mock.DataResponse(map[string]any{
			"repository": map[string]any{"issue": map[string]any{
				"id": issueNodeID,
				"projectItems": map[string]any{
					"nodes": []any{map[string]any{
						"id": itemNodeID, "fullDatabaseId": fmt.Sprintf("%d", itemID),
						"project": map[string]any{"id": "PVT_project1"},
					}},
					"pageInfo": map[string]any{"hasNextPage": false},
				},
			}},
		}),
	)
}

func projectItemIssueByNodeIDMatcher(itemNodeID string, itemID int, issueNodeID string) githubv4mock.Matcher {
	matcher := githubv4mock.NewQueryMatcher(
		batchProjectItemsByNodeIDQuery{},
		map[string]any{"ids": []githubv4.ID{githubv4.ID(itemNodeID)}},
		githubv4mock.DataResponse(map[string]any{
			"nodes": []any{map[string]any{
				"id": itemNodeID, "fullDatabaseId": fmt.Sprintf("%d", itemID),
				"project": map[string]any{"id": "PVT_project1"},
				"content": map[string]any{"__typename": "Issue", "id": issueNodeID},
			}},
		}),
	)
	matcher.Variables["ids"] = []any{itemNodeID}
	return matcher
}

func Test_ResolveProjectFieldByID_AttachedIssueFieldMetadata(t *testing.T) {
	queryTransport := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
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

func Test_BuildIssueFieldUpdate(t *testing.T) {
	number := githubv4.Float(3.5)
	optionID := githubv4.ID("IFO_high")
	deleteValue := githubv4.Boolean(true)
	tests := []struct {
		name     string
		dataType string
		options  []ResolvedFieldOption
		value    any
		want     IssueFieldCreateOrUpdateInput
	}{
		{"text", "TEXT", nil, "Acme", IssueFieldCreateOrUpdateInput{TextValue: githubv4.NewString(githubv4.String("Acme"))}},
		{"number", "NUMBER", nil, 3.5, IssueFieldCreateOrUpdateInput{NumberValue: &number}},
		{"date", "DATE", nil, "2026-08-01", IssueFieldCreateOrUpdateInput{DateValue: githubv4.NewString(githubv4.String("2026-08-01"))}},
		{"single select name", "SINGLE_SELECT", []ResolvedFieldOption{{ID: "IFO_high", Name: "High"}}, "high", IssueFieldCreateOrUpdateInput{SingleSelectOptionID: &optionID}},
		{"single select ID", "SINGLE_SELECT", []ResolvedFieldOption{{ID: "IFO_other", Name: "IFO_high"}, {ID: "IFO_high", Name: "High"}}, "IFO_high", IssueFieldCreateOrUpdateInput{SingleSelectOptionID: &optionID}},
		{"clear", "TEXT", nil, nil, IssueFieldCreateOrUpdateInput{Delete: &deleteValue}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := &ResolvedField{
				Name: "Customer", DataType: tt.dataType, Options: tt.options, IsIssueField: true, IssueFieldNodeID: "IF_customer",
			}
			got, err := buildIssueFieldUpdate(field, tt.value)
			require.NoError(t, err)
			tt.want.FieldID = githubv4.ID("IF_customer")
			assert.Equal(t, tt.want, *got)
		})
	}
}

func Test_BuildIssueFieldUpdate_StructuredErrors(t *testing.T) {
	tests := []struct {
		name  string
		field *ResolvedField
		value any
		code  string
	}{
		{"missing metadata", &ResolvedField{Name: "Customer", DataType: "TEXT", IsIssueField: true}, "Acme", "issue_field_metadata_unavailable"},
		{"wrong value type", &ResolvedField{Name: "Customer", DataType: "TEXT", IsIssueField: true, IssueFieldNodeID: "IF_customer"}, 42.0, "invalid_field_value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildIssueFieldUpdate(tt.field, tt.value)
			require.Error(t, err)
			var response map[string]any
			require.NoError(t, json.Unmarshal([]byte(err.Error()), &response))
			assert.Equal(t, tt.code, response["error"])
		})
	}
}

func Test_UpdateProjectItem_AttachedIssueField(t *testing.T) {
	queryTransport := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				attachedIssueFieldNode("PVTSSF_risk", 704, "IF_risk", "Risk", "SINGLE_SELECT", []map[string]any{{"id": "IFO_high", "name": "High"}}),
			})),
		),
	)
	transport := &mutationAwareTransport{
		t: t, queries: queryTransport.Transport,
		mutationRespond: func(_ int, req capturedGraphQLRequest) (int, string) {
			assert.Contains(t, req.Query, "setIssueFieldValue")
			assert.Equal(t, "update_issue_suggestions", req.Headers.Get(headers.GraphQLFeaturesHeader))
			input := req.Variables["input"].(map[string]any)
			assert.Equal(t, "I_123", input["issueId"])
			assert.Equal(t, "IFO_high", input["issueFields"].([]any)[0].(map[string]any)["singleSelectOptionId"])
			return http.StatusOK, `{"data":{"setIssueFieldValue":{"issue":{"id":"I_123"}}}}`
		},
	}
	deps := BaseDeps{
		Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetOrgsProjectsV2ItemsByProjectByItemID: mockResponse(t, http.StatusOK, issueProjectItemFixture(nil)),
		})),
		GQLClient: githubv4.NewClient(&http.Client{
			Transport: &transportpkg.GraphQLFeaturesTransport{Transport: transport},
		}),
	}
	serverTool := ProjectsWrite(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method": projectsMethodUpdateProjectItem, "owner": "octo-org", "owner_type": "org",
		"project_number": float64(1), "item_id": float64(1001),
		"updated_field": map[string]any{"id": float64(704), "value": "high"},
	})
	result, err := handler(ContextWithDeps(t.Context(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)
	assert.Len(t, transport.mutationCalls, 1)
}

func Test_ProjectItemReads_FieldNamesIncludeAttachedIssueFieldValues(t *testing.T) {
	fieldValue := map[string]any{
		"id": 701, "issue_field_id": 9001, "name": "Customer", "data_type": "text", "value": "Acme",
	}
	tests := []struct {
		name   string
		tool   func(translations.TranslationHelperFunc) inventory.ServerTool
		method string
		route  string
		body   any
	}{
		{"get", ProjectsGet, projectsMethodGetProjectItem, GetOrgsProjectsV2ItemsByProjectByItemID, issueProjectItemFixture([]map[string]any{fieldValue})},
		{"list", ProjectsList, projectsMethodListProjectItems, GetOrgsProjectsV2ItemsByProject, []map[string]any{issueProjectItemFixture([]map[string]any{fieldValue})}},
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
					require.NoError(t, json.NewEncoder(w).Encode(tt.body))
				},
			})
			deps := BaseDeps{Client: mustNewGHClient(t, rest), GQLClient: gql}
			args := map[string]any{
				"method": tt.method, "owner": "octo-org", "owner_type": "org",
				"project_number": float64(1), "field_names": []any{"customer"},
			}
			if tt.method == projectsMethodGetProjectItem {
				args["item_id"] = float64(1001)
			}
			request := createMCPRequest(args)
			serverTool := tt.tool(translations.NullTranslationHelper)
			result, err := serverTool.Handler(deps)(ContextWithDeps(t.Context(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, getTextResult(t, result).Text)
			assert.Contains(t, getTextResult(t, result).Text, `"name":"Customer"`)
			assert.Contains(t, getTextResult(t, result).Text, `"value":"Acme"`)
		})
	}
}

func Test_IssueFieldItemTypeValidation(t *testing.T) {
	for _, contentType := range []string{"PullRequest", "DraftIssue"} {
		t.Run(contentType, func(t *testing.T) {
			item := issueProjectItemFixture(nil)
			item["content_type"] = contentType
			item["content"] = map[string]any{"node_id": "CONTENT_1"}
			raw, err := json.Marshal(item)
			require.NoError(t, err)
			var projectItem github.ProjectV2Item
			require.NoError(t, json.Unmarshal(raw, &projectItem))

			_, err = projectItemIssueNodeID(&projectItem)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `"error":"unsupported_item_type"`)

			node := batchProjectItemIssueNode{ID: githubv4.ID("PVTI_1")}
			node.Content.TypeName = githubv4.String(contentType)
			_, err = batchProjectItemIssueNodeID(node)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `"error":"unsupported_item_type"`)
		})
	}
}

func Test_UpdateProjectItemsBatch_AttachedIssueFields(t *testing.T) {
	tests := []struct {
		name          string
		fieldNode     map[string]any
		updatedField  map[string]any
		item          map[string]any
		extraMatchers []githubv4mock.Matcher
		restHandlers  map[string]http.HandlerFunc
		issueNodeID   string
		itemNodeID    string
		itemID        int
		valueKey      string
		value         any
	}{
		{
			name: "issue reference", fieldNode: attachedIssueFieldNode("PVTF_customer", 701, "IF_customer", "Customer", "TEXT", nil),
			updatedField:  map[string]any{"name": "Customer", "value": "Acme"},
			item:          map[string]any{"item_owner": "octo-org", "item_repo": "roadmap", "issue_number": float64(5)},
			extraMatchers: []githubv4mock.Matcher{issueProjectItemMatcher("I_5", "PVTI_5", 1005)},
			restHandlers:  map[string]http.HandlerFunc{}, issueNodeID: "I_5", itemNodeID: "PVTI_5", itemID: 1005,
			valueKey: "textValue", value: "Acme",
		},
		{
			name: "numeric IDs clear", fieldNode: attachedIssueFieldNode("PVTF_customer", 701, "IF_customer", "Customer", "TEXT", nil),
			updatedField: map[string]any{"id": float64(701), "value": nil},
			item:         map[string]any{"item_id": float64(1001)},
			restHandlers: map[string]http.HandlerFunc{
				GetOrgsProjectsV2ItemsByProjectByItemID: mockResponse(t, http.StatusOK, issueProjectItemFixture(nil)),
			},
			issueNodeID: "I_123", itemNodeID: "PVTI_1", itemID: 1001, valueKey: "delete", value: true,
		},
		{
			name: "node ID and option name", fieldNode: attachedIssueFieldNode("PVTSSF_risk", 704, "IF_risk", "Risk", "SINGLE_SELECT", []map[string]any{{"id": "IFO_high", "name": "High"}}),
			updatedField:  map[string]any{"id": float64(704), "value": "high"},
			item:          map[string]any{"node_id": "PVTI_1"},
			extraMatchers: []githubv4mock.Matcher{projectItemIssueByNodeIDMatcher("PVTI_1", 1001, "I_123")},
			restHandlers:  map[string]http.HandlerFunc{}, issueNodeID: "I_123", itemNodeID: "PVTI_1", itemID: 1001,
			valueKey: "singleSelectOptionId", value: "IFO_high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchers := []githubv4mock.Matcher{
				projectIDMatcher("octo-org", 1, "PVT_project1"),
				githubv4mock.NewQueryMatcher(
					projectFieldsTestQuery{}, fieldsQueryVars("octo-org", 1),
					githubv4mock.DataResponse(fieldsResponse([]map[string]any{tt.fieldNode})),
				),
			}
			matchers = append(matchers, tt.extraMatchers...)
			transport := &mutationAwareTransport{
				t: t, queries: githubv4mock.NewMockedHTTPClient(matchers...).Transport,
				mutationRespond: func(_ int, req capturedGraphQLRequest) (int, string) {
					assert.Contains(t, req.Query, "setIssueFieldValue")
					assert.Equal(t, "update_issue_suggestions", req.Headers.Get(headers.GraphQLFeaturesHeader))
					input := req.Variables["input"].(map[string]any)
					assert.Equal(t, tt.issueNodeID, input["issueId"])
					field := input["issueFields"].([]any)[0].(map[string]any)
					assert.Equal(t, tt.value, field[tt.valueKey])
					return http.StatusOK, issueFieldMutationDataResponse(t, map[int]string{0: tt.issueNodeID})
				},
			}
			result, _, err := updateProjectItemsBatch(
				t.Context(),
				mustNewGHClient(t, MockHTTPClientWithHandlers(tt.restHandlers)),
				githubv4.NewClient(&http.Client{Transport: &transportpkg.GraphQLFeaturesTransport{Transport: transport}}),
				"octo-org", "org", 1,
				map[string]any{"updated_field": tt.updatedField, "items": []any{tt.item}},
			)
			require.NoError(t, err)
			require.False(t, result.IsError, getTextResult(t, result).Text)
			assert.Contains(t, getTextResult(t, result).Text, tt.itemNodeID)
			assert.Contains(t, getTextResult(t, result).Text, fmt.Sprintf(`"item_id":%d`, tt.itemID))
		})
	}
}
