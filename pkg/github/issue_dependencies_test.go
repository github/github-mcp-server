package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	endpointBlockedBy = EndpointPattern("GET /repos/{owner}/{repo}/issues/{issue_number}/dependencies/blocked_by")
	endpointBlocking  = EndpointPattern("GET /repos/{owner}/{repo}/issues/{issue_number}/dependencies/blocking")
	endpointAddBlock  = EndpointPattern("POST /repos/{owner}/{repo}/issues/{issue_number}/dependencies/blocked_by")
	endpointRemoveBlk = EndpointPattern("DELETE /repos/{owner}/{repo}/issues/{issue_number}/dependencies/blocked_by/{issue_id}")
	endpointGetIssue  = EndpointPattern("GET /repos/{owner}/{repo}/issues/{issue_number}")
)

// jsonHandler writes the given status code and JSON-encoded body.
func jsonHandler(status int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(MustMarshal(body))
	}
}

func Test_IssueDependencyRead(t *testing.T) {
	// Verify tool definition once (flag-gated variant snap)
	serverTool := IssueDependencyRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name+"_ff_"+FeatureFlagIssueDependencies, tool))
	require.Equal(t, FeatureFlagIssueDependencies, serverTool.FeatureFlagEnable)

	assert.Equal(t, "issue_dependency_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "issue_number")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "issue_number"})

	blockedByIssues := []map[string]any{
		{
			"number":         7,
			"title":          "Blocker",
			"state":          "open",
			"html_url":       "https://github.com/owner/repo/issues/7",
			"repository_url": "https://api.github.com/repos/owner/repo",
		},
	}
	blockingIssues := []map[string]any{
		{
			"number":         8,
			"title":          "Blocked A",
			"state":          "open",
			"html_url":       "https://github.com/owner/repo/issues/8",
			"repository_url": "https://api.github.com/repos/owner/repo",
		},
		{
			"number":         9,
			"title":          "Blocked B",
			"state":          "closed",
			"html_url":       "https://github.com/owner/repo/issues/9",
			"repository_url": "https://api.github.com/repos/owner/repo",
		},
	}

	// A handler that also advertises a next page via the Link header.
	blockingHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/issues/123/dependencies/blocking?page=2>; rel="next"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(MustMarshal(blockingIssues))
	}

	tests := []struct {
		name          string
		method        string
		option        MockBackendOption
		expectedCount int
		expectedFirst int
		expectedNext  bool
	}{
		{
			name:          "get_blocked_by returns blockers",
			method:        "get_blocked_by",
			option:        WithRequestMatch(endpointBlockedBy, blockedByIssues),
			expectedCount: 1,
			expectedFirst: 7,
			expectedNext:  false,
		},
		{
			name:          "get_blocking returns blocked issues",
			method:        "get_blocking",
			option:        WithRequestMatchHandler(endpointBlocking, blockingHandler),
			expectedCount: 2,
			expectedFirst: 8,
			expectedNext:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, NewMockedHTTPClient(tc.option))
			deps := BaseDeps{Client: client}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"method":       tc.method,
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "expected result to not be an error")

			text := getTextResult(t, result)
			var payload struct {
				Issues   []MinimalIssueRef `json:"issues"`
				PageInfo struct {
					HasNextPage bool `json:"hasNextPage"`
					NextPage    int  `json:"nextPage"`
				} `json:"pageInfo"`
			}
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload))
			require.Len(t, payload.Issues, tc.expectedCount)
			assert.Equal(t, tc.expectedFirst, payload.Issues[0].Number)
			assert.Equal(t, "owner/repo", payload.Issues[0].Repository)
			assert.Equal(t, tc.expectedNext, payload.PageInfo.HasNextPage)
		})
	}
}

func Test_IssueDependencyRead_Errors(t *testing.T) {
	serverTool := IssueDependencyRead(translations.NullTranslationHelper)

	t.Run("missing required param", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method": "get_blocked_by",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		getErrorResult(t, result)
	})

	t.Run("API error is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(endpointBlockedBy, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message": "Not Found"}`))
			})),
		))
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"method":       "get_blocked_by",
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(123),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		getErrorResult(t, result)
	})
}

func Test_IssueDependencyWrite(t *testing.T) {
	// Verify tool definition once (flag-gated variant snap)
	serverTool := IssueDependencyWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name+"_ff_"+FeatureFlagIssueDependencies, tool))
	require.Equal(t, FeatureFlagIssueDependencies, serverTool.FeatureFlagEnable)

	assert.Equal(t, "issue_dependency_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "type")
	assert.Contains(t, schema.Properties, "issue_number")
	assert.Contains(t, schema.Properties, "related_issue_number")
	assert.ElementsMatch(t, schema.Required, []string{"method", "type", "owner", "repo", "issue_number", "related_issue_number"})

	// issue returned by the blocking-issue resolve GET; its id is what the
	// dependency endpoints operate on.
	resolvedIssue := func(number, id int) map[string]any {
		return map[string]any{
			"id":             id,
			"number":         number,
			"title":          "Resolved",
			"state":          "open",
			"html_url":       "https://github.com/owner/repo/issues/" + strconv.Itoa(number),
			"repository_url": "https://api.github.com/repos/owner/repo",
		}
	}
	// issue returned by the add/remove endpoints (the blocked issue).
	blockedIssue := func(number int) map[string]any {
		return map[string]any{
			"number":         number,
			"title":          "Blocked",
			"state":          "open",
			"html_url":       "https://github.com/owner/repo/issues/" + strconv.Itoa(number),
			"repository_url": "https://api.github.com/repos/owner/repo",
		}
	}

	tests := []struct {
		name            string
		method          string
		relationship    string
		options         []MockBackendOption
		expectedMessage string
		expectedBlocked int
		expectedBlockng int
	}{
		{
			name:         "add blocked_by uses subject as blocked",
			method:       "add",
			relationship: "blocked_by",
			// subject(1) is blocked by related(2): resolve related(2), block issue 1.
			options: []MockBackendOption{
				WithRequestMatch(endpointGetIssue, resolvedIssue(2, 1002)),
				WithRequestMatchHandler(endpointAddBlock, jsonHandler(http.StatusCreated, blockedIssue(1))),
			},
			expectedMessage: "dependency added",
			expectedBlocked: 1,
			expectedBlockng: 2,
		},
		{
			name:         "add blocking swaps roles",
			method:       "add",
			relationship: "blocking",
			// subject(1) blocks related(2): resolve subject(1), block issue 2.
			options: []MockBackendOption{
				WithRequestMatch(endpointGetIssue, resolvedIssue(1, 1001)),
				WithRequestMatchHandler(endpointAddBlock, jsonHandler(http.StatusCreated, blockedIssue(2))),
			},
			expectedMessage: "dependency added",
			expectedBlocked: 2,
			expectedBlockng: 1,
		},
		{
			name:         "remove blocked_by",
			method:       "remove",
			relationship: "blocked_by",
			options: []MockBackendOption{
				WithRequestMatch(endpointGetIssue, resolvedIssue(2, 1002)),
				WithRequestMatch(endpointRemoveBlk, blockedIssue(1)),
			},
			expectedMessage: "dependency removed",
			expectedBlocked: 1,
			expectedBlockng: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, NewMockedHTTPClient(tc.options...))
			deps := BaseDeps{Client: client}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"method":               tc.method,
				"type":                 tc.relationship,
				"owner":                "owner",
				"repo":                 "repo",
				"issue_number":         float64(1),
				"related_issue_number": float64(2),
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "expected result to not be an error")

			text := getTextResult(t, result)
			var payload struct {
				Message       string          `json:"message"`
				BlockedIssue  MinimalIssueRef `json:"blocked_issue"`
				BlockingIssue MinimalIssueRef `json:"blocking_issue"`
			}
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload))
			assert.Equal(t, tc.expectedMessage, payload.Message)
			assert.Equal(t, tc.expectedBlocked, payload.BlockedIssue.Number)
			assert.Equal(t, tc.expectedBlockng, payload.BlockingIssue.Number)
		})
	}

	t.Run("self dependency fails before any API call", func(t *testing.T) {
		// Register no handlers: the handler must return before resolving or mutating.
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":               "add",
			"type":                 "blocked_by",
			"owner":                "owner",
			"repo":                 "repo",
			"issue_number":         float64(1),
			"related_issue_number": float64(1),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError, "expected result to be an error")

		text := getTextResult(t, result)
		assert.Contains(t, text.Text, "itself")
	})
}

func Test_IssueDependencyWrite_Validation(t *testing.T) {
	serverTool := IssueDependencyWrite(translations.NullTranslationHelper)

	cases := []struct {
		name string
		args map[string]any
	}{
		{
			name: "unknown type",
			args: map[string]any{
				"method":               "add",
				"type":                 "related_to",
				"owner":                "owner",
				"repo":                 "repo",
				"issue_number":         float64(1),
				"related_issue_number": float64(2),
			},
		},
		{
			name: "missing related_issue_number",
			args: map[string]any{
				"method":       "add",
				"type":         "blocked_by",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, NewMockedHTTPClient())
			deps := BaseDeps{Client: client}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			getErrorResult(t, result)
		})
	}
}
