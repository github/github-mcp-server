package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// IssueWriteUIResourceURI is the URI for the create_issue_ui tool's MCP App UI resource.
const IssueWriteUIResourceURI = "ui://github-mcp-server/issue-write"

// CreateIssueUI creates a tool that shows an interactive UI for creating GitHub issues.
// This tool only displays the form - the actual issue creation happens when the user
// clicks "Create Issue" in the UI, which calls the issue_write tool.
func CreateIssueUI(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "create_issue_ui",
			Description: t("TOOL_CREATE_ISSUE_UI_DESCRIPTION", "Show an interactive UI for creating a new issue in a GitHub repository. The user will fill in the issue details and submit the form. You can pre-fill fields like title, body, labels, assignees, milestone, and type. For best results, verify that labels, assignees, milestones, and issue types exist in the repository before pre-filling them (use list_label, list_assignees, list_milestones, and list_issue_types tools)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_ISSUE_UI_USER_TITLE", "Create issue form"),
				ReadOnlyHint: true, // The tool itself doesn't create anything, just shows UI
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (user or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"title": {
						Type:        "string",
						Description: "Pre-fill the issue title",
					},
					"body": {
						Type:        "string",
						Description: "Pre-fill the issue body content (supports GitHub Flavored Markdown)",
					},
					"labels": {
						Type:        "array",
						Description: "Pre-select labels by name. Use list_label to get valid label names for the repository.",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"assignees": {
						Type:        "array",
						Description: "Pre-select assignees by username. Use list_assignees to get valid usernames for the repository.",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"milestone": {
						Type:        "number",
						Description: "Pre-select milestone by number. Use list_milestones to get valid milestone numbers for the repository.",
					},
					"type": {
						Type:        "string",
						Description: "Pre-select issue type by name. Use list_issue_types to get valid types for the organization.",
					},
				},
				Required: []string{"owner", "repo"},
			},
			// MCP Apps UI metadata - links this tool to its UI resource
			Meta: mcp.Meta{
				"ui": map[string]any{
					"resourceUri": IssueWriteUIResourceURI,
				},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(_ context.Context, _ ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Return a simple confirmation message
			// The UI will be rendered by the host and will handle the actual form
			return utils.NewToolResultText("Ready to create an issue in " + owner + "/" + repo), nil, nil
		},
	)
}

// IssueWriteUIHTML is the HTML content for the issue_write tool's MCP App UI.
// This UI provides a GitHub-like interface for creating issues.
//
// How this MCP App works:
// 1. Server registers this HTML as a resource at ui://github-mcp-server/issue-write
