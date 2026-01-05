package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func GetCommit(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "get_commit",
			Description: t("TOOL_GET_COMMITS_DESCRIPTION", "Get details for a commit from a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_COMMITS_USER_TITLE", "Get commit details"),
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
					"sha": {
						Type:        "string",
						Description: "Commit SHA, branch name, or tag name",
					},
					"include_diff": {
						Type:        "boolean",
						Description: "Whether to include file diffs and stats in the response. Default is true.",
						Default:     json.RawMessage(`true`),
					},
				},
				Required: []string{"owner", "repo", "sha"},
			}),
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
			sha, err := RequiredParam[string](args, "sha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			includeDiff, err := OptionalBoolParamWithDefault(args, "include_diff", true)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
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
			commit, resp, err := client.Repositories.GetCommit(ctx, owner, repo, sha, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get commit: %s", sha),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get commit", resp, body), nil, nil
			}

			// Convert to minimal commit
			minimalCommit := convertToMinimalCommit(commit, includeDiff)

			r, err := json.Marshal(minimalCommit)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// ListCommits creates a tool to get commits of a branch in a repository.
func ListCommits(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "list_commits",
			Description: t("TOOL_LIST_COMMITS_DESCRIPTION", "Get list of commits of a branch in a GitHub repository. Returns at least 30 results per page by default, but can return more if specified using the perPage parameter (up to 100)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_COMMITS_USER_TITLE", "List commits"),
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
					"sha": {
						Type:        "string",
						Description: "Commit SHA, branch or tag name to list commits of. If not provided, uses the default branch of the repository. If a commit SHA is provided, will list commits up to that SHA.",
					},
					"author": {
						Type:        "string",
						Description: "Author username or email address to filter commits by",
					},
				},
				Required: []string{"owner", "repo"},
			}),
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
			sha, err := OptionalParam[string](args, "sha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			author, err := OptionalParam[string](args, "author")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			commits, resp, err := client.Repositories.ListCommits(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to list commits: %s", sha),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list commits", resp, body), nil, nil
			}

			// Convert to minimal commits
			minimalCommits := make([]MinimalCommit, len(commits))
			for i, commit := range commits {
				minimalCommits[i] = convertToMinimalCommit(commit, false)
			}

			r, err := json.Marshal(minimalCommits)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// ListBranches creates a tool to list branches in a GitHub repository.
func ListBranches(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "list_branches",
			Description: t("TOOL_LIST_BRANCHES_DESCRIPTION", "List branches in a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_BRANCHES_USER_TITLE", "List branches"),
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
				},
				Required: []string{"owner", "repo"},
			}),
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
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			opts := &github.BranchListOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			branches, resp, err := client.Repositories.ListBranches(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list branches",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list branches", resp, body), nil, nil
			}

			// Convert to minimal branches
			minimalBranches := make([]MinimalBranch, 0, len(branches))
			for _, branch := range branches {
				minimalBranches = append(minimalBranches, convertToMinimalBranch(branch))
			}

			r, err := json.Marshal(minimalBranches)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// CreateOrUpdateFile creates a tool to create or update a file in a GitHub repository.
func CreateOrUpdateFile(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name: "create_or_update_file",
			Description: t("TOOL_CREATE_OR_UPDATE_FILE_DESCRIPTION", `Create or update a single file in a GitHub repository. 
If updating, you should provide the SHA of the file you want to update. Use this tool to create or update a file in a GitHub repository remotely; do not use it for local file operations.

In order to obtain the SHA of original file version before updating, use the following git command:
git ls-tree HEAD <path to file>

If the SHA is not provided, the tool will attempt to acquire it by fetching the current file contents from the repository, which may lead to rewriting latest committed changes if the file has changed since last retrieval.
`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_OR_UPDATE_FILE_USER_TITLE", "Create or update file"),
				ReadOnlyHint: false,
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
						Description: "Path where to create/update the file",
					},
					"content": {
						Type:        "string",
						Description: "Content of the file",
					},
					"message": {
						Type:        "string",
						Description: "Commit message",
					},
					"branch": {
						Type:        "string",
						Description: "Branch to create/update the file in",
					},
					"sha": {
						Type:        "string",
						Description: "The blob SHA of the file being replaced.",
					},
				},
				Required: []string{"owner", "repo", "path", "content", "message", "branch"},
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
			path, err := RequiredParam[string](args, "path")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			content, err := RequiredParam[string](args, "content")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			message, err := RequiredParam[string](args, "message")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			branch, err := RequiredParam[string](args, "branch")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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
			sha, err := OptionalParam[string](args, "sha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if sha != "" {
				opts.SHA = github.Ptr(sha)
			}

			// Create or update the file
			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			path = strings.TrimPrefix(path, "/")

			// SHA validation using conditional HEAD request (efficient - no body transfer)
			var previousSHA string
			contentURL := fmt.Sprintf("repos/%s/%s/contents/%s", owner, repo, url.PathEscape(path))
			if branch != "" {
				contentURL += "?ref=" + url.QueryEscape(branch)
			}

			if sha != "" {
				// User provided SHA - validate it's still current
				req, err := client.NewRequest("HEAD", contentURL, nil)
				if err == nil {
					req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, sha))
					resp, _ := client.Do(ctx, req, nil)
					if resp != nil {
						defer resp.Body.Close()

						switch resp.StatusCode {
						case http.StatusNotModified:
							// SHA matches current - proceed
							opts.SHA = github.Ptr(sha)
						case http.StatusOK:
							// SHA is stale - reject with current SHA so user can check diff
							currentSHA := strings.Trim(resp.Header.Get("ETag"), `"`)
							return utils.NewToolResultError(fmt.Sprintf(
								"SHA mismatch: provided SHA %s is stale. Current file SHA is %s. "+
									"Use get_file_contents or compare commits to review changes before updating.",
								sha, currentSHA)), nil, nil
						case http.StatusNotFound:
							// File doesn't exist - this is a create, ignore provided SHA
						}
					}
				}
			} else {
				// No SHA provided - check if file exists to warn about blind update
				req, err := client.NewRequest("HEAD", contentURL, nil)
				if err == nil {
					resp, _ := client.Do(ctx, req, nil)
					if resp != nil {
						defer resp.Body.Close()
						if resp.StatusCode == http.StatusOK {
							previousSHA = strings.Trim(resp.Header.Get("ETag"), `"`)
						}
						// 404 = new file, no previous SHA needed
					}
				}
			}

			if previousSHA != "" {
				opts.SHA = github.Ptr(previousSHA)
			}

			fileContent, resp, err := client.Repositories.CreateFile(ctx, owner, repo, path, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create/update file",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 && resp.StatusCode != 201 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create/update file", resp, body), nil, nil
			}

			r, err := json.Marshal(fileContent)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			// Warn if file was updated without SHA validation (blind update)
			if sha == "" && previousSHA != "" {
				return utils.NewToolResultText(fmt.Sprintf(
					"Warning: File updated without SHA validation. Previous file SHA was %s. "+
						`Verify no unintended changes were overwritten: 
1. Extract the SHA of the local version using git ls-tree HEAD %s.
2. Compare with the previous SHA above.
3. Revert changes if shas do not match.

%s`,
					previousSHA, path, string(r))), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// CreateRepository creates a tool to create a new GitHub repository.
func CreateRepository(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "create_repository",
			Description: t("TOOL_CREATE_REPOSITORY_DESCRIPTION", "Create a new GitHub repository in your account or specified organization"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_REPOSITORY_USER_TITLE", "Create repository"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {
						Type:        "string",
						Description: "Repository name",
					},
					"description": {
						Type:        "string",
						Description: "Repository description",
					},
					"organization": {
						Type:        "string",
						Description: "Organization to create the repository in (omit to create in your personal account)",
					},
					"private": {
						Type:        "boolean",
						Description: "Whether repo should be private",
					},
					"autoInit": {
						Type:        "boolean",
						Description: "Initialize with README",
					},
				},
				Required: []string{"name"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			name, err := RequiredParam[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			description, err := OptionalParam[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			organization, err := OptionalParam[string](args, "organization")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			private, err := OptionalParam[bool](args, "private")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			autoInit, err := OptionalParam[bool](args, "autoInit")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			repo := &github.Repository{
				Name:        github.Ptr(name),
				Description: github.Ptr(description),
				Private:     github.Ptr(private),
				AutoInit:    github.Ptr(autoInit),
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			createdRepo, resp, err := client.Repositories.Create(ctx, organization, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create repository",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create repository", resp, body), nil, nil
			}

			// Return minimal response with just essential information
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", createdRepo.GetID()),
				URL: createdRepo.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GetFileContents creates a tool to get the contents of a file or directory from a GitHub repository.
func GetFileContents(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "get_file_contents",
			Description: t("TOOL_GET_FILE_CONTENTS_DESCRIPTION", "Get the contents of a file or directory from a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_FILE_CONTENTS_USER_TITLE", "Get file or directory contents"),
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
						Description: "Path to file/directory",
						Default:     json.RawMessage(`"/"`),
					},
					"ref": {
						Type:        "string",
						Description: "Accepts optional git refs such as `refs/tags/{tag}`, `refs/heads/{branch}` or `refs/pull/{pr_number}/head`",
					},
					"sha": {
						Type:        "string",
						Description: "Accepts optional commit SHA. If specified, it will be used instead of ref",
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

			path, err := OptionalParam[string](args, "path")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			path = strings.TrimPrefix(path, "/")

			ref, err := OptionalParam[string](args, "ref")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			originalRef := ref

			sha, err := OptionalParam[string](args, "sha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError("failed to get GitHub client"), nil, nil
			}

			rawOpts, fallbackUsed, err := resolveGitReference(ctx, client, owner, repo, ref, sha)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to resolve git reference: %s", err)), nil, nil
			}

			if rawOpts.SHA != "" {
				ref = rawOpts.SHA
			}

			var fileSHA string
			opts := &github.RepositoryContentGetOptions{Ref: ref}

			// Always call GitHub Contents API first to get metadata including SHA and determine if it's a file or directory
			fileContent, dirContent, respContents, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
			if respContents != nil {
				defer func() { _ = respContents.Body.Close() }()
			}

			// The path does not point to a file or directory.
			// Instead let's try to find it in the Git Tree by matching the end of the path.
			if err != nil || (fileContent == nil && dirContent == nil) {
				return matchFiles(ctx, client, owner, repo, ref, path, rawOpts, 0)
			}

			if fileContent != nil && fileContent.SHA != nil {
				fileSHA = *fileContent.SHA

				rawClient, err := deps.GetRawClient(ctx)
				if err != nil {
					return utils.NewToolResultError("failed to get GitHub raw content client"), nil, nil
				}
				resp, err := rawClient.GetRawContent(ctx, owner, repo, path, rawOpts)
				if err != nil {
					return utils.NewToolResultError("failed to get raw repository content"), nil, nil
				}
				defer func() {
					_ = resp.Body.Close()
				}()

				if resp.StatusCode == http.StatusOK {
					// If the raw content is found, return it directly
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return ghErrors.NewGitHubRawAPIErrorResponse(ctx, "failed to get raw repository content", resp, err), nil, nil
					}
					contentType := resp.Header.Get("Content-Type")

					var resourceURI string
					switch {
					case sha != "":
						resourceURI, err = url.JoinPath("repo://", owner, repo, "sha", sha, "contents", path)
						if err != nil {
							return nil, nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					case ref != "":
						resourceURI, err = url.JoinPath("repo://", owner, repo, ref, "contents", path)
						if err != nil {
							return nil, nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					default:
						resourceURI, err = url.JoinPath("repo://", owner, repo, "contents", path)
						if err != nil {
							return nil, nil, fmt.Errorf("failed to create resource URI: %w", err)
						}
					}

					// main branch ref passed in ref parameter but it doesn't exist - default branch was used
					var successNote string
					if fallbackUsed {
						successNote = fmt.Sprintf(" Note: the provided ref '%s' does not exist, default branch '%s' was used instead.", originalRef, rawOpts.Ref)
					}

					// Determine if content is text or binary
					isTextContent := strings.HasPrefix(contentType, "text/") ||
						contentType == "application/json" ||
						contentType == "application/xml" ||
						strings.HasSuffix(contentType, "+json") ||
						strings.HasSuffix(contentType, "+xml")

					if isTextContent {
						result := &mcp.ResourceContents{
							URI:      resourceURI,
							Text:     string(body),
							MIMEType: contentType,
						}
						// Include SHA in the result metadata
						if fileSHA != "" {
							return utils.NewToolResultResource(fmt.Sprintf("successfully downloaded text file (SHA: %s)", fileSHA)+successNote, result), nil, nil
						}
						return utils.NewToolResultResource("successfully downloaded text file"+successNote, result), nil, nil
					}

					result := &mcp.ResourceContents{
						URI:      resourceURI,
						Blob:     body,
						MIMEType: contentType,
					}
					// Include SHA in the result metadata
					if fileSHA != "" {
						return utils.NewToolResultResource(fmt.Sprintf("successfully downloaded binary file (SHA: %s)", fileSHA)+successNote, result), nil, nil
					}
					return utils.NewToolResultResource("successfully downloaded binary file"+successNote, result), nil, nil
				}

				// Raw API call failed
				return matchFiles(ctx, client, owner, repo, ref, path, rawOpts, resp.StatusCode)
			} else if dirContent != nil {
				// file content or file SHA is nil which means it's a directory
				r, err := json.Marshal(dirContent)
				if err != nil {
					return utils.NewToolResultError("failed to marshal response"), nil, nil
				}
				return utils.NewToolResultText(string(r)), nil, nil
			}

			return utils.NewToolResultError("failed to get file contents"), nil, nil
		},
	)
}

// ForkRepository creates a tool to fork a repository.
func ForkRepository(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "fork_repository",
			Description: t("TOOL_FORK_REPOSITORY_DESCRIPTION", "Fork a GitHub repository to your account or specified organization"),
			Icons:       octicons.Icons("repo-forked"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_FORK_REPOSITORY_USER_TITLE", "Fork repository"),
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
					"organization": {
						Type:        "string",
						Description: "Organization to fork to",
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
			org, err := OptionalParam[string](args, "organization")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			opts := &github.RepositoryCreateForkOptions{}
			if org != "" {
				opts.Organization = org
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			forkedRepo, resp, err := client.Repositories.CreateFork(ctx, owner, repo, opts)
			if err != nil {
				// Check if it's an acceptedError. An acceptedError indicates that the update is in progress,
				// and it's not a real error.
				if resp != nil && resp.StatusCode == http.StatusAccepted && isAcceptedError(err) {
					return utils.NewToolResultText("Fork is in progress"), nil, nil
				}
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to fork repository",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusAccepted {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to fork repository", resp, body), nil, nil
			}

			// Return minimal response with just essential information
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", forkedRepo.GetID()),
				URL: forkedRepo.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// DeleteFile creates a tool to delete a file in a GitHub repository.
// This tool uses a more roundabout way of deleting a file than just using the client.Repositories.DeleteFile.
// This is because REST file deletion endpoint (and client.Repositories.DeleteFile) don't add commit signing to the deletion commit,
// unlike how the endpoint backing the create_or_update_files tool does. This appears to be a quirk of the API.
// The approach implemented here gets automatic commit signing when used with either the github-actions user or as an app,
// both of which suit an LLM well.
func DeleteFile(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "delete_file",
			Description: t("TOOL_DELETE_FILE_DESCRIPTION", "Delete a file from a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_DELETE_FILE_USER_TITLE", "Delete file"),
				ReadOnlyHint:    false,
				DestructiveHint: github.Ptr(true),
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
						Description: "Path to the file to delete",
					},
					"message": {
						Type:        "string",
						Description: "Commit message",
					},
					"branch": {
						Type:        "string",
						Description: "Branch to delete the file from",
					},
				},
				Required: []string{"owner", "repo", "path", "message", "branch"},
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
			path, err := RequiredParam[string](args, "path")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			message, err := RequiredParam[string](args, "message")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			branch, err := RequiredParam[string](args, "branch")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get the reference for the branch
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get branch reference: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Get the commit object that the branch points to
			baseCommit, resp, err := client.Git.GetCommit(ctx, owner, repo, *ref.Object.SHA)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get base commit",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get commit", resp, body), nil, nil
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
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create tree", resp, body), nil, nil
			}

			// Create a new commit with the new tree
			commit := github.Commit{
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
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create commit", resp, body), nil, nil
			}

			// Update the branch reference to point to the new commit
			ref.Object.SHA = newCommit.SHA
			_, resp, err = client.Git.UpdateRef(ctx, owner, repo, *ref.Ref, github.UpdateRef{
				SHA:   *newCommit.SHA,
				Force: github.Ptr(false),
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to update reference",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to update reference", resp, body), nil, nil
			}

			// Create a response similar to what the DeleteFile API would return
			response := map[string]interface{}{
				"commit":  newCommit,
				"content": nil,
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// CreateBranch creates a tool to create a new branch.
func CreateBranch(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "create_branch",
			Description: t("TOOL_CREATE_BRANCH_DESCRIPTION", "Create a new branch in a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_BRANCH_USER_TITLE", "Create branch"),
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
					"branch": {
						Type:        "string",
						Description: "Name for new branch",
					},
					"from_branch": {
						Type:        "string",
						Description: "Source branch (defaults to repo default)",
					},
				},
				Required: []string{"owner", "repo", "branch"},
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
			branch, err := RequiredParam[string](args, "branch")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			fromBranch, err := OptionalParam[string](args, "from_branch")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
					), nil, nil
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
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Create new branch
			newRef := github.CreateRef{
				Ref: "refs/heads/" + branch,
				SHA: *ref.Object.SHA,
			}

			createdRef, resp, err := client.Git.CreateRef(ctx, owner, repo, newRef)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create branch",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(createdRef)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// PushFiles creates a tool to push multiple files in a single commit to a GitHub repository.
func PushFiles(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "push_files",
			Description: t("TOOL_PUSH_FILES_DESCRIPTION", "Push multiple files to a GitHub repository in a single commit"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_PUSH_FILES_USER_TITLE", "Push files to repository"),
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
					"branch": {
						Type:        "string",
						Description: "Branch to push to",
					},
					"files": {
						Type:        "array",
						Description: "Array of file objects to push, each object with path (string) and content (string)",
						Items: &jsonschema.Schema{
							Type: "object",
							Properties: map[string]*jsonschema.Schema{
								"path": {
									Type:        "string",
									Description: "path to the file",
								},
								"content": {
									Type:        "string",
									Description: "file content",
								},
							},
							Required: []string{"path", "content"},
						},
					},
					"message": {
						Type:        "string",
						Description: "Commit message",
					},
				},
				Required: []string{"owner", "repo", "branch", "files", "message"},
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
			branch, err := RequiredParam[string](args, "branch")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			message, err := RequiredParam[string](args, "message")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Parse files parameter - this should be an array of objects with path and content
			filesObj, ok := args["files"].([]interface{})
			if !ok {
				return utils.NewToolResultError("files parameter must be an array of objects with path and content"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get the reference for the branch
			var repositoryIsEmpty bool
			var branchNotFound bool
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
			if err != nil {
				ghErr, isGhErr := err.(*github.ErrorResponse)
				if isGhErr {
					if ghErr.Response.StatusCode == http.StatusConflict && ghErr.Message == "Git Repository is empty." {
						repositoryIsEmpty = true
					} else if ghErr.Response.StatusCode == http.StatusNotFound {
						branchNotFound = true
					}
				}

				if !repositoryIsEmpty && !branchNotFound {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get branch reference",
						resp,
						err,
					), nil, nil
				}
			}
			// Only close resp if it's not nil and not an error case where resp might be nil
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}

			var baseCommit *github.Commit
			if !repositoryIsEmpty {
				if branchNotFound {
					ref, err = createReferenceFromDefaultBranch(ctx, client, owner, repo, branch)
					if err != nil {
						return utils.NewToolResultError(fmt.Sprintf("failed to create branch from default: %v", err)), nil, nil
					}
				}

				// Get the commit object that the branch points to
				baseCommit, resp, err = client.Git.GetCommit(ctx, owner, repo, *ref.Object.SHA)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to get base commit",
						resp,
						err,
					), nil, nil
				}
				if resp != nil && resp.Body != nil {
					defer func() { _ = resp.Body.Close() }()
				}
			} else {
				var base *github.Commit
				// Repository is empty, need to initialize it first
				ref, base, err = initializeRepository(ctx, client, owner, repo)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("failed to initialize repository: %v", err)), nil, nil
				}

				defaultBranch := strings.TrimPrefix(*ref.Ref, "refs/heads/")
				if branch != defaultBranch {
					// Create the requested branch from the default branch
					ref, err = createReferenceFromDefaultBranch(ctx, client, owner, repo, branch)
					if err != nil {
						return utils.NewToolResultError(fmt.Sprintf("failed to create branch from default: %v", err)), nil, nil
					}
				}

				baseCommit = base
			}

			// Create tree entries for all files (or remaining files if empty repo)
			var entries []*github.TreeEntry

			for _, file := range filesObj {
				fileMap, ok := file.(map[string]interface{})
				if !ok {
					return utils.NewToolResultError("each file must be an object with path and content"), nil, nil
				}

				path, ok := fileMap["path"].(string)
				if !ok || path == "" {
					return utils.NewToolResultError("each file must have a path"), nil, nil
				}

				content, ok := fileMap["content"].(string)
				if !ok {
					return utils.NewToolResultError("each file must have content"), nil, nil
				}

				// Create a tree entry for the file
				entries = append(entries, &github.TreeEntry{
					Path:    github.Ptr(path),
					Mode:    github.Ptr("100644"), // Regular file mode
					Type:    github.Ptr("blob"),
					Content: github.Ptr(content),
				})
			}

			// Create a new tree with the file entries (baseCommit is now guaranteed to exist)
			newTree, resp, err := client.Git.CreateTree(ctx, owner, repo, *baseCommit.Tree.SHA, entries)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create tree",
					resp,
					err,
				), nil, nil
			}
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}

			// Create a new commit (baseCommit always has a value now)
			commit := github.Commit{
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
				), nil, nil
			}
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}

			// Update the reference to point to the new commit
			ref.Object.SHA = newCommit.SHA
			updatedRef, resp, err := client.Git.UpdateRef(ctx, owner, repo, *ref.Ref, github.UpdateRef{
				SHA:   *newCommit.SHA,
				Force: github.Ptr(false),
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to update reference",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(updatedRef)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// ListTags creates a tool to list tags in a GitHub repository.
func ListTags(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "list_tags",
			Description: t("TOOL_LIST_TAGS_DESCRIPTION", "List git tags in a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_TAGS_USER_TITLE", "List tags"),
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
				},
				Required: []string{"owner", "repo"},
			}),
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

			tags, resp, err := client.Repositories.ListTags(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list tags",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list tags", resp, body), nil, nil
			}

			r, err := json.Marshal(tags)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GetTag creates a tool to get details about a specific tag in a GitHub repository.
func GetTag(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "get_tag",
			Description: t("TOOL_GET_TAG_DESCRIPTION", "Get details about a specific git tag in a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_TAG_USER_TITLE", "Get tag details"),
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
					"tag": {
						Type:        "string",
						Description: "Tag name",
					},
				},
				Required: []string{"owner", "repo", "tag"},
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
			tag, err := RequiredParam[string](args, "tag")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// First get the tag reference
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/tags/"+tag)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get tag reference",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get tag reference", resp, body), nil, nil
			}

			// Then get the tag object
			tagObj, resp, err := client.Git.GetTag(ctx, owner, repo, *ref.Object.SHA)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get tag object",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get tag object", resp, body), nil, nil
			}

			r, err := json.Marshal(tagObj)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// ListReleases creates a tool to list releases in a GitHub repository.
func ListReleases(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "list_releases",
			Description: t("TOOL_LIST_RELEASES_DESCRIPTION", "List releases in a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_RELEASES_USER_TITLE", "List releases"),
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
				},
				Required: []string{"owner", "repo"},
			}),
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

			releases, resp, err := client.Repositories.ListReleases(ctx, owner, repo, opts)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to list releases: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list releases", resp, body), nil, nil
			}

			r, err := json.Marshal(releases)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GetLatestRelease creates a tool to get the latest release in a GitHub repository.
func GetLatestRelease(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "get_latest_release",
			Description: t("TOOL_GET_LATEST_RELEASE_DESCRIPTION", "Get the latest release in a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_LATEST_RELEASE_USER_TITLE", "Get latest release"),
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			release, resp, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get latest release: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get latest release", resp, body), nil, nil
			}

			r, err := json.Marshal(release)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func GetReleaseByTag(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataRepos,
		mcp.Tool{
			Name:        "get_release_by_tag",
			Description: t("TOOL_GET_RELEASE_BY_TAG_DESCRIPTION", "Get a specific release by its tag name in a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_RELEASE_BY_TAG_USER_TITLE", "Get a release by tag name"),
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
					"tag": {
						Type:        "string",
						Description: "Tag name (e.g., 'v1.0.0')",
					},
				},
				Required: []string{"owner", "repo", "tag"},
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
			tag, err := RequiredParam[string](args, "tag")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			release, resp, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get release by tag: %s", tag),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get release by tag", resp, body), nil, nil
			}

			r, err := json.Marshal(release)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// ListStarredRepositories creates a tool to list starred repositories for the authenticated user or a specified user.
func ListStarredRepositories(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataStargazers,
		mcp.Tool{
			Name:        "list_starred_repositories",
			Description: t("TOOL_LIST_STARRED_REPOSITORIES_DESCRIPTION", "List starred repositories"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_STARRED_REPOSITORIES_USER_TITLE", "List starred repositories"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"username": {
						Type:        "string",
						Description: "Username to list starred repositories for. Defaults to the authenticated user.",
					},
					"sort": {
						Type:        "string",
						Description: "How to sort the results. Can be either 'created' (when the repository was starred) or 'updated' (when the repository was last pushed to).",
						Enum:        []any{"created", "updated"},
					},
					"direction": {
						Type:        "string",
						Description: "The direction to sort the results by.",
						Enum:        []any{"asc", "desc"},
					},
				},
			}),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			username, err := OptionalParam[string](args, "username")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			sort, err := OptionalParam[string](args, "sort")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			direction, err := OptionalParam[string](args, "direction")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list starred repositories", resp, body), nil, nil
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
				return nil, nil, fmt.Errorf("failed to marshal starred repositories: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// StarRepository creates a tool to star a repository.
func StarRepository(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataStargazers,
		mcp.Tool{
			Name:        "star_repository",
			Description: t("TOOL_STAR_REPOSITORY_DESCRIPTION", "Star a GitHub repository"),
			Icons:       octicons.Icons("star-fill"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_STAR_REPOSITORY_USER_TITLE", "Star repository"),
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Activity.Star(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to star repository %s/%s", owner, repo),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 204 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to star repository", resp, body), nil, nil
			}

			return utils.NewToolResultText(fmt.Sprintf("Successfully starred repository %s/%s", owner, repo)), nil, nil
		},
	)
}

// UnstarRepository creates a tool to unstar a repository.
func UnstarRepository(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataStargazers,
		mcp.Tool{
			Name:        "unstar_repository",
			Description: t("TOOL_UNSTAR_REPOSITORY_DESCRIPTION", "Unstar a GitHub repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UNSTAR_REPOSITORY_USER_TITLE", "Unstar repository"),
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Activity.Unstar(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to unstar repository %s/%s", owner, repo),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 204 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to unstar repository", resp, body), nil, nil
			}

			return utils.NewToolResultText(fmt.Sprintf("Successfully unstarred repository %s/%s", owner, repo)), nil, nil
		},
	)
}
