package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ListOrganizations creates a tool to list organizations a user is part of.
func ListOrganizations(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_organizations",
			mcp.WithDescription(t("TOOL_LIST_ORGANIZATIONS_DESCRIPTION", "List organizations the authenticated user is a member of")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_ORGANIZATIONS_USER_TITLE", "List organizations"),
				ReadOnlyHint: true,
			}),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ListOptions{
				Page:    pagination.page,
				PerPage: pagination.perPage,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Call the GitHub API to list orgs
			orgs, resp, err := client.Organizations.List(ctx, "", opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list organizations: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list organizations: %s", string(body))), nil
			}

			r, err := json.Marshal(orgs)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetOrganization creates a tool to get details for a specific organization.
func GetOrganization(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_organization",
			mcp.WithDescription(t("TOOL_GET_ORGANIZATION_DESCRIPTION", "Get information about an organization")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_ORGANIZATION_USER_TITLE", "Get organization"),
				ReadOnlyHint: true,
			}),
			mcp.WithString("org",
				mcp.Required(),
				mcp.Description("Organization name"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			orgName, err := requiredParam[string](request, "org")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Call the GitHub API to get org details
			org, resp, err := client.Organizations.Get(ctx, orgName)
			if err != nil {
				return nil, fmt.Errorf("failed to get organization: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get organization: %s", string(body))), nil
			}

			r, err := json.Marshal(org)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}