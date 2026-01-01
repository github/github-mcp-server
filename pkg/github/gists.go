package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListGists creates a tool to list gists for a user
func ListGists(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGists,
		mcp.Tool{
			Name:        "list_gists",
			Description: t("TOOL_LIST_GISTS_DESCRIPTION", "List gists for a user"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_GISTS", "List Gists"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"username": {
						Type:        "string",
						Description: "GitHub username (omit for authenticated user's gists)",
					},
					"since": {
						Type:        "string",
						Description: "Only gists updated after this time (ISO 8601 timestamp)",
					},
				},
			}),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			username, err := OptionalParam[string](args, "username")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			since, err := OptionalParam[string](args, "since")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			opts := &github.GistListOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			// Parse since timestamp if provided
			if since != "" {
				sinceTime, err := parseISOTimestamp(since)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("invalid since timestamp: %v", err)), nil, nil
				}
				opts.Since = sinceTime
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			gists, resp, err := client.Gists.List(ctx, username, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list gists", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list gists", resp, body), nil, nil
			}

			r, err := json.Marshal(gists)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GetGist creates a tool to get the content of a gist
func GetGist(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGists,
		mcp.Tool{
			Name:        "get_gist",
			Description: t("TOOL_GET_GIST_DESCRIPTION", "Get gist content of a particular gist, by gist ID"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_GIST", "Get Gist Content"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"gist_id": {
						Type:        "string",
						Description: "The ID of the gist",
					},
				},
				Required: []string{"gist_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			gistID, err := RequiredParam[string](args, "gist_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			gist, resp, err := client.Gists.Get(ctx, gistID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get gist", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get gist", resp, body), nil, nil
			}

			r, err := json.Marshal(gist)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// CreateGist creates a tool to create a new gist
func CreateGist(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGists,
		mcp.Tool{
			Name:        "create_gist",
			Description: t("TOOL_CREATE_GIST_DESCRIPTION", "Create a new gist"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_GIST", "Create Gist"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"description": {
						Type:        "string",
						Description: "Description of the gist",
					},
					"filename": {
						Type:        "string",
						Description: "Filename for simple single-file gist creation",
					},
					"content": {
						Type:        "string",
						Description: "Content for simple single-file gist creation",
					},
					"public": {
						Type:        "boolean",
						Description: "Whether the gist is public",
						Default:     json.RawMessage(`false`),
					},
				},
				Required: []string{"filename", "content"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			description, err := OptionalParam[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			filename, err := RequiredParam[string](args, "filename")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			content, err := RequiredParam[string](args, "content")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			public, err := OptionalParam[bool](args, "public")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			files := make(map[github.GistFilename]github.GistFile)
			files[github.GistFilename(filename)] = github.GistFile{
				Filename: github.Ptr(filename),
				Content:  github.Ptr(content),
			}

			gist := &github.Gist{
				Files:       files,
				Public:      github.Ptr(public),
				Description: github.Ptr(description),
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			createdGist, resp, err := client.Gists.Create(ctx, gist)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create gist", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create gist", resp, body), nil, nil
			}

			minimalResponse := MinimalResponse{
				ID:  createdGist.GetID(),
				URL: createdGist.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// UpdateGist creates a tool to edit an existing gist
func UpdateGist(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGists,
		mcp.Tool{
			Name:        "update_gist",
			Description: t("TOOL_UPDATE_GIST_DESCRIPTION", "Update an existing gist"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_GIST", "Update Gist"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"gist_id": {
						Type:        "string",
						Description: "ID of the gist to update",
					},
					"description": {
						Type:        "string",
						Description: "Updated description of the gist",
					},
					"filename": {
						Type:        "string",
						Description: "Filename to update or create",
					},
					"content": {
						Type:        "string",
						Description: "Content for the file",
					},
				},
				Required: []string{"gist_id", "filename", "content"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			gistID, err := RequiredParam[string](args, "gist_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			description, err := OptionalParam[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			filename, err := RequiredParam[string](args, "filename")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			content, err := RequiredParam[string](args, "content")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			files := make(map[github.GistFilename]github.GistFile)
			files[github.GistFilename(filename)] = github.GistFile{
				Filename: github.Ptr(filename),
				Content:  github.Ptr(content),
			}

			gist := &github.Gist{
				Files:       files,
				Description: github.Ptr(description),
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			updatedGist, resp, err := client.Gists.Edit(ctx, gistID, gist)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update gist", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to update gist", resp, body), nil, nil
			}

			minimalResponse := MinimalResponse{
				ID:  updatedGist.GetID(),
				URL: updatedGist.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
