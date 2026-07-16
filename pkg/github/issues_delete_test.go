package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	gogithub "github.com/google/go-github/v89/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_IssueRequest_EmptyFieldValues_OmittedByJSON pins the omitempty
// behaviour that makes the DELETE fallback necessary. If go-github ever drops
// the tag, the REST PATCH alone could clear field values and this test would
// fail to remind us.
func Test_IssueRequest_EmptyFieldValues_OmittedByJSON(t *testing.T) {
	t.Parallel()

	req := &gogithub.IssueRequest{
		Title:            gogithub.Ptr("still here"),
		IssueFieldValues: []*gogithub.IssueRequestFieldValue{},
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	assert.NotContains(t, string(body), "issue_field_values",
		"empty IssueFieldValues should be dropped by omitempty — this is why the REST PATCH alone can't clear field values when the merged list ends up empty, and why we fall back to the dedicated DELETE endpoint")
	assert.Contains(t, string(body), `"title":"still here"`,
		"sanity check: other fields still serialise")
}

// Test_UpdateIssue_DeleteLastFieldValueCallsDeleteEndpoint covers the bug fix:
// when the kept set ends up empty, the PATCH alone can't clear the field
// (omitempty strips the empty slice), so UpdateIssue follows up with a DELETE
// to the dedicated endpoint.
func Test_UpdateIssue_DeleteLastFieldValueCallsDeleteEndpoint(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  gogithub.Ptr(42),
		Title:   gogithub.Ptr("Test issue"),
		State:   gogithub.Ptr("open"),
		HTMLURL: gogithub.Ptr("https://github.com/owner/repo/issues/42"),
	}

	var (
		mu                sync.Mutex
		capturedPatchBody []byte
		deletePaths       []string
	)

	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			capturedPatchBody = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockIssue)
		},
		DeleteReposIssuesIssueFieldValueByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			deletePaths = append(deletePaths, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		},
	}))

	// Existing field values for the merge step. Returning the field we're
	// about to delete makes the kept list empty, triggering the fallback DELETE.
	existingFieldsResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issue": map[string]any{
				"issueFieldValues": map[string]any{
					"nodes": []any{
						map[string]any{
							"__typename": "IssueFieldSingleSelectValue",
							"field": map[string]any{
								"fullDatabaseId": "101",
								"name":           "Priority",
							},
							"value": "P1",
						},
					},
				},
			},
		},
	})

	gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					Issue struct {
						IssueFieldValues struct {
							Nodes []IssueFieldValueFragment
						} `graphql:"issueFieldValues(first: 25)"`
					} `graphql:"issue(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner":  githubv4.String("owner"),
				"repo":   githubv4.String("repo"),
				"number": githubv4.Int(42),
			},
			existingFieldsResponse,
		),
	))

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		gqlClient,
		"owner", "repo", 42,
		"", "", nil, nil, 0, "",
		nil,
		[]int64{101},
		"", "", 0,
	)
	require.NoError(t, err)
	if result.IsError {
		t.Fatalf("expected non-error result, got: %s", getTextResult(t, result).Text)
	}

	mu.Lock()
	defer mu.Unlock()
	require.NotContains(t, string(capturedPatchBody), "issue_field_values",
		"REST PATCH body must not carry issue_field_values when the kept set is empty (PATCH body was: %s)", string(capturedPatchBody))
	require.Equal(t, []string{"/repos/owner/repo/issues/42/issue-field-values/101"}, deletePaths,
		"expected exactly one DELETE call to the dedicated endpoint for field id 101")
}

// Test_UpdateIssue_DeleteOneOfManyUsesSetSemantics: when the kept set is
// non-empty, set semantics handle the deletion implicitly via the PATCH — no
// DELETE follow-up needed.
func Test_UpdateIssue_DeleteOneOfManyUsesSetSemantics(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  gogithub.Ptr(42),
		Title:   gogithub.Ptr("Test issue"),
		State:   gogithub.Ptr("open"),
		HTMLURL: gogithub.Ptr("https://github.com/owner/repo/issues/42"),
	}

	var (
		mu          sync.Mutex
		deletePaths []string
	)

	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
			"issue_field_values": []any{
				map[string]any{"field_id": float64(202), "value": "High"},
			},
		}).andThen(
			mockResponse(t, http.StatusOK, mockIssue),
		),
		DeleteReposIssuesIssueFieldValueByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			deletePaths = append(deletePaths, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		},
	}))

	existingFieldsResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issue": map[string]any{
				"issueFieldValues": map[string]any{
					"nodes": []any{
						map[string]any{
							"__typename": "IssueFieldSingleSelectValue",
							"field":      map[string]any{"fullDatabaseId": "101", "name": "Priority"},
							"value":      "P1",
						},
						map[string]any{
							"__typename": "IssueFieldSingleSelectValue",
							"field":      map[string]any{"fullDatabaseId": "202", "name": "Impact"},
							"value":      "High",
						},
					},
				},
			},
		},
	})

	gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					Issue struct {
						IssueFieldValues struct {
							Nodes []IssueFieldValueFragment
						} `graphql:"issueFieldValues(first: 25)"`
					} `graphql:"issue(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner":  githubv4.String("owner"),
				"repo":   githubv4.String("repo"),
				"number": githubv4.Int(42),
			},
			existingFieldsResponse,
		),
	))

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		gqlClient,
		"owner", "repo", 42,
		"", "", nil, nil, 0, "",
		nil,
		[]int64{101},
		"", "", 0,
	)
	require.NoError(t, err)
	if result.IsError {
		t.Fatalf("expected non-error result, got: %s", getTextResult(t, result).Text)
	}

	mu.Lock()
	defer mu.Unlock()
	require.Empty(t, deletePaths,
		"no DELETE call should fire when the kept set is non-empty — the PATCH's set semantics clear the deleted field on the server side")
}

// Test_UpdateIssue_DeleteAbsentFieldIsNoOp: deleting a field that isn't set
// must not fire a DELETE (the endpoint would 404), preserving the pre-fix
// silent-no-op behaviour so idempotent delete:true callers don't break on
// retry.
func Test_UpdateIssue_DeleteAbsentFieldIsNoOp(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  gogithub.Ptr(42),
		Title:   gogithub.Ptr("Test issue"),
		State:   gogithub.Ptr("open"),
		HTMLURL: gogithub.Ptr("https://github.com/owner/repo/issues/42"),
	}

	var (
		mu                sync.Mutex
		capturedPatchBody []byte
		deletePaths       []string
	)

	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			capturedPatchBody = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockIssue)
		},
		DeleteReposIssuesIssueFieldValueByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			deletePaths = append(deletePaths, r.URL.Path)
			mu.Unlock()
			// Fail loudly: if we get here, the fix is wrong.
			w.WriteHeader(http.StatusNotFound)
		},
	}))

	// Issue has no field values at all.
	existingFieldsResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issue": map[string]any{
				"issueFieldValues": map[string]any{
					"nodes": []any{},
				},
			},
		},
	})

	gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					Issue struct {
						IssueFieldValues struct {
							Nodes []IssueFieldValueFragment
						} `graphql:"issueFieldValues(first: 25)"`
					} `graphql:"issue(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner":  githubv4.String("owner"),
				"repo":   githubv4.String("repo"),
				"number": githubv4.Int(42),
			},
			existingFieldsResponse,
		),
	))

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		gqlClient,
		"owner", "repo", 42,
		"", "", nil, nil, 0, "",
		nil,
		[]int64{101}, // ask to delete a field that isn't set
		"", "", 0,
	)
	require.NoError(t, err)
	if result.IsError {
		t.Fatalf("expected non-error result, got: %s", getTextResult(t, result).Text)
	}

	mu.Lock()
	defer mu.Unlock()
	require.NotContains(t, string(capturedPatchBody), "issue_field_values",
		"PATCH body must not carry issue_field_values when nothing changed")
	require.Empty(t, deletePaths,
		"no DELETE call should fire for a field that isn't present on the issue — preserves the pre-fix silent-no-op behaviour and avoids a guaranteed 404")
}

// Test_UpdateIssue_DeleteFallbackContinuesOnPartialFailure: a failing DELETE
// must not short-circuit subsequent ones, and the error must name which IDs
// succeeded and which failed so callers can retry the right ones.
func Test_UpdateIssue_DeleteFallbackContinuesOnPartialFailure(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  gogithub.Ptr(42),
		Title:   gogithub.Ptr("Test issue"),
		State:   gogithub.Ptr("open"),
		HTMLURL: gogithub.Ptr("https://github.com/owner/repo/issues/42"),
	}

	var (
		mu          sync.Mutex
		deletePaths []string
	)

	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockIssue)
		},
		DeleteReposIssuesIssueFieldValueByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			deletePaths = append(deletePaths, r.URL.Path)
			mu.Unlock()
			// Field 202 fails; 101 and 303 succeed. All three should fire and
			// the error must name 202 as failed and 101/303 as cleared.
			if strings.HasSuffix(r.URL.Path, "/202") {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"message":"simulated failure"}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		},
	}))

	existingFieldsResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issue": map[string]any{
				"issueFieldValues": map[string]any{
					"nodes": []any{
						map[string]any{
							"__typename": "IssueFieldSingleSelectValue",
							"field":      map[string]any{"fullDatabaseId": "101", "name": "Priority"},
							"value":      "P1",
						},
						map[string]any{
							"__typename": "IssueFieldSingleSelectValue",
							"field":      map[string]any{"fullDatabaseId": "202", "name": "Visibility"},
							"value":      "High",
						},
						map[string]any{
							"__typename": "IssueFieldSingleSelectValue",
							"field":      map[string]any{"fullDatabaseId": "303", "name": "Impact"},
							"value":      "Critical",
						},
					},
				},
			},
		},
	})

	gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct {
					Issue struct {
						IssueFieldValues struct {
							Nodes []IssueFieldValueFragment
						} `graphql:"issueFieldValues(first: 25)"`
					} `graphql:"issue(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}{},
			map[string]any{
				"owner":  githubv4.String("owner"),
				"repo":   githubv4.String("repo"),
				"number": githubv4.Int(42),
			},
			existingFieldsResponse,
		),
	))

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		gqlClient,
		"owner", "repo", 42,
		"", "", nil, nil, 0, "",
		nil,
		[]int64{101, 202, 303},
		"", "", 0,
	)
	require.NoError(t, err)
	require.True(t, result.IsError, "expected an error result because field 202 failed")

	mu.Lock()
	defer mu.Unlock()
	// All three DELETEs must have fired — the middle failure must not short-circuit the third.
	require.Len(t, deletePaths, 3,
		"all three DELETE calls should fire even though one fails; got paths: %v", deletePaths)

	resultText := getTextResult(t, result).Text
	require.Contains(t, resultText, "failed=[202]",
		"error must name the failed field ID so the caller can retry it; got: %s", resultText)
	require.Contains(t, resultText, "cleared=[101 303]",
		"error must name the cleared field IDs so the caller knows what's already done; got: %s", resultText)
}
