package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const actionsCheckRunJSON = `{
	"id":101,"name":"build","status":"completed","conclusion":"failure",
	"head_sha":"abc123","node_id":"check-node","url":"https://api.github.com/repos/owner/repo/check-runs/101",
	"html_url":"https://github.com/owner/repo/runs/101","details_url":"https://ci.example/build/101",
	"started_at":"2026-10-01T10:00:00Z","completed_at":"2026-10-01T10:01:00Z",
	"check_suite":{"id":42,"node_id":"suite-node"},
	"app":{"id":15368,"slug":"github-actions","owner":{"login":"github"},"permissions":{"checks":"write"}},
	"output":{"title":"Build failed","summary":"One test failed","text":"Expected 2, got 1",
		"annotations_count":2,"annotations_url":"https://api.github.com/repos/owner/repo/check-runs/101/annotations",
		"annotations":[{"path":"main.go","message":"Do not embed annotations in check details"}]}
}`

const actionsCheckSummaryJSON = `{
	"id":101,"name":"build","status":"completed","conclusion":"failure",
	"head_sha":"abc123","check_suite_id":42,"app":{"id":15368,"slug":"github-actions"},
	"html_url":"https://github.com/owner/repo/runs/101","details_url":"https://ci.example/build/101",
	"started_at":"2026-10-01T10:00:00Z","completed_at":"2026-10-01T10:01:00Z"
}`

func TestActionsChecksWire(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		method   string
		field    string
		args     map[string]any
		path     string
		query    string
		body     string
		payload  string
		nextPage int
	}{
		{
			name: "check summaries for a commit",
			tool: "actions_list", method: "list_check_runs", field: "check_runs",
			args: map[string]any{"ref": "abc123"},
			path: "/repos/owner/repo/commits/abc123/check-runs", query: "filter=latest&page=1&per_page=30",
			body:    `{"total_count":1,"check_runs":[` + actionsCheckRunJSON + `]}`,
			payload: `{"total_count":1,"check_runs":[` + actionsCheckSummaryJSON + `]}`,
		},
		{
			name: "check summaries for a suite with pagination",
			tool: "actions_list", method: "list_check_runs", field: "check_runs",
			args: map[string]any{"resource_id": "42", "page": 1, "perPage": 1, "check_runs_filter": map[string]any{"filter": "all"}},
			path: "/repos/owner/repo/check-suites/42/check-runs", query: "filter=all&page=1&per_page=1",
			body:     `{"total_count":2,"check_runs":[` + actionsCheckRunJSON + `]}`,
			payload:  `{"total_count":2,"check_runs":[` + actionsCheckSummaryJSON + `],"next_page":2}`,
			nextPage: 2,
		},
		{
			name: "reference lookup filters",
			tool: "actions_list", method: "list_check_runs", field: "check_runs",
			args: map[string]any{
				"ref": "abc123", "page": 2, "perPage": 5,
				"check_runs_filter": map[string]any{"check_name": "build", "status": "completed", "filter": "all", "app_id": 15368},
			},
			path: "/repos/owner/repo/commits/abc123/check-runs", query: "app_id=15368&check_name=build&filter=all&page=2&per_page=5&status=completed",
			body: `{"total_count":0,"check_runs":[]}`, payload: `{"total_count":0,"check_runs":[]}`,
		},
		{
			name: "repeated names retain distinct attempts",
			tool: "actions_list", method: "list_check_runs", field: "check_runs",
			args: map[string]any{"resource_id": "42", "check_runs_filter": map[string]any{"filter": "all"}},
			path: "/repos/owner/repo/check-suites/42/check-runs", query: "filter=all&page=1&per_page=30",
			body: `{"total_count":2,"check_runs":[` + actionsCheckRunJSON + `,
				{"id":100,"name":"build","status":"completed","conclusion":"success","check_suite":{"id":42}}]}`,
			payload: `{"total_count":2,"check_runs":[` + actionsCheckSummaryJSON + `,
				{"id":100,"name":"build","status":"completed","conclusion":"success","check_suite_id":42}]}`,
		},
		{
			name: "single check output without embedded annotations",
			tool: "actions_get", method: "get_check_run", field: "check_run",
			args: map[string]any{"resource_id": "101"},
			path: "/repos/owner/repo/check-runs/101",
			body: actionsCheckRunJSON,
			payload: `{
				"id":101,"name":"build","status":"completed","conclusion":"failure",
				"head_sha":"abc123","check_suite_id":42,"app":{"id":15368,"slug":"github-actions"},
				"html_url":"https://github.com/owner/repo/runs/101","details_url":"https://ci.example/build/101",
				"started_at":"2026-10-01T10:00:00Z","completed_at":"2026-10-01T10:01:00Z",
				"output":{"title":"Build failed","summary":"One test failed","text":"Expected 2, got 1","annotations_count":2}
			}`,
		},
		{
			name: "pending check without output",
			tool: "actions_get", method: "get_check_run", field: "check_run",
			args:    map[string]any{"resource_id": "102"},
			path:    "/repos/owner/repo/check-runs/102",
			body:    `{"id":102,"name":"build","status":"queued","conclusion":null,"output":null,"app":null,"check_suite":null}`,
			payload: `{"id":102,"name":"build","status":"queued"}`,
		},
		{
			name: "annotation page with source coordinates",
			tool: "actions_list", method: "list_check_run_annotations", field: "check_run_annotations",
			args: map[string]any{"resource_id": "101", "perPage": 1},
			path: "/repos/owner/repo/check-runs/101/annotations", query: "page=1&per_page=1",
			body: `[{"path":"main.go","start_line":10,"end_line":10,"start_column":2,"end_column":8,
				"annotation_level":"failure","title":"Assertion","message":"Expected 2","raw_details":"Got 1"}]`,
			payload: `{"annotations":[{"path":"main.go","start_line":10,"end_line":10,"start_column":2,"end_column":8,
				"annotation_level":"failure","title":"Assertion","message":"Expected 2","raw_details":"Got 1"}],"next_page":2}`,
			nextPage: 2,
		},
		{
			name: "last annotation page",
			tool: "actions_list", method: "list_check_run_annotations", field: "check_run_annotations",
			args: map[string]any{"resource_id": "101", "page": 2},
			path: "/repos/owner/repo/check-runs/101/annotations", query: "page=2&per_page=30",
			body: `[]`, payload: `{"annotations":[]}`,
		},
	}

	for _, protocol := range typedActionsProtocols {
		for _, tc := range tests {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				requests := 0
				client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, tc.path, r.URL.Path)
					assert.Equal(t, tc.query, r.URL.RawQuery)
					if tc.nextPage != 0 {
						w.Header().Set("Link", fmt.Sprintf("<https://api.github.com%s?page=%d>; rel=\"next\"", tc.path, tc.nextPage))
					}
					_, _ = w.Write([]byte(tc.body))
				})}}
				session, schemas := actionsTypedSession(t, BaseDeps{Client: mustNewGHClient(t, client)}, protocol)
				args := actionsCaseArgs(actionsWireCase{method: tc.method, args: tc.args})
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: args})
				require.NoError(t, err)
				require.False(t, result.IsError, getTextResult(t, result).Text)

				var payload any
				require.NoError(t, json.Unmarshal([]byte(tc.payload), &payload))
				legacy := mustMarshalJSON(t, payload)
				if schemas[tc.tool] == nil {
					assert.JSONEq(t, legacy, getTextResult(t, result).Text)
					assert.Nil(t, result.StructuredContent)
				} else {
					modern := mustMarshalJSON(t, map[string]any{"method": tc.method, tc.field: payload})
					assertActionsWireResult(t, result, schemas[tc.tool], legacy, false, modern)
				}
				assert.Equal(t, 1, requests, "detail and list methods must not fetch extra checks, pages, or logs")
			})
		}
	}

}

func TestActionsChecksValidation(t *testing.T) {
	tests := []struct {
		tool, method string
		args         map[string]any
		want         string
	}{
		{"actions_list", "list_check_runs", nil, "exactly one"},
		{"actions_list", "list_check_runs", map[string]any{"ref": "abc123", "resource_id": "42"}, "exactly one"},
		{"actions_list", "list_check_runs", map[string]any{"resource_id": "-1"}, "positive check suite ID"},
		{"actions_list", "list_check_runs", map[string]any{"resource_id": "bad"}, "positive check suite ID"},
		{"actions_list", "list_check_runs", map[string]any{"resource_id": "9223372036854775808"}, "positive check suite ID"},
		{"actions_list", "list_check_runs", map[string]any{"ref": false}, "ref"},
		{"actions_list", "list_check_runs", map[string]any{"ref": "abc123", "check_runs_filter": false}, "check_runs_filter"},
		{"actions_list", "list_check_runs", map[string]any{"ref": "abc123", "check_runs_filter": map[string]any{"status": "failure"}}, "status"},
		{"actions_list", "list_check_runs", map[string]any{"ref": "abc123", "check_runs_filter": map[string]any{"filter": "invalid"}}, "filter"},
		{"actions_list", "list_check_runs", map[string]any{"ref": "abc123", "check_runs_filter": map[string]any{"app_id": 0}}, "app_id"},
		{"actions_list", "list_check_runs", map[string]any{"resource_id": "42", "check_runs_filter": map[string]any{"app_id": 1}}, "only supported with ref"},
		{"actions_list", "list_check_runs", map[string]any{"ref": "abc123", "page": -1}, "page"},
		{"actions_list", "list_check_run_annotations", map[string]any{"resource_id": "101", "perPage": 101}, "perPage"},
		{"actions_list", "list_check_run_annotations", map[string]any{"resource_id": "0"}, "positive check run ID"},
		{"actions_list", "list_check_run_annotations", nil, "resource_id"},
		{"actions_get", "get_check_run", map[string]any{"resource_id": "0"}, "positive check run ID"},
		{"actions_get", "get_check_run", map[string]any{"resource_id": "-1"}, "positive check run ID"},
		{"actions_get", "get_check_run", map[string]any{"resource_id": "1.5"}, "resource_id"},
	}
	deps := BaseDeps{Client: mustNewGHClient(t, &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("invalid input must not make an API request")
	})}})}
	for _, tc := range tests {
		t.Run("direct/"+tc.method+"/"+mustMarshalJSON(t, tc.args), func(t *testing.T) {
			tool := ActionsList(translations.NullTranslationHelper)
			if tc.tool == "actions_get" {
				tool = ActionsGet(translations.NullTranslationHelper)
			}
			request := createMCPRequest(actionsCaseArgs(actionsWireCase{method: tc.method, args: tc.args}))
			result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getTextResult(t, result).Text, tc.want)
		})
	}
	for _, protocol := range typedActionsProtocols {
		session, _ := actionsTypedSession(t, deps, protocol)
		for _, tc := range tests {
			t.Run(protocol+"/"+tc.method+"/"+mustMarshalJSON(t, tc.args), func(t *testing.T) {
				args := actionsCaseArgs(actionsWireCase{method: tc.method, args: tc.args})
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: args})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getTextResult(t, result).Text, tc.want)
				assert.Nil(t, result.StructuredContent)
			})
		}
	}
}

func TestActionsChecksAPIErrors(t *testing.T) {
	for _, protocol := range typedActionsProtocols {
		for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests} {
			for _, tc := range []actionsWireCase{
				{name: "actions_list", method: "list_check_runs", args: map[string]any{"ref": "abc123"}},
				{name: "actions_list", method: "list_check_runs", args: map[string]any{"resource_id": "42"}},
				{name: "actions_list", method: "list_check_run_annotations", args: map[string]any{"resource_id": "101"}},
				{name: "actions_get", method: "get_check_run", args: map[string]any{"resource_id": "101"}},
			} {
				t.Run(fmt.Sprintf("%s/%s/%d/%v", protocol, tc.method, status, tc.args), func(t *testing.T) {
					requests := 0
					client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						requests++
						w.WriteHeader(status)
						_, _ = w.Write([]byte(`{"message":"denied"}`))
					})}}
					session, _ := actionsTypedSession(t, BaseDeps{Client: mustNewGHClient(t, client)}, protocol)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: actionsCaseArgs(tc)})
					require.NoError(t, err)
					require.True(t, result.IsError)
					assert.Contains(t, getTextResult(t, result).Text, "denied")
					assert.Nil(t, result.StructuredContent)
					assert.Equal(t, 1, requests)
				})
			}
		}
	}
}

func TestActionsChecksIFC(t *testing.T) {
	for _, protocol := range typedActionsProtocols {
		for _, private := range []bool{false, true} {
			for _, tc := range []actionsWireCase{
				{name: "actions_list", method: "list_check_runs", args: map[string]any{"ref": "abc123"}, body: `{"total_count":1,"check_runs":[` + actionsCheckRunJSON + `]}`},
				{name: "actions_list", method: "list_check_run_annotations", args: map[string]any{"resource_id": "101"}, body: `[]`},
				{name: "actions_get", method: "get_check_run", args: map[string]any{"resource_id": "101"}, body: actionsCheckRunJSON},
			} {
				t.Run(fmt.Sprintf("%s/%s/private=%t", protocol, tc.method, private), func(t *testing.T) {
					client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == "/repos/owner/repo" {
							_, _ = fmt.Fprintf(w, `{"private":%t}`, private)
						} else {
							_, _ = w.Write([]byte(tc.body))
						}
					})}}
					deps := BaseDeps{
						Client: mustNewGHClient(t, client), RepoAccessCache: stubRepoAccessCache(nil, time.Minute),
						featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
					}
					session, _ := actionsTypedSession(t, deps, protocol)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: actionsCaseArgs(tc)})
					require.NoError(t, err)
					require.False(t, result.IsError)
					label := unmarshalIFC(t, result.Meta["ifc"])
					assert.Equal(t, "untrusted", label["integrity"])
					confidentiality := "public"
					if private {
						confidentiality = "private"
					}
					assert.Equal(t, confidentiality, label["confidentiality"])
				})
			}
		}
	}
}

func TestActionsChecksLongDiagnostics(t *testing.T) {
	diagnostic := strings.Repeat("expected 2, got 1\n", 3000)
	for _, protocol := range typedActionsProtocols {
		for _, annotations := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/annotations=%t", protocol, annotations), func(t *testing.T) {
				tool, method, field := "actions_get", "get_check_run", "check_run"
				body := mustMarshalJSON(t, &github.CheckRun{
					ID: new(int64(101)), Name: new("build"), Status: new("completed"),
					Output: &github.CheckRunOutput{Title: new("Build\u200b failed"), Summary: new(diagnostic), Text: new(diagnostic)},
				})
				if annotations {
					tool, method, field = "actions_list", "list_check_run_annotations", "check_run_annotations"
					body = mustMarshalJSON(t, []*github.CheckRunAnnotation{{
						Path: new("main.go"), StartLine: new(1), EndLine: new(2), Title: new("Build\u200b failed"),
						Message: new(diagnostic), RawDetails: new(diagnostic),
					}})
				}
				client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, _ = w.Write([]byte(body))
				})}}
				session, schemas := actionsTypedSession(t, BaseDeps{Client: mustNewGHClient(t, client)}, protocol)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
					Name: tool, Arguments: map[string]any{"owner": "owner", "repo": "repo", "method": method, "resource_id": "101"},
				})
				require.NoError(t, err)
				require.False(t, result.IsError)
				var payload map[string]json.RawMessage
				require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &payload))
				if schemas[tool] != nil {
					require.NoError(t, json.Unmarshal(payload[field], &payload))
				}
				if annotations {
					var output []ActionsCheckRunAnnotation
					require.NoError(t, json.Unmarshal(payload["annotations"], &output))
					require.Len(t, output, 1)
					assert.Equal(t, "Build failed", output[0].Title)
					assert.Equal(t, diagnostic, output[0].Message)
					assert.Equal(t, diagnostic, output[0].RawDetails)
					assert.Nil(t, output[0].StartColumn)
					assert.Nil(t, output[0].EndColumn)
				} else {
					var output ActionsCheckRunOutput
					require.NoError(t, json.Unmarshal(payload["output"], &output))
					assert.Equal(t, "Build failed", output.Title)
					assert.Equal(t, diagnostic, output.Summary)
					assert.Equal(t, diagnostic, output.Text)
				}
			})
		}
	}
}

func TestActionsCheckPayloadSizes(t *testing.T) {
	run := actionsTestWorkflowRun()
	job := actionsTestWorkflowJob()
	minimalJob, err := convertToMinimalWorkflowJob(job)
	require.NoError(t, err)
	for _, tc := range []struct {
		name, field string
		raw, output any
	}{
		{"workflow run", "check_suite_id", run, convertToMinimalWorkflowRun(run)},
		{"workflow job", "check_run_id", job, minimalJob},
	} {
		payload := marshalActionsObject(t, tc.output)
		fullSize, size := len(mustMarshalJSON(t, tc.raw)), len(mustMarshalJSON(t, payload))
		delete(payload, tc.field)
		previousSize := len(mustMarshalJSON(t, payload))
		assert.LessOrEqual(t, size-previousSize, 64, "only the compact linkage ID should be added")
		assert.Less(t, size, fullSize)
		t.Logf("%s: %d -> %d bytes after linkage ID (+%d); full API object is %d bytes", tc.name, previousSize, size, size-previousSize, fullSize)
	}
	var checkRun github.CheckRun
	require.NoError(t, json.Unmarshal([]byte(actionsCheckRunJSON), &checkRun))
	checkRun.Output.Text = new(strings.Repeat("diagnostic", 5000))
	summary := mustMarshalJSON(t, convertToActionsCheckRunSummary(&checkRun))
	assert.Less(t, len(summary), 1000)
	assert.NotContains(t, summary, "output")
	assert.NotContains(t, summary, "annotations")
	assert.NotContains(t, summary, "permissions")
	assert.NotContains(t, summary, "node_id")
	t.Logf("check summary: %d bytes with a 50,000-byte upstream output.text", len(summary))
}
