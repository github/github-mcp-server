package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SearchRepositories creates a tool to search for GitHub repositories.
func SearchRepositories(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Repository search query. Examples: 'machine learning in:name stars:>1000 language:python', 'topic:react', 'user:facebook'. Supports advanced search syntax for precise filtering.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort repositories by field, defaults to best match",
				Enum:        []any{"stars", "forks", "help-wanted-issues", "updated"},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
			"minimal_output": {
				Type:        "boolean",
				Description: "Return minimal repository information (default: true). When false, returns full GitHub API repository objects.",
				Default:     json.RawMessage(`true`),
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "search_repositories",
			Description: t("TOOL_SEARCH_REPOSITORIES_DESCRIPTION", "Find GitHub repositories by name, description, readme, topics, or other metadata. Perfect for discovering projects, finding examples, or locating specific repositories across GitHub."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_REPOSITORIES_USER_TITLE", "Search repositories"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			query, err := RequiredParam[string](args, "query")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			sort, err := OptionalParam[string](args, "sort")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			order, err := OptionalParam[string](args, "order")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			minimalOutput, err := OptionalBoolParamWithDefault(args, "minimal_output", true)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			opts := &github.SearchOptions{
				Sort:  sort,
				Order: order,
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			result, resp, err := client.Search.Repositories(ctx, query, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to search repositories with query '%s'", query),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to search repositories", resp, body), nil, nil
			}

			// Return either minimal or full response based on parameter
			var r []byte
			if minimalOutput {
				minimalRepos := make([]MinimalRepository, 0, len(result.Repositories))
				for _, repo := range result.Repositories {
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
					if repo.CreatedAt != nil {
						minimalRepo.CreatedAt = repo.CreatedAt.Format("2006-01-02T15:04:05Z")
					}
					if repo.Topics != nil {
						minimalRepo.Topics = repo.Topics
					}

					minimalRepos = append(minimalRepos, minimalRepo)
				}

				minimalResult := &MinimalSearchRepositoriesResult{
					TotalCount:        result.GetTotal(),
					IncompleteResults: result.GetIncompleteResults(),
					Items:             minimalRepos,
				}

				r, err = json.Marshal(minimalResult)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to marshal minimal response", err), nil, nil
				}
			} else {
				r, err = json.Marshal(result)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to marshal full response", err), nil, nil
				}
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// SearchCode creates a tool to search for code across GitHub repositories.
func SearchCode(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Search query using GitHub's powerful code search syntax. Examples: 'content:Skill language:Java org:github', 'NOT is:archived language:Python OR language:go', 'repo:github/github-mcp-server'. Supports exact matching, language filters, path filters, and more.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort field ('indexed' only)",
			},
			"order": {
				Type:        "string",
				Description: "Sort order for results",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "search_code",
			Description: t("TOOL_SEARCH_CODE_DESCRIPTION", "Fast and precise code search across ALL GitHub repositories using GitHub's native search engine. Best for finding exact symbols, functions, classes, or specific code patterns."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_CODE_USER_TITLE", "Search code"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			query, err := RequiredParam[string](args, "query")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			sort, err := OptionalParam[string](args, "sort")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			order, err := OptionalParam[string](args, "order")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			opts := &github.SearchOptions{
				Sort:  sort,
				Order: order,
				ListOptions: github.ListOptions{
					PerPage: pagination.PerPage,
					Page:    pagination.Page,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			result, resp, err := client.Search.Code(ctx, query, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to search code with query '%s'", query),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to search code", resp, body), nil, nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func userOrOrgHandler(ctx context.Context, accountType string, deps ToolDependencies, args map[string]any) (*mcp.CallToolResult, any, error) {
	query, err := RequiredParam[string](args, "query")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	sort, err := OptionalParam[string](args, "sort")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	order, err := OptionalParam[string](args, "order")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	pagination, err := OptionalPaginationParams(args)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	opts := &github.SearchOptions{
		Sort:  sort,
		Order: order,
		ListOptions: github.ListOptions{
			PerPage: pagination.PerPage,
			Page:    pagination.Page,
		},
	}

	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
	}

	searchQuery := query
	if !hasTypeFilter(query) {
		searchQuery = "type:" + accountType + " " + query
	}
	result, resp, err := client.Search.Users(ctx, searchQuery, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to search %ss with query '%s'", accountType, query),
			resp,
			err,
		), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, fmt.Sprintf("failed to search %ss", accountType), resp, body), nil, nil
	}

	minimalUsers := make([]MinimalUser, 0, len(result.Users))

	for _, user := range result.Users {
		if user.Login != nil {
			mu := MinimalUser{
				Login:      user.GetLogin(),
				ID:         user.GetID(),
				ProfileURL: user.GetHTMLURL(),
				AvatarURL:  user.GetAvatarURL(),
			}
			minimalUsers = append(minimalUsers, mu)
		}
	}
	minimalResp := &MinimalSearchUsersResult{
		TotalCount:        result.GetTotal(),
		IncompleteResults: result.GetIncompleteResults(),
		Items:             minimalUsers,
	}
	if result.Total != nil {
		minimalResp.TotalCount = *result.Total
	}
	if result.IncompleteResults != nil {
		minimalResp.IncompleteResults = *result.IncompleteResults
	}

	r, err := json.Marshal(minimalResp)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
	}
	return utils.NewToolResultText(string(r)), nil, nil
}

// SearchUsers creates a tool to search for GitHub users.
func SearchUsers(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "User search query. Examples: 'john smith', 'location:seattle', 'followers:>100'. Search is automatically scoped to type:user.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort users by number of followers or repositories, or when the person joined GitHub.",
				Enum:        []any{"followers", "repositories", "joined"},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataUsers,
		mcp.Tool{
			Name:        "search_users",
			Description: t("TOOL_SEARCH_USERS_DESCRIPTION", "Find GitHub users by username, real name, or other profile information. Useful for locating developers, contributors, or team members."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_USERS_USER_TITLE", "Search users"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			return userOrOrgHandler(ctx, "user", deps, args)
		},
	)
}

// SearchOrgs creates a tool to search for GitHub organizations.
func SearchOrgs(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Organization search query. Examples: 'microsoft', 'location:california', 'created:>=2025-01-01'. Search is automatically scoped to type:org.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort field by category",
				Enum:        []any{"followers", "repositories", "joined"},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataOrgs,
		mcp.Tool{
			Name:        "search_orgs",
			Description: t("TOOL_SEARCH_ORGS_DESCRIPTION", "Find GitHub organizations by name, location, or other organization metadata. Ideal for discovering companies, open source foundations, or teams."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_ORGS_USER_TITLE", "Search organizations"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			return userOrOrgHandler(ctx, "org", deps, args)
		},
	)
}

// SearchCommits creates a tool to search for commits across GitHub repositories.
func SearchCommits(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Commit search query. Examples: 'repo:owner/repo fix bug', 'author:defunkt', 'committer-date:>2024-01-01'. Supports advanced search syntax.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort field ('author-date' or 'committer-date')",
				Enum:        []any{"author-date", "committer-date"},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "search_commits",
			Description: t("TOOL_SEARCH_COMMITS_DESCRIPTION", "Search for commits across GitHub repositories using specialized commit search syntax. Great for finding specific changes, authors, or messages."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_COMMITS_USER_TITLE", "Search commits"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			query, err := RequiredParam[string](args, "query")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			sort, err := OptionalParam[string](args, "sort")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			order, err := OptionalParam[string](args, "order")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			opts := &github.SearchOptions{
				Sort:  sort,
				Order: order,
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			result, resp, err := client.Search.Commits(ctx, query, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to search commits with query '%s'", query),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to search commits", resp, body), nil, nil
			}

			convertCommitResultToMinimalCommit := func(commit *github.CommitResult) MinimalCommit {
				minimalCommit := MinimalCommit{
					SHA:     commit.GetSHA(),
					HTMLURL: commit.GetHTMLURL(),
				}

				if commit.Commit != nil {
					minimalCommit.Commit = &MinimalCommitInfo{
						Message: commit.Commit.GetMessage(),
					}

					if commit.Commit.Author != nil {
						minimalCommit.Commit.Author = &MinimalCommitAuthor{
							Name:  commit.Commit.Author.GetName(),
							Email: commit.Commit.Author.GetEmail(),
						}
						if commit.Commit.Author.Date != nil {
							minimalCommit.Commit.Author.Date = commit.Commit.Author.Date.Format(time.RFC3339)
						}
					}

					if commit.Commit.Committer != nil {
						minimalCommit.Commit.Committer = &MinimalCommitAuthor{
							Name:  commit.Commit.Committer.GetName(),
							Email: commit.Commit.Committer.GetEmail(),
						}
						if commit.Commit.Committer.Date != nil {
							minimalCommit.Commit.Committer.Date = commit.Commit.Committer.Date.Format(time.RFC3339)
						}
					}
				}

				if commit.Author != nil {
					minimalCommit.Author = &MinimalUser{
						Login:      commit.Author.GetLogin(),
						ID:         commit.Author.GetID(),
						ProfileURL: commit.Author.GetHTMLURL(),
						AvatarURL:  commit.Author.GetAvatarURL(),
					}
				}

				if commit.Committer != nil {
					minimalCommit.Committer = &MinimalUser{
						Login:      commit.Committer.GetLogin(),
						ID:         commit.Committer.GetID(),
						ProfileURL: commit.Committer.GetHTMLURL(),
						AvatarURL:  commit.Committer.GetAvatarURL(),
					}
				}

				return minimalCommit
			}

			minimalCommits := make([]MinimalCommit, 0, len(result.Commits))
			for _, commit := range result.Commits {
				minimalCommits = append(minimalCommits, convertCommitResultToMinimalCommit(commit))
			}

			minimalResult := &MinimalSearchCommitsResult{
				TotalCount:        result.GetTotal(),
				IncompleteResults: result.GetIncompleteResults(),
				Items:             minimalCommits,
			}

			r, err := json.Marshal(minimalResult)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
