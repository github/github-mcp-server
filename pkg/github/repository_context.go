package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v87/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RepositoryRef identifies a GitHub repository by owner and name.
type RepositoryRef struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	FullName string `json:"full_name"`
}

// RepositoryAccessStatus describes whether the current token can access a repository.
type RepositoryAccessStatus struct {
	Accessible  bool                    `json:"accessible"`
	Private     bool                    `json:"private,omitempty"`
	Permissions *RepositoryPermissions  `json:"permissions,omitempty"`
	Error       string                  `json:"error,omitempty"`
	Hint        string                  `json:"hint,omitempty"`
}

// RepositoryPermissions summarizes repository permissions returned by the GitHub API.
type RepositoryPermissions struct {
	Admin bool `json:"admin"`
	Push  bool `json:"push"`
	Pull  bool `json:"pull"`
}

// RepositoryContextResponse is returned by the get_repository_context tool.
type RepositoryContextResponse struct {
	DefaultRepository *RepositoryRef          `json:"default_repository,omitempty"`
	FocusMode         bool                    `json:"focus_mode"`
	TokenType         string                  `json:"token_type,omitempty"`
	RepositoryAccess  *RepositoryAccessStatus `json:"repository_access,omitempty"`
}

// repositoryDiscoveryTools are open-world tools hidden in repository focus mode.
var repositoryDiscoveryTools = map[string]struct{}{
	"search_repositories":        {},
	"search_users":               {},
	"search_orgs":                {},
	"list_starred_repositories":  {},
	"create_repository":          {},
	"fork_repository":            {},
}

// ParseRepositoryRef parses owner/repo from common Git remote and URL formats.
func ParseRepositoryRef(raw string) (RepositoryRef, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return RepositoryRef{}, fmt.Errorf("repository reference is empty")
	}

	if strings.Contains(trimmed, "://") || strings.HasPrefix(trimmed, "git@") {
		return parseRepositoryRemote(trimmed)
	}

	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return RepositoryRef{}, fmt.Errorf("repository must be in owner/repo format, got %q", raw)
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")
	if owner == "" || repo == "" {
		return RepositoryRef{}, fmt.Errorf("repository must be in owner/repo format, got %q", raw)
	}

	return RepositoryRef{
		Owner:    owner,
		Repo:     repo,
		FullName: owner + "/" + repo,
	}, nil
}

func parseRepositoryRemote(raw string) (RepositoryRef, error) {
	normalized := raw
	if strings.HasPrefix(normalized, "git@") {
		normalized = strings.Replace(normalized, ":", "/", 1)
		normalized = strings.TrimPrefix(normalized, "git@")
		normalized = "https://" + normalized
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return RepositoryRef{}, fmt.Errorf("invalid repository URL: %w", err)
	}

	path := strings.Trim(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return RepositoryRef{}, fmt.Errorf("repository URL must include owner and repo, got %q", raw)
	}

	owner := parts[len(parts)-2]
	repo := parts[len(parts)-1]
	if owner == "" || repo == "" {
		return RepositoryRef{}, fmt.Errorf("repository URL must include owner and repo, got %q", raw)
	}

	return RepositoryRef{
		Owner:    owner,
		Repo:     repo,
		FullName: owner + "/" + repo,
	}, nil
}

// VerifyRepositoryAccess checks whether the token can access the given repository.
func VerifyRepositoryAccess(ctx context.Context, client *github.Client, ref RepositoryRef, tokenType utils.TokenType) RepositoryAccessStatus {
	repository, resp, err := client.Repositories.Get(ctx, ref.Owner, ref.Repo)
	if err != nil {
		status := RepositoryAccessStatus{
			Accessible: false,
			Error:      err.Error(),
		}
		if tokenType == utils.TokenTypeFineGrainedPersonalAccessToken {
			status.Hint = fmt.Sprintf(
				"Fine-grained personal access tokens must explicitly include repository %s with Metadata plus the permissions you need (for example Issues and Pull requests). Collaborator access alone is not enough unless the token is authorized for this repository.",
				ref.FullName,
			)
		}
		if resp != nil && resp.StatusCode == 404 {
			status.Hint = strings.TrimSpace(status.Hint + " The repository was not found or the token cannot access it.")
		}
		return status
	}

	status := RepositoryAccessStatus{
		Accessible: true,
		Private:  repository.GetPrivate(),
	}
	if repository.Permissions != nil {
		status.Permissions = &RepositoryPermissions{
			Admin: repository.Permissions.GetAdmin(),
			Push:  repository.Permissions.GetPush(),
			Pull:  repository.Permissions.GetPull(),
		}
	}
	return status
}

func tokenTypeName(tokenType utils.TokenType) string {
	switch tokenType {
	case utils.TokenTypePersonalAccessToken:
		return "classic_pat"
	case utils.TokenTypeFineGrainedPersonalAccessToken:
		return "fine_grained_pat"
	case utils.TokenTypeOAuthAccessToken:
		return "oauth"
	case utils.TokenTypeUserToServerGitHubAppToken:
		return "github_app_user"
	case utils.TokenTypeServerToServerGitHubAppToken:
		return "github_app_installation"
	default:
		return "unknown"
	}
}

func DetectTokenType(token string) utils.TokenType {
	for prefix, tokenType := range map[string]utils.TokenType{
		"ghp_":        utils.TokenTypePersonalAccessToken,
		"github_pat_": utils.TokenTypeFineGrainedPersonalAccessToken,
		"gho_":        utils.TokenTypeOAuthAccessToken,
		"ghu_":        utils.TokenTypeUserToServerGitHubAppToken,
		"ghs_":        utils.TokenTypeServerToServerGitHubAppToken,
	} {
		if strings.HasPrefix(token, prefix) {
			return tokenType
		}
	}
	return utils.TokenTypeUnknown
}

// ResolveRepositoryFocusConfig parses the configured repository and determines focus mode.
func ResolveRepositoryFocusConfig(defaultRepository string, allowDiscoveryTools bool) (RepositoryRef, bool, error) {
	if strings.TrimSpace(defaultRepository) == "" {
		return RepositoryRef{}, false, nil
	}

	ref, err := ParseRepositoryRef(defaultRepository)
	if err != nil {
		return RepositoryRef{}, false, err
	}

	return ref, !allowDiscoveryTools, nil
}

// BuildRepositoryContextConfig constructs runtime repository context from server config.
func BuildRepositoryContextConfig(defaultRepository, token string, allowDiscoveryTools bool) (RepositoryContextConfig, error) {
	ref, focusMode, err := ResolveRepositoryFocusConfig(defaultRepository, allowDiscoveryTools)
	if err != nil {
		return RepositoryContextConfig{}, err
	}

	cfg := RepositoryContextConfig{
		FocusMode: focusMode,
		Token:     token,
	}
	if ref.FullName != "" {
		cfg.DefaultRepository = &ref
	}
	return cfg, nil
}

func CreateRepositoryFocusFilter(enabled bool) inventory.ToolFilter {
	if !enabled {
		return func(_ context.Context, _ *inventory.ServerTool) (bool, error) {
			return true, nil
		}
	}

	return func(_ context.Context, tool *inventory.ServerTool) (bool, error) {
		_, excluded := repositoryDiscoveryTools[tool.Tool.Name]
		return !excluded, nil
	}
}

// GetRepositoryContext creates a tool that returns the configured default repository and access status.
func GetRepositoryContext(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataContext,
		mcp.Tool{
			Name: "get_repository_context",
			Description: t(
				"TOOL_GET_REPOSITORY_CONTEXT_DESCRIPTION",
				"Get the configured default repository and token access status. Call this first for project-focused work instead of searching repositories. Returns owner/repo to use with repo-scoped tools like list_issues and list_pull_requests.",
			),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_REPOSITORY_CONTEXT_USER_TITLE", "Get repository context"),
				ReadOnlyHint: true,
			},
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		nil,
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) {
			repoCtx := deps.GetRepositoryContext()
			response := RepositoryContextResponse{
				FocusMode: repoCtx.FocusMode,
			}

			if repoCtx.Token != "" {
				response.TokenType = tokenTypeName(DetectTokenType(repoCtx.Token))
			}

			if repoCtx.DefaultRepository != nil {
				response.DefaultRepository = repoCtx.DefaultRepository

				client, err := deps.GetClient(ctx)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
				}

				access := VerifyRepositoryAccess(ctx, client, *repoCtx.DefaultRepository, DetectTokenType(repoCtx.Token))
				response.RepositoryAccess = &access
			}

			return MarshalledTextResult(response), response, nil
		},
	)
}
