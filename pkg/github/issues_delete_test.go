package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/githubv4mock"
	gogithub "github.com/google/go-github/v92/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/require"
)

// newStaleFieldValuesGQLClient answers a read of issue 42's field values with
// a snapshot in which field 202 still holds a value another caller has since
// changed. An update that writes back what it read reverts that change.
func newStaleFieldValuesGQLClient() *githubv4.Client {
	return githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
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
			githubv4mock.DataResponse(map[string]any{
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
									"__typename": "IssueFieldDateValue",
									"field":      map[string]any{"fullDatabaseId": "202", "name": "Target date"},
									"value":      "2026-01-01",
								},
							},
						},
					},
				},
			}),
		),
	))
}

// Test_UpdateIssue_SetFieldDoesNotRewriteOtherFields: setting one field must
// send only that field, through the additive endpoint, never via the issue
// PATCH (which replaces the whole set).
func Test_UpdateIssue_SetFieldDoesNotRewriteOtherFields(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  new(42),
		Title:   new("Test issue"),
		State:   new("open"),
		HTMLURL: new("https://github.com/owner/repo/issues/42"),
	}

	var (
		mu         sync.Mutex
		patchBody  []byte
		fieldsBody []byte
	)

	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			patchBody = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockIssue)
		},
		PostReposIssuesIssueFieldValuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			fieldsBody = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		},
	}))

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		newStaleFieldValuesGQLClient(),
		"owner", "repo", 42,
		"", "", nil, nil, 0, "",
		[]*gogithub.IssueRequestFieldValue{{FieldID: 101, Value: "P2"}},
		nil,
		"", "", 0,
	)
	require.NoError(t, err)
	if result.IsError {
		t.Fatalf("expected non-error result, got: %s", getTextResult(t, result).Text)
	}

	mu.Lock()
	defer mu.Unlock()
	require.NotContains(t, string(patchBody), "issue_field_values",
		"the issue PATCH replaces the whole field set, so it must not carry field values (PATCH body was: %s)", string(patchBody))
	require.JSONEq(t, `{"issue_field_values":[{"field_id":101,"value":"P2"}]}`, string(fieldsBody),
		"only the requested field should be sent, through the additive issue-field-values endpoint")
}

// Test_UpdateIssue_DeleteFieldOnlyClearsThatField: clearing one field uses the
// per-field DELETE endpoint and leaves every other field alone. In particular
// it must not POST an empty list, which the endpoint treats as "clear all".
func Test_UpdateIssue_DeleteFieldOnlyClearsThatField(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  new(42),
		Title:   new("Test issue"),
		State:   new("open"),
		HTMLURL: new("https://github.com/owner/repo/issues/42"),
	}

	var (
		mu          sync.Mutex
		patchBody   []byte
		postCalls   int
		deletePaths []string
	)

	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			patchBody = body
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(mockIssue)
		},
		PostReposIssuesIssueFieldValuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			postCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		},
		DeleteReposIssuesIssueFieldValueByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			deletePaths = append(deletePaths, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		},
	}))

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		newStaleFieldValuesGQLClient(),
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
	require.NotContains(t, string(patchBody), "issue_field_values",
		"the issue PATCH must not carry field values when clearing a field (PATCH body was: %s)", string(patchBody))
	require.Zero(t, postCalls, "clearing a field must not call the add endpoint")
	require.Equal(t, []string{"/repos/owner/repo/issues/42/issue-field-values/101"}, deletePaths,
		"expected exactly one DELETE call to the dedicated endpoint for field id 101")
}

// Test_UpdateIssue_DeleteAbsentFieldIsNoOp: the DELETE endpoint returns 404
// when the field has no value. That must not be reported as a failure, so
// idempotent delete:true callers don't break on retry.
func Test_UpdateIssue_DeleteAbsentFieldIsNoOp(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  new(42),
		Title:   new("Test issue"),
		State:   new("open"),
		HTMLURL: new("https://github.com/owner/repo/issues/42"),
	}

	var (
		mu          sync.Mutex
		deletePaths []string
	)

	restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssue),
		DeleteReposIssuesIssueFieldValueByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			deletePaths = append(deletePaths, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		},
	}))

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
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
	require.Equal(t, []string{"/repos/owner/repo/issues/42/issue-field-values/101"}, deletePaths)
}

// Test_UpdateIssue_DeleteContinuesOnPartialFailure: a failing DELETE
// must not short-circuit subsequent ones, and the error must name which IDs
// succeeded and which failed so callers can retry the right ones.
func Test_UpdateIssue_DeleteContinuesOnPartialFailure(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  new(42),
		Title:   new("Test issue"),
		State:   new("open"),
		HTMLURL: new("https://github.com/owner/repo/issues/42"),
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

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
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

// Test_UpdateIssue_FieldWriteFailuresReportWhatWasApplied: field values are
// written before the issue PATCH, so a rejected field value leaves the issue
// untouched, and every failure says exactly which writes were applied.
func Test_UpdateIssue_FieldWriteFailuresReportWhatWasApplied(t *testing.T) {
	t.Parallel()

	mockIssue := &gogithub.Issue{
		Number:  new(42),
		Title:   new("Test issue"),
		State:   new("open"),
		HTMLURL: new("https://github.com/owner/repo/issues/42"),
	}
	failWith := func(status int) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"simulated failure"}`))
		}
	}

	tests := []struct {
		name            string
		values          []*gogithub.IssueRequestFieldValue
		fieldIDsToClear []int64
		postStatus      int
		deleteStatus    int
		patchStatus     int
		wantCalls       []string
		wantErrContains string
		wantErrOmits    string
	}{
		{
			name:            "rejected field value changes nothing",
			values:          []*gogithub.IssueRequestFieldValue{{FieldID: 202, Value: "not-a-date"}},
			postStatus:      http.StatusUnprocessableEntity,
			wantCalls:       []string{"POST"},
			wantErrContains: "failed to set issue field values; the issue was not updated",
		},
		{
			name:            "server error on the field write does not claim nothing changed",
			values:          []*gogithub.IssueRequestFieldValue{{FieldID: 101, Value: "P2"}},
			postStatus:      http.StatusBadGateway,
			wantCalls:       []string{"POST"},
			wantErrContains: "failed to set issue field values, which may have been applied anyway; the rest of the issue update was not attempted",
			wantErrOmits:    "the issue was not updated",
		},
		{
			name:            "failed clear names the set and cleared fields and skips the PATCH",
			values:          []*gogithub.IssueRequestFieldValue{{FieldID: 101, Value: "P2"}},
			fieldIDsToClear: []int64{202},
			postStatus:      http.StatusOK,
			deleteStatus:    http.StatusInternalServerError,
			wantCalls:       []string{"POST", "DELETE"},
			wantErrContains: "failed to clear issue field values: failed=[202], cleared=[], set=[101]; the rest of the issue update was not applied",
		},
		{
			name:            "failed PATCH after field writes says the fields were updated",
			values:          []*gogithub.IssueRequestFieldValue{{FieldID: 101, Value: "P2"}},
			postStatus:      http.StatusOK,
			patchStatus:     http.StatusUnprocessableEntity,
			wantCalls:       []string{"POST", "PATCH"},
			wantErrContains: "issue field values updated but the rest of the issue update failed",
		},
		{
			name:            "failed PATCH after a confirmed clear says the fields were updated",
			fieldIDsToClear: []int64{101},
			deleteStatus:    http.StatusNoContent,
			patchStatus:     http.StatusUnprocessableEntity,
			wantCalls:       []string{"DELETE", "PATCH"},
			wantErrContains: "issue field values updated but the rest of the issue update failed",
		},
		{
			name:            "failed PATCH after clearing only absent fields does not claim a field write",
			fieldIDsToClear: []int64{101, 202},
			deleteStatus:    http.StatusNotFound,
			patchStatus:     http.StatusUnprocessableEntity,
			wantCalls:       []string{"DELETE", "DELETE", "PATCH"},
			wantErrContains: "failed to update issue",
			wantErrOmits:    "issue field values updated",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var (
				mu    sync.Mutex
				calls []string
			)
			record := func(method string, next http.HandlerFunc) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					calls = append(calls, method)
					mu.Unlock()
					next(w, r)
				}
			}
			respond := func(status int, body any) http.HandlerFunc {
				if status >= http.StatusBadRequest {
					return failWith(status)
				}
				return mockResponse(t, status, body)
			}

			restClient := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesIssueFieldValuesByOwnerByRepoByIssueNumber:  record("POST", respond(tc.postStatus, []any{})),
				DeleteReposIssuesIssueFieldValueByOwnerByRepoByIssueNumber: record("DELETE", respond(tc.deleteStatus, "")),
				PatchReposIssuesByOwnerByRepoByIssueNumber:                 record("PATCH", respond(max(tc.patchStatus, http.StatusOK), mockIssue)),
			}))

			result, err := UpdateIssue(
				context.Background(),
				restClient,
				githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
				"owner", "repo", 42,
				"", "new body", nil, nil, 0, "",
				tc.values,
				tc.fieldIDsToClear,
				"", "", 0,
			)
			require.NoError(t, err)
			require.True(t, result.IsError, "expected an error result")
			require.Contains(t, getTextResult(t, result).Text, tc.wantErrContains)
			if tc.wantErrOmits != "" {
				require.NotContains(t, getTextResult(t, result).Text, tc.wantErrOmits)
			}

			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, tc.wantCalls, calls)
		})
	}
}

// failingPostTransport fails every POST without a response, as a dropped
// connection does, and passes other requests on.
type failingPostTransport struct {
	next  http.RoundTripper
	mu    sync.Mutex
	calls []string
}

func (f *failingPostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req.Method)
	f.mu.Unlock()
	if req.Method == http.MethodPost {
		return nil, errors.New("connection reset by peer")
	}
	return f.next.RoundTrip(req)
}

// Test_UpdateIssue_LostFieldWriteResponseDoesNotClaimNothingChanged: when the
// field-values POST gets no response, GitHub may have applied it. The error
// must not say the issue is unchanged, and the PATCH must not be sent.
func Test_UpdateIssue_LostFieldWriteResponseDoesNotClaimNothingChanged(t *testing.T) {
	t.Parallel()

	transport := &failingPostTransport{next: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, &gogithub.Issue{Number: new(42)}),
	}).Transport}
	restClient := mustNewGHClient(t, &http.Client{Transport: transport})

	result, err := UpdateIssue(
		context.Background(),
		restClient,
		githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
		"owner", "repo", 42,
		"", "new body", nil, nil, 0, "",
		[]*gogithub.IssueRequestFieldValue{{FieldID: 101, Value: "P2"}},
		nil,
		"", "", 0,
	)
	require.NoError(t, err)
	require.True(t, result.IsError, "expected an error result")

	resultText := getTextResult(t, result).Text
	require.Contains(t, resultText, "failed to set issue field values, which may have been applied anyway; the rest of the issue update was not attempted")
	require.NotContains(t, resultText, "the issue was not updated")

	transport.mu.Lock()
	defer transport.mu.Unlock()
	require.Equal(t, []string{"POST"}, transport.calls, "the PATCH must not be sent after a failed field write")
}
