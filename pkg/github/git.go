package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
func GetRepositoryTree(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGit,
		mcp.Tool{
			Name:        "get_repository_tree",
			Description: t("TOOL_GET_REPOSITORY_TREE_DESCRIPTION", "Get the tree structure (files and directories) of a GitHub repository at a specific ref or SHA"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_REPOSITORY_TREE_USER_TITLE", "Get repository tree"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"tree_sha": {
						Type:        "string",
						Description: "The SHA1 value or ref (branch or tag) name of the tree. Defaults to the repository's default branch",
					},
					"recursive": {
						Type:        "boolean",
						Description: "Setting this parameter to true returns the objects or subtrees referenced by the tree. Default is false",
						Default:     json.RawMessage(`false`),
					},
					"path_filter": {
						Type:        "string",
						Description: "Optional path prefix to filter the tree results (e.g., 'src/' to only show files in the src directory)",
					},
				},
				Required: []string{"owner", "repo"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			treeSHA, err := OptionalParam[string](args, "tree_sha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			recursive, err := OptionalBoolParamWithDefault(args, "recursive", false)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pathFilter, err := OptionalParam[string](args, "path_filter")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError("failed to get GitHub client"), nil, nil
			}

			// If no tree_sha is provided, use the repository's default branch
			if treeSHA == "" {
				repoInfo, repoResp, err := client.Repositories.Get(ctx, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get repository info",
						repoResp,
						err,
					), nil, nil
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
				), nil, nil
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
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
