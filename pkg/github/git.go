package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TreeEntryResponse represents a single entry in a Git tree.
type TreeEntryResponse struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size *int   `json:"size,omitempty"`
	Mode string `json:"mode"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// TreeResponse represents the response structure for a Git tree.
type TreeResponse struct {
	SHA       string              `json:"sha"`
	Truncated bool                `json:"truncated"`
	Tree      []TreeEntryResponse `json:"tree"`
	TreeSHA   string              `json:"tree_sha"`
	Owner     string              `json:"owner"`
	Repo      string              `json:"repo"`
	Recursive bool                `json:"recursive"`
	Count     int                 `json:"count"`
}

// GetRepositoryTree creates a tool to get the tree structure of a GitHub repository.
func GetRepositoryTree(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_repository_tree",
			mcp.WithDescription(t("TOOL_GET_REPOSITORY_TREE_DESCRIPTION", "Get the tree structure (files and directories) of a GitHub repository at a specific ref or SHA")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_REPOSITORY_TREE_USER_TITLE", "Get repository tree"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner (username or organization)"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("tree_sha",
				mcp.Description("The SHA1 value or ref (branch or tag) name of the tree. Defaults to the repository's default branch"),
			),
			mcp.WithBoolean("recursive",
				mcp.Description("Setting this parameter to true returns the objects or subtrees referenced by the tree. Default is false"),
				mcp.DefaultBool(false),
			),
			mcp.WithString("path_filter",
				mcp.Description("Optional path prefix to filter the tree results (e.g., 'src/' to only show files in the src directory)"),
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
			treeSHA, err := OptionalParam[string](request, "tree_sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			recursive, err := OptionalBoolParamWithDefault(request, "recursive", false)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pathFilter, err := OptionalParam[string](request, "path_filter")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError("failed to get GitHub client"), nil
			}

			// If no tree_sha is provided, use the repository's default branch
			if treeSHA == "" {
				repoInfo, repoResp, err := client.Repositories.Get(ctx, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get repository info",
						repoResp,
						err,
					), nil
				}
				treeSHA = *repoInfo.DefaultBranch
			}

			// Get the tree using the GitHub Git Tree API
			tree, resp, err := client.Git.GetTree(ctx, owner, repo, treeSHA, recursive)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get repository tree",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Filter tree entries if path_filter is provided
			var filteredEntries []*github.TreeEntry
			if pathFilter != "" {
				for _, entry := range tree.Entries {
					if strings.HasPrefix(entry.GetPath(), pathFilter) {
						filteredEntries = append(filteredEntries, entry)
					}
				}
			} else {
				filteredEntries = tree.Entries
			}

			treeEntries := make([]TreeEntryResponse, len(filteredEntries))
			for i, entry := range filteredEntries {
				treeEntries[i] = TreeEntryResponse{
					Path: entry.GetPath(),
					Type: entry.GetType(),
					Mode: entry.GetMode(),
					SHA:  entry.GetSHA(),
					URL:  entry.GetURL(),
				}
				if entry.Size != nil {
					treeEntries[i].Size = entry.Size
				}
			}

			response := TreeResponse{
				SHA:       *tree.SHA,
				Truncated: *tree.Truncated,
				Tree:      treeEntries,
				TreeSHA:   treeSHA,
				Owner:     owner,
				Repo:      repo,
				Recursive: recursive,
				Count:     len(filteredEntries),
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
