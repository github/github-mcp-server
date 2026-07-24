package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedGraphQLRequest is one HTTP request observed by sequencedGraphQLTransport.
type capturedGraphQLRequest struct {
	Query     string
	Variables map[string]any
	Headers   http.Header
}

// sequencedGraphQLTransport is a minimal fake http.RoundTripper for exercising
// executeAliasedMutation without needing to hand-construct
// the exact minified GraphQL query text that reflect.StructOf produces: each call
// is served by the next entry in responses, in order, and the parsed query +
// variables are recorded for assertions.
type sequencedGraphQLTransport struct {
	t         *testing.T
	responses []func(req capturedGraphQLRequest) (status int, body string)
	calls     []capturedGraphQLRequest
}

func (s *sequencedGraphQLTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	captured := capturedGraphQLRequest{Query: parsed.Query, Variables: parsed.Variables, Headers: req.Header.Clone()}
	s.calls = append(s.calls, captured)

	idx := len(s.calls) - 1
	if idx >= len(s.responses) {
		s.t.Fatalf("unexpected GraphQL call #%d (query: %s)", idx, parsed.Query)
	}
	status, body := s.responses[idx](captured)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

type errorGraphQLTransport struct {
	err   error
	calls int
}

func (t *errorGraphQLTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls++
	return nil, t.err
}

// mutationDataResponse builds a `{"data": {...}}` JSON body with one
// "itemN"."projectV2Item" entry per populated index in ids.
func mutationDataResponse(t *testing.T, ids map[int]struct{ NodeID, FullDatabaseID string }) string {
	t.Helper()
	data := make(map[string]any, len(ids))
	for i, v := range ids {
		data[fmt.Sprintf("item%d", i)] = map[string]any{
			"projectV2Item": map[string]any{
				"id":             v.NodeID,
				"fullDatabaseId": v.FullDatabaseID,
			},
		}
	}
	body, err := json.Marshal(map[string]any{"data": data})
	require.NoError(t, err)
	return string(body)
}

func issueFieldMutationDataResponse(t *testing.T, ids map[int]string) string {
	t.Helper()
	data := make(map[string]any, len(ids))
	for i, id := range ids {
		data[fmt.Sprintf("item%d", i)] = map[string]any{
			"issue": map[string]any{"id": id},
		}
	}
	body, err := json.Marshal(map[string]any{"data": data})
	require.NoError(t, err)
	return string(body)
}

func mutationErrorResponse(t *testing.T, data map[string]any, message string) string {
	t.Helper()
	payload := map[string]any{
		"errors": []map[string]any{{"message": message}},
	}
	if data != nil {
		payload["data"] = data
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	return string(body)
}

func newTestGQLClient(transport http.RoundTripper) *githubv4.Client {
	return githubv4.NewClient(&http.Client{Transport: transport})
}

func inputsOfSize(n int) []githubv4.Input {
	inputs := make([]githubv4.Input, n)
	for i := range n {
		inputs[i] = githubv4.UpdateProjectV2ItemFieldValueInput{
			ProjectID: githubv4.ID("PVT_project"),
			ItemID:    githubv4.ID(fmt.Sprintf("PVTI_item%d", i)),
			FieldID:   githubv4.ID("PVTF_field"),
			Value:     githubv4.ProjectV2FieldValue{Text: githubv4.NewString("v")},
		}
	}
	return inputs
}

func issueFieldInputsOfSize(n int) []githubv4.Input {
	inputs := make([]githubv4.Input, n)
	for i := range n {
		inputs[i] = SetIssueFieldValueInput{
			IssueID: githubv4.ID(fmt.Sprintf("I_issue%d", i)),
			IssueFields: []IssueFieldCreateOrUpdateInput{{
				FieldID:   githubv4.ID("IF_field"),
				TextValue: githubv4.NewString("v"),
			}},
		}
	}
	return inputs
}

func Test_BuildAliasedMutationType_FieldNamesAndTags(t *testing.T) {
	for _, size := range []int{1, 2, 20} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			typ := buildAliasedMutationType(batchMutationUpdate, size)
			require.Equal(t, size, typ.NumField())
			for i := range size {
				field := typ.Field(i)
				assert.Equal(t, fmt.Sprintf("Item%d", i), field.Name)

				tag, ok := field.Tag.Lookup("graphql")
				require.True(t, ok)

				wantVar := "input"
				if i > 0 {
					wantVar = fmt.Sprintf("input%d", i)
				}
				wantTag := fmt.Sprintf("item%d: updateProjectV2ItemFieldValue(input: $%s)", i, wantVar)
				assert.Equal(t, wantTag, tag)

				// No owner/id/name/value data may ever appear in the tag: only
				// positional aliases and variable references.
				assert.NotContains(t, tag, "PVT_")
				assert.NotContains(t, tag, "octo")
			}
		})
	}
}

func Test_BuildAliasedMutationType_ClearKindUsesClearMutation(t *testing.T) {
	typ := buildAliasedMutationType(batchMutationClear, 2)
	tag0 := typ.Field(0).Tag.Get("graphql")
	tag1 := typ.Field(1).Tag.Get("graphql")
	assert.Equal(t, "item0: clearProjectV2ItemFieldValue(input: $input)", tag0)
	assert.Equal(t, "item1: clearProjectV2ItemFieldValue(input: $input1)", tag1)
}

func Test_BuildAliasedMutationType_IssueFieldKindUsesSetIssueFieldValue(t *testing.T) {
	typ := buildAliasedMutationType(batchMutationSetIssueField, 2)
	assert.Equal(t, reflect.TypeFor[issueFieldMutationResult](), typ.Field(0).Type)
	assert.Equal(t, "item0: setIssueFieldValue(input: $input)", typ.Field(0).Tag.Get("graphql"))
	assert.Equal(t, "item1: setIssueFieldValue(input: $input1)", typ.Field(1).Tag.Get("graphql"))
}

func Test_BuildAliasedMutationType_CachedByKindAndSize(t *testing.T) {
	a := buildAliasedMutationType(batchMutationUpdate, 3)
	b := buildAliasedMutationType(batchMutationUpdate, 3)
	assert.True(t, a == b, "expected the same cached reflect.Type for identical (kind, size)")

	c := buildAliasedMutationType(batchMutationClear, 3)
	assert.False(t, a == c, "update and clear must not share a cached type")

	d := buildAliasedMutationType(batchMutationUpdate, 4)
	assert.False(t, a == d, "different sizes must not share a cached type")
}

func Test_ExecuteAliasedMutation_OneAlias(t *testing.T) {
	transport := &sequencedGraphQLTransport{
		t: t,
		responses: []func(capturedGraphQLRequest) (int, string){
			func(req capturedGraphQLRequest) (int, string) {
				// Single alias: the only input is bound positionally via
				// Client.Mutate's third argument, so no extra variables map entries.
				assert.Len(t, req.Variables, 1)
				assert.Contains(t, req.Variables, "input")
				return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
					0: {NodeID: "PVTI_item0", FullDatabaseID: "1001"},
				})
			},
		},
	}
	gqlClient := newTestGQLClient(transport)

	outcomes, err := executeAliasedMutation(context.Background(), gqlClient, batchMutationUpdate, inputsOfSize(1))
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	assert.True(t, outcomes[0].Populated)
	assert.Equal(t, "PVTI_item0", outcomes[0].NodeID)
	assert.Equal(t, "1001", outcomes[0].FullDatabaseID)
}

func Test_ExecuteAliasedMutation_TwoAliases_FirstInputWorkaround(t *testing.T) {
	transport := &sequencedGraphQLTransport{
		t: t,
		responses: []func(capturedGraphQLRequest) (int, string){
			func(req capturedGraphQLRequest) (int, string) {
				// Alias 0's input is always bound to the reserved "input" wire
				// variable by Client.Mutate; alias 1's input must be supplied
				// separately (as "input1") since a GraphQL variable can only be
				// referenced with one value per request.
				require.Contains(t, req.Variables, "input1")
				require.Contains(t, req.Variables, "input")
				return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
					0: {NodeID: "PVTI_item0", FullDatabaseID: "1001"},
					1: {NodeID: "PVTI_item1", FullDatabaseID: "1002"},
				})
			},
		},
	}

	gqlClient := newTestGQLClient(transport)

	outcomes, err := executeAliasedMutation(context.Background(), gqlClient, batchMutationUpdate, inputsOfSize(2))
	require.NoError(t, err)
	require.Len(t, outcomes, 2)
	assert.True(t, outcomes[0].Populated)
	assert.True(t, outcomes[1].Populated)
}

func Test_ExecuteAliasedMutation_IssueFieldAliases(t *testing.T) {
	transport := &sequencedGraphQLTransport{
		t: t,
		responses: []func(capturedGraphQLRequest) (int, string){
			func(req capturedGraphQLRequest) (int, string) {
				assert.Contains(t, req.Query, "setIssueFieldValue")
				assert.NotContains(t, req.Query, "updateProjectV2ItemFieldValue")
				return http.StatusOK, issueFieldMutationDataResponse(t, map[int]string{
					0: "I_issue0",
					1: "I_issue1",
				})
			},
		},
	}

	outcomes, err := executeAliasedMutation(t.Context(), newTestGQLClient(transport), batchMutationSetIssueField, issueFieldInputsOfSize(2))
	require.NoError(t, err)
	require.Len(t, outcomes, 2)
	assert.Equal(t, mutationAliasOutcome{Populated: true, NodeID: "I_issue0"}, outcomes[0])
	assert.Equal(t, mutationAliasOutcome{Populated: true, NodeID: "I_issue1"}, outcomes[1])
}

func Test_ExecuteAliasedMutation_IssueFieldPartialData(t *testing.T) {
	transport := &sequencedGraphQLTransport{
		t: t,
		responses: []func(capturedGraphQLRequest) (int, string){
			func(capturedGraphQLRequest) (int, string) {
				data := map[string]any{
					"item0": map[string]any{"issue": map[string]any{"id": "I_issue0"}},
				}
				return http.StatusOK, mutationErrorResponse(t, data, "item1 failed")
			},
		},
	}

	outcomes, err := executeAliasedMutation(t.Context(), newTestGQLClient(transport), batchMutationSetIssueField, issueFieldInputsOfSize(2))
	require.Error(t, err)
	assert.True(t, isGraphQLResponseError(err))
	assert.Equal(t, mutationAliasOutcome{Populated: true, NodeID: "I_issue0"}, outcomes[0])
	assert.Equal(t, mutationAliasOutcome{}, outcomes[1])
}

func Test_ExecuteAliasedMutation_PreservesPartialDataWithGraphQLErrors(t *testing.T) {
	transport := &sequencedGraphQLTransport{
		t: t,
		responses: []func(capturedGraphQLRequest) (int, string){
			func(capturedGraphQLRequest) (int, string) {
				data := map[string]any{
					"item0": map[string]any{
						"projectV2Item": map[string]any{
							"id":             "PVTI_item0",
							"fullDatabaseId": "1001",
						},
					},
				}
				return http.StatusOK, mutationErrorResponse(t, data, "item1 failed")
			},
		},
	}

	outcomes, err := executeAliasedMutation(t.Context(), newTestGQLClient(transport), batchMutationUpdate, inputsOfSize(2))
	require.Error(t, err)
	assert.True(t, isGraphQLResponseError(err))
	require.Len(t, outcomes, 2)
	assert.Equal(t, mutationAliasOutcome{
		Populated:      true,
		NodeID:         "PVTI_item0",
		FullDatabaseID: "1001",
	}, outcomes[0])
	assert.Equal(t, mutationAliasOutcome{}, outcomes[1])
}

func Test_ExecuteAliasedMutation_TwentyAliases(t *testing.T) {
	ids := make(map[int]struct{ NodeID, FullDatabaseID string }, 20)
	for i := range 20 {
		ids[i] = struct{ NodeID, FullDatabaseID string }{
			NodeID:         fmt.Sprintf("PVTI_item%d", i),
			FullDatabaseID: fmt.Sprintf("%d", 1000+i),
		}
	}
	transport := &sequencedGraphQLTransport{
		t: t,
		responses: []func(capturedGraphQLRequest) (int, string){
			func(req capturedGraphQLRequest) (int, string) {
				assert.Len(t, req.Variables, 20) // "input" (positional) plus input1..input19
				return http.StatusOK, mutationDataResponse(t, ids)
			},
		},
	}
	gqlClient := newTestGQLClient(transport)

	outcomes, err := executeAliasedMutation(context.Background(), gqlClient, batchMutationUpdate, inputsOfSize(20))
	require.NoError(t, err)
	require.Len(t, outcomes, 20)
	for i, oc := range outcomes {
		assert.Truef(t, oc.Populated, "outcome %d should be populated", i)
	}
}

func Test_ExecuteAliasedMutation_ChunkSizeExceeded(t *testing.T) {
	gqlClient := newTestGQLClient(&sequencedGraphQLTransport{t: t})
	_, err := executeAliasedMutation(context.Background(), gqlClient, batchMutationUpdate, inputsOfSize(21))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds wire chunk size")
}

func Test_ExecuteAliasedMutation_EmptyInputsIsNoop(t *testing.T) {
	gqlClient := newTestGQLClient(&sequencedGraphQLTransport{t: t})
	outcomes, err := executeAliasedMutation(context.Background(), gqlClient, batchMutationUpdate, nil)
	require.NoError(t, err)
	assert.Nil(t, outcomes)
}

func Test_ProjectV2ItemMutationResult_ReflectFieldTypeIsConcrete(t *testing.T) {
	// executeAliasedMutation type-asserts each reflected field back to
	// projectV2ItemMutationResult directly; guard that assumption here.
	typ := buildAliasedMutationType(batchMutationUpdate, 1)
	assert.Equal(t, reflect.TypeFor[projectV2ItemMutationResult](), typ.Field(0).Type)
}

func Test_IsGraphQLResponseError(t *testing.T) {
	graphqlTransport := &sequencedGraphQLTransport{
		t: t,
		responses: []func(capturedGraphQLRequest) (int, string){
			func(_ capturedGraphQLRequest) (int, string) {
				return http.StatusOK, mutationErrorResponse(t, nil, "mutation failed")
			},
		},
	}
	_, graphqlErr := executeAliasedMutation(t.Context(), newTestGQLClient(graphqlTransport), batchMutationUpdate, inputsOfSize(1))
	require.Error(t, graphqlErr)
	assert.True(t, isGraphQLResponseError(graphqlErr))

	transport := &errorGraphQLTransport{err: context.DeadlineExceeded}
	_, transportErr := executeAliasedMutation(t.Context(), newTestGQLClient(transport), batchMutationUpdate, inputsOfSize(1))
	require.Error(t, transportErr)
	assert.False(t, isGraphQLResponseError(transportErr))
	assert.False(t, isGraphQLResponseError(errors.New("plain error")))
	assert.False(t, isGraphQLResponseError(nil))
}
