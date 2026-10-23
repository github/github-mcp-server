package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListAutomationTags(t *testing.T) {
	toolDef := ListAutomationTags(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))
	assert.True(t, toolDef.Tool.Annotations.ReadOnlyHint)

	t.Run("validates required parameters and pagination", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			args      map[string]any
			wantError string
		}{
			{"missing owner", map[string]any{"repo": "repo"}, "owner"},
			{"empty owner", map[string]any{"owner": "", "repo": "repo"}, "owner"},
			{"missing repo", map[string]any{"owner": "owner"}, "repo"},
			{"empty repo", map[string]any{"owner": "owner", "repo": ""}, "repo"},
			{"zero page", map[string]any{"owner": "owner", "repo": "repo", "page": 0}, "page"},
			{"negative page", map[string]any{"owner": "owner", "repo": "repo", "page": -1}, "page"},
			{"zero page size", map[string]any{"owner": "owner", "repo": "repo", "perPage": 0}, "perPage"},
			{"negative page size", map[string]any{"owner": "owner", "repo": "repo", "perPage": -1}, "perPage"},
			{"page size limit", map[string]any{"owner": "owner", "repo": "repo", "perPage": 101}, "perPage"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				calls := 0
				client := mustNewGHClient(t, MockHTTPClientWithHandler(func(w http.ResponseWriter, _ *http.Request) {
					calls++
					_, err := fmt.Fprint(w, `{"tags":[],"total_count":0}`)
					require.NoError(t, err)
				}))
				deps := BaseDeps{Client: client}
				handler := toolDef.Handler(deps)
				request := createMCPRequest(tc.args)
				result, err := handler(ContextWithDeps(context.Background(), deps), &request)
				require.NoError(t, err)
				assert.Zero(t, calls)
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, tc.wantError)
			})
		}
	})

	t.Run("discovery", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			toolsets []string
			scopes   []string
			want     bool
		}{
			{"default", []string{"default"}, []string{"repo"}, false},
			{"opt in read only", []string{"copilot_automations"}, []string{"repo"}, true},
			{"missing scope", []string{"copilot_automations"}, []string{}, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				inv, err := NewInventory(translations.NullTranslationHelper).
					WithToolsets(tc.toolsets).WithReadOnly(true).
					WithFilter(CreateToolScopeFilter(tc.scopes)).Build()
				require.NoError(t, err)
				require.Empty(t, inv.UnrecognizedToolsets())
				var found bool
				for _, tool := range inv.AvailableTools(context.Background()) {
					found = found || tool.Tool.Name == "list_automation_tags"
				}
				assert.Equal(t, tc.want, found)
			})
		}
	})

	for _, protocol := range []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25"} {
		t.Run(protocol, func(t *testing.T) {
			for _, tc := range []struct {
				name        string
				args        map[string]any
				status      int
				body        string
				link        string
				query       string
				escapedPath string
				want        string
				wantError   string
			}{
				{
					name:   "page and additive fields",
					args:   map[string]any{"owner": "owner", "repo": "repo", "page": 2, "perPage": 1},
					status: http.StatusOK,
					body:   `{"tags":[{"id":"tag-1","name":"Release","description":"","version":"opaque-version","created_at":"2026-10-01T00:00:00Z","updated_at":"2026-10-02T00:00:00Z","future_field":true}],"total_count":3,"future_field":true}`,
					link:   `</agents/repos/owner/repo/automation-tags?page=3&per_page=1>; rel="next"`,
					query:  "page=2&per_page=1",
					want:   `{"tags":[{"id":"tag-1","name":"Release","description":"","version":"opaque-version","created_at":"2026-10-01T00:00:00Z","updated_at":"2026-10-02T00:00:00Z"}],"total_count":3,"nextPage":3}`,
				},
				{
					name:   "empty visible catalog",
					status: http.StatusOK, body: `{"tags":[],"total_count":0}`,
					query: "page=1&per_page=30", want: `{"tags":[],"total_count":0,"nextPage":0}`,
				},
				{
					name:   "last page",
					args:   map[string]any{"owner": "owner", "repo": "repo", "page": 3, "perPage": 1},
					status: http.StatusOK, body: `{"tags":[],"total_count":2}`,
					query: "page=3&per_page=1", want: `{"tags":[],"total_count":2,"nextPage":0}`,
				},
				{
					name:   "maximum page size",
					args:   map[string]any{"owner": "owner", "repo": "repo", "perPage": 100},
					status: http.StatusOK, body: `{"tags":[],"total_count":0}`,
					query: "page=1&per_page=100", want: `{"tags":[],"total_count":0,"nextPage":0}`,
				},
				{
					name:        "escaped path segments",
					args:        map[string]any{"owner": "own er/team", "repo": "re/po"},
					status:      http.StatusOK,
					body:        `{"tags":[],"total_count":0}`,
					escapedPath: "/agents/repos/own%20er%2Fteam/re%2Fpo/automation-tags",
					want:        `{"tags":[],"total_count":0,"nextPage":0}`,
				},
				{name: "non-disclosure", status: http.StatusNotFound, body: `{"message":"Not Found"}`, wantError: "404"},
				{name: "empty denial", status: http.StatusNotFound, wantError: "404"},
				{name: "bad request", status: http.StatusBadRequest, body: `{"message":"Bad Request"}`, wantError: "400"},
				{name: "permission denied", status: http.StatusForbidden, body: `{"message":"Forbidden"}`, wantError: "403"},
				{name: "unauthenticated", status: http.StatusUnauthorized, body: `{"message":"Requires authentication"}`, wantError: "401"},
				{name: "temporarily unavailable", status: http.StatusServiceUnavailable, body: `{"message":"Unavailable"}`, wantError: "503"},
				{name: "server error", status: http.StatusInternalServerError, body: `{"message":"Internal Server Error"}`, wantError: "500"},
				{name: "validation", status: http.StatusUnprocessableEntity, body: `{"message":"Validation Failed"}`, wantError: "Validation Failed"},
				{name: "unexpected success status", status: http.StatusNoContent, wantError: "204"},
				{name: "missing owner", args: map[string]any{"repo": "repo"}, wantError: "owner"},
				{name: "empty repo", args: map[string]any{"owner": "owner", "repo": ""}, wantError: "repo"},
				{name: "dot owner", args: map[string]any{"owner": ".", "repo": "repo"}, wantError: `owner must not be "."`},
				{name: "dot dot owner", args: map[string]any{"owner": "..", "repo": "repo"}, wantError: `owner must not be ".."`},
				{name: "dot repo", args: map[string]any{"owner": "owner", "repo": "."}, wantError: `repo must not be "."`},
				{name: "dot dot repo", args: map[string]any{"owner": "owner", "repo": ".."}, wantError: `repo must not be ".."`},
				{name: "invalid page", args: map[string]any{"owner": "owner", "repo": "repo", "page": 0}, wantError: "page"},
				{name: "fractional page", args: map[string]any{"owner": "owner", "repo": "repo", "page": 1.5}, wantError: "page"},
				{name: "page size limit", args: map[string]any{"owner": "owner", "repo": "repo", "perPage": 101}, wantError: "perPage"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					calls := 0
					client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
						"": func(w http.ResponseWriter, r *http.Request) {
							calls++
							if tc.status == 0 {
								_, err := fmt.Fprint(w, `{}`)
								require.NoError(t, err)
								return
							}
							assert.Equal(t, "api.github.com", r.URL.Host)
							assert.Equal(t, http.MethodGet, r.Method)
							escapedPath := tc.escapedPath
							if escapedPath == "" {
								escapedPath = "/agents/repos/owner/repo/automation-tags"
							}
							assert.Equal(t, escapedPath, r.URL.EscapedPath())
							query := tc.query
							if query == "" {
								query = "page=1&per_page=30"
							}
							assert.Equal(t, query, r.URL.RawQuery)
							w.Header().Set("Content-Type", "application/json")
							w.Header().Set("Link", tc.link)
							w.WriteHeader(tc.status)
							_, err := fmt.Fprint(w, tc.body)
							require.NoError(t, err)
						},
					}))
					deps := BaseDeps{Client: client}
					inv, err := NewInventory(translations.NullTranslationHelper).
						WithToolsets([]string{"copilot_automations"}).WithReadOnly(true).Build()
					require.NoError(t, err)
					server := mcp.NewServer(&mcp.Implementation{Name: "automation-tags", Version: "v1"}, nil)
					server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
					inv.RegisterTools(context.Background(), server, deps)
					session := connectCommentVisibilityClient(t, server, protocol)
					list, err := session.ListTools(context.Background(), nil)
					require.NoError(t, err)
					require.Len(t, list.Tools, 1)
					assert.Equal(t, "list_automation_tags", list.Tools[0].Name)
					args := tc.args
					if args == nil {
						args = map[string]any{"owner": "owner", "repo": "repo"}
					}
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
						Name: "list_automation_tags", Arguments: args,
					})
					require.NoError(t, err)
					if tc.status == 0 {
						assert.Zero(t, calls)
					} else {
						assert.Equal(t, 1, calls)
					}
					if tc.wantError != "" {
						require.True(t, result.IsError)
						assert.Contains(t, getErrorResult(t, result).Text, tc.wantError)
						assert.Nil(t, result.StructuredContent)
						return
					}
					require.False(t, result.IsError)
					assert.JSONEq(t, tc.want, getTextResult(t, result).Text)
					if protocol == inventory.ProtocolVersionMultiRoundTrip {
						require.NotNil(t, list.Tools[0].OutputSchema)
						encoded, err := json.Marshal(result.StructuredContent)
						require.NoError(t, err)
						assert.JSONEq(t, tc.want, string(encoded))
					} else {
						assert.Nil(t, result.StructuredContent)
					}
				})
			}
		})
	}
}
