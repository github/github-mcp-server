package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ListWorkflows creates a tool to list workflows in a repository
func ListWorkflows(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_workflows",
			mcp.WithDescription(t("TOOL_LIST_WORKFLOWS_DESCRIPTION", "List workflows in a repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("The number of results per page (max 100)"),
			),
			mcp.WithNumber("page",
				mcp.Description("The page number of the results to fetch"),
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

			// Get optional pagination parameters
			perPage, err := OptionalIntParam(request, "per_page")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			page, err := OptionalIntParam(request, "page")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Set up list options
			opts := &github.ListOptions{
				PerPage: perPage,
				Page:    page,
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
}

// ListWorkflowRuns creates a tool to list workflow runs for a specific workflow
func ListWorkflowRuns(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_workflow_runs",
			mcp.WithDescription(t("TOOL_LIST_WORKFLOW_RUNS_DESCRIPTION", "List workflow runs for a specific workflow")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("workflow_id",
				mcp.Required(),
				mcp.Description("The workflow ID or workflow file name"),
			),
			mcp.WithString("actor",
				mcp.Description("Returns someone's workflow runs. Use the login for the user who created the workflow run."),
			),
			mcp.WithString("branch",
				mcp.Description("Returns workflow runs associated with a branch. Use the name of the branch."),
			),
			mcp.WithString("event",
				mcp.Description("Returns workflow runs for an event. For example, push, pull_request, or issue."),
			),
			mcp.WithString("status",
				mcp.Description("Returns workflow runs with the check run status. For example, completed, in_progress, or requested."),
			),
			mcp.WithNumber("per_page",
				mcp.Description("The number of results per page (max 100)"),
			),
			mcp.WithNumber("page",
				mcp.Description("The page number of the results to fetch"),
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

			// Get optional filtering parameters
			actor, err := OptionalParam[string](request, "actor")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			branch, err := OptionalParam[string](request, "branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			event, err := OptionalParam[string](request, "event")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			status, err := OptionalParam[string](request, "status")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get optional pagination parameters
			perPage, err := OptionalIntParam(request, "per_page")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			page, err := OptionalIntParam(request, "page")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Set up list options
			opts := &github.ListWorkflowRunsOptions{
				Actor:  actor,
				Branch: branch,
				Event:  event,
				Status: status,
				ListOptions: github.ListOptions{
					PerPage: perPage,
					Page:    page,
				},
			}

			workflowRuns, resp, err := client.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, workflowID, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list workflow runs: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(workflowRuns)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// RunWorkflow creates a tool to run an Actions workflow
func RunWorkflow(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("run_workflow",
			mcp.WithDescription(t("TOOL_RUN_WORKFLOW_DESCRIPTION", "Run an Actions workflow")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("workflow_file",
				mcp.Required(),
				mcp.Description("The workflow ID or workflow file name"),
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
			workflowFile, err := RequiredParam[string](request, "workflow_file")
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

			resp, err := client.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, repo, workflowFile, event)
			if err != nil {
				return nil, fmt.Errorf("failed to run workflow: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			result := map[string]any{
				"message":     "Workflow run has been queued",
				"workflow":    workflowFile,
				"ref":         ref,
				"inputs":      inputs,
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

// GetWorkflowRun creates a tool to get details of a specific workflow run
func GetWorkflowRun(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_workflow_run",
			mcp.WithDescription(t("TOOL_GET_WORKFLOW_RUN_DESCRIPTION", "Get details of a specific workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
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

			workflowRun, resp, err := client.Actions.GetWorkflowRunByID(ctx, owner, repo, runID)
			if err != nil {
				return nil, fmt.Errorf("failed to get workflow run: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(workflowRun)
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
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
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

// ListWorkflowJobs creates a tool to list jobs for a specific workflow run
func ListWorkflowJobs(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_workflow_jobs",
			mcp.WithDescription(t("TOOL_LIST_WORKFLOW_JOBS_DESCRIPTION", "List jobs for a specific workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("run_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the workflow run"),
			),
			mcp.WithString("filter",
				mcp.Description("Filters jobs by their completed_at timestamp. Can be one of: latest, all"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("The number of results per page (max 100)"),
			),
			mcp.WithNumber("page",
				mcp.Description("The page number of the results to fetch"),
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

			// Get optional filtering parameters
			filter, err := OptionalParam[string](request, "filter")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get optional pagination parameters
			perPage, err := OptionalIntParam(request, "per_page")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			page, err := OptionalIntParam(request, "page")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Set up list options
			opts := &github.ListWorkflowJobsOptions{
				Filter: filter,
				ListOptions: github.ListOptions{
					PerPage: perPage,
					Page:    page,
				},
			}

			jobs, resp, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list workflow jobs: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Add optimization tip for failed job debugging
			response := map[string]any{
				"jobs":             jobs,
				"optimization_tip": "For debugging failed jobs, consider using get_job_logs with failed_only=true and run_id=" + fmt.Sprintf("%d", runID) + " to get logs directly without needing to list jobs first",
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetJobLogs creates a tool to download logs for a specific workflow job or get failed job logs efficiently
func GetJobLogs(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_job_logs",
			mcp.WithDescription(t("TOOL_GET_JOB_LOGS_DESCRIPTION", "Download logs for a specific workflow job or efficiently get all failed job logs for a workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
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
				return handleFailedJobLogs(ctx, client, owner, repo, int64(runID), returnContent)
			} else if jobID > 0 {
				// Handle single job mode
				return handleSingleJobLogs(ctx, client, owner, repo, int64(jobID), returnContent)
			}

			return mcp.NewToolResultError("Either job_id must be provided for single job logs, or run_id with failed_only=true for failed job logs"), nil
		}
}

// handleFailedJobLogs gets logs for all failed jobs in a workflow run
func handleFailedJobLogs(ctx context.Context, client *github.Client, owner, repo string, runID int64, returnContent bool) (*mcp.CallToolResult, error) {
	// First, get all jobs for the workflow run
	jobs, resp, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, &github.ListWorkflowJobsOptions{
		Filter: "latest",
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list workflow jobs: %v", err)), nil
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
		jobResult, err := getJobLogData(ctx, client, owner, repo, job.GetID(), job.GetName(), returnContent)
		if err != nil {
			// Continue with other jobs even if one fails
			jobResult = map[string]any{
				"job_id":   job.GetID(),
				"job_name": job.GetName(),
				"error":    err.Error(),
			}
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
func handleSingleJobLogs(ctx context.Context, client *github.Client, owner, repo string, jobID int64, returnContent bool) (*mcp.CallToolResult, error) {
	jobResult, err := getJobLogData(ctx, client, owner, repo, jobID, "", returnContent)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	r, err := json.Marshal(jobResult)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// getJobLogData retrieves log data for a single job, either as URL or content
func getJobLogData(ctx context.Context, client *github.Client, owner, repo string, jobID int64, jobName string, returnContent bool) (map[string]any, error) {
	// Get the download URL for the job logs
	url, resp, err := client.Actions.GetWorkflowJobLogs(ctx, owner, repo, jobID, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to get job logs for job %d: %w", jobID, err)
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
		content, err := downloadLogContent(url.String())
		if err != nil {
			return nil, fmt.Errorf("failed to download log content for job %d: %w", jobID, err)
		}
		result["logs_content"] = content
		result["message"] = "Job logs content retrieved successfully"
	} else {
		// Return just the URL
		result["logs_url"] = url.String()
		result["message"] = "Job logs are available for download"
		result["note"] = "The logs_url provides a download link for the individual job logs in plain text format. Use return_content=true to get the actual log content."
	}

	return result, nil
}

// downloadLogContent downloads the actual log content from a GitHub logs URL
func downloadLogContent(logURL string) (string, error) {
	httpResp, err := http.Get(logURL)
	if err != nil {
		return "", fmt.Errorf("failed to download logs: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download logs: HTTP %d", httpResp.StatusCode)
	}

	content, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read log content: %w", err)
	}

	// Clean up and format the log content for better readability
	logContent := strings.TrimSpace(string(content))
	return logContent, nil
}

// RerunWorkflowRun creates a tool to re-run an entire workflow run
func RerunWorkflowRun(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("rerun_workflow_run",
			mcp.WithDescription(t("TOOL_RERUN_WORKFLOW_RUN_DESCRIPTION", "Re-run an entire workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
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
				return nil, fmt.Errorf("failed to rerun workflow run: %w", err)
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
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
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
				return nil, fmt.Errorf("failed to rerun failed jobs: %w", err)
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
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
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
				return nil, fmt.Errorf("failed to cancel workflow run: %w", err)
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

// ListWorkflowRunArtifacts creates a tool to list artifacts for a workflow run
func ListWorkflowRunArtifacts(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_workflow_run_artifacts",
			mcp.WithDescription(t("TOOL_LIST_WORKFLOW_RUN_ARTIFACTS_DESCRIPTION", "List artifacts for a workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("run_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the workflow run"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("The number of results per page (max 100)"),
			),
			mcp.WithNumber("page",
				mcp.Description("The page number of the results to fetch"),
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

			// Get optional pagination parameters
			perPage, err := OptionalIntParam(request, "per_page")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			page, err := OptionalIntParam(request, "page")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Set up list options
			opts := &github.ListOptions{
				PerPage: perPage,
				Page:    page,
			}

			artifacts, resp, err := client.Actions.ListWorkflowRunArtifacts(ctx, owner, repo, runID, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list workflow run artifacts: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(artifacts)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// DownloadWorkflowRunArtifact creates a tool to download a workflow run artifact
func DownloadWorkflowRunArtifact(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("download_workflow_run_artifact",
			mcp.WithDescription(t("TOOL_DOWNLOAD_WORKFLOW_RUN_ARTIFACT_DESCRIPTION", "Get download URL for a workflow run artifact")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("artifact_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the artifact"),
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
			artifactIDInt, err := RequiredInt(request, "artifact_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			artifactID := int64(artifactIDInt)

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get the download URL for the artifact
			url, resp, err := client.Actions.DownloadArtifact(ctx, owner, repo, artifactID, 1)
			if err != nil {
				return nil, fmt.Errorf("failed to get artifact download URL: %w", err)
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
				ReadOnlyHint:    ToBoolPtr(false),
				DestructiveHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
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
				return nil, fmt.Errorf("failed to delete workflow run logs: %w", err)
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
func GetWorkflowRunUsage(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_workflow_run_usage",
			mcp.WithDescription(t("TOOL_GET_WORKFLOW_RUN_USAGE_DESCRIPTION", "Get usage metrics for a workflow run")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The account owner of the repository. The name is not case sensitive."),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
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

			usage, resp, err := client.Actions.GetWorkflowRunUsageByID(ctx, owner, repo, runID)
			if err != nil {
				return nil, fmt.Errorf("failed to get workflow run usage: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(usage)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
