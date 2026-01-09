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
	"github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// initializeRepository creates an initial commit in an empty repository and returns the default branch ref and base commit
func initializeRepository(ctx context.Context, client *github.Client, owner, repo string) (ref *github.Reference, baseCommit *github.Commit, err error) {
	// First, we need to check what the default branch in this empty repo should be:
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

	// Create an initial empty commit to create the default branch
	createResp, resp, err := client.Repositories.CreateFile(ctx, owner, repo, "README.md", fileOpts)
	if err != nil {
		_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to create initial file", resp, err)
		return nil, nil, fmt.Errorf("failed to create initial file: %w", err)
	}
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	// Get the commit that was just created to use as base for remaining files
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

// createReferenceFromDefaultBranch creates a new branch reference from the repository's default branch
func createReferenceFromDefaultBranch(ctx context.Context, client *github.Client, owner, repo, branch string) (*github.Reference, error) {
	defaultRef, err := resolveDefaultBranch(ctx, client, owner, repo)
	if err != nil {
		_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to resolve default branch", nil, err)
		return nil, fmt.Errorf("failed to resolve default branch: %w", err)
	}

	// Create the new branch reference
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

// matchFiles searches for files in the Git tree that match the given path.
// It's used when GetContents fails or returns unexpected results.
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

	// Step 2: Filter tree for matching paths
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

// looksLikeSHA returns true if the string appears to be a Git commit SHA.
// A SHA is a 40-character hexadecimal string.
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

// resolveGitReference takes a user-provided ref and sha and resolves them into a
// definitive commit SHA and its corresponding fully-qualified reference.
//
// The resolution logic follows a clear priority:
//
//  1. If a specific commit `sha` is provided, it takes precedence and is used directly,
//     and all reference resolution is skipped.
//
//     1a. If `sha` is empty but `ref` looks like a commit SHA (40 hexadecimal characters),
//     it is returned as-is without any API calls or reference resolution.
//
//  2. If no `sha` is provided and `ref` does not look like a SHA, the function resolves
//     the `ref` string into a fully-qualified format (e.g., "refs/heads/main") by trying
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
func resolveGitReference(ctx context.Context, githubClient *github.Client, owner, repo, ref, sha string) (*raw.ContentOpts, bool, error) {
	// 1) If SHA explicitly provided, it's the highest priority.
	if sha != "" {
		return &raw.ContentOpts{Ref: "", SHA: sha}, false, nil
	}

	// 1a) If sha is empty but ref looks like a SHA, return it without changes
	if looksLikeSHA(ref) {
		return &raw.ContentOpts{Ref: "", SHA: ref}, false, nil
	}

	originalRef := ref // Keep original ref for clearer error messages down the line.

	// 2) If no SHA is provided, we try to resolve the ref into a fully-qualified format.
	var reference *github.Reference
	var resp *github.Response
	var err error
	var fallbackUsed bool

	switch {
	case originalRef == "":
		// 2a) If ref is empty, determine the default branch.
		reference, err = resolveDefaultBranch(ctx, githubClient, owner, repo)
		if err != nil {
			return nil, false, err // Error is already wrapped in resolveDefaultBranch.
		}
		ref = reference.GetRef()
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
						if originalRef == "main" {
							reference, err = resolveDefaultBranch(ctx, githubClient, owner, repo)
							if err != nil {
								return nil, false, err // Error is already wrapped in resolveDefaultBranch.
							}
							// Update ref to the actual default branch ref so the note can be generated
							ref = reference.GetRef()
							fallbackUsed = true
							break
						}
						return nil, false, fmt.Errorf("could not resolve ref %q as a branch or a tag", originalRef)
					}

					// The tag lookup failed for a different reason.
					_, _ = ghErrors.NewGitHubAPIErrorToCtx(ctx, "failed to get reference (tag)", resp, err)
					return nil, false, fmt.Errorf("failed to get reference for tag '%s': %w", originalRef, err)
				}
			} else {
				// The branch lookup failed for a different reason.
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
					return nil, false, err // Error is already wrapped in resolveDefaultBranch.
				}
				// Update ref to the actual default branch ref so the note can be generated
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
