package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// issueUpdateTool is a helper to create single-field issue update tools.
func issueUpdateTool(
	t translations.TranslationHelperFunc,
	name, description, title string,
	extraProps map[string]*jsonschema.Schema,
	extraRequired []string,
	buildRequest func(args map[string]any) (*github.IssueRequest, error),
) inventory.ServerTool {
	props := map[string]*jsonschema.Schema{
		"owner": {
			Type:        "string",
			Description: "Repository owner (username or organization)",
		},
		"repo": {
			Type:        "string",
			Description: "Repository name",
		},
		"issue_number": {
			Type:        "number",
			Description: "The issue number to update",
			Minimum:     jsonschema.Ptr(1.0),
		},
	}
	maps.Copy(props, extraProps)

	required := append([]string{"owner", "repo", "issue_number"}, extraRequired...)

	return NewTool(
		ToolsetMetadataIssuesGranular,
		mcp.Tool{
			Name:        name,
			Description: t("TOOL_"+name+"_DESCRIPTION", description),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_"+name+"_USER_TITLE", title),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: props,
				Required:   required,
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			issueReq, err := buildRequest(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			issue, _, err := client.Issues.Edit(ctx, owner, repo, issueNumber, issueReq)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to update issue", err), nil, nil
			}

			r, err := json.Marshal(issue)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GranularCreateIssue creates a tool to create a new issue.
func GranularCreateIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssuesGranular,
		mcp.Tool{
			Name:        "create_issue",
			Description: t("TOOL_CREATE_ISSUE_DESCRIPTION", "Create a new issue in a GitHub repository with a title and optional body."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_CREATE_ISSUE_USER_TITLE", "Create Issue"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"title": {
						Type:        "string",
						Description: "Issue title",
					},
					"body": {
						Type:        "string",
						Description: "Issue body content (optional)",
					},
				},
				Required: []string{"owner", "repo", "title"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			title, err := RequiredParam[string](args, "title")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			body, _ := OptionalParam[string](args, "body")

			issueReq := &github.IssueRequest{
				Title: &title,
			}
			if body != "" {
				issueReq.Body = &body
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			issue, _, err := client.Issues.Create(ctx, owner, repo, issueReq)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create issue", err), nil, nil
			}

			r, err := json.Marshal(issue)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GranularUpdateIssueTitle creates a tool to update an issue's title.
func GranularUpdateIssueTitle(t translations.TranslationHelperFunc) inventory.ServerTool {
	return issueUpdateTool(t,
		"update_issue_title",
		"Update the title of an existing issue.",
		"Update Issue Title",
		map[string]*jsonschema.Schema{
			"title": {Type: "string", Description: "The new title for the issue"},
		},
		[]string{"title"},
		func(args map[string]any) (*github.IssueRequest, error) {
			title, err := RequiredParam[string](args, "title")
			if err != nil {
				return nil, err
			}
			return &github.IssueRequest{Title: &title}, nil
		},
	)
}

// GranularUpdateIssueBody creates a tool to update an issue's body.
func GranularUpdateIssueBody(t translations.TranslationHelperFunc) inventory.ServerTool {
	return issueUpdateTool(t,
		"update_issue_body",
		"Update the body content of an existing issue.",
		"Update Issue Body",
		map[string]*jsonschema.Schema{
			"body": {Type: "string", Description: "The new body content for the issue"},
		},
		[]string{"body"},
		func(args map[string]any) (*github.IssueRequest, error) {
			body, err := RequiredParam[string](args, "body")
			if err != nil {
				return nil, err
			}
			return &github.IssueRequest{Body: &body}, nil
		},
	)
}

// GranularUpdateIssueAssignees creates a tool to update an issue's assignees.
func GranularUpdateIssueAssignees(t translations.TranslationHelperFunc) inventory.ServerTool {
	return issueUpdateTool(t,
		"update_issue_assignees",
		"Update the assignees of an existing issue. This replaces the current assignees with the provided list.",
		"Update Issue Assignees",
		map[string]*jsonschema.Schema{
			"assignees": {
				Type:        "array",
				Description: "GitHub usernames to assign to this issue",
				Items:       &jsonschema.Schema{Type: "string"},
			},
		},
		[]string{"assignees"},
		func(args map[string]any) (*github.IssueRequest, error) {
			raw, _ := OptionalParam[[]any](args, "assignees")
			if len(raw) == 0 {
				return nil, fmt.Errorf("missing required parameter: assignees")
			}
			assignees := make([]string, 0, len(raw))
			for _, v := range raw {
				if s, ok := v.(string); ok {
					assignees = append(assignees, s)
				}
			}
			return &github.IssueRequest{Assignees: &assignees}, nil
		},
	)
}

// GranularUpdateIssueLabels creates a tool to update an issue's labels.
func GranularUpdateIssueLabels(t translations.TranslationHelperFunc) inventory.ServerTool {
	return issueUpdateTool(t,
		"update_issue_labels",
		"Update the labels of an existing issue. This replaces the current labels with the provided list.",
		"Update Issue Labels",
		map[string]*jsonschema.Schema{
			"labels": {
				Type:        "array",
				Description: "Labels to apply to this issue",
				Items:       &jsonschema.Schema{Type: "string"},
			},
		},
		[]string{"labels"},
		func(args map[string]any) (*github.IssueRequest, error) {
			raw, _ := OptionalParam[[]any](args, "labels")
			if len(raw) == 0 {
				return nil, fmt.Errorf("missing required parameter: labels")
			}
			labels := make([]string, 0, len(raw))
			for _, v := range raw {
				if s, ok := v.(string); ok {
					labels = append(labels, s)
				}
			}
			return &github.IssueRequest{Labels: &labels}, nil
		},
	)
}

// GranularUpdateIssueMilestone creates a tool to update an issue's milestone.
func GranularUpdateIssueMilestone(t translations.TranslationHelperFunc) inventory.ServerTool {
	return issueUpdateTool(t,
		"update_issue_milestone",
		"Update the milestone of an existing issue.",
		"Update Issue Milestone",
		map[string]*jsonschema.Schema{
			"milestone": {
				Type:        "number",
				Description: "The milestone number to set on the issue",
				Minimum:     jsonschema.Ptr(0.0),
			},
		},
		[]string{"milestone"},
		func(args map[string]any) (*github.IssueRequest, error) {
			milestone, err := RequiredParam[float64](args, "milestone")
			if err != nil {
				return nil, err
			}
			m := int(milestone)
			return &github.IssueRequest{Milestone: &m}, nil
		},
	)
}

// GranularUpdateIssueType creates a tool to update an issue's type.
func GranularUpdateIssueType(t translations.TranslationHelperFunc) inventory.ServerTool {
	return issueUpdateTool(t,
		"update_issue_type",
		"Update the type of an existing issue (e.g. 'bug', 'feature').",
		"Update Issue Type",
		map[string]*jsonschema.Schema{
			"issue_type": {
				Type:        "string",
				Description: "The issue type to set",
			},
		},
		[]string{"issue_type"},
		func(args map[string]any) (*github.IssueRequest, error) {
			issueType, err := RequiredParam[string](args, "issue_type")
			if err != nil {
				return nil, err
			}
			return &github.IssueRequest{Type: &issueType}, nil
		},
	)
}

// GranularUpdateIssueState creates a tool to update an issue's state.
func GranularUpdateIssueState(t translations.TranslationHelperFunc) inventory.ServerTool {
	return issueUpdateTool(t,
		"update_issue_state",
		"Update the state of an existing issue (open or closed), with an optional state reason.",
		"Update Issue State",
		map[string]*jsonschema.Schema{
			"state": {
				Type:        "string",
				Description: "The new state for the issue",
				Enum:        []any{"open", "closed"},
			},
			"state_reason": {
				Type:        "string",
				Description: "The reason for the state change (only for closed state)",
				Enum:        []any{"completed", "not_planned", "duplicate"},
			},
		},
		[]string{"state"},
		func(args map[string]any) (*github.IssueRequest, error) {
			state, err := RequiredParam[string](args, "state")
			if err != nil {
				return nil, err
			}
			req := &github.IssueRequest{State: &state}

			stateReason, _ := OptionalParam[string](args, "state_reason")
			if stateReason != "" {
				req.StateReason = &stateReason
			}
			return req, nil
		},
	)
}

// GranularAddSubIssue creates a tool to add a sub-issue.
func GranularAddSubIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssuesGranular,
		mcp.Tool{
			Name:        "add_sub_issue",
			Description: t("TOOL_ADD_SUB_ISSUE_DESCRIPTION", "Add a sub-issue to a parent issue."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ADD_SUB_ISSUE_USER_TITLE", "Add Sub-Issue"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"issue_number": {
						Type:        "number",
						Description: "The parent issue number",
						Minimum:     jsonschema.Ptr(1.0),
					},
					"sub_issue_id": {
						Type:        "number",
						Description: "The global node ID of the issue to add as a sub-issue",
					},
					"replace_parent": {
						Type:        "boolean",
						Description: "If true, reparent the sub-issue if it already has a parent",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "sub_issue_id"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			subIssueID, err := RequiredParam[float64](args, "sub_issue_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			replaceParent, _ := OptionalParam[bool](args, "replace_parent")

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			parentNodeID, err := getGranularIssueNodeID(ctx, gqlClient, owner, repo, issueNumber)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get parent issue", err), nil, nil
			}

			var mutation struct {
				AddSubIssue struct {
					Issue struct {
						ID    string
						Title string
						URL   string
					}
					SubIssue struct {
						ID    string
						Title string
						URL   string
					}
				} `graphql:"addSubIssue(input: $input)"`
			}

			input := GranularAddSubIssueInput{
				IssueID:       parentNodeID,
				SubIssueID:    fmt.Sprintf("%d", int(subIssueID)),
				ReplaceParent: githubv4.Boolean(replaceParent),
			}

			if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
				return utils.NewToolResultErrorFromErr("failed to add sub-issue", err), nil, nil
			}

			r, err := json.Marshal(mutation.AddSubIssue)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GranularRemoveSubIssue creates a tool to remove a sub-issue.
func GranularRemoveSubIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssuesGranular,
		mcp.Tool{
			Name:        "remove_sub_issue",
			Description: t("TOOL_REMOVE_SUB_ISSUE_DESCRIPTION", "Remove a sub-issue from a parent issue."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REMOVE_SUB_ISSUE_USER_TITLE", "Remove Sub-Issue"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"issue_number": {
						Type:        "number",
						Description: "The parent issue number",
						Minimum:     jsonschema.Ptr(1.0),
					},
					"sub_issue_id": {
						Type:        "number",
						Description: "The global node ID of the sub-issue to remove",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "sub_issue_id"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			subIssueID, err := RequiredParam[float64](args, "sub_issue_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			parentNodeID, err := getGranularIssueNodeID(ctx, gqlClient, owner, repo, issueNumber)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get parent issue", err), nil, nil
			}

			var mutation struct {
				RemoveSubIssue struct {
					Issue struct {
						ID    string
						Title string
						URL   string
					}
					SubIssue struct {
						ID    string
						Title string
						URL   string
					}
				} `graphql:"removeSubIssue(input: $input)"`
			}

			input := GranularRemoveSubIssueInput{
				IssueID:    parentNodeID,
				SubIssueID: fmt.Sprintf("%d", int(subIssueID)),
			}

			if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
				return utils.NewToolResultErrorFromErr("failed to remove sub-issue", err), nil, nil
			}

			r, err := json.Marshal(mutation.RemoveSubIssue)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GranularReprioritizeSubIssue creates a tool to reorder a sub-issue.
func GranularReprioritizeSubIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssuesGranular,
		mcp.Tool{
			Name:        "reprioritize_sub_issue",
			Description: t("TOOL_REPRIORITIZE_SUB_ISSUE_DESCRIPTION", "Reprioritize (reorder) a sub-issue relative to other sub-issues."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REPRIORITIZE_SUB_ISSUE_USER_TITLE", "Reprioritize Sub-Issue"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"issue_number": {
						Type:        "number",
						Description: "The parent issue number",
						Minimum:     jsonschema.Ptr(1.0),
					},
					"sub_issue_id": {
						Type:        "number",
						Description: "The global node ID of the sub-issue to reorder",
					},
					"after_id": {
						Type:        "number",
						Description: "The global node ID of the sub-issue to place this after (optional)",
					},
					"before_id": {
						Type:        "number",
						Description: "The global node ID of the sub-issue to place this before (optional)",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "sub_issue_id"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			subIssueID, err := RequiredParam[float64](args, "sub_issue_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			afterID, _ := OptionalParam[float64](args, "after_id")
			beforeID, _ := OptionalParam[float64](args, "before_id")

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			parentNodeID, err := getGranularIssueNodeID(ctx, gqlClient, owner, repo, issueNumber)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get parent issue", err), nil, nil
			}

			var mutation struct {
				ReprioritizeSubIssue struct {
					Issue struct {
						ID    string
						Title string
						URL   string
					}
				} `graphql:"reprioritizeSubIssue(input: $input)"`
			}

			input := GranularReprioritizeSubIssueInput{
				IssueID:    parentNodeID,
				SubIssueID: fmt.Sprintf("%d", int(subIssueID)),
			}
			if afterID != 0 {
				id := githubv4.ID(fmt.Sprintf("%d", int(afterID)))
				input.AfterID = &id
			}
			if beforeID != 0 {
				id := githubv4.ID(fmt.Sprintf("%d", int(beforeID)))
				input.BeforeID = &id
			}

			if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
				return utils.NewToolResultErrorFromErr("failed to reprioritize sub-issue", err), nil, nil
			}

			r, err := json.Marshal(mutation.ReprioritizeSubIssue)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GraphQL input types for sub-issue mutations.

// GranularAddSubIssueInput is the input for the addSubIssue GraphQL mutation.
type GranularAddSubIssueInput struct {
	IssueID       string           `json:"issueId"`
	SubIssueID    string           `json:"subIssueId"`
	ReplaceParent githubv4.Boolean `json:"replaceParent"`
}

// GranularRemoveSubIssueInput is the input for the removeSubIssue GraphQL mutation.
type GranularRemoveSubIssueInput struct {
	IssueID    string `json:"issueId"`
	SubIssueID string `json:"subIssueId"`
}

// GranularReprioritizeSubIssueInput is the input for the reprioritizeSubIssue GraphQL mutation.
type GranularReprioritizeSubIssueInput struct {
	IssueID    string       `json:"issueId"`
	SubIssueID string       `json:"subIssueId"`
	AfterID    *githubv4.ID `json:"afterId,omitempty"`
	BeforeID   *githubv4.ID `json:"beforeId,omitempty"`
}

// getGranularIssueNodeID fetches the GraphQL node ID for an issue.
func getGranularIssueNodeID(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, issueNumber int) (string, error) {
	var query struct {
		Repository struct {
			Issue struct {
				ID string
			} `graphql:"issue(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	vars := map[string]any{
		"owner":  githubv4.String(owner),
		"name":   githubv4.String(repo),
		"number": githubv4.Int(issueNumber), // #nosec G115 - issue numbers are always small positive integers
	}

	if err := gqlClient.Query(ctx, &query, vars); err != nil {
		return "", fmt.Errorf("failed to query issue node ID: %w", err)
	}

	return query.Repository.Issue.ID, nil
}
