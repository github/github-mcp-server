package github

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/sanitize"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func validateActionsChecksListInput(input ActionsListInput) error {
	if input.Page < 1 || input.PerPage < 1 || input.PerPage > 100 {
		return fmt.Errorf("page must be positive and perPage must be between 1 and 100")
	}
	if input.Method != actionsMethodListCheckRuns {
		return nil
	}
	if (input.Ref == "") == (input.ResourceID == "") {
		return fmt.Errorf("list_check_runs requires exactly one of ref or resource_id (a check suite ID)")
	}
	filter := input.CheckRunsFilter
	if filter.Status != "" && !slices.Contains([]string{"queued", "in_progress", "completed"}, filter.Status) {
		return fmt.Errorf("check_runs_filter.status must be queued, in_progress, or completed")
	}
	if filter.Filter != "" && !slices.Contains([]string{"latest", "all"}, filter.Filter) {
		return fmt.Errorf("check_runs_filter.filter must be latest or all")
	}
	if filter.AppID != nil {
		if *filter.AppID <= 0 {
			return fmt.Errorf("check_runs_filter.app_id must be positive")
		}
		if input.Ref == "" {
			return fmt.Errorf("check_runs_filter.app_id is only supported with ref")
		}
	}
	return nil
}

func convertToActionsCheckRunSummary(checkRun *github.CheckRun) ActionsCheckRunSummary {
	summary := ActionsCheckRunSummary{
		MinimalCheckRun: convertToMinimalCheckRun(checkRun),
		HeadSHA:         checkRun.GetHeadSHA(),
		CheckSuiteID:    checkRun.GetCheckSuite().GetID(),
	}
	if app := checkRun.GetApp(); app != nil {
		summary.App = &ActionsCheckApp{ID: app.GetID(), Slug: app.GetSlug()}
	}
	return summary
}

func listActionsCheckRuns(ctx context.Context, client *github.Client, owner, repo, ref string, checkSuiteID int64, filter ActionsCheckRunsFilter, pagination PaginationParams) (*mcp.CallToolResult, *ActionsListOutput, error) {
	opts := &github.ListCheckRunsOptions{
		Filter:      new("latest"),
		ListOptions: github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage},
	}
	if filter.CheckName != "" {
		opts.CheckName = &filter.CheckName
	}
	if filter.Status != "" {
		opts.Status = &filter.Status
	}
	if filter.Filter != "" {
		opts.Filter = &filter.Filter
	}
	opts.AppID = filter.AppID

	var checkRuns *github.ListCheckRunsResults
	var resp *github.Response
	var err error
	if ref != "" {
		checkRuns, resp, err = client.Checks.ListCheckRunsForRef(ctx, owner, repo, ref, opts)
	} else {
		checkRuns, resp, err = client.Checks.ListCheckRunsCheckSuite(ctx, owner, repo, checkSuiteID, opts)
	}
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list check runs", resp, err), nil, nil
	}
	if checkRuns == nil {
		return utils.NewToolResultError("GitHub returned no check runs response"), nil, nil
	}

	output := &ActionsCheckRunsOutput{
		TotalCount: checkRuns.GetTotal(),
		CheckRuns:  make([]ActionsCheckRunSummary, 0, len(checkRuns.CheckRuns)),
		NextPage:   resp.NextPage,
	}
	for _, checkRun := range checkRuns.CheckRuns {
		if checkRun != nil {
			output.CheckRuns = append(output.CheckRuns, convertToActionsCheckRunSummary(checkRun))
		}
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal check runs: %w", err)
	}
	return utils.NewToolResultText(string(raw)), &ActionsListOutput{Method: actionsMethodListCheckRuns, CheckRuns: output}, nil
}

func getActionsCheckRun(ctx context.Context, client *github.Client, owner, repo string, checkRunID int64) (*mcp.CallToolResult, *ActionsGetOutput, error) {
	checkRun, resp, err := client.Checks.GetCheckRun(ctx, owner, repo, checkRunID)
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get check run", resp, err), nil, nil
	}
	if checkRun == nil {
		return utils.NewToolResultError("GitHub returned no check run"), nil, nil
	}

	output := &ActionsCheckRun{ActionsCheckRunSummary: convertToActionsCheckRunSummary(checkRun)}
	if details := checkRun.GetOutput(); details != nil {
		output.Output = &ActionsCheckRunOutput{
			Title:            sanitize.Content(details.GetTitle()),
			Summary:          sanitize.Content(details.GetSummary()),
			Text:             sanitize.Content(details.GetText()),
			AnnotationsCount: details.GetAnnotationsCount(),
		}
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal check run: %w", err)
	}
	return utils.NewToolResultText(string(raw)), &ActionsGetOutput{Method: actionsMethodGetCheckRun, CheckRun: output}, nil
}

func listActionsCheckRunAnnotations(ctx context.Context, client *github.Client, owner, repo string, checkRunID int64, pagination PaginationParams) (*mcp.CallToolResult, *ActionsListOutput, error) {
	annotations, resp, err := client.Checks.ListCheckRunAnnotations(ctx, owner, repo, checkRunID, &github.ListOptions{
		Page: pagination.Page, PerPage: pagination.PerPage,
	})
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list check run annotations", resp, err), nil, nil
	}

	output := &ActionsCheckRunAnnotationsOutput{
		Annotations: make([]ActionsCheckRunAnnotation, 0, len(annotations)),
		NextPage:    resp.NextPage,
	}
	for _, annotation := range annotations {
		if annotation != nil {
			output.Annotations = append(output.Annotations, ActionsCheckRunAnnotation{
				Path:            annotation.GetPath(),
				StartLine:       annotation.GetStartLine(),
				EndLine:         annotation.GetEndLine(),
				StartColumn:     annotation.StartColumn,
				EndColumn:       annotation.EndColumn,
				AnnotationLevel: annotation.GetAnnotationLevel(),
				Title:           sanitize.Content(annotation.GetTitle()),
				Message:         sanitize.Content(annotation.GetMessage()),
				RawDetails:      sanitize.Content(annotation.GetRawDetails()),
			})
		}
	}
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal check run annotations: %w", err)
	}
	return utils.NewToolResultText(string(raw)), &ActionsListOutput{Method: actionsMethodListCheckRunAnnotations, CheckRunAnnotations: output}, nil
}
