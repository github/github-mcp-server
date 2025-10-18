package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// CreateRelease creates a tool to create a new release in a GitHub repository.
func CreateRelease(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_release",
			mcp.WithDescription(t("TOOL_CREATE_RELEASE_DESCRIPTION", "Create a new release in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_RELEASE_USER_TITLE", "Create release"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("tag_name",
				mcp.Required(),
				mcp.Description("The name of the tag for this release"),
			),
			mcp.WithString("target_commitish",
				mcp.Description("The commitish value for the tag (branch or commit SHA). Defaults to the repository's default branch"),
			),
			mcp.WithString("name",
				mcp.Description("The name of the release"),
			),
			mcp.WithString("body",
				mcp.Description("Text describing the contents of the release"),
			),
			mcp.WithBoolean("draft",
				mcp.Description("Whether this is a draft (unpublished) release. Default: false"),
			),
			mcp.WithBoolean("prerelease",
				mcp.Description("Whether this is a pre-release. Default: false"),
			),
			mcp.WithBoolean("generate_release_notes",
				mcp.Description("Whether to automatically generate release notes from commits. Default: false"),
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
			tagName, err := RequiredParam[string](request, "tag_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Optional parameters
			targetCommitish, err := OptionalParam[string](request, "target_commitish")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := OptionalParam[string](request, "name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body, err := OptionalParam[string](request, "body")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			draft, err := OptionalParam[bool](request, "draft")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			prerelease, err := OptionalParam[bool](request, "prerelease")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			generateReleaseNotes, err := OptionalParam[bool](request, "generate_release_notes")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			releaseRequest := &github.RepositoryRelease{
				TagName:              github.Ptr(tagName),
				TargetCommitish:      github.Ptr(targetCommitish),
				Name:                 github.Ptr(name),
				Body:                 github.Ptr(body),
				Draft:                github.Ptr(draft),
				Prerelease:           github.Ptr(prerelease),
				GenerateReleaseNotes: github.Ptr(generateReleaseNotes),
			}

			release, resp, err := client.Repositories.CreateRelease(ctx, owner, repo, releaseRequest)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to create release with tag: %s", tagName),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()
			
			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to create release: %s", string(body))), nil
			}

			r, err := json.Marshal(release)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}