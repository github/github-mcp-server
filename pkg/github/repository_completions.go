package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
)

// CompletionHandlerFunc is a function that handles completion requests.
type CompletionHandlerFunc func(ctx context.Context, request mcp.CompleteRequest) (*mcp.CompleteResult, error)

// RepositoryCompletionHandler handles completion requests for repository resources.
func RepositoryCompletionHandler(getClient GetClientFn) CompletionHandlerFunc {
	return func(ctx context.Context, request mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		// Extract the resource reference
		ref, ok := request.Params.Ref.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid ref type")
		}

		refType, ok := ref["type"].(string)
		if !ok || refType != "ref/resource" {
			return nil, fmt.Errorf("unsupported ref type: %s", refType)
		}

		uri, ok := ref["uri"].(string)
		if !ok {
			return nil, fmt.Errorf("missing uri in resource reference")
		}

		// Only handle repo:// URIs
		if !strings.HasPrefix(uri, "repo://") {
			return &mcp.CompleteResult{
				Completion: struct {
					Values  []string `json:"values"`
					Total   int      `json:"total,omitempty"`
					HasMore bool     `json:"hasMore,omitempty"`
				}{
					Values: []string{},
					Total:  0,
				},
			}, nil
		}

		argumentName := request.Params.Argument.Name
		argumentValue := request.Params.Argument.Value

		switch argumentName {
		case "owner":
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			return completeOwner(ctx, client, argumentValue)
		case "repo":
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			return completeRepo(ctx, client, argumentValue, uri)
		case "branch":
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			return completeBranch(ctx, client, argumentValue, uri)
		case "sha":
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			return completeCommit(ctx, client, argumentValue, uri)
		case "tag":
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			return completeTag(ctx, client, argumentValue, uri)
		case "prNumber":
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			return completePullRequest(ctx, client, argumentValue, uri)
		case "path":
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			return completePath(ctx, client, argumentValue, uri)
		default:
			// Return empty completion for unsupported arguments
			return &mcp.CompleteResult{
				Completion: struct {
					Values  []string `json:"values"`
					Total   int      `json:"total,omitempty"`
					HasMore bool     `json:"hasMore,omitempty"`
				}{
					Values: []string{},
					Total:  0,
				},
			}, nil
		}
	}
}

// completeOwner provides completions for repository owners (users/orgs)
func completeOwner(ctx context.Context, client *github.Client, value string) (*mcp.CompleteResult, error) {
	if value == "" {
		// Return empty completion for now - could add popular orgs/users here
		return &mcp.CompleteResult{
			Completion: struct {
				Values  []string `json:"values"`
				Total   int      `json:"total,omitempty"`
				HasMore bool     `json:"hasMore,omitempty"`
			}{
				Values: []string{},
				Total:  0,
			},
		}, nil
	}

	// Search for users/organizations
	query := fmt.Sprintf("%s in:login", value)
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 10,
		},
	}

	result, _, err := client.Search.Users(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}

	var values []string
	for _, user := range result.Users {
		if user.Login != nil && strings.HasPrefix(strings.ToLower(*user.Login), strings.ToLower(value)) {
			values = append(values, *user.Login)
		}
	}

	total := 0
	hasMore := false
	if result.Total != nil {
		total = *result.Total
		hasMore = total > len(values)
	}

	return &mcp.CompleteResult{
		Completion: struct {
			Values  []string `json:"values"`
			Total   int      `json:"total,omitempty"`
			HasMore bool     `json:"hasMore,omitempty"`
		}{
			Values:  values,
			Total:   total,
			HasMore: hasMore,
		},
	}, nil
}

// completeRepo provides completions for repository names
func completeRepo(ctx context.Context, client *github.Client, value string, uri string) (*mcp.CompleteResult, error) {
	// Extract owner from URI
	owner := extractOwnerFromURI(uri)
	if owner == "" {
		return &mcp.CompleteResult{
			Completion: struct {
				Values  []string `json:"values"`
				Total   int      `json:"total,omitempty"`
				HasMore bool     `json:"hasMore,omitempty"`
			}{
				Values: []string{},
				Total:  0,
			},
		}, nil
	}

	// Search for repositories
	query := fmt.Sprintf("user:%s %s in:name", owner, value)
	if value == "" {
		query = fmt.Sprintf("user:%s", owner)
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 10,
		},
	}

	result, _, err := client.Search.Repositories(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search repositories: %w", err)
	}

	var values []string
	for _, repo := range result.Repositories {
		if repo.Name != nil && strings.HasPrefix(strings.ToLower(*repo.Name), strings.ToLower(value)) {
			values = append(values, *repo.Name)
		}
	}

	total := 0
	hasMore := false
	if result.Total != nil {
		total = *result.Total
		hasMore = total > len(values)
	}

	return &mcp.CompleteResult{
		Completion: struct {
			Values  []string `json:"values"`
			Total   int      `json:"total,omitempty"`
			HasMore bool     `json:"hasMore,omitempty"`
		}{
			Values:  values,
			Total:   total,
			HasMore: hasMore,
		},
	}, nil
}

// completeBranch provides completions for branch names
func completeBranch(ctx context.Context, client *github.Client, value string, uri string) (*mcp.CompleteResult, error) {
	owner, repo := extractOwnerRepoFromURI(uri)
	if owner == "" || repo == "" {
		return &mcp.CompleteResult{
			Completion: struct {
				Values  []string `json:"values"`
				Total   int      `json:"total,omitempty"`
				HasMore bool     `json:"hasMore,omitempty"`
			}{
				Values: []string{},
				Total:  0,
			},
		}, nil
	}

	// List branches
	opts := &github.BranchListOptions{
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 30,
		},
	}

	branches, _, err := client.Repositories.ListBranches(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var values []string
	for _, branch := range branches {
		if branch.Name != nil && strings.HasPrefix(strings.ToLower(*branch.Name), strings.ToLower(value)) {
			values = append(values, *branch.Name)
		}
	}

	return &mcp.CompleteResult{
		Completion: struct {
			Values  []string `json:"values"`
			Total   int      `json:"total,omitempty"`
			HasMore bool     `json:"hasMore,omitempty"`
		}{
			Values:  values,
			Total:   len(values),
			HasMore: len(branches) >= 30, // Might have more
		},
	}, nil
}

// completeCommit provides completions for commit SHAs
func completeCommit(ctx context.Context, client *github.Client, value string, uri string) (*mcp.CompleteResult, error) {
	owner, repo := extractOwnerRepoFromURI(uri)
	if owner == "" || repo == "" {
		return &mcp.CompleteResult{
			Completion: struct {
				Values  []string `json:"values"`
				Total   int      `json:"total,omitempty"`
				HasMore bool     `json:"hasMore,omitempty"`
			}{
				Values: []string{},
				Total:  0,
			},
		}, nil
	}

	// If user has typed some characters, search for commits
	if len(value) >= 3 {
		// List recent commits
		opts := &github.CommitsListOptions{
			ListOptions: github.ListOptions{
				Page:    1,
				PerPage: 10,
			},
		}

		commits, _, err := client.Repositories.ListCommits(ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list commits: %w", err)
		}

		var values []string
		for _, commit := range commits {
			if commit.SHA != nil && strings.HasPrefix(strings.ToLower(*commit.SHA), strings.ToLower(value)) {
				values = append(values, *commit.SHA)
			}
		}

		return &mcp.CompleteResult{
			Completion: struct {
				Values  []string `json:"values"`
				Total   int      `json:"total,omitempty"`
				HasMore bool     `json:"hasMore,omitempty"`
			}{
				Values:  values,
				Total:   len(values),
				HasMore: len(commits) >= 10,
			},
		}, nil
	}

	// For short prefixes, return empty completion
	return &mcp.CompleteResult{
		Completion: struct {
			Values  []string `json:"values"`
			Total   int      `json:"total,omitempty"`
			HasMore bool     `json:"hasMore,omitempty"`
		}{
			Values: []string{},
			Total:  0,
		},
	}, nil
}

// completeTag provides completions for tag names
func completeTag(ctx context.Context, client *github.Client, value string, uri string) (*mcp.CompleteResult, error) {
	owner, repo := extractOwnerRepoFromURI(uri)
	if owner == "" || repo == "" {
		return &mcp.CompleteResult{
			Completion: struct {
				Values  []string `json:"values"`
				Total   int      `json:"total,omitempty"`
				HasMore bool     `json:"hasMore,omitempty"`
			}{
				Values: []string{},
				Total:  0,
			},
		}, nil
	}

	// List tags
	opts := &github.ListOptions{
		Page:    1,
		PerPage: 30,
	}

	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	var values []string
	for _, tag := range tags {
		if tag.Name != nil && strings.HasPrefix(strings.ToLower(*tag.Name), strings.ToLower(value)) {
			values = append(values, *tag.Name)
		}
	}

	return &mcp.CompleteResult{
		Completion: struct {
			Values  []string `json:"values"`
			Total   int      `json:"total,omitempty"`
			HasMore bool     `json:"hasMore,omitempty"`
		}{
			Values:  values,
			Total:   len(values),
			HasMore: len(tags) >= 30,
		},
	}, nil
}

// completePullRequest provides completions for pull request numbers
func completePullRequest(ctx context.Context, client *github.Client, value string, uri string) (*mcp.CompleteResult, error) {
	owner, repo := extractOwnerRepoFromURI(uri)
	if owner == "" || repo == "" {
		return &mcp.CompleteResult{
			Completion: struct {
				Values  []string `json:"values"`
				Total   int      `json:"total,omitempty"`
				HasMore bool     `json:"hasMore,omitempty"`
			}{
				Values: []string{},
				Total:  0,
			},
		}, nil
	}

	// List pull requests
	opts := &github.PullRequestListOptions{
		State: "all",
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: 20,
		},
	}

	prs, _, err := client.PullRequests.List(ctx, owner, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests: %w", err)
	}

	var values []string
	for _, pr := range prs {
		if pr.Number != nil {
			prNumber := fmt.Sprintf("%d", *pr.Number)
			if strings.HasPrefix(prNumber, value) {
				values = append(values, prNumber)
			}
		}
	}

	return &mcp.CompleteResult{
		Completion: struct {
			Values  []string `json:"values"`
			Total   int      `json:"total,omitempty"`
			HasMore bool     `json:"hasMore,omitempty"`
		}{
			Values:  values,
			Total:   len(values),
			HasMore: len(prs) >= 20,
		},
	}, nil
}

// completePath provides completions for file/directory paths
func completePath(ctx context.Context, client *github.Client, value string, uri string) (*mcp.CompleteResult, error) {
	owner, repo := extractOwnerRepoFromURI(uri)
	if owner == "" || repo == "" {
		return &mcp.CompleteResult{
			Completion: struct {
				Values  []string `json:"values"`
				Total   int      `json:"total,omitempty"`
				HasMore bool     `json:"hasMore,omitempty"`
			}{
				Values: []string{},
				Total:  0,
			},
		}, nil
	}

	// Determine the directory to list based on the current path value
	var dirPath string
	var prefix string
	
	if value == "" {
		dirPath = ""
		prefix = ""
	} else if strings.HasSuffix(value, "/") {
		dirPath = strings.TrimSuffix(value, "/")
		prefix = ""
	} else {
		lastSlash := strings.LastIndex(value, "/")
		if lastSlash == -1 {
			dirPath = ""
			prefix = value
		} else {
			dirPath = value[:lastSlash]
			prefix = value[lastSlash+1:]
		}
	}

	// Get repository contents for the directory
	opts := &github.RepositoryContentGetOptions{}
	
	// Extract ref if present in URI (branch, commit, etc.)
	if ref := extractRefFromURI(uri); ref != "" {
		opts.Ref = ref
	}

	_, directoryContent, _, err := client.Repositories.GetContents(ctx, owner, repo, dirPath, opts)
	if err != nil {
		// If directory doesn't exist, return empty completion
		return &mcp.CompleteResult{
			Completion: struct {
				Values  []string `json:"values"`
				Total   int      `json:"total,omitempty"`
				HasMore bool     `json:"hasMore,omitempty"`
			}{
				Values: []string{},
				Total:  0,
			},
		}, nil
	}

	var values []string
	for _, entry := range directoryContent {
		if entry.Name != nil && strings.HasPrefix(strings.ToLower(*entry.Name), strings.ToLower(prefix)) {
			entryPath := *entry.Name
			if dirPath != "" {
				entryPath = dirPath + "/" + entryPath
			}
			
			// Add trailing slash for directories
			if entry.Type != nil && *entry.Type == "dir" {
				entryPath += "/"
			}
			
			values = append(values, entryPath)
		}
	}

	return &mcp.CompleteResult{
		Completion: struct {
			Values  []string `json:"values"`
			Total   int      `json:"total,omitempty"`
			HasMore bool     `json:"hasMore,omitempty"`
		}{
			Values:  values,
			Total:   len(values),
			HasMore: false,
		},
	}, nil
}

// Helper functions to extract information from URI

// extractOwnerFromURI extracts the owner from a repo:// URI
func extractOwnerFromURI(uri string) string {
	// Parse URI like repo://{owner}/{repo}/...
	if !strings.HasPrefix(uri, "repo://") {
		return ""
	}
	
	path := strings.TrimPrefix(uri, "repo://")
	parts := strings.Split(path, "/")
	if len(parts) > 0 && strings.Contains(parts[0], "{owner}") {
		return "" // Template not filled
	}
	if len(parts) > 0 {
		return parts[0]
	}
	
	return ""
}

// extractOwnerRepoFromURI extracts owner and repo from a repo:// URI
func extractOwnerRepoFromURI(uri string) (string, string) {
	if !strings.HasPrefix(uri, "repo://") {
		return "", ""
	}
	
	path := strings.TrimPrefix(uri, "repo://")
	parts := strings.Split(path, "/")
	
	if len(parts) >= 2 {
		owner := parts[0]
		repo := parts[1]
		
		// Skip if still templates
		if strings.Contains(owner, "{") || strings.Contains(repo, "{") {
			return "", ""
		}
		
		return owner, repo
	}
	
	return "", ""
}

// extractRefFromURI extracts the ref (branch, commit, tag) from a repo:// URI
func extractRefFromURI(uri string) string {
	if !strings.HasPrefix(uri, "repo://") {
		return ""
	}
	
	path := strings.TrimPrefix(uri, "repo://")
	
	// Look for patterns like /refs/heads/{branch}, /sha/{sha}, /refs/tags/{tag}, /refs/pull/{prNumber}/head
	if strings.Contains(path, "/refs/heads/") {
		parts := strings.Split(path, "/refs/heads/")
		if len(parts) > 1 {
			branchPart := strings.Split(parts[1], "/")[0]
			if !strings.Contains(branchPart, "{") {
				return "refs/heads/" + branchPart
			}
		}
	} else if strings.Contains(path, "/sha/") {
		parts := strings.Split(path, "/sha/")
		if len(parts) > 1 {
			shaPart := strings.Split(parts[1], "/")[0]
			if !strings.Contains(shaPart, "{") {
				return shaPart
			}
		}
	} else if strings.Contains(path, "/refs/tags/") {
		parts := strings.Split(path, "/refs/tags/")
		if len(parts) > 1 {
			tagPart := strings.Split(parts[1], "/")[0]
			if !strings.Contains(tagPart, "{") {
				return "refs/tags/" + tagPart
			}
		}
	} else if strings.Contains(path, "/refs/pull/") && strings.Contains(path, "/head") {
		parts := strings.Split(path, "/refs/pull/")
		if len(parts) > 1 {
			prPart := strings.Split(parts[1], "/head")[0]
			if !strings.Contains(prPart, "{") {
				return "refs/pull/" + prPart + "/head"
			}
		}
	}
	
	return ""
}