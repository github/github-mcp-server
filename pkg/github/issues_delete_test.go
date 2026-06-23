package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	gogithub "github.com/google/go-github/v87/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_IssueRequest_EmptyFieldValues_OmittedByJSON pins the omitempty behaviour
// that motivates the DELETE-endpoint fallback in UpdateIssue. If go-github's
// IssueRequest ever drops the `omitempty` tag from `issue_field_values`, this
// test will fail — at which point the fallback could potentially be revisited.
// Until then, an empty `[]*IssueRequestFieldValue{}` is serialised as nothing
// at all, so the REST PATCH alone can never clear a field's last value via
// the set-semantics path.
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

// Test_UpdateIssue_DeleteLastFieldValueCallsDeleteEndpoint: regression test for
// the delete:true bug. When the kept set after merge + filter ends up empty
// (e.g. deleting the only remaining field value), the PATCH alone cannot carry
// the deletion intent because go-github strips the empty issue_field_values
// slice via omitempty. UpdateIssue follows up with a per-field DELETE to the
// dedicated `/repos/{owner}/{repo}/issues/{number}/issue-field-values/{id}`
// endpoint.
//
// Asserts both halves:
//
//   - the PATCH body does NOT carry an `issue_field_values` key (we don't want
//     to double-clear or rely on a value omitempty is about to strip)
//   - a DELETE for the field ID fires after the PATCH
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

	// Existing field values for the merge step. Returning only the field about
	// to be deleted is the worst case: the kept list ends up empty and the
	// fallback DELETE is the only thing that can clear it.
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

// Test_UpdateIssue_DeleteOneOfManyUsesSetSemantics verifies that when the kept
// set after merge + filter is non-empty (deleting 1 of N existing fields), the
// PATCH carries the kept fields and the dotcom REST handler's set semantics do
// the deletion implicitly — no fallback DELETE call is needed.
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
