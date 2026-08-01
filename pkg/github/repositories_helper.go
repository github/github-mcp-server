package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// initializeRepository 创建s 一个initial commit in 一个空 仓库 和返回 默认分支 ref 和base commit
func initializeRepository(ctx context.Context, client *github.Client, owner, repo string) (ref *github.Reference, baseCommit *github.Commit, err error) {
	// First, we need to 检查 what 默认分支 in this 空 repo 应当是:
	repository, resp, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get repository", resp, err)
		return nil, nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	defaultBranch := repository.GetDefaultBranch()

	fileOpts := &github.RepositoryContentFileOptions{
		Message: github.Ptr("Initial commit"),
		Content: []byte(""),
		Branch:  github.Ptr(defaultBranch),
	}

	// Create 一个initial 空 commit to 创建 默认分支
	createResp, resp, err := client.Repositories.CreateFile(ctx, owner, repo, "README.md", fileOpts)
	if err != nil {
		_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to create initial file", resp, err)
		return nil, nil, fmt.Errorf("failed to create initial file: %w", err)
	}
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	// Get commit that was just 创建d to use as base f或remaining 文件s
	baseCommit, resp, err = client.Git.GetCommit(ctx, owner, repo, *createResp.Commit.SHA)
	if err != nil {
		_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get initial commit", resp, err)
		return nil, nil, fmt.Errorf("failed to get initial commit: %w", err)
	}
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	ref, resp, err = client.Git.GetRef(ctx, owner, repo, "refs/heads/"+defaultBranch)
	if err != nil {
		_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get final reference", resp, err)
		return nil, nil, fmt.Errorf("failed to get branch reference after initial commit: %w", err)
	}
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	return ref, baseCommit, nil
}

// 创建ReferenceFromDefaultBranch 创建s 一个新的 分支 reference 来自仓库's 默认分支
func createReferenceFromDefaultBranch(ctx context.Context, client *github.Client, owner, repo, branch string) (*github.Reference, error) {
	defaultRef, err := resolveDefaultBranch(ctx, client, owner, repo)
	if err != nil {
		_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to resolve default branch", nil, err)
		return nil, fmt.Errorf("failed to resolve default branch: %w", err)
	}

	// Create 新的 分支 reference
	createdRef, resp, err := client.Git.CreateRef(ctx, owner, repo, github.CreateRef{
		Ref: "refs/heads/" + branch,
		SHA: *defaultRef.Object.SHA,
	})
	if err != nil {
		_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to create new branch reference", resp, err)
		return nil, fmt.Errorf("failed to create new branch reference: %w", err)
	}
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	return createdRef, nil
}

// matchFiles searches f或文件s 在Git tree that match given 路径.
// It's 在以下情况使用： GetContents fails 或返回 unexpected 结果.
func matchFiles(ctx context.Context, client *github.Client, owner, repo, ref, path string, rawOpts *raw.ContentOpts, rawAPIResponseCode int) (*mcp.CallToolResult, any, error) {
	// Step 1: Get Git Tree recursively
	tree, response, err := client.Git.GetTree(ctx, owner, repo, ref, true)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get git tree",
			response,
			err,
		), nil, nil
	}
	defer func() { _ = response.Body.Close() }()

	// Step 2: Filter tree f或matching 路径s
	const maxMatchingFiles = 3
	matchingFiles := filterPaths(tree.Entries, path, maxMatchingFiles)
	if len(matchingFiles) > 0 {
		matchingFilesJSON, err := json.Marshal(matchingFiles)
		if err != nil {
			return utils.NewToolResultError(fmt.Sprintf("failed to marshal matching files: %s", err)), nil, nil
		}
		resolvedRefs, err := json.Marshal(rawOpts)
		if err != nil {
			return utils.NewToolResultError(fmt.Sprintf("failed to marshal resolved refs: %s", err)), nil, nil
		}
		if rawAPIResponseCode > 0 {
			return utils.NewToolResultText(fmt.Sprintf("Resolved potential matches in the repository tree (resolved refs: %s, matching files: %s), but the content API returned an unexpected status code %d.", string(resolvedRefs), string(matchingFilesJSON), rawAPIResponseCode)), nil, nil
		}
		return utils.NewToolResultText(fmt.Sprintf("Resolved potential matches in the repository tree (resolved refs: %s, matching files: %s).", string(resolvedRefs), string(matchingFilesJSON))), nil, nil
	}
	return utils.NewToolResultError("Failed to get file contents. The path does not point to a file or directory, or the file does not exist in the repository."), nil, nil
}

// 筛选Paths 筛选s entries in 一个GitHub tree to find 路径s that
// match given suffix.
// maxResults limits number of 结果 返回ed to 第一个 maxResults entries,
// 一个maxResults of -1 means no limit.
// It 返回 一个slice of strings containing matching 路径s.
// Directories are 返回ed with 一个trailing slash.
func filterPaths(entries []*github.TreeEntry, path string, maxResults int) []string {
	// Remove trailing slash f或matching purposes, 但flag whether we
	// 仅want directories.
	dirOnly := false
	if strings.HasSuffix(path, "/") {
		dirOnly = true
		path = strings.TrimSuffix(path, "/")
	}

	matchedPaths := []string{}
	for _, entry := range entries {
		if len(matchedPaths) == maxResults {
			break // Limit number of 结果 to maxResults
		}
		if dirOnly && entry.GetType() != "tree" {
			continue // Skip non-directory entries if dir仅is 真
		}
		entryPath := entry.GetPath()
		if entryPath == "" {
			continue // Skip 空 路径s
		}
		if strings.HasSuffix(entryPath, path) {
			if entry.GetType() == "tree" {
				entryPath += "/" // Return directories with 一个trailing slash
			}
			matchedPaths = append(matchedPaths, entryPath)
		}
	}
	return matchedPaths
}

// looksLikeSH一个返回 真 如果string appears to be 一个Git commit SHA.
// 一个SH一个is 一个40-character hexadecimal string.
func looksLikeSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// resolveGitReference takes 一个user-provided ref 和sha 和resolves them into a
// definitive commit SH一个和its corresponding fully-qualified reference.
//
// resolution logic follows 一个clear priority:
//
//  1. If 一个specific commit `sha` is provided, it takes precedence 和is used directly,
//     和所有reference resolution is skipped.
//
//     1a. If `sha` is 空 但`ref` looks like 一个commit SH一个(40 hexadecimal characters),
//     it is 返回ed as-is without any API 调用 或reference resolution.
//
//  2. 如果没有`sha` is provided 和`ref` does 不look like 一个SHA, 函数 resolves
//     `ref` string into 一个fully-qualified format (e.g., "refs/heads/main") by trying
//     following steps in order:
//     a). **Empty Ref:** If `ref` is 空, 仓库's 默认分支 is used.
//     b). **Fully-Qualified:** If `ref` al读取y starts with "refs/", it's considered fully
//     qualified 和used as-is.
//     c). **Partially-Qualified:** If `ref` starts with "heads/" 或"tags/", it is
//     prefixed with "refs/" to make it fully-qualified.
//     d). **Short Name:** Otherwise, `ref` is treated as 一个short name. 函数
//     第一个 attempts to resolve it as 一个分支 ("refs/heads/<ref>"). If that
//     返回 一个404 不Found 错误, it 然后attempts to resolve it as 一个tag
//     ("refs/tags/<ref>").
//
//  3. **Final Lookup:** Once 一个fully-qualified ref is determined, 一个final API 调用
//     is made to fetch that reference's definitive commit SHA.
//
// Any unexpected (non-404) 错误s during resolution process are 返回ed
// immediately. 所有API 错误s are logged with rich 上下文 to aid diagnostics.
func resolveGitReference(ctx context.Context, githubClient *github.Client, owner, repo, ref, sha string) (*raw.ContentOpts, bool, error) {
	// 1) If SH一个explicitly provided, it's highest priority.
	if sha != "" {
		return &raw.ContentOpts{Ref: "", SHA: sha}, false, nil
	}

	// 1a) If sha is 空 但ref looks like 一个SHA, 返回 it without changes
	if looksLikeSHA(ref) {
		return &raw.ContentOpts{Ref: "", SHA: ref}, false, nil
	}

	originalRef := ref // Keep original ref f或clearer 错误 messages down 行.

	// 2) 如果没有SH一个is provided, we try to resolve ref into 一个fully-qualified format.
	var reference *github.Reference
	var resp *github.Response
	var err error
	var fallbackUsed bool

	switch {
	case originalRef == "":
		// 2a) If ref is 空, determine 默认分支.
		reference, err = resolveDefaultBranch(ctx, githubClient, owner, repo)
		if err != nil {
			return nil, false, err // Err或is al读取y wrapped in resolveDefaultBranch.
		}
		ref = reference.GetRef()
	case strings.HasPrefix(originalRef, "refs/"):
		// 2b) Al读取y fully qualified. reference will be fetched at end.
	case strings.HasPrefix(originalRef, "heads/") || strings.HasPrefix(originalRef, "tags/"):
		// 2c) Partially qualified. Make it fully qualified.
		ref = "refs/" + originalRef
	default:
		// 2d) It's 一个short name, so we try to resolve it to either 一个分支 或一个tag.
		branchRef := "refs/heads/" + originalRef
		reference, resp, err = githubClient.Git.GetRef(ctx, owner, repo, branchRef)

		if err == nil {
			ref = branchRef // It's 一个分支.
		} else {
			// 分支 lookup failed. Check if it was 一个404 不Found 错误.
			ghErr, isGhErr := err.(*github.ErrorResponse)
			if isGhErr && ghErr.Response.StatusCode == http.StatusNotFound {
				tagRef := "refs/tags/" + originalRef
				reference, resp, err = githubClient.Git.GetRef(ctx, owner, repo, tagRef)
				if err == nil {
					ref = tagRef // It's 一个tag.
				} else {
					// tag lookup 也failed. Check if it was 一个404 不Found 错误.
					ghErr2, isGhErr2 := err.(*github.ErrorResponse)
					if isGhErr2 && ghErr2.Response.StatusCode == http.StatusNotFound {
						if originalRef == "main" {
							reference, err = resolveDefaultBranch(ctx, githubClient, owner, repo)
							if err != nil {
								return nil, false, err // Err或is al读取y wrapped in resolveDefaultBranch.
							}
							// Update ref 到actual 默认分支 ref so note 可以 generated
							ref = reference.GetRef()
							fallbackUsed = true
							break
						}
						return nil, false, fmt.Errorf("could not resolve ref %q as a branch or a tag", originalRef)
					}

					// tag lookup failed f或一个different reason.
					_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get reference (tag)", resp, err)
					return nil, false, fmt.Errorf("failed to get reference for tag '%s': %w", originalRef, err)
				}
			} else {
				// 分支 lookup failed f或一个different reason.
				_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get reference (branch)", resp, err)
				return nil, false, fmt.Errorf("failed to get reference for branch '%s': %w", originalRef, err)
			}
		}
	}

	if reference == nil {
		reference, resp, err = githubClient.Git.GetRef(ctx, owner, repo, ref)
		if err != nil {
			if ref == "refs/heads/main" {
				reference, err = resolveDefaultBranch(ctx, githubClient, owner, repo)
				if err != nil {
					return nil, false, err // Err或is al读取y wrapped in resolveDefaultBranch.
				}
				// Update ref 到actual 默认分支 ref so note 可以 generated
				ref = reference.GetRef()
				fallbackUsed = true
			} else {
				_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get final reference", resp, err)
				return nil, false, fmt.Errorf("failed to get final reference for %q: %w", ref, err)
			}
		}
	}

	sha = reference.GetObject().GetSHA()
	return &raw.ContentOpts{Ref: ref, SHA: sha}, fallbackUsed, nil
}

func resolveDefaultBranch(ctx context.Context, githubClient *github.Client, owner, repo string) (*github.Reference, error) {
	repoInfo, resp, err := githubClient.Repositories.Get(ctx, owner, repo)
	if err != nil {
		_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get repository info", resp, err)
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	defaultBranch := repoInfo.GetDefaultBranch()

	defaultRef, resp, err := githubClient.Git.GetRef(ctx, owner, repo, "heads/"+defaultBranch)
	if err != nil {
		_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get default branch reference", resp, err)
		return nil, fmt.Errorf("failed to get default branch reference: %w", err)
	}

	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	return defaultRef, nil
}
