package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// UIGet creates a tool to fetch UI data for MCP Apps.
func UIGet(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataContext, // Use context toolset so it's always available
		mcp.Tool{
			Name:        "ui_get",
			Description: t("TOOL_UI_GET_DESCRIPTION", "Fetch UI data for MCP Apps (labels, assignees, milestones, issue types, branches)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UI_GET_USER_TITLE", "Get UI data"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Enum:        []any{"labels", "assignees", "milestones", "issue_types", "branches"},
						Description: "The type of data to fetch",
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (required for all methods)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name (required for labels, assignees, milestones, branches)",
					},
				},
				Required: []string{"method", "owner"},
			},
		},
		[]scopes.Scope{scopes.Repo, scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			switch method {
			case "labels":
				return uiGetLabels(ctx, deps, args, owner)
			case "assignees":
				return uiGetAssignees(ctx, deps, args, owner)
			case "milestones":
				return uiGetMilestones(ctx, deps, args, owner)
			case "issue_types":
				return uiGetIssueTypes(ctx, deps, owner)
			case "branches":
				return uiGetBranches(ctx, deps, args, owner)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		})
	st.InsidersOnly = true
	return st
}

func uiGetLabels(ctx context.Context, deps ToolDependencies, args map[string]any, owner string) (*mcp.CallToolResult, any, error) {
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	client, err := deps.GetGQLClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
	}

	var query struct {
		Repository struct {
			Labels struct {
				Nodes []struct {
					ID          githubv4.ID
					Name        githubv4.String
					Color       githubv4.String
					Description githubv4.String
				}
				TotalCount githubv4.Int
			} `graphql:"labels(first: 100)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	vars := map[string]any{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
	}

	if err := client.Query(ctx, &query, vars); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to list labels", err), nil, nil
	}

	labels := make([]map[string]any, len(query.Repository.Labels.Nodes))
	for i, labelNode := range query.Repository.Labels.Nodes {
		labels[i] = map[string]any{
			"id":          fmt.Sprintf("%v", labelNode.ID),
			"name":        string(labelNode.Name),
			"color":       string(labelNode.Color),
			"description": string(labelNode.Description),
		}
	}

	response := map[string]any{
		"labels":     labels,
		"totalCount": int(query.Repository.Labels.TotalCount),
	}

	out, err := json.Marshal(response)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal labels: %w", err)
	}

	return utils.NewToolResultText(string(out)), nil, nil
}

func uiGetAssignees(ctx context.Context, deps ToolDependencies, args map[string]any, owner string) (*mcp.CallToolResult, any, error) {
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
	}

	opts := &github.ListOptions{PerPage: 100}
	var allAssignees []*github.User

	for {
		assignees, resp, err := client.Issues.ListAssignees(ctx, owner, repo, opts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list assignees", resp, err), nil, nil
		}
		allAssignees = append(allAssignees, assignees...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	result := make([]map[string]string, len(allAssignees))
	for i, u := range allAssignees {
		result[i] = map[string]string{
			"login":      u.GetLogin(),
			"avatar_url": u.GetAvatarURL(),
		}
	}

	out, err := json.Marshal(map[string]any{
		"assignees":  result,
		"totalCount": len(result),
	})
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal assignees", err), nil, nil
	}

	return utils.NewToolResultText(string(out)), nil, nil
}

func uiGetMilestones(ctx context.Context, deps ToolDependencies, args map[string]any, owner string) (*mcp.CallToolResult, any, error) {
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
	}

	opts := &github.MilestoneListOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allMilestones []*github.Milestone
	for {
		milestones, resp, err := client.Issues.ListMilestones(ctx, owner, repo, opts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list milestones", resp, err), nil, nil
		}
		allMilestones = append(allMilestones, milestones...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	result := make([]map[string]any, len(allMilestones))
	for i, m := range allMilestones {
		result[i] = map[string]any{
			"number":      m.GetNumber(),
			"title":       m.GetTitle(),
			"description": m.GetDescription(),
			"state":       m.GetState(),
			"open_issues": m.GetOpenIssues(),
			"due_on":      m.GetDueOn().Format("2006-01-02"),
		}
	}

	out, err := json.Marshal(map[string]any{
		"milestones": result,
		"totalCount": len(result),
	})
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal milestones", err), nil, nil
	}

	return utils.NewToolResultText(string(out)), nil, nil
}

func uiGetIssueTypes(ctx context.Context, deps ToolDependencies, owner string) (*mcp.CallToolResult, any, error) {
	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
	}

	issueTypes, resp, err := client.Organizations.ListIssueTypes(ctx, owner)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to list issue types", err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list issue types", resp, body), nil, nil
	}

	r, err := json.Marshal(issueTypes)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal issue types", err), nil, nil
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func uiGetBranches(ctx context.Context, deps ToolDependencies, args map[string]any, owner string) (*mcp.CallToolResult, any, error) {
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	client, err := deps.GetClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
	}

	opts := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	branches, resp, err := client.Repositories.ListBranches(ctx, owner, repo, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list branches", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list branches", resp, body), nil, nil
	}

	minimalBranches := make([]MinimalBranch, 0, len(branches))
	for _, branch := range branches {
		minimalBranches = append(minimalBranches, convertToMinimalBranch(branch))
	}

	r, err := json.Marshal(map[string]any{
		"branches":   minimalBranches,
		"totalCount": len(minimalBranches),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}
