package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fieldNode is a generic project field response node for use in mock data,
// covering data types beyond SINGLE_SELECT (statusFieldNode in
// projects_resolver_test.go is fixed to SINGLE_SELECT). See the comment on
// listAllProjectFields's inline-fragment decoding: the underlying jsonutil
// decoder populates id/databaseId/name/dataType identically across all three
// ProjectV2*Field fragments for a flat node object, so a single flat map
// (with "options" only where relevant) is sufficient regardless of dataType.
func fieldNode(nodeID string, databaseID int, name, dataType string) map[string]any {
	return map[string]any{
		"id":         nodeID,
		"databaseId": databaseID,
		"name":       name,
		"dataType":   dataType,
	}
}

// projectIDMatcher returns the githubv4mock matcher for the org project-node-ID
// resolution query issued once per update_project_items call.
func projectIDMatcher(owner string, projectNumber int, projectNodeID string) githubv4mock.Matcher {
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
			"projectNumber": githubv4.Int(int32(projectNumber)), //nolint:gosec
		},
		githubv4mock.DataResponse(map[string]any{
			"organization": map[string]any{
				"projectV2": map[string]any{"id": projectNodeID},
			},
		}),
	)
}

// mutationAwareTransport routes GraphQL requests to a fixed query-matcher
// transport (e.g. githubv4mock.NewMockedHTTPClient's Transport) for ordinary
// queries/lookups, and to a sequenced, call-counted responder for mutation
// requests, so end-to-end tests can assert on aliased-mutation call counts and
// per-call variables without needing to hand-construct the exact minified
// mutation query text that reflect.StructOf produces.
type mutationAwareTransport struct {
	t               *testing.T
	queries         http.RoundTripper
	mutationRespond func(callIndex int, req capturedGraphQLRequest) (status int, body string)
	queryCalls      []capturedGraphQLRequest
	mutationCalls   []capturedGraphQLRequest
}

func (m *mutationAwareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()

	var parsed struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	if !strings.HasPrefix(strings.TrimSpace(parsed.Query), "mutation") {
		m.queryCalls = append(m.queryCalls, capturedGraphQLRequest{Query: parsed.Query, Variables: parsed.Variables, Headers: req.Header.Clone()})
		req.Body = io.NopCloser(strings.NewReader(string(raw)))
		return m.queries.RoundTrip(req)
	}

	captured := capturedGraphQLRequest{Query: parsed.Query, Variables: parsed.Variables, Headers: req.Header.Clone()}
	idx := len(m.mutationCalls)
	m.mutationCalls = append(m.mutationCalls, captured)
	if m.mutationRespond == nil {
		m.t.Fatalf("unexpected mutation call #%d (query: %s)", idx, parsed.Query)
	}
	status, body := m.mutationRespond(idx, captured)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

type gatedIssueLookupTransport struct {
	gate      <-chan struct{}
	started   chan int
	projectID string

	mu     sync.Mutex
	active int
	peak   int
	calls  map[int]int
}

func newGatedIssueLookupTransport(gate <-chan struct{}, projectID string) *gatedIssueLookupTransport {
	return &gatedIssueLookupTransport{
		gate:      gate,
		started:   make(chan int, maxProjectItemsPerBatch),
		projectID: projectID,
		calls:     make(map[int]int),
	}
}

func (t *gatedIssueLookupTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()

	var parsed struct {
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	rawIssueNumber, ok := parsed.Variables["issueNumber"].(float64)
	if !ok {
		return nil, fmt.Errorf("issueNumber variable is missing or invalid")
	}
	issueNumber := int(rawIssueNumber)

	t.mu.Lock()
	t.calls[issueNumber]++
	t.active++
	t.peak = max(t.peak, t.active)
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		t.active--
		t.mu.Unlock()
	}()

	t.started <- issueNumber
	select {
	case <-t.gate:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}

	body, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"repository": map[string]any{
				"issue": map[string]any{
					"projectItems": map[string]any{
						"nodes": []any{
							map[string]any{
								"id":             fmt.Sprintf("PVTI_item%d", issueNumber),
								"fullDatabaseId": fmt.Sprintf("%d", 1000+issueNumber),
								"project":        map[string]any{"id": t.projectID},
							},
						},
						"pageInfo": map[string]any{
							"hasNextPage": false, "hasPreviousPage": false,
							"startCursor": "page-one", "endCursor": "page-one",
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func (t *gatedIssueLookupTransport) snapshot() (active int, peak int, calls map[int]int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.active, t.peak, maps.Clone(t.calls)
}

func issueBatchItems(issueNumbers ...int) []parsedBatchItem {
	items := make([]parsedBatchItem, 0, len(issueNumbers))
	for index, issueNumber := range issueNumbers {
		items = append(items, parsedBatchItem{
			index:       index,
			refKind:     batchRefIssue,
			issueOwner:  "octo-org",
			issueRepo:   "roadmap",
			issueNumber: issueNumber,
		})
	}
	return items
}

func waitForIssueLookups(ctx context.Context, t *testing.T, started <-chan int, count int) {
	t.Helper()
	for range count {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %d issue lookups to start: %v", count, ctx.Err())
		}
	}
}

func waitForIssueLookupResults(ctx context.Context, t *testing.T, results <-chan map[issueRefKey]itemLookupResult) map[issueRefKey]itemLookupResult {
	t.Helper()
	select {
	case resolved := <-results:
		return resolved
	case <-ctx.Done():
		t.Fatalf("timed out waiting for issue lookups to finish: %v", ctx.Err())
		return nil
	}
}

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

func Test_UpdateProjectItemsBatch_InvalidSharedValueIsTopLevelError(t *testing.T) {
	queryTransport := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				statusFieldNode("PVTSSF_status", 101, "Status", []map[string]any{
					{"id": "OPT_todo", "name": "Todo"},
				}),
			})),
		),
	)
	transport := &mutationAwareTransport{
		t:       t,
		queries: queryTransport.Transport,
		mutationRespond: func(_ int, _ capturedGraphQLRequest) (int, string) {
			t.Fatal("invalid shared values must fail before writes")
			return http.StatusInternalServerError, ""
		},
	}

	result, structured, err := updateProjectItemsBatch(
		t.Context(),
		nil,
		newTestGQLClient(transport),
		"octo-org",
		"org",
		1,
		map[string]any{
			"updated_field": map[string]any{"name": "Status", "value": "Missing"},
			"items":         []any{map[string]any{"node_id": "PVTI_item1"}},
		},
	)
	require.NoError(t, err)
	assert.Nil(t, structured)
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getErrorResult(t, result).Text), &response))
	assert.Equal(t, "option_not_found", response["error"])
	assert.Equal(t, "Missing", response["name"])
	assert.Equal(t, []any{map[string]any{"name": "Todo"}}, response["candidates"])
	assert.Empty(t, transport.mutationCalls)
}

func Test_ProjectsWrite_UpdateProjectItems_NodeIDBypassesRESTLookup(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	queryTransport := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				fieldNode("PVTF_notes", 101, "Notes", "TEXT"),
			})),
		),
	)

	transport := &mutationAwareTransport{
		t:       t,
		queries: queryTransport.Transport,
		mutationRespond: func(_ int, req capturedGraphQLRequest) (int, string) {
			assert.Contains(t, req.Query, "updateProjectV2ItemFieldValue")
			return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
				0: {NodeID: "PVTI_item1", FullDatabaseID: "1001"},
			})
		},
	}
	gqlClient := newTestGQLClient(transport)

	// No REST handlers registered at all: if the implementation ever fell back
	// to a REST lookup for a node_id-addressed item, this would 404.
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"updated_field":  map[string]any{"name": "Notes", "value": "hello"},
		"items": []any{
			map[string]any{"node_id": "PVTI_item1"},
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, float64(1), response["succeeded"])
	assert.Equal(t, float64(0), response["failed"])
	assert.Equal(t, float64(0), response["unknown"])
}

func Test_ProjectsWrite_UpdateProjectItems_NumericItemIDDeduplicatesRESTLookup(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	queryTransport := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				fieldNode("PVTF_notes", 101, "Notes", "TEXT"),
			})),
		),
	)
	transport := &mutationAwareTransport{
		t:       t,
		queries: queryTransport.Transport,
		mutationRespond: func(_ int, req capturedGraphQLRequest) (int, string) {
			require.Len(t, req.Variables, 1)
			assert.Equal(t, "PVTF_notes", req.Variables["input"].(map[string]any)["fieldId"])
			return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
				0: {NodeID: "PVTI_item1001", FullDatabaseID: "1001"},
			})
		},
	}
	gqlClient := newTestGQLClient(transport)

	var restCalls int32
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetOrgsProjectsV2ItemsByProjectByItemID: func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&restCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1001,"node_id":"PVTI_item1001"}`))
		},
	}))

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"updated_field":  map[string]any{"name": "Notes", "value": "hello"},
		"items": []any{
			map[string]any{"item_id": float64(1001)},
			map[string]any{"item_id": float64(1001)},
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, float64(1), response["succeeded"])
	assert.Equal(t, float64(1), response["failed"])
	assert.Equal(t, int32(1), atomic.LoadInt32(&restCalls), "the same numeric item_id must only be resolved once")
	results := response["results"].([]any)
	assert.Equal(t, "duplicate_target", results[1].(map[string]any)["error"].(map[string]any)["code"])
}

func Test_ProjectsWrite_UpdateProjectItems_IssueRefPaginationIsDeduplicated(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	queryTransport := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
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
									"id":             "PVTI_other",
									"fullDatabaseId": "9999",
									"project":        map[string]any{"id": "PVT_other"},
								},
							},
							"pageInfo": map[string]any{
								"hasNextPage": true, "hasPreviousPage": false,
								"startCursor": "page-one", "endCursor": "page-one",
							},
						},
					},
				},
			}),
		),
		githubv4mock.NewQueryMatcher(
			resolveItemByIssuePageQuery{},
			map[string]any{
				"issueOwner":  githubv4.String("github"),
				"issueRepo":   githubv4.String("planning-tracking"),
				"issueNumber": githubv4.Int(123),
				"after":       githubv4.String("page-one"),
			},
			githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{
					"issue": map[string]any{
						"projectItems": map[string]any{
							"nodes": []any{
								map[string]any{
									"id":             "PVTI_item2002",
									"fullDatabaseId": "2002",
									"project":        map[string]any{"id": "PVT_project1"},
								},
							},
							"pageInfo": map[string]any{
								"hasNextPage": false, "hasPreviousPage": true,
								"startCursor": "page-two", "endCursor": "page-two",
							},
						},
					},
				},
			}),
		),
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				fieldNode("PVTF_notes", 101, "Notes", "TEXT"),
			})),
		),
	)
	transport := &mutationAwareTransport{
		t:       t,
		queries: queryTransport.Transport,
		mutationRespond: func(_ int, req capturedGraphQLRequest) (int, string) {
			require.Len(t, req.Variables, 1)
			assert.Equal(t, "PVTI_item2002", req.Variables["input"].(map[string]any)["itemId"])
			assert.Equal(t, "PVTF_notes", req.Variables["input"].(map[string]any)["fieldId"])
			return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
				0: {NodeID: "PVTI_item2002", FullDatabaseID: "2002"},
			})
		},
	}
	gqlClient := newTestGQLClient(transport)
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"updated_field":  map[string]any{"name": "Notes", "value": "hello"},
		"items": []any{
			map[string]any{
				"item_owner": "github", "item_repo": "planning-tracking", "issue_number": float64(123),
			},
			map[string]any{
				"item_owner": "github", "item_repo": "planning-tracking", "issue_number": float64(123),
			},
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, float64(1), response["succeeded"])
	assert.Equal(t, float64(1), response["failed"])
	results := response["results"].([]any)
	item := results[0].(map[string]any)["item"].(map[string]any)
	assert.Equal(t, "PVTI_item2002", item["node_id"])
	assert.Equal(t, "2002", item["full_database_id"])
	assert.Equal(t, "duplicate_target", results[1].(map[string]any)["error"].(map[string]any)["code"])
	issueResolutionCalls := 0
	for _, call := range transport.queryCalls {
		if strings.Contains(call.Query, "projectItems") {
			issueResolutionCalls++
		}
	}
	assert.Equal(t, 2, issueResolutionCalls, "duplicate issue refs should share one two-page resolution chain")
	assert.Len(t, transport.queryCalls, 4, "expected project, fields, and two issue-page queries")
}

func Test_ProjectsWrite_UpdateProjectItems_DuplicateTargetRejected(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	queryTransport := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				fieldNode("PVTF_notes", 101, "Notes", "TEXT"),
			})),
		),
	)
	transport := &mutationAwareTransport{
		t:       t,
		queries: queryTransport.Transport,
		mutationRespond: func(_ int, req capturedGraphQLRequest) (int, string) {
			require.Len(t, req.Variables, 1)
			assert.Equal(t, 1, strings.Count(req.Query, "updateProjectV2ItemFieldValue"))
			assert.Equal(t, "PVTI_item1", req.Variables["input"].(map[string]any)["itemId"])
			return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
				0: {NodeID: "PVTI_item1", FullDatabaseID: "1001"},
			})
		},
	}
	gqlClient := newTestGQLClient(transport)
	var restCalls int32
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetOrgsProjectsV2ItemsByProjectByItemID: func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&restCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":1001,"node_id":"PVTI_item1"}`))
		},
	}))

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"updated_field":  map[string]any{"name": "Notes", "value": "hello"},
		"items": []any{
			map[string]any{"node_id": "PVTI_item1"},
			map[string]any{"item_id": float64(1001)},
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, float64(1), response["succeeded"])
	assert.Equal(t, float64(1), response["failed"])

	results := response["results"].([]any)
	second := results[1].(map[string]any)
	assert.Equal(t, "failed", second["status"])
	assert.Equal(t, "duplicate_target", second["error"].(map[string]any)["code"])
	assert.Equal(t, int32(1), atomic.LoadInt32(&restCalls))
	assert.Len(t, transport.mutationCalls, 1)
}

func Test_ProjectsWrite_UpdateProjectItems_TwentyWritesIsOneMutationRequest(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)
	transport := chunkSizeTestRun(t, toolDef, 20)
	assert.Len(t, transport.mutationCalls, 1)
}

func Test_ProjectsWrite_UpdateProjectItems_TwentyOneWritesIsTwoMutationRequests(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)
	transport := chunkSizeTestRun(t, toolDef, 21)
	assert.Len(t, transport.mutationCalls, 2)
}

func Test_ProjectsWrite_UpdateProjectItems_MaximumWritesIsThreeMutationRequests(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)
	transport := chunkSizeTestRun(t, toolDef, maxProjectItemsPerBatch)
	assert.Len(t, transport.mutationCalls, 3)
}

// chunkSizeTestRun runs an update_project_items call with itemCount node_id
// items (all TEXT field updates), returning the mutationAwareTransport so the
// caller can assert on how many aliased-mutation HTTP requests were made.
func chunkSizeTestRun(t *testing.T, toolDef inventory.ServerTool, itemCount int) *mutationAwareTransport {
	t.Helper()

	queryTransport := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				fieldNode("PVTF_notes", 101, "Notes", "TEXT"),
			})),
		),
	)
	transport := &mutationAwareTransport{
		t:       t,
		queries: queryTransport.Transport,
		mutationRespond: func(_ int, req capturedGraphQLRequest) (int, string) {
			// input (index 0) plus inputN for each additional alias in this chunk.
			chunkSize := len(req.Variables)
			ids := make(map[int]struct{ NodeID, FullDatabaseID string }, chunkSize)
			for i := range chunkSize {
				ids[i] = struct{ NodeID, FullDatabaseID string }{
					NodeID:         "PVTI_chunk",
					FullDatabaseID: "1",
				}
			}
			return http.StatusOK, mutationDataResponse(t, ids)
		},
	}
	gqlClient := newTestGQLClient(transport)
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))

	items := make([]any, itemCount)
	for i := range itemCount {
		items[i] = map[string]any{"node_id": fmt.Sprintf("PVTI_item%d", i)}
	}

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"updated_field":  map[string]any{"name": "Notes", "value": "hello"},
		"items":          items,
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, float64(itemCount), response["succeeded"])

	return transport
}

func Test_ProjectsWrite_UpdateProjectItems_SharedNullClearsAllItemsInOrder(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	queryTransport := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				fieldNode("PVTF_notes", 101, "Notes", "TEXT"),
			})),
		),
	)
	transport := &mutationAwareTransport{
		t:       t,
		queries: queryTransport.Transport,
		mutationRespond: func(_ int, req capturedGraphQLRequest) (int, string) {
			assert.Contains(t, req.Query, "clearProjectV2ItemFieldValue")
			assert.NotContains(t, req.Query, "updateProjectV2ItemFieldValue")
			for _, input := range req.Variables {
				assert.NotContains(t, input.(map[string]any), "value")
			}
			return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
				0: {NodeID: "PVTI_item0", FullDatabaseID: "1000"},
				1: {NodeID: "PVTI_item1", FullDatabaseID: "1001"},
				2: {NodeID: "PVTI_item2", FullDatabaseID: "1002"},
			})
		},
	}
	gqlClient := newTestGQLClient(transport)
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"updated_field":  map[string]any{"name": "Notes", "value": nil},
		"items": []any{
			map[string]any{"node_id": "PVTI_item0"},
			map[string]any{"node_id": "PVTI_item1"},
			map[string]any{"node_id": "PVTI_item2"},
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, float64(3), response["succeeded"])

	results := response["results"].([]any)
	require.Len(t, results, 3)
	for i, r := range results {
		entry := r.(map[string]any)
		assert.Equal(t, float64(i), entry["index"])
		assert.Equal(t, "succeeded", entry["status"])
		assert.Equal(t, fmt.Sprintf("%d", 1000+i), entry["item"].(map[string]any)["full_database_id"])
	}
	assert.Len(t, transport.mutationCalls, 1)
}

func Test_ProjectsWrite_UpdateProjectItems_TransportFailureAbortsLaterChunks(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	queryTransport := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				fieldNode("PVTF_notes", 101, "Notes", "TEXT"),
			})),
		),
	)
	transport := &mutationAwareTransport{
		t:       t,
		queries: queryTransport.Transport,
		mutationRespond: func(callIndex int, _ capturedGraphQLRequest) (int, string) {
			if callIndex == 0 {
				// Systemic transport-level failure: no data at all.
				return http.StatusInternalServerError, `{"message":"internal server error"}`
			}
			t.Fatalf("chunk #%d must not execute after an ambiguous chunk-level failure", callIndex)
			return http.StatusInternalServerError, ""
		},
	}
	gqlClient := newTestGQLClient(transport)
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))

	items := make([]any, 25)
	for i := range 25 {
		items[i] = map[string]any{"node_id": fmt.Sprintf("PVTI_item%d", i)}
	}

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"updated_field":  map[string]any{"name": "Notes", "value": "x"},
		"items":          items,
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	// No item succeeded (all unknown after the abort), so IsError is set per
	// the "no item succeeded" rule, even though nothing was deterministically
	// rejected; the structured result (with unknown statuses) is still available.
	assert.True(t, result.IsError)

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, float64(0), response["succeeded"])
	assert.Equal(t, float64(25), response["unknown"])
	assert.Len(t, transport.mutationCalls, 1, "only the first (failing) chunk should have been sent")

	results := response["results"].([]any)
	for _, r := range results {
		assert.Equal(t, "unknown", r.(map[string]any)["status"])
	}
}

func Test_ProjectsWrite_UpdateProjectItems_AllFailedSetsIsError(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
	mocked := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
	)
	countingTransport := &requestCountingTransport{inner: mocked.Transport}
	gqlClient := newTestGQLClient(countingTransport)

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"updated_field":  map[string]any{"name": "Notes", "value": "x"},
		"items": []any{
			map[string]any{},
			map[string]any{"node_id": ""},
			map[string]any{"item_id": float64(0)},
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.True(t, result.IsError, "IsError must be set when no item in the batch succeeds")

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, float64(0), response["succeeded"])
	assert.Equal(t, float64(3), response["failed"])
	assert.Zero(t, countingTransport.count, "an all-invalid batch should not perform GraphQL resolution")
}

func Test_ProjectsWrite_UpdateProjectItems_MixedOutcomeKeepsIsErrorFalse(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	queryTransport := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				fieldNode("PVTF_notes", 101, "Notes", "TEXT"),
			})),
		),
	)
	transport := &mutationAwareTransport{
		t:       t,
		queries: queryTransport.Transport,
		mutationRespond: func(_ int, _ capturedGraphQLRequest) (int, string) {
			return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
				0: {NodeID: "PVTI_item0", FullDatabaseID: "1000"},
			})
		},
	}
	gqlClient := newTestGQLClient(transport)
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"updated_field":  map[string]any{"name": "Notes", "value": "x"},
		"items": []any{
			map[string]any{"node_id": "PVTI_item0"},
			map[string]any{}, // deterministic failure
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError, "mixed outcomes must keep IsError false so the structured result stays available")

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, float64(1), response["succeeded"])
	assert.Equal(t, float64(1), response["failed"])
}

// Test_ProjectsWrite_UpdateProjectItems_EnterpriseClientWiring verifies the
// batch mutation path works unchanged when gqlClient was constructed via
// githubv4.NewEnterpriseClient (GHES), not just githubv4.NewClient: the
// reflection-based mutation logic never assumes a specific endpoint and only
// ever uses the injected client.
func Test_ProjectsWrite_UpdateProjectItems_EnterpriseClientWiring(t *testing.T) {
	toolDef := ProjectsWrite(translations.NullTranslationHelper)

	queryTransport := githubv4mock.NewMockedHTTPClient(
		projectIDMatcher("octo-org", 1, "PVT_project1"),
		githubv4mock.NewQueryMatcher(
			projectFieldsWithIssueFieldsTestQuery{},
			fieldsQueryVars("octo-org", 1),
			githubv4mock.DataResponse(fieldsResponse([]map[string]any{
				fieldNode("PVTF_notes", 101, "Notes", "TEXT"),
			})),
		),
	)
	transport := &mutationAwareTransport{
		t:       t,
		queries: queryTransport.Transport,
		mutationRespond: func(_ int, _ capturedGraphQLRequest) (int, string) {
			return http.StatusOK, mutationDataResponse(t, map[int]struct{ NodeID, FullDatabaseID string }{
				0: {NodeID: "PVTI_item0", FullDatabaseID: "1000"},
			})
		},
	}
	gqlClient := githubv4.NewEnterpriseClient("https://ghe.example.com/graphql", &http.Client{Transport: transport})
	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))

	deps := BaseDeps{Client: restClient, GQLClient: gqlClient}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":         "update_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"updated_field":  map[string]any{"name": "Notes", "value": "x"},
		"items": []any{
			map[string]any{"node_id": "PVTI_item0"},
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)

	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, float64(1), response["succeeded"])
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

func Test_ConvertProjectFieldValue_SingleSelect_IDPrecedesName(t *testing.T) {
	field := &ResolvedField{
		Name:     "Status",
		DataType: "SINGLE_SELECT",
		Options: []ResolvedFieldOption{
			{ID: "OPT_other", Name: "OPT_target"},
			{ID: "OPT_target", Name: "Target"},
		},
	}

	v, err := convertProjectFieldValue(field, "OPT_target")
	require.NoError(t, err)
	require.NotNil(t, v.SingleSelectOptionID)
	assert.Equal(t, "OPT_target", string(*v.SingleSelectOptionID))
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
					projectFieldsWithIssueFieldsTestQuery{},
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
			projectFieldsWithIssueFieldsTestQuery{},
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

			resolved := resolveItemNodeIDsByNumericID(t.Context(), client, "octocat", tt.ownerType, 1, []int64{1001, 1001}, false)

			require.NoError(t, resolved[1001].err)
			assert.Equal(t, "PVTI_item1001", resolved[1001].nodeID)
			assert.Equal(t, 1, calls)
		})
	}
}

func Test_ResolveIssueRefs_DeduplicatesAndBoundsConcurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	gate := make(chan struct{})
	transport := newGatedIssueLookupTransport(gate, "PVT_project")
	results := make(chan map[issueRefKey]itemLookupResult, 1)
	go func() {
		results <- resolveIssueRefs(
			ctx,
			newTestGQLClient(transport),
			githubv4.ID("PVT_project"),
			issueBatchItems(1, 2, 3, 4, 5, 6, 1),
		)
	}()

	waitForIssueLookups(ctx, t, transport.started, batchItemLookupConcurrency)
	active, peak, calls := transport.snapshot()
	assert.Equal(t, batchItemLookupConcurrency, active)
	assert.Equal(t, batchItemLookupConcurrency, peak)
	assert.Len(t, calls, batchItemLookupConcurrency)

	close(gate)
	resolved := waitForIssueLookupResults(ctx, t, results)

	require.Len(t, resolved, 6)
	for issueNumber := 1; issueNumber <= 6; issueNumber++ {
		key := issueRefKey{owner: "octo-org", repo: "roadmap", number: issueNumber}
		result, ok := resolved[key]
		require.True(t, ok)
		require.NoError(t, result.err)
		assert.Equal(t, fmt.Sprintf("PVTI_item%d", issueNumber), result.nodeID)
		assert.Equal(t, int64(1000+issueNumber), result.fullDatabaseID)
	}

	active, peak, calls = transport.snapshot()
	assert.Zero(t, active)
	assert.Equal(t, batchItemLookupConcurrency, peak)
	require.Len(t, calls, 6)
	for issueNumber := 1; issueNumber <= 6; issueNumber++ {
		assert.Equal(t, 1, calls[issueNumber])
	}
}

func Test_ResolveIssueRefs_CancellationPopulatesWaitingRefs(t *testing.T) {
	testCtx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	ctx, cancel := context.WithCancel(testCtx)
	defer cancel()

	gate := make(chan struct{})
	defer close(gate)
	transport := newGatedIssueLookupTransport(gate, "PVT_project")
	results := make(chan map[issueRefKey]itemLookupResult, 1)
	go func() {
		results <- resolveIssueRefs(
			ctx,
			newTestGQLClient(transport),
			githubv4.ID("PVT_project"),
			issueBatchItems(1, 2, 3, 4, 5, 6, 7),
		)
	}()

	waitForIssueLookups(testCtx, t, transport.started, batchItemLookupConcurrency)
	_, peak, startedCalls := transport.snapshot()
	require.Equal(t, batchItemLookupConcurrency, peak)
	require.Len(t, startedCalls, batchItemLookupConcurrency)

	cancel()
	resolved := waitForIssueLookupResults(testCtx, t, results)

	require.Len(t, resolved, 7)
	waiting := 0
	for issueNumber := 1; issueNumber <= 7; issueNumber++ {
		key := issueRefKey{owner: "octo-org", repo: "roadmap", number: issueNumber}
		result, ok := resolved[key]
		require.True(t, ok)
		require.ErrorIs(t, result.err, context.Canceled)
		if _, started := startedCalls[issueNumber]; !started {
			waiting++
			assert.Equal(t, context.Canceled, result.err)
		}
	}
	assert.Equal(t, 2, waiting)

	active, peak, calls := transport.snapshot()
	assert.Zero(t, active)
	assert.Equal(t, batchItemLookupConcurrency, peak)
	assert.Equal(t, startedCalls, calls)
}

func Test_BatchErrorFromResolution(t *testing.T) {
	t.Run("generic wrapped error", func(t *testing.T) {
		err := batchErrorFromResolution(fmt.Errorf("item lookup failed: %w", context.DeadlineExceeded))

		assert.Equal(t, "resolution_failed", err.Code)
		assert.Equal(t, "item lookup failed: context deadline exceeded", err.Message)
	})

	t.Run("structured error", func(t *testing.T) {
		candidates := []any{map[string]any{"id": "PVTI_1"}}
		structured := ghErrors.NewStructuredResolutionError(
			"item_not_found",
			"octo/repo#42",
			"Check that the item belongs to the project.",
			candidates,
		)

		err := batchErrorFromResolution(fmt.Errorf("resolve item: %w", structured))

		assert.Equal(t, structured.Kind, err.Code)
		assert.Equal(t, "item_not_found: octo/repo#42", err.Message)
		assert.Equal(t, structured.Hint, err.Hint)
		assert.Equal(t, candidates, err.Candidates)
	})
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
