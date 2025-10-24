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

// ReleaseRead creates a tool for reading GitHub releases in a repository.
// Supports multiple methods: list, get_latest, and get_by_tag.
func ReleaseRead(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("release_read",
			mcp.WithDescription(t("TOOL_RELEASE_READ_DESCRIPTION", `Read operations for GitHub releases in a repository.

Available methods:
- list: List all releases in a repository.
- get_latest: Get the latest release in a repository.
- get_by_tag: Get a specific release by its tag name.
`)),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_RELEASE_READ_USER_TITLE", "Read operations for releases"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("method",
				mcp.Required(),
				mcp.Enum("list", "get_latest", "get_by_tag"),
				mcp.Description("The read operation to perform on releases."),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("tag",
				mcp.Description("Tag name (required for get_by_tag method)"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			method, err := RequiredParam[string](request, "method")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case "list":
				return ListReleasesMethod(ctx, client, owner, repo, request)
			case "get_latest":
				return GetLatestReleaseMethod(ctx, client, owner, repo)
			case "get_by_tag":
				return GetReleaseByTagMethod(ctx, client, owner, repo, request)
			default:
				return mcp.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil
			}
		}
}

// ListReleasesMethod handles the "list" method for ReleaseRead
func ListReleasesMethod(ctx context.Context, client *github.Client, owner, repo string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pagination, err := OptionalPaginationParams(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	releases, resp, err := client.Repositories.ListReleases(ctx, owner, repo, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to list releases",
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to list releases: %s", string(body))), nil
	}

	r, err := json.Marshal(releases)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// GetLatestReleaseMethod handles the "get_latest" method for ReleaseRead
func GetLatestReleaseMethod(ctx context.Context, client *github.Client, owner, repo string) (*mcp.CallToolResult, error) {
	release, resp, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get latest release",
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to get latest release: %s", string(body))), nil
	}

	r, err := json.Marshal(release)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// GetReleaseByTagMethod handles the "get_by_tag" method for ReleaseRead
func GetReleaseByTagMethod(ctx context.Context, client *github.Client, owner, repo string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tag, err := RequiredParam[string](request, "tag")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	release, resp, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			fmt.Sprintf("failed to get release by tag: %s", tag),
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to get release by tag: %s", string(body))), nil
	}

	r, err := json.Marshal(release)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}
