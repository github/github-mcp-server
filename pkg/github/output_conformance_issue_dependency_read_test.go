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

	"github.com/github/github-mcp-server/pkg/translations"
)

// The end-to-end guarantee for issue_dependency_read: what the real handler
// emits must validate against the outputSchema the tool advertises. The
// structured-content mirror publishes the exact bytes of the single text block
// as structuredContent, so validating the text block here is equivalent to
// validating what a client receives.
//
// Every value of the tool's `method` enum is covered, and the enum itself is
// pinned below so adding a method without adding a case fails the test rather
// than silently shipping an unvalidated branch.
func TestIssueDependencyReadOutputValidatesAgainstDeclaredSchema(t *testing.T) {
	t.Parallel()

	serverTool := IssueDependencyRead(translations.NullTranslationHelper)
	resolved := resolveToolSchema(t, issueDependencyReadOutputSchema)

	// A fully populated issue: every field the projection reads is present.
	fullIssue := &gogithub.Issue{
		Number:        gogithub.Ptr(7),
		Title:         gogithub.Ptr("Blocker"),
		State:         gogithub.Ptr("open"),
		HTMLURL:       gogithub.Ptr("https://github.com/owner/repo/issues/7"),
		RepositoryURL: gogithub.Ptr("https://api.github.com/repos/owner/repo"),
	}
	// A second issue in a different repo, closed, to exercise the upper-casing
	// of state and a cross-repo "owner/repo" derivation in the same payload.
	crossRepoIssue := &gogithub.Issue{
		Number:        gogithub.Ptr(9),
		Title:         gogithub.Ptr("Blocked elsewhere"),
		State:         gogithub.Ptr("closed"),
		HTMLURL:       gogithub.Ptr("https://github.com/other/thing/issues/9"),
		RepositoryURL: gogithub.Ptr("https://api.github.com/repos/other/thing"),
	}

	// Advertises a next page so pageInfo.hasNextPage/nextPage are non-zero
	// rather than always taking the same branch.
	withNextPage := func(body any) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/issues/123/dependencies/blocked_by?page=2>; rel="next"`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(MustMarshal(body))
		}
	}

	tests := []struct {
		name     string
		method   string
		handlers map[string]http.HandlerFunc
		// Shape expectations, so a case cannot pass by validating a payload
		// that never contained what the case is named for.
		wantIssues       int
		wantNextPage     bool
		wantNoRepository bool
	}{
		{
			name:   "get_blocked_by",
			method: "get_blocked_by",
			handlers: map[string]http.HandlerFunc{
				string(endpointBlockedBy): mockResponse(t, http.StatusOK, []*gogithub.Issue{fullIssue, crossRepoIssue}),
			},
			wantIssues: 2,
		},
		{
			// Empty collection: the list method returning zero items. `issues`
			// is still required, so [] must be emitted (never null, never absent).
			name:   "get_blocked_by with no blockers",
			method: "get_blocked_by",
			handlers: map[string]http.HandlerFunc{
				string(endpointBlockedBy): mockResponse(t, http.StatusOK, []*gogithub.Issue{}),
			},
			wantIssues: 0,
		},
		{
			// A fully sparse issue: every optional field absent. `repository`
			// is omitempty and drops out entirely, so this exercises the
			// minimalIssueRef required set rather than a convenient fixture.
			name:   "get_blocked_by with all optionals absent",
			method: "get_blocked_by",
			handlers: map[string]http.HandlerFunc{
				string(endpointBlockedBy): mockResponse(t, http.StatusOK, []*gogithub.Issue{{}}),
			},
			wantIssues:       1,
			wantNoRepository: true,
		},
		{
			name:   "get_blocked_by with next page",
			method: "get_blocked_by",
			handlers: map[string]http.HandlerFunc{
				string(endpointBlockedBy): withNextPage([]*gogithub.Issue{fullIssue}),
			},
			wantIssues:   1,
			wantNextPage: true,
		},
		{
			name:   "get_blocking",
			method: "get_blocking",
			handlers: map[string]http.HandlerFunc{
				string(endpointBlocking): mockResponse(t, http.StatusOK, []*gogithub.Issue{fullIssue, crossRepoIssue}),
			},
			wantIssues: 2,
		},
		{
			name:   "get_blocking with none",
			method: "get_blocking",
			handlers: map[string]http.HandlerFunc{
				string(endpointBlocking): mockResponse(t, http.StatusOK, []*gogithub.Issue{}),
			},
			wantIssues: 0,
		},
		{
			name:   "get_blocking with all optionals absent",
			method: "get_blocking",
			handlers: map[string]http.HandlerFunc{
				string(endpointBlocking): mockResponse(t, http.StatusOK, []*gogithub.Issue{{}}),
			},
			wantIssues:       1,
			wantNoRepository: true,
		},
		{
			name:   "get_blocking with next page",
			method: "get_blocking",
			handlers: map[string]http.HandlerFunc{
				string(endpointBlocking): withNextPage([]*gogithub.Issue{crossRepoIssue}),
			},
			wantIssues:   1,
			wantNextPage: true,
		},
	}

	// Pin the enum: a new method must gain a case above, not slip through.
	covered := map[string]bool{}
	for _, tt := range tests {
		covered[tt.method] = true
	}
	inputSchema, ok := serverTool.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "input schema should be a *jsonschema.Schema")
	for _, m := range inputSchema.Properties["method"].Enum {
		assert.True(t, covered[m.(string)],
			"method %q is advertised in the enum but has no output-conformance case", m)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(tt.handlers))}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(map[string]any{
				"method":       tt.method,
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
			})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "handler should succeed")

			text := getTextResult(t, result)

			// A nil slice marshalled as `null` would violate the schema's
			// required `issues` array; the handler normalises to [].
			assert.NotEqual(t, "null", strings.TrimSpace(text.Text))

			var payload any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload),
				"handler output must be JSON for the mirror to publish it as structuredContent")

			require.NoError(t, resolved.Validate(payload),
				"real handler output for method=%s must conform to the advertised outputSchema", tt.method)

			// Prove the payload really carried what this case is named for;
			// otherwise an always-empty response would validate trivially.
			var shape struct {
				Issues   []map[string]any `json:"issues"`
				PageInfo struct {
					HasNextPage bool `json:"hasNextPage"`
					NextPage    int  `json:"nextPage"`
				} `json:"pageInfo"`
			}
			require.NoError(t, json.Unmarshal([]byte(text.Text), &shape))
			require.Len(t, shape.Issues, tt.wantIssues)
			assert.Equal(t, tt.wantNextPage, shape.PageInfo.HasNextPage)
			if tt.wantNextPage {
				assert.NotZero(t, shape.PageInfo.NextPage)
			} else {
				assert.Zero(t, shape.PageInfo.NextPage)
			}
			if tt.wantNoRepository {
				for _, issue := range shape.Issues {
					assert.NotContains(t, issue, "repository",
						"the sparse case must actually omit the optional field it is testing")
				}
			}
		})
	}

	// The cases above are only evidence if the validator can fail. Pin the
	// shapes this schema must reject, so a future widening that makes every
	// payload pass shows up here rather than as a silent loss of coverage.
	t.Run("schema rejects drifted output", func(t *testing.T) {
		valid := map[string]any{
			"number": float64(7), "title": "Blocker", "state": "OPEN",
			"url": "https://github.com/owner/repo/issues/7",
		}
		pageInfo := map[string]any{"hasNextPage": false, "nextPage": float64(0)}

		assert.Error(t, resolved.Validate(map[string]any{"issues": []any{valid}}),
			"pageInfo is required")
		assert.Error(t, resolved.Validate(map[string]any{"pageInfo": pageInfo}),
			"issues is required")
		assert.Error(t, resolved.Validate(map[string]any{"issues": nil, "pageInfo": pageInfo}),
			"issues must be an array, never null")
		assert.Error(t, resolved.Validate(map[string]any{
			"issues":   []any{map[string]any{"number": float64(7), "title": "Blocker", "state": "OPEN"}},
			"pageInfo": pageInfo,
		}), "an issue ref missing url must be rejected")
		assert.Error(t, resolved.Validate(map[string]any{
			"issues":   []any{valid},
			"pageInfo": map[string]any{"hasNextPage": false},
		}), "pageInfo missing nextPage must be rejected")
	})
}

// A 200 whose body is JSON `null` leaves go-github's slice nil. `issues` is
// required and typed as an array, so a nil slice reaching the marshaller
// unnormalised would emit `"issues":null` and break every client reading
// structuredContent.
func TestIssueDependencyReadNeverEmitsNullIssues(t *testing.T) {
	t.Parallel()

	serverTool := IssueDependencyRead(translations.NullTranslationHelper)
	resolved := resolveToolSchema(t, issueDependencyReadOutputSchema)

	nullBody := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`null`))
	}

	for _, tc := range []struct {
		method   string
		endpoint EndpointPattern
	}{
		{"get_blocked_by", endpointBlockedBy},
		{"get_blocking", endpointBlocking},
	} {
		t.Run(tc.method, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				string(tc.endpoint): nullBody,
			}))}

			request := createMCPRequest(map[string]any{
				"method":       tc.method,
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
			})
			result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError)

			text := getTextResult(t, result)
			assert.NotContains(t, text.Text, `"issues":null`,
				"a nil slice must be normalised to [] before marshalling, or structuredContent violates the schema")

			var payload any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload))
			require.NoError(t, resolved.Validate(payload))
		})
	}
}
