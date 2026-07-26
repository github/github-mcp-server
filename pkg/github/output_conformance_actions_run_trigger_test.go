package github

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/pkg/translations"
)

// The end-to-end guarantee for actions_run_trigger: what the real handler
// actually emits must validate against the schema the tool advertises. The
// structured-content mirror publishes the exact bytes of the text block as
// structuredContent, so validating the text block here is equivalent to
// validating what a client receives.
//
// Every value of the tool's `method` enum is exercised; the subtest table is
// cross-checked against the enum at the end so a newly added method cannot slip
// through uncovered.
func TestActionsRunTriggerOutputValidatesAgainstDeclaredSchema(t *testing.T) {
	t.Parallel()

	serverTool := ActionsRunTrigger(translations.NullTranslationHelper)
	resolved := resolveToolSchema(t, actionsRunTriggerOutputSchema)

	// 204 No Content — what POST .../dispatches really returns.
	noContent := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	// 201 Created — what the rerun endpoints really return.
	created := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) }
	// 202 Accepted — what POST .../cancel really returns. go-github surfaces
	// this as an *AcceptedError, which the handler deliberately swallows, so
	// this case also pins that the "error that isn't an error" path still
	// produces schema-conformant output.
	accepted := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) }

	tests := []struct {
		name string
		// method is the enum value under test; used for coverage bookkeeping.
		method      string
		args        map[string]any
		handlers    map[string]http.HandlerFunc
		wantMessage string
		// wantFields are keys the payload must carry, asserted so a handler
		// that silently stopped emitting a required field cannot pass merely
		// because some other anyOf branch happens to accept the remainder.
		wantFields []string
		// wantStatusCode pins the HTTP status the handler echoes back. Note
		// that the sibling `status` string cannot be pinned here: the mock
		// transport builds an http.Response without a Status line, so
		// resp.Status is "" under test where production would say
		// "204 No Content". The schema only constrains it to a string, so
		// both validate.
		wantStatusCode int
	}{
		{
			name:   "run_workflow",
			method: actionsMethodRunWorkflow,
			args: map[string]any{
				"workflow_id": "12345",
				"ref":         "main",
				"inputs":      map[string]any{"FIELD1": "value1"},
			},
			handlers: map[string]http.HandlerFunc{
				PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowID: noContent,
			},
			wantMessage:    "Workflow run has been queued",
			wantFields:     []string{"message", "workflow_type", "workflow_id", "ref", "inputs", "status", "status_code"},
			wantStatusCode: http.StatusNoContent,
		},
		{
			// The other addressing mode: a workflow file name rather than a
			// numeric ID, which flips workflow_type to "workflow_file". The
			// schema pins that enum, so both values need exercising.
			name:   "run_workflow by file name",
			method: actionsMethodRunWorkflow,
			args: map[string]any{
				"workflow_id": "ci.yaml",
				"ref":         "refs/heads/main",
				"inputs":      map[string]any{},
			},
			handlers: map[string]http.HandlerFunc{
				PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowID: noContent,
			},
			wantMessage:    "Workflow run has been queued",
			wantFields:     []string{"message", "workflow_type", "workflow_id", "ref", "inputs", "status", "status_code"},
			wantStatusCode: http.StatusNoContent,
		},
		{
			// The sparse case: every optional argument absent. `inputs` is a
			// nil map here, which marshals to a literal JSON null while still
			// being a REQUIRED property of the run_workflow branch — exactly
			// the combination that exercises the branch's required set rather
			// than a convenient fixture.
			name:   "run_workflow with all optionals absent",
			method: actionsMethodRunWorkflow,
			args: map[string]any{
				"workflow_id": "12345",
				"ref":         "main",
			},
			handlers: map[string]http.HandlerFunc{
				PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowID: noContent,
			},
			wantMessage:    "Workflow run has been queued",
			wantFields:     []string{"message", "workflow_type", "workflow_id", "ref", "inputs", "status", "status_code"},
			wantStatusCode: http.StatusNoContent,
		},
		{
			name:     "rerun_workflow_run",
			method:   actionsMethodRerunWorkflowRun,
			args:     map[string]any{"run_id": float64(12345)},
			handlers: map[string]http.HandlerFunc{PostReposActionsRunsRerunByOwnerByRepoByRunID: created},
			// All four acknowledgement methods share a branch; only the text
			// differs, so pinning the text is what proves the right code path
			// ran rather than a neighbouring one.
			wantMessage:    "Workflow run has been queued for re-run",
			wantFields:     []string{"message", "run_id", "status", "status_code"},
			wantStatusCode: http.StatusCreated,
		},
		{
			name:           "rerun_failed_jobs",
			method:         actionsMethodRerunFailedJobs,
			args:           map[string]any{"run_id": float64(12345)},
			handlers:       map[string]http.HandlerFunc{PostReposActionsRunsRerunFailedJobsByOwnerByRepoByRunID: created},
			wantMessage:    "Failed jobs have been queued for re-run",
			wantFields:     []string{"message", "run_id", "status", "status_code"},
			wantStatusCode: http.StatusCreated,
		},
		{
			name:           "cancel_workflow_run",
			method:         actionsMethodCancelWorkflowRun,
			args:           map[string]any{"run_id": float64(12345)},
			handlers:       map[string]http.HandlerFunc{PostReposActionsRunsCancelByOwnerByRepoByRunID: accepted},
			wantMessage:    "Workflow run has been cancelled",
			wantFields:     []string{"message", "run_id", "status", "status_code"},
			wantStatusCode: http.StatusAccepted,
		},
		{
			name:           "delete_workflow_run_logs",
			method:         actionsMethodDeleteWorkflowRunLogs,
			args:           map[string]any{"run_id": float64(12345)},
			handlers:       map[string]http.HandlerFunc{DeleteReposActionsRunsLogsByOwnerByRepoByRunID: noContent},
			wantMessage:    "Workflow run logs have been deleted",
			wantFields:     []string{"message", "run_id", "status", "status_code"},
			wantStatusCode: http.StatusNoContent,
		},
	}

	covered := map[string]bool{}

	for _, tt := range tests {
		covered[tt.method] = true

		t.Run(tt.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(tt.handlers))}
			handler := serverTool.Handler(deps)

			args := map[string]any{
				"method": tt.method,
				"owner":  "octocat",
				"repo":   "repo",
			}
			maps.Copy(args, tt.args)

			request := createMCPRequest(args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError, "handler should succeed; a failing handler must not masquerade as conformance")

			text := getTextResult(t, result)

			// Unmarshal before validating: the mirror can only publish
			// structuredContent if the text block is JSON in the first place.
			var payload any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &payload),
				"handler output must be JSON for the mirror to publish it as structuredContent")

			obj, ok := payload.(map[string]any)
			require.True(t, ok, "actions_run_trigger declares an object root; got %T", payload)
			for _, field := range tt.wantFields {
				assert.Contains(t, obj, field,
					"method=%s must emit %q, which the schema branch lists as required", tt.method, field)
			}
			assert.Equal(t, tt.wantMessage, obj["message"],
				"payload must come from the handler for method=%s, not a neighbouring code path", tt.method)
			assert.Equal(t, float64(tt.wantStatusCode), obj["status_code"],
				"status_code must echo the upstream HTTP status for method=%s", tt.method)

			require.NoError(t, resolved.Validate(payload),
				"real handler output for method=%s must conform to the advertised outputSchema", tt.method)
		})
	}

	// Coverage is only meaningful if it tracks the enum. A method added to the
	// tool without a case here would otherwise ship unvalidated.
	t.Run("every method enum value is covered", func(t *testing.T) {
		inputSchema, ok := serverTool.Tool.InputSchema.(*jsonschema.Schema)
		require.True(t, ok, "expected a *jsonschema.Schema input schema")
		methodSchema, ok := inputSchema.Properties["method"]
		require.True(t, ok, "actions_run_trigger must declare a method property")
		require.NotEmpty(t, methodSchema.Enum, "method must be a closed enum")

		for _, v := range methodSchema.Enum {
			name, ok := v.(string)
			require.True(t, ok, "method enum values must be strings, got %T", v)
			assert.True(t, covered[name],
				"method %q is advertised in the enum but its real output is never validated against the schema", name)
		}

		// And nothing here tests a method the tool no longer advertises.
		for name := range covered {
			assert.Contains(t, methodSchema.Enum, any(name),
				"this test exercises method %q, which the tool no longer advertises", name)
		}
	})
}
