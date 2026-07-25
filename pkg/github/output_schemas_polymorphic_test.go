package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	gogithub "github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

// polymorphicSchemas is the full set declared by this package, keyed by tool.
func polymorphicSchemas() map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"actions_get":              actionsGetOutputSchema,
		"actions_list":             actionsListOutputSchema,
		"actions_run_trigger":      actionsRunTriggerOutputSchema,
		"discussion_comment_write": discussionCommentWriteOutputSchema,
		"issue_dependency_read":    issueDependencyReadOutputSchema,
		"issue_read":               issueReadOutputSchema,
		"pull_request_read":        pullRequestReadOutputSchema,
	}
}

// oneOf requires EXACTLY ONE branch to match. These unions have structurally
// identical branches, empty collections that satisfy several array branches at
// once, and optional fields that overlap between branches — so oneOf would
// reject valid output. Guard against a well-meaning future "tightening".
func TestPolymorphicOutputSchemasNeverUseOneOf(t *testing.T) {
	for tool, schema := range polymorphicSchemas() {
		t.Run(tool, func(t *testing.T) {
			assert.NotContains(t, string(schema), `"oneOf"`,
				"use anyOf; oneOf requires exactly one branch to match and breaks on ambiguous payloads")
		})
	}
}

func TestPolymorphicOutputSchemasResolve(t *testing.T) {
	for tool, schema := range polymorphicSchemas() {
		t.Run(tool, func(t *testing.T) {
			var s jsonschema.Schema
			require.NoError(t, json.Unmarshal(schema, &s))
			_, err := s.Resolve(nil)
			require.NoError(t, err, "schema must resolve; a dangling $ref would fail at request time")
		})
	}
}

// Pins which schemas the protocol-version gate will withhold from clients
// older than 2026-07-28. A root of {"type":"object"} is representable under
// 2025-11-25 and ships to everyone; anything else is gated.
func TestPolymorphicOutputSchemaRootKinds(t *testing.T) {
	wantObjectRoot := map[string]bool{
		"actions_get":              true,
		"actions_list":             true,
		"actions_run_trigger":      true,
		"discussion_comment_write": true,
		"issue_dependency_read":    true,
		// Bare anyOf spanning objects and arrays — not representable before
		// 2026-07-28, so these two are the gated ones.
		"issue_read":        false,
		"pull_request_read": false,
	}
	for tool, schema := range polymorphicSchemas() {
		t.Run(tool, func(t *testing.T) {
			assert.Equal(t, wantObjectRoot[tool], inventory.HasObjectRootOutputSchema(schema))
		})
	}
}

// Every schema must reject values that match no branch. A union that accepts
// anything documents nothing.
func TestPolymorphicOutputSchemasRejectGarbage(t *testing.T) {
	for tool, schema := range polymorphicSchemas() {
		t.Run(tool, func(t *testing.T) {
			var s jsonschema.Schema
			require.NoError(t, json.Unmarshal(schema, &s))
			resolved, err := s.Resolve(nil)
			require.NoError(t, err)
			assert.Error(t, resolved.Validate("a bare string"))
			assert.Error(t, resolved.Validate(float64(42)))
			assert.Error(t, resolved.Validate(nil))
		})
	}
}

func resolveToolSchema(t *testing.T, schema json.RawMessage) *jsonschema.Resolved {
	t.Helper()
	var s jsonschema.Schema
	require.NoError(t, json.Unmarshal(schema, &s))
	resolved, err := s.Resolve(nil)
	require.NoError(t, err)
	return resolved
}

// The end-to-end guarantee: what the real handler actually emits must validate
// against the schema the tool advertises. The mirror sets structuredContent to
// the exact bytes of the text block, so validating the text block here is
// equivalent to validating the structured content a client would receive.
func TestIssueReadOutputValidatesAgainstDeclaredSchema(t *testing.T) {
	t.Parallel()

	serverTool := IssueRead(translations.NullTranslationHelper)
	resolved := resolveToolSchema(t, issueReadOutputSchema)

	mockIssue := &gogithub.Issue{
		Number:  gogithub.Ptr(1),
		Title:   gogithub.Ptr("Test issue"),
		State:   gogithub.Ptr("open"),
		HTMLURL: gogithub.Ptr("https://github.com/octocat/repo/issues/1"),
		User:    &gogithub.User{Login: gogithub.Ptr("octocat")},
	}

	tests := []struct {
		name     string
		method   string
		handlers map[string]http.HandlerFunc
	}{
		{
			name:   "get",
			method: "get",
			handlers: map[string]http.HandlerFunc{
				GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssue),
			},
		},
		{
			// A fully sparse issue: every optional field absent. Exercises the
			// branch's required set rather than a convenient fixture.
			name:   "get with all optionals absent",
			method: "get",
			handlers: map[string]http.HandlerFunc{
				GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, &gogithub.Issue{}),
			},
		},
		{
			name:   "get_comments",
			method: "get_comments",
			handlers: map[string]http.HandlerFunc{
				GetReposIssuesCommentsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK,
					[]*gogithub.IssueComment{{
						ID:      gogithub.Ptr(int64(1)),
						Body:    gogithub.Ptr("hello"),
						HTMLURL: gogithub.Ptr("https://github.com/octocat/repo/issues/1#issuecomment-1"),
						User:    &gogithub.User{Login: gogithub.Ptr("octocat")},
					}}),
			},
		},
		{
			// The ambiguous case that makes oneOf unusable: [] satisfies every
			// array branch at once.
			name:   "get_comments with no comments",
			method: "get_comments",
			handlers: map[string]http.HandlerFunc{
				GetReposIssuesCommentsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*gogithub.IssueComment{}),
			},
		},
		{
			name:   "get_sub_issues",
			method: "get_sub_issues",
			handlers: map[string]http.HandlerFunc{
				GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK,
					[]*gogithub.Issue{mockIssue}),
			},
		},
		{
			name:   "get_sub_issues with none",
			method: "get_sub_issues",
			handlers: map[string]http.HandlerFunc{
				GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*gogithub.Issue{}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(tt.handlers))}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"method":       tt.method,
				"owner":        "octocat",
				"repo":         "repo",
				"issue_number": float64(1),
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "handler should succeed")

			text := getTextResult(t, result)
			var payload any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload),
				"handler output must be JSON for the mirror to publish it as structuredContent")

			require.NoError(t, resolved.Validate(payload),
				"real handler output for method=%s must conform to the advertised outputSchema", tt.method)
		})
	}
}

// go-github declares `var subIssues []*SubIssue` and leaves it nil on an empty
// body, which marshals to the literal `null` rather than `[]`. The schema
// rejects null, deliberately: the right fix is to normalise in the handler,
// not to widen the contract for every caller.
func TestGetSubIssuesNeverEmitsNull(t *testing.T) {
	t.Parallel()

	serverTool := IssueRead(translations.NullTranslationHelper)
	deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		// A 200 whose body is JSON null — what go-github leaves the slice nil on.
		GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`null`))
		},
	}))}

	request := createMCPRequest(map[string]any{
		"method": "get_sub_issues", "owner": "octocat", "repo": "repo", "issue_number": float64(1),
	})
	result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := getTextResult(t, result)
	assert.NotEqual(t, "null", strings.TrimSpace(text.Text),
		"a nil slice must be normalised to [] before marshalling, or structuredContent violates the schema")

	require.NoError(t, resolveToolSchema(t, issueReadOutputSchema).Validate(func() any {
		var v any
		require.NoError(t, json.Unmarshal([]byte(text.Text), &v))
		return v
	}()))
}
