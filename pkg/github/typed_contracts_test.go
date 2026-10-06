package github

import (
	"encoding/json"
	"testing"

	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsOutputMethodContracts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		schema   *jsonschema.Schema
		method   string
		field    string
		other    string
		value    string
		optional bool
	}{
		{"list/workflows", actionsListOutputSchema(), "list_workflows", "workflows", "artifacts", `{}`, true},
		{"list/runs", actionsListOutputSchema(), "list_workflow_runs", "workflow_runs", "workflows", `{"total_count":0,"workflow_runs":[]}`, false},
		{"list/jobs", actionsListOutputSchema(), "list_workflow_jobs", "workflow_jobs", "workflows", `{"total_count":0,"jobs":[]}`, false},
		{"list/artifacts", actionsListOutputSchema(), "list_workflow_run_artifacts", "artifacts", "workflows", `{}`, true},
		{"get/workflow", actionsGetOutputSchema(), "get_workflow", "workflow", "workflow_run", `{}`, true},
		{"get/run", actionsGetOutputSchema(), "get_workflow_run", "workflow_run", "workflow", `{"id":7,"name":"","status":"","workflow_id":0,"head_branch":"","run_number":0,"run_attempt":0}`, false},
		{"get/job", actionsGetOutputSchema(), "get_workflow_job", "workflow_job", "workflow", `{"id":8,"run_id":7,"name":"","status":""}`, true},
		{"get/usage", actionsGetOutputSchema(), "get_workflow_run_usage", "usage", "workflow", `{}`, true},
		{"get/artifact", actionsGetOutputSchema(), "download_workflow_run_artifact", "artifact", "workflow", `{"artifact_id":9,"download_url":"","message":"","note":""}`, false},
		{"get/logs", actionsGetOutputSchema(), "get_workflow_run_logs_url", "logs", "workflow", `{"logs_url":"","message":"","note":"","optimization_tip":"","warning":""}`, false},
		{"trigger/dispatch", actionsRunTriggerOutputSchema(), "run_workflow", "dispatch", "cancel", `{"inputs":null,"message":"","ref":"main","status":"","status_code":204,"workflow_id":"ci.yml","workflow_type":"workflow_file"}`, false},
		{"trigger/rerun", actionsRunTriggerOutputSchema(), "rerun_workflow_run", "rerun", "cancel", `{"message":"","run_id":7,"status":"","status_code":201}`, false},
		{"trigger/rerun_failed", actionsRunTriggerOutputSchema(), "rerun_failed_jobs", "rerun_failed", "cancel", `{"message":"","run_id":7,"status":"","status_code":201}`, false},
		{"trigger/cancel", actionsRunTriggerOutputSchema(), "cancel_workflow_run", "cancel", "rerun", `{"message":"","run_id":7,"status":"","status_code":202}`, false},
		{"trigger/delete_logs", actionsRunTriggerOutputSchema(), "delete_workflow_run_logs", "delete_logs", "rerun", `{"message":"","run_id":7,"status":"","status_code":204}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema, err := tc.schema.Resolve(nil)
			require.NoError(t, err)
			valid := []string{`{"method":"` + tc.method + `","` + tc.field + `":` + tc.value + `}`}
			if tc.optional {
				valid = append(valid, `{"method":"`+tc.method+`"}`, `{"method":"`+tc.method+`","`+tc.field+`":null}`)
			}
			for _, raw := range valid {
				var value any
				require.NoError(t, json.Unmarshal([]byte(raw), &value))
				assert.NoError(t, schema.Validate(value), raw)
			}
			invalid := []string{
				`{}`,
				`{"` + tc.field + `":` + tc.value + `}`,
				`{"method":"` + tc.method + `","` + tc.other + `":` + tc.value + `}`,
				`{"method":"` + tc.method + `","` + tc.field + `":` + tc.value + `,"` + tc.other + `":` + tc.value + `}`,
				`{"method":"` + tc.method + `","` + tc.other + `":null}`,
				`{"method":"` + tc.method + `","` + tc.field + `":` + tc.value + `,"` + tc.other + `":null}`,
			}
			if !tc.optional {
				invalid = append(invalid, `{"method":"`+tc.method+`"}`)
			}
			for _, raw := range invalid {
				var value any
				require.NoError(t, json.Unmarshal([]byte(raw), &value))
				assert.Error(t, schema.Validate(value), raw)
			}
		})
	}
}

func TestUIGetOutputRejectsMismatchedPayloads(t *testing.T) {
	schema, err := uiGetOutputSchema().Resolve(nil)
	require.NoError(t, err)
	for _, method := range []string{"labels", "assignees", "milestones", "issue_types", "branches", "issue_fields", "reviewers"} {
		t.Run(method, func(t *testing.T) {
			payload := "labels"
			if method == payload {
				payload = "assignees"
			}
			for _, value := range []map[string]any{
				{"method": method},
				{"method": method, payload: map[string]any{payload: []any{}, "totalCount": 0, "has_more": false}},
			} {
				assert.Error(t, schema.Validate(value), "%v", value)
			}
		})
	}
	assert.Error(t, schema.Validate(map[string]any{
		"method":      "labels",
		"labels":      map[string]any{"labels": []any{}, "totalCount": 0, "has_more": false},
		"issue_types": []any{},
	}))
}

func TestProjectOutputUsersExcludeDetails(t *testing.T) {
	user := &github.User{Login: new("octocat"), Email: new("private@example.com")}
	project := convertToMinimalProject(&github.ProjectV2{Owner: user, Creator: user, DeletedBy: user})
	require.Nil(t, project.Owner.Details)
	require.Nil(t, project.Creator.Details)
	require.Nil(t, project.DeletedBy.Details)
	var status statusUpdateNode
	status.Creator.Login = "octocat"
	require.Nil(t, convertToMinimalStatusUpdate(status).Creator.Details)
	for name, schema := range map[string]*jsonschema.Schema{
		"list": projectsListOutputSchema(), "get": projectsGetOutputSchema(), "write": projectsWriteOutputSchema(),
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotContains(t, mustMarshalJSON(t, schema), `"details"`)
		})
	}
	schema, err := projectOutputSchema[MinimalProject]().Resolve(nil)
	require.NoError(t, err)
	for _, field := range []string{"owner", "creator", "deleted_by"} {
		assert.NoError(t, schema.Validate(map[string]any{field: map[string]any{"login": "octocat"}}))
		assert.Error(t, schema.Validate(map[string]any{field: map[string]any{"login": "octocat", "details": map[string]any{}}}))
	}
}
