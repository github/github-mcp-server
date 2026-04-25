package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// validTagName checks whether a tag name is a plausible Git tag.
// Tags must be non-empty and must not contain spaces or control characters.
var validTagRe = regexp.MustCompile(`^[^\s~^:?*\[\\]+$`)

func isValidTagName(tag string) bool {
	return tag != "" && validTagRe.MatchString(tag)
}

// CreateRelease creates a tool to create a new release in a GitHub repository.
func CreateRelease(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "create_release",
			Description: t("TOOL_CREATE_RELEASE_DESCRIPTION", "Create a new release in a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_RELEASE_USER_TITLE", "Create release"),
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
					"tag_name": {
						Type:        "string",
						Description: "The name of the tag for the release",
					},
					"name": {
						Type:        "string",
						Description: "The name of the release",
					},
					"body": {
						Type:        "string",
						Description: "Text describing the contents of the release (supports Markdown)",
					},
					"draft": {
						Type:        "boolean",
						Description: "Whether to create a draft (unpublished) release",
					},
					"prerelease": {
						Type:        "boolean",
						Description: "Whether to identify the release as a prerelease",
					},
					"target_commitish": {
						Type:        "string",
						Description: "The commitish value that determines where the Git tag is created from (defaults to the default branch)",
					},
					"generate_release_notes": {
						Type:        "boolean",
						Description: "Whether to automatically generate release notes",
					},
				},
				Required: []string{"owner", "repo", "tag_name"},
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
			tagName, err := RequiredParam[string](args, "tag_name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if !isValidTagName(tagName) {
				return utils.NewToolResultError(fmt.Sprintf("invalid tag name %q: must not contain spaces or special ref characters", tagName)), nil, nil
			}

			name, err := OptionalParam[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			body, err := OptionalParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			draft, err := OptionalParam[bool](args, "draft")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			prerelease, err := OptionalParam[bool](args, "prerelease")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			targetCommitish, err := OptionalParam[string](args, "target_commitish")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			generateNotes, err := OptionalParam[bool](args, "generate_release_notes")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			releaseReq := &github.RepositoryRelease{
				TagName:              github.Ptr(tagName),
				Name:                 github.Ptr(name),
				Body:                 github.Ptr(body),
				Draft:                github.Ptr(draft),
				Prerelease:           github.Ptr(prerelease),
				GenerateReleaseNotes: github.Ptr(generateNotes),
			}
			if targetCommitish != "" {
				releaseReq.TargetCommitish = github.Ptr(targetCommitish)
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			release, resp, err := client.Repositories.CreateRelease(ctx, owner, repo, releaseReq)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create release", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				respBody, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create release", resp, respBody), nil, nil
			}

			result := convertToMinimalRelease(release)
			r, err := json.Marshal(result)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// UpdateRelease creates a tool to update an existing release in a GitHub repository.
func UpdateRelease(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "update_release",
			Description: t("TOOL_UPDATE_RELEASE_DESCRIPTION", "Update an existing release in a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_RELEASE_USER_TITLE", "Update release"),
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
					"release_id": {
						Type:        "number",
						Description: "The unique identifier of the release",
					},
					"tag_name": {
						Type:        "string",
						Description: "The name of the tag for the release",
					},
					"name": {
						Type:        "string",
						Description: "The name of the release",
					},
					"body": {
						Type:        "string",
						Description: "Text describing the contents of the release (supports Markdown)",
					},
					"draft": {
						Type:        "boolean",
						Description: "Whether to mark the release as a draft",
					},
					"prerelease": {
						Type:        "boolean",
						Description: "Whether to mark the release as a prerelease",
					},
				},
				Required: []string{"owner", "repo", "release_id"},
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
			releaseIDFloat, err := RequiredParam[float64](args, "release_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			releaseID := int64(releaseIDFloat)

			releaseReq := &github.RepositoryRelease{}

			tagName, err := OptionalParam[string](args, "tag_name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if tagName != "" {
				if !isValidTagName(tagName) {
					return utils.NewToolResultError(fmt.Sprintf("invalid tag name %q: must not contain spaces or special ref characters", tagName)), nil, nil
				}
				releaseReq.TagName = github.Ptr(tagName)
			}

			name, err := OptionalParam[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if name != "" {
				releaseReq.Name = github.Ptr(name)
			}

			body, err := OptionalParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if body != "" {
				releaseReq.Body = github.Ptr(body)
			}

			// For boolean fields, use OptionalParamOK to distinguish between
			// "not provided" and "explicitly set to false".
			if draftVal, ok, err := OptionalParamOK[bool](args, "draft"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				releaseReq.Draft = github.Ptr(draftVal)
			}

			if prereleaseVal, ok, err := OptionalParamOK[bool](args, "prerelease"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				releaseReq.Prerelease = github.Ptr(prereleaseVal)
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			release, resp, err := client.Repositories.EditRelease(ctx, owner, repo, releaseID, releaseReq)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update release", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				respBody, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to update release", resp, respBody), nil, nil
			}

			result := convertToMinimalRelease(release)
			r, err := json.Marshal(result)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// DeleteRelease creates a tool to delete a release in a GitHub repository.
func DeleteRelease(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "delete_release",
			Description: t("TOOL_DELETE_RELEASE_DESCRIPTION", "Delete a release from a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_DELETE_RELEASE_USER_TITLE", "Delete release"),
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
					"release_id": {
						Type:        "number",
						Description: "The unique identifier of the release",
					},
				},
				Required: []string{"owner", "repo", "release_id"},
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
			releaseIDFloat, err := RequiredParam[float64](args, "release_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			releaseID := int64(releaseIDFloat)

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Repositories.DeleteRelease(ctx, owner, repo, releaseID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete release", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusNoContent {
				respBody, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to delete release", resp, respBody), nil, nil
			}

			result := map[string]any{
				"message":    "Release deleted successfully",
				"release_id": releaseID,
			}
			r, err := json.Marshal(result)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GetReleaseByID creates a tool to get a specific release by its ID.
func GetReleaseByID(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "get_release",
			Description: t("TOOL_GET_RELEASE_DESCRIPTION", "Get details of a release by its ID"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_RELEASE_USER_TITLE", "Get release"),
				ReadOnlyHint: true,
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
					"release_id": {
						Type:        "number",
						Description: "The unique identifier of the release",
					},
				},
				Required: []string{"owner", "repo", "release_id"},
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
			releaseIDFloat, err := RequiredParam[float64](args, "release_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			releaseID := int64(releaseIDFloat)

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			release, resp, err := client.Repositories.GetRelease(ctx, owner, repo, releaseID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get release", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				respBody, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get release", resp, respBody), nil, nil
			}

			result := convertToMinimalRelease(release)
			r, err := json.Marshal(result)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// ListReleaseAssets creates a tool to list the assets for a specific release.
func ListReleaseAssets(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "list_release_assets",
			Description: t("TOOL_LIST_RELEASE_ASSETS_DESCRIPTION", "List assets for a release in a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_RELEASE_ASSETS_USER_TITLE", "List release assets"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
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
					"release_id": {
						Type:        "number",
						Description: "The unique identifier of the release",
					},
				},
				Required: []string{"owner", "repo", "release_id"},
			}),
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
			releaseIDFloat, err := RequiredParam[float64](args, "release_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			releaseID := int64(releaseIDFloat)

			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			opts := &github.ListOptions{
				Page:    pagination.Page,
				PerPage: pagination.PerPage,
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			assets, resp, err := client.Repositories.ListReleaseAssets(ctx, owner, repo, releaseID, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list release assets", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				respBody, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list release assets", resp, respBody), nil, nil
			}

			type MinimalAsset struct {
				ID            int64  `json:"id"`
				Name          string `json:"name"`
				ContentType   string `json:"content_type"`
				Size          int    `json:"size"`
				DownloadCount int    `json:"download_count"`
				DownloadURL   string `json:"browser_download_url"`
			}

			minimalAssets := make([]MinimalAsset, 0, len(assets))
			for _, asset := range assets {
				if asset != nil {
					minimalAssets = append(minimalAssets, MinimalAsset{
						ID:            asset.GetID(),
						Name:          asset.GetName(),
						ContentType:   asset.GetContentType(),
						Size:          asset.GetSize(),
						DownloadCount: asset.GetDownloadCount(),
						DownloadURL:   asset.GetBrowserDownloadURL(),
					})
				}
			}

			r, err := json.Marshal(minimalAssets)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// DeleteReleaseAsset creates a tool to delete a release asset.
func DeleteReleaseAsset(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "delete_release_asset",
			Description: t("TOOL_DELETE_RELEASE_ASSET_DESCRIPTION", "Delete an asset from a release"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_DELETE_RELEASE_ASSET_USER_TITLE", "Delete release asset"),
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
					"asset_id": {
						Type:        "number",
						Description: "The unique identifier of the release asset",
					},
				},
				Required: []string{"owner", "repo", "asset_id"},
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
			assetIDFloat, err := RequiredParam[float64](args, "asset_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			assetID := int64(assetIDFloat)

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Repositories.DeleteReleaseAsset(ctx, owner, repo, assetID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to delete release asset", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusNoContent {
				respBody, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to delete release asset", resp, respBody), nil, nil
			}

			result := map[string]any{
				"message":  "Release asset deleted successfully",
				"asset_id": assetID,
			}
			r, err := json.Marshal(result)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
