package github

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CompareFileContents creates a tool to compare a file between two git refs,
// producing semantic diffs for structured formats (JSON, YAML) and falling back
// to unified diffs for unsupported formats.
func CompareFileContents(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "compare_file_contents",
			Description: t("TOOL_COMPARE_FILE_CONTENTS_DESCRIPTION", "Compare a file between two git refs, with semantic diffs for structured formats (JSON, YAML)"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_COMPARE_FILE_CONTENTS_USER_TITLE", "Compare file contents"),
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
					"path": {
						Type:        "string",
						Description: "Path to the file to compare",
					},
					"base": {
						Type:        "string",
						Description: "Base git ref to compare from (commit SHA, branch name, or tag)",
					},
					"head": {
						Type:        "string",
						Description: "Head git ref to compare to (commit SHA, branch name, or tag)",
					},
				},
				Required: []string{"owner", "repo", "path", "base", "head"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			path, err := RequiredParam[string](args, "path")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			base, err := RequiredParam[string](args, "base")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			head, err := RequiredParam[string](args, "head")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			rawClient, err := deps.GetRawClient(ctx)
			if err != nil {
				return utils.NewToolResultError("failed to get raw content client"), nil, nil
			}

			baseContent, err := fetchFileContent(ctx, rawClient, owner, repo, path, base)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to fetch base file: %s", err)), nil, nil
			}

			headContent, err := fetchFileContent(ctx, rawClient, owner, repo, path, head)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to fetch head file: %s", err)), nil, nil
			}

			diff, format, isFallback, err := SemanticDiff(baseContent, headContent, path)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to compute diff: %s", err)), nil, nil
			}

			var header string
			if isFallback {
				header = fmt.Sprintf("Format: %s (unified diff — no semantic diff available for this format)", format)
			} else {
				header = fmt.Sprintf("Format: %s (semantic diff)", format)
			}

			result := fmt.Sprintf("%s\n\n%s", header, diff)

			return utils.NewToolResultText(result), nil, nil
		},
	)
	st.FeatureFlagEnable = "semantic_diff"
	return st
}

// fetchFileContent retrieves the raw content of a file at a given ref.
func fetchFileContent(ctx context.Context, client *raw.Client, owner, repo, path, ref string) ([]byte, error) {
	resp, err := client.GetRawContent(ctx, owner, repo, path, &raw.ContentOpts{Ref: ref})
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("file %q not found at ref %q", path, ref)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %q at ref %q", resp.StatusCode, path, ref)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}
