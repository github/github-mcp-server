package github

import (
	"context"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func ListStarredRepositories(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	tool := mcp.Tool{
		Name:        "list_starred_repositories",
		Description: t("TOOL_LIST_STARRED_REPOSITORIES_DESCRIPTION", "List starred repositories"),
		Annotations: &mcp.ToolAnnotations{
			Title:        t("TOOL_LIST_STARRED_REPOSITORIES_USER_TITLE", "List starred repositories"),
			ReadOnlyHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"username": {
					Type:        "string",
					Description: "Username to list starred repositories for. Defaults to the authenticated user.",
				},
				"sort": {
					Type:        "string",
					Description: "How to sort the results. Can be either 'created' (when the repository was starred) or 'updated' (when the repository was last pushed to).",
					Enum:        []any{"created", "updated"},
				},
				"direction": {
					Type:        "string",
					Description: "The direction to sort the results by.",
					Enum:        []any{"asc", "desc"},
				},
			},
		},
	}

	// Add pagination parameters
	tool.InputSchema = WithPagination(tool.InputSchema.(*jsonschema.Schema))

	handler := mcp.ToolHandlerFor[map[string]any, any](func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		username, err := OptionalParam[string](args, "username")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		sort, err := OptionalParam[string](args, "sort")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		direction, err := OptionalParam[string](args, "direction")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		pagination, err := OptionalPaginationParams(args)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
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
			return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
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
			), nil, err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, err
			}
			return utils.NewToolResultError(fmt.Sprintf("failed to list starred repositories: %s", string(body))), nil, nil
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

		return MarshalledTextResult(minimalRepos), nil, nil
	})

	return tool, handler
}

// StarRepository creates a tool to star a repository.
func StarRepository(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	tool := mcp.Tool{
		Name:        "star_repository",
		Description: t("TOOL_STAR_REPOSITORY_DESCRIPTION", "Star a GitHub repository"),
		Annotations: &mcp.ToolAnnotations{
			Title:        t("TOOL_STAR_REPOSITORY_USER_TITLE", "Star repository"),
			ReadOnlyHint: false,
		},
		InputSchema: &jsonschema.Schema{
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
		},
	}

	handler := mcp.ToolHandlerFor[map[string]any, any](func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, err := RequiredParam[string](args, "owner")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		repo, err := RequiredParam[string](args, "repo")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}

		client, err := getClient(ctx)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
		}

		resp, err := client.Activity.Star(ctx, owner, repo)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx,
				fmt.Sprintf("failed to star repository %s/%s", owner, repo),
				resp,
				err,
			), nil, err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNoContent {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, err
			}
			return utils.NewToolResultError(fmt.Sprintf("failed to star repository: %s", string(body))), nil, nil
		}

		return utils.NewToolResultText(fmt.Sprintf("Successfully starred repository %s/%s", owner, repo)), nil, nil
	})

	return tool, handler
}

// UnstarRepository creates a tool to unstar a repository.
func UnstarRepository(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	tool := mcp.Tool{
		Name:        "unstar_repository",
		Description: t("TOOL_UNSTAR_REPOSITORY_DESCRIPTION", "Unstar a GitHub repository"),
		Annotations: &mcp.ToolAnnotations{
			Title:        t("TOOL_UNSTAR_REPOSITORY_USER_TITLE", "Unstar repository"),
			ReadOnlyHint: false,
		},
		InputSchema: &jsonschema.Schema{
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
		},
	}

	handler := mcp.ToolHandlerFor[map[string]any, any](func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		owner, err := RequiredParam[string](args, "owner")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		repo, err := RequiredParam[string](args, "repo")
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}

		client, err := getClient(ctx)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, err
		}

		resp, err := client.Activity.Unstar(ctx, owner, repo)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx,
				fmt.Sprintf("failed to unstar repository %s/%s", owner, repo),
				resp,
				err,
			), nil, err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNoContent {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, err
			}
			return utils.NewToolResultError(fmt.Sprintf("failed to unstar repository: %s", string(body))), nil, nil
		}

		return utils.NewToolResultText(fmt.Sprintf("Successfully unstarred repository %s/%s", owner, repo)), nil, nil
	})

	return tool, handler
}
