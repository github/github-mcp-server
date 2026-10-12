package github

import (
	"context"
	"fmt"
	"maps"
	"strings"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

func normalizeConfidence(confidence string) string {
	return strings.ToUpper(strings.TrimSpace(confidence))
}

// issueUpdateTool is a helper to create single-field issue update tools.
func issueUpdateTool[In interface {
	issueCoordinate() GranularIssueCoordinate
}](
	t translations.TranslationHelperFunc,
	name, description, title string,
	extraProps map[string]*jsonschema.Schema,
	extraRequired []string,
	buildRequest func(args In) github.UpdateIssueRequest,
	normalizer inventory.InputNormalizer,
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
			Minimum:     new(1.0),
		},
	}
	maps.Copy(props, extraProps)

	required := append([]string{"owner", "repo", "issue_number"}, extraRequired...)

	st := NewTool[In, *MinimalResponse](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        name,
			Description: t("TOOL_"+strings.ToUpper(name)+"_DESCRIPTION", description),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_"+strings.ToUpper(name)+"_USER_TITLE", title),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: props,
				Required:   required,
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args In) (*mcp.CallToolResult, *MinimalResponse, error) {
			coordinate := args.issueCoordinate()
			owner, repo, issueNumber := coordinate.Owner, coordinate.Repo, coordinate.IssueNumber
			issueReq := buildRequest(args)

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			issue, resp, err := client.Issues.Update(ctx, owner, repo, issueNumber, issueReq)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return granularIssueMinimalResult(&MinimalResponse{
				ID:  fmt.Sprintf("%d", issue.GetID()),
				URL: issue.GetHTMLURL(),
			})
		},
		normalizer,
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// GranularCreateIssue creates a tool to create a new issue.
func GranularCreateIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularCreateIssueInput, *MinimalResponse](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "create_issue",
			Description: t("TOOL_CREATE_ISSUE_DESCRIPTION", "Create a new issue in a GitHub repository with a title and optional body."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_CREATE_ISSUE_USER_TITLE", "Create Issue"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
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
					"parent_issue_number": {
						Type:        "number",
						Description: "Issue number of the parent issue. The new issue is created and attached to this parent in the same operation.",
						Minimum:     new(1.0),
					},
					"parent_owner": {
						Type:        "string",
						Description: "Repository owner of the parent issue. Must be provided with parent_repo. Omit both to use owner and repo. Only used when parent_issue_number is provided.",
					},
					"parent_repo": {
						Type:        "string",
						Description: "Repository name of the parent issue. Must be provided with parent_owner. Omit both to use owner and repo. Only used when parent_issue_number is provided.",
					},
				},
				Required: []string{"owner", "repo", "title"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularCreateIssueInput) (*mcp.CallToolResult, *MinimalResponse, error) {
			owner, repo, title, body := args.Owner, args.Repo, args.Title, args.Body
			parentProvided := args.ParentIssueNumber != nil
			parentIssueNumber := 0
			if parentProvided {
				parentIssueNumber = *args.ParentIssueNumber
			}
			parentOwner, parentRepo := args.ParentOwner, args.ParentRepo
			issueReq := github.CreateIssueRequest{Title: title}
			if body != "" {
				issueReq.Body = &body
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			if parentProvided {
				gqlClient, err := deps.GetGQLClient(ctx)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
				}
				result, output, err := createIssueWithParent(ctx, client, gqlClient, owner, repo, title, body, nil, nil, 0, "", parentIssueNumber, parentOwner, parentRepo)
				return result, output, err
			}

			issue, resp, err := client.Issues.Create(ctx, owner, repo, issueReq)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return granularIssueMinimalResult(&MinimalResponse{
				ID:  fmt.Sprintf("%d", issue.GetID()),
				URL: issue.GetHTMLURL(),
			})
		},
		granularIssueInputNormalizer(normalizeGranularCreateIssueArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
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
		func(args GranularIssueTitleInput) github.UpdateIssueRequest {
			return github.UpdateIssueRequest{Title: &args.Title}
		},
		granularIssueInputNormalizer(normalizeGranularIssueUpdateArguments("title")),
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
		func(args GranularIssueBodyInput) github.UpdateIssueRequest {
			return github.UpdateIssueRequest{Body: &args.Body}
		},
		granularIssueInputNormalizer(normalizeGranularIssueUpdateArguments("body")),
	)
}

// GranularUpdateIssueAssignees creates a tool to update an issue's assignees.
func GranularUpdateIssueAssignees(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularIssueAssigneesInput, *MinimalResponse](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "update_issue_assignees",
			Description: t("TOOL_UPDATE_ISSUE_ASSIGNEES_DESCRIPTION", "Update the assignees of an existing issue. This replaces the current assignees with the provided list. When setting values, include a confidence level (LOW, MEDIUM, or HIGH) reflecting how certain you are about the choice."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_ISSUE_ASSIGNEES_USER_TITLE", "Update Issue Assignees"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
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
						Minimum:     new(1.0),
					},
					"assignees": {
						Type:        "array",
						Description: "GitHub usernames to assign to this issue.",
						Items: &jsonschema.Schema{
							OneOf: []*jsonschema.Schema{
								{Type: "string", Description: "GitHub username"},
								{
									Type: "object",
									Properties: map[string]*jsonschema.Schema{
										"login": {
											Type:        "string",
											Description: "GitHub username",
										},
										"rationale": {
											Type: "string",
											Description: "One concise sentence explaining what specifically about the issue led you to choose this assignee. " +
												"State the concrete signal (e.g. 'Authored the file the crash originates in').",
											MaxLength: new(280),
										},
										"confidence": {
											Type:        "string",
											Description: "How confident you are in this choice. Use 'HIGH' for clear signal or explicit user request, 'MEDIUM' for reasonable inference with some ambiguity, 'LOW' for best guess with limited signal.",
											Enum:        []any{"LOW", "MEDIUM", "HIGH"},
										},
										"is_suggestion": {
											Type: "boolean",
											Description: "If true, this assignee is sent to the API as a suggestion (suggest:true) rather than an applied assignee. " +
												"Whether the assignee is applied or recorded as a proposal is determined by the API.",
										},
									},
									Required: []string{"login"},
								},
							},
						},
					},
				},
				Required: []string{"owner", "repo", "issue_number", "assignees"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularIssueAssigneesInput) (*mcp.CallToolResult, *MinimalResponse, error) {
			owner, repo, issueNumber := args.Owner, args.Repo, args.IssueNumber

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			body := granularIssueAssigneesPayload(args.Assignees)

			apiURL := fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, issueNumber)
			req, err := client.NewRequest(ctx, "PATCH", apiURL, body)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			issue := &github.Issue{}
			resp, err := client.Do(req, issue)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return granularIssueMinimalResult(&MinimalResponse{
				ID:  fmt.Sprintf("%d", issue.GetID()),
				URL: issue.GetHTMLURL(),
			})
		},
		granularIssueInputNormalizer(normalizeGranularUpdateIssueAssigneesArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// labelWithIntent represents the object form of a label entry, allowing a
// rationale, confidence level, and/or suggest flag to be sent alongside the label name.
type labelWithIntent struct {
	Name       string `json:"name"`
	Rationale  string `json:"rationale,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Suggest    bool   `json:"suggest,omitempty"`
}

// labelsUpdateRequest is a custom request body for updating an issue's labels
// where individual labels may optionally include a rationale. Each element of
// Labels is either a string (label name) or a labelWithIntent object.
type labelsUpdateRequest struct {
	Labels []any `json:"labels"`
}

// assigneeWithIntent represents the object form of an assignee entry, allowing a
// rationale, confidence level, and/or suggest flag to be sent alongside the login.
type assigneeWithIntent struct {
	Login      string `json:"login"`
	Rationale  string `json:"rationale,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Suggest    bool   `json:"suggest,omitempty"`
}

// assigneesUpdateRequest is a custom request body for updating an issue's
// assignees where individual assignees may optionally include a rationale. Each
// element of Assignees is either a string (login) or an assigneeWithIntent object.
type assigneesUpdateRequest struct {
	Assignees []any `json:"assignees"`
}

// GranularUpdateIssueLabels creates a tool to update an issue's labels.
func GranularUpdateIssueLabels(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularIssueLabelsInput, *MinimalResponse](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "update_issue_labels",
			Description: t("TOOL_UPDATE_ISSUE_LABELS_DESCRIPTION", "Update the labels of an existing issue. This replaces the current labels with the provided list. When setting values, include a confidence level (LOW, MEDIUM, or HIGH) reflecting how certain you are about the choice."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_ISSUE_LABELS_USER_TITLE", "Update Issue Labels"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
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
						Minimum:     new(1.0),
					},
					"labels": {
						Type:        "array",
						Description: "Labels to apply to this issue.",
						Items: &jsonschema.Schema{
							OneOf: []*jsonschema.Schema{
								{Type: "string", Description: "Label name"},
								{
									Type: "object",
									Properties: map[string]*jsonschema.Schema{
										"name": {
											Type:        "string",
											Description: "Label name",
										},
										"rationale": {
											Type: "string",
											Description: "One concise sentence explaining what specifically about the issue led you to choose this label. " +
												"State the concrete signal (e.g. 'Reports a crash when saving' → bug).",
											MaxLength: new(280),
										},
										"confidence": {
											Type:        "string",
											Description: "How confident you are in this choice. Use 'HIGH' for clear signal or explicit user request, 'MEDIUM' for reasonable inference with some ambiguity, 'LOW' for best guess with limited signal.",
											Enum:        []any{"LOW", "MEDIUM", "HIGH"},
										},
										"is_suggestion": {
											Type: "boolean",
											Description: "If true, this label is sent to the API as a suggestion (suggest:true) rather than an applied label. " +
												"Whether the label is applied or recorded as a proposal is determined by the API.",
										},
									},
									Required: []string{"name"},
								},
							},
						},
					},
				},
				Required: []string{"owner", "repo", "issue_number", "labels"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularIssueLabelsInput) (*mcp.CallToolResult, *MinimalResponse, error) {
			owner, repo, issueNumber := args.Owner, args.Repo, args.IssueNumber

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			body := granularIssueLabelsPayload(args.Labels)

			apiURL := fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, issueNumber)
			req, err := client.NewRequest(ctx, "PATCH", apiURL, body)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			issue := &github.Issue{}
			resp, err := client.Do(req, issue)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return granularIssueMinimalResult(&MinimalResponse{
				ID:  fmt.Sprintf("%d", issue.GetID()),
				URL: issue.GetHTMLURL(),
			})
		},
		granularIssueInputNormalizer(normalizeGranularUpdateIssueLabelsArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
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
				Minimum:     new(1.0),
			},
		},
		[]string{"milestone"},
		func(args GranularIssueMilestoneInput) github.UpdateIssueRequest {
			return github.UpdateIssueRequest{Milestone: &args.Milestone}
		},
		granularIssueInputNormalizer(normalizeGranularIssueUpdateArguments("milestone")),
	)
}

// issueTypeWithIntent represents the object form of the issue type field,
// allowing a rationale, confidence level, and/or suggest flag to be sent alongside the type name.
type issueTypeWithIntent struct {
	Value      string `json:"value"`
	Rationale  string `json:"rationale,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Suggest    bool   `json:"suggest,omitempty"`
}

// issueTypeUpdateRequest is a custom request body for updating an issue type
// with optional intent metadata, using the object form that the REST API accepts.
type issueTypeUpdateRequest struct {
	Type issueTypeWithIntent `json:"type"`
}

// GranularUpdateIssueType creates a tool to set or clear an issue's type.
func GranularUpdateIssueType(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularIssueTypeInput, *MinimalResponse](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "update_issue_type",
			Description: t("TOOL_UPDATE_ISSUE_TYPE_DESCRIPTION", "Set or remove the type of an existing issue. Pass null to remove the current type. When setting a value, include a confidence level (LOW, MEDIUM, or HIGH) reflecting how certain you are about the choice."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_ISSUE_TYPE_USER_TITLE", "Update Issue Type"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
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
						Minimum:     new(1.0),
					},
					"issue_type": {
						AnyOf: []*jsonschema.Schema{
							{Type: "string", MinLength: new(1)},
							{Type: "null"},
						},
						Description: "The issue type to set, or null to remove the current type",
					},
					"rationale": {
						Type: "string",
						Description: "One concise sentence explaining what specifically about the issue led you to choose this type. " +
							"State the concrete signal (e.g. 'Reports a crash when saving' → bug, 'Asks for dark mode support' → feature).",
						MaxLength: new(280),
					},
					"confidence": {
						Type:        "string",
						Description: "How confident you are in this choice. Use 'HIGH' for clear signal or explicit user request, 'MEDIUM' for reasonable inference with some ambiguity, 'LOW' for best guess with limited signal.",
						Enum:        []any{"LOW", "MEDIUM", "HIGH"},
					},
					"is_suggestion": {
						Type: "boolean",
						Description: "If true, this issue type change is sent to the API as a suggestion (suggest:true) rather than an applied value. " +
							"Whether the type is applied or recorded as a proposal is determined by the API.",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "issue_type"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularIssueTypeInput) (*mcp.CallToolResult, *MinimalResponse, error) {
			owner, repo, issueNumber := args.Owner, args.Repo, args.IssueNumber
			issueType, rationale, confidence, isSuggestion := args.IssueType, args.Rationale, args.Confidence, args.IsSuggestion

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			var body any
			switch {
			case issueType == nil:
				body = map[string]any{"type": nil}
			case rationale != "" || isSuggestion || confidence != "":
				body = &issueTypeUpdateRequest{
					Type: issueTypeWithIntent{
						Value:      *issueType,
						Rationale:  rationale,
						Confidence: confidence,
						Suggest:    isSuggestion,
					},
				}
			default:
				body = &github.UpdateIssueRequest{Type: issueType}
			}

			apiURL := fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, issueNumber)
			req, err := client.NewRequest(ctx, "PATCH", apiURL, body)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			issue := &github.Issue{}
			resp, err := client.Do(req, issue)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return granularIssueMinimalResult(&MinimalResponse{
				ID:  fmt.Sprintf("%d", issue.GetID()),
				URL: issue.GetHTMLURL(),
			})
		},
		granularIssueInputNormalizer(normalizeGranularUpdateIssueTypeArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// stateWithIntent represents the object form of the state field, allowing
// rationale, confidence, and/or suggest flag to be sent alongside the state value.
type stateWithIntent struct {
	Value      string `json:"value"`
	Rationale  string `json:"rationale,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Suggest    bool   `json:"suggest,omitempty"`
}

// stateUpdateRequest is a custom request body for updating an issue's state
// with optional intent metadata, using the object form that the REST API accepts.
type stateUpdateRequest struct {
	State            stateWithIntent `json:"state"`
	StateReason      string          `json:"state_reason,omitempty"`
	DuplicateIssueID *int64          `json:"duplicate_issue_id,omitempty"`
}

// GranularUpdateIssueState creates a tool to update an issue's state.
func GranularUpdateIssueState(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularIssueStateInput, *MinimalResponse](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "update_issue_state",
			Description: t("TOOL_UPDATE_ISSUE_STATE_DESCRIPTION", "Update the state of an existing issue (open or closed), with an optional state reason. When closing, include a confidence level (LOW, MEDIUM, or HIGH) reflecting how certain you are about the decision. Use is_suggestion to propose the change without applying it directly."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_ISSUE_STATE_USER_TITLE", "Update Issue State"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
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
						Minimum:     new(1.0),
					},
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
					"rationale": {
						Type: "string",
						Description: "One concise sentence explaining what specifically about the issue led you to choose this state. " +
							"State the concrete signal (e.g. 'The reported crash is fixed in v2.1' → completed).",
						MaxLength: new(280),
					},
					"confidence": {
						Type:        "string",
						Description: "How confident you are in this choice. Use 'HIGH' for clear signal or explicit user request, 'MEDIUM' for reasonable inference with some ambiguity, 'LOW' for best guess with limited signal.",
						Enum:        []any{"LOW", "MEDIUM", "HIGH"},
					},
					"is_suggestion": {
						Type: "boolean",
						Description: "If true, this state change is sent to the API as a suggestion (suggest:true) rather than an applied change. " +
							"Whether the change is applied or recorded as a proposal is determined by the API.",
					},
					"duplicate_of": {
						Type:        "number",
						Description: "The issue number of the canonical issue this issue duplicates. Only valid when state_reason is 'duplicate'. Required when is_suggestion is true and state_reason is 'duplicate'. The issue number is resolved to a database ID before being sent to the API.",
						Minimum:     new(1.0),
					},
				},
				Required: []string{"owner", "repo", "issue_number", "state"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularIssueStateInput) (*mcp.CallToolResult, *MinimalResponse, error) {
			owner, repo, issueNumber := args.Owner, args.Repo, args.IssueNumber
			state, stateReason := args.State, args.StateReason
			rationale, confidence, isSuggestion, duplicateOf := args.Rationale, args.Confidence, args.IsSuggestion, args.DuplicateOf

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			var body any
			if rationale != "" || isSuggestion || confidence != "" || duplicateOf != 0 {
				req := &stateUpdateRequest{
					State: stateWithIntent{
						Value:      state,
						Rationale:  rationale,
						Confidence: confidence,
						Suggest:    isSuggestion,
					},
					StateReason: stateReason,
				}
				if duplicateOf != 0 {
					duplicateIssue, resp, err := client.Issues.Get(ctx, owner, repo, duplicateOf)
					if err != nil {
						return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get duplicate issue", resp, err), nil, nil
					}
					_ = resp.Body.Close()
					id := duplicateIssue.GetID()
					req.DuplicateIssueID = &id
				}
				body = req
			} else {
				req := &github.UpdateIssueRequest{State: &state}
				if stateReason != "" {
					req.StateReason = &stateReason
				}
				body = req
			}

			apiURL := fmt.Sprintf("repos/%s/%s/issues/%d", owner, repo, issueNumber)
			req, err := client.NewRequest(ctx, "PATCH", apiURL, body)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			issue := &github.Issue{}
			resp, err := client.Do(req, issue)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return granularIssueMinimalResult(&MinimalResponse{
				ID:  fmt.Sprintf("%d", issue.GetID()),
				URL: issue.GetHTMLURL(),
			})
		},
		granularIssueInputNormalizer(normalizeGranularUpdateIssueStateArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// GranularAddSubIssue creates a tool to add a sub-issue.
func GranularAddSubIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularAddSubIssueInput, *SubIssueWriteOutput](
		ToolsetMetadataIssues,
		mcp.Tool{
			OutputSchema: subIssueWriteOutputSchema(),
			Name:         "add_sub_issue",
			Description:  t("TOOL_ADD_SUB_ISSUE_DESCRIPTION", "Add a sub-issue to a parent issue."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ADD_SUB_ISSUE_USER_TITLE", "Add Sub-Issue"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
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
						Minimum:     new(1.0),
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
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularAddSubIssueInput) (*mcp.CallToolResult, *SubIssueWriteOutput, error) {
			owner, repo, issueNumber, subIssueID, replaceParent := args.Owner, args.Repo, args.IssueNumber, args.SubIssueID, args.ReplaceParent

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			result, output, err := addSubIssue(ctx, client, owner, repo, issueNumber, subIssueID, replaceParent)
			return subIssueWriteResult("add")(result, output, err)
		},
		granularIssueInputNormalizer(normalizeGranularAddSubIssueArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// GranularRemoveSubIssue creates a tool to remove a sub-issue.
func GranularRemoveSubIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularRemoveSubIssueInput, *SubIssueWriteOutput](
		ToolsetMetadataIssues,
		mcp.Tool{
			OutputSchema: subIssueWriteOutputSchema(),
			Name:         "remove_sub_issue",
			Description:  t("TOOL_REMOVE_SUB_ISSUE_DESCRIPTION", "Remove a sub-issue from a parent issue."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REMOVE_SUB_ISSUE_USER_TITLE", "Remove Sub-Issue"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
				OpenWorldHint:   new(true),
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
						Minimum:     new(1.0),
					},
					"sub_issue_id": {
						Type:        "number",
						Description: "The ID of the sub-issue to remove. ID is not the same as issue number",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "sub_issue_id"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularRemoveSubIssueInput) (*mcp.CallToolResult, *SubIssueWriteOutput, error) {
			owner, repo, issueNumber, subIssueID := args.Owner, args.Repo, args.IssueNumber, args.SubIssueID

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			result, output, err := removeSubIssue(ctx, client, owner, repo, issueNumber, subIssueID)
			return subIssueWriteResult("remove")(result, output, err)
		},
		granularIssueInputNormalizer(normalizeGranularRemoveSubIssueArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// GranularReprioritizeSubIssue creates a tool to reorder a sub-issue.
func GranularReprioritizeSubIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularReprioritizeSubIssueInput, *SubIssueWriteOutput](
		ToolsetMetadataIssues,
		mcp.Tool{
			OutputSchema: subIssueWriteOutputSchema(),
			Name:         "reprioritize_sub_issue",
			Description:  t("TOOL_REPRIORITIZE_SUB_ISSUE_DESCRIPTION", "Reprioritize (reorder) a sub-issue relative to other sub-issues."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REPRIORITIZE_SUB_ISSUE_USER_TITLE", "Reprioritize Sub-Issue"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
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
						Minimum:     new(1.0),
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
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularReprioritizeSubIssueInput) (*mcp.CallToolResult, *SubIssueWriteOutput, error) {
			owner, repo, issueNumber, subIssueID, afterID, beforeID := args.Owner, args.Repo, args.IssueNumber, args.SubIssueID, args.AfterID, args.BeforeID

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			result, output, err := reprioritizeSubIssue(ctx, client, owner, repo, issueNumber, subIssueID, afterID, beforeID)
			return subIssueWriteResult("reprioritize")(result, output, err)
		},
		granularIssueInputNormalizer(normalizeGranularReprioritizeSubIssueArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
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
	Rationale            *githubv4.String  `json:"rationale,omitempty"`
	Confidence           *string           `json:"confidence,omitempty"`
	Suggest              *githubv4.Boolean `json:"suggest,omitempty"`
}

type setIssueFieldValueMutation struct {
	SetIssueFieldValue struct {
		Issue struct {
			ID  githubv4.ID
			URL githubv4.String
		}
	} `graphql:"setIssueFieldValue(input: $input)"`
}

// SetIssueFieldValues updates Issue Field values and returns the updated issue.
func SetIssueFieldValues(ctx context.Context, gqlClient *githubv4.Client, input SetIssueFieldValueInput) (MinimalResponse, error) {
	var mutation setIssueFieldValueMutation
	if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
		return MinimalResponse{}, err
	}
	return MinimalResponse{
		ID:  fmt.Sprintf("%v", mutation.SetIssueFieldValue.Issue.ID),
		URL: string(mutation.SetIssueFieldValue.Issue.URL),
	}, nil
}

// GranularSetIssueFields creates a tool to set issue field values on an issue using GraphQL.
func GranularSetIssueFields(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularSetIssueFieldsInput, *MinimalResponse](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "set_issue_fields",
			Description: t("TOOL_SET_ISSUE_FIELDS_DESCRIPTION", "Set issue field values for an issue. Fields are organization-level custom fields (text, number, date, or single select). Use this to create or update field values on an issue."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_SET_ISSUE_FIELDS_USER_TITLE", "Set Issue Fields"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
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
						Minimum:     new(1.0),
					},
					"fields": {
						Type:        "array",
						Description: "Array of issue field values to set. Each element must have a 'field_id' (string, the GraphQL node ID of the field) and exactly one value field: 'text_value' for text fields, 'number_value' for number fields, 'date_value' (ISO 8601 date string) for date fields, or 'single_select_option_id' (the GraphQL node ID of the option) for single select fields. Set 'delete' to true to remove a field value.",
						MinItems:    new(1),
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
								"rationale": {
									Type: "string",
									Description: "One concise sentence explaining what specifically about the issue led you to choose this field value. " +
										"State the concrete signal (e.g. 'Reports a crash when saving' → high priority).",
									MaxLength: new(280),
								},
								"confidence": {
									Type:        "string",
									Description: "How confident you are in this choice. Use 'HIGH' for clear signal or explicit user request, 'MEDIUM' for reasonable inference with some ambiguity, 'LOW' for best guess with limited signal.",
									Enum:        []any{"LOW", "MEDIUM", "HIGH"},
								},
								"is_suggestion": {
									Type: "boolean",
									Description: "If true, this field value is sent to the API as a suggestion (suggest:true) rather than an applied value. " +
										"Whether the value is applied or recorded as a proposal is determined by the API.",
								},
							},
							Required: []string{"field_id"},
						},
					},
				},
				Required: []string{"owner", "repo", "issue_number", "fields"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularSetIssueFieldsInput) (*mcp.CallToolResult, *MinimalResponse, error) {
			owner, repo, issueNumber := args.Owner, args.Repo, args.IssueNumber
			issueFields := granularIssueFieldMutation(args.Fields)

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			// Resolve issue node ID
			issueID, _, err := fetchIssueIDs(ctx, gqlClient, owner, repo, issueNumber, 0)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get issue", err), nil, nil
			}

			mutationInput := SetIssueFieldValueInput{
				IssueID:     issueID,
				IssueFields: issueFields,
			}

			// The rationale and suggest input fields on IssueFieldCreateOrUpdateInput
			// are gated behind the update_issue_suggestions GraphQL feature flag.
			ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "update_issue_suggestions")
			response, err := SetIssueFieldValues(ctxWithFeatures, gqlClient, mutationInput)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to set issue field values", err), nil, nil
			}

			return granularIssueMinimalResult(&response)
		},
		granularIssueInputNormalizer(normalizeGranularSetIssueFieldsArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// GranularAddIssueReaction adds a reaction to an issue or pull request.
func GranularAddIssueReaction(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularAddIssueReactionInput, *MinimalResponse](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "add_issue_reaction",
			Description: t("TOOL_ADD_ISSUE_REACTION_DESCRIPTION", "Add a reaction to an issue or pull request."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ADD_ISSUE_REACTION_USER_TITLE", "Add Reaction to Issue or Pull Request"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
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
						Description: "The issue number",
						Minimum:     new(1.0),
					},
					"content": {
						Type:        "string",
						Description: "The emoji reaction type",
						Enum:        []any{"+1", "-1", "laugh", "confused", "heart", "hooray", "rocket", "eyes"},
					},
				},
				Required: []string{"owner", "repo", "issue_number", "content"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularAddIssueReactionInput) (*mcp.CallToolResult, *MinimalResponse, error) {
			owner, repo, issueNumber, content := args.Owner, args.Repo, args.IssueNumber, args.Content

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			reaction, resp, err := client.Reactions.CreateIssueReaction(ctx, owner, repo, issueNumber, content)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to add reaction to issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return granularIssueMinimalResult(&MinimalResponse{
				ID:  fmt.Sprintf("%d", reaction.GetID()),
				URL: fmt.Sprintf("%srepos/%s/%s/issues/%d/reactions/%d", client.BaseURL(), owner, repo, issueNumber, reaction.GetID()),
			})
		},
		granularIssueInputNormalizer(normalizeGranularAddIssueReactionArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// GranularRemoveIssueReaction removes a reaction from an issue or pull request.
func GranularRemoveIssueReaction(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularRemoveIssueReactionInput, *GranularIssueReactionMessage](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "remove_issue_reaction",
			Description: t("TOOL_REMOVE_ISSUE_REACTION_DESCRIPTION", "Remove a reaction from an issue or pull request."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REMOVE_ISSUE_REACTION_USER_TITLE", "Remove Reaction from Issue or Pull Request"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
				OpenWorldHint:   new(true),
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
						Description: "The issue number",
						Minimum:     new(1.0),
					},
					"reaction_id": {
						Type:        "number",
						Description: "The reaction ID to remove",
						Minimum:     new(1.0),
					},
				},
				Required: []string{"owner", "repo", "issue_number", "reaction_id"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularRemoveIssueReactionInput) (*mcp.CallToolResult, *GranularIssueReactionMessage, error) {
			owner, repo, issueNumber, reactionID := args.Owner, args.Repo, args.IssueNumber, args.ReactionID

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			resp, err := client.Reactions.DeleteIssueReaction(ctx, owner, repo, issueNumber, reactionID)
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to remove reaction from issue", resp, err), nil, nil
			}

			return utils.NewToolResultText("reaction successfully removed from issue"), &GranularIssueReactionMessage{Message: "reaction successfully removed from issue"}, nil
		},
		granularIssueInputNormalizer(normalizeGranularRemoveIssueReactionArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// GranularAddIssueCommentReaction adds a reaction to an issue or pull request comment.
func GranularAddIssueCommentReaction(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularAddIssueCommentReactionInput, *MinimalResponse](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "add_issue_comment_reaction",
			Description: t("TOOL_ADD_ISSUE_COMMENT_REACTION_DESCRIPTION", "Add a reaction to an issue or pull request comment."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ADD_ISSUE_COMMENT_REACTION_USER_TITLE", "Add Reaction to Issue or Pull Request Comment"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
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
					"comment_id": {
						Type:        "number",
						Description: "The issue or pull request comment ID",
						Minimum:     new(1.0),
					},
					"content": {
						Type:        "string",
						Description: "The emoji reaction type",
						Enum:        []any{"+1", "-1", "laugh", "confused", "heart", "hooray", "rocket", "eyes"},
					},
				},
				Required: []string{"owner", "repo", "comment_id", "content"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularAddIssueCommentReactionInput) (*mcp.CallToolResult, *MinimalResponse, error) {
			owner, repo, commentID, content := args.Owner, args.Repo, args.CommentID, args.Content

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			reaction, resp, err := client.Reactions.CreateIssueCommentReaction(ctx, owner, repo, commentID, content)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to add reaction to issue comment", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return granularIssueMinimalResult(&MinimalResponse{
				ID:  fmt.Sprintf("%d", reaction.GetID()),
				URL: fmt.Sprintf("%srepos/%s/%s/issues/comments/%d/reactions/%d", client.BaseURL(), owner, repo, commentID, reaction.GetID()),
			})
		},
		granularIssueInputNormalizer(normalizeGranularAddIssueCommentReactionArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// GranularRemoveIssueCommentReaction removes a reaction from an issue or pull request comment.
func GranularRemoveIssueCommentReaction(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool[GranularRemoveIssueCommentReactionInput, *GranularIssueReactionMessage](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "remove_issue_comment_reaction",
			Description: t("TOOL_REMOVE_ISSUE_COMMENT_REACTION_DESCRIPTION", "Remove a reaction from an issue or pull request comment."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REMOVE_ISSUE_COMMENT_REACTION_USER_TITLE", "Remove Reaction from Issue or Pull Request Comment"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
				OpenWorldHint:   new(true),
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
					"comment_id": {
						Type:        "number",
						Description: "The issue or pull request comment ID",
						Minimum:     new(1.0),
					},
					"reaction_id": {
						Type:        "number",
						Description: "The reaction ID to remove",
						Minimum:     new(1.0),
					},
				},
				Required: []string{"owner", "repo", "comment_id", "reaction_id"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularRemoveIssueCommentReactionInput) (*mcp.CallToolResult, *GranularIssueReactionMessage, error) {
			owner, repo, commentID, reactionID := args.Owner, args.Repo, args.CommentID, args.ReactionID

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			resp, err := client.Reactions.DeleteIssueCommentReaction(ctx, owner, repo, commentID, reactionID)
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to remove reaction from issue comment", resp, err), nil, nil
			}

			return utils.NewToolResultText("reaction successfully removed from issue comment"), &GranularIssueReactionMessage{Message: "reaction successfully removed from issue comment"}, nil
		},
		granularIssueInputNormalizer(normalizeGranularRemoveIssueCommentReactionArguments),
	)
	st.FeatureRule = issuesGranularFeatureRule
	return st
}

// GranularHideIssueComment hides (minimizes) an issue or pull request conversation comment.
func GranularHideIssueComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return commentVisibilityTool(t, issueCommentVisibilityTarget, true)
}

// GranularUnhideIssueComment unhides (unminimizes) an issue or pull request conversation comment.
func GranularUnhideIssueComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return commentVisibilityTool(t, issueCommentVisibilityTarget, false)
}
