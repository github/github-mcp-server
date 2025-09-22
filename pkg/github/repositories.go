package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func GetCommit(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_commit",
			mcp.WithDescription(t("TOOL_GET_COMMITS_DESCRIPTION", "Get details for a commit from a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_COMMITS_USER_TITLE", "Get commit details"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("sha",
				mcp.Required(),
				mcp.Description("Commit SHA, branch name, or tag name"),
			),
			mcp.WithBoolean("include_diff",
				mcp.Description("Whether to include file diffs and stats in the response. Default is true."),
				mcp.DefaultBool(true),
			),
			WithPagination(),
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
			sha, err := RequiredParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			includeDiff, err := OptionalBoolParamWithDefault(request, "include_diff", true)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ListOptions{
				Page:    pagination.Page,
				PerPage: pagination.PerPage,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			commit, resp, err := client.Repositories.GetCommit(ctx, owner, repo, sha, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get commit: %s", sha),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get commit: %s", string(body))), nil
			}

			// Convert to minimal commit
			minimalCommit := convertToMinimalCommit(commit, includeDiff)

			r, err := json.Marshal(minimalCommit)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListCommits creates a tool to get commits of a branch in a repository.
func ListCommits(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_commits",
			mcp.WithDescription(t("TOOL_LIST_COMMITS_DESCRIPTION", "Get list of commits of a branch in a GitHub repository. Returns at least 30 results per page by default, but can return more if specified using the perPage parameter (up to 100).")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_COMMITS_USER_TITLE", "List commits"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("sha",
				mcp.Description("Commit SHA, branch or tag name to list commits of. If not provided, uses the default branch of the repository. If a commit SHA is provided, will list commits up to that SHA."),
			),
			mcp.WithString("author",
				mcp.Description("Author username or email address to filter commits by"),
			),
			WithPagination(),
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
			sha, err := OptionalParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			author, err := OptionalParam[string](request, "author")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			// Set default perPage to 30 if not provided
			perPage := pagination.PerPage
			if perPage == 0 {
				perPage = 30
			}
			opts := &github.CommitsListOptions{
				SHA:    sha,
				Author: author,
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: perPage,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			commits, resp, err := client.Repositories.ListCommits(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list commits: %s", sha),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list commits: %s", string(body))), nil
			}

			// Convert to minimal commits
			minimalCommits := make([]MinimalCommit, len(commits))
			for i, commit := range commits {
				minimalCommits[i] = convertToMinimalCommit(commit, false)
			}

			r, err := json.Marshal(minimalCommits)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListBranches creates a tool to list branches in a GitHub repository.
func ListBranches(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_branches",
			mcp.WithDescription(t("TOOL_LIST_BRANCHES_DESCRIPTION", "List branches in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_BRANCHES_USER_TITLE", "List branches"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			WithPagination(),
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
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.BranchListOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			branches, resp, err := client.Repositories.ListBranches(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list branches",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to list branches: %s", string(body))), nil
			}

			// Convert to minimal branches
			minimalBranches := make([]MinimalBranch, 0, len(branches))
			for _, branch := range branches {
				minimalBranches = append(minimalBranches, convertToMinimalBranch(branch))
			}

			r, err := json.Marshal(minimalBranches)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// CreateOrUpdateFile creates a tool to create or update a file in a GitHub repository.
func CreateOrUpdateFile(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_or_update_file",
			mcp.WithDescription(t("TOOL_CREATE_OR_UPDATE_FILE_DESCRIPTION", "Create or update a single file in a GitHub repository. If updating, you must provide the SHA of the file you want to update. Use this tool to create or update a file in a GitHub repository remotely; do not use it for local file operations.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_OR_UPDATE_FILE_USER_TITLE", "Create or update file"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner (username or organization)"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Path where to create/update the file"),
			),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("Content of the file"),
			),
			mcp.WithString("message",
				mcp.Required(),
				mcp.Description("Commit message"),
			),
			mcp.WithString("branch",
				mcp.Required(),
				mcp.Description("Branch to create/update the file in"),
			),
			mcp.WithString("sha",
				mcp.Description("Required if updating an existing file. The blob SHA of the file being replaced."),
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
			path, err := RequiredParam[string](request, "path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			content, err := RequiredParam[string](request, "content")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			message, err := RequiredParam[string](request, "message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			branch, err := RequiredParam[string](request, "branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// json.Marshal encodes byte arrays with base64, which is required for the API.
			contentBytes := []byte(content)

			// Create the file options
			opts := &github.RepositoryContentFileOptions{
				Message: github.Ptr(message),
				Content: contentBytes,
				Branch:  github.Ptr(branch),
			}

			// If SHA is provided, set it (for updates)
			sha, err := OptionalParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if sha != "" {
				opts.SHA = github.Ptr(sha)
			}

			// Create or update the file
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			fileContent, resp, err := client.Repositories.CreateFile(ctx, owner, repo, path, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create/update file",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 && resp.StatusCode != 201 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to create/update file: %s", string(body))), nil
			}

			r, err := json.Marshal(fileContent)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// CreateRepository creates a tool to create a new GitHub repository.
func CreateRepository(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_repository",
			mcp.WithDescription(t("TOOL_CREATE_REPOSITORY_DESCRIPTION", "Create a new GitHub repository in your account or specified organization")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_REPOSITORY_USER_TITLE", "Create repository"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("description",
				mcp.Description("Repository description"),
			),
			mcp.WithString("organization",
				mcp.Description("Organization to create the repository in (omit to create in your personal account)"),
			),
			mcp.WithBoolean("private",
				mcp.Description("Whether repo should be private"),
			),
			mcp.WithBoolean("autoInit",
				mcp.Description("Initialize with README"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := RequiredParam[string](request, "name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			description, err := OptionalParam[string](request, "description")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			organization, err := OptionalParam[string](request, "organization")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			private, err := OptionalParam[bool](request, "private")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			autoInit, err := OptionalParam[bool](request, "autoInit")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			repo := &github.Repository{
				Name:        github.Ptr(name),
				Description: github.Ptr(description),
				Private:     github.Ptr(private),
				AutoInit:    github.Ptr(autoInit),
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			createdRepo, resp, err := client.Repositories.Create(ctx, organization, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create repository",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to create repository: %s", string(body))), nil
			}

			// Return minimal response with just essential information
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", createdRepo.GetID()),
				URL: createdRepo.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetFileContents creates a tool to get the contents of a file or directory from a GitHub repository.
func GetFileContents(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_file_contents",
			mcp.WithDescription(t("TOOL_GET_FILE_CONTENTS_DESCRIPTION", "Get the contents of a file or directory from a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_FILE_CONTENTS_USER_TITLE", "Get file or directory contents"),
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
			mcp.WithString("path",
				mcp.Description("Path to file/directory (directories must end with a slash '/')"),
				mcp.DefaultString("/"),
			),
			mcp.WithString("ref",
				mcp.Description("Accepts optional git refs such as `refs/tags/{tag}`, `refs/heads/{branch}` or `refs/pull/{pr_number}/head`"),
			),
			mcp.WithString("sha",
				mcp.Description("Accepts optional commit SHA. If specified, it will be used instead of ref"),
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
			path, err := RequiredParam[string](request, "path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ref, err := OptionalParam[string](request, "ref")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sha, err := OptionalParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError("failed to get GitHub client"), nil
			}

			rawOpts, err := resolveGitReference(ctx, client, owner, repo, ref, sha)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to resolve git reference: %s", err)), nil
			}

			// If the path is (most likely) not to be a directory, we will
			// first try to get the raw content from the GitHub raw content API.
			if path != "" && !strings.HasSuffix(path, "/") {
				// First, get file info from Contents API to retrieve SHA
				var fileSHA string
				opts := &github.RepositoryContentGetOptions{Ref: ref}
				fileContent, _, respContents, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
				if respContents != nil {
					defer func() { _ = respContents.Body.Close() }()
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get file SHA",
						respContents,
						err,
					), nil
				}
				if fileContent == nil || fileContent.SHA == nil {
					return mcp.NewToolResultError("file content SHA is nil"), nil
				}
				fileSHA = *fileContent.SHA

				rawClient, err := getRawClient(ctx)
				if err != nil {
					return mcp.NewToolResultError("failed to get GitHub raw content client"), nil
				}
				resp, err := rawClient.GetRawContent(ctx, owner, repo, path, rawOpts)
				if err != nil {
					return mcp.NewToolResultError("failed to get raw repository content"), nil
				}
				defer func() {
					_ = resp.Body.Close()
				}()

				if resp.StatusCode == http.StatusOK {
					// If the raw content is found, return it directly
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return mcp.NewToolResultError("failed to read response body"), nil
					}
					contentType := resp.Header.Get("Content-Type")

					var resourceURI string
					switch {
					case sha != "":
						resourceURI, err = url.JoinPath("repo://", owner, repo, "sha", sha, "contents", path)
						if err != nil {
							return nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					case ref != "":
						resourceURI, err = url.JoinPath("repo://", owner, repo, ref, "contents", path)
						if err != nil {
							return nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					default:
						resourceURI, err = url.JoinPath("repo://", owner, repo, "contents", path)
						if err != nil {
							return nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					}

					if strings.HasPrefix(contentType, "application") || strings.HasPrefix(contentType, "text") {
						result := mcp.TextResourceContents{
							URI:      resourceURI,
							Text:     string(body),
							MIMEType: contentType,
						}
						// Include SHA in the result metadata
						if fileSHA != "" {
							return mcp.NewToolResultResource(fmt.Sprintf("successfully downloaded text file (SHA: %s)", fileSHA), result), nil
						}
						return mcp.NewToolResultResource("successfully downloaded text file", result), nil
					}

					result := mcp.BlobResourceContents{
						URI:      resourceURI,
						Blob:     base64.StdEncoding.EncodeToString(body),
						MIMEType: contentType,
					}
					// Include SHA in the result metadata
					if fileSHA != "" {
						return mcp.NewToolResultResource(fmt.Sprintf("successfully downloaded binary file (SHA: %s)", fileSHA), result), nil
					}
					return mcp.NewToolResultResource("successfully downloaded binary file", result), nil

				}
			}

			if rawOpts.SHA != "" {
				ref = rawOpts.SHA
			}
			if strings.HasSuffix(path, "/") {
				opts := &github.RepositoryContentGetOptions{Ref: ref}
				_, dirContent, resp, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
				if err == nil && resp.StatusCode == http.StatusOK {
					defer func() { _ = resp.Body.Close() }()
					r, err := json.Marshal(dirContent)
					if err != nil {
						return mcp.NewToolResultError("failed to marshal response"), nil
					}
					return mcp.NewToolResultText(string(r)), nil
				}
			}

			// The path does not point to a file or directory.
			// Instead let's try to find it in the Git Tree by matching the end of the path.

			// Step 1: Get Git Tree recursively
			tree, resp, err := client.Git.GetTree(ctx, owner, repo, ref, true)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get git tree",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Step 2: Filter tree for matching paths
			const maxMatchingFiles = 3
			matchingFiles := filterPaths(tree.Entries, path, maxMatchingFiles)
			if len(matchingFiles) > 0 {
				matchingFilesJSON, err := json.Marshal(matchingFiles)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to marshal matching files: %s", err)), nil
				}
				resolvedRefs, err := json.Marshal(rawOpts)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to marshal resolved refs: %s", err)), nil
				}
				return mcp.NewToolResultText(fmt.Sprintf("Path did not point to a file or directory, but resolved git ref to %s with possible path matches: %s", resolvedRefs, matchingFilesJSON)), nil
			}

			return mcp.NewToolResultError("Failed to get file contents. The path does not point to a file or directory, or the file does not exist in the repository."), nil
		}
}

// ForkRepository creates a tool to fork a repository.
func ForkRepository(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("fork_repository",
			mcp.WithDescription(t("TOOL_FORK_REPOSITORY_DESCRIPTION", "Fork a GitHub repository to your account or specified organization")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_FORK_REPOSITORY_USER_TITLE", "Fork repository"),
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
			mcp.WithString("organization",
				mcp.Description("Organization to fork to"),
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
			org, err := OptionalParam[string](request, "organization")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.RepositoryCreateForkOptions{}
			if org != "" {
				opts.Organization = org
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			forkedRepo, resp, err := client.Repositories.CreateFork(ctx, owner, repo, opts)
			if err != nil {
				// Check if it's an acceptedError. An acceptedError indicates that the update is in progress,
				// and it's not a real error.
				if resp != nil && resp.StatusCode == http.StatusAccepted && isAcceptedError(err) {
					return mcp.NewToolResultText("Fork is in progress"), nil
				}
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to fork repository",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusAccepted {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to fork repository: %s", string(body))), nil
			}

			// Return minimal response with just essential information
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", forkedRepo.GetID()),
				URL: forkedRepo.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// DeleteFile creates a tool to delete a file in a GitHub repository.
// This tool uses a more roundabout way of deleting a file than just using the client.Repositories.DeleteFile.
// This is because REST file deletion endpoint (and client.Repositories.DeleteFile) don't add commit signing to the deletion commit,
// unlike how the endpoint backing the create_or_update_files tool does. This appears to be a quirk of the API.
// The approach implemented here gets automatic commit signing when used with either the github-actions user or as an app,
// both of which suit an LLM well.
func DeleteFile(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("delete_file",
			mcp.WithDescription(t("TOOL_DELETE_FILE_DESCRIPTION", "Delete a file from a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:           t("TOOL_DELETE_FILE_USER_TITLE", "Delete file"),
				ReadOnlyHint:    ToBoolPtr(false),
				DestructiveHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner (username or organization)"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Path to the file to delete"),
			),
			mcp.WithString("message",
				mcp.Required(),
				mcp.Description("Commit message"),
			),
			mcp.WithString("branch",
				mcp.Required(),
				mcp.Description("Branch to delete the file from"),
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
			path, err := RequiredParam[string](request, "path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			message, err := RequiredParam[string](request, "message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			branch, err := RequiredParam[string](request, "branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get the reference for the branch
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
			if err != nil {
				return nil, fmt.Errorf("failed to get branch reference: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Get the commit object that the branch points to
			baseCommit, resp, err := client.Git.GetCommit(ctx, owner, repo, *ref.Object.SHA)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get base commit",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get commit: %s", string(body))), nil
			}

			// Create a tree entry for the file deletion by setting SHA to nil
			treeEntries := []*github.TreeEntry{
				{
					Path: github.Ptr(path),
					Mode: github.Ptr("100644"), // Regular file mode
					Type: github.Ptr("blob"),
					SHA:  nil, // Setting SHA to nil deletes the file
				},
			}

			// Create a new tree with the deletion
			newTree, resp, err := client.Git.CreateTree(ctx, owner, repo, *baseCommit.Tree.SHA, treeEntries)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create tree",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to create tree: %s", string(body))), nil
			}

			// Create a new commit with the new tree
			commit := &github.Commit{
				Message: github.Ptr(message),
				Tree:    newTree,
				Parents: []*github.Commit{{SHA: baseCommit.SHA}},
			}
			newCommit, resp, err := client.Git.CreateCommit(ctx, owner, repo, commit, nil)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create commit",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to create commit: %s", string(body))), nil
			}

			// Update the branch reference to point to the new commit
			ref.Object.SHA = newCommit.SHA
			_, resp, err = client.Git.UpdateRef(ctx, owner, repo, ref, false)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to update reference",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to update reference: %s", string(body))), nil
			}

			// Create a response similar to what the DeleteFile API would return
			response := map[string]interface{}{
				"commit":  newCommit,
				"content": nil,
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// CreateBranch creates a tool to create a new branch.
func CreateBranch(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_branch",
			mcp.WithDescription(t("TOOL_CREATE_BRANCH_DESCRIPTION", "Create a new branch in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_BRANCH_USER_TITLE", "Create branch"),
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
			mcp.WithString("branch",
				mcp.Required(),
				mcp.Description("Name for new branch"),
			),
			mcp.WithString("from_branch",
				mcp.Description("Source branch (defaults to repo default)"),
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
			branch, err := RequiredParam[string](request, "branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fromBranch, err := OptionalParam[string](request, "from_branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get the source branch SHA
			var ref *github.Reference

			if fromBranch == "" {
				// Get default branch if from_branch not specified
				repository, resp, err := client.Repositories.Get(ctx, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get repository",
						resp,
						err,
					), nil
				}
				defer func() { _ = resp.Body.Close() }()

				fromBranch = *repository.DefaultBranch
			}

			// Get SHA of source branch
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+fromBranch)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get reference",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Create new branch
			newRef := &github.Reference{
				Ref:    github.Ptr("refs/heads/" + branch),
				Object: &github.GitObject{SHA: ref.Object.SHA},
			}

			createdRef, resp, err := client.Git.CreateRef(ctx, owner, repo, newRef)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create branch",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(createdRef)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// PushFiles creates a tool to push multiple files in a single commit to a GitHub repository.
func PushFiles(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("push_files",
			mcp.WithDescription(t("TOOL_PUSH_FILES_DESCRIPTION", "Push multiple files to a GitHub repository in a single commit")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_PUSH_FILES_USER_TITLE", "Push files to repository"),
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
			mcp.WithString("branch",
				mcp.Required(),
				mcp.Description("Branch to push to"),
			),
			mcp.WithArray("files",
				mcp.Required(),
				mcp.Items(
					map[string]interface{}{
						"type":                 "object",
						"additionalProperties": false,
						"required":             []string{"path", "content"},
						"properties": map[string]interface{}{
							"path": map[string]interface{}{
								"type":        "string",
								"description": "path to the file",
							},
							"content": map[string]interface{}{
								"type":        "string",
								"description": "file content",
							},
						},
					}),
				mcp.Description("Array of file objects to push, each object with path (string) and content (string)"),
			),
			mcp.WithString("message",
				mcp.Required(),
				mcp.Description("Commit message"),
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
			branch, err := RequiredParam[string](request, "branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			message, err := RequiredParam[string](request, "message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Parse files parameter - this should be an array of objects with path and content
			filesObj, ok := request.GetArguments()["files"].([]interface{})
			if !ok {
				return mcp.NewToolResultError("files parameter must be an array of objects with path and content"), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get the reference for the branch
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get branch reference",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Get the commit object that the branch points to
			baseCommit, resp, err := client.Git.GetCommit(ctx, owner, repo, *ref.Object.SHA)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get base commit",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Create tree entries for all files
			var entries []*github.TreeEntry

			for _, file := range filesObj {
				fileMap, ok := file.(map[string]interface{})
				if !ok {
					return mcp.NewToolResultError("each file must be an object with path and content"), nil
				}

				path, ok := fileMap["path"].(string)
				if !ok || path == "" {
					return mcp.NewToolResultError("each file must have a path"), nil
				}

				content, ok := fileMap["content"].(string)
				if !ok {
					return mcp.NewToolResultError("each file must have content"), nil
				}

				// Create a tree entry for the file
				entries = append(entries, &github.TreeEntry{
					Path:    github.Ptr(path),
					Mode:    github.Ptr("100644"), // Regular file mode
					Type:    github.Ptr("blob"),
					Content: github.Ptr(content),
				})
			}

			// Create a new tree with the file entries
			newTree, resp, err := client.Git.CreateTree(ctx, owner, repo, *baseCommit.Tree.SHA, entries)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create tree",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Create a new commit
			commit := &github.Commit{
				Message: github.Ptr(message),
				Tree:    newTree,
				Parents: []*github.Commit{{SHA: baseCommit.SHA}},
			}
			newCommit, resp, err := client.Git.CreateCommit(ctx, owner, repo, commit, nil)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create commit",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Update the reference to point to the new commit
			ref.Object.SHA = newCommit.SHA
			updatedRef, resp, err := client.Git.UpdateRef(ctx, owner, repo, ref, false)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to update reference",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(updatedRef)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListTags creates a tool to list tags in a GitHub repository.
func ListTags(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_tags",
			mcp.WithDescription(t("TOOL_LIST_TAGS_DESCRIPTION", "List git tags in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_TAGS_USER_TITLE", "List tags"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			WithPagination(),
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
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ListOptions{
				Page:    pagination.Page,
				PerPage: pagination.PerPage,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			tags, resp, err := client.Repositories.ListTags(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list tags",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to list tags: %s", string(body))), nil
			}

			r, err := json.Marshal(tags)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetTag creates a tool to get details about a specific tag in a GitHub repository.
func GetTag(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_tag",
			mcp.WithDescription(t("TOOL_GET_TAG_DESCRIPTION", "Get details about a specific git tag in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_TAG_USER_TITLE", "Get tag details"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("tag",
				mcp.Required(),
				mcp.Description("Tag name"),
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
			tag, err := RequiredParam[string](request, "tag")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// First get the tag reference
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/tags/"+tag)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get tag reference",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get tag reference: %s", string(body))), nil
			}

			// Then get the tag object
			tagObj, resp, err := client.Git.GetTag(ctx, owner, repo, *ref.Object.SHA)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get tag object",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get tag object: %s", string(body))), nil
			}

			r, err := json.Marshal(tagObj)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListReleases creates a tool to list releases in a GitHub repository.
func ListReleases(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_releases",
			mcp.WithDescription(t("TOOL_LIST_RELEASES_DESCRIPTION", "List releases in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_RELEASES_USER_TITLE", "List releases"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			WithPagination(),
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
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ListOptions{
				Page:    pagination.Page,
				PerPage: pagination.PerPage,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			releases, resp, err := client.Repositories.ListReleases(ctx, owner, repo, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list releases: %w", err)
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
}

// GetLatestRelease creates a tool to get the latest release in a GitHub repository.
func GetLatestRelease(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_latest_release",
			mcp.WithDescription(t("TOOL_GET_LATEST_RELEASE_DESCRIPTION", "Get the latest release in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_LATEST_RELEASE_USER_TITLE", "Get latest release"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			release, resp, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
			if err != nil {
				return nil, fmt.Errorf("failed to get latest release: %w", err)
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
}

func GetReleaseByTag(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_release_by_tag",
			mcp.WithDescription(t("TOOL_GET_RELEASE_BY_TAG_DESCRIPTION", "Get a specific release by its tag name in a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_RELEASE_BY_TAG_USER_TITLE", "Get a release by tag name"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("tag",
				mcp.Required(),
				mcp.Description("Tag name (e.g., 'v1.0.0')"),
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
			tag, err := RequiredParam[string](request, "tag")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
}

// filterPaths filters the entries in a GitHub tree to find paths that
// match the given suffix.
// maxResults limits the number of results returned to first maxResults entries,
// a maxResults of -1 means no limit.
// It returns a slice of strings containing the matching paths.
// Directories are returned with a trailing slash.
func filterPaths(entries []*github.TreeEntry, path string, maxResults int) []string {
	// Remove trailing slash for matching purposes, but flag whether we
	// only want directories.
	dirOnly := false
	if strings.HasSuffix(path, "/") {
		dirOnly = true
		path = strings.TrimSuffix(path, "/")
	}

	matchedPaths := []string{}
	for _, entry := range entries {
		if len(matchedPaths) == maxResults {
			break // Limit the number of results to maxResults
		}
		if dirOnly && entry.GetType() != "tree" {
			continue // Skip non-directory entries if dirOnly is true
		}
		entryPath := entry.GetPath()
		if entryPath == "" {
			continue // Skip empty paths
		}
		if strings.HasSuffix(entryPath, path) {
			if entry.GetType() == "tree" {
				entryPath += "/" // Return directories with a trailing slash
			}
			matchedPaths = append(matchedPaths, entryPath)
		}
	}
	return matchedPaths
}

// resolveGitReference takes a user-provided ref and sha and resolves them into a
// definitive commit SHA and its corresponding fully-qualified reference.
//
// The resolution logic follows a clear priority:
//
//  1. If a specific commit `sha` is provided, it takes precedence and is used directly,
//     and all reference resolution is skipped.
//
//  2. If no `sha` is provided, the function resolves the `ref`
//     string into a fully-qualified format (e.g., "refs/heads/main") by trying
//     the following steps in order:
//     a). **Empty Ref:** If `ref` is empty, the repository's default branch is used.
//     b). **Fully-Qualified:** If `ref` already starts with "refs/", it's considered fully
//     qualified and used as-is.
//     c). **Partially-Qualified:** If `ref` starts with "heads/" or "tags/", it is
//     prefixed with "refs/" to make it fully-qualified.
//     d). **Short Name:** Otherwise, the `ref` is treated as a short name. The function
//     first attempts to resolve it as a branch ("refs/heads/<ref>"). If that
//     returns a 404 Not Found error, it then attempts to resolve it as a tag
//     ("refs/tags/<ref>").
//
//  3. **Final Lookup:** Once a fully-qualified ref is determined, a final API call
//     is made to fetch that reference's definitive commit SHA.
//
// Any unexpected (non-404) errors during the resolution process are returned
// immediately. All API errors are logged with rich context to aid diagnostics.
func resolveGitReference(ctx context.Context, githubClient *github.Client, owner, repo, ref, sha string) (*raw.ContentOpts, error) {
	// 1) If SHA explicitly provided, it's the highest priority.
	if sha != "" {
		return &raw.ContentOpts{Ref: "", SHA: sha}, nil
	}

	originalRef := ref // Keep original ref for clearer error messages down the line.

	// 2) If no SHA is provided, we try to resolve the ref into a fully-qualified format.
	var reference *github.Reference
	var resp *github.Response
	var err error

	switch {
	case originalRef == "":
		// 2a) If ref is empty, determine the default branch.
		repoInfo, resp, err := githubClient.Repositories.Get(ctx, owner, repo)
		if err != nil {
			_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get repository info", resp, err)
			return nil, fmt.Errorf("failed to get repository info: %w", err)
		}
		ref = fmt.Sprintf("refs/heads/%s", repoInfo.GetDefaultBranch())
	case strings.HasPrefix(originalRef, "refs/"):
		// 2b) Already fully qualified. The reference will be fetched at the end.
	case strings.HasPrefix(originalRef, "heads/") || strings.HasPrefix(originalRef, "tags/"):
		// 2c) Partially qualified. Make it fully qualified.
		ref = "refs/" + originalRef
	default:
		// 2d) It's a short name, so we try to resolve it to either a branch or a tag.
		branchRef := "refs/heads/" + originalRef
		reference, resp, err = githubClient.Git.GetRef(ctx, owner, repo, branchRef)

		if err == nil {
			ref = branchRef // It's a branch.
		} else {
			// The branch lookup failed. Check if it was a 404 Not Found error.
			ghErr, isGhErr := err.(*github.ErrorResponse)
			if isGhErr && ghErr.Response.StatusCode == http.StatusNotFound {
				tagRef := "refs/tags/" + originalRef
				reference, resp, err = githubClient.Git.GetRef(ctx, owner, repo, tagRef)
				if err == nil {
					ref = tagRef // It's a tag.
				} else {
					// The tag lookup also failed. Check if it was a 404 Not Found error.
					ghErr2, isGhErr2 := err.(*github.ErrorResponse)
					if isGhErr2 && ghErr2.Response.StatusCode == http.StatusNotFound {
						return nil, fmt.Errorf("could not resolve ref %q as a branch or a tag", originalRef)
					}
					// The tag lookup failed for a different reason.
					_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get reference (tag)", resp, err)
					return nil, fmt.Errorf("failed to get reference for tag '%s': %w", originalRef, err)
				}
			} else {
				// The branch lookup failed for a different reason.
				_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get reference (branch)", resp, err)
				return nil, fmt.Errorf("failed to get reference for branch '%s': %w", originalRef, err)
			}
		}
	}

	if reference == nil {
		reference, resp, err = githubClient.Git.GetRef(ctx, owner, repo, ref)
		if err != nil {
			_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get final reference", resp, err)
			return nil, fmt.Errorf("failed to get final reference for %q: %w", ref, err)
		}
	}

	sha = reference.GetObject().GetSHA()
	return &raw.ContentOpts{Ref: ref, SHA: sha}, nil
}

// ListStarredRepositories creates a tool to list starred repositories for the authenticated user or a specified user.
func ListStarredRepositories(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_starred_repositories",
			mcp.WithDescription(t("TOOL_LIST_STARRED_REPOSITORIES_DESCRIPTION", "List starred repositories")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_STARRED_REPOSITORIES_USER_TITLE", "List starred repositories"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("username",
				mcp.Description("Username to list starred repositories for. Defaults to the authenticated user."),
			),
			mcp.WithString("sort",
				mcp.Description("How to sort the results. Can be either 'created' (when the repository was starred) or 'updated' (when the repository was last pushed to)."),
				mcp.Enum("created", "updated"),
			),
			mcp.WithString("direction",
				mcp.Description("The direction to sort the results by."),
				mcp.Enum("asc", "desc"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			username, err := OptionalParam[string](request, "username")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sort, err := OptionalParam[string](request, "sort")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			direction, err := OptionalParam[string](request, "direction")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ActivityListStarredOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}
			if sort != "" {
				opts.Sort = sort
			}
			if direction != "" {
				opts.Direction = direction
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			var repos []*github.StarredRepository
			var resp *github.Response
			if username == "" {
				// List starred repositories for the authenticated user
				repos, resp, err = client.Activity.ListStarred(ctx, "", opts)
			} else {
				// List starred repositories for a specific user
				repos, resp, err = client.Activity.ListStarred(ctx, username, opts)
			}

			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list starred repositories for user '%s'", username),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list starred repositories: %s", string(body))), nil
			}

			// Convert to minimal format
			minimalRepos := make([]MinimalRepository, 0, len(repos))
			for _, starredRepo := range repos {
				repo := starredRepo.Repository
				minimalRepo := MinimalRepository{
					ID:            repo.GetID(),
					Name:          repo.GetName(),
					FullName:      repo.GetFullName(),
					Description:   repo.GetDescription(),
					HTMLURL:       repo.GetHTMLURL(),
					Language:      repo.GetLanguage(),
					Stars:         repo.GetStargazersCount(),
					Forks:         repo.GetForksCount(),
					OpenIssues:    repo.GetOpenIssuesCount(),
					Private:       repo.GetPrivate(),
					Fork:          repo.GetFork(),
					Archived:      repo.GetArchived(),
					DefaultBranch: repo.GetDefaultBranch(),
				}

				if repo.UpdatedAt != nil {
					minimalRepo.UpdatedAt = repo.UpdatedAt.Format("2006-01-02T15:04:05Z")
				}

				minimalRepos = append(minimalRepos, minimalRepo)
			}

			r, err := json.Marshal(minimalRepos)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal starred repositories: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// StarRepository creates a tool to star a repository.
func StarRepository(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("star_repository",
			mcp.WithDescription(t("TOOL_STAR_REPOSITORY_DESCRIPTION", "Star a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_STAR_REPOSITORY_USER_TITLE", "Star repository"),
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Activity.Star(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to star repository %s/%s", owner, repo),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 204 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to star repository: %s", string(body))), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Successfully starred repository %s/%s", owner, repo)), nil
		}
}

// UnstarRepository creates a tool to unstar a repository.
func UnstarRepository(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("unstar_repository",
			mcp.WithDescription(t("TOOL_UNSTAR_REPOSITORY_DESCRIPTION", "Unstar a GitHub repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_UNSTAR_REPOSITORY_USER_TITLE", "Unstar repository"),
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Activity.Unstar(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to unstar repository %s/%s", owner, repo),
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 204 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to unstar repository: %s", string(body))), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Successfully unstarred repository %s/%s", owner, repo)), nil
		}
}
