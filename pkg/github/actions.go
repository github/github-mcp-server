package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/github/github-mcp-server/internal/profiler"
	buffer "github.com/github/github-mcp-server/pkg/buffer"
	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	DescriptionRepositoryOwner = "Repository owner"
	DescriptionRepositoryName  = "Repository name"
)

type actionsActionType int

const (
	actionsActionTypeUnknown actionsActionType = iota
	actionsActionTypeListWorkflows
	actionsActionTypeGetWorkflow
	actionsActionTypeGetWorkflowRun
	actionsActionTypeListWorkflowRuns
	actionsActionTypeGetWorkflowJob
	actionsActionTypeListWorkflowJobs
	actionsActionTypeDownloadWorkflowArtifact
	actionsActionTypeListWorkflowArtifacts
	actionsActionTypeGetWorkflowRunUsage
)

var actionsResourceTypes = map[actionsActionType]string{
	actionsActionTypeListWorkflows:            "list_workflows",
	actionsActionTypeGetWorkflow:              "get_workflow",
	actionsActionTypeGetWorkflowRun:           "get_workflow_run",
	actionsActionTypeListWorkflowRuns:         "list_workflow_runs",
	actionsActionTypeGetWorkflowJob:           "get_workflow_job",
	actionsActionTypeListWorkflowJobs:         "list_workflow_jobs",
	actionsActionTypeDownloadWorkflowArtifact: "download_workflow_run_artifact",
	actionsActionTypeListWorkflowArtifacts:    "list_workflow_run_artifacts",
	actionsActionTypeGetWorkflowRunUsage:      "get_workflow_run_usage",
}

func (r actionsActionType) String() string {
	if str, ok := actionsResourceTypes[r]; ok {
		return str
	}

	return "unknown"
}

func ActionFromString(s string) actionsActionType {
	for r, str := range actionsResourceTypes {
		if str == strings.ToLower(s) {
			return r
		}
	}
	return actionsActionTypeUnknown
}

func ActionsRead(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("actions_read",
			mcp.WithDescription(t("TOOL_ACTIONS_READ_DESCRIPTION", "Tools for reading GitHub Actions resources. Use this tool to get details about individual Actions Workflows, list and get individual Actions Workflow Runs, Jobs, and Artifacts.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_ACTIONS_READ_USER_TITLE", "Get details of GitHub Actions resources (workflows, workflow runs, jobs, and artifacts)"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("action",
				mcp.Required(),
				mcp.Description("The action to perform"),
				mcp.Enum(
					actionsActionTypeGetWorkflow.String(),
					actionsActionTypeGetWorkflowRun.String(),
					actionsActionTypeListWorkflowRuns.String(),
					actionsActionTypeGetWorkflowJob.String(),
					actionsActionTypeListWorkflowJobs.String(),
					actionsActionTypeDownloadWorkflowArtifact.String(),
					actionsActionTypeListWorkflowArtifacts.String(),
					actionsActionTypeGetWorkflowRunUsage.String(),
				),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryOwner),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryName),
			),
			mcp.WithNumber("resource_id",
				mcp.Description(`The unique identifier of the resource. This will vary based on the "action" provided, so ensure you provide the correct ID:
- Do not provide any resource ID for 'list_workflows' action.
- Provide a workflow ID or Filename for 'get_workflow' and 'list_workflow_runs' actions.
- Provide a workflow run ID for 'get_workflow_run', 'list_workflow_jobs', 'download_workflow_run_artifact', 'list_workflow_run_artifacts' and 'get_workflow_run_usage' actions.
- Provide a job ID for the 'get_workflow_job' action.
`),
			),
			mcp.WithObject("workflow_runs_filter",
				mcp.Description("Filters for workflow runs. **ONLY** used when action is 'list_workflow_runs'"),
				mcp.Properties(map[string]any{
					"actor": map[string]any{
						"type":        "string",
						"description": "Returns someone's workflow runs. Use the login for the user who created the workflow run.",
					},
					"branch": map[string]any{
						"type":        "string",
						"description": "Returns workflow runs associated with a branch. Use the name of the branch.",
					},
					"event": map[string]any{
						"type":        "string",
						"description": "Returns workflow runs for a specific event type",
						"enum": []string{
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
					"status": map[string]any{
						"type":        "string",
						"description": "Returns workflow runs with the check run status",
						"enum":        []string{"queued", "in_progress", "completed", "requested", "waiting"},
					},
				}),
			),
			mcp.WithObject("workflow_jobs_filter",
				mcp.Description("Filters for workflow jobs. **ONLY** used when action is 'list_workflow_jobs'"),
				mcp.Properties(map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"description": "Filters jobs by their completed_at timestamp",
						"enum":        []string{"latest", "all"},
					},
				}),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			actionTypeStr, err := RequiredParam[string](request, "action")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			resourceType := ActionFromString(actionTypeStr)
			if resourceType == actionsActionTypeUnknown {
				return mcp.NewToolResultError(fmt.Sprintf("unknown action: %s", actionTypeStr)), nil
			}

			resourceID, err := OptionalParam[string](request, "resource_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			var resourceIDInt int64
			var parseErr error
			switch resourceType {
			case actionsActionTypeListWorkflows:
				// No resource ID required
			case actionsActionTypeGetWorkflow, actionsActionTypeListWorkflowRuns:
				// Do nothing, we accept both a string workflow ID or filename
			default:
				if resourceID == "" {
					return mcp.NewToolResultError(fmt.Sprintf("missing required parameter for action %s: resource_id", actionTypeStr)), nil
				}

				// For other actions, resource ID must be an integer
				resourceIDInt, parseErr = strconv.ParseInt(resourceID, 10, 64)
				if parseErr != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid resource_id, must be an integer for action %s: %v", actionTypeStr, parseErr)), nil
				}
			}

			switch resourceType {
			case actionsActionTypeListWorkflows:
				return listWorkflows(ctx, client, request, owner, repo, pagination)
			case actionsActionTypeGetWorkflow:
				return getWorkflow(ctx, client, request, owner, repo, resourceID)
			case actionsActionTypeGetWorkflowRun:
				return getWorkflowRun(ctx, client, request, owner, repo, resourceIDInt)
			case actionsActionTypeListWorkflowRuns:
				return listWorkflowRuns(ctx, client, request, owner, repo, resourceID, pagination)
			case actionsActionTypeGetWorkflowJob:
				return getWorkflowJob(ctx, client, request, owner, repo, resourceIDInt)
			case actionsActionTypeListWorkflowJobs:
				return listWorkflowJobs(ctx, client, request, owner, repo, resourceIDInt, pagination)
			case actionsActionTypeDownloadWorkflowArtifact:
				return downloadWorkflowArtifact(ctx, client, request, owner, repo, resourceIDInt)
			case actionsActionTypeListWorkflowArtifacts:
				return listWorkflowArtifacts(ctx, client, request, owner, repo, resourceIDInt, pagination)
			case actionsActionTypeGetWorkflowRunUsage:
				return getWorkflowRunUsage(ctx, client, request, owner, repo, resourceIDInt)
			case actionsActionTypeUnknown:
				return mcp.NewToolResultError(fmt.Sprintf("unknown action: %s", actionTypeStr)), nil
			default:
				// Should not reach here
				return mcp.NewToolResultError("unhandled action type"), nil
			}
		}
}

func getWorkflow(ctx context.Context, client *github.Client, _ mcp.CallToolRequest, owner, repo string, resourceID string) (*mcp.CallToolResult, error) {
	var workflow *github.Workflow
	var resp *github.Response
	var err error

	if workflowIDInt, parseErr := strconv.ParseInt(resourceID, 10, 64); parseErr == nil {
		workflow, resp, err = client.Actions.GetWorkflowByID(ctx, owner, repo, workflowIDInt)
	} else {
		workflow, resp, err = client.Actions.GetWorkflowByFileName(ctx, owner, repo, resourceID)
	}

	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow", resp, err), nil
	}

	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func getWorkflowRun(ctx context.Context, client *github.Client, _ mcp.CallToolRequest, owner, repo string, resourceID int64) (*mcp.CallToolResult, error) {
	workflowRun, resp, err := client.Actions.GetWorkflowRunByID(ctx, owner, repo, resourceID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(workflowRun)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow run: %w", err)
	}
	return mcp.NewToolResultText(string(r)), nil
}

func listWorkflowRuns(ctx context.Context, client *github.Client, request mcp.CallToolRequest, owner, repo string, resourceID string, pagination PaginationParams) (*mcp.CallToolResult, error) {
	filterArgs, err := OptionalParam[map[string]any](request, "workflow_runs_filter")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	filterArgsTyped := make(map[string]string)
	for k, v := range filterArgs {
		if strVal, ok := v.(string); ok {
			filterArgsTyped[k] = strVal
		} else {
			filterArgsTyped[k] = ""
		}
	}

	listWorkflowRunsOptions := &github.ListWorkflowRunsOptions{
		Actor:  filterArgsTyped["actor"],
		Branch: filterArgsTyped["branch"],
		Event:  filterArgsTyped["event"],
		Status: filterArgsTyped["status"],
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	var workflowRuns *github.WorkflowRuns
	var resp *github.Response

	if workflowIDInt, parseErr := strconv.ParseInt(resourceID, 10, 64); parseErr == nil {
		workflowRuns, resp, err = client.Actions.ListWorkflowRunsByID(ctx, owner, repo, workflowIDInt, listWorkflowRunsOptions)
	} else {
		workflowRuns, resp, err = client.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, resourceID, listWorkflowRunsOptions)
	}

	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflow runs", resp, err), nil
	}

	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(workflowRuns)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow runs: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func getWorkflowJob(ctx context.Context, client *github.Client, _ mcp.CallToolRequest, owner, repo string, resourceID int64) (*mcp.CallToolResult, error) {
	workflowJob, resp, err := client.Actions.GetWorkflowJobByID(ctx, owner, repo, resourceID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow job", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(workflowJob)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow job: %w", err)
	}
	return mcp.NewToolResultText(string(r)), nil
}

func listWorkflowJobs(ctx context.Context, client *github.Client, request mcp.CallToolRequest, owner, repo string, resourceID int64, pagination PaginationParams) (*mcp.CallToolResult, error) {
	filterArgs, err := OptionalParam[map[string]any](request, "workflow_jobs_filter")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	filterArgsTyped := make(map[string]string)
	for k, v := range filterArgs {
		if strVal, ok := v.(string); ok {
			filterArgsTyped[k] = strVal
		} else {
			filterArgsTyped[k] = ""
		}
	}

	workflowJobs, resp, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, resourceID, &github.ListWorkflowJobsOptions{
		Filter: filterArgsTyped["filter"],
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	})
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflow jobs", resp, err), nil
	}

	// Add optimization tip for failed job debugging
	response := map[string]any{
		"jobs":             workflowJobs,
		"optimization_tip": "For debugging failed jobs, consider using get_job_logs with failed_only=true and run_id=" + fmt.Sprintf("%d", resourceID) + " to get logs directly without needing to list jobs first",
	}

	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow jobs: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func downloadWorkflowArtifact(ctx context.Context, client *github.Client, _ mcp.CallToolRequest, owner, repo string, resourceID int64) (*mcp.CallToolResult, error) {
	// Get the download URL for the artifact
	url, resp, err := client.Actions.DownloadArtifact(ctx, owner, repo, resourceID, 1)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get artifact download URL", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Create response with the download URL and information
	result := map[string]any{
		"download_url": url.String(),
		"message":      "Artifact is available for download",
		"note":         "The download_url provides a download link for the artifact as a ZIP archive. The link is temporary and expires after a short time.",
		"artifact_id":  resourceID,
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func listWorkflowArtifacts(ctx context.Context, client *github.Client, _ mcp.CallToolRequest, owner, repo string, resourceID int64, pagination PaginationParams) (*mcp.CallToolResult, error) {
	// Set up list options
	opts := &github.ListOptions{
		PerPage: pagination.PerPage,
		Page:    pagination.Page,
	}

	artifacts, resp, err := client.Actions.ListWorkflowRunArtifacts(ctx, owner, repo, resourceID, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflow run artifacts", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	r, err := json.Marshal(artifacts)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// ListWorkflows creates a tool to list workflows in a repository
func listWorkflows(ctx context.Context, client *github.Client, _ mcp.CallToolRequest, owner, repo string, pagination PaginationParams) (*mcp.CallToolResult, error) {
	// Set up list options
	opts := &github.ListOptions{
		PerPage: pagination.PerPage,
		Page:    pagination.Page,
	}

	workflows, resp, err := client.Actions.ListWorkflows(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	r, err := json.Marshal(workflows)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// RunWorkflow creates a tool to run an Actions workflow
func RunWorkflow(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("run_workflow",
			mcp.WithDescription(t("TOOL_RUN_WORKFLOW_DESCRIPTION", "Run an Actions workflow by workflow ID or filename")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_RUN_WORKFLOW_USER_TITLE", "Run workflow"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryOwner),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryName),
			),
			mcp.WithString("workflow_id",
				mcp.Required(),
				mcp.Description("The workflow ID (numeric) or workflow file name (e.g., main.yml, ci.yaml)"),
			),
			mcp.WithString("ref",
				mcp.Required(),
				mcp.Description("The git reference for the workflow. The reference can be a branch or tag name."),
			),
			mcp.WithObject("inputs",
				mcp.Description("Inputs the workflow accepts"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			workflowID, err := RequiredParam[string](request, "workflow_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ref, err := RequiredParam[string](request, "ref")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get optional inputs parameter
			var inputs map[string]interface{}
			if requestInputs, ok := request.GetArguments()["inputs"]; ok {
				if inputsMap, ok := requestInputs.(map[string]interface{}); ok {
					inputs = inputsMap
				}
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			event := github.CreateWorkflowDispatchEventRequest{
				Ref:    ref,
				Inputs: inputs,
			}

			var resp *github.Response
			var workflowType string

			if workflowIDInt, parseErr := strconv.ParseInt(workflowID, 10, 64); parseErr == nil {
				resp, err = client.Actions.CreateWorkflowDispatchEventByID(ctx, owner, repo, workflowIDInt, event)
				workflowType = "workflow_id"
			} else {
				resp, err = client.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, repo, workflowID, event)
				workflowType = "workflow_file"
			}

			if err != nil {
				return nil, fmt.Errorf("failed to run workflow: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			result := map[string]any{
				"message":       "Workflow run has been queued",
				"workflow_type": workflowType,
				"workflow_id":   workflowID,
				"ref":           ref,
				"inputs":        inputs,
				"status":        resp.Status,
				"status_code":   resp.StatusCode,
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetWorkflowRunLogs creates a tool to download logs for a specific workflow run
func GetWorkflowRunLogs(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_workflow_run_logs",
			mcp.WithDescription(t("TOOL_GET_WORKFLOW_RUN_LOGS_DESCRIPTION", "Download logs for a specific workflow run (EXPENSIVE: downloads ALL logs as ZIP. Consider using get_job_logs with failed_only=true for debugging failed jobs)")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_WORKFLOW_RUN_LOGS_USER_TITLE", "Get workflow run logs"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryOwner),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryName),
			),
			mcp.WithNumber("run_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the workflow run"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runIDInt, err := RequiredInt(request, "run_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runID := int64(runIDInt)

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get the download URL for the logs
			url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, 1)
			if err != nil {
				return nil, fmt.Errorf("failed to get workflow run logs: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Create response with the logs URL and information
			result := map[string]any{
				"logs_url":         url.String(),
				"message":          "Workflow run logs are available for download",
				"note":             "The logs_url provides a download link for the complete workflow run logs as a ZIP archive. You can download this archive to extract and examine individual job logs.",
				"warning":          "This downloads ALL logs as a ZIP file which can be large and expensive. For debugging failed jobs, consider using get_job_logs with failed_only=true and run_id instead.",
				"optimization_tip": "Use: get_job_logs with parameters {run_id: " + fmt.Sprintf("%d", runID) + ", failed_only: true} for more efficient failed job debugging",
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetJobLogs creates a tool to download logs for a specific workflow job or efficiently get all failed job logs for a workflow run
func GetJobLogs(getClient GetClientFn, t translations.TranslationHelperFunc, contentWindowSize int) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_job_logs",
			mcp.WithDescription(t("TOOL_GET_JOB_LOGS_DESCRIPTION", "Download logs for a specific workflow job or efficiently get all failed job logs for a workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_JOB_LOGS_USER_TITLE", "Get job logs"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryOwner),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryName),
			),
			mcp.WithNumber("job_id",
				mcp.Description("The unique identifier of the workflow job (required for single job logs)"),
			),
			mcp.WithNumber("run_id",
				mcp.Description("Workflow run ID (required when using failed_only)"),
			),
			mcp.WithBoolean("failed_only",
				mcp.Description("When true, gets logs for all failed jobs in run_id"),
			),
			mcp.WithBoolean("return_content",
				mcp.Description("Returns actual log content instead of URLs"),
			),
			mcp.WithNumber("tail_lines",
				mcp.Description("Number of lines to return from the end of the log"),
				mcp.DefaultNumber(500),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get optional parameters
			jobID, err := OptionalIntParam(request, "job_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runID, err := OptionalIntParam(request, "run_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			failedOnly, err := OptionalParam[bool](request, "failed_only")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			returnContent, err := OptionalParam[bool](request, "return_content")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			tailLines, err := OptionalIntParam(request, "tail_lines")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			// Default to 500 lines if not specified
			if tailLines == 0 {
				tailLines = 500
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Validate parameters
			if failedOnly && runID == 0 {
				return mcp.NewToolResultError("run_id is required when failed_only is true"), nil
			}
			if !failedOnly && jobID == 0 {
				return mcp.NewToolResultError("job_id is required when failed_only is false"), nil
			}

			if failedOnly && runID > 0 {
				// Handle failed-only mode: get logs for all failed jobs in the workflow run
				return handleFailedJobLogs(ctx, client, owner, repo, int64(runID), returnContent, tailLines, contentWindowSize)
			} else if jobID > 0 {
				// Handle single job mode
				return handleSingleJobLogs(ctx, client, owner, repo, int64(jobID), returnContent, tailLines, contentWindowSize)
			}

			return mcp.NewToolResultError("Either job_id must be provided for single job logs, or run_id with failed_only=true for failed job logs"), nil
		}
}

// handleFailedJobLogs gets logs for all failed jobs in a workflow run
func handleFailedJobLogs(ctx context.Context, client *github.Client, owner, repo string, runID int64, returnContent bool, tailLines int, contentWindowSize int) (*mcp.CallToolResult, error) {
	// First, get all jobs for the workflow run
	jobs, resp, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, &github.ListWorkflowJobsOptions{
		Filter: "latest",
	})
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflow jobs", resp, err), nil
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
		result := map[string]any{
			"message":     "No failed jobs found in this workflow run",
			"run_id":      runID,
			"total_jobs":  len(jobs.Jobs),
			"failed_jobs": 0,
		}
		r, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(r)), nil
	}

	// Collect logs for all failed jobs
	var logResults []map[string]any
	for _, job := range failedJobs {
		jobResult, resp, err := getJobLogData(ctx, client, owner, repo, job.GetID(), job.GetName(), returnContent, tailLines, contentWindowSize)
		if err != nil {
			// Continue with other jobs even if one fails
			jobResult = map[string]any{
				"job_id":   job.GetID(),
				"job_name": job.GetName(),
				"error":    err.Error(),
			}
			// Enable reporting of status codes and error causes
			_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get job logs", resp, err) // Explicitly ignore error for graceful handling
		}

		logResults = append(logResults, jobResult)
	}

	result := map[string]any{
		"message":       fmt.Sprintf("Retrieved logs for %d failed jobs", len(failedJobs)),
		"run_id":        runID,
		"total_jobs":    len(jobs.Jobs),
		"failed_jobs":   len(failedJobs),
		"logs":          logResults,
		"return_format": map[string]bool{"content": returnContent, "urls": !returnContent},
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// handleSingleJobLogs gets logs for a single job
func handleSingleJobLogs(ctx context.Context, client *github.Client, owner, repo string, jobID int64, returnContent bool, tailLines int, contentWindowSize int) (*mcp.CallToolResult, error) {
	jobResult, resp, err := getJobLogData(ctx, client, owner, repo, jobID, "", returnContent, tailLines, contentWindowSize)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get job logs", resp, err), nil
	}

	r, err := json.Marshal(jobResult)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// getJobLogData retrieves log data for a single job, either as URL or content
func getJobLogData(ctx context.Context, client *github.Client, owner, repo string, jobID int64, jobName string, returnContent bool, tailLines int, contentWindowSize int) (map[string]any, *github.Response, error) {
	// Get the download URL for the job logs
	url, resp, err := client.Actions.GetWorkflowJobLogs(ctx, owner, repo, jobID, 1)
	if err != nil {
		return nil, resp, fmt.Errorf("failed to get job logs for job %d: %w", jobID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	result := map[string]any{
		"job_id": jobID,
	}
	if jobName != "" {
		result["job_name"] = jobName
	}

	if returnContent {
		// Download and return the actual log content
		content, originalLength, httpResp, err := downloadLogContent(ctx, url.String(), tailLines, contentWindowSize) //nolint:bodyclose // Response body is closed in downloadLogContent, but we need to return httpResp
		if err != nil {
			// To keep the return value consistent wrap the response as a GitHub Response
			ghRes := &github.Response{
				Response: httpResp,
			}
			return nil, ghRes, fmt.Errorf("failed to download log content for job %d: %w", jobID, err)
		}
		result["logs_content"] = content
		result["message"] = "Job logs content retrieved successfully"
		result["original_length"] = originalLength
	} else {
		// Return just the URL
		result["logs_url"] = url.String()
		result["message"] = "Job logs are available for download"
		result["note"] = "The logs_url provides a download link for the individual job logs in plain text format. Use return_content=true to get the actual log content."
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

	bufferSize := tailLines
	if bufferSize > maxLines {
		bufferSize = maxLines
	}

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

// RerunWorkflowRun creates a tool to re-run an entire workflow run
func RerunWorkflowRun(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("rerun_workflow_run",
			mcp.WithDescription(t("TOOL_RERUN_WORKFLOW_RUN_DESCRIPTION", "Re-run an entire workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_RERUN_WORKFLOW_RUN_USER_TITLE", "Rerun workflow run"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryOwner),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryName),
			),
			mcp.WithNumber("run_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the workflow run"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runIDInt, err := RequiredInt(request, "run_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runID := int64(runIDInt)

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Actions.RerunWorkflowByID(ctx, owner, repo, runID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to rerun workflow run", resp, err), nil
			}
			defer func() { _ = resp.Body.Close() }()

			result := map[string]any{
				"message":     "Workflow run has been queued for re-run",
				"run_id":      runID,
				"status":      resp.Status,
				"status_code": resp.StatusCode,
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// RerunFailedJobs creates a tool to re-run only the failed jobs in a workflow run
func RerunFailedJobs(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("rerun_failed_jobs",
			mcp.WithDescription(t("TOOL_RERUN_FAILED_JOBS_DESCRIPTION", "Re-run only the failed jobs in a workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_RERUN_FAILED_JOBS_USER_TITLE", "Rerun failed jobs"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryOwner),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryName),
			),
			mcp.WithNumber("run_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the workflow run"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runIDInt, err := RequiredInt(request, "run_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runID := int64(runIDInt)

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Actions.RerunFailedJobsByID(ctx, owner, repo, runID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to rerun failed jobs", resp, err), nil
			}
			defer func() { _ = resp.Body.Close() }()

			result := map[string]any{
				"message":     "Failed jobs have been queued for re-run",
				"run_id":      runID,
				"status":      resp.Status,
				"status_code": resp.StatusCode,
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// CancelWorkflowRun creates a tool to cancel a workflow run
func CancelWorkflowRun(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("cancel_workflow_run",
			mcp.WithDescription(t("TOOL_CANCEL_WORKFLOW_RUN_DESCRIPTION", "Cancel a workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CANCEL_WORKFLOW_RUN_USER_TITLE", "Cancel workflow run"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryOwner),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryName),
			),
			mcp.WithNumber("run_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the workflow run"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runIDInt, err := RequiredInt(request, "run_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runID := int64(runIDInt)

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Actions.CancelWorkflowRunByID(ctx, owner, repo, runID)
			if err != nil {
				if _, ok := err.(*github.AcceptedError); !ok {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to cancel workflow run", resp, err), nil
				}
			}
			defer func() { _ = resp.Body.Close() }()

			result := map[string]any{
				"message":     "Workflow run has been cancelled",
				"run_id":      runID,
				"status":      resp.Status,
				"status_code": resp.StatusCode,
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// DeleteWorkflowRunLogs creates a tool to delete logs for a workflow run
func DeleteWorkflowRunLogs(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("delete_workflow_run_logs",
			mcp.WithDescription(t("TOOL_DELETE_WORKFLOW_RUN_LOGS_DESCRIPTION", "Delete logs for a workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           t("TOOL_DELETE_WORKFLOW_RUN_LOGS_USER_TITLE", "Delete workflow logs"),
				ReadOnlyHint:    ToBoolPtr(false),
				DestructiveHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryOwner),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description(DescriptionRepositoryName),
			),
			mcp.WithNumber("run_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the workflow run"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runIDInt, err := RequiredInt(request, "run_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			runID := int64(runIDInt)

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Actions.DeleteWorkflowRunLogs(ctx, owner, repo, runID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete workflow run logs", resp, err), nil
			}
			defer func() { _ = resp.Body.Close() }()

			result := map[string]any{
				"message":     "Workflow run logs have been deleted",
				"run_id":      runID,
				"status":      resp.Status,
				"status_code": resp.StatusCode,
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetWorkflowRunUsage creates a tool to get usage metrics for a workflow run
func getWorkflowRunUsage(ctx context.Context, client *github.Client, request mcp.CallToolRequest, owner, repo string, resourceID int64) (*mcp.CallToolResult, error) {
	usage, resp, err := client.Actions.GetWorkflowRunUsageByID(ctx, owner, repo, resourceID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run usage", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	r, err := json.Marshal(usage)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}
