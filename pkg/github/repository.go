package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RepositoryDetails is the response type for GetRepository.
// It is a superset of MinimalRepository, adding fields relevant to
// a single-repo lookup (visibility, URLs, template flag, pushed_at).
type RepositoryDetails struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	FullName      string   `json:"full_name"`
	Description   string   `json:"description,omitempty"`
	Visibility    string   `json:"visibility"`
	DefaultBranch string   `json:"default_branch,omitempty"`
	IsFork        bool     `json:"is_fork"`
	IsArchived    bool     `json:"is_archived"`
	IsTemplate    bool     `json:"is_template"`
	Stars         int      `json:"stargazers_count"`
	Forks         int      `json:"forks_count"`
	OpenIssues    int      `json:"open_issues_count"`
	Topics        []string `json:"topics,omitempty"`
	CloneURL      string   `json:"clone_url,omitempty"`
	SSHURL        string   `json:"ssh_url,omitempty"`
	HTMLURL       string   `json:"html_url,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
	PushedAt      string   `json:"pushed_at,omitempty"`
}

func convertToRepositoryDetails(r *github.Repository) RepositoryDetails {
	d := RepositoryDetails{
		ID:         r.GetID(),
		Name:       r.GetName(),
		FullName:   r.GetFullName(),
		IsFork:     r.GetFork(),
		IsArchived: r.GetArchived(),
		IsTemplate: r.GetIsTemplate(),
		Stars:      r.GetStargazersCount(),
		Forks:      r.GetForksCount(),
		OpenIssues: r.GetOpenIssuesCount(),
		Topics:     r.Topics,
		CloneURL:   r.GetCloneURL(),
		SSHURL:     r.GetSSHURL(),
		HTMLURL:    r.GetHTMLURL(),
	}
	if r.Description != nil {
		d.Description = r.GetDescription()
	}
	if r.DefaultBranch != nil {
		d.DefaultBranch = r.GetDefaultBranch()
	}
	if r.Visibility != nil {
		d.Visibility = r.GetVisibility()
	}
	if r.CreatedAt != nil {
		d.CreatedAt = r.CreatedAt.String()
	}
	if r.UpdatedAt != nil {
		d.UpdatedAt = r.UpdatedAt.String()
	}
	if r.PushedAt != nil {
		d.PushedAt = r.PushedAt.String()
	}
	return d
}

// GetRepository returns a tool that fetches metadata for a single GitHub repository.
func GetRepository(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "get_repository",
			Description: t("TOOL_GET_REPOSITORY_DESCRIPTION", "Get metadata for a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_REPOSITORY_USER_TITLE", "Get repository"),
				ReadOnlyHint: true,
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
					"url": {
						Type:        "string",
						Description: "GitHub repository URL (e.g. https://github.com/owner/repo). If provided, owner and repo are parsed from the URL.",
					},
				},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			if err := ApplyURLParam(args); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			repository, resp, err := client.Repositories.Get(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get repository",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get repository", resp, body), nil, nil
			}

			r, err := json.Marshal(convertToRepositoryDetails(repository))
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
