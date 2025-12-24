package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/github/github-mcp-server/internal/profiler"
	buffer "github.com/github/github-mcp-server/pkg/buffer"
	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	DescriptionRepositoryOwner = "Repository owner"
	DescriptionRepositoryName  = "Repository name"
)

// FeatureFlagConsolidatedActions is the feature flag that disables individual actions tools
// in favor of the consolidated actions tools.
const FeatureFlagConsolidatedActions = "remote_mcp_consolidated_actions"

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

// ListWorkflows creates a tool to list workflows in a repository
func ListWorkflows(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "list_workflows",
			Description: t("TOOL_LIST_WORKFLOWS_DESCRIPTION", "List workflows in a repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_WORKFLOWS_USER_TITLE", "List workflows"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
				},
				Required: []string{"owner", "repo"},
			}),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get optional pagination parameters
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Set up list options
			opts := &github.ListOptions{
				PerPage: pagination.PerPage,
				Page:    pagination.Page,
			}

			workflows, resp, err := client.Actions.ListWorkflows(ctx, owner, repo, opts)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to list workflows: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(workflows)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// ListWorkflowRuns creates a tool to list workflow runs for a specific workflow
func ListWorkflowRuns(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "list_workflow_runs",
			Description: t("TOOL_LIST_WORKFLOW_RUNS_DESCRIPTION", "List workflow runs for a specific workflow"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_WORKFLOW_RUNS_USER_TITLE", "List workflow runs"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"workflow_id": {
						Type:        "string",
						Description: "The workflow ID or workflow file name",
					},
					"actor": {
						Type:        "string",
						Description: "Returns someone's workflow runs. Use the login for the user who created the workflow run.",
					},
					"branch": {
						Type:        "string",
						Description: "Returns workflow runs associated with a branch. Use the name of the branch.",
					},
					"event": {
						Type:        "string",
						Description: "Returns workflow runs for a specific event type",
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
						Description: "Returns workflow runs with the check run status",
						Enum:        []any{"queued", "in_progress", "completed", "requested", "waiting"},
					},
				},
				Required: []string{"owner", "repo", "workflow_id"},
			}),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			workflowID, err := RequiredParam[string](args, "workflow_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get optional filtering parameters
			actor, err := OptionalParam[string](args, "actor")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			branch, err := OptionalParam[string](args, "branch")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			event, err := OptionalParam[string](args, "event")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			status, err := OptionalParam[string](args, "status")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get optional pagination parameters
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Set up list options
			opts := &github.ListWorkflowRunsOptions{
				Actor:  actor,
				Branch: branch,
				Event:  event,
				Status: status,
				ListOptions: github.ListOptions{
					PerPage: pagination.PerPage,
					Page:    pagination.Page,
				},
			}

			workflowRuns, resp, err := client.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, workflowID, opts)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to list workflow runs: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(workflowRuns)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// RunWorkflow creates a tool to run an Actions workflow
func RunWorkflow(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "run_workflow",
			Description: t("TOOL_RUN_WORKFLOW_DESCRIPTION", "Run an Actions workflow by workflow ID or filename"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_RUN_WORKFLOW_USER_TITLE", "Run workflow"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"workflow_id": {
						Type:        "string",
						Description: "The workflow ID (numeric) or workflow file name (e.g., main.yml, ci.yaml)",
					},
					"ref": {
						Type:        "string",
						Description: "The git reference for the workflow. The reference can be a branch or tag name.",
					},
					"inputs": {
						Type:        "object",
						Description: "Inputs the workflow accepts",
					},
				},
				Required: []string{"owner", "repo", "workflow_id", "ref"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			workflowID, err := RequiredParam[string](args, "workflow_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ref, err := RequiredParam[string](args, "ref")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get optional inputs parameter
			var inputs map[string]interface{}
			if requestInputs, ok := args["inputs"]; ok {
				if inputsMap, ok := requestInputs.(map[string]interface{}); ok {
					inputs = inputsMap
				}
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
				return nil, nil, fmt.Errorf("failed to run workflow: %w", err)
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
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// GetWorkflowRun creates a tool to get details of a specific workflow run
func GetWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "get_workflow_run",
			Description: t("TOOL_GET_WORKFLOW_RUN_DESCRIPTION", "Get details of a specific workflow run"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_WORKFLOW_RUN_USER_TITLE", "Get workflow run"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"run_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow run",
					},
				},
				Required: []string{"owner", "repo", "run_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runIDInt, err := RequiredInt(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runID := int64(runIDInt)

			workflowRun, resp, err := client.Actions.GetWorkflowRunByID(ctx, owner, repo, runID)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get workflow run: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(workflowRun)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// GetWorkflowRunLogs creates a tool to download logs for a specific workflow run
func GetWorkflowRunLogs(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "get_workflow_run_logs",
			Description: t("TOOL_GET_WORKFLOW_RUN_LOGS_DESCRIPTION", "Download logs for a specific workflow run (EXPENSIVE: downloads ALL logs as ZIP. Consider using get_job_logs with failed_only=true for debugging failed jobs)"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_WORKFLOW_RUN_LOGS_USER_TITLE", "Get workflow run logs"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"run_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow run",
					},
				},
				Required: []string{"owner", "repo", "run_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runIDInt, err := RequiredInt(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runID := int64(runIDInt)

			// Get the download URL for the logs
			url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, 1)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get workflow run logs: %w", err)
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
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// ListWorkflowJobs creates a tool to list jobs for a specific workflow run
func ListWorkflowJobs(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "list_workflow_jobs",
			Description: t("TOOL_LIST_WORKFLOW_JOBS_DESCRIPTION", "List jobs for a specific workflow run"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_WORKFLOW_JOBS_USER_TITLE", "List workflow jobs"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"run_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow run",
					},
					"filter": {
						Type:        "string",
						Description: "Filters jobs by their completed_at timestamp",
						Enum:        []any{"latest", "all"},
					},
				},
				Required: []string{"owner", "repo", "run_id"},
			}),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runIDInt, err := RequiredInt(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runID := int64(runIDInt)

			// Get optional filtering parameters
			filter, err := OptionalParam[string](args, "filter")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get optional pagination parameters
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Set up list options
			opts := &github.ListWorkflowJobsOptions{
				Filter: filter,
				ListOptions: github.ListOptions{
					PerPage: pagination.PerPage,
					Page:    pagination.Page,
				},
			}

			jobs, resp, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, opts)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to list workflow jobs: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Add optimization tip for failed job debugging
			response := map[string]any{
				"jobs":             jobs,
				"optimization_tip": "For debugging failed jobs, consider using get_job_logs with failed_only=true and run_id=" + fmt.Sprintf("%d", runID) + " to get logs directly without needing to list jobs first",
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// GetJobLogs creates a tool to download logs for a specific workflow job or efficiently get all failed job logs for a workflow run
func GetJobLogs(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "get_job_logs",
			Description: t("TOOL_GET_JOB_LOGS_DESCRIPTION", "Download logs for a specific workflow job or efficiently get all failed job logs for a workflow run"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_JOB_LOGS_USER_TITLE", "Get job logs"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"job_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow job (required for single job logs)",
					},
					"run_id": {
						Type:        "number",
						Description: "Workflow run ID (required when using failed_only)",
					},
					"failed_only": {
						Type:        "boolean",
						Description: "When true, gets logs for all failed jobs in run_id",
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
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get optional parameters
			jobID, err := OptionalIntParam(args, "job_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runID, err := OptionalIntParam(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			failedOnly, err := OptionalParam[bool](args, "failed_only")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			returnContent, err := OptionalParam[bool](args, "return_content")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			tailLines, err := OptionalIntParam(args, "tail_lines")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			// Default to 500 lines if not specified
			if tailLines == 0 {
				tailLines = 500
			}

			// Validate parameters
			if failedOnly && runID == 0 {
				return utils.NewToolResultError("run_id is required when failed_only is true"), nil, nil
			}
			if !failedOnly && jobID == 0 {
				return utils.NewToolResultError("job_id is required when failed_only is false"), nil, nil
			}

			if failedOnly && runID > 0 {
				// Handle failed-only mode: get logs for all failed jobs in the workflow run
				return handleFailedJobLogs(ctx, client, owner, repo, int64(runID), returnContent, tailLines, deps.GetContentWindowSize())
			} else if jobID > 0 {
				// Handle single job mode
				return handleSingleJobLogs(ctx, client, owner, repo, int64(jobID), returnContent, tailLines, deps.GetContentWindowSize())
			}

			return utils.NewToolResultError("Either job_id must be provided for single job logs, or run_id with failed_only=true for failed job logs"), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// handleFailedJobLogs gets logs for all failed jobs in a workflow run
func handleFailedJobLogs(ctx context.Context, client *github.Client, owner, repo string, runID int64, returnContent bool, tailLines int, contentWindowSize int) (*mcp.CallToolResult, any, error) {
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
		result := map[string]any{
			"message":     "No failed jobs found in this workflow run",
			"run_id":      runID,
			"total_jobs":  len(jobs.Jobs),
			"failed_jobs": 0,
		}
		r, _ := json.Marshal(result)
		return utils.NewToolResultText(string(r)), nil, nil
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
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

// handleSingleJobLogs gets logs for a single job
func handleSingleJobLogs(ctx context.Context, client *github.Client, owner, repo string, jobID int64, returnContent bool, tailLines int, contentWindowSize int) (*mcp.CallToolResult, any, error) {
	jobResult, resp, err := getJobLogData(ctx, client, owner, repo, jobID, "", returnContent, tailLines, contentWindowSize)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get job logs", resp, err), nil, nil
	}

	r, err := json.Marshal(jobResult)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
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
func RerunWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "rerun_workflow_run",
			Description: t("TOOL_RERUN_WORKFLOW_RUN_DESCRIPTION", "Re-run an entire workflow run"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_RERUN_WORKFLOW_RUN_USER_TITLE", "Rerun workflow run"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"run_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow run",
					},
				},
				Required: []string{"owner", "repo", "run_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runIDInt, err := RequiredInt(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runID := int64(runIDInt)

			resp, err := client.Actions.RerunWorkflowByID(ctx, owner, repo, runID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to rerun workflow run", resp, err), nil, nil
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
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// RerunFailedJobs creates a tool to re-run only the failed jobs in a workflow run
func RerunFailedJobs(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "rerun_failed_jobs",
			Description: t("TOOL_RERUN_FAILED_JOBS_DESCRIPTION", "Re-run only the failed jobs in a workflow run"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_RERUN_FAILED_JOBS_USER_TITLE", "Rerun failed jobs"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"run_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow run",
					},
				},
				Required: []string{"owner", "repo", "run_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runIDInt, err := RequiredInt(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runID := int64(runIDInt)

			resp, err := client.Actions.RerunFailedJobsByID(ctx, owner, repo, runID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to rerun failed jobs", resp, err), nil, nil
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
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// CancelWorkflowRun creates a tool to cancel a workflow run
func CancelWorkflowRun(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "cancel_workflow_run",
			Description: t("TOOL_CANCEL_WORKFLOW_RUN_DESCRIPTION", "Cancel a workflow run"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CANCEL_WORKFLOW_RUN_USER_TITLE", "Cancel workflow run"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"run_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow run",
					},
				},
				Required: []string{"owner", "repo", "run_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runIDInt, err := RequiredInt(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runID := int64(runIDInt)

			resp, err := client.Actions.CancelWorkflowRunByID(ctx, owner, repo, runID)
			if err != nil {
				if _, ok := err.(*github.AcceptedError); !ok {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to cancel workflow run", resp, err), nil, nil
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
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// ListWorkflowRunArtifacts creates a tool to list artifacts for a workflow run
func ListWorkflowRunArtifacts(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "list_workflow_run_artifacts",
			Description: t("TOOL_LIST_WORKFLOW_RUN_ARTIFACTS_DESCRIPTION", "List artifacts for a workflow run"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_WORKFLOW_RUN_ARTIFACTS_USER_TITLE", "List workflow artifacts"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"run_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow run",
					},
				},
				Required: []string{"owner", "repo", "run_id"},
			}),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runIDInt, err := RequiredInt(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runID := int64(runIDInt)

			// Get optional pagination parameters
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Set up list options
			opts := &github.ListOptions{
				PerPage: pagination.PerPage,
				Page:    pagination.Page,
			}

			artifacts, resp, err := client.Actions.ListWorkflowRunArtifacts(ctx, owner, repo, runID, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflow run artifacts", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(artifacts)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// DownloadWorkflowRunArtifact creates a tool to download a workflow run artifact
func DownloadWorkflowRunArtifact(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "download_workflow_run_artifact",
			Description: t("TOOL_DOWNLOAD_WORKFLOW_RUN_ARTIFACT_DESCRIPTION", "Get download URL for a workflow run artifact"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_DOWNLOAD_WORKFLOW_RUN_ARTIFACT_USER_TITLE", "Download workflow artifact"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"artifact_id": {
						Type:        "number",
						Description: "The unique identifier of the artifact",
					},
				},
				Required: []string{"owner", "repo", "artifact_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			artifactIDInt, err := RequiredInt(args, "artifact_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			artifactID := int64(artifactIDInt)

			// Get the download URL for the artifact
			url, resp, err := client.Actions.DownloadArtifact(ctx, owner, repo, artifactID, 1)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get artifact download URL", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Create response with the download URL and information
			result := map[string]any{
				"download_url": url.String(),
				"message":      "Artifact is available for download",
				"note":         "The download_url provides a download link for the artifact as a ZIP archive. The link is temporary and expires after a short time.",
				"artifact_id":  artifactID,
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// DeleteWorkflowRunLogs creates a tool to delete logs for a workflow run
func DeleteWorkflowRunLogs(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "delete_workflow_run_logs",
			Description: t("TOOL_DELETE_WORKFLOW_RUN_LOGS_DESCRIPTION", "Delete logs for a workflow run"),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_DELETE_WORKFLOW_RUN_LOGS_USER_TITLE", "Delete workflow logs"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"run_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow run",
					},
				},
				Required: []string{"owner", "repo", "run_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runIDInt, err := RequiredInt(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runID := int64(runIDInt)

			resp, err := client.Actions.DeleteWorkflowRunLogs(ctx, owner, repo, runID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete workflow run logs", resp, err), nil, nil
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
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// GetWorkflowRunUsage creates a tool to get usage metrics for a workflow run
func GetWorkflowRunUsage(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "get_workflow_run_usage",
			Description: t("TOOL_GET_WORKFLOW_RUN_USAGE_DESCRIPTION", "Get usage metrics for a workflow run"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_WORKFLOW_RUN_USAGE_USER_TITLE", "Get workflow usage"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: DescriptionRepositoryOwner,
					},
					"repo": {
						Type:        "string",
						Description: DescriptionRepositoryName,
					},
					"run_id": {
						Type:        "number",
						Description: "The unique identifier of the workflow run",
					},
				},
				Required: []string{"owner", "repo", "run_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runIDInt, err := RequiredInt(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			runID := int64(runIDInt)

			usage, resp, err := client.Actions.GetWorkflowRunUsageByID(ctx, owner, repo, runID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run usage", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(usage)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	tool.FeatureFlagDisable = FeatureFlagConsolidatedActions
	return tool
}

// ActionsList returns the tool and handler for listing GitHub Actions resources.
func ActionsList(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name: "actions_list",
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
						Enum: []any{
							actionsMethodListWorkflows,
							actionsMethodListWorkflowRuns,
							actionsMethodListWorkflowJobs,
							actionsMethodListWorkflowArtifacts,
						},
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
						Minimum:     jsonschema.Ptr(1.0),
					},
					"per_page": {
						Type:        "number",
						Description: "Results per page for pagination (default: 30, max: 100)",
						Minimum:     jsonschema.Ptr(1.0),
						Maximum:     jsonschema.Ptr(100.0),
					},
				},
				Required: []string{"method", "owner", "repo"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			resourceID, err := OptionalParam[string](args, "resource_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
				return listWorkflows(ctx, client, owner, repo, pagination)
			case actionsMethodListWorkflowRuns:
				return listWorkflowRuns(ctx, client, args, owner, repo, resourceID, pagination)
			case actionsMethodListWorkflowJobs:
				return listWorkflowJobs(ctx, client, args, owner, repo, resourceIDInt, pagination)
			case actionsMethodListWorkflowArtifacts:
				return listWorkflowArtifacts(ctx, client, owner, repo, resourceIDInt, pagination)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
	)
	tool.FeatureFlagEnable = FeatureFlagConsolidatedActions
	return tool
}

// ActionsGet returns the tool and handler for getting GitHub Actions resources.
func ActionsGet(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name: "actions_get",
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
						Enum: []any{
							actionsMethodGetWorkflow,
							actionsMethodGetWorkflowRun,
							actionsMethodGetWorkflowJob,
							actionsMethodDownloadWorkflowArtifact,
							actionsMethodGetWorkflowRunUsage,
							actionsMethodGetWorkflowRunLogsURL,
						},
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
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			resourceID, err := RequiredParam[string](args, "resource_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
				return getWorkflow(ctx, client, owner, repo, resourceID)
			case actionsMethodGetWorkflowRun:
				return getWorkflowRun(ctx, client, owner, repo, resourceIDInt)
			case actionsMethodGetWorkflowJob:
				return getWorkflowJob(ctx, client, owner, repo, resourceIDInt)
			case actionsMethodDownloadWorkflowArtifact:
				return downloadWorkflowArtifact(ctx, client, owner, repo, resourceIDInt)
			case actionsMethodGetWorkflowRunUsage:
				return getWorkflowRunUsage(ctx, client, owner, repo, resourceIDInt)
			case actionsMethodGetWorkflowRunLogsURL:
				return getWorkflowRunLogsURL(ctx, client, owner, repo, resourceIDInt)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
	)
	tool.FeatureFlagEnable = FeatureFlagConsolidatedActions
	return tool
}

// ActionsRunTrigger returns the tool and handler for triggering GitHub Actions workflows.
func ActionsRunTrigger(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name:        "actions_run_trigger",
			Description: t("TOOL_ACTIONS_RUN_TRIGGER_DESCRIPTION", "Trigger GitHub Actions workflow operations, including running, re-running, cancelling workflow runs, and deleting workflow run logs."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ACTIONS_RUN_TRIGGER_USER_TITLE", "Trigger GitHub Actions workflow actions"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The method to execute",
						Enum: []any{
							actionsMethodRunWorkflow,
							actionsMethodRerunWorkflowRun,
							actionsMethodRerunFailedJobs,
							actionsMethodCancelWorkflowRun,
							actionsMethodDeleteWorkflowRunLogs,
						},
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
					},
					"run_id": {
						Type:        "number",
						Description: "The ID of the workflow run. Required for all methods except 'run_workflow'.",
					},
				},
				Required: []string{"method", "owner", "repo"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get optional parameters
			workflowID, _ := OptionalParam[string](args, "workflow_id")
			ref, _ := OptionalParam[string](args, "ref")
			runID, _ := OptionalIntParam(args, "run_id")

			// Get optional inputs parameter
			var inputs map[string]interface{}
			if requestInputs, ok := args["inputs"]; ok {
				if inputsMap, ok := requestInputs.(map[string]interface{}); ok {
					inputs = inputsMap
				}
			}

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
	)
	tool.FeatureFlagEnable = FeatureFlagConsolidatedActions
	return tool
}

// ActionsGetJobLogs returns the tool and handler for getting workflow job logs.
func ActionsGetJobLogs(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataActions,
		mcp.Tool{
			Name: "get_job_logs",
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
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			jobID, err := OptionalIntParam(args, "job_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			runID, err := OptionalIntParam(args, "run_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			failedOnly, err := OptionalParam[bool](args, "failed_only")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			returnContent, err := OptionalParam[bool](args, "return_content")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			tailLines, err := OptionalIntParam(args, "tail_lines")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			// Default to 500 lines if not specified
			if tailLines == 0 {
				tailLines = 500
			}

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

			if failedOnly && runID > 0 {
				// Handle failed-only mode: get logs for all failed jobs in the workflow run
				return handleFailedJobLogs(ctx, client, owner, repo, int64(runID), returnContent, tailLines, deps.GetContentWindowSize())
			} else if jobID > 0 {
				// Handle single job mode
				return handleSingleJobLogs(ctx, client, owner, repo, int64(jobID), returnContent, tailLines, deps.GetContentWindowSize())
			}

			return utils.NewToolResultError("Either job_id must be provided for single job logs, or run_id with failed_only=true for failed job logs"), nil, nil
		},
	)
	tool.FeatureFlagEnable = FeatureFlagConsolidatedActions
	return tool
}

// Helper functions for consolidated actions tools

func getWorkflow(ctx context.Context, client *github.Client, owner, repo, resourceID string) (*mcp.CallToolResult, any, error) {
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

	return utils.NewToolResultText(string(r)), nil, nil
}

func getWorkflowRun(ctx context.Context, client *github.Client, owner, repo string, resourceID int64) (*mcp.CallToolResult, any, error) {
	workflowRun, resp, err := client.Actions.GetWorkflowRunByID(ctx, owner, repo, resourceID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(workflowRun)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal workflow run: %w", err)
	}
	return utils.NewToolResultText(string(r)), nil, nil
}

func getWorkflowJob(ctx context.Context, client *github.Client, owner, repo string, resourceID int64) (*mcp.CallToolResult, any, error) {
	workflowJob, resp, err := client.Actions.GetWorkflowJobByID(ctx, owner, repo, resourceID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow job", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(workflowJob)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal workflow job: %w", err)
	}
	return utils.NewToolResultText(string(r)), nil, nil
}

func listWorkflows(ctx context.Context, client *github.Client, owner, repo string, pagination PaginationParams) (*mcp.CallToolResult, any, error) {
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

	return utils.NewToolResultText(string(r)), nil, nil
}

func listWorkflowRuns(ctx context.Context, client *github.Client, args map[string]any, owner, repo, resourceID string, pagination PaginationParams) (*mcp.CallToolResult, any, error) {
	filterArgs, err := OptionalParam[map[string]any](args, "workflow_runs_filter")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
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
	r, err := json.Marshal(workflowRuns)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal workflow runs: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func listWorkflowJobs(ctx context.Context, client *github.Client, args map[string]any, owner, repo string, resourceID int64, pagination PaginationParams) (*mcp.CallToolResult, any, error) {
	filterArgs, err := OptionalParam[map[string]any](args, "workflow_jobs_filter")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
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
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list workflow jobs", resp, err), nil, nil
	}

	response := map[string]any{
		"jobs": workflowJobs,
	}

	defer func() { _ = resp.Body.Close() }()
	r, err := json.Marshal(response)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal workflow jobs: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func listWorkflowArtifacts(ctx context.Context, client *github.Client, owner, repo string, resourceID int64, pagination PaginationParams) (*mcp.CallToolResult, any, error) {
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

	return utils.NewToolResultText(string(r)), nil, nil
}

func downloadWorkflowArtifact(ctx context.Context, client *github.Client, owner, repo string, resourceID int64) (*mcp.CallToolResult, any, error) {
	// Get the download URL for the artifact
	url, resp, err := client.Actions.DownloadArtifact(ctx, owner, repo, resourceID, 1)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get artifact download URL", resp, err), nil, nil
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
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func getWorkflowRunLogsURL(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*mcp.CallToolResult, any, error) {
	// Get the download URL for the logs
	url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, owner, repo, runID, 1)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run logs", resp, err), nil, nil
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
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func getWorkflowRunUsage(ctx context.Context, client *github.Client, owner, repo string, resourceID int64) (*mcp.CallToolResult, any, error) {
	usage, resp, err := client.Actions.GetWorkflowRunUsageByID(ctx, owner, repo, resourceID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get workflow run usage", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	r, err := json.Marshal(usage)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func runWorkflow(ctx context.Context, client *github.Client, owner, repo, workflowID, ref string, inputs map[string]interface{}) (*mcp.CallToolResult, any, error) {
	event := github.CreateWorkflowDispatchEventRequest{
		Ref:    ref,
		Inputs: inputs,
	}

	var resp *github.Response
	var err error
	var workflowType string

	if workflowIDInt, parseErr := strconv.ParseInt(workflowID, 10, 64); parseErr == nil {
		resp, err = client.Actions.CreateWorkflowDispatchEventByID(ctx, owner, repo, workflowIDInt, event)
		workflowType = "workflow_id"
	} else {
		resp, err = client.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, repo, workflowID, event)
		workflowType = "workflow_file"
	}

	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to run workflow", resp, err), nil, nil
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
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func rerunWorkflowRun(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*mcp.CallToolResult, any, error) {
	resp, err := client.Actions.RerunWorkflowByID(ctx, owner, repo, runID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to rerun workflow run", resp, err), nil, nil
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
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func rerunFailedJobs(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*mcp.CallToolResult, any, error) {
	resp, err := client.Actions.RerunFailedJobsByID(ctx, owner, repo, runID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to rerun failed jobs", resp, err), nil, nil
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
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func cancelWorkflowRun(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*mcp.CallToolResult, any, error) {
	resp, err := client.Actions.CancelWorkflowRunByID(ctx, owner, repo, runID)
	if err != nil {
		var acceptedErr *github.AcceptedError
		if !errors.As(err, &acceptedErr) {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to cancel workflow run", resp, err), nil, nil
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
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func deleteWorkflowRunLogs(ctx context.Context, client *github.Client, owner, repo string, runID int64) (*mcp.CallToolResult, any, error) {
	resp, err := client.Actions.DeleteWorkflowRunLogs(ctx, owner, repo, runID)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete workflow run logs", resp, err), nil, nil
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
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}
