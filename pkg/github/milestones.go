package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	milestoneStateOpen   = "open"
	milestoneStateClosed = "closed"
)

// ListMilestones lists milestones for a repository.
func ListMilestones(getClient GetClientFn, cache *lockdown.RepoAccessCache, t translations.TranslationHelperFunc, flags FeatureFlags) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	tool := mcp.Tool{
		Name:        "list_milestones",
		Description: t("TOOL_LIST_MILESTONES_DESCRIPTION", "List milestones for a repository."),
		Annotations: &mcp.ToolAnnotations{
			Title:        t("TOOL_LIST_MILESTONES_TITLE", "List repository milestones."),
			ReadOnlyHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {
					Type:        "string",
					Description: "Repository owner (username or organization name)",
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
				},
				"state": {
					Type:        "string",
					Description: "Filter by state: open, closed, or all",
					Enum:        []any{milestoneStateOpen, milestoneStateClosed, "all"},
				},
				"sort": {
					Type:        "string",
					Description: "Sort field: due_on or completeness",
					Enum:        []any{"due_on", "completeness"},
				},
				"direction": {
					Type:        "string",
					Description: "Sort direction: asc or desc",
					Enum:        []any{"asc", "desc"},
				},
				"per_page": {
					Type:        "number",
					Description: "Results per page (max 100)",
				},
				"page": {
					Type:        "number",
					Description: "Page number (1-indexed)",
				},
			},
			Required: []string{"owner", "repo"},
		},
	}

	handler := mcp.ToolHandlerFor[map[string]any, any](func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, err := RequiredParam[string](args, "owner")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		repo, err := RequiredParam[string](args, "repo")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}

		state, err := OptionalParam[string](args, "state")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		if state != "" && state != milestoneStateOpen && state != milestoneStateClosed && state != "all" {
			return utils.NewToolResultError("state must be 'open', 'closed', or 'all'"), nil, nil
		}

		sort, err := OptionalParam[string](args, "sort")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		if sort != "" && sort != "due_on" && sort != "completeness" {
			return utils.NewToolResultError("sort must be 'due_on' or 'completeness'"), nil, nil
		}

		direction, err := OptionalParam[string](args, "direction")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		if direction != "" && direction != "asc" && direction != "desc" {
			return utils.NewToolResultError("direction must be 'asc' or 'desc'"), nil, nil
		}

		perPage, err := OptionalIntParam(args, "per_page")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		page, err := OptionalIntParam(args, "page")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}

		client, err := getClient(ctx)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
		}

		opts := &github.MilestoneListOptions{
			State:     state,
			Sort:      sort,
			Direction: direction,
		}
		if perPage > 0 {
			opts.ListOptions.PerPage = perPage
		}
		if page > 0 {
			opts.ListOptions.Page = page
		}

		milestones, resp, err := client.Issues.ListMilestones(ctx, owner, repo, opts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx,
				"failed to list milestones",
				resp,
				err,
			), nil, nil
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read response body: %w", err)
			}
			return utils.NewToolResultError(fmt.Sprintf("failed to list milestones: %s", string(body))), nil, nil
		}

		if flags.LockdownMode {
			if cache == nil {
				return nil, nil, fmt.Errorf("lockdown cache is not configured")
			}
			filtered := make([]*github.Milestone, 0, len(milestones))
			for _, milestone := range milestones {
				creator := milestone.Creator
				if creator == nil || creator.GetLogin() == "" {
					filtered = append(filtered, milestone)
					continue
				}
				isSafeContent, err := cache.IsSafeContent(ctx, creator.GetLogin(), owner, repo)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("failed to check lockdown mode: %v", err)), nil, nil
				}
				if isSafeContent {
					filtered = append(filtered, milestone)
				}
			}
			milestones = filtered
		}

		result := make([]map[string]any, 0, len(milestones))
		for _, m := range milestones {
			result = append(result, milestoneSummary(m))
		}

		payload := map[string]any{
			"milestones": result,
			"count":      len(result),
		}
		out, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal milestones: %w", err)
		}

		return utils.NewToolResultText(string(out)), nil, nil
	})

	return tool, handler
}

// GetMilestone fetches a single milestone by number.
func GetMilestone(getClient GetClientFn, cache *lockdown.RepoAccessCache, t translations.TranslationHelperFunc, flags FeatureFlags) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	tool := mcp.Tool{
		Name:        "get_milestone",
		Description: t("TOOL_GET_MILESTONE_DESCRIPTION", "Get a milestone by number."),
		Annotations: &mcp.ToolAnnotations{
			Title:        t("TOOL_GET_MILESTONE_TITLE", "Get repository milestone."),
			ReadOnlyHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {
					Type:        "string",
					Description: "Repository owner (username or organization name)",
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
				},
				"milestone_number": {
					Type:        "number",
					Description: "Milestone number to fetch",
				},
			},
			Required: []string{"owner", "repo", "milestone_number"},
		},
	}

	handler := mcp.ToolHandlerFor[map[string]any, any](func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, err := RequiredParam[string](args, "owner")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		repo, err := RequiredParam[string](args, "repo")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		number, err := RequiredInt(args, "milestone_number")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}

		client, err := getClient(ctx)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
		}

		milestone, resp, err := client.Issues.GetMilestone(ctx, owner, repo, number)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx,
				"failed to get milestone",
				resp,
				err,
			), nil, nil
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read response body: %w", err)
			}
			return utils.NewToolResultError(fmt.Sprintf("failed to get milestone: %s", string(body))), nil, nil
		}

		if flags.LockdownMode {
			if cache == nil {
				return nil, nil, fmt.Errorf("lockdown cache is not configured")
			}
			creator := milestone.Creator
			if creator != nil && creator.GetLogin() != "" {
				isSafeContent, err := cache.IsSafeContent(ctx, creator.GetLogin(), owner, repo)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("failed to check lockdown mode: %v", err)), nil, nil
				}
				if !isSafeContent {
					return utils.NewToolResultError("access to milestone is restricted by lockdown mode"), nil, nil
				}
			}
		}

		out, err := json.Marshal(milestoneSummary(milestone))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal milestone: %w", err)
		}

		return utils.NewToolResultText(string(out)), nil, nil
	})

	return tool, handler
}

func MilestoneWrite(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	tool := mcp.Tool{
		Name:        "milestone_write",
		Description: t("TOOL_MILESTONE_WRITE_DESCRIPTION", "Create, update, or delete milestones in a repository."),
		Annotations: &mcp.ToolAnnotations{
			Title:        t("TOOL_MILESTONE_WRITE_TITLE", "Write operations on repository milestones."),
			ReadOnlyHint: false,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"method": {
					Type:        "string",
					Description: "Operation to perform: 'create', 'update', or 'delete'",
					Enum:        []any{"create", "update", "delete"},
				},
				"owner": {
					Type:        "string",
					Description: "Repository owner (username or organization name)",
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
				},
				"title": {
					Type:        "string",
					Description: "Milestone title (required for create)",
				},
				"description": {
					Type:        "string",
					Description: "Milestone description",
				},
				"state": {
					Type:        "string",
					Description: "Milestone state: 'open' or 'closed'",
					Enum:        []any{milestoneStateOpen, milestoneStateClosed},
				},
				"due_on": {
					Type:        "string",
					Description: "Due date in ISO-8601 date (YYYY-MM-DD) or RFC3339 timestamp",
				},
				"milestone_number": {
					Type:        "number",
					Description: "Milestone number to update or delete",
				},
			},
			Required: []string{"method", "owner", "repo"},
		},
	}

	handler := mcp.ToolHandlerFor[map[string]any, any](func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		method, err := RequiredParam[string](args, "method")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		method = strings.ToLower(method)

		owner, err := RequiredParam[string](args, "owner")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		repo, err := RequiredParam[string](args, "repo")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}

		title, err := OptionalParam[string](args, "title")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		description, err := OptionalParam[string](args, "description")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		state, err := OptionalParam[string](args, "state")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		if state != "" && state != milestoneStateOpen && state != milestoneStateClosed {
			return utils.NewToolResultError("state must be 'open' or 'closed'"), nil, nil
		}
		dueOnRaw, err := OptionalParam[string](args, "due_on")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		dueOn, err := parseDueOn(dueOnRaw)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}

		client, err := getClient(ctx)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
		}

		switch method {
		case "create":
			if title == "" {
				return utils.NewToolResultError("missing required parameter: title"), nil, nil
			}
			return createMilestone(ctx, client, owner, repo, title, description, state, dueOn)
		case "update":
			milestoneNumber, err := RequiredInt(args, "milestone_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if title == "" && description == "" && state == "" && dueOn == nil {
				return utils.NewToolResultError("at least one of title, description, state, or due_on must be provided for update"), nil, nil
			}
			return updateMilestone(ctx, client, owner, repo, milestoneNumber, title, description, state, dueOn)
		case "delete":
			milestoneNumber, err := RequiredInt(args, "milestone_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			return deleteMilestone(ctx, client, owner, repo, milestoneNumber)
		default:
			return utils.NewToolResultError("invalid method, must be either 'create', 'update', or 'delete'"), nil, nil
		}
	})

	return tool, handler
}

func createMilestone(ctx context.Context, client *github.Client, owner, repo, title, description, state string, dueOn *time.Time) (*mcp.CallToolResult, any, error) {
	req := &github.Milestone{
		Title: github.Ptr(title),
	}

	if description != "" {
		req.Description = github.Ptr(description)
	}
	if state != "" {
		req.State = github.Ptr(state)
	}
	if dueOn != nil {
		req.DueOn = &github.Timestamp{Time: *dueOn}
	}

	milestone, resp, err := client.Issues.CreateMilestone(ctx, owner, repo, req)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to create milestone",
			resp,
			err,
		), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return utils.NewToolResultError(fmt.Sprintf("failed to create milestone: %s", string(body))), nil, nil
	}

	return marshalMilestoneResponse(milestone)
}

func updateMilestone(ctx context.Context, client *github.Client, owner, repo string, number int, title, description, state string, dueOn *time.Time) (*mcp.CallToolResult, any, error) {
	req := &github.Milestone{}
	if title != "" {
		req.Title = github.Ptr(title)
	}
	if description != "" {
		req.Description = github.Ptr(description)
	}
	if state != "" {
		req.State = github.Ptr(state)
	}
	if dueOn != nil {
		req.DueOn = &github.Timestamp{Time: *dueOn}
	}

	milestone, resp, err := client.Issues.EditMilestone(ctx, owner, repo, number, req)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to update milestone",
			resp,
			err,
		), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return utils.NewToolResultError(fmt.Sprintf("failed to update milestone: %s", string(body))), nil, nil
	}

	return marshalMilestoneResponse(milestone)
}

func deleteMilestone(ctx context.Context, client *github.Client, owner, repo string, number int) (*mcp.CallToolResult, any, error) {
	resp, err := client.Issues.DeleteMilestone(ctx, owner, repo, number)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to delete milestone",
			resp,
			err,
		), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return utils.NewToolResultError(fmt.Sprintf("failed to delete milestone: %s", string(body))), nil, nil
	}

	return utils.NewToolResultText(fmt.Sprintf("milestone %d deleted", number)), nil, nil
}

func milestoneSummary(milestone *github.Milestone) map[string]any {
	dueOn := ""
	if milestone.DueOn != nil {
		dueOn = milestone.DueOn.Time.Format(time.RFC3339)
	}

	return map[string]any{
		"id":            fmt.Sprintf("%d", milestone.GetID()),
		"number":        milestone.GetNumber(),
		"title":         milestone.GetTitle(),
		"state":         milestone.GetState(),
		"description":   milestone.GetDescription(),
		"due_on":        dueOn,
		"open_issues":   milestone.GetOpenIssues(),
		"closed_issues": milestone.GetClosedIssues(),
		"url":           milestone.GetHTMLURL(),
	}
}

func marshalMilestoneResponse(milestone *github.Milestone) (*mcp.CallToolResult, any, error) {
	minimalResponse := map[string]any{
		"id":     fmt.Sprintf("%d", milestone.GetID()),
		"number": milestone.GetNumber(),
		"url":    milestone.GetHTMLURL(),
	}

	out, err := json.Marshal(minimalResponse)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(out)), nil, nil
}

func parseDueOn(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}

	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return &ts, nil
	}

	if ts, err := time.Parse("2006-01-02", value); err == nil {
		return &ts, nil
	}

	return nil, fmt.Errorf("invalid due_on format; use YYYY-MM-DD or RFC3339 timestamp")
}
