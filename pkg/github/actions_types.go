package github

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
)

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

func actionsInputSchema(tool inventory.ServerTool) inventory.ServerTool {
	schema := tool.Tool.InputSchema.(*jsonschema.Schema)
	// These enums were descriptive on the raw registration path. Keep the
	// supported values discoverable without replacing legacy handler errors.
	for _, field := range []string{"method", "workflow_runs_filter", "workflow_jobs_filter"} {
		property := schema.Properties[field]
		if property == nil {
			continue
		}
		if field == "method" {
			describeActionsEnum(property)
			continue
		}
		for _, nested := range property.Properties {
			describeActionsEnum(nested)
		}
		schema.Properties[field] = &jsonschema.Schema{
			Description: property.Description,
			AnyOf: []*jsonschema.Schema{
				property,
				{Not: &jsonschema.Schema{Type: "object"}},
			},
		}
	}
	return tool
}

func describeActionsEnum(schema *jsonschema.Schema) {
	if len(schema.Enum) == 0 {
		return
	}
	values := make([]string, 0, len(schema.Enum))
	for _, value := range schema.Enum {
		values = append(values, fmt.Sprint(value))
	}
	schema.Description += " Supported values: " + strings.Join(values, ", ") + "."
	schema.Enum = nil
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

type ActionsListOutput struct {
	Workflows *github.Workflows
	Runs      *MinimalWorkflowRunsResult
	Jobs      *ActionsJobsOutput
	Artifacts *github.ArtifactList
}

type ActionsJobsOutput struct {
	Jobs MinimalWorkflowJobsResult `json:"jobs"`
}

func (out ActionsListOutput) MarshalJSON() ([]byte, error) {
	switch {
	case out.Workflows != nil:
		return json.Marshal(out.Workflows)
	case out.Runs != nil:
		return json.Marshal(out.Runs)
	case out.Jobs != nil:
		return json.Marshal(out.Jobs)
	default:
		return json.Marshal(out.Artifacts)
	}
}

func actionsListOutputSchema() *jsonschema.Schema {
	return actionsUnionSchema(
		&jsonschema.Schema{Type: "null"},
		repositoryOutputSchema[github.Workflows](),
		repositoryOutputSchema[MinimalWorkflowRunsResult](),
		repositoryOutputSchema[ActionsJobsOutput](),
		repositoryOutputSchema[github.ArtifactList](),
	)
}

type ActionsGetOutput struct {
	Workflow *github.Workflow
	Run      *MinimalWorkflowRun
	Job      *github.WorkflowJob
	Usage    *github.WorkflowRunUsage
	Artifact *ActionsArtifactDownloadOutput
	Logs     *ActionsRunLogsOutput
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

func (out ActionsGetOutput) MarshalJSON() ([]byte, error) {
	switch {
	case out.Workflow != nil:
		return json.Marshal(out.Workflow)
	case out.Run != nil:
		return json.Marshal(out.Run)
	case out.Job != nil:
		return json.Marshal(out.Job)
	case out.Usage != nil:
		return json.Marshal(out.Usage)
	case out.Artifact != nil:
		return json.Marshal(out.Artifact)
	default:
		return json.Marshal(out.Logs)
	}
}

func actionsGetOutputSchema() *jsonschema.Schema {
	return actionsUnionSchema(
		&jsonschema.Schema{Type: "null"},
		repositoryOutputSchema[github.Workflow](),
		repositoryOutputSchema[MinimalWorkflowRun](),
		repositoryOutputSchema[github.WorkflowJob](),
		repositoryOutputSchema[github.WorkflowRunUsage](),
		repositoryOutputSchema[ActionsArtifactDownloadOutput](),
		repositoryOutputSchema[ActionsRunLogsOutput](),
	)
}

type ActionsRunTriggerOutput struct {
	Dispatch *ActionsDispatchOutput
	Run      *ActionsRunOperationOutput
}

// Workflow dispatch inputs are arbitrary user JSON, echoed verbatim by the
// legacy response. This is the only open-ended output field.
type ActionsDispatchOutput struct {
	Inputs       json.RawMessage `json:"inputs"`
	Message      string          `json:"message"`
	Ref          string          `json:"ref"`
	Status       string          `json:"status"`
	StatusCode   int             `json:"status_code"`
	WorkflowID   string          `json:"workflow_id"`
	WorkflowType string          `json:"workflow_type"`
}

type ActionsRunOperationOutput struct {
	Message    string `json:"message"`
	RunID      int64  `json:"run_id"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
}

func (out ActionsRunTriggerOutput) MarshalJSON() ([]byte, error) {
	if out.Dispatch != nil {
		return json.Marshal(out.Dispatch)
	}
	return json.Marshal(out.Run)
}

func actionsRunTriggerOutputSchema() *jsonschema.Schema {
	dispatch := repositoryOutputSchema[ActionsDispatchOutput]()
	dispatch.Properties["inputs"] = &jsonschema.Schema{
		Types:                []string{"object", "null"},
		AdditionalProperties: &jsonschema.Schema{},
	}
	return repositoryUnionSchema(
		&jsonschema.Schema{Type: "null"},
		dispatch,
		repositoryOutputSchema[ActionsRunOperationOutput](),
	)
}

// Sparse API objects can satisfy more than one method shape (including {}).
// anyOf preserves those legitimate responses without weakening field types.
func actionsUnionSchema(variants ...*jsonschema.Schema) *jsonschema.Schema {
	schema := repositoryUnionSchema(variants...)
	schema.AnyOf, schema.OneOf = schema.OneOf, nil
	return schema
}

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
		repositoryOutputSchema[ActionsJobLogContent](),
		repositoryOutputSchema[ActionsJobLogURL](),
		repositoryOutputSchema[ActionsJobLogError](),
	)
}

func actionsJobLogsOutputSchema() *jsonschema.Schema {
	failed := repositoryOutputSchema[ActionsFailedJobLogsOutput]()
	failed.Properties["logs"].Items = actionsJobLogSchema()
	return repositoryUnionSchema(
		&jsonschema.Schema{Type: "null"},
		repositoryOutputSchema[ActionsJobLogContent](),
		repositoryOutputSchema[ActionsJobLogURL](),
		failed,
	)
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
