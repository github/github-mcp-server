package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxPatchFileBytes  = 4 * 1024 * 1024
	maxPatchInputBytes = 256 * 1024
	maxPatchEdits      = 100
)

type exactTextEdit struct {
	OldText             string
	NewText             string
	ExpectedOccurrences int
}

func applyExactTextEdits(content string, edits []exactTextEdit) (string, error) {
	if len(edits) == 0 {
		return "", fmt.Errorf("at least one edit is required")
	}
	if len(edits) > maxPatchEdits {
		return "", fmt.Errorf("too many edits: got %d, maximum is %d", len(edits), maxPatchEdits)
	}

	patchBytes := 0
	updated := content
	for i, edit := range edits {
		if edit.OldText == "" {
			return "", fmt.Errorf("edit %d old_text must not be empty", i)
		}
		if edit.ExpectedOccurrences < 1 {
			return "", fmt.Errorf("edit %d expected_occurrences must be at least 1", i)
		}
		patchBytes += len(edit.OldText) + len(edit.NewText)
		if patchBytes > maxPatchInputBytes {
			return "", fmt.Errorf("patch input exceeds %d bytes", maxPatchInputBytes)
		}

		actual := strings.Count(updated, edit.OldText)
		if actual != edit.ExpectedOccurrences {
			return "", fmt.Errorf("edit %d occurrence mismatch: expected %d exact matches, found %d", i, edit.ExpectedOccurrences, actual)
		}
		updated = strings.Replace(updated, edit.OldText, edit.NewText, edit.ExpectedOccurrences)
	}

	if updated == content {
		return "", fmt.Errorf("patch produced no content change")
	}
	return updated, nil
}

func parseExactTextEdits(args map[string]any) ([]exactTextEdit, error) {
	raw, ok := args["edits"]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: edits")
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("edits must be an array")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("edits must contain at least one item")
	}
	if len(items) > maxPatchEdits {
		return nil, fmt.Errorf("too many edits: got %d, maximum is %d", len(items), maxPatchEdits)
	}

	edits := make([]exactTextEdit, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("edit %d must be an object", i)
		}
		oldText, ok := obj["old_text"].(string)
		if !ok {
			return nil, fmt.Errorf("edit %d old_text must be a string", i)
		}
		newText, ok := obj["new_text"].(string)
		if !ok {
			return nil, fmt.Errorf("edit %d new_text must be a string", i)
		}
		expected := 1
		if value, exists := obj["expected_occurrences"]; exists {
			switch typed := value.(type) {
			case float64:
				if typed < 1 || typed != float64(int(typed)) {
					return nil, fmt.Errorf("edit %d expected_occurrences must be a positive integer", i)
				}
				expected = int(typed)
			case int:
				if typed < 1 {
					return nil, fmt.Errorf("edit %d expected_occurrences must be a positive integer", i)
				}
				expected = typed
			default:
				return nil, fmt.Errorf("edit %d expected_occurrences must be a positive integer", i)
			}
		}
		edits = append(edits, exactTextEdit{OldText: oldText, NewText: newText, ExpectedOccurrences: expected})
	}
	return edits, nil
}

// ApplyFilePatch creates a tool that applies exact text edits to an existing file without
// requiring the MCP client to transmit the complete replacement file.
func ApplyFilePatch(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "apply_file_patch",
			Description: t("TOOL_APPLY_FILE_PATCH_DESCRIPTION", `Apply bounded exact text edits to one existing UTF-8 file in a GitHub repository. The server fetches the current blob directly, so the caller does not need to resend the complete file. The update is bound to both the expected branch HEAD and expected file blob SHA. Each old_text must match exactly the declared number of occurrences; fuzzy patching is never performed. The branch ref is updated without force, so concurrent branch movement fails closed.`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_APPLY_FILE_PATCH_USER_TITLE", "Apply file patch"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":             {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":              {Type: "string", Description: "Repository name"},
					"path":              {Type: "string", Description: "Repository-relative path of the existing UTF-8 file to patch"},
					"branch":            {Type: "string", Description: "Branch to update"},
					"expected_head_sha": {Type: "string", Description: "Exact commit SHA the branch must currently point to"},
					"expected_blob_sha": {Type: "string", Description: "Exact blob SHA of the file being patched"},
					"message":           {Type: "string", Description: "Commit message"},
					"edits": {
						Type:        "array",
						Description: "Ordered exact text replacements. Each old_text must match exactly expected_occurrences times at that step (default 1).",
						MinItems:    jsonschema.Ptr(1),
						MaxItems:    jsonschema.Ptr(maxPatchEdits),
						Items: &jsonschema.Schema{
							Type: "object",
							Properties: map[string]*jsonschema.Schema{
								"old_text":             {Type: "string", Description: "Exact text that must already exist; must not be empty"},
								"new_text":             {Type: "string", Description: "Replacement text"},
								"expected_occurrences": {Type: "number", Description: "Exact positive integer number of non-overlapping matches required before replacement; defaults to 1", Minimum: jsonschema.Ptr(1.0), Default: json.RawMessage("1")},
							},
							Required: []string{"old_text", "new_text"},
						},
					},
				},
				Required: []string{"owner", "repo", "path", "branch", "expected_head_sha", "expected_blob_sha", "message", "edits"},
			},
		},
		scopes.RequireAll(scopes.Repo),
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
			path, err = validateRelativePath(path)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("invalid path: %s", err)), nil, nil
			}
			branch, err := RequiredParam[string](args, "branch")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			expectedHead, err := RequiredParam[string](args, "expected_head_sha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			expectedBlob, err := RequiredParam[string](args, "expected_blob_sha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			message, err := RequiredParam[string](args, "message")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			edits, err := parseExactTextEdits(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if !looksLikeSHA(expectedHead) || !looksLikeSHA(expectedBlob) {
				return utils.NewToolResultError("expected_head_sha and expected_blob_sha must be 40-character Git SHA-1 values"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get branch reference", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			currentHead := ref.GetObject().GetSHA()
			if !strings.EqualFold(currentHead, expectedHead) {
				return utils.NewToolResultError(fmt.Sprintf("branch HEAD mismatch: expected %s, current %s", expectedHead, currentHead)), nil, nil
			}

			baseCommit, resp, err := client.Git.GetCommit(ctx, owner, repo, expectedHead)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get expected base commit", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			if baseCommit.Tree == nil || baseCommit.Tree.SHA == nil {
				return utils.NewToolResultError("expected base commit has no tree SHA"), nil, nil
			}

			entry, resp, err := getTreeEntry(ctx, client, owner, repo, baseCommit.Tree.GetSHA(), path)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to resolve file in expected tree", resp, err), nil, nil
			}
			if entry == nil || entry.GetType() != "blob" {
				return utils.NewToolResultError(fmt.Sprintf("path %s is not an existing file blob", path)), nil, nil
			}
			if entry.GetMode() == gitSymlinkMode {
				return newSymlinkWriteBlockedResult(path, ""), nil, nil
			}
			if entry.GetMode() != "100644" && entry.GetMode() != "100755" {
				return utils.NewToolResultError(fmt.Sprintf("unsupported file mode %s at %s", entry.GetMode(), path)), nil, nil
			}
			if !strings.EqualFold(entry.GetSHA(), expectedBlob) {
				return utils.NewToolResultError(fmt.Sprintf("blob SHA mismatch: expected %s, current %s", expectedBlob, entry.GetSHA())), nil, nil
			}

			original, resp, err := getVerifiedBlob(ctx, client, owner, repo, entry.GetSHA())
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to read expected file blob", resp, err), nil, nil
			}
			if len(original) > maxPatchFileBytes {
				return utils.NewToolResultError(fmt.Sprintf("file is %d bytes; maximum patchable size is %d bytes", len(original), maxPatchFileBytes)), nil, nil
			}
			if !utf8.Valid(original) {
				return utils.NewToolResultError("file is not valid UTF-8 text"), nil, nil
			}

			patched, err := applyExactTextEdits(string(original), edits)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if len(patched) > maxPatchFileBytes {
				return utils.NewToolResultError(fmt.Sprintf("patched file would be %d bytes; maximum is %d bytes", len(patched), maxPatchFileBytes)), nil, nil
			}

			blob, resp, err := client.Git.CreateBlob(ctx, owner, repo, github.Blob{Content: github.Ptr(patched), Encoding: github.Ptr("utf-8")})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create patched blob", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}

			newTree, resp, err := client.Git.CreateTree(ctx, owner, repo, baseCommit.Tree.GetSHA(), []*github.TreeEntry{{Path: github.Ptr(path), Mode: github.Ptr(entry.GetMode()), Type: github.Ptr("blob"), SHA: blob.SHA}})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create patched tree", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}

			newCommit, resp, err := client.Git.CreateCommit(ctx, owner, repo, github.Commit{Message: github.Ptr(message), Tree: newTree, Parents: []*github.Commit{{SHA: github.Ptr(expectedHead)}}}, nil)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create patch commit", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}

			_, resp, err = client.Git.UpdateRef(ctx, owner, repo, ref.GetRef(), github.UpdateRef{SHA: newCommit.GetSHA(), Force: github.Ptr(false)})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update branch reference; the branch may have moved", resp, err), nil, nil
			}
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}

			result := map[string]any{
				"owner": owner, "repo": repo, "branch": branch, "path": path,
				"previous_head_sha": expectedHead, "commit_sha": newCommit.GetSHA(), "tree_sha": newTree.GetSHA(),
				"previous_blob_sha": expectedBlob, "blob_sha": blob.GetSHA(),
				"before_size": len(original), "after_size": len(patched), "edits_applied": len(edits),
			}
			return MarshalledTextResult(result), nil, nil
		},
	)
	tool.ScopeAccess = scopes.DynamicChallenge(
		[]scopes.Scope{scopes.Repo, scopes.Workflow},
		tool.ScopeAccess.Visible,
		workflowScopeChallengeForPath,
	)
	return tool
}
