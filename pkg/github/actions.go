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
	"github.com/github/github-mcp-server/pkg/scopes"
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

// ListWorkflows creates a tool to list workflows in a repository
func ListWorkflows(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "list_workflows",
			Description: t("TOOL_LIST_WORKFLOWS_DESCRIPTION", "List workflows in a repository"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
		}
}

// ListWorkflowRuns creates a tool to list workflow runs for a specific workflow
func ListWorkflowRuns(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "list_workflow_runs",
			Description: t("TOOL_LIST_WORKFLOW_RUNS_DESCRIPTION", "List workflow runs for a specific workflow"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
		}
}

// RunWorkflow creates a tool to run an Actions workflow
func RunWorkflow(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "run_workflow",
			Description: t("TOOL_RUN_WORKFLOW_DESCRIPTION", "Run an Actions workflow by workflow ID or filename"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
		}
}

// GetWorkflowRun creates a tool to get details of a specific workflow run
func GetWorkflowRun(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "get_workflow_run",
			Description: t("TOOL_GET_WORKFLOW_RUN_DESCRIPTION", "Get details of a specific workflow run"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

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
		}
}

// GetWorkflowRunLogs creates a tool to download logs for a specific workflow run
func GetWorkflowRunLogs(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "get_workflow_run_logs",
			Description: t("TOOL_GET_WORKFLOW_RUN_LOGS_DESCRIPTION", "Download logs for a specific workflow run (EXPENSIVE: downloads ALL logs as ZIP. Consider using get_job_logs with failed_only=true for debugging failed jobs)"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

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
		}
}

// ListWorkflowJobs creates a tool to list jobs for a specific workflow run
func ListWorkflowJobs(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "list_workflow_jobs",
			Description: t("TOOL_LIST_WORKFLOW_JOBS_DESCRIPTION", "List jobs for a specific workflow run"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
		}
}

// GetJobLogs creates a tool to download logs for a specific workflow job or efficiently get all failed job logs for a workflow run
func GetJobLogs(getClient GetClientFn, t translations.TranslationHelperFunc, contentWindowSize int) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "get_job_logs",
			Description: t("TOOL_GET_JOB_LOGS_DESCRIPTION", "Download logs for a specific workflow job or efficiently get all failed job logs for a workflow run"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
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
				return handleFailedJobLogs(ctx, client, owner, repo, int64(runID), returnContent, tailLines, contentWindowSize)
			} else if jobID > 0 {
				// Handle single job mode
				return handleSingleJobLogs(ctx, client, owner, repo, int64(jobID), returnContent, tailLines, contentWindowSize)
			}

			return utils.NewToolResultError("Either job_id must be provided for single job logs, or run_id with failed_only=true for failed job logs"), nil, nil
		}
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
func RerunWorkflowRun(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "rerun_workflow_run",
			Description: t("TOOL_RERUN_WORKFLOW_RUN_DESCRIPTION", "Re-run an entire workflow run"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

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
}

// RerunFailedJobs creates a tool to re-run only the failed jobs in a workflow run
func RerunFailedJobs(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "rerun_failed_jobs",
			Description: t("TOOL_RERUN_FAILED_JOBS_DESCRIPTION", "Re-run only the failed jobs in a workflow run"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

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
}

// CancelWorkflowRun creates a tool to cancel a workflow run
func CancelWorkflowRun(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "cancel_workflow_run",
			Description: t("TOOL_CANCEL_WORKFLOW_RUN_DESCRIPTION", "Cancel a workflow run"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

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
		}
}

// ListWorkflowRunArtifacts creates a tool to list artifacts for a workflow run
func ListWorkflowRunArtifacts(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "list_workflow_run_artifacts",
			Description: t("TOOL_LIST_WORKFLOW_RUN_ARTIFACTS_DESCRIPTION", "List artifacts for a workflow run"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
		}
}

// DownloadWorkflowRunArtifact creates a tool to download a workflow run artifact
func DownloadWorkflowRunArtifact(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "download_workflow_run_artifact",
			Description: t("TOOL_DOWNLOAD_WORKFLOW_RUN_ARTIFACT_DESCRIPTION", "Get download URL for a workflow run artifact"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

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
		}
}

// DeleteWorkflowRunLogs creates a tool to delete logs for a workflow run
func DeleteWorkflowRunLogs(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "delete_workflow_run_logs",
			Description: t("TOOL_DELETE_WORKFLOW_RUN_LOGS_DESCRIPTION", "Delete logs for a workflow run"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

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
}

// GetWorkflowRunUsage creates a tool to get usage metrics for a workflow run
func GetWorkflowRunUsage(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	return mcp.Tool{
			Name:        "get_workflow_run_usage",
			Description: t("TOOL_GET_WORKFLOW_RUN_USAGE_DESCRIPTION", "Get usage metrics for a workflow run"),
			Meta:        NewToolMeta(ToolsetMetadataActions.ID, scopes.Repo),
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
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

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
		}
}
