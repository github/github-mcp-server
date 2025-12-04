package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SearchRepositories creates a tool to search for GitHub repositories.
func SearchRepositories(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[SearchRepositoriesInput, any]) {
	return mcp.Tool{
			Name:        "search_repositories",
			Description: t("TOOL_SEARCH_REPOSITORIES_DESCRIPTION", "Find GitHub repositories by name, description, readme, topics, or other metadata. Perfect for discovering projects, finding examples, or locating specific repositories across GitHub."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_REPOSITORIES_USER_TITLE", "Search repositories"),
				ReadOnlyHint: true,
			},
			InputSchema: SearchRepositoriesInput{}.MCPSchema(),
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, input SearchRepositoriesInput) (*mcp.CallToolResult, any, error) {
			opts := &github.SearchOptions{
				Sort:  input.Sort,
				Order: input.Order,
				ListOptions: github.ListOptions{
					Page:    input.Page,
					PerPage: input.PerPage,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			result, resp, err := client.Search.Repositories(ctx, input.Query, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to search repositories with query '%s'", input.Query),
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
				return utils.NewToolResultError(fmt.Sprintf("failed to search repositories: %s", string(body))), nil, nil
			}

			// Return either minimal or full response based on parameter
			var r []byte
			if input.MinimalOutput {
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
		}
}

// SearchCode creates a tool to search for code across GitHub repositories.
func SearchCode(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[SearchCodeInput, any]) {
	return mcp.Tool{
			Name:        "search_code",
			Description: t("TOOL_SEARCH_CODE_DESCRIPTION", "Fast and precise code search across ALL GitHub repositories using GitHub's native search engine. Best for finding exact symbols, functions, classes, or specific code patterns."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_CODE_USER_TITLE", "Search code"),
				ReadOnlyHint: true,
			},
			InputSchema: SearchCodeInput{}.MCPSchema(),
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, input SearchCodeInput) (*mcp.CallToolResult, any, error) {
			opts := &github.SearchOptions{
				Sort:  input.Sort,
				Order: input.Order,
				ListOptions: github.ListOptions{
					PerPage: input.PerPage,
					Page:    input.Page,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			result, resp, err := client.Search.Code(ctx, input.Query, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to search code with query '%s'", input.Query),
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
				return utils.NewToolResultError(fmt.Sprintf("failed to search code: %s", string(body))), nil, nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		}
}

// userOrOrgSearchInput is a common interface for user/org search inputs
type userOrOrgSearchInput interface {
	GetQuery() string
	GetSort() string
	GetOrder() string
	GetPage() int
	GetPerPage() int
}

// Add getters to SearchUsersInput
func (s SearchUsersInput) GetQuery() string { return s.Query }
func (s SearchUsersInput) GetSort() string  { return s.Sort }
func (s SearchUsersInput) GetOrder() string { return s.Order }
func (s SearchUsersInput) GetPage() int     { return s.Page }
func (s SearchUsersInput) GetPerPage() int  { return s.PerPage }

// Add getters to SearchOrgsInput
func (s SearchOrgsInput) GetQuery() string { return s.Query }
func (s SearchOrgsInput) GetSort() string  { return s.Sort }
func (s SearchOrgsInput) GetOrder() string { return s.Order }
func (s SearchOrgsInput) GetPage() int     { return s.Page }
func (s SearchOrgsInput) GetPerPage() int  { return s.PerPage }

func userOrOrgHandlerTyped[T userOrOrgSearchInput](accountType string, getClient GetClientFn) mcp.ToolHandlerFor[T, any] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input T) (*mcp.CallToolResult, any, error) {
		opts := &github.SearchOptions{
			Sort:  input.GetSort(),
			Order: input.GetOrder(),
			ListOptions: github.ListOptions{
				PerPage: input.GetPerPage(),
				Page:    input.GetPage(),
			},
		}

		client, err := getClient(ctx)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
		}

		searchQuery := input.GetQuery()
		if !hasTypeFilter(searchQuery) {
			searchQuery = "type:" + accountType + " " + searchQuery
		}
		result, resp, err := client.Search.Users(ctx, searchQuery, opts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx,
				fmt.Sprintf("failed to search %ss with query '%s'", accountType, input.GetQuery()),
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
			return utils.NewToolResultError(fmt.Sprintf("failed to search %ss: %s", accountType, string(body))), nil, nil
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
}

// SearchUsers creates a tool to search for GitHub users.
func SearchUsers(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[SearchUsersInput, any]) {
	return mcp.Tool{
		Name:        "search_users",
		Description: t("TOOL_SEARCH_USERS_DESCRIPTION", "Find GitHub users by username, real name, or other profile information. Useful for locating developers, contributors, or team members."),
		Annotations: &mcp.ToolAnnotations{
			Title:        t("TOOL_SEARCH_USERS_USER_TITLE", "Search users"),
			ReadOnlyHint: true,
		},
		InputSchema: SearchUsersInput{}.MCPSchema(),
	}, userOrOrgHandlerTyped[SearchUsersInput]("user", getClient)
}

// SearchOrgs creates a tool to search for GitHub organizations.
func SearchOrgs(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, mcp.ToolHandlerFor[SearchOrgsInput, any]) {
	return mcp.Tool{
		Name:        "search_orgs",
		Description: t("TOOL_SEARCH_ORGS_DESCRIPTION", "Find GitHub organizations by name, location, or other organization metadata. Ideal for discovering companies, open source foundations, or teams."),
		Annotations: &mcp.ToolAnnotations{
			Title:        t("TOOL_SEARCH_ORGS_USER_TITLE", "Search organizations"),
			ReadOnlyHint: true,
		},
		InputSchema: SearchOrgsInput{}.MCPSchema(),
	}, userOrOrgHandlerTyped[SearchOrgsInput]("org", getClient)
}
