package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v76/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ListStarredRepositories creates a tool to list starred repositories for the authenticated user or a specified user.
func ListStarredRepositories(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_starred_repositories",
			mcp.WithDescription(t("TOOL_LIST_STARRED_REPOSITORIES_DESCRIPTION", "List starred repositories")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_STARRED_REPOSITORIES_USER_TITLE", "List starred repositories"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("username",
				mcp.Description("Username to list starred repositories for. Defaults to the authenticated user."),
			),
			mcp.WithString("sort",
				mcp.Description("How to sort the results. Can be either 'created' (when the repository was starred) or 'updated' (when the repository was last pushed to)."),
				mcp.Enum("created", "updated"),
			),
			mcp.WithString("direction",
				mcp.Description("The direction to sort the results by."),
				mcp.Enum("asc", "desc"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			username, err := OptionalParam[string](request, "username")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sort, err := OptionalParam[string](request, "sort")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			direction, err := OptionalParam[string](request, "direction")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ActivityListStarredOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}
			if sort != "" {
				opts.Sort = sort
			}
			if direction != "" {
				opts.Direction = direction
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			var repos []*github.StarredRepository
			var resp *github.Response
			if username == "" {
				// List starred repositories for the authenticated user
				repos, resp, err = client.Activity.ListStarred(ctx, "", opts)
			} else {
				// List starred repositories for a specific user
				repos, resp, err = client.Activity.ListStarred(ctx, username, opts)
			}

			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list starred repositories for user '%s'", username),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list starred repositories: %s", string(body))), nil
			}

			// Convert to minimal format
			minimalRepos := make([]MinimalRepository, 0, len(repos))
			for _, starredRepo := range repos {
				repo := starredRepo.Repository
				minimalRepo := MinimalRepository{
					ID:            repo.GetID(),
					Name:          repo.GetName(),
					FullName:      repo.GetFullName(),
					Description:   repo.GetDescription(),
					HTMLURL:       repo.GetHTMLURL(),
					Language:      repo.GetLanguage(),
					Stars:         repo.GetStargazersCount(),
					Forks:         repo.GetForksCount(),
					OpenIssues:    repo.GetOpenIssuesCount(),
					Private:       repo.GetPrivate(),
					Fork:          repo.GetFork(),
					Archived:      repo.GetArchived(),
					DefaultBranch: repo.GetDefaultBranch(),
				}

				if repo.UpdatedAt != nil {
					minimalRepo.UpdatedAt = repo.UpdatedAt.Format("2006-01-02T15:04:05Z")
				}

				minimalRepos = append(minimalRepos, minimalRepo)
			}

			r, err := json.Marshal(minimalRepos)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal starred repositories: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// StarRepository creates a tool to star a repository.
func StarRepository(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("star_repository",
			mcp.WithDescription(t("TOOL_STAR_REPOSITORY_DESCRIPTION", "Star a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_STAR_REPOSITORY_USER_TITLE", "Star repository"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Activity.Star(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to star repository %s/%s", owner, repo),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 204 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to star repository: %s", string(body))), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Successfully starred repository %s/%s", owner, repo)), nil
		}
}

// UnstarRepository creates a tool to unstar a repository.
func UnstarRepository(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("unstar_repository",
			mcp.WithDescription(t("TOOL_UNSTAR_REPOSITORY_DESCRIPTION", "Unstar a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_UNSTAR_REPOSITORY_USER_TITLE", "Unstar repository"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Activity.Unstar(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to unstar repository %s/%s", owner, repo),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 204 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to unstar repository: %s", string(body))), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Successfully unstarred repository %s/%s", owner, repo)), nil
		}
}
