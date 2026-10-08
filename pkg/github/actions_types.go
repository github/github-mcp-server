package github

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ActionsListMethod string
type ActionsGetMethod string
type ActionsRunTriggerMethod string
type ActionsWorkflowState string
type ActionsWorkflowType string

var actionsEnumSchemas = sync.OnceValue(func() map[reflect.Type]*jsonschema.Schema {
	return map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[ActionsListMethod](): {Type: "string", Enum: []any{
			actionsMethodListWorkflows, actionsMethodListWorkflowRuns,
			actionsMethodListWorkflowJobs, actionsMethodListWorkflowArtifacts,
		}},
		reflect.TypeFor[ActionsGetMethod](): {Type: "string", Enum: []any{
			actionsMethodGetWorkflow, actionsMethodGetWorkflowRun, actionsMethodGetWorkflowJob,
			actionsMethodDownloadWorkflowArtifact, actionsMethodGetWorkflowRunUsage, actionsMethodGetWorkflowRunLogsURL,
		}},
		reflect.TypeFor[ActionsRunTriggerMethod](): {Type: "string", Enum: []any{
			actionsMethodRunWorkflow, actionsMethodRerunWorkflowRun, actionsMethodRerunFailedJobs,
			actionsMethodCancelWorkflowRun, actionsMethodDeleteWorkflowRunLogs,
		}},
		reflect.TypeFor[ActionsWorkflowState](): {Type: "string", Enum: []any{
			"active", "deleted", "disabled_fork", "disabled_inactivity", "disabled_manually",
		}},
		reflect.TypeFor[ActionsWorkflowType](): {Type: "string", Enum: []any{"workflow_file", "workflow_id"}},
	}
})

type ActionsListInput struct {
	Method             string                    `json:"method"`
	Owner              string                    `json:"owner"`
	Repo               string                    `json:"repo"`
	ResourceID         string                    `json:"resource_id,omitempty"`
	Page               int                       `json:"page,omitempty"`
	PerPage            int                       `json:"perPage,omitempty"`
	WorkflowRunsFilter ActionsWorkflowRunsFilter `json:"workflow_runs_filter"`
	WorkflowJobsFilter ActionsWorkflowJobsFilter `json:"workflow_jobs_filter"`
}

type ActionsWorkflowRunsFilter struct {
	Actor           string `json:"actor,omitempty"`
	Branch          string `json:"branch,omitempty"`
	Event           string `json:"event,omitempty"`
	Status          string `json:"status,omitempty"`
	validationError string
}

type ActionsWorkflowJobsFilter struct {
	Filter          string `json:"filter,omitempty"`
	validationError string
}

// The legacy handler validates filters only after client acquisition and
// resource-ID validation. Retain that ordering even for non-object filters.
func (filter *ActionsWorkflowRunsFilter) UnmarshalJSON(raw []byte) error {
	type fields ActionsWorkflowRunsFilter
	*filter = ActionsWorkflowRunsFilter{}
	message, err := decodeActionsFilter(raw, "workflow_runs_filter", (*fields)(filter))
	filter.validationError = message
	return err
}

func (filter *ActionsWorkflowJobsFilter) UnmarshalJSON(raw []byte) error {
	type fields ActionsWorkflowJobsFilter
	*filter = ActionsWorkflowJobsFilter{}
	message, err := decodeActionsFilter(raw, "workflow_jobs_filter", (*fields)(filter))
	filter.validationError = message
	return err
}

func decodeActionsFilter(raw []byte, field string, output any) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	if _, err := OptionalParam[map[string]any](map[string]any{field: value}, field); err != nil {
		return err.Error(), nil
	}
	return "", json.Unmarshal(raw, output)
}

func newActionsTool[In, Out any](
	toolset inventory.ToolsetMetadata,
	tool mcp.Tool,
	scopeAccess inventory.ScopeAccess,
	handler func(context.Context, ToolDependencies, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error),
	normalizers ...inventory.InputNormalizer,
) inventory.ServerTool {
	cached, _ := actionsValidationSchemas.LoadOrStore(tool.Name, sync.OnceValue(func() *jsonschema.Schema {
		return actionsValidationSchema(tool.InputSchema)
	}))
	validation := cached.(func() *jsonschema.Schema)()
	return NewToolWithSchemaOptions(toolset, tool, scopeAccess,
		inventory.TypedSchemaOptions{ValidationInputSchema: validation}, handler, normalizers...)
}

var actionsValidationSchemas sync.Map

func actionsValidationSchema(inputSchema any) *jsonschema.Schema {
	raw, err := json.Marshal(inputSchema)
	if err != nil {
		panic(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		panic(err)
	}
	// The legacy raw handler treated enums and pagination bounds as metadata.
	// Relax only SDK validation; leave the advertised contract unchanged.
	for _, field := range []string{"method", "workflow_runs_filter", "workflow_jobs_filter"} {
		property := schema.Properties[field]
		if property == nil {
			continue
		}
		if field == "method" {
			property.Enum = nil
			continue
		}
		for _, nested := range property.Properties {
			nested.Enum = nil
		}
		schema.Properties[field] = &jsonschema.Schema{
			Description: property.Description,
			AnyOf: []*jsonschema.Schema{
				property,
				{Not: &jsonschema.Schema{Type: "object"}},
			},
		}
	}
	for _, field := range []string{"page", "perPage"} {
		if property := schema.Properties[field]; property != nil {
			property.Minimum, property.Maximum = nil, nil
		}
	}
	return &schema
}

type ActionsGetInput struct {
	Method     string `json:"method"`
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	ResourceID string `json:"resource_id"`
}

type ActionsRunTriggerInput struct {
	Method     string         `json:"method"`
	Owner      string         `json:"owner"`
	Repo       string         `json:"repo"`
	WorkflowID string         `json:"workflow_id,omitempty"`
	Ref        string         `json:"ref,omitempty"`
	RunID      int            `json:"run_id,omitempty"`
	Inputs     map[string]any `json:"inputs,omitempty"`
}

type ActionsGetJobLogsInput struct {
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	JobID         int    `json:"job_id,omitempty"`
	RunID         int    `json:"run_id,omitempty"`
	FailedOnly    bool   `json:"failed_only,omitempty"`
	ReturnContent bool   `json:"return_content,omitempty"`
	TailLines     int    `json:"tail_lines,omitempty"`
}

// Method is omitted only on SDK-validated error zero values, which the shared
// output middleware removes. Every successful handler sets its discriminant.
type ActionsListOutput struct {
	Method    ActionsListMethod          `json:"method,omitempty"`
	Workflows *ActionsWorkflowsOutput    `json:"workflows,omitempty"`
	Runs      *MinimalWorkflowRunsResult `json:"workflow_runs,omitempty"`
	Jobs      *MinimalWorkflowJobsResult `json:"workflow_jobs,omitempty"`
	Artifacts *ActionsArtifactsOutput    `json:"artifacts,omitempty"`
}

type ActionsWorkflow struct {
	ID        *int64                `json:"id,omitempty"`
	Name      *string               `json:"name,omitempty"`
	Path      *string               `json:"path,omitempty"`
	State     *ActionsWorkflowState `json:"state,omitempty"`
	HTMLURL   *string               `json:"html_url,omitempty"`
	CreatedAt string                `json:"created_at,omitempty"`
	UpdatedAt string                `json:"updated_at,omitempty"`
}

type ActionsWorkflowsOutput struct {
	TotalCount *int               `json:"total_count,omitempty"`
	Workflows  []*ActionsWorkflow `json:"workflows,omitempty"`
}

type ActionsArtifact struct {
	ID          *int64                      `json:"id,omitempty"`
	Name        *string                     `json:"name,omitempty"`
	SizeInBytes *int64                      `json:"size_in_bytes,omitempty"`
	Expired     *bool                       `json:"expired,omitempty"`
	Digest      *string                     `json:"digest,omitempty"`
	CreatedAt   string                      `json:"created_at,omitempty"`
	UpdatedAt   string                      `json:"updated_at,omitempty"`
	ExpiresAt   string                      `json:"expires_at,omitempty"`
	WorkflowRun *ActionsArtifactWorkflowRun `json:"workflow_run,omitempty"`
}

type ActionsArtifactWorkflowRun struct {
	ID         *int64  `json:"id,omitempty"`
	HeadBranch *string `json:"head_branch,omitempty"`
	HeadSHA    *string `json:"head_sha,omitempty"`
}

type ActionsArtifactsOutput struct {
	TotalCount *int64             `json:"total_count,omitempty"`
	Artifacts  []*ActionsArtifact `json:"artifacts,omitempty"`
}

type ActionsJobsOutput struct {
	Jobs MinimalWorkflowJobsResult `json:"jobs"`
}

var actionsListOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	schema := actionsOutputSchema[ActionsListOutput]()
	constrainMethodOutput(schema, map[string]string{
		actionsMethodListWorkflows: "workflows", actionsMethodListWorkflowRuns: "workflow_runs",
		actionsMethodListWorkflowJobs: "workflow_jobs", actionsMethodListWorkflowArtifacts: "artifacts",
	}, "workflows", "artifacts")
	return schema
})

type ActionsGetOutput struct {
	Method   ActionsGetMethod               `json:"method,omitempty"`
	Workflow *ActionsWorkflow               `json:"workflow,omitempty"`
	Run      *MinimalWorkflowRun            `json:"workflow_run,omitempty"`
	Job      *MinimalWorkflowJob            `json:"workflow_job,omitempty"`
	Usage    *ActionsRunUsageOutput         `json:"usage,omitempty"`
	Artifact *ActionsArtifactDownloadOutput `json:"artifact,omitempty"`
	Logs     *ActionsRunLogsOutput          `json:"logs,omitempty"`
}

type ActionsRunUsageOutput struct {
	Billable      []ActionsRunnerUsage `json:"billable,omitempty"`
	RunDurationMS *int64               `json:"run_duration_ms,omitempty"`
}

type ActionsRunnerUsage struct {
	Runner  string               `json:"runner"`
	TotalMS *int64               `json:"total_ms,omitempty"`
	Jobs    *int                 `json:"jobs,omitempty"`
	JobRuns []ActionsJobRunUsage `json:"job_runs,omitempty"`
}

type ActionsJobRunUsage struct {
	JobID      *int   `json:"job_id,omitempty"`
	DurationMS *int64 `json:"duration_ms,omitempty"`
}

func convertToActionsWorkflow(workflow *github.Workflow) *ActionsWorkflow {
	if workflow == nil {
		return nil
	}
	var state *ActionsWorkflowState
	if workflow.State != nil {
		state = new(ActionsWorkflowState(*workflow.State))
	}
	return &ActionsWorkflow{
		ID: workflow.ID, Name: workflow.Name, Path: workflow.Path, State: state,
		HTMLURL: workflow.HTMLURL, CreatedAt: formatMinimalTimestamp(workflow.CreatedAt),
		UpdatedAt: formatMinimalTimestamp(workflow.UpdatedAt),
	}
}

func convertToActionsWorkflows(workflows *github.Workflows) *ActionsWorkflowsOutput {
	if workflows == nil {
		return nil
	}
	out := &ActionsWorkflowsOutput{TotalCount: workflows.TotalCount}
	for _, workflow := range workflows.Workflows {
		out.Workflows = append(out.Workflows, convertToActionsWorkflow(workflow))
	}
	return out
}

func convertToActionsArtifacts(artifacts *github.ArtifactList) *ActionsArtifactsOutput {
	if artifacts == nil {
		return nil
	}
	out := &ActionsArtifactsOutput{TotalCount: artifacts.TotalCount}
	for _, artifact := range artifacts.Artifacts {
		if artifact == nil {
			out.Artifacts = append(out.Artifacts, nil)
			continue
		}
		item := &ActionsArtifact{
			ID: artifact.ID, Name: artifact.Name, SizeInBytes: artifact.SizeInBytes,
			Expired: artifact.Expired, Digest: artifact.Digest,
			CreatedAt: formatMinimalTimestamp(artifact.CreatedAt),
			UpdatedAt: formatMinimalTimestamp(artifact.UpdatedAt),
			ExpiresAt: formatMinimalTimestamp(artifact.ExpiresAt),
		}
		if run := artifact.WorkflowRun; run != nil {
			item.WorkflowRun = &ActionsArtifactWorkflowRun{
				ID: run.ID, HeadBranch: run.HeadBranch, HeadSHA: run.HeadSHA,
			}
		}
		out.Artifacts = append(out.Artifacts, item)
	}
	return out
}

func convertToActionsRunUsage(usage *github.WorkflowRunUsage) *ActionsRunUsageOutput {
	if usage == nil {
		return nil
	}
	out := &ActionsRunUsageOutput{RunDurationMS: usage.RunDurationMS}
	if usage.Billable == nil {
		return out
	}
	for runner, bill := range *usage.Billable {
		item := ActionsRunnerUsage{Runner: runner}
		if bill != nil {
			item.TotalMS, item.Jobs = bill.TotalMS, bill.Jobs
			for _, job := range bill.JobRuns {
				if job != nil {
					item.JobRuns = append(item.JobRuns, ActionsJobRunUsage{
						JobID: job.JobID, DurationMS: job.DurationMS,
					})
				}
			}
		}
		out.Billable = append(out.Billable, item)
	}
	slices.SortFunc(out.Billable, func(a, b ActionsRunnerUsage) int {
		return strings.Compare(a.Runner, b.Runner)
	})
	return out
}

type ActionsArtifactDownloadOutput struct {
	ArtifactID  int64  `json:"artifact_id"`
	DownloadURL string `json:"download_url"`
	Message     string `json:"message"`
	Note        string `json:"note"`
}

type ActionsRunLogsOutput struct {
	LogsURL         string `json:"logs_url"`
	Message         string `json:"message"`
	Note            string `json:"note"`
	OptimizationTip string `json:"optimization_tip"`
	Warning         string `json:"warning"`
}

var actionsGetOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	schema := actionsOutputSchema[ActionsGetOutput]()
	constrainMethodOutput(schema, map[string]string{
		actionsMethodGetWorkflow: "workflow", actionsMethodGetWorkflowRun: "workflow_run",
		actionsMethodGetWorkflowJob: "workflow_job", actionsMethodGetWorkflowRunUsage: "usage",
		actionsMethodDownloadWorkflowArtifact: "artifact", actionsMethodGetWorkflowRunLogsURL: "logs",
	}, "workflow", "workflow_job", "usage")
	return schema
})

type ActionsRunTriggerOutput struct {
	Method      ActionsRunTriggerMethod    `json:"method,omitempty"`
	Dispatch    *ActionsDispatchOutput     `json:"dispatch,omitempty"`
	Rerun       *ActionsRunOperationOutput `json:"rerun,omitempty"`
	RerunFailed *ActionsRunOperationOutput `json:"rerun_failed,omitempty"`
	Cancel      *ActionsRunOperationOutput `json:"cancel,omitempty"`
	DeleteLogs  *ActionsRunOperationOutput `json:"delete_logs,omitempty"`
}

// Workflow dispatch inputs are arbitrary user JSON, echoed verbatim by the
// legacy response. This is the only open-ended output field.
type ActionsDispatchOutput struct {
	Inputs       json.RawMessage     `json:"inputs"`
	Message      string              `json:"message"`
	Ref          string              `json:"ref"`
	Status       string              `json:"status"`
	StatusCode   int                 `json:"status_code"`
	WorkflowID   string              `json:"workflow_id"`
	WorkflowType ActionsWorkflowType `json:"workflow_type"`
}

type ActionsRunOperationOutput struct {
	Message    string `json:"message"`
	RunID      int64  `json:"run_id"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
}

var actionsRunTriggerOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	schema := actionsOutputSchema[ActionsRunTriggerOutput]()
	dispatch := schema.Properties["dispatch"]
	dispatch.Properties["inputs"] = &jsonschema.Schema{
		Types:                []string{"object", "null"},
		AdditionalProperties: &jsonschema.Schema{},
	}
	schema.Defs["run_operation"] = actionsOutputSchema[ActionsRunOperationOutput]()
	for _, field := range []string{"rerun", "rerun_failed", "cancel", "delete_logs"} {
		schema.Properties[field] = &jsonschema.Schema{Ref: "#/$defs/run_operation"}
	}
	constrainMethodOutput(schema, map[string]string{
		actionsMethodRunWorkflow: "dispatch", actionsMethodRerunWorkflowRun: "rerun",
		actionsMethodRerunFailedJobs: "rerun_failed", actionsMethodCancelWorkflowRun: "cancel",
		actionsMethodDeleteWorkflowRunLogs: "delete_logs",
	})
	return schema
})

type ActionsJobLogsOutput struct {
	Single *ActionsJobLog
	Failed *ActionsFailedJobLogsOutput
}

type ActionsJobLog struct {
	Content *ActionsJobLogContent
	URL     *ActionsJobLogURL
	Error   *ActionsJobLogError
}

type ActionsJobLogContent struct {
	JobID          int64  `json:"job_id"`
	JobName        string `json:"job_name,omitempty"`
	LogsContent    string `json:"logs_content"`
	Message        string `json:"message"`
	OriginalLength int    `json:"original_length"`
}

type ActionsJobLogURL struct {
	JobID   int64  `json:"job_id"`
	JobName string `json:"job_name,omitempty"`
	LogsURL string `json:"logs_url"`
	Message string `json:"message"`
	Note    string `json:"note"`
}

type ActionsJobLogError struct {
	Error   string `json:"error"`
	JobID   int64  `json:"job_id"`
	JobName string `json:"job_name"`
}

func (out ActionsJobLog) MarshalJSON() ([]byte, error) {
	switch {
	case out.Content != nil:
		return json.Marshal(out.Content)
	case out.URL != nil:
		return json.Marshal(out.URL)
	default:
		return json.Marshal(out.Error)
	}
}

type ActionsFailedJobLogsOutput struct {
	FailedJobs   int                      `json:"failed_jobs"`
	Logs         *[]ActionsJobLog         `json:"logs,omitempty"`
	Message      string                   `json:"message"`
	ReturnFormat *ActionsLogsReturnFormat `json:"return_format,omitempty"`
	RunID        int64                    `json:"run_id"`
	TotalJobs    int                      `json:"total_jobs"`
}

type ActionsLogsReturnFormat struct {
	Content bool `json:"content"`
	URLs    bool `json:"urls"`
}

func (out ActionsJobLogsOutput) MarshalJSON() ([]byte, error) {
	if out.Single != nil {
		return json.Marshal(out.Single)
	}
	return json.Marshal(out.Failed)
}

func actionsJobLogSchema() *jsonschema.Schema {
	return repositoryUnionSchema(
		actionsOutputSchema[ActionsJobLogContent](),
		actionsOutputSchema[ActionsJobLogURL](),
		actionsOutputSchema[ActionsJobLogError](),
	)
}

var actionsJobLogsOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	failed := actionsOutputSchema[ActionsFailedJobLogsOutput]()
	failed.Properties["logs"].Items = actionsJobLogSchema()
	return repositoryUnionSchema(
		&jsonschema.Schema{Type: "null"},
		actionsOutputSchema[ActionsJobLogContent](),
		actionsOutputSchema[ActionsJobLogURL](),
		failed,
	)
})

func actionsOutputSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](&jsonschema.ForOptions{TypeSchemas: actionsEnumSchemas()})
	if err != nil {
		panic(err)
	}
	if schema.Defs == nil {
		schema.Defs = make(map[string]*jsonschema.Schema)
	}
	describeActionsOutput(schema)
	return schema
}

func describeActionsOutput(schema *jsonschema.Schema) {
	if schema == nil {
		return
	}
	for name, property := range schema.Properties {
		switch {
		case strings.HasSuffix(name, "_at"):
			property.Description = "Timestamp in RFC3339 format."
			property.Format = "date-time"
		case strings.HasSuffix(name, "_ms"):
			property.Description = "Duration in milliseconds."
		case name == "size_in_bytes":
			property.Description = "Artifact size in bytes."
		case name == "original_length":
			property.Description = "Number of log lines before truncation."
		case name == "state":
			property.Description = "Workflow state, e.g. active, deleted, disabled_fork, disabled_inactivity, disabled_manually."
		case name == "conclusion":
			property.Description = "Completion outcome, e.g. success, failure, cancelled, skipped, timed_out, action_required."
			property.Enum = inventory.EnumSchema(WorkflowConclusionValues()...).Enum
		case name == "status" && schema.Properties["status_code"] != nil:
			property.Description = "HTTP response status."
		case name == "status":
			property.Description = "Lifecycle status, e.g. queued, in_progress, completed, waiting."
			// Sparse legacy minimal DTOs serialize an unknown status as "".
			property.Enum = inventory.EnumSchema(append(WorkflowStatusValues(), "")...).Enum
		case name == "method":
			property.Description = "Operation that produced this response."
		case name == "runner":
			property.Description = "Runner environment name; custom environments are supported."
		case name == "html_url":
			property.Description = "Browser URL."
		}
		describeActionsOutput(property)
	}
	describeActionsOutput(schema.Items)
	for _, variant := range schema.AnyOf {
		describeActionsOutput(variant)
	}
	for _, variant := range schema.OneOf {
		describeActionsOutput(variant)
	}
}

// Normalize only fields inspected by each legacy method. Ignored optional
// trigger values and non-string filter values retain their legacy defaults.
func normalizeActionsArguments(kind string) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, &inventory.ToolInputError{Message: "invalid arguments: " + err.Error()}
		}
		if args == nil {
			return nil, &inventory.ToolInputError{Message: "invalid arguments: arguments must be a JSON object"}
		}
		if err := normalizeActionsFields(args, kind); err != nil {
			return nil, &inventory.ToolInputError{Message: err.Error()}
		}
		return json.Marshal(args)
	}
}

func normalizeActionsFields(args map[string]any, kind string) error {
	for _, field := range []string{"owner", "repo"} {
		if _, err := RequiredParam[string](args, field); err != nil {
			return err
		}
	}
	if kind != "logs" {
		if _, err := RequiredParam[string](args, "method"); err != nil {
			return err
		}
	}
	switch kind {
	case "get":
		_, err := RequiredParam[string](args, "resource_id")
		return err
	case "list":
		if _, err := OptionalParam[string](args, "resource_id"); err != nil {
			return err
		}
		pagination, err := OptionalPaginationParams(args)
		if err != nil {
			return err
		}
		args["page"], args["perPage"] = pagination.Page, pagination.PerPage
		method := args["method"]
		for _, field := range []string{"workflow_runs_filter", "workflow_jobs_filter"} {
			relevant := field == "workflow_runs_filter" && method == actionsMethodListWorkflowRuns ||
				field == "workflow_jobs_filter" && method == actionsMethodListWorkflowJobs
			if !relevant {
				delete(args, field)
				continue
			}
			filter, err := OptionalParam[map[string]any](args, field)
			if err != nil {
				// The typed filter retains the error until its method runs.
				continue
			}
			if filter == nil {
				delete(args, field)
				continue
			}
			for key, value := range filter {
				if _, ok := value.(string); !ok {
					filter[key] = ""
				}
			}
		}
	case "trigger":
		for _, field := range []string{"workflow_id", "ref"} {
			if _, ok := args[field].(string); !ok {
				delete(args, field)
			}
		}
		runID, _ := OptionalIntParam(args, "run_id")
		args["run_id"] = runID
		_, err := OptionalParam[map[string]any](args, "inputs")
		return err
	case "logs":
		for _, field := range []string{"job_id", "run_id"} {
			value, err := OptionalIntParam(args, field)
			if err != nil {
				return err
			}
			args[field] = value
		}
		for _, field := range []string{"failed_only", "return_content"} {
			if _, err := OptionalParam[bool](args, field); err != nil {
				return err
			}
		}
		tailLines, err := OptionalIntParam(args, "tail_lines")
		if err != nil {
			return err
		}
		if tailLines <= 0 {
			tailLines = 500
		}
		args["tail_lines"] = tailLines
	default:
		panic("unknown Actions argument kind: " + kind)
	}
	return nil
}
