package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/github/github-mcp-server/v2/internal/profiler"
	buffer "github.com/github/github-mcp-server/v2/pkg/buffer"
	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	DescriptionRepositoryOwner = "Repository owner"
	DescriptionRepositoryName  = "Repository name"
)

// Method constants for consolidated actions tools
const (
	actionsMethodListWorkflows            = "list_workflows"
	actionsMethodListWorkflowRuns         = "list_workflow_runs"
	actionsMethodListWorkflowJobs         = "list_workflow_jobs"
	actionsMethodListWorkflowArtifacts    = "list_workflow_run_artifacts"
	actionsMethodGetWorkflow              = "get_workflow"
	actionsMethodGetWorkflowRun           = "get_workflow_run"
	actionsMethodGetWorkflowJob           = "get_workflow_job"
	actionsMethodGetWorkflowRunUsage      = "get_workflow_run_usage"
	actionsMethodGetWorkflowRunLogsURL    = "get_workflow_run_logs_url"
	actionsMethodDownloadWorkflowArtifact = "download_workflow_run_artifact"
	actionsMethodRunWorkflow              = "run_workflow"
	actionsMethodRerunWorkflowRun         = "rerun_workflow_run"
	actionsMethodRerunFailedJobs          = "rerun_failed_jobs"
	actionsMethodCancelWorkflowRun        = "cancel_workflow_run"
	actionsMethodDeleteWorkflowRunLogs    = "delete_workflow_run_logs"
)

// handleFailedJobLogs gets logs for all failed jobs in a workflow run
func handleFailedJobLogs(ctx context.Context, client *github.Client, owner, repo string, runID int64, returnContent bool, tailLines int, contentWindowSize int) (*mcp.CallToolResult, *ActionsJobLogsOutput, error) {
	// First, get all jobs for the workflow run
	jobs, resp, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, &github.ListWorkflowJobsOptions{
		Filter: "latest",
	})
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflow jobs", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Filter for failed jobs
	var failedJobs []*github.WorkflowJob
	for _, job := range jobs.Jobs {
		if job.GetConclusion() == "failure" {
			failedJobs = append(failedJobs, job)
		}
	}

	if len(failedJobs) == 0 {
		result := &ActionsFailedJobLogsOutput{
			Message:   "No failed jobs found in this workflow run",
			RunID:     runID,
			TotalJobs: len(jobs.Jobs),
		}
		r, _ := json.Marshal(result)
		return utils.NewToolResultText(string(r)), &ActionsJobLogsOutput{Failed: result}, nil
	}

	// Collect logs for all failed jobs
	var logResults []ActionsJobLog
	for _, job := range failedJobs {
		jobResult, resp, err := getJobLogData(ctx, client, owner, repo, job.GetID(), job.GetName(), returnContent, tailLines, contentWindowSize)
		if err != nil {
			// Continue with other jobs even if one fails
			jobResult = &ActionsJobLog{Error: &ActionsJobLogError{
				JobID: job.GetID(), JobName: job.GetName(), Error: err.Error(),
			}}
			// Enable reporting of status codes and error causes
			_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get job logs", resp, err) // Explicitly ignore error for graceful handling
		}

		logResults = append(logResults, *jobResult)
	}

	result := &ActionsFailedJobLogsOutput{
		Message:      fmt.Sprintf("Retrieved logs for %d failed jobs", len(failedJobs)),
		RunID:        runID,
		TotalJobs:    len(jobs.Jobs),
		FailedJobs:   len(failedJobs),
		Logs:         &logResults,
		ReturnFormat: &ActionsLogsReturnFormat{Content: returnContent, URLs: !returnContent},
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsJobLogsOutput{Failed: result}, nil
}

// handleSingleJobLogs gets logs for a single job
func handleSingleJobLogs(ctx context.Context, client *github.Client, owner, repo string, jobID int64, returnContent bool, tailLines int, contentWindowSize int) (*mcp.CallToolResult, *ActionsJobLogsOutput, error) {
	jobResult, resp, err := getJobLogData(ctx, client, owner, repo, jobID, "", returnContent, tailLines, contentWindowSize)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get job logs", resp, err), nil, nil
	}

	r, err := json.Marshal(jobResult)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsJobLogsOutput{Single: jobResult}, nil
}

// getJobLogData retrieves log data for a single job, either as URL or content
func getJobLogData(ctx context.Context, client *github.Client, owner, repo string, jobID int64, jobName string, returnContent bool, tailLines int, contentWindowSize int) (*ActionsJobLog, *github.Response, error) {
	// Get the download URL for the job logs
	url, resp, err := client.Actions.GetWorkflowJobLogs(ctx, owner, repo, jobID, 1)
	if err != nil {
		return nil, resp, fmt.Errorf("failed to get job logs for job %d: %w", jobID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	result := &ActionsJobLog{}
	if returnContent {
		// Download and return the actual log content
		content, originalLength, httpResp, err := downloadLogContent(ctx, url.String(), tailLines, contentWindowSize) //nolint:bodyclose // Response body is closed in downloadLogContent, but we need to return httpResp
		if err != nil {
			var ghResp *github.Response
			if httpResp != nil {
				ghResp = &github.Response{Response: httpResp}
			}
			return nil, ghResp, fmt.Errorf("failed to download log content for job %d: %w", jobID, err)
		}
		result.Content = &ActionsJobLogContent{
			JobID: jobID, JobName: jobName, LogsContent: content,
			Message: "Job logs content retrieved successfully", OriginalLength: originalLength,
		}
	} else {
		// Return just the URL
		result.URL = &ActionsJobLogURL{
			JobID: jobID, JobName: jobName, LogsURL: url.String(),
			Message: "Job logs are available for download",
			Note:    "The logs_url provides a download link for the individual job logs in plain text format. Use return_content=true to get the actual log content.",
		}
	}

	return result, resp, nil
}

func downloadLogContent(ctx context.Context, logURL string, tailLines int, maxLines int) (string, int, *http.Response, error) {
	prof := profiler.New(nil, profiler.IsProfilingEnabled())
	finish := prof.Start(ctx, "log_buffer_processing")

	httpResp, err := http.Get(logURL) //nolint:gosec
	if err != nil {
		return "", 0, httpResp, fmt.Errorf("failed to download logs: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		return "", 0, httpResp, fmt.Errorf("failed to download logs: HTTP %d", httpResp.StatusCode)
	}

	bufferSize := min(tailLines, maxLines)

	processedInput, totalLines, httpResp, err := buffer.ProcessResponseAsRingBufferToEnd(httpResp, bufferSize)
	if err != nil {
		return "", 0, httpResp, fmt.Errorf("failed to process log content: %w", err)
	}

	lines := strings.Split(processedInput, "\n")
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	finalResult := strings.Join(lines, "\n")

	_ = finish(len(lines), int64(len(finalResult)))

	return finalResult, totalLines, httpResp, nil
}

// ActionsList returns the tool and handler for listing GitHub Actions resources.
func ActionsList(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := newActionsTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:         "actions_list",
			OutputSchema: actionsListOutputSchema(),
			Description: t("TOOL_ACTIONS_LIST_DESCRIPTION",
				`Tools for listing GitHub Actions resources.
Use this tool to list workflows in a repository, or list workflow runs, jobs, and artifacts for a specific workflow or workflow run.
`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ACTIONS_LIST_USER_TITLE", "List GitHub Actions workflows in a repository"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The action to perform",
						Enum:        actionsEnumSchemas()[reflect.TypeFor[ActionsListMethod]()].Enum,
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"resource_id": {
						Type: "string",
						Description: `The unique identifier of the resource. This will vary based on the "method" provided, so ensure you provide the correct ID:
- Do not provide any resource ID for 'list_workflows' method.
- Provide a workflow ID or workflow file name (e.g. ci.yaml) for 'list_workflow_runs' method, or omit to list all workflow runs in the repository.
- Provide a workflow run ID for 'list_workflow_jobs' and 'list_workflow_run_artifacts' methods.
`,
					},
					"workflow_runs_filter": {
						Type:        "object",
						Description: "Filters for workflow runs. **ONLY** used when method is 'list_workflow_runs'",
						Properties: map[string]*jsonschema.Schema{
							"actor": {
								Type:        "string",
								Description: "Filter to a specific GitHub user's workflow runs.",
							},
							"branch": {
								Type:        "string",
								Description: "Filter workflow runs to a specific Git branch. Use the name of the branch.",
							},
							"event": {
								Type:        "string",
								Description: "Filter workflow runs to a specific event type",
								Enum: []any{
									"branch_protection_rule",
									"check_run",
									"check_suite",
									"create",
									"delete",
									"deployment",
									"deployment_status",
									"discussion",
									"discussion_comment",
									"fork",
									"gollum",
									"issue_comment",
									"issues",
									"label",
									"merge_group",
									"milestone",
									"page_build",
									"public",
									"pull_request",
									"pull_request_review",
									"pull_request_review_comment",
									"pull_request_target",
									"push",
									"registry_package",
									"release",
									"repository_dispatch",
									"schedule",
									"status",
									"watch",
									"workflow_call",
									"workflow_dispatch",
									"workflow_run",
								},
							},
							"status": {
								Type:        "string",
								Description: "Filter workflow runs to only runs with a specific status",
								Enum:        []any{"queued", "in_progress", "completed", "requested", "waiting"},
							},
						},
					},
					"workflow_jobs_filter": {
						Type:        "object",
						Description: "Filters for workflow jobs. **ONLY** used when method is 'list_workflow_jobs'",
						Properties: map[string]*jsonschema.Schema{
							"filter": {
								Type:        "string",
								Description: "Filters jobs by their completed_at timestamp",
								Enum:        []any{"latest", "all"},
							},
						},
					},
					"page": {
						Type:        "number",
						Description: "Page number for pagination (default: 1)",
						Minimum:     new(1.0),
					},
					"perPage": {
						Type:        "number",
						Description: "Results per page for pagination (default: 30, max: 100)",
						Minimum:     new(1.0),
						Maximum:     new(100.0),
					},
				},
				Required: []string{"method", "owner", "repo"},
			},
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ActionsListInput) (*mcp.CallToolResult, *ActionsListOutput, error) {
			owner, repo, method, resourceID := args.Owner, args.Repo, args.Method, args.ResourceID
			pagination := PaginationParams{Page: args.Page, PerPage: args.PerPage}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// attachIFC adds the IFC label to a successful Actions result when
			// IFC labels are enabled. Workflow definitions, runs, jobs,
			// artifacts and logs echo attacker-influenceable run output, so
			// integrity is untrusted; confidentiality follows repo visibility.
			attachIFC := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelActionsResult)
			}

			var resourceIDInt int64
			var parseErr error
			switch method {
			case actionsMethodListWorkflows:
				// Do nothing, no resource ID needed
			case actionsMethodListWorkflowRuns:
				// resource_id is optional for list_workflow_runs
				// If not provided, list all workflow runs in the repository
			default:
				if resourceID == "" {
					return utils.NewToolResultError(fmt.Sprintf("missing required parameter for method %s: resource_id", method)), nil, nil
				}

				// resource ID must be an integer for jobs and artifacts
				resourceIDInt, parseErr = strconv.ParseInt(resourceID, 10, 64)
				if parseErr != nil {
					return utils.NewToolResultError(fmt.Sprintf("invalid resource_id, must be an integer for method %s: %v", method, parseErr)), nil, nil
				}
			}

			switch method {
			case actionsMethodListWorkflows:
				result, payload, err := listWorkflows(ctx, client, owner, repo, pagination)
				return attachIFC(result), payload, err
			case actionsMethodListWorkflowRuns:
				result, payload, err := listWorkflowRuns(ctx, client, args.WorkflowRunsFilter, owner, repo, resourceID, pagination)
				return attachIFC(result), payload, err
			case actionsMethodListWorkflowJobs:
				result, payload, err := listWorkflowJobs(ctx, client, args.WorkflowJobsFilter, owner, repo, resourceIDInt, pagination)
				return attachIFC(result), payload, err
			case actionsMethodListWorkflowArtifacts:
				result, payload, err := listWorkflowArtifacts(ctx, client, owner, repo, resourceIDInt, pagination)
				return attachIFC(result), payload, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
		normalizeActionsArguments("list"),
	)
	return tool
}

// ActionsGet returns the tool and handler for getting GitHub Actions resources.
func ActionsGet(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := newActionsTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:         "actions_get",
			OutputSchema: actionsGetOutputSchema(),
			Description: t("TOOL_ACTIONS_GET_DESCRIPTION", `Get details about specific GitHub Actions resources.
Use this tool to get details about individual workflows, workflow runs, jobs, and artifacts by their unique IDs.
`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ACTIONS_GET_USER_TITLE", "Get details of GitHub Actions resources (workflows, workflow runs, jobs, and artifacts)"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The method to execute",
						Enum:        actionsEnumSchemas()[reflect.TypeFor[ActionsGetMethod]()].Enum,
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"resource_id": {
						Type: "string",
						Description: `The unique identifier of the resource. This will vary based on the "method" provided, so ensure you provide the correct ID:
- Provide a workflow ID or workflow file name (e.g. ci.yaml) for 'get_workflow' method.
- Provide a workflow run ID for 'get_workflow_run', 'get_workflow_run_usage', and 'get_workflow_run_logs_url' methods.
- Provide an artifact ID for 'download_workflow_run_artifact' method.
- Provide a job ID for 'get_workflow_job' method.
`,
					},
				},
				Required: []string{"method", "owner", "repo", "resource_id"},
			},
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ActionsGetInput) (*mcp.CallToolResult, *ActionsGetOutput, error) {
			owner, repo, method, resourceID := args.Owner, args.Repo, args.Method, args.ResourceID

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// attachIFC adds the IFC label to a successful Actions result when
			// IFC labels are enabled. Workflow runs, jobs, artifacts, usage,
			// and log URLs reflect attacker-influenceable run output, so
			// integrity is untrusted; confidentiality follows repo visibility.
			attachIFC := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelActionsResult)
			}

			var resourceIDInt int64
			var parseErr error
			switch method {
			case actionsMethodGetWorkflow:
				// Do nothing, we accept both a string workflow ID or filename
			default:
				// For other methods, resource ID must be an integer
				resourceIDInt, parseErr = strconv.ParseInt(resourceID, 10, 64)
				if parseErr != nil {
					return utils.NewToolResultError(fmt.Sprintf("invalid resource_id, must be an integer for method %s: %v", method, parseErr)), nil, nil
				}
			}

			switch method {
			case actionsMethodGetWorkflow:
				result, payload, err := getWorkflow(ctx, client, owner, repo, resourceID)
				return attachIFC(result), payload, err
			case actionsMethodGetWorkflowRun:
				result, payload, err := getWorkflowRun(ctx, client, owner, repo, resourceIDInt)
				return attachIFC(result), payload, err
			case actionsMethodGetWorkflowJob:
				result, payload, err := getWorkflowJob(ctx, client, owner, repo, resourceIDInt)
				return attachIFC(result), payload, err
			case actionsMethodDownloadWorkflowArtifact:
				result, payload, err := downloadWorkflowArtifact(ctx, client, owner, repo, resourceIDInt)
				return attachIFC(result), payload, err
			case actionsMethodGetWorkflowRunUsage:
				result, payload, err := getWorkflowRunUsage(ctx, client, owner, repo, resourceIDInt)
				return attachIFC(result), payload, err
			case actionsMethodGetWorkflowRunLogsURL:
				result, payload, err := getWorkflowRunLogsURL(ctx, client, owner, repo, resourceIDInt)
				return attachIFC(result), payload, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
		normalizeActionsArguments("get"),
	)
	return tool
}

// ActionsRunTrigger returns the tool and handler for triggering GitHub Actions workflows.
func ActionsRunTrigger(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := newActionsTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:         "actions_run_trigger",
			OutputSchema: actionsRunTriggerOutputSchema(),
			Description:  t("TOOL_ACTIONS_RUN_TRIGGER_DESCRIPTION", "Trigger GitHub Actions workflow operations, including running, re-running, cancelling workflow runs, and deleting workflow run logs."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ACTIONS_RUN_TRIGGER_USER_TITLE", "Trigger GitHub Actions workflow actions"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The method to execute",
						Enum:        actionsEnumSchemas()[reflect.TypeFor[ActionsRunTriggerMethod]()].Enum,
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"workflow_id": {
						Type:        "string",
						Description: "The workflow ID (numeric) or workflow file name (e.g., main.yml, ci.yaml). Required for 'run_workflow' method.",
					},
					"ref": {
						Type:        "string",
						Description: "The git reference for the workflow. The reference can be a branch or tag name. Required for 'run_workflow' method.",
					},
					"inputs": {
						Type:        "object",
						Description: "Inputs the workflow accepts. Only used for 'run_workflow' method.",
						Properties:  map[string]*jsonschema.Schema{},
					},
					"run_id": {
						Type:        "number",
						Description: "The ID of the workflow run. Required for all methods except 'run_workflow'.",
					},
				},
				Required: []string{"method", "owner", "repo"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ActionsRunTriggerInput) (*mcp.CallToolResult, *ActionsRunTriggerOutput, error) {
			owner, repo, method := args.Owner, args.Repo, args.Method
			workflowID, ref, runID, inputs := args.WorkflowID, args.Ref, args.RunID, args.Inputs

			// Validate required parameters based on action type
			if method == actionsMethodRunWorkflow {
				if workflowID == "" {
					return utils.NewToolResultError("workflow_id is required for run_workflow action"), nil, nil
				}
				if ref == "" {
					return utils.NewToolResultError("ref is required for run_workflow action"), nil, nil
				}
			} else if runID == 0 {
				return utils.NewToolResultError("missing required parameter: run_id"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case actionsMethodRunWorkflow:
				return runWorkflow(ctx, client, owner, repo, workflowID, ref, inputs)
			case actionsMethodRerunWorkflowRun:
				return rerunWorkflowRun(ctx, client, owner, repo, int64(runID))
			case actionsMethodRerunFailedJobs:
				return rerunFailedJobs(ctx, client, owner, repo, int64(runID))
			case actionsMethodCancelWorkflowRun:
				return cancelWorkflowRun(ctx, client, owner, repo, int64(runID))
			case actionsMethodDeleteWorkflowRunLogs:
				return deleteWorkflowRunLogs(ctx, client, owner, repo, int64(runID))
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
		normalizeActionsArguments("trigger"),
	)
	return tool
}

// ActionsGetJobLogs returns the tool and handler for getting workflow job logs.
func ActionsGetJobLogs(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := newActionsTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:         "get_job_logs",
			OutputSchema: actionsJobLogsOutputSchema(),
			Description: t("TOOL_GET_JOB_LOGS_CONSOLIDATED_DESCRIPTION", `Get logs for GitHub Actions workflow jobs.
Use this tool to retrieve logs for a specific job or all failed jobs in a workflow run.
For single job logs, provide job_id. For all failed jobs in a run, provide run_id with failed_only=true.
`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_JOB_LOGS_CONSOLIDATED_USER_TITLE", "Get GitHub Actions workflow job logs"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"job_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow job. Required when getting logs for a single job.",
					},
					"run_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow run. Required when failed_only is true to get logs for all failed jobs in the run.",
					},
					"failed_only": {
						Type:        "boolean",
						Description: "When true, gets logs for all failed jobs in the workflow run specified by run_id. Requires run_id to be provided.",
					},
					"return_content": {
						Type:        "boolean",
						Description: "Returns actual log content instead of URLs",
					},
					"tail_lines": {
						Type:        "number",
						Description: "Number of lines to return from the end of the log",
						Default:     json.RawMessage(`500`),
					},
				},
				Required: []string{"owner", "repo"},
			},
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args ActionsGetJobLogsInput) (*mcp.CallToolResult, *ActionsJobLogsOutput, error) {
			owner, repo, jobID, runID := args.Owner, args.Repo, args.JobID, args.RunID
			failedOnly, returnContent, tailLines := args.FailedOnly, args.ReturnContent, args.TailLines

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Validate parameters
			if failedOnly && runID == 0 {
				return utils.NewToolResultError("run_id is required when failed_only is true"), nil, nil
			}
			if !failedOnly && jobID == 0 {
				return utils.NewToolResultError("job_id is required when failed_only is false"), nil, nil
			}

			// attachIFC adds the IFC label to a successful result when IFC
			// labels are enabled. Job logs echo attacker-influenceable run
			// output, so integrity is untrusted; confidentiality follows repo
			// visibility.
			attachIFC := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelActionsResult)
			}

			if failedOnly && runID > 0 {
				// Handle failed-only mode: get logs for all failed jobs in the workflow run
				result, payload, err := handleFailedJobLogs(ctx, client, owner, repo, int64(runID), returnContent, tailLines, deps.GetContentWindowSize())
				return attachIFC(result), payload, err
			} else if jobID > 0 {
				// Handle single job mode
				result, payload, err := handleSingleJobLogs(ctx, client, owner, repo, int64(jobID), returnContent, tailLines, deps.GetContentWindowSize())
				return attachIFC(result), payload, err
			}

			return utils.NewToolResultError("Either job_id must be provided for single job logs, or run_id with failed_only=true for failed job logs"), nil, nil
		},
		normalizeActionsArguments("logs"),
	)
	return tool
}

// Helper functions for consolidated actions tools

func getWorkflow(ctx context.Context, client *github.Client, owner, repo, resourceID string) (*mcp.CallToolResult, *ActionsGetOutput, error) {
	var workflow *github.Workflow
	var resp *github.Response
	var err error

	if workflowIDInt, parseErr := strconv.ParseInt(resourceID, 10, 64); parseErr == nil {
		workflow, resp, err = client.Actions.GetWorkflowByID(ctx, owner, repo, workflowIDInt)
	} else {
		workflow, resp, err = client.Actions.GetWorkflowByFileName(ctx, owner, repo, resourceID)
	}

	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow", resp, err), nil, nil
	}

	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(workflow)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal workflow: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsGetOutput{Method: actionsMethodGetWorkflow, Workflow: convertToActionsWorkflow(workflow)}, nil
}

func getWorkflowRun(ctx context.Context, client *github.Client, owner, repo string, resourceID int64) (*mcp.CallToolResult, *ActionsGetOutput, error) {
	workflowRun, resp, err := client.Actions.GetWorkflowRunByID(ctx, owner, repo, resourceID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()
	output := convertToMinimalWorkflowRun(workflowRun)
	r, err := json.Marshal(output)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal workflow run: %w", err)
	}
	return utils.NewToolResultText(string(r)), &ActionsGetOutput{Method: actionsMethodGetWorkflowRun, Run: &output}, nil
}

func getWorkflowJob(ctx context.Context, client *github.Client, owner, repo string, resourceID int64) (*mcp.CallToolResult, *ActionsGetOutput, error) {
	workflowJob, resp, err := client.Actions.GetWorkflowJobByID(ctx, owner, repo, resourceID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow job", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(workflowJob)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal workflow job: %w", err)
	}
	var output *MinimalWorkflowJob
	if workflowJob != nil {
		job := convertToMinimalWorkflowJob(workflowJob)
		output = &job
	}
	return utils.NewToolResultText(string(r)), &ActionsGetOutput{Method: actionsMethodGetWorkflowJob, Job: output}, nil
}

func listWorkflows(ctx context.Context, client *github.Client, owner, repo string, pagination PaginationParams) (*mcp.CallToolResult, *ActionsListOutput, error) {
	opts := &github.ListOptions{
		PerPage: pagination.PerPage,
		Page:    pagination.Page,
	}

	workflows, resp, err := client.Actions.ListWorkflows(ctx, owner, repo, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflows", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	r, err := json.Marshal(workflows)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal workflows: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsListOutput{Method: actionsMethodListWorkflows, Workflows: convertToActionsWorkflows(workflows)}, nil
}

func listWorkflowRuns(ctx context.Context, client *github.Client, filter ActionsWorkflowRunsFilter, owner, repo, resourceID string, pagination PaginationParams) (*mcp.CallToolResult, *ActionsListOutput, error) {
	if filter.validationError != "" {
		return utils.NewToolResultError(filter.validationError), nil, nil
	}

	listWorkflowRunsOptions := &github.ListWorkflowRunsOptions{
		Actor:  filter.Actor,
		Branch: filter.Branch,
		Event:  filter.Event,
		Status: filter.Status,
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	var workflowRuns *github.WorkflowRuns
	var resp *github.Response
	var err error

	if resourceID == "" {
		workflowRuns, resp, err = client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, listWorkflowRunsOptions)
	} else if workflowIDInt, parseErr := strconv.ParseInt(resourceID, 10, 64); parseErr == nil {
		workflowRuns, resp, err = client.Actions.ListWorkflowRunsByID(ctx, owner, repo, workflowIDInt, listWorkflowRunsOptions)
	} else {
		workflowRuns, resp, err = client.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, resourceID, listWorkflowRunsOptions)
	}

	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflow runs", resp, err), nil, nil
	}

	defer func() { _ = resp.Body.Close() }()
	output := convertToMinimalWorkflowRuns(workflowRuns)
	r, err := json.Marshal(output)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal workflow runs: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsListOutput{Method: actionsMethodListWorkflowRuns, Runs: &output}, nil
}

func listWorkflowJobs(ctx context.Context, client *github.Client, filter ActionsWorkflowJobsFilter, owner, repo string, resourceID int64, pagination PaginationParams) (*mcp.CallToolResult, *ActionsListOutput, error) {
	if filter.validationError != "" {
		return utils.NewToolResultError(filter.validationError), nil, nil
	}

	workflowJobs, resp, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, resourceID, &github.ListWorkflowJobsOptions{
		Filter: filter.Filter,
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	})
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflow jobs", resp, err), nil, nil
	}

	response := &ActionsJobsOutput{
		Jobs: convertToMinimalWorkflowJobs(workflowJobs),
	}

	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(response)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal workflow jobs: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsListOutput{Method: actionsMethodListWorkflowJobs, Jobs: &response.Jobs}, nil
}

func listWorkflowArtifacts(ctx context.Context, client *github.Client, owner, repo string, resourceID int64, pagination PaginationParams) (*mcp.CallToolResult, *ActionsListOutput, error) {
	opts := &github.ListOptions{
		PerPage: pagination.PerPage,
		Page:    pagination.Page,
	}

	artifacts, resp, err := client.Actions.ListWorkflowRunArtifacts(ctx, owner, repo, resourceID, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflow run artifacts", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	r, err := json.Marshal(artifacts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsListOutput{Method: actionsMethodListWorkflowArtifacts, Artifacts: convertToActionsArtifacts(artifacts)}, nil
}

func downloadWorkflowArtifact(ctx context.Context, client *github.Client, owner, repo string, resourceID int64) (*mcp.CallToolResult, *ActionsGetOutput, error) {
	// Get the download URL for the artifact
	url, resp, err := client.Actions.DownloadArtifact(ctx, owner, repo, resourceID, 1)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get artifact download URL", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Create response with the download URL and information
	result := &ActionsArtifactDownloadOutput{
		DownloadURL: url.String(),
		Message:     "Artifact is available for download",
		Note:        "The download_url provides a download link for the artifact as a ZIP archive. The link is temporary and expires after a short time.",
		ArtifactID:  resourceID,
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsGetOutput{Method: actionsMethodDownloadWorkflowArtifact, Artifact: result}, nil
}

func getWorkflowRunLogsURL(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*mcp.CallToolResult, *ActionsGetOutput, error) {
	// Get the download URL for the logs
	url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, 1)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run logs", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Create response with the logs URL and information
	result := &ActionsRunLogsOutput{
		LogsURL:         url.String(),
		Message:         "Workflow run logs are available for download",
		Note:            "The logs_url provides a download link for the complete workflow run logs as a ZIP archive. You can download this archive to extract and examine individual job logs.",
		Warning:         "This downloads ALL logs as a ZIP file which can be large and expensive. For debugging failed jobs, consider using get_job_logs with failed_only=true and run_id instead.",
		OptimizationTip: "Use: get_job_logs with parameters {run_id: " + fmt.Sprintf("%d", runID) + ", failed_only: true} for more efficient failed job debugging",
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsGetOutput{Method: actionsMethodGetWorkflowRunLogsURL, Logs: result}, nil
}

func getWorkflowRunUsage(ctx context.Context, client *github.Client, owner, repo string, resourceID int64) (*mcp.CallToolResult, *ActionsGetOutput, error) {
	usage, resp, err := client.Actions.GetWorkflowRunUsageByID(ctx, owner, repo, resourceID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run usage", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	r, err := json.Marshal(usage)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsGetOutput{Method: actionsMethodGetWorkflowRunUsage, Usage: convertToActionsRunUsage(usage)}, nil
}

func runWorkflow(ctx context.Context, client *github.Client, owner, repo, workflowID, ref string, inputs map[string]any) (*mcp.CallToolResult, *ActionsRunTriggerOutput, error) {
	event := github.CreateWorkflowDispatchEventRequest{
		Ref:    ref,
		Inputs: inputs,
	}

	var resp *github.Response
	var err error
	var workflowType ActionsWorkflowType

	if workflowIDInt, parseErr := strconv.ParseInt(workflowID, 10, 64); parseErr == nil {
		_, resp, err = client.Actions.CreateWorkflowDispatchEventByID(ctx, owner, repo, workflowIDInt, event)
		workflowType = "workflow_id"
	} else {
		_, resp, err = client.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, repo, workflowID, event)
		workflowType = "workflow_file"
	}

	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to run workflow", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	inputJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	result := &ActionsDispatchOutput{
		Message:      "Workflow run has been queued",
		WorkflowType: workflowType,
		WorkflowID:   workflowID,
		Ref:          ref,
		Inputs:       inputJSON,
		Status:       resp.Status,
		StatusCode:   resp.StatusCode,
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsRunTriggerOutput{Method: actionsMethodRunWorkflow, Dispatch: result}, nil
}

func rerunWorkflowRun(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*mcp.CallToolResult, *ActionsRunTriggerOutput, error) {
	resp, err := client.Actions.RerunWorkflowByID(ctx, owner, repo, runID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to rerun workflow run", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	result := &ActionsRunOperationOutput{
		Message:    "Workflow run has been queued for re-run",
		RunID:      runID,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsRunTriggerOutput{Method: actionsMethodRerunWorkflowRun, Rerun: result}, nil
}

func rerunFailedJobs(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*mcp.CallToolResult, *ActionsRunTriggerOutput, error) {
	resp, err := client.Actions.RerunFailedJobsByID(ctx, owner, repo, runID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to rerun failed jobs", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	result := &ActionsRunOperationOutput{
		Message:    "Failed jobs have been queued for re-run",
		RunID:      runID,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsRunTriggerOutput{Method: actionsMethodRerunFailedJobs, RerunFailed: result}, nil
}

func cancelWorkflowRun(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*mcp.CallToolResult, *ActionsRunTriggerOutput, error) {
	resp, err := client.Actions.CancelWorkflowRunByID(ctx, owner, repo, runID)
	if err != nil {
		if _, ok := errors.AsType[*github.AcceptedError](err); !ok {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to cancel workflow run", resp, err), nil, nil
		}
	}
	defer func() { _ = resp.Body.Close() }()

	result := &ActionsRunOperationOutput{
		Message:    "Workflow run has been cancelled",
		RunID:      runID,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsRunTriggerOutput{Method: actionsMethodCancelWorkflowRun, Cancel: result}, nil
}

func deleteWorkflowRunLogs(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*mcp.CallToolResult, *ActionsRunTriggerOutput, error) {
	resp, err := client.Actions.DeleteWorkflowRunLogs(ctx, owner, repo, runID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete workflow run logs", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	result := &ActionsRunOperationOutput{
		Message:    "Workflow run logs have been deleted",
		RunID:      runID,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &ActionsRunTriggerOutput{Method: actionsMethodDeleteWorkflowRunLogs, DeleteLogs: result}, nil
}
