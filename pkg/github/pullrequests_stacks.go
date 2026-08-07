package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
)

// StackLayer represents a pull request layer inside a stack.
type StackLayer struct {
	PullNumber     int    `json:"pull_number,omitempty"`
	Head           string `json:"head,omitempty"`
	Base           string `json:"base,omitempty"`
	Title          string `json:"title,omitempty"`
	State          string `json:"state,omitempty"`
	Mergeable      *bool  `json:"mergeable,omitempty"`
	ReviewDecision string `json:"review_decision,omitempty"`
}

// Stack represents a GitHub native pull request stack.
type Stack struct {
	ID           int64        `json:"id,omitempty"`
	StackNumber  int          `json:"stack_number,omitempty"`
	Title        string       `json:"title,omitempty"`
	Base         string       `json:"base,omitempty"`
	PullRequests []StackLayer `json:"pull_requests,omitempty"`
	CreatedAt    string       `json:"created_at,omitempty"`
	UpdatedAt    string       `json:"updated_at,omitempty"`
}

// LinkStackInput represents the JSON payload to create/link a stack.
type LinkStackInput struct {
	Base        string `json:"base,omitempty"`
	PullNumbers []int  `json:"pull_numbers"`
}

// UpdateStackInput represents the JSON payload to update a stack.
type UpdateStackInput struct {
	Base        string `json:"base,omitempty"`
	PullNumbers []int  `json:"pull_numbers,omitempty"`
}

func parseIntArray(args map[string]any, p string) ([]int, error) {
	val, ok := args[p]
	if !ok {
		return nil, nil
	}
	switch v := val.(type) {
	case []any:
		res := make([]int, len(v))
		for i, item := range v {
			num, err := toInt(item)
			if err != nil {
				return nil, fmt.Errorf("item at index %d in %s is invalid: %w", i, p, err)
			}
			res[i] = num
		}
		return res, nil
	case []int:
		return v, nil
	case []float64:
		res := make([]int, len(v))
		for i, num := range v {
			res[i] = int(num)
		}
		return res, nil
	default:
		return nil, fmt.Errorf("parameter %s is not an array", p)
	}
}

// GetStack creates a tool to fetch details for a pull request stack.
func GetStack(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"stackNumber": {
				Type:        "number",
				Description: "Stack number",
			},
			"pullNumber": {
				Type:        "number",
				Description: "Pull request number contained within the target stack",
			},
		},
		Required: []string{"owner", "repo"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "get_stack",
			Description: t("TOOL_GET_STACK_DESCRIPTION", "Get details of a specific pull request stack in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_STACK_TITLE", "Get pull request stack details"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
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

			stackNumber, err := OptionalIntParam(args, "stackNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			pullNumber, err := OptionalIntParam(args, "pullNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if stackNumber == 0 && pullNumber == 0 {
				return utils.NewToolResultError("must provide either stackNumber or pullNumber"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			var urlStr string
			if stackNumber != 0 {
				urlStr = fmt.Sprintf("repos/%s/%s/stacks/%d", owner, repo, stackNumber)
			} else {
				urlStr = fmt.Sprintf("repos/%s/%s/stacks?pull_request=%d", owner, repo, pullNumber)
			}

			req, err := client.NewRequest(http.MethodGet, urlStr, nil)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			if stackNumber != 0 {
				var stack Stack
				resp, err := client.Do(ctx, req, &stack)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get pull request stack", resp, err), nil, nil
				}

				r, err := json.Marshal(stack)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
				}
				return utils.NewToolResultText(string(r)), nil, nil
			}

			var stacks []Stack
			resp, err := client.Do(ctx, req, &stacks)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get pull request stack", resp, err), nil, nil
			}

			r, err := json.Marshal(stacks)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// ListStacks creates a tool to list pull request stacks in a repository.
func ListStacks(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
		},
		Required: []string{"owner", "repo"},
	}
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "list_stacks",
			Description: t("TOOL_LIST_STACKS_DESCRIPTION", "List pull request stacks in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_STACKS_TITLE", "List pull request stacks"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
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

			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			urlStr := fmt.Sprintf("repos/%s/%s/stacks?page=%d&per_page=%d", owner, repo, pagination.Page, pagination.PerPage)
			req, err := client.NewRequest(http.MethodGet, urlStr, nil)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			var stacks []Stack
			resp, err := client.Do(ctx, req, &stacks)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list pull request stacks", resp, err), nil, nil
			}

			r, err := json.Marshal(stacks)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// LinkStack creates a tool to link PRs into a new stack.
func LinkStack(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"pullNumbers": {
				Type:        "array",
				Description: "Ordered list of pull request numbers (bottom to top)",
				Items: &jsonschema.Schema{
					Type: "number",
				},
			},
			"base": {
				Type:        "string",
				Description: "Base/trunk branch name",
			},
		},
		Required: []string{"owner", "repo", "pullNumbers"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "link_stack",
			Description: t("TOOL_LINK_STACK_DESCRIPTION", "Create or link a pull request stack from an ordered sequence of pull request numbers."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LINK_STACK_TITLE", "Link pull request stack"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
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

			pullNumbers, err := parseIntArray(args, "pullNumbers")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if len(pullNumbers) == 0 {
				return utils.NewToolResultError("missing required parameter: pullNumbers"), nil, nil
			}

			base, err := OptionalParam[string](args, "base")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			input := LinkStackInput{
				Base:        base,
				PullNumbers: pullNumbers,
			}

			urlStr := fmt.Sprintf("repos/%s/%s/stacks", owner, repo)
			req, err := client.NewRequest(http.MethodPost, urlStr, input)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			var stack Stack
			resp, err := client.Do(ctx, req, &stack)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to link pull request stack", resp, err), nil, nil
			}

			r, err := json.Marshal(stack)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// UpdateStack creates a tool to update an existing pull request stack.
func UpdateStack(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"stackNumber": {
				Type:        "number",
				Description: "Stack number to update",
			},
			"pullNumbers": {
				Type:        "array",
				Description: "Updated ordered list of pull request numbers",
				Items: &jsonschema.Schema{
					Type: "number",
				},
			},
			"base": {
				Type:        "string",
				Description: "Updated base/trunk branch name",
			},
		},
		Required: []string{"owner", "repo", "stackNumber"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "update_stack",
			Description: t("TOOL_UPDATE_STACK_DESCRIPTION", "Update an existing pull request stack's layers or base branch."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_STACK_TITLE", "Update pull request stack"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
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
			stackNumber, err := RequiredInt(args, "stackNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			pullNumbers, err := parseIntArray(args, "pullNumbers")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			base, err := OptionalParam[string](args, "base")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			input := UpdateStackInput{
				Base:        base,
				PullNumbers: pullNumbers,
			}

			urlStr := fmt.Sprintf("repos/%s/%s/stacks/%d", owner, repo, stackNumber)
			req, err := client.NewRequest(http.MethodPatch, urlStr, input)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			var stack Stack
			resp, err := client.Do(ctx, req, &stack)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update pull request stack", resp, err), nil, nil
			}

			r, err := json.Marshal(stack)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// DissolveStack creates a tool to dissolve a pull request stack.
func DissolveStack(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"stackNumber": {
				Type:        "number",
				Description: "Stack number to dissolve",
			},
		},
		Required: []string{"owner", "repo", "stackNumber"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "dissolve_stack",
			Description: t("TOOL_DISSOLVE_STACK_DESCRIPTION", "Dissolve a pull request stack object without deleting the underlying pull requests."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_DISSOLVE_STACK_TITLE", "Dissolve pull request stack"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
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
			stackNumber, err := RequiredInt(args, "stackNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			urlStr := fmt.Sprintf("repos/%s/%s/stacks/%d", owner, repo, stackNumber)
			req, err := client.NewRequest(http.MethodDelete, urlStr, nil)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
			}

			resp, err := client.Do(ctx, req, nil)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to dissolve pull request stack", resp, err), nil, nil
			}

			return utils.NewToolResultText(fmt.Sprintf("Successfully dissolved stack %d in %s/%s", stackNumber, owner, repo)), nil, nil
		},
	)
}
