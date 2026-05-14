package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
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

	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        name,
			Description: t("TOOL_"+strings.ToUpper(name)+"_DESCRIPTION", description),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_"+strings.ToUpper(name)+"_USER_TITLE", title),
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

			issue, resp, err := client.Issues.Edit(ctx, owner, repo, issueNumber, issueReq)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(MinimalResponse{
				ID:  fmt.Sprintf("%d", issue.GetID()),
				URL: issue.GetHTMLURL(),
			})
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	st.FeatureFlagEnable = FeatureFlagIssuesGranular
	return st
}

// GranularCreateIssue creates a tool to create a new issue.
func GranularCreateIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataIssues,
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

			issue, resp, err := client.Issues.Create(ctx, owner, repo, issueReq)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(MinimalResponse{
				ID:  fmt.Sprintf("%d", issue.GetID()),
				URL: issue.GetHTMLURL(),
			})
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	st.FeatureFlagEnable = FeatureFlagIssuesGranular
	return st
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
			if _, ok := args["assignees"]; !ok {
				return nil, fmt.Errorf("missing required parameter: assignees")
			}
			assignees, err := OptionalStringArrayParam(args, "assignees")
			if err != nil {
				return nil, err
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
			if _, ok := args["labels"]; !ok {
				return nil, fmt.Errorf("missing required parameter: labels")
			}
			labels, err := OptionalStringArrayParam(args, "labels")
			if err != nil {
				return nil, err
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
				Type:        "integer",
				Description: "The milestone number to set on the issue",
				Minimum:     jsonschema.Ptr(1.0),
			},
		},
		[]string{"milestone"},
		func(args map[string]any) (*github.IssueRequest, error) {
			milestone, err := RequiredInt(args, "milestone")
			if err != nil {
				return nil, err
			}
			return &github.IssueRequest{Milestone: &milestone}, nil
		},
	)
}

// issueTypeWithRationale represents the object form of the issue type field,
// allowing a rationale to be sent alongside the type name.
type issueTypeWithRationale struct {
	Value     string `json:"value"`
	Rationale string `json:"rationale"`
}

// issueTypeUpdateRequest is a custom request body for updating an issue type
// with an optional rationale, using the object form that the REST API accepts.
type issueTypeUpdateRequest struct {
	Type issueTypeWithRationale `json:"type"`
}

// GranularUpdateIssueType creates a tool to update an issue's type.
func GranularUpdateIssueType(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "update_issue_type",
			Description: t("TOOL_UPDATE_ISSUE_TYPE_DESCRIPTION", "Update the type of an existing issue (e.g. 'bug', 'feature')."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_ISSUE_TYPE_USER_TITLE", "Update Issue Type"),
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
						Description: "The issue number to update",
						Minimum:     jsonschema.Ptr(1.0),
					},
					"issue_type": {
						Type:        "string",
						Description: "The issue type to set",
					},
					"rationale": {
						Type: "string",
						Description: "One concise sentence explaining what specifically about the issue led you to choose this type. " +
							"State the concrete signal (e.g. 'Reports a crash when saving' → bug, 'Asks for dark mode support' → feature).",
						MaxLength: jsonschema.Ptr(280),
					},
				},
				Required: []string{"owner", "repo", "issue_number", "issue_type"},
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
			issueType, err := RequiredParam[string](args, "issue_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			rationale, err := OptionalParam[string](args, "rationale")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			rationale = strings.TrimSpace(rationale)
			if len([]rune(rationale)) > 280 {
				return utils.NewToolResultError("parameter rationale must be 280 characters or less"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			var body any
			if rationale != "" {
				body = &issueTypeUpdateRequest{
					Type: issueTypeWithRationale{
						Value:     issueType,
						Rationale: rationale,
					},
				}
			} else {
				body = &github.IssueRequest{Type: &issueType}
			}

			apiURL := fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, issueNumber)
			req, err := client.NewRequest("PATCH", apiURL, body)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			issue := &github.Issue{}
			resp, err := client.Do(ctx, req, issue)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(MinimalResponse{
				ID:  fmt.Sprintf("%d", issue.GetID()),
				URL: issue.GetHTMLURL(),
			})
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	st.FeatureFlagEnable = FeatureFlagIssuesGranular
	return st
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
	st := NewTool(
		ToolsetMetadataIssues,
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
						Description: "The ID of the sub-issue to add. ID is not the same as issue number",
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
			subIssueID, err := RequiredInt(args, "sub_issue_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			replaceParent, _ := OptionalParam[bool](args, "replace_parent")

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			result, err := AddSubIssue(ctx, client, owner, repo, issueNumber, subIssueID, replaceParent)
			return result, nil, err
		},
	)
	st.FeatureFlagEnable = FeatureFlagIssuesGranular
	return st
}

// GranularRemoveSubIssue creates a tool to remove a sub-issue.
func GranularRemoveSubIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataIssues,
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
						Description: "The ID of the sub-issue to remove. ID is not the same as issue number",
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
			subIssueID, err := RequiredInt(args, "sub_issue_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			result, err := RemoveSubIssue(ctx, client, owner, repo, issueNumber, subIssueID)
			return result, nil, err
		},
	)
	st.FeatureFlagEnable = FeatureFlagIssuesGranular
	return st
}

// GranularReprioritizeSubIssue creates a tool to reorder a sub-issue.
func GranularReprioritizeSubIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataIssues,
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
						Description: "The ID of the sub-issue to reorder. ID is not the same as issue number",
					},
					"after_id": {
						Type:        "number",
						Description: "The ID of the sub-issue to place this after (either after_id OR before_id should be specified)",
					},
					"before_id": {
						Type:        "number",
						Description: "The ID of the sub-issue to place this before (either after_id OR before_id should be specified)",
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
			subIssueID, err := RequiredInt(args, "sub_issue_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			afterID, err := OptionalIntParam(args, "after_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			beforeID, err := OptionalIntParam(args, "before_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			result, err := ReprioritizeSubIssue(ctx, client, owner, repo, issueNumber, subIssueID, afterID, beforeID)
			return result, nil, err
		},
	)
	st.FeatureFlagEnable = FeatureFlagIssuesGranular
	return st
}

// SetIssueFieldValueInput represents the input for the setIssueFieldValue GraphQL mutation.
type SetIssueFieldValueInput struct {
	IssueID          githubv4.ID                     `json:"issueId"`
	IssueFields      []IssueFieldCreateOrUpdateInput `json:"issueFields"`
	ClientMutationID *githubv4.String                `json:"clientMutationId,omitempty"`
}

// IssueFieldCreateOrUpdateInput represents a single field value to set on an issue.
type IssueFieldCreateOrUpdateInput struct {
	FieldID              githubv4.ID       `json:"fieldId"`
	TextValue            *githubv4.String  `json:"textValue,omitempty"`
	NumberValue          *githubv4.Float   `json:"numberValue,omitempty"`
	DateValue            *githubv4.String  `json:"dateValue,omitempty"`
	SingleSelectOptionID *githubv4.ID      `json:"singleSelectOptionId,omitempty"`
	Delete               *githubv4.Boolean `json:"delete,omitempty"`
}

// GranularSetIssueFields creates a tool to set issue field values on an issue using GraphQL.
func GranularSetIssueFields(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "set_issue_fields",
			Description: t("TOOL_SET_ISSUE_FIELDS_DESCRIPTION", "Set issue field values for an issue. Fields are organization-level custom fields (text, number, date, or single select). Use this to create or update field values on an issue."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_SET_ISSUE_FIELDS_USER_TITLE", "Set Issue Fields"),
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
						Description: "The issue number to update",
						Minimum:     jsonschema.Ptr(1.0),
					},
					"fields": {
						Type:        "array",
						Description: "Array of issue field values to set. Each element must have a 'field_id' (string, the GraphQL node ID of the field) and exactly one value field: 'text_value' for text fields, 'number_value' for number fields, 'date_value' (ISO 8601 date string) for date fields, or 'single_select_option_id' (the GraphQL node ID of the option) for single select fields. Set 'delete' to true to remove a field value.",
						MinItems:    jsonschema.Ptr(1),
						Items: &jsonschema.Schema{
							Type: "object",
							Properties: map[string]*jsonschema.Schema{
								"field_id": {
									Type:        "string",
									Description: "The GraphQL node ID of the issue field",
								},
								"text_value": {
									Type:        "string",
									Description: "The value to set for a text field",
								},
								"number_value": {
									Type:        "number",
									Description: "The value to set for a number field",
								},
								"date_value": {
									Type:        "string",
									Description: "The value to set for a date field (ISO 8601 date string)",
								},
								"single_select_option_id": {
									Type:        "string",
									Description: "The GraphQL node ID of the option to set for a single select field",
								},
								"delete": {
									Type:        "boolean",
									Description: "Set to true to delete this field value",
								},
							},
							Required: []string{"field_id"},
						},
					},
				},
				Required: []string{"owner", "repo", "issue_number", "fields"},
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

			fieldsRaw, ok := args["fields"]
			if !ok {
				return utils.NewToolResultError("missing required parameter: fields"), nil, nil
			}

			// Accept both []any and []map[string]any input forms
			var fieldMaps []map[string]any
			switch v := fieldsRaw.(type) {
			case []any:
				for _, f := range v {
					fieldMap, ok := f.(map[string]any)
					if !ok {
						return utils.NewToolResultError("each field must be an object with 'field_id' and a value"), nil, nil
					}
					fieldMaps = append(fieldMaps, fieldMap)
				}
			case []map[string]any:
				fieldMaps = v
			default:
				return utils.NewToolResultError("invalid parameter: fields must be an array"), nil, nil
			}
			if len(fieldMaps) == 0 {
				return utils.NewToolResultError("fields array must not be empty"), nil, nil
			}

			issueFields := make([]IssueFieldCreateOrUpdateInput, 0, len(fieldMaps))
			for _, fieldMap := range fieldMaps {
				fieldID, err := RequiredParam[string](fieldMap, "field_id")
				if err != nil {
					return utils.NewToolResultError("field_id is required and must be a string"), nil, nil
				}

				input := IssueFieldCreateOrUpdateInput{
					FieldID: githubv4.ID(fieldID),
				}

				// Count how many value keys are present; exactly one is required.
				valueCount := 0

				if v, err := OptionalParam[string](fieldMap, "text_value"); err == nil && v != "" {
					input.TextValue = githubv4.NewString(githubv4.String(v))
					valueCount++
				}
				if v, err := OptionalParam[float64](fieldMap, "number_value"); err == nil {
					if _, exists := fieldMap["number_value"]; exists {
						gqlFloat := githubv4.Float(v)
						input.NumberValue = &gqlFloat
						valueCount++
					}
				}
				if v, err := OptionalParam[string](fieldMap, "date_value"); err == nil && v != "" {
					input.DateValue = githubv4.NewString(githubv4.String(v))
					valueCount++
				}
				if v, err := OptionalParam[string](fieldMap, "single_select_option_id"); err == nil && v != "" {
					optionID := githubv4.ID(v)
					input.SingleSelectOptionID = &optionID
					valueCount++
				}
				if _, exists := fieldMap["delete"]; exists {
					del, err := OptionalParam[bool](fieldMap, "delete")
					if err == nil && del {
						deleteVal := githubv4.Boolean(true)
						input.Delete = &deleteVal
						valueCount++
					}
				}

				if valueCount == 0 {
					return utils.NewToolResultError("each field must have a value (text_value, number_value, date_value, single_select_option_id) or delete: true"), nil, nil
				}
				if valueCount > 1 {
					return utils.NewToolResultError("each field must have exactly one value (text_value, number_value, date_value, single_select_option_id) or delete: true, but multiple were provided"), nil, nil
				}

				issueFields = append(issueFields, input)
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			// Resolve issue node ID
			issueID, _, err := fetchIssueIDs(ctx, gqlClient, owner, repo, issueNumber, 0)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get issue", err), nil, nil
			}

			// Execute the setIssueFieldValue mutation
			var mutation struct {
				SetIssueFieldValue struct {
					Issue struct {
						ID     githubv4.ID
						Number githubv4.Int
						URL    githubv4.String
					}
					IssueFieldValues []struct {
						TextValue struct {
							Value string
						} `graphql:"... on IssueFieldTextValue"`
						SingleSelectValue struct {
							Name string
						} `graphql:"... on IssueFieldSingleSelectValue"`
						DateValue struct {
							Value string
						} `graphql:"... on IssueFieldDateValue"`
						NumberValue struct {
							Value float64
						} `graphql:"... on IssueFieldNumberValue"`
					}
				} `graphql:"setIssueFieldValue(input: $input)"`
			}

			mutationInput := SetIssueFieldValueInput{
				IssueID:     issueID,
				IssueFields: issueFields,
			}

			if err := gqlClient.Mutate(ctx, &mutation, mutationInput, nil); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to set issue field values", err), nil, nil
			}

			r, err := json.Marshal(MinimalResponse{
				ID:  fmt.Sprintf("%v", mutation.SetIssueFieldValue.Issue.ID),
				URL: string(mutation.SetIssueFieldValue.Issue.URL),
			})
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	st.FeatureFlagEnable = FeatureFlagIssuesGranular
	return st
}
