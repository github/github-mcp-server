package github

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/url"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/sanitize"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AutomationTag struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ListAutomationTagsResponse struct {
	Tags       []AutomationTag `json:"tags"`
	TotalCount int             `json:"total_count"`
	NextPage   int             `json:"nextPage" jsonschema:"Next page from the REST Link header, or 0 when there is no next page"`
}

func ListAutomationTags(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[RepositoryListInput, *ListAutomationTagsResponse](
		ToolsetMetadataCopilotAutomations,
		mcp.Tool{
			Name:        "list_automation_tags",
			Description: t("TOOL_LIST_AUTOMATION_TAGS_DESCRIPTION", "List caller-visible automation tags in a repository. Tags organize automations; they do not execute them. Use nextPage with the same perPage to continue. Pages are not a snapshot across concurrent changes."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_AUTOMATION_TAGS_USER_TITLE", "List automation tags"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {Type: "string", Description: "Repository owner", MinLength: new(1)},
					"repo":  {Type: "string", Description: "Repository name", MinLength: new(1)},
				},
				Required: []string{"owner", "repo"},
			}),
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input RepositoryListInput) (*mcp.CallToolResult, *ListAutomationTagsResponse, error) {
			if input.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if input.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}
			if input.Page != nil && *input.Page < 1 {
				return utils.NewToolResultError("page must be at least 1"), nil, nil
			}
			if input.PerPage != nil && (*input.PerPage < 1 || *input.PerPage > 100) {
				return utils.NewToolResultError("perPage must be between 1 and 100"), nil, nil
			}
			if input.Owner == "." || input.Owner == ".." {
				return utils.NewToolResultError(fmt.Sprintf("owner must not be %q", input.Owner)), nil, nil
			}
			if input.Repo == "." || input.Repo == ".." {
				return utils.NewToolResultError(fmt.Sprintf("repo must not be %q", input.Repo)), nil, nil
			}
			pagination := PaginationParams{Page: 1, PerPage: 30}
			if input.Page != nil {
				pagination.Page = *input.Page
			}
			if input.PerPage != nil {
				pagination.PerPage = *input.PerPage
			}
			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			apiURL := fmt.Sprintf("agents/repos/%s/%s/automation-tags?page=%d&per_page=%d",
				url.PathEscape(input.Owner), url.PathEscape(input.Repo), pagination.Page, pagination.PerPage)
			req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create automation tags request", err), nil, nil
			}
			var output ListAutomationTagsResponse
			resp, err := client.Do(req, &output)
			if resp != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list automation tags", resp, err), nil, nil
			}
			if resp.StatusCode != http.StatusOK {
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list automation tags", resp, nil), nil, nil
			}
			for i := range output.Tags {
				sanitizeAutomationTagNameAndDescription(&output.Tags[i])
			}
			output.NextPage = resp.NextPage
			return MarshalledTextResult(&output), &output, nil
		},
	)
}

type GetAutomationTagInput struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	TagID string `json:"tag_id"`
}

func GetAutomationTag(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[GetAutomationTagInput, *AutomationTag](
		ToolsetMetadataCopilotAutomations,
		mcp.Tool{
			Name:        "get_automation_tag",
			Description: t("TOOL_GET_AUTOMATION_TAG_DESCRIPTION", "Get a caller-visible automation tag in a repository by ID. Tags organize automations; they do not execute them. A tag that is missing, deleted, or not visible to the caller returns a 404 error."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_AUTOMATION_TAG_USER_TITLE", "Get automation tag"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":  {Type: "string", Description: "Repository owner", MinLength: new(1)},
					"repo":   {Type: "string", Description: "Repository name", MinLength: new(1)},
					"tag_id": {Type: "string", Description: "Automation tag ID, as returned by list_automation_tags", MinLength: new(1)},
				},
				Required: []string{"owner", "repo", "tag_id"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input GetAutomationTagInput) (*mcp.CallToolResult, *AutomationTag, error) {
			if input.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if input.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}
			if input.TagID == "" {
				return utils.NewToolResultError("missing required parameter: tag_id"), nil, nil
			}
			if input.Owner == "." {
				return utils.NewToolResultError(`owner must not be "."`), nil, nil
			}
			if input.Repo == "." {
				return utils.NewToolResultError(`repo must not be "."`), nil, nil
			}
			// URL resolution removes a "." segment, which would send this request to
			// ".../automation-tags/". NewRequest already rejects "..".
			if input.TagID == "." {
				return utils.NewToolResultError(`tag_id must not be "."`), nil, nil
			}
			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			apiURL := fmt.Sprintf("agents/repos/%s/%s/automation-tags/%s",
				url.PathEscape(input.Owner), url.PathEscape(input.Repo), url.PathEscape(input.TagID))
			req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create automation tag request", err), nil, nil
			}
			var output AutomationTag
			resp, err := client.Do(req, &output)
			if resp != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get automation tag", resp, err), nil, nil
			}
			if resp.StatusCode != http.StatusOK {
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get automation tag", resp, nil), nil, nil
			}
			sanitizeAutomationTagNameAndDescription(&output)
			return MarshalledTextResult(&output), &output, nil
		},
	)
}

type CreateAutomationTagInput struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func CreateAutomationTag(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[CreateAutomationTagInput, *AutomationTag](
		ToolsetMetadataCopilotAutomations,
		mcp.Tool{
			Name:        "create_automation_tag",
			Description: t("TOOL_CREATE_AUTOMATION_TAG_DESCRIPTION", "Create an automation tag in a repository. Tags organize automations; they do not execute them. Each call, including a retry, creates a new tag, even when another tag has the same name."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_CREATE_AUTOMATION_TAG_USER_TITLE", "Create automation tag"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":       {Type: "string", Description: "Repository owner", MinLength: new(1)},
					"repo":        {Type: "string", Description: "Repository name", MinLength: new(1)},
					"name":        {Type: "string", Description: "Display name for the new tag"},
					"description": {Type: "string", Description: "Description of the new tag. Omit for an empty description."},
				},
				Required: []string{"owner", "repo", "name"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input CreateAutomationTagInput) (*mcp.CallToolResult, *AutomationTag, error) {
			if input.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if input.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}
			if input.Owner == "." {
				return utils.NewToolResultError(`owner must not be "."`), nil, nil
			}
			if input.Repo == "." {
				return utils.NewToolResultError(`repo must not be "."`), nil, nil
			}
			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			// A reused key returns the tag it first created without comparing name or
			// description, so each call sends a new key.
			body := map[string]any{"name": input.Name, "idempotency_key": rand.Text()}
			if input.Description != "" {
				body["description"] = input.Description
			}
			apiURL := fmt.Sprintf("agents/repos/%s/%s/automation-tags", url.PathEscape(input.Owner), url.PathEscape(input.Repo))
			req, err := client.NewRequest(ctx, http.MethodPost, apiURL, body)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create automation tag request", err), nil, nil
			}
			var output AutomationTag
			resp, err := client.Do(req, &output)
			if resp != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create automation tag", resp, err), nil, nil
			}
			if resp.StatusCode != http.StatusCreated {
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create automation tag", resp, nil), nil, nil
			}
			return MarshalledTextResult(&output), &output, nil
		},
	)
}

func sanitizeAutomationTagNameAndDescription(tag *AutomationTag) {
	tag.Name = sanitize.PlainText(tag.Name)
	tag.Description = sanitize.Content(tag.Description)
}
