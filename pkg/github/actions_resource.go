package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/github/github-mcp-server/internal/profiler"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GetWorkflowRunLogsResource defines the resource template and handler for getting workflow run logs.
func GetWorkflowRunLogsResource(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.ResourceTemplate, server.ResourceTemplateHandlerFunc) {
	return mcp.NewResourceTemplate(
			"actions://{owner}/{repo}/runs/{runId}/logs", // Resource template
			t("RESOURCE_WORKFLOW_RUN_LOGS_DESCRIPTION", "Workflow Run Logs"),
		),
		WorkflowRunLogsResourceHandler(getClient)
}

// GetJobLogsResource defines the resource template and handler for getting individual job logs.
func GetJobLogsResource(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.ResourceTemplate, server.ResourceTemplateHandlerFunc) {
	return mcp.NewResourceTemplate(
			"actions://{owner}/{repo}/jobs/{jobId}/logs", // Resource template
			t("RESOURCE_JOB_LOGS_DESCRIPTION", "Job Logs"),
		),
		JobLogsResourceHandler(getClient)
}

// WorkflowRunLogsResourceHandler returns a handler function for workflow run logs requests.
func WorkflowRunLogsResourceHandler(getClient GetClientFn) func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Parse parameters from the URI template matcher
		owner, ok := request.Params.Arguments["owner"].([]string)
		if !ok || len(owner) == 0 {
			return nil, errors.New("owner is required")
		}

		repo, ok := request.Params.Arguments["repo"].([]string)
		if !ok || len(repo) == 0 {
			return nil, errors.New("repo is required")
		}

		runIDStr, ok := request.Params.Arguments["runId"].([]string)
		if !ok || len(runIDStr) == 0 {
			return nil, errors.New("runId is required")
		}

		runID, err := strconv.ParseInt(runIDStr[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid runId: %w", err)
		}

		client, err := getClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}

		// Get the JIT URL for workflow run logs
		url, resp, err := client.Actions.GetWorkflowRunLogs(ctx, owner[0], repo[0], runID, 1)
		if err != nil {
			return nil, fmt.Errorf("failed to get workflow run logs URL: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Download the logs content immediately using the JIT URL
		content, err := downloadLogsFromJITURL(ctx, url.String())
		if err != nil {
			return nil, fmt.Errorf("failed to download workflow run logs: %w", err)
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "application/zip",
				Text:     fmt.Sprintf("Workflow run logs for run %d (ZIP archive)\n\nNote: This is a ZIP archive containing all job logs. Download URL was: %s\n\nContent length: %d bytes", runID, url.String(), len(content)),
			},
		}, nil
	}
}

// JobLogsResourceHandler returns a handler function for individual job logs requests.
func JobLogsResourceHandler(getClient GetClientFn) func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Parse parameters from the URI template matcher
		owner, ok := request.Params.Arguments["owner"].([]string)
		if !ok || len(owner) == 0 {
			return nil, errors.New("owner is required")
		}

		repo, ok := request.Params.Arguments["repo"].([]string)
		if !ok || len(repo) == 0 {
			return nil, errors.New("repo is required")
		}

		jobIDStr, ok := request.Params.Arguments["jobId"].([]string)
		if !ok || len(jobIDStr) == 0 {
			return nil, errors.New("jobId is required")
		}

		jobID, err := strconv.ParseInt(jobIDStr[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid jobId: %w", err)
		}

		client, err := getClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}

		// Get the JIT URL for job logs
		url, resp, err := client.Actions.GetWorkflowJobLogs(ctx, owner[0], repo[0], jobID, 1)
		if err != nil {
			return nil, fmt.Errorf("failed to get job logs URL: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Download the logs content immediately using the JIT URL
		content, err := downloadLogsFromJITURL(ctx, url.String())
		if err != nil {
			return nil, fmt.Errorf("failed to download job logs: %w", err)
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "text/plain",
				Text:     content,
			},
		}, nil
	}
}

// downloadLogsFromJITURL downloads content from a GitHub JIT URL
func downloadLogsFromJITURL(ctx context.Context, jitURL string) (string, error) {
	prof := profiler.New(nil, profiler.IsProfilingEnabled())
	finish := prof.Start(ctx, "download_jit_logs")

	httpResp, err := http.Get(jitURL) //nolint:gosec
	if err != nil {
		_ = finish(0, 0)
		return "", fmt.Errorf("failed to download from JIT URL: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		_ = finish(0, 0)
		return "", fmt.Errorf("failed to download logs: HTTP %d", httpResp.StatusCode)
	}

	// For large files, we should limit the content size to avoid memory issues
	const maxContentSize = 10 * 1024 * 1024 // 10MB limit

	// Read the content with a size limit
	content := make([]byte, 0, 1024*1024) // Start with 1MB capacity
	buffer := make([]byte, 32*1024)       // 32KB read buffer
	totalRead := 0

	for {
		n, err := httpResp.Body.Read(buffer)
		if n > 0 {
			if totalRead+n > maxContentSize {
				// Truncate if content is too large
				remaining := maxContentSize - totalRead
				content = append(content, buffer[:remaining]...)
				content = append(content, []byte(fmt.Sprintf("\n\n[Content truncated - original size exceeded %d bytes]", maxContentSize))...)
				break
			}
			content = append(content, buffer[:n]...)
			totalRead += n
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			_ = finish(0, int64(totalRead))
			return "", fmt.Errorf("failed to read response body: %w", err)
		}
	}

	// Count lines for profiler
	lines := 1
	for _, b := range content {
		if b == '\n' {
			lines++
		}
	}

	_ = finish(lines, int64(len(content)))
	return string(content), nil
}
