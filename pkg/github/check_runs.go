package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/go-github/v87/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/utils"
)

const (
	checkRunsSourceChecksAPI      = "checks_api"
	checkRunsSourceWorkflowRuns   = "workflow_runs"
	checkRunsSourceCommitStatuses = "commit_statuses"
)

func isAccessDenied(resp *github.Response) bool {
	return resp != nil && resp.StatusCode == http.StatusForbidden
}

func checkRunsAccessErrMsg(base, owner, repo string) string {
	return fmt.Sprintf("%s. Check runs require the Checks API (checks:read for GitHub Apps, repo scope for classic PATs). "+
		"When using hosted MCP, the GitHub App installation must include Checks: Read permission. "+
		"Fallbacks using workflow runs and commit statuses were also unavailable for %s/%s.",
		base, owner, repo)
}

func GetPullRequestCheckRuns(ctx context.Context, client *github.Client, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	headSHA, errResult, err := getPullRequestHeadSHA(ctx, client, owner, repo, pullNumber)
	if errResult != nil || err != nil {
		return errResult, err
	}

	result, resp, err := fetchCheckRunsFromChecksAPI(ctx, client, owner, repo, headSHA, pagination)
	if err == nil {
		return marshalCheckRunsResult(result)
	}
	if !isAccessDenied(resp) {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get check runs", resp, err), nil
	}
	closeResponse(resp)

	// Checks API is unavailable (common on hosted MCP without checks:read). Try fallbacks.
	workflowFallback, workflowResp, workflowErr := fetchCheckRunsFromWorkflowRuns(ctx, client, owner, repo, headSHA, pagination)
	if workflowErr == nil {
		return marshalCheckRunsResult(workflowFallback)
	}
	if !isAccessDenied(workflowResp) {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get check runs", workflowResp, workflowErr), nil
	}
	closeResponse(workflowResp)

	statusFallback, statusResp, statusErr := fetchCheckRunsFromCommitStatuses(ctx, client, owner, repo, headSHA, pagination)
	if statusErr == nil {
		return marshalCheckRunsResult(statusFallback)
	}
	closeResponse(statusResp)

	return ghErrors.NewGitHubAPIErrorResponse(ctx,
		checkRunsAccessErrMsg("failed to get check runs", owner, repo),
		resp,
		err,
	), nil
}

func getPullRequestHeadSHA(ctx context.Context, client *github.Client, owner, repo string, pullNumber int) (string, *mcp.CallToolResult, error) {
	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
	if err != nil {
		return "", ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get pull request", resp, err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := readResponseBody(resp)
		if readErr != nil {
			return "", nil, readErr
		}
		return "", ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request", resp, body), nil
	}

	return pr.GetHead().GetSHA(), nil, nil
}

func fetchCheckRunsFromChecksAPI(ctx context.Context, client *github.Client, owner, repo, headSHA string, pagination PaginationParams) (MinimalCheckRunsResult, *github.Response, error) {
	opts := &github.ListCheckRunsOptions{
		ListOptions: github.ListOptions{
			PerPage: pagination.PerPage,
			Page:    pagination.Page,
		},
	}

	checkRuns, resp, err := client.Checks.ListCheckRunsForRef(ctx, owner, repo, headSHA, opts)
	if err != nil {
		return MinimalCheckRunsResult{}, resp, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := readResponseBody(resp)
		if readErr != nil {
			return MinimalCheckRunsResult{}, resp, readErr
		}
		return MinimalCheckRunsResult{}, resp, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	minimalCheckRuns := make([]MinimalCheckRun, 0, len(checkRuns.CheckRuns))
	for _, checkRun := range checkRuns.CheckRuns {
		minimalCheckRuns = append(minimalCheckRuns, convertToMinimalCheckRun(checkRun))
	}

	return MinimalCheckRunsResult{
		TotalCount: checkRuns.GetTotal(),
		CheckRuns:  minimalCheckRuns,
		Source:     checkRunsSourceChecksAPI,
	}, resp, nil
}

func fetchCheckRunsFromWorkflowRuns(ctx context.Context, client *github.Client, owner, repo, headSHA string, pagination PaginationParams) (MinimalCheckRunsResult, *github.Response, error) {
	opts := &github.ListWorkflowRunsOptions{
		HeadSHA: headSHA,
		ListOptions: github.ListOptions{
			PerPage: pagination.PerPage,
			Page:    pagination.Page,
		},
	}

	runs, resp, err := client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
	if err != nil {
		return MinimalCheckRunsResult{}, resp, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := readResponseBody(resp)
		if readErr != nil {
			return MinimalCheckRunsResult{}, resp, readErr
		}
		return MinimalCheckRunsResult{}, resp, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	minimalCheckRuns := make([]MinimalCheckRun, 0, len(runs.WorkflowRuns))
	for _, run := range runs.WorkflowRuns {
		minimalCheckRuns = append(minimalCheckRuns, convertWorkflowRunToMinimalCheckRun(run))
	}

	return MinimalCheckRunsResult{
		TotalCount: runs.GetTotalCount(),
		CheckRuns:  minimalCheckRuns,
		Source:     checkRunsSourceWorkflowRuns,
	}, resp, nil
}

func fetchCheckRunsFromCommitStatuses(ctx context.Context, client *github.Client, owner, repo, headSHA string, pagination PaginationParams) (MinimalCheckRunsResult, *github.Response, error) {
	opts := &github.ListOptions{
		PerPage: pagination.PerPage,
		Page:    pagination.Page,
	}

	statuses, resp, err := client.Repositories.ListStatuses(ctx, owner, repo, headSHA, opts)
	if err != nil {
		return MinimalCheckRunsResult{}, resp, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := readResponseBody(resp)
		if readErr != nil {
			return MinimalCheckRunsResult{}, resp, readErr
		}
		return MinimalCheckRunsResult{}, resp, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	minimalCheckRuns := make([]MinimalCheckRun, 0, len(statuses))
	for _, status := range statuses {
		minimalCheckRuns = append(minimalCheckRuns, convertCommitStatusToMinimalCheckRun(status))
	}

	return MinimalCheckRunsResult{
		TotalCount: len(minimalCheckRuns),
		CheckRuns:  minimalCheckRuns,
		Source:     checkRunsSourceCommitStatuses,
	}, resp, nil
}

func marshalCheckRunsResult(result MinimalCheckRunsResult) (*mcp.CallToolResult, error) {
	r, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(r)), nil
}

func closeResponse(resp *github.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

func readResponseBody(resp *github.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return body, nil
}
