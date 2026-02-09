package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// FeatureFlagCompareFileContents is the feature flag for the compare_file_contents tool.
const FeatureFlagCompareFileContents = "mcp_compare_file_contents"

// CompareFileContents creates a tool to compare two versions of a file in a GitHub repository.
// For supported formats (JSON, YAML, CSV, TOML), it produces semantic diffs showing
// only meaningful changes. For other formats, it falls back to unified diff.
func CompareFileContents(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name: "compare_file_contents",
			Description: t("TOOL_COMPARE_FILE_CONTENTS_DESCRIPTION", `Compare two versions of a file in a GitHub repository.
For structured formats (JSON, YAML, CSV, TOML), produces a semantic diff that shows only meaningful changes, ignoring formatting differences.
For other file types, produces a standard unified diff.
This is useful for understanding what actually changed between two versions of a file, especially for configuration files and data files where reformatting can obscure real changes.`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_COMPARE_FILE_CONTENTS_USER_TITLE", "Compare file contents between revisions"),
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
						Description: "Base ref to compare from (commit SHA, branch name, or tag name)",
					},
					"head": {
						Type:        "string",
						Description: "Head ref to compare to (commit SHA, branch name, or tag name)",
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			baseContent, baseErr := getFileAtRef(ctx, client, owner, repo, path, base)
			headContent, headErr := getFileAtRef(ctx, client, owner, repo, path, head)

			// If both sides fail, report the errors
			if baseErr != nil && headErr != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get file at both refs: base %q: %s, head %q: %s", base, baseErr, head, headErr)), nil, nil
			}

			// A nil content with no error won't happen from getFileAtRef,
			// but a non-nil error on one side means the file doesn't exist at that ref.
			// Pass nil to SemanticDiff to indicate added/deleted file.
			if baseErr != nil {
				baseContent = nil
			}
			if headErr != nil {
				headContent = nil
			}

			result := SemanticDiff(path, baseContent, headContent)

			output, err := json.Marshal(result)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal diff result: %w", err)
			}

			return utils.NewToolResultText(string(output)), nil, nil
		},
	)
	tool.FeatureFlagEnable = FeatureFlagCompareFileContents
	return tool
}

// getFileAtRef fetches file content from a GitHub repository at a specific ref.
func getFileAtRef(ctx context.Context, client *github.Client, owner, repo, path, ref string) ([]byte, error) {
	opts := &github.RepositoryContentGetOptions{Ref: ref}
	fileContent, _, resp, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("no response received")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if fileContent == nil {
		return nil, fmt.Errorf("path %q is a directory, not a file", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	if len(content) > MaxSemanticDiffFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of %d bytes", MaxSemanticDiffFileSize)
	}

	return []byte(content), nil
}
