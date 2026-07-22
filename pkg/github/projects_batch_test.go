package github

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_UpdateProjectItemsBatch_TopLevelGuards(t *testing.T) {
	tooMany := make([]any, maxProjectItemsPerBatch+1)
	validItem := map[string]any{"node_id": "PVTI_item1"}
	validField := map[string]any{"name": "Notes", "value": "hello"}
	tests := []struct {
		name    string
		args    map[string]any
		wantErr string
	}{
		{name: "missing items", args: map[string]any{}, wantErr: "missing required parameter: items"},
		{name: "non-array items", args: map[string]any{"items": "invalid"}, wantErr: "items must be an array"},
		{name: "empty items", args: map[string]any{"items": []any{}}, wantErr: "items must contain at least one entry"},
		{name: "too many items", args: map[string]any{"items": tooMany}, wantErr: "items exceeds maximum of 50 entries"},
		{name: "missing updated field", args: map[string]any{"items": []any{validItem}}, wantErr: "missing required parameter: updated_field"},
		{name: "malformed updated field", args: map[string]any{"items": []any{validItem}, "updated_field": "invalid"}, wantErr: "updated_field must be an object"},
		{name: "missing field value", args: map[string]any{"items": []any{validItem}, "updated_field": map[string]any{"name": "Notes"}}, wantErr: "updated_field.value is required"},
		{name: "missing field reference", args: map[string]any{"items": []any{validItem}, "updated_field": map[string]any{"value": "hello"}}, wantErr: "updated_field requires either id or name"},
		{name: "ambiguous field reference", args: map[string]any{"items": []any{validItem}, "updated_field": map[string]any{"id": float64(1), "name": "Notes", "value": "hello"}}, wantErr: "updated_field must set either id or name"},
		{name: "nil GraphQL client", args: map[string]any{"items": []any{validItem}, "updated_field": validField}, wantErr: "gqlClient is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, structured, err := updateProjectItemsBatch(t.Context(), nil, nil, "octo-org", "org", 1, tt.args)
			require.NoError(t, err)
			assert.Nil(t, structured)
			assert.Contains(t, getErrorResult(t, result).Text, tt.wantErr)
		})
	}
}

func Test_ParseItemRef_ExactlyOneFormRequired(t *testing.T) {
	tests := []struct {
		name    string
		entry   map[string]any
		wantErr string
	}{
		{
			name:    "none provided",
			entry:   map[string]any{},
			wantErr: "exactly one of",
		},
		{
			name:    "node_id and item_id both provided",
			entry:   map[string]any{"node_id": "PVTI_x", "item_id": float64(1)},
			wantErr: "not more than one",
		},
		{
			name:    "item_id and issue ref both provided",
			entry:   map[string]any{"item_id": float64(1), "item_owner": "o", "item_repo": "r", "issue_number": float64(1)},
			wantErr: "not more than one",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parsedBatchItem{}
			err := p.parseItemRef(tt.entry)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func Test_ParseItemRef_NodeIDBypassesLookup(t *testing.T) {
	p := parsedBatchItem{}
	err := p.parseItemRef(map[string]any{"node_id": "PVTI_abc123"})
	require.NoError(t, err)
	assert.Equal(t, batchRefNodeID, p.refKind)
	assert.Equal(t, "PVTI_abc123", p.nodeID)
}

func Test_ParseItemRef_ItemID(t *testing.T) {
	p := parsedBatchItem{}
	err := p.parseItemRef(map[string]any{"item_id": float64(42)})
	require.NoError(t, err)
	assert.Equal(t, batchRefItemID, p.refKind)
	assert.Equal(t, int64(42), p.itemID)
}

func Test_ParseItemRef_IssueRef(t *testing.T) {
	p := parsedBatchItem{}
	err := p.parseItemRef(map[string]any{"item_owner": "github", "item_repo": "planning-tracking", "issue_number": float64(123)})
	require.NoError(t, err)
	assert.Equal(t, batchRefIssue, p.refKind)
	assert.Equal(t, "github", p.issueOwner)
	assert.Equal(t, "planning-tracking", p.issueRepo)
	assert.Equal(t, 123, p.issueNumber)
}

func Test_ParseItemRef_InvalidNumericReferences(t *testing.T) {
	issueRef := func(value any) map[string]any {
		return map[string]any{
			"item_owner":   "github",
			"item_repo":    "planning-tracking",
			"issue_number": value,
		}
	}
	tests := []struct {
		name  string
		entry map[string]any
	}{
		{name: "zero item ID", entry: map[string]any{"item_id": float64(0)}},
		{name: "negative item ID", entry: map[string]any{"item_id": float64(-1)}},
		{name: "fractional item ID", entry: map[string]any{"item_id": float64(1.5)}},
		{name: "NaN item ID", entry: map[string]any{"item_id": math.NaN()}},
		{name: "infinite item ID", entry: map[string]any{"item_id": math.Inf(1)}},
		{name: "overflowing item ID", entry: map[string]any{"item_id": math.MaxFloat64}},
		{name: "zero issue number", entry: issueRef(float64(0))},
		{name: "negative issue number", entry: issueRef(float64(-1))},
		{name: "fractional issue number", entry: issueRef(float64(1.5))},
		{name: "overflowing issue number", entry: issueRef(float64(math.MaxInt32) + 1)},
		{name: "NaN issue number", entry: issueRef(math.NaN())},
		{name: "infinite issue number", entry: issueRef(math.Inf(1))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parsedBatchItem{}
			err := p.parseItemRef(tt.entry)
			require.Error(t, err)
		})
	}
}

func Test_ParseItemRef_PartialIssueRefIsError(t *testing.T) {
	p := parsedBatchItem{}
	err := p.parseItemRef(map[string]any{"item_owner": "github"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must all be provided together")
}

func Test_ParseBatchItemEntry_InvalidShape(t *testing.T) {
	p := parseBatchItemEntry(0, "not-an-object")
	require.NotNil(t, p.err)
	assert.Equal(t, "invalid_item", p.err.Code)
}

func Test_ParseBatchItemEntry_RejectsPerItemUpdatedField(t *testing.T) {
	p := parseBatchItemEntry(0, map[string]any{
		"node_id":       "PVTI_1",
		"updated_field": map[string]any{"name": "Notes", "value": "x"},
	})
	require.NotNil(t, p.err)
	assert.Contains(t, p.err.Message, "use the top-level updated_field")
}

func Test_ConvertProjectFieldValue_Text(t *testing.T) {
	field := &ResolvedField{Name: "Notes", DataType: "TEXT"}
	v, err := convertProjectFieldValue(field, "hello")
	require.NoError(t, err)
	require.NotNil(t, v.Text)
	assert.Equal(t, "hello", string(*v.Text))
}

func Test_ConvertProjectFieldValue_Text_WrongType(t *testing.T) {
	field := &ResolvedField{Name: "Notes", DataType: "TEXT"}
	_, err := convertProjectFieldValue(field, float64(1))
	require.Error(t, err)
}

func Test_ConvertProjectFieldValue_Number(t *testing.T) {
	field := &ResolvedField{Name: "Estimate", DataType: "NUMBER"}
	v, err := convertProjectFieldValue(field, float64(8))
	require.NoError(t, err)
	require.NotNil(t, v.Number)
	assert.InDelta(t, 8.0, float64(*v.Number), 0.0001)
}

func Test_ConvertProjectFieldValue_Number_NonFinite(t *testing.T) {
	field := &ResolvedField{Name: "Estimate", DataType: "NUMBER"}
	for _, value := range []float64{math.NaN(), math.Inf(-1), math.Inf(1)} {
		_, err := convertProjectFieldValue(field, value)
		require.Error(t, err)
	}
}

func Test_ConvertProjectFieldValue_Date(t *testing.T) {
	field := &ResolvedField{Name: "Due", DataType: "DATE"}
	v, err := convertProjectFieldValue(field, "2024-01-15")
	require.NoError(t, err)
	require.NotNil(t, v.Date)
	assert.Equal(t, 2024, v.Date.Year())
	assert.Equal(t, 1, int(v.Date.Month()))
	assert.Equal(t, 15, v.Date.Day())
}

func Test_ConvertProjectFieldValue_Date_BadFormat(t *testing.T) {
	field := &ResolvedField{Name: "Due", DataType: "DATE"}
	_, err := convertProjectFieldValue(field, "01/15/2024")
	require.Error(t, err)
}

func Test_ConvertProjectFieldValue_SingleSelect_ByName(t *testing.T) {
	field := &ResolvedField{
		Name:     "Status",
		DataType: "SINGLE_SELECT",
		Options:  []ResolvedFieldOption{{ID: "OPT_1", Name: "In Progress"}},
	}
	v, err := convertProjectFieldValue(field, "In Progress")
	require.NoError(t, err)
	require.NotNil(t, v.SingleSelectOptionID)
	assert.Equal(t, "OPT_1", string(*v.SingleSelectOptionID))
}

func Test_ConvertProjectFieldValue_SingleSelect_ByOptionID(t *testing.T) {
	field := &ResolvedField{
		Name:     "Status",
		DataType: "SINGLE_SELECT",
		Options:  []ResolvedFieldOption{{ID: "OPT_1", Name: "In Progress"}},
	}
	v, err := convertProjectFieldValue(field, "OPT_1")
	require.NoError(t, err)
	require.NotNil(t, v.SingleSelectOptionID)
	assert.Equal(t, "OPT_1", string(*v.SingleSelectOptionID))
}

func Test_ConvertProjectFieldValue_SingleSelect_Unknown(t *testing.T) {
	field := &ResolvedField{
		Name:     "Status",
		DataType: "SINGLE_SELECT",
		Options:  []ResolvedFieldOption{{ID: "OPT_1", Name: "In Progress"}},
	}
	_, err := convertProjectFieldValue(field, "Nonexistent")
	require.Error(t, err)
}

func Test_ConvertProjectFieldValue_Iteration(t *testing.T) {
	field := &ResolvedField{Name: "Sprint", DataType: "ITERATION"}
	v, err := convertProjectFieldValue(field, "abc123==")
	require.NoError(t, err)
	require.NotNil(t, v.IterationID)
	assert.Equal(t, "abc123==", string(*v.IterationID))
}

func Test_ConvertProjectFieldValue_Iteration_EmptyIsError(t *testing.T) {
	field := &ResolvedField{Name: "Sprint", DataType: "ITERATION"}
	_, err := convertProjectFieldValue(field, "")
	require.Error(t, err)
}

func Test_ConvertProjectFieldValue_UnsupportedDataType(t *testing.T) {
	field := &ResolvedField{Name: "Assignees", DataType: "ASSIGNEES"}
	_, err := convertProjectFieldValue(field, "someone")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported data type")
	assert.Contains(t, err.Error(), "update_project_item")
}

func Test_ResolveBatchProjectField_ByIDAndName(t *testing.T) {
	tests := []struct {
		name   string
		spec   batchFieldSpec
		wantID string
	}{
		{name: "numeric ID", spec: batchFieldSpec{id: 101}, wantID: "PVTF_status"},
		{name: "case-insensitive name", spec: batchFieldSpec{name: "priority"}, wantID: "PVTF_priority"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocked := githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					projectFieldsTestQuery{},
					fieldsQueryVars("octo-org", 7),
					githubv4mock.DataResponse(fieldsResponse([]map[string]any{
						statusFieldNode("PVTF_status", 101, "Status", nil),
						statusFieldNode("PVTF_priority", 202, "Priority", nil),
					})),
				),
			)

			field, err := resolveBatchProjectField(t.Context(), githubv4.NewClient(mocked), "octo-org", "org", 7, tt.spec)
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, field.NodeID)
		})
	}
}

func Test_ResolveBatchProjectField_AmbiguousName(t *testing.T) {
	mocked := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			projectFieldsTestQuery{},
			fieldsQueryVars("octo-org", 7),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_status1", 101, "Status", nil),
				statusFieldNode("PVTSSF_status2", 202, "Status", nil),
			})),
		),
	)

	_, err := resolveBatchProjectField(
		t.Context(),
		githubv4.NewClient(mocked),
		"octo-org",
		"org",
		7,
		batchFieldSpec{name: "status"},
	)
	require.Error(t, err)

	var response struct {
		Error      string           `json:"error"`
		Candidates []map[string]any `json:"candidates"`
	}
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &response))
	assert.Equal(t, "field_ambiguous", response.Error)
	require.Len(t, response.Candidates, 2)
	assert.ElementsMatch(t, []any{"101", "202"}, []any{response.Candidates[0]["id"], response.Candidates[1]["id"]})
}

func Test_ResolveItemNodeIDsByNumericID_DeduplicatesOrgAndUserLookups(t *testing.T) {
	tests := []struct {
		name      string
		ownerType string
		endpoint  string
	}{
		{name: "organization", ownerType: "org", endpoint: GetOrgsProjectsV2ItemsByProjectByItemID},
		{name: "user", ownerType: "user", endpoint: GetUsersProjectsV2ItemsByUsernameByProjectByItemID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				tt.endpoint: func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"id":1001,"node_id":"PVTI_item1001"}`))
				},
			}))

			resolved := resolveItemNodeIDsByNumericID(t.Context(), client, "octocat", tt.ownerType, 1, []int64{1001, 1001})

			require.NoError(t, resolved[1001].err)
			assert.Equal(t, "PVTI_item1001", resolved[1001].nodeID)
			assert.Equal(t, 1, calls)
		})
	}
}

func Test_ExecuteBatchWrites_AllAliasGraphQLErrorContinues(t *testing.T) {
	transport := &sequencedGraphQLTransport{
		t: t,
		responses: []func(capturedGraphQLRequest) (int, string){
			func(_ capturedGraphQLRequest) (int, string) {
				return http.StatusOK, mutationErrorResponse(t, map[string]any{}, "all aliases failed")
			},
			func(_ capturedGraphQLRequest) (int, string) {
				return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
					0: {NodeID: "PVTI_item20", FullDatabaseID: "1020"},
				})
			},
		},
	}
	items, results := batchItemsOfSize(21)

	executeTestBatchWrites(t.Context(), newTestGQLClient(transport), items, results)

	assert.Len(t, transport.calls, 2)
	for i := range 20 {
		assert.Equal(t, batchItemUnknown, results[i].Status)
	}
	assert.Equal(t, batchItemSucceeded, results[20].Status)
}

func Test_ExecuteBatchWrites_PartialGraphQLErrorPreservesSuccess(t *testing.T) {
	transport := &sequencedGraphQLTransport{
		t: t,
		responses: []func(capturedGraphQLRequest) (int, string){
			func(_ capturedGraphQLRequest) (int, string) {
				return http.StatusOK, mutationErrorResponse(t, map[string]any{
					"item0": map[string]any{
						"projectV2Item": map[string]any{"id": "PVTI_item0", "fullDatabaseId": "1000"},
					},
					"item1": nil,
				}, "item1 failed")
			},
		},
	}
	items, results := batchItemsOfSize(2)

	executeTestBatchWrites(t.Context(), newTestGQLClient(transport), items, results)

	assert.Equal(t, batchItemSucceeded, results[0].Status)
	assert.Equal(t, items[0].ref, results[0].Ref)
	assert.Equal(t, batchItemUnknown, results[1].Status)
	assert.Equal(t, items[1].ref, results[1].Ref)
}

func Test_ExecuteBatchWrites_AmbiguousSuccessResponseAborts(t *testing.T) {
	tests := []struct {
		name               string
		body               string
		confirmedSuccesses int
	}{
		{
			name: "null data",
			body: `{"data":null}`,
		},
		{
			name: "missing data",
			body: `{}`,
		},
		{
			name: "partial data without errors",
			body: mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
				0: {NodeID: "PVTI_item0", FullDatabaseID: "1000"},
			}),
			confirmedSuccesses: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &sequencedGraphQLTransport{
				t: t,
				responses: []func(capturedGraphQLRequest) (int, string){
					func(_ capturedGraphQLRequest) (int, string) {
						return http.StatusOK, tt.body
					},
					func(_ capturedGraphQLRequest) (int, string) {
						return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
							0: {NodeID: "PVTI_item20", FullDatabaseID: "1020"},
						})
					},
				},
			}
			items, results := batchItemsOfSize(21)

			executeTestBatchWrites(t.Context(), newTestGQLClient(transport), items, results)

			assert.Len(t, transport.calls, 1)
			for i, result := range results {
				if i < tt.confirmedSuccesses {
					assert.Equal(t, batchItemSucceeded, result.Status)
					continue
				}
				assert.Equal(t, batchItemUnknown, result.Status)
			}
		})
	}
}

func Test_ExecuteBatchWrites_TransportTimeoutAborts(t *testing.T) {
	transport := &errorGraphQLTransport{err: context.DeadlineExceeded}
	items, results := batchItemsOfSize(21)

	executeTestBatchWrites(t.Context(), newTestGQLClient(transport), items, results)

	assert.Equal(t, 1, transport.calls)
	for _, result := range results {
		assert.Equal(t, batchItemUnknown, result.Status)
	}
}

func Test_ExecuteBatchWrites_CanceledContextSkipsWrites(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	transport := &sequencedGraphQLTransport{t: t}
	items, results := batchItemsOfSize(21)

	executeTestBatchWrites(ctx, newTestGQLClient(transport), items, results)

	assert.Empty(t, transport.calls)
	for _, result := range results {
		assert.Equal(t, batchItemUnknown, result.Status)
	}
}

func executeTestBatchWrites(ctx context.Context, gqlClient *githubv4.Client, items []resolvedBatchItem, results []batchItemResult) {
	executeBatchWrites(
		ctx,
		batchWriteOperation{
			gqlClient: gqlClient,
			kind:      batchMutationUpdate,
			projectID: githubv4.ID("PVT_project"),
			fieldID:   githubv4.ID("PVTF_field"),
			value:     githubv4.ProjectV2FieldValue{Text: githubv4.NewString("value")},
		},
		items,
		results,
	)
}

func batchItemsOfSize(n int) ([]resolvedBatchItem, []batchItemResult) {
	items := make([]resolvedBatchItem, n)
	for i := range n {
		nodeID := fmt.Sprintf("PVTI_item%d", i)
		items[i] = resolvedBatchItem{
			index:  i,
			ref:    map[string]any{"node_id": nodeID},
			nodeID: nodeID,
		}
	}
	return items, make([]batchItemResult, n)
}
