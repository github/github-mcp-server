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

// searchRepositories creates a tool to search for GitHub repositories.
func searchRepositories(client *github.Client, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_repositories",
			mcp.WithDescription(t("TOOL_SEARCH_REPOSITORIES_DESCRIPTION", "Search for GitHub repositories")),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query"),
			),
			mcp.WithNumber("page",
				mcp.Description("Page number for pagination"),
			),
			mcp.WithNumber("perPage",
				mcp.Description("Results per page (max 100)"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, err := requiredParam[string](request, "query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			page, err := optionalIntParamWithDefault(request, "page", 1)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			perPage, err := optionalIntParamWithDefault(request, "perPage", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.SearchOptions{
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: perPage,
				},
			}

			result, resp, err := client.Search.Repositories(ctx, query, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to search repositories: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to search repositories: %s", string(body))), nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// searchCode creates a tool to search for code across GitHub repositories.
func searchCode(client *github.Client, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_code",
			mcp.WithDescription(t("TOOL_SEARCH_CODE_DESCRIPTION", "Search for code across GitHub repositories")),
			mcp.WithString("q",
				mcp.Required(),
				mcp.Description("Search query using GitHub code search syntax"),
			),
			mcp.WithString("sort",
				mcp.Description("Sort field ('indexed' only)"),
			),
			mcp.WithString("order",
				mcp.Description("Sort order ('asc' or 'desc')"),
				mcp.Enum("asc", "desc"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Results per page (max 100)"),
				mcp.Min(1),
				mcp.Max(100),
			),
			mcp.WithNumber("page",
				mcp.Description("Page number"),
				mcp.Min(1),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, err := requiredParam[string](request, "q")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sort, err := optionalParam[string](request, "sort")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			order, err := optionalParam[string](request, "order")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			perPage, err := optionalIntParamWithDefault(request, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			page, err := optionalIntParamWithDefault(request, "page", 1)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.SearchOptions{
				Sort:  sort,
				Order: order,
				ListOptions: github.ListOptions{
					PerPage: perPage,
					Page:    page,
				},
			}

			result, resp, err := client.Search.Code(ctx, query, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to search code: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to search code: %s", string(body))), nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// searchUsers creates a tool to search for GitHub users.
func searchUsers(client *github.Client, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_users",
			mcp.WithDescription(t("TOOL_SEARCH_USERS_DESCRIPTION", "Search for GitHub users")),
			mcp.WithString("q",
				mcp.Required(),
				mcp.Description("Search query using GitHub users search syntax"),
			),
			mcp.WithString("sort",
				mcp.Description("Sort field (followers, repositories, joined)"),
				mcp.Enum("followers", "repositories", "joined"),
			),
			mcp.WithString("order",
				mcp.Description("Sort order ('asc' or 'desc')"),
				mcp.Enum("asc", "desc"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Results per page (max 100)"),
				mcp.Min(1),
				mcp.Max(100),
			),
			mcp.WithNumber("page",
				mcp.Description("Page number"),
				mcp.Min(1),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query, err := requiredParam[string](request, "q")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sort, err := optionalParam[string](request, "sort")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			order, err := optionalParam[string](request, "order")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			perPage, err := optionalIntParamWithDefault(request, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			page, err := optionalIntParamWithDefault(request, "page", 1)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.SearchOptions{
				Sort:  sort,
				Order: order,
				ListOptions: github.ListOptions{
					PerPage: perPage,
					Page:    page,
				},
			}

			result, resp, err := client.Search.Users(ctx, query, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to search users: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to search users: %s", string(body))), nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
