package github

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SyncLocalRepository creates a tool to sync a local directory to a GitHub repository.
// This tool reads files from the local filesystem and pushes them to GitHub.
func SyncLocalRepository(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("sync_local_repository",
			mcp.WithDescription(t("TOOL_SYNC_LOCAL_REPO_DESCRIPTION", "Sync a local directory to a GitHub repository. Reads local files and pushes them to GitHub in a single commit. Respects .gitignore patterns.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_SYNC_LOCAL_REPO_USER_TITLE", "Sync local directory to GitHub"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("local_path",
				mcp.Required(),
				mcp.Description("Absolute path to the local directory to sync"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner (username or organization)"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("branch",
				mcp.Description("Branch to push to (default: main)"),
			),
			mcp.WithString("message",
				mcp.Required(),
				mcp.Description("Commit message"),
			),
			mcp.WithBoolean("create_repo",
				mcp.Description("Create the repository if it doesn't exist (default: false)"),
			),
			mcp.WithBoolean("private",
				mcp.Description("Make the repository private if creating (default: true)"),
			),
			mcp.WithNumber("max_files",
				mcp.Description("Maximum number of files to sync (default: 100, max: 500)"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Parse parameters
			localPath, err := RequiredParam[string](request, "local_path")
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
			message, err := RequiredParam[string](request, "message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			branch, err := OptionalParam[string](request, "branch")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if branch == "" {
				branch = "main"
			}

			createRepo, err := OptionalBoolParamWithDefault(request, "create_repo", false)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			private, err := OptionalBoolParamWithDefault(request, "private", true)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			maxFilesFloat, err := OptionalParam[float64](request, "max_files")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			maxFiles := 100
			if maxFilesFloat > 0 {
				maxFiles = int(maxFilesFloat)
				if maxFiles > 500 {
					maxFiles = 500
				}
			}

			// Validate local path exists
			info, err := os.Stat(localPath)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("local path does not exist: %s", localPath)), nil
			}
			if !info.IsDir() {
				return mcp.NewToolResultError(fmt.Sprintf("local path is not a directory: %s", localPath)), nil
			}

			// Get GitHub client
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Check if repository exists
			_, resp, err := client.Repositories.Get(ctx, owner, repo)
			repoExists := err == nil && resp.StatusCode == 200
			if resp != nil {
				_ = resp.Body.Close()
			}

			// Create repository if needed
			if !repoExists {
				if !createRepo {
					return mcp.NewToolResultError(fmt.Sprintf("repository %s/%s does not exist. Set create_repo=true to create it.", owner, repo)), nil
				}

				newRepo := &github.Repository{
					Name:    github.Ptr(repo),
					Private: github.Ptr(private),
					AutoInit: github.Ptr(true), // Initialize with README to create default branch
				}
				_, resp, err := client.Repositories.Create(ctx, "", newRepo)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to create repository",
						resp,
						err,
					), nil
				}
				if resp != nil {
					_ = resp.Body.Close()
				}
			}

			// Load gitignore patterns
			ignorePatterns := loadGitignorePatterns(localPath)

			// Collect files to sync
			var files []fileEntry
			err = filepath.WalkDir(localPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				// Get relative path
				relPath, err := filepath.Rel(localPath, path)
				if err != nil {
					return err
				}

				// Convert to forward slashes for GitHub
				relPath = filepath.ToSlash(relPath)

				// Skip root
				if relPath == "." {
					return nil
				}

				// Skip .git directory
				if strings.HasPrefix(relPath, ".git/") || relPath == ".git" {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				// Check gitignore patterns
				if shouldIgnore(relPath, d.IsDir(), ignorePatterns) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				// Skip directories (we only push files)
				if d.IsDir() {
					return nil
				}

				// Check file count limit
				if len(files) >= maxFiles {
					return fmt.Errorf("exceeded maximum file count (%d). Increase max_files or reduce directory size", maxFiles)
				}

				// Read file content
				content, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %w", relPath, err)
				}

				// Skip binary files (simple heuristic: check for null bytes)
				if isBinaryContent(content) {
					return nil // Skip binary files silently
				}

				files = append(files, fileEntry{
					Path:    relPath,
					Content: string(content),
				})

				return nil
			})

			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to collect files: %s", err.Error())), nil
			}

			if len(files) == 0 {
				return mcp.NewToolResultError("no files found to sync"), nil
			}

			// Get the reference for the branch
			ref, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get branch reference '%s'. Make sure the branch exists.", branch),
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
			for _, file := range files {
				entries = append(entries, &github.TreeEntry{
					Path:    github.Ptr(file.Path),
					Mode:    github.Ptr("100644"), // Regular file mode
					Type:    github.Ptr("blob"),
					Content: github.Ptr(file.Content),
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
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Update the branch reference
			ref.Object.SHA = newCommit.SHA
			_, resp, err = client.Git.UpdateRef(ctx, owner, repo, *ref.Ref, github.UpdateRef{
				SHA:   *newCommit.SHA,
				Force: github.Ptr(false),
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to update branch reference",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			// Return success result
			result := map[string]interface{}{
				"success":      true,
				"commit_sha":   *newCommit.SHA,
				"files_synced": len(files),
				"repository":   fmt.Sprintf("https://github.com/%s/%s", owner, repo),
				"branch":       branch,
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

type fileEntry struct {
	Path    string
	Content string
}

// loadGitignorePatterns loads patterns from .gitignore file
func loadGitignorePatterns(rootPath string) []string {
	gitignorePath := filepath.Join(rootPath, ".gitignore")
	file, err := os.Open(gitignorePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// shouldIgnore checks if a path should be ignored based on gitignore patterns
func shouldIgnore(path string, isDir bool, patterns []string) bool {
	// Always ignore common patterns
	defaultIgnore := []string{
		"node_modules",
		".git",
		"__pycache__",
		".pyc",
		".pyo",
		".exe",
		".dll",
		".so",
		".dylib",
		".DS_Store",
		"Thumbs.db",
		".env",
		".env.local",
		"dist",
		"build",
		".next",
		".nuxt",
		"vendor",
		".idea",
		".vscode",
	}

	// Check default ignores
	baseName := filepath.Base(path)
	for _, pattern := range defaultIgnore {
		if baseName == pattern || strings.HasSuffix(path, pattern) {
			return true
		}
		if strings.Contains(path, pattern+"/") {
			return true
		}
	}

	// Check gitignore patterns (simplified matching)
	for _, pattern := range patterns {
		// Remove leading slash
		pattern = strings.TrimPrefix(pattern, "/")

		// Handle negation (not fully supported)
		if strings.HasPrefix(pattern, "!") {
			continue
		}

		// Handle directory-only patterns
		if strings.HasSuffix(pattern, "/") {
			pattern = strings.TrimSuffix(pattern, "/")
			if !isDir {
				continue
			}
		}

		// Simple glob matching
		if matched, _ := filepath.Match(pattern, baseName); matched {
			return true
		}

		// Check if pattern matches path
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return false
}

// isBinaryContent checks if content appears to be binary
func isBinaryContent(content []byte) bool {
	// Check first 8000 bytes for null bytes (common heuristic)
	checkLen := len(content)
	if checkLen > 8000 {
		checkLen = 8000
	}

	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}
