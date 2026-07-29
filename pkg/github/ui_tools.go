package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// uiGetMaxPages bounds how many pages each ui_get pagination loop will fetch.
// ui_get backs a synchronous UI picker (dropdowns for labels/assignees/etc. in
// the MCP App issue/PR write surfaces), so responsiveness matters more than
// completeness. At PerPage 100 this caps a call at ~1000 items and a bounded
// number of API round-trips, keeping latency predictable on very large
// repos/orgs. Results past the cap are truncated and surfaced via a "has_more"
// flag, which is acceptable because the picker pairs truncation with typeahead.
const uiGetMaxPages = 10

// uiGetMethods is the single source for ui_get's schema enum (methodEnum) and its
// unknown-method error (unknownMethodError).
var uiGetMethods = []string{"labels", "assignees", "milestones", "issue_types", "branches", "issue_fields", "reviewers"}

// UIGet creates a tool to fetch UI data for MCP Apps.
func UIGet(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataContext, // Use context toolset so it's always available
		mcp.Tool{
			Name:        "ui_get",
			Description: t("TOOL_UI_GET_DESCRIPTION", "Fetch UI data for MCP Apps (labels, assignees, milestones, issue types, branches, issue fields, reviewers)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UI_GET_USER_TITLE", "Get UI data"),
				ReadOnlyHint: true,
			},
			// ui_get only backs MCP App views; declaring app-only visibility keeps
			// it out of the agent's tool list while remaining callable by the views
			// via tools/call (per the MCP Apps 2026-01-26 spec).
			Meta: mcp.Meta{
				"ui": map[string]any{
					"visibility": []string{"app"},
				},
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Enum:        methodEnum(uiGetMethods),
						Description: "The type of data to fetch",
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (required for all methods)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name (required for labels, assignees, milestones, branches, issue fields, reviewers)",
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
			case "issue_fields":
				return uiGetIssueFields(ctx, deps, args, owner)
			case "reviewers":
				return uiGetReviewers(ctx, deps, args, owner)
			default:
				return unknownMethodError(method, uiGetMethods), nil, nil
			}
		})
	st.FeatureFlagEnable = MCPAppsFeatureFlag
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
				PageInfo   struct {
					HasNextPage githubv4.Boolean
					EndCursor   githubv4.String
				}
			} `graphql:"labels(first: 100, after: $cursor)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	vars := map[string]any{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"cursor": (*githubv4.String)(nil),
	}

	labels := make([]map[string]any, 0)
	var totalCount int
	hasMore := false
	for page := 1; ; page++ {
		if err := client.Query(ctx, &query, vars); err != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to list labels", err), nil, nil
		}
		for _, labelNode := range query.Repository.Labels.Nodes {
			labels = append(labels, map[string]any{
				"id":          fmt.Sprintf("%v", labelNode.ID),
				"name":        string(labelNode.Name),
				"color":       string(labelNode.Color),
				"description": string(labelNode.Description),
			})
		}
		totalCount = int(query.Repository.Labels.TotalCount)
		if !query.Repository.Labels.PageInfo.HasNextPage {
			break
		}
		if page >= uiGetMaxPages {
			hasMore = true
			break
		}
		vars["cursor"] = githubv4.NewString(query.Repository.Labels.PageInfo.EndCursor)
	}

	response := map[string]any{
		"labels":     labels,
		"totalCount": totalCount,
		"has_more":   hasMore,
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
	hasMore := false

	for page := 1; ; page++ {
		assignees, resp, err := client.Issues.ListAssignees(ctx, owner, repo, opts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list assignees", resp, err), nil, nil
		}
		allAssignees = append(allAssignees, assignees...)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if resp.NextPage == 0 {
			break
		}
		if page >= uiGetMaxPages {
			hasMore = true
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
		"has_more":   hasMore,
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
	hasMore := false
	for page := 1; ; page++ {
		milestones, resp, err := client.Issues.ListMilestones(ctx, owner, repo, opts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list milestones", resp, err), nil, nil
		}
		allMilestones = append(allMilestones, milestones...)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if resp.NextPage == 0 {
			break
		}
		if page >= uiGetMaxPages {
			hasMore = true
			break
		}
		opts.Page = resp.NextPage
	}

	result := make([]map[string]any, len(allMilestones))
	for i, m := range allMilestones {
		dueOn := ""
		if m.DueOn != nil {
			dueOn = m.GetDueOn().Format("2006-01-02")
		}
		result[i] = map[string]any{
			"number":      m.GetNumber(),
			"title":       m.GetTitle(),
			"description": m.GetDescription(),
			"state":       m.GetState(),
			"open_issues": m.GetOpenIssues(),
			"due_on":      dueOn,
		}
	}

	out, err := json.Marshal(map[string]any{
		"milestones": result,
		"totalCount": len(result),
		"has_more":   hasMore,
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
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list issue types", resp, err), nil, nil
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
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
	}

	opts := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allBranches []*github.Branch
	hasMore := false
	for page := 1; ; page++ {
		branches, resp, err := client.Repositories.ListBranches(ctx, owner, repo, opts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list branches", resp, err), nil, nil
		}
		allBranches = append(allBranches, branches...)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if resp.NextPage == 0 {
			break
		}
		if page >= uiGetMaxPages {
			hasMore = true
			break
		}
		opts.Page = resp.NextPage
	}

	minimalBranches := make([]MinimalBranch, 0, len(allBranches))
	for _, branch := range allBranches {
		minimalBranches = append(minimalBranches, convertToMinimalBranch(branch))
	}

	r, err := json.Marshal(map[string]any{
		"branches":   minimalBranches,
		"totalCount": len(minimalBranches),
		"has_more":   hasMore,
	})
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func uiGetIssueFields(ctx context.Context, deps ToolDependencies, args map[string]any, owner string) (*mcp.CallToolResult, any, error) {
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	gqlClient, err := deps.GetGQLClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
	}

	fields, err := fetchIssueFields(ctx, gqlClient, owner, repo)
	if err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to list issue fields", err), nil, nil
	}

	return marshalUIGetIssueFields(fields)
}

func marshalUIGetIssueFields(fields []IssueField) (*mcp.CallToolResult, any, error) {
	resultFields := make([]map[string]any, 0, len(fields))
	for _, field := range fields {
		if !uiSupportedIssueFieldDataType(field.DataType) {
			continue
		}

		fieldResult := map[string]any{
			"id":          field.ID,
			"name":        field.Name,
			"data_type":   field.DataType,
			"description": field.Description,
		}

		if field.DataType == "single_select" {
			fieldOptions := append([]IssueSingleSelectFieldOption(nil), field.Options...)
			sort.SliceStable(fieldOptions, func(i, j int) bool {
				left, leftOK := issueFieldOptionPriority(fieldOptions[i])
				right, rightOK := issueFieldOptionPriority(fieldOptions[j])
				if leftOK != rightOK {
					return leftOK
				}
				return left < right
			})

			options := make([]map[string]string, 0, len(fieldOptions))
			for _, option := range fieldOptions {
				options = append(options, map[string]string{
					"name":        option.Name,
					"description": option.Description,
					"color":       option.Color,
				})
			}
			fieldResult["options"] = options
		}

		resultFields = append(resultFields, fieldResult)
	}

	r, err := json.Marshal(map[string]any{
		"fields":     resultFields,
		"totalCount": len(resultFields),
	})
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal issue fields", err), nil, nil
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func uiSupportedIssueFieldDataType(dataType string) bool {
	switch dataType {
	case "text", "number", "date", "single_select":
		return true
	default:
		return false
	}
}

func issueFieldOptionPriority(option IssueSingleSelectFieldOption) (int, bool) {
	if option.Priority == nil {
		return 0, false
	}
	return *option.Priority, true
}

func uiGetReviewers(ctx context.Context, deps ToolDependencies, args map[string]any, owner string) (*mcp.CallToolResult, any, error) {
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
	}

	collaboratorOpts := &github.ListCollaboratorsOptions{
		Affiliation: "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var allCollaborators []*github.User
	hasMore := false
	for page := 1; ; page++ {
		collaborators, resp, err := client.Repositories.ListCollaborators(ctx, owner, repo, collaboratorOpts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list reviewers", resp, err), nil, nil
		}
		allCollaborators = append(allCollaborators, collaborators...)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if resp.NextPage == 0 {
			break
		}
		if page >= uiGetMaxPages {
			hasMore = true
			break
		}
		collaboratorOpts.Page = resp.NextPage
	}

	teamOpts := &github.ListOptions{PerPage: 100}
	var allTeams []*github.Team
	for page := 1; ; page++ {
		teams, resp, err := client.Repositories.ListTeams(ctx, owner, repo, teamOpts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list reviewer teams", resp, err), nil, nil
		}
		allTeams = append(allTeams, teams...)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if resp.NextPage == 0 {
			break
		}
		if page >= uiGetMaxPages {
			hasMore = true
			break
		}
		teamOpts.Page = resp.NextPage
	}

	users := make([]map[string]string, 0, len(allCollaborators))
	for _, user := range allCollaborators {
		login := user.GetLogin()
		if user.GetType() == "Bot" || strings.HasSuffix(login, "[bot]") {
			continue
		}
		users = append(users, map[string]string{
			"login":      login,
			"avatar_url": user.GetAvatarURL(),
		})
	}

	teams := make([]map[string]string, len(allTeams))
	for i, team := range allTeams {
		teams[i] = map[string]string{
			"slug": team.GetSlug(),
			"name": team.GetName(),
			"org":  owner,
		}
	}

	r, err := json.Marshal(map[string]any{
		"users":      users,
		"teams":      teams,
		"totalCount": len(users) + len(teams),
		"has_more":   hasMore,
	})
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal reviewers", err), nil, nil
	}

	return utils.NewToolResultText(string(r)), nil, nil
}
