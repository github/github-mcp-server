package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func actionsTypedSession(t *testing.T, deps ToolDependencies, protocol string) (*mcp.ClientSession, map[string]*jsonschema.Resolved) {
	t.Helper()
	tr := translations.NullTranslationHelper
	tools := []inventory.ServerTool{ActionsList(tr), ActionsGet(tr), ActionsRunTrigger(tr), ActionsGetJobLogs(tr)}
	inv, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "actions", Version: "v1"}, nil)
	inv.RegisterTools(context.Background(), server, deps)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	if protocol == "" || protocol == "unknown" {
		server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				switch req := req.(type) {
				case *mcp.ListToolsRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: protocol}
				case *mcp.CallToolRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: protocol}
				}
				return next(ctx, method, req)
			}
		})
	}
	version := protocol
	if version == "" || version == "unknown" {
		version = inventory.ProtocolVersionMultiRoundTrip
	}
	session := connectCommentVisibilityClient(t, server, version)
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 4)
	schemas := make(map[string]*jsonschema.Resolved)
	for _, tool := range list.Tools {
		if os.Getenv("ACTIONS_LEGACY_BASELINE") == "true" {
			continue
		}
		if protocol != inventory.ProtocolVersionMultiRoundTrip {
			assert.Nil(t, tool.OutputSchema, tool.Name)
			continue
		}
		require.NotNil(t, tool.OutputSchema)
		var schema jsonschema.Schema
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
		resolved, err := schema.Resolve(nil)
		require.NoError(t, err)
		schemas[tool.Name] = resolved
	}
	return session, schemas
}

var typedActionsProtocols = []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", "", "unknown"}

type actionsWireCase struct {
	name, method string
	args         map[string]any
	path, query  string
	body, text   string
	status       int
}

func actionsWireCases() []actionsWireCase {
	run := `{"id":7,"name":"CI","workflow_id":2,"run_number":3,"run_attempt":1,"status":"completed","conclusion":"failure","head_branch":"main","head_commit":{"message":"change"}}`
	return []actionsWireCase{
		{"actions_list", "list_workflows", nil, "/repos/owner/repo/actions/workflows", "page=1&per_page=30", `{"total_count":1,"workflows":[{"id":2,"name":"CI","created_at":"2026-01-01T00:00:00Z"}]}`, `{"total_count":1,"workflows":[{"id":2,"name":"CI","created_at":"2026-01-01T00:00:00Z"}]}`, 200},
		{"actions_list", "list_workflow_runs", map[string]any{"workflow_runs_filter": map[string]any{"actor": "octocat", "branch": false, "event": 1}}, "/repos/owner/repo/actions/runs", "actor=octocat&page=1&per_page=30", `{"total_count":1,"workflow_runs":[` + run + `]}`, `{"total_count":1,"workflow_runs":[` + run + `]}`, 200},
		{"actions_list", "list_workflow_runs", map[string]any{"resource_id": "ci.yml", "page": "2e0", "perPage": "5.0"}, "/repos/owner/repo/actions/workflows/ci.yml/runs", "page=2&per_page=5", `{"total_count":0,"workflow_runs":[]}`, `{"total_count":0,"workflow_runs":[]}`, 200},
		{"actions_list", "list_workflow_runs", map[string]any{"resource_id": "2"}, "/repos/owner/repo/actions/workflows/2/runs", "page=1&per_page=30", `null`, `{"total_count":0,"workflow_runs":[]}`, 200},
		{"actions_list", "list_workflow_jobs", map[string]any{"resource_id": "7", "workflow_jobs_filter": map[string]any{"filter": "all"}}, "/repos/owner/repo/actions/runs/7/jobs", "filter=all&page=1&per_page=30", `{"total_count":1,"jobs":[{"id":8,"run_id":7,"name":"test","status":"completed"}]}`, `{"jobs":{"total_count":1,"jobs":[{"id":8,"run_id":7,"name":"test","status":"completed"}]}}`, 200},
		{"actions_list", "list_workflow_run_artifacts", map[string]any{"resource_id": "7", "workflow_jobs_filter": false}, "/repos/owner/repo/actions/runs/7/artifacts", "page=1&per_page=30", `{"total_count":1,"artifacts":[{"id":9,"digest":"sha256:abc","workflow_run":{"id":7}}]}`, `{"total_count":1,"artifacts":[{"id":9,"digest":"sha256:abc","workflow_run":{"id":7}}]}`, 200},
		{"actions_get", "get_workflow", map[string]any{"resource_id": "ci.yml"}, "/repos/owner/repo/actions/workflows/ci.yml", "", `{"id":2,"name":"CI"}`, `{"id":2,"name":"CI"}`, 200},
		{"actions_get", "get_workflow_run", map[string]any{"resource_id": "7"}, "/repos/owner/repo/actions/runs/7", "", run, run, 200},
		{"actions_get", "get_workflow_job", map[string]any{"resource_id": "8"}, "/repos/owner/repo/actions/jobs/8", "", `{"id":8,"steps":[{"name":"test","number":1,"started_at":"2026-01-01T00:00:00Z"}],"labels":["linux"]}`, `{"id":8,"steps":[{"name":"test","number":1,"started_at":"2026-01-01T00:00:00Z"}],"labels":["linux"]}`, 200},
		{"actions_get", "get_workflow_run_usage", map[string]any{"resource_id": "7"}, "/repos/owner/repo/actions/runs/7/timing", "", `{"billable":{"CUSTOM":{"total_ms":42,"jobs":1,"job_runs":[{"job_id":8,"duration_ms":42}]}},"run_duration_ms":42}`, `{"billable":{"CUSTOM":{"total_ms":42,"jobs":1,"job_runs":[{"job_id":8,"duration_ms":42}]}},"run_duration_ms":42}`, 200},
		{"actions_get", "get_workflow", map[string]any{"resource_id": "-2"}, "/repos/owner/repo/actions/workflows/-2", "", `{}`, `{}`, 200},
		{"actions_get", "get_workflow_job", map[string]any{"resource_id": "8"}, "/repos/owner/repo/actions/jobs/8", "", `null`, `null`, 200},
		{"actions_get", "get_workflow_run_usage", map[string]any{"resource_id": "7"}, "/repos/owner/repo/actions/runs/7/timing", "", `{}`, `{}`, 200},
		{"actions_list", "list_workflows", map[string]any{"page": 0, "perPage": 0, "workflow_runs_filter": nil}, "/repos/owner/repo/actions/workflows", "page=1&per_page=30", `{}`, `{}`, 200},
		{"actions_list", "list_workflow_run_artifacts", map[string]any{"resource_id": "7"}, "/repos/owner/repo/actions/runs/7/artifacts", "page=1&per_page=30", `{}`, `{}`, 200},
		{"actions_list", "list_workflow_runs", map[string]any{"workflow_runs_filter": map[string]any{"event": "custom", "status": "custom"}}, "/repos/owner/repo/actions/runs", "event=custom&page=1&per_page=30&status=custom", `{}`, `{"total_count":0,"workflow_runs":[]}`, 200},
		{"actions_get", "download_workflow_run_artifact", map[string]any{"resource_id": "9"}, "/repos/owner/repo/actions/artifacts/9/zip", "", "", `{"artifact_id":9,"download_url":"https://logs.example/archive","message":"Artifact is available for download","note":"The download_url provides a download link for the artifact as a ZIP archive. The link is temporary and expires after a short time."}`, 302},
		{"actions_get", "get_workflow_run_logs_url", map[string]any{"resource_id": "7"}, "/repos/owner/repo/actions/runs/7/logs", "", "", `{"logs_url":"https://logs.example/archive","message":"Workflow run logs are available for download","note":"The logs_url provides a download link for the complete workflow run logs as a ZIP archive. You can download this archive to extract and examine individual job logs.","optimization_tip":"Use: get_job_logs with parameters {run_id: 7, failed_only: true} for more efficient failed job debugging","warning":"This downloads ALL logs as a ZIP file which can be large and expensive. For debugging failed jobs, consider using get_job_logs with failed_only=true and run_id instead."}`, 302},
		{"actions_run_trigger", "run_workflow", map[string]any{"workflow_id": "ci.yml", "ref": "main"}, "/repos/owner/repo/actions/workflows/ci.yml/dispatches", "", "", `{"inputs":null,"message":"Workflow run has been queued","ref":"main","status":"204 No Content","status_code":204,"workflow_id":"ci.yml","workflow_type":"workflow_file"}`, 204},
		{"actions_run_trigger", "run_workflow", map[string]any{"workflow_id": "2", "ref": "main", "inputs": map[string]any{"flag": true, "nested": []any{nil, 42, map[string]any{"s": "x"}}}}, "/repos/owner/repo/actions/workflows/2/dispatches", "", "", `{"inputs":{"flag":true,"nested":[null,42,{"s":"x"}]},"message":"Workflow run has been queued","ref":"main","status":"204 No Content","status_code":204,"workflow_id":"2","workflow_type":"workflow_id"}`, 204},
		{"actions_run_trigger", "rerun_workflow_run", map[string]any{"run_id": "7.0", "workflow_id": false, "ref": nil}, "/repos/owner/repo/actions/runs/7/rerun", "", "", `{"message":"Workflow run has been queued for re-run","run_id":7,"status":"201 Created","status_code":201}`, 201},
		{"actions_run_trigger", "rerun_failed_jobs", map[string]any{"run_id": "7e0"}, "/repos/owner/repo/actions/runs/7/rerun-failed-jobs", "", "", `{"message":"Failed jobs have been queued for re-run","run_id":7,"status":"201 Created","status_code":201}`, 201},
		{"actions_run_trigger", "cancel_workflow_run", map[string]any{"run_id": 7}, "/repos/owner/repo/actions/runs/7/cancel", "", `{}`, `{"message":"Workflow run has been cancelled","run_id":7,"status":"202 Accepted","status_code":202}`, 202},
		{"actions_run_trigger", "delete_workflow_run_logs", map[string]any{"run_id": 7}, "/repos/owner/repo/actions/runs/7/logs", "", "", `{"message":"Workflow run logs have been deleted","run_id":7,"status":"204 No Content","status_code":204}`, 204},
	}
}

func actionsCaseArgs(tc actionsWireCase) map[string]any {
	args := map[string]any{"owner": "owner", "repo": "repo"}
	if tc.method != "" {
		args["method"] = tc.method
	}
	maps.Copy(args, tc.args)
	return args
}

func actionsCaseDeps(t *testing.T, current **actionsWireCase, apiError bool) BaseDeps {
	t.Helper()
	client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host == "logs.example" {
			w.Header().Set("Location", "https://logs.example/archive")
			w.WriteHeader(http.StatusFound)
			return
		}
		tc := *current
		assert.Equal(t, tc.path, r.URL.Path)
		assert.Equal(t, tc.query, r.URL.RawQuery)
		if apiError {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
			return
		}
		if tc.method == "run_workflow" {
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, tc.args["ref"], payload["ref"])
			assert.JSONEq(t, mustMarshalJSON(t, tc.args["inputs"]), mustMarshalJSON(t, payload["inputs"]))
		}
		if tc.status == 302 {
			w.Header().Set("Location", "https://logs.example/archive")
		}
		w.WriteHeader(tc.status)
		_, _ = w.Write([]byte(tc.body))
	})}}
	return BaseDeps{Client: mustNewGHClient(t, client), ContentWindowSize: 100,
		RepoAccessCache: stubRepoAccessCache(nil, time.Minute)}
}

func assertActionsWireResult(t *testing.T, result *mcp.CallToolResult, schema *jsonschema.Resolved, text string, isError bool, projected ...string) {
	t.Helper()
	require.Equal(t, isError, result.IsError, mustMarshalJSON(t, result))
	require.Len(t, result.Content, 1)
	if schema == nil || isError {
		assert.Equal(t, text, getTextResult(t, result).Text)
		assert.Nil(t, result.StructuredContent)
		return
	}
	if text != "null" {
		require.NotNil(t, result.StructuredContent)
	}
	var output any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
	require.NoError(t, schema.Validate(output))
	expected := text
	if len(projected) != 0 {
		expected = projected[0]
	}
	assert.JSONEq(t, expected, mustMarshalJSON(t, output))
	assert.Equal(t, mustMarshalJSON(t, output), getTextResult(t, result).Text)
}

func TestTypedActionsWireOutputs(t *testing.T) {
	cases := append(actionsWireCases(),
		actionsWireCase{name: "actions_list", method: "list_workflows", path: "/repos/owner/repo/actions/workflows", query: "page=1&per_page=30", body: "null", text: "null", status: 200},
		actionsWireCase{name: "actions_list", method: "list_workflows", path: "/repos/owner/repo/actions/workflows", query: "page=1&per_page=30", body: `{"total_count":0,"workflows":[]}`, text: `{"total_count":0}`, status: 200},
		actionsWireCase{name: "actions_list", method: "list_workflow_run_artifacts", args: map[string]any{"resource_id": "7"}, path: "/repos/owner/repo/actions/runs/7/artifacts", query: "page=1&per_page=30", body: "null", text: "null", status: 200},
		actionsWireCase{name: "actions_list", method: "list_workflow_run_artifacts", args: map[string]any{"resource_id": "7"}, path: "/repos/owner/repo/actions/runs/7/artifacts", query: "page=1&per_page=30", body: `{"total_count":0,"artifacts":[]}`, text: `{"total_count":0}`, status: 200},
		actionsWireCase{name: "actions_get", method: "get_workflow_job", args: map[string]any{"resource_id": "8"}, path: "/repos/owner/repo/actions/jobs/8", body: `{}`, text: `{}`, status: 200},
		actionsWireCase{name: "actions_get", method: "get_workflow", args: map[string]any{"resource_id": "ci.yml"}, path: "/repos/owner/repo/actions/workflows/ci.yml", body: `null`, text: `null`, status: 200},
		actionsWireCase{name: "actions_get", method: "get_workflow_run_usage", args: map[string]any{"resource_id": "7"}, path: "/repos/owner/repo/actions/runs/7/timing", body: `null`, text: `null`, status: 200},
		actionsWireCase{name: "actions_list", method: "list_workflows", args: map[string]any{"page": -1, "perPage": 101}, path: "/repos/owner/repo/actions/workflows", query: "page=-1&per_page=101", body: `{}`, text: `{}`, status: 200},
		actionsWireCase{name: "actions_list", method: "list_workflows", args: map[string]any{"page": 1, "perPage": -1}, path: "/repos/owner/repo/actions/workflows", query: "page=1&per_page=-1", body: `{}`, text: `{}`, status: 200},
	)
	for _, protocol := range typedActionsProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			var current *actionsWireCase
			session, schemas := actionsTypedSession(t, actionsCaseDeps(t, &current, false), protocol)
			for _, tc := range cases {
				t.Run(tc.name+"/"+tc.method+"/"+mustMarshalJSON(t, tc.args), func(t *testing.T) {
					current = &tc
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: actionsCaseArgs(tc)})
					require.NoError(t, err)
					projected := tc.text
					switch {
					case tc.method == "list_workflow_jobs":
						var legacy ActionsJobsOutput
						require.NoError(t, json.Unmarshal([]byte(tc.text), &legacy))
						projected = mustMarshalJSON(t, legacy.Jobs)
					case tc.method == "get_workflow_job" && tc.body == "{}":
						projected = `{"id":0,"run_id":0,"name":"","status":""}`
					case tc.method == "get_workflow_job" && tc.body != "null":
						projected = `{"id":8,"run_id":0,"name":"","status":"","steps":[{"name":"test","number":1,"status":"","started_at":"2026-01-01T00:00:00Z"}],"labels":["linux"]}`
					case tc.method == "get_workflow_run_usage" && tc.body != "{}" && tc.body != "null":
						projected = `{"billable":[{"runner":"CUSTOM","total_ms":42,"jobs":1,"job_runs":[{"job_id":8,"duration_ms":42}]}],"run_duration_ms":42}`
					}
					projected = actionsProjectedEnvelope(t, tc.method, projected)
					assertActionsWireResult(t, result, schemas[tc.name], tc.text, false, projected)
				})
			}
		})
	}
}

func actionsProjectedEnvelope(t *testing.T, method, projected string) string {
	t.Helper()
	field := map[string]string{
		"list_workflows": "workflows", "list_workflow_runs": "workflow_runs",
		"list_workflow_jobs": "workflow_jobs", "list_workflow_run_artifacts": "artifacts",
		"get_workflow": "workflow", "get_workflow_run": "workflow_run", "get_workflow_job": "workflow_job",
		"get_workflow_run_usage": "usage", "download_workflow_run_artifact": "artifact", "get_workflow_run_logs_url": "logs",
		"run_workflow": "dispatch", "rerun_workflow_run": "rerun", "rerun_failed_jobs": "rerun_failed",
		"cancel_workflow_run": "cancel", "delete_workflow_run_logs": "delete_logs",
	}[method]
	require.NotEmpty(t, field)
	envelope := map[string]any{"method": method}
	if projected != "null" {
		envelope[field] = json.RawMessage(projected)
	}
	return mustMarshalJSON(t, envelope)
}

func TestTypedActionsWireErrors(t *testing.T) {
	cases := []actionsWireCase{
		{name: "actions_list", args: map[string]any{"owner": "", "method": false}, text: "missing required parameter: owner"},
		{name: "actions_list", method: "list_workflows", args: map[string]any{"resource_id": 7}, text: "parameter resource_id is not of type string, is float64"},
		{name: "actions_list", method: "list_workflows", args: map[string]any{"page": "bad"}, text: "parameter page is not a valid number: invalid numeric value: bad"},
		{name: "actions_list", method: "list_workflow_jobs", text: "missing required parameter for method list_workflow_jobs: resource_id"},
		{name: "actions_list", method: "list_workflow_jobs", args: map[string]any{"resource_id": "bad", "workflow_jobs_filter": false}, text: `invalid resource_id, must be an integer for method list_workflow_jobs: strconv.ParseInt: parsing "bad": invalid syntax`},
		{name: "actions_list", method: "list_workflow_runs", args: map[string]any{"workflow_runs_filter": false}, text: "parameter workflow_runs_filter is not of type map[string]interface {}, is bool"},
		{name: "actions_list", method: "list_workflow_jobs", args: map[string]any{"resource_id": "7", "workflow_jobs_filter": nil}, text: "parameter workflow_jobs_filter is not of type map[string]interface {}, is <nil>"},
		{name: "actions_list", method: "list_workflow_jobs", args: map[string]any{"workflow_jobs_filter": false}, text: "missing required parameter for method list_workflow_jobs: resource_id"},
		{name: "actions_list", method: "unknown", args: map[string]any{"resource_id": "7"}, text: "unknown method: unknown"},
		{name: "actions_get", method: "get_workflow", text: "missing required parameter: resource_id"},
		{name: "actions_get", method: "get_workflow_job", args: map[string]any{"resource_id": "7.0"}, text: `invalid resource_id, must be an integer for method get_workflow_job: strconv.ParseInt: parsing "7.0": invalid syntax`},
		{name: "actions_get", method: "unknown", args: map[string]any{"resource_id": "7"}, text: "unknown method: unknown"},
		{name: "actions_run_trigger", method: "run_workflow", args: map[string]any{"workflow_id": false}, text: "workflow_id is required for run_workflow action"},
		{name: "actions_run_trigger", method: "run_workflow", args: map[string]any{"workflow_id": "ci.yml", "ref": false}, text: "ref is required for run_workflow action"},
		{name: "actions_run_trigger", method: "rerun_workflow_run", args: map[string]any{"run_id": "bad"}, text: "missing required parameter: run_id"},
		{name: "actions_run_trigger", method: "run_workflow", args: map[string]any{"inputs": nil}, text: "parameter inputs is not of type map[string]interface {}, is <nil>"},
		{name: "get_job_logs", args: map[string]any{"job_id": 1.5}, text: "parameter job_id is not a valid number: non-integer numeric value: 1.5"},
		{name: "get_job_logs", args: map[string]any{"job_id": "8", "failed_only": nil}, text: "parameter failed_only is not of type bool, is <nil>"},
		{name: "get_job_logs", args: map[string]any{"failed_only": true}, text: "run_id is required when failed_only is true"},
		{name: "get_job_logs", args: map[string]any{"job_id": -1}, text: "Either job_id must be provided for single job logs, or run_id with failed_only=true for failed job logs"},
	}

	for _, protocol := range typedActionsProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			var current *actionsWireCase
			session, schemas := actionsTypedSession(t, actionsCaseDeps(t, &current, false), protocol)
			for _, tc := range cases {
				t.Run(tc.name+"/"+tc.text, func(t *testing.T) {
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: actionsCaseArgs(tc)})
					require.NoError(t, err)
					assertActionsWireResult(t, result, schemas[tc.name], tc.text, true)
				})
			}
			session, _ = actionsTypedSession(t, actionsCaseDeps(t, &current, true), protocol)
			for _, tc := range actionsWireCases() {
				current = &tc
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: actionsCaseArgs(tc)})
				require.NoError(t, err)
				require.True(t, result.IsError)
				require.Len(t, result.Content, 1)
				assert.Contains(t, getTextResult(t, result).Text, "Forbidden")
				assert.Nil(t, result.StructuredContent)
			}
		})
	}
}

func TestTypedActionsClientErrorPrecedence(t *testing.T) {
	for _, protocol := range typedActionsProtocols {
		session, _ := actionsTypedSession(t, failingSearchClientDeps{}, protocol)
		for _, tc := range []actionsWireCase{
			{name: "actions_list", method: "list_workflow_jobs", args: map[string]any{"resource_id": "bad", "workflow_jobs_filter": false}},
			{name: "actions_list", method: "list_workflow_runs", args: map[string]any{"workflow_runs_filter": nil}},
			{name: "actions_get", method: "unknown", args: map[string]any{"resource_id": "bad"}},
			{name: "get_job_logs", args: map[string]any{"failed_only": true}},
			{name: "actions_run_trigger", method: "unknown", args: map[string]any{"run_id": 7}},
		} {
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: actionsCaseArgs(tc)})
			if os.Getenv("ACTIONS_LEGACY_BASELINE") == "true" {
				require.EqualError(t, err, `calling "tools/call": failed to get GitHub client: client unavailable`)
				continue
			}
			require.NoError(t, err)
			assertActionsWireResult(t, result, nil, "failed to get GitHub client: client unavailable", true)
		}
	}
}

func TestTypedActionsIFCAndAnnotations(t *testing.T) {
	for _, protocol := range typedActionsProtocols {
		for _, private := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				for _, tc := range []actionsWireCase{
					{name: "actions_list", method: "list_workflows"},
					{name: "actions_get", method: "get_workflow", args: map[string]any{"resource_id": "ci.yml"}},
					{name: "get_job_logs", args: map[string]any{"run_id": "7e0", "failed_only": true}},
					{name: "actions_run_trigger", method: "delete_workflow_run_logs", args: map[string]any{"run_id": 7}},
				} {
					t.Run(fmt.Sprintf("%s/%s/private=%t/enabled=%t", tc.name, protocol, private, enabled), func(t *testing.T) {
						client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if r.URL.Path == "/repos/owner/repo" {
								_, _ = fmt.Fprintf(w, `{"private":%t}`, private)
								return
							}
							if r.Method == http.MethodDelete {
								w.WriteHeader(http.StatusNoContent)
								return
							}
							_, _ = w.Write([]byte(`{}`))
						})}}
						deps := BaseDeps{Client: mustNewGHClient(t, client),
							RepoAccessCache: stubRepoAccessCache(nil, time.Minute)}
						if enabled {
							deps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
						}
						session, schemas := actionsTypedSession(t, deps, protocol)
						listed, err := session.ListTools(context.Background(), nil)
						require.NoError(t, err)
						for _, tool := range listed.Tools {
							require.NotNil(t, tool.Annotations)
							assert.Equal(t, tool.Name != "actions_run_trigger", tool.Annotations.ReadOnlyHint)
							if tool.Name == "actions_run_trigger" {
								require.NotNil(t, tool.Annotations.DestructiveHint)
								assert.True(t, *tool.Annotations.DestructiveHint)
							}
						}
						result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: actionsCaseArgs(tc)})
						require.NoError(t, err)
						text := getTextResult(t, result).Text
						projected := text
						if tc.method != "" {
							legacy := "{}"
							if tc.method == "delete_workflow_run_logs" {
								legacy = `{"message":"Workflow run logs have been deleted","run_id":7,"status":"204 No Content","status_code":204}`
							}
							projected = actionsProjectedEnvelope(t, tc.method, legacy)
							text = legacy
						}
						assertActionsWireResult(t, result, schemas[tc.name], text, false, projected)
						if !enabled || tc.name == "actions_run_trigger" {
							assert.Nil(t, result.Meta["ifc"])
							return
						}
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
}

func TestTypedActionsMinimalProjection(t *testing.T) {
	tests := []struct {
		actionsWireCase
		raw       any
		projected string
	}{
		{
			actionsWireCase: actionsWireCase{name: "actions_get", method: "get_workflow", args: map[string]any{"resource_id": "ci.yml"}, path: "/repos/owner/repo/actions/workflows/ci.yml", status: 200},
			raw: &github.Workflow{
				ID: new(int64(2)), Name: new("CI"), State: new("active"), Path: new(".github/workflows/ci.yml"),
				NodeID: new("node"), URL: new("https://api.github.com/workflows/2"),
				HTMLURL: new("https://github.com/owner/repo/actions/workflows/ci.yml"), BadgeURL: new("https://badge"),
			},
			projected: `{"id":2,"name":"CI","state":"active","path":".github/workflows/ci.yml","html_url":"https://github.com/owner/repo/actions/workflows/ci.yml"}`,
		},
		{
			actionsWireCase: actionsWireCase{name: "actions_list", method: "list_workflows", path: "/repos/owner/repo/actions/workflows", query: "page=1&per_page=30", status: 200},
			raw: &github.Workflows{TotalCount: new(1), Workflows: []*github.Workflow{{
				ID: new(int64(2)), Name: new("CI"), URL: new("https://api.github.com/workflows/2"), BadgeURL: new("https://badge"),
			}}},
			projected: `{"total_count":1,"workflows":[{"id":2,"name":"CI"}]}`,
		},
		{
			actionsWireCase: actionsWireCase{name: "actions_list", method: "list_workflow_run_artifacts", args: map[string]any{"resource_id": "7"}, path: "/repos/owner/repo/actions/runs/7/artifacts", query: "page=1&per_page=30", status: 200},
			raw: &github.ArtifactList{TotalCount: new(int64(1)), Artifacts: []*github.Artifact{{
				ID: new(int64(9)), Name: new("build"), SizeInBytes: new(int64(0)), Expired: new(false),
				Digest: new("sha256:abc"), NodeID: new("node"), URL: new("https://api.github.com/artifacts/9"),
				ArchiveDownloadURL: new("https://api.github.com/artifacts/9/zip"),
				WorkflowRun:        &github.ArtifactWorkflowRun{ID: new(int64(7)), HeadBranch: new("main"), HeadSHA: new("abc")},
			}}},
			projected: `{"total_count":1,"artifacts":[{"id":9,"name":"build","size_in_bytes":0,"expired":false,"digest":"sha256:abc","workflow_run":{"id":7,"head_branch":"main","head_sha":"abc"}}]}`,
		},
		{
			actionsWireCase: actionsWireCase{name: "actions_get", method: "get_workflow_job", args: map[string]any{"resource_id": "8"}, path: "/repos/owner/repo/actions/jobs/8", status: 200},
			raw: &github.WorkflowJob{
				ID: new(int64(8)), RunID: new(int64(7)), Name: new("test"), Status: new("completed"), Conclusion: new("failure"),
				NodeID: new("node"), URL: new("https://api.github.com/jobs/8"), RunURL: new("https://api.github.com/runs/7"),
				HTMLURL: new("https://github.com/owner/repo/actions/runs/7/job/8"),
			},
			projected: `{"id":8,"run_id":7,"name":"test","status":"completed","conclusion":"failure","html_url":"https://github.com/owner/repo/actions/runs/7/job/8"}`,
		},
		{
			actionsWireCase: actionsWireCase{name: "actions_get", method: "get_workflow_run_usage", args: map[string]any{"resource_id": "7"}, path: "/repos/owner/repo/actions/runs/7/timing", status: 200},
			raw: &github.WorkflowRunUsage{RunDurationMS: new(int64(42)), Billable: &github.WorkflowRunBillMap{
				"CUSTOM":  {TotalMS: new(int64(42)), Jobs: new(1), JobRuns: []*github.WorkflowRunJobRun{{JobID: new(8), DurationMS: new(int64(42))}}},
				"ANOTHER": nil,
			}},
			projected: `{"billable":[{"runner":"ANOTHER"},{"runner":"CUSTOM","total_ms":42,"jobs":1,"job_runs":[{"job_id":8,"duration_ms":42}]}],"run_duration_ms":42}`,
		},
	}
	for _, protocol := range typedActionsProtocols {
		for _, tc := range tests {
			t.Run(protocol+"/"+tc.method, func(t *testing.T) {
				current := tc.actionsWireCase
				current.body = mustMarshalJSON(t, tc.raw)
				current.text = current.body
				currentCase := &current
				session, schemas := actionsTypedSession(t, actionsCaseDeps(t, &currentCase, false), protocol)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: current.name, Arguments: actionsCaseArgs(current)})
				require.NoError(t, err)
				assertActionsWireResult(t, result, schemas[current.name], current.text, false, actionsProjectedEnvelope(t, current.method, tc.projected))
			})
		}
	}
}

func TestActionsConcreteSchemas(t *testing.T) {
	for name, schema := range map[string]*jsonschema.Schema{
		"list": actionsListOutputSchema(), "get": actionsGetOutputSchema(),
		"trigger": actionsRunTriggerOutputSchema(), "logs": actionsJobLogsOutputSchema(),
	} {
		t.Run(name, func(t *testing.T) {
			if name != "logs" {
				assert.Equal(t, "object", schema.Type)
				assert.Empty(t, schema.AnyOf)
				assert.Len(t, schema.OneOf, len(schema.Properties["method"].Enum))
				require.NotEmpty(t, schema.Properties["method"].Enum)
			}
			resolved, err := schema.Resolve(nil)
			require.NoError(t, err)
			if name != "logs" {
				require.Error(t, resolved.Validate(nil))
			}
			for _, raw := range []string{`[]`, `42`, `{"unknown":true}`, `{"id":"wrong"}`, `{"jobs":"wrong"}`, `{"logs_url":false}`, `{"method":"unknown"}`} {
				var invalid any
				require.NoError(t, json.Unmarshal([]byte(raw), &invalid))
				require.Error(t, resolved.Validate(invalid), raw)
			}
		})
	}
	seen := make(map[reflect.Type]bool)
	var check func(reflect.Type)
	check = func(typ reflect.Type) {
		if seen[typ] || typ == reflect.TypeFor[json.RawMessage]() {
			return
		}
		seen[typ] = true
		switch typ.Kind() {
		case reflect.Interface:
			t.Errorf("untyped Actions output: %s", typ)
		case reflect.Map:
			t.Errorf("map Actions output: %s", typ)
		case reflect.Array, reflect.Slice, reflect.Pointer:
			check(typ.Elem())
		case reflect.Struct:
			assert.NotEqual(t, "github.com/google/go-github/v92/github", typ.PkgPath(), "raw GitHub output: %s", typ)
			for field := range typ.Fields() {
				if field.IsExported() {
					check(field.Type)
				}
			}
		}
	}
	check(reflect.TypeFor[ActionsListOutput]())
	check(reflect.TypeFor[ActionsGetOutput]())
	check(reflect.TypeFor[ActionsRunTriggerOutput]())
	check(reflect.TypeFor[ActionsJobLogsOutput]())
}

func TestActionsOutputSchemasCached(t *testing.T) {
	for _, schema := range []func() *jsonschema.Schema{
		actionsListOutputSchema, actionsGetOutputSchema, actionsRunTriggerOutputSchema, actionsJobLogsOutputSchema,
	} {
		assert.Same(t, schema(), schema())
	}
}

func TestActionsAdvertisedInputMetadata(t *testing.T) {
	tr := translations.NullTranslationHelper
	list := ActionsList(tr).Tool.InputSchema.(*jsonschema.Schema)
	get := ActionsGet(tr).Tool.InputSchema.(*jsonschema.Schema)
	trigger := ActionsRunTrigger(tr).Tool.InputSchema.(*jsonschema.Schema)
	assert.Equal(t, []any{"list_workflows", "list_workflow_runs", "list_workflow_jobs", "list_workflow_run_artifacts"}, list.Properties["method"].Enum)
	assert.Equal(t, []any{"get_workflow", "get_workflow_run", "get_workflow_job", "download_workflow_run_artifact", "get_workflow_run_usage", "get_workflow_run_logs_url"}, get.Properties["method"].Enum)
	assert.Equal(t, []any{"run_workflow", "rerun_workflow_run", "rerun_failed_jobs", "cancel_workflow_run", "delete_workflow_run_logs"}, trigger.Properties["method"].Enum)
	runs := list.Properties["workflow_runs_filter"]
	assert.Equal(t, "object", runs.Type)
	assert.Empty(t, runs.AnyOf)
	assert.Len(t, runs.Properties["event"].Enum, 32)
	assert.Equal(t, []any{"queued", "in_progress", "completed", "requested", "waiting"}, runs.Properties["status"].Enum)
	assert.Equal(t, []any{"latest", "all"}, list.Properties["workflow_jobs_filter"].Properties["filter"].Enum)
	require.NotNil(t, list.Properties["page"].Minimum)
	require.NotNil(t, list.Properties["perPage"].Minimum)
	require.NotNil(t, list.Properties["perPage"].Maximum)
	assert.Equal(t, 1.0, *list.Properties["page"].Minimum)
	assert.Equal(t, 1.0, *list.Properties["perPage"].Minimum)
	assert.Equal(t, 100.0, *list.Properties["perPage"].Maximum)
	assert.Equal(t, "The action to perform", list.Properties["method"].Description)
}

func TestActionsOutputEnumValues(t *testing.T) {
	get := actionsGetOutputSchema()
	assert.Equal(t, []any{"active", "deleted", "disabled_fork", "disabled_inactivity", "disabled_manually"}, get.Properties["workflow"].Properties["state"].Enum)
	assert.Equal(t, inventory.EnumSchema(append(WorkflowStatusValues(), "")...).Enum, get.Properties["workflow_job"].Properties["status"].Enum)
	assert.Equal(t, inventory.EnumSchema(WorkflowConclusionValues()...).Enum, get.Properties["workflow_run"].Properties["conclusion"].Enum)
	for _, name := range []string{"workflow", "workflow_run", "workflow_job"} {
		schema := get.Properties[name]
		assert.Equal(t, "date-time", schema.Properties["created_at"].Format)
		assert.Equal(t, "Timestamp in RFC3339 format.", schema.Properties["created_at"].Description)
	}
	usage := get.Properties["usage"]
	assert.Equal(t, "Duration in milliseconds.", usage.Properties["run_duration_ms"].Description)
	assert.Equal(t, "Duration in milliseconds.", usage.Properties["billable"].Items.Properties["total_ms"].Description)
}

func TestTypedActionsJobLogWireOutputs(t *testing.T) {
	logServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("first\nsecond\nthird"))
	}))
	defer logServer.Close()
	const note = "The logs_url provides a download link for the individual job logs in plain text format. Use return_content=true to get the actual log content."
	for _, protocol := range typedActionsProtocols {
		for _, outcome := range []string{"single", "failed", "partial", "none"} {
			for _, content := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/content=%t", protocol, outcome, content), func(t *testing.T) {
					client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						switch {
						case strings.HasSuffix(r.URL.Path, "/jobs"):
							body := `{"total_count":2,"jobs":[{"id":8,"name":"test","conclusion":"failure"},{"id":9,"name":"build","conclusion":"failure"}]}`
							if outcome == "none" {
								body = `{"total_count":1,"jobs":[{"id":8,"conclusion":"success"}]}`
							}
							_, _ = w.Write([]byte(body))
						case outcome == "partial" && strings.HasSuffix(r.URL.Path, "/9/logs"):
							w.WriteHeader(403)
							_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
						default:
							w.Header().Set("Location", logServer.URL)
							w.WriteHeader(302)
						}
					})}}
					deps := BaseDeps{Client: mustNewGHClient(t, client), ContentWindowSize: 100,
						RepoAccessCache: stubRepoAccessCache(nil, time.Minute)}
					session, schemas := actionsTypedSession(t, deps, protocol)
					args := map[string]any{"owner": "owner", "repo": "repo", "return_content": content, "tail_lines": "2.0"}
					if outcome == "single" {
						args["job_id"] = "8e0"
					} else {
						args["run_id"], args["failed_only"] = "7", true
					}
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_job_logs", Arguments: args})
					require.NoError(t, err)
					text := getTextResult(t, result).Text
					assertActionsWireResult(t, result, schemas["get_job_logs"], text, false)
					var payload map[string]any
					require.NoError(t, json.Unmarshal([]byte(text), &payload))
					switch {
					case outcome == "none":
						assertActionsLogText(t, schemas["get_job_logs"], `{"failed_jobs":0,"message":"No failed jobs found in this workflow run","run_id":7,"total_jobs":1}`, text)
					case outcome == "single" && content:
						assertActionsLogText(t, schemas["get_job_logs"], `{"job_id":8,"logs_content":"second\nthird","message":"Job logs content retrieved successfully","original_length":3}`, text)
					case outcome == "single":
						assertActionsLogText(t, schemas["get_job_logs"], fmt.Sprintf(`{"job_id":8,"logs_url":%q,"message":"Job logs are available for download","note":%q}`, logServer.URL, note), text)
					default:
						logs := payload["logs"].([]any)
						require.Len(t, logs, 2)
						if outcome == "partial" {
							assert.Contains(t, logs[1].(map[string]any)["error"], "Forbidden")
						}

						require.Equal(t, float64(2), payload["failed_jobs"])
						require.Equal(t, "test", logs[0].(map[string]any)["job_name"])
					}
				})
			}
		}
	}
}

func assertActionsLogText(t *testing.T, schema *jsonschema.Resolved, expected, actual string) {
	t.Helper()
	if schema == nil {
		require.Equal(t, expected, actual)
	} else {
		require.JSONEq(t, expected, actual)
	}
}
