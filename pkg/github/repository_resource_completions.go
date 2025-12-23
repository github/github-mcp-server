package github

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CompleteHandler defines function signature for completion handlers
type CompleteHandler func(ctx context.Context, client *github.Client, resolved map[string]string, argValue string) ([]string, error)

// RepositoryResourceArgumentResolvers is a map of argument names to their completion handlers
var RepositoryResourceArgumentResolvers = map[string]CompleteHandler{
	"owner":    completeOwner,
	"repo":     completeRepo,
	"branch":   completeBranch,
	"sha":      completeSHA,
	"tag":      completeTag,
	"prNumber": completePRNumber,
	"path":     completePath,
}

// RepositoryResourceCompletionHandler returns a CompletionHandlerFunc for repository resource completions.
func RepositoryResourceCompletionHandler(getClient GetClientFn) func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		if req.Params.Ref.Type != "ref/resource" {
			return nil, nil // Not a resource completion
		}

		argName := req.Params.Argument.Name
		argValue := req.Params.Argument.Value
		resolved := req.Params.Context.Arguments
		if resolved == nil {
			resolved = map[string]string{}
		}

		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}

		// Argument resolver functions
		resolvers := RepositoryResourceArgumentResolvers

		resolver, ok := resolvers[argName]
		if !ok {
			return nil, errors.New("no resolver for argument: " + argName)
		}

		values, err := resolver(ctx, client, resolved, argValue)
		if err != nil {
			return nil, err
		}
		if len(values) > 100 {
			values = values[:100]
		}

		return &mcp.CompleteResult{
			Completion: mcp.CompletionResultDetails{
				Values:  values,
				Total:   len(values),
				HasMore: false,
			},
		}, nil
	}
}

// --- Per-argument resolver functions ---

func completeOwner(ctx context.Context, client *github.Client, _ map[string]string, argValue string) ([]string, error) {
	var values []string
	user, _, err := client.Users.Get(ctx, "")
	if err == nil && user.GetLogin() != "" {
		values = append(values, user.GetLogin())
	}

	orgs, _, err := client.Organizations.List(ctx, "", &github.ListOptions{PerPage: 100})
	if err != nil {
		return nil, err
	}
	for _, org := range orgs {
		values = append(values, org.GetLogin())
	}

	// filter values based on argValue and replace values slice
	if argValue != "" {
		var filteredValues []string
		for _, value := range values {
			if strings.Contains(value, argValue) {
				filteredValues = append(filteredValues, value)
			}
		}
		values = filteredValues
	}
	if len(values) > 100 {
		values = values[:100]
		return values, nil // Limit to 100 results
	}
	// Else also do a client.Search.Users()
	if argValue == "" {
		return values, nil // No need to search if no argValue
	}
	users, _, err := client.Search.Users(ctx, argValue, &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 100 - len(values)}})
	if err != nil || users == nil {
		return nil, err
	}
	for _, user := range users.Users {
		values = append(values, user.GetLogin())
	}

	if len(values) > 100 {
		values = values[:100]
	}
	return values, nil
}

func completeRepo(ctx context.Context, client *github.Client, resolved map[string]string, argValue string) ([]string, error) {
	var values []string
	owner := resolved["owner"]
	if owner == "" {
		return values, errors.New("owner not specified")
	}

	query := fmt.Sprintf("org:%s", owner)

	if argValue != "" {
		query = fmt.Sprintf("%s %s", query, argValue)
	}
	repos, _, err := client.Search.Repositories(ctx, query, &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 100}})
	if err != nil || repos == nil {
		return values, errors.New("failed to get repositories")
	}
	// filter repos based on argValue
	for _, repo := range repos.Repositories {
		name := repo.GetName()
		if argValue == "" || strings.HasPrefix(name, argValue) {
			values = append(values, name)
		}
	}

	return values, nil
}

func completeBranch(ctx context.Context, client *github.Client, resolved map[string]string, argValue string) ([]string, error) {
	var values []string
	owner := resolved["owner"]
	repo := resolved["repo"]
	if owner == "" || repo == "" {
		return values, errors.New("owner or repo not specified")
	}
	branches, _, _ := client.Repositories.ListBranches(ctx, owner, repo, nil)

	for _, branch := range branches {
		if argValue == "" || strings.HasPrefix(branch.GetName(), argValue) {
			values = append(values, branch.GetName())
		}
	}
	if len(values) > 100 {
		values = values[:100]
	}
	return values, nil
}

func completeSHA(ctx context.Context, client *github.Client, resolved map[string]string, argValue string) ([]string, error) {
	var values []string
	owner := resolved["owner"]
	repo := resolved["repo"]
	if owner == "" || repo == "" {
		return values, errors.New("owner or repo not specified")
	}
	commits, _, _ := client.Repositories.ListCommits(ctx, owner, repo, nil)

	for _, commit := range commits {
		sha := commit.GetSHA()
		if argValue == "" || strings.HasPrefix(sha, argValue) {
			values = append(values, sha)
		}
	}
	if len(values) > 100 {
		values = values[:100]
	}
	return values, nil
}

func completeTag(ctx context.Context, client *github.Client, resolved map[string]string, argValue string) ([]string, error) {
	owner := resolved["owner"]
	repo := resolved["repo"]
	if owner == "" || repo == "" {
		return nil, errors.New("owner or repo not specified")
	}
	tags, _, _ := client.Repositories.ListTags(ctx, owner, repo, nil)
	var values []string
	for _, tag := range tags {
		if argValue == "" || strings.Contains(tag.GetName(), argValue) {
			values = append(values, tag.GetName())
		}
	}
	if len(values) > 100 {
		values = values[:100]
	}
	return values, nil
}

func completePRNumber(ctx context.Context, client *github.Client, resolved map[string]string, argValue string) ([]string, error) {
	var values []string
	owner := resolved["owner"]
	repo := resolved["repo"]
	if owner == "" || repo == "" {
		return values, errors.New("owner or repo not specified")
	}

	prs, _, err := client.Search.Issues(ctx, fmt.Sprintf("repo:%s/%s is:open is:pr", owner, repo), &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 100}})
	if err != nil {
		return values, err
	}
	for _, pr := range prs.Issues {
		num := fmt.Sprintf("%d", pr.GetNumber())
		if argValue == "" || strings.HasPrefix(num, argValue) {
			values = append(values, num)
		}
	}
	if len(values) > 100 {
		values = values[:100]
	}
	return values, nil
}

func completePath(ctx context.Context, client *github.Client, resolved map[string]string, argValue string) ([]string, error) {
	owner := resolved["owner"]
	repo := resolved["repo"]
	if owner == "" || repo == "" {
		return nil, errors.New("owner or repo not specified")
	}
	refVal := resolved["branch"]
	if refVal == "" {
		refVal = resolved["sha"]
	}
	if refVal == "" {
		refVal = resolved["tag"]
	}
	if refVal == "" {
		refVal = "HEAD"
	}

	// Determine the prefix to complete (directory path or file path)
	prefix := argValue
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		lastSlash := strings.LastIndex(prefix, "/")
		if lastSlash >= 0 {
			prefix = prefix[:lastSlash+1]
		} else {
			prefix = ""
		}
	}

	// Get the tree for the ref (recursive)
	tree, _, err := client.Git.GetTree(ctx, owner, repo, refVal, true)
	if err != nil || tree == nil {
		return nil, errors.New("failed to get file tree")
	}

	// Collect immediate children of the prefix (files and directories, no duplicates)
	dirs := map[string]struct{}{}
	files := map[string]struct{}{}
	prefixLen := len(prefix)
	for _, entry := range tree.Entries {
		if !strings.HasPrefix(entry.GetPath(), prefix) {
			continue
		}
		rel := entry.GetPath()[prefixLen:]
		if rel == "" {
			continue
		}
		// Only immediate children
		slashIdx := strings.Index(rel, "/")
		if slashIdx >= 0 {
			// Directory: only add the directory name (with trailing slash), prefixed with full path
			dirName := prefix + rel[:slashIdx+1]
			dirs[dirName] = struct{}{}
		} else if entry.GetType() == "blob" {
			// File: add as-is, prefixed with full path
			fileName := prefix + rel
			files[fileName] = struct{}{}
		}
	}

	// Optionally filter by argValue (if user is typing after last slash)
	var filter string
	if argValue != "" {
		if lastSlash := strings.LastIndex(argValue, "/"); lastSlash >= 0 {
			filter = argValue[lastSlash+1:]
		} else {
			filter = argValue
		}
	}

	var values []string
	// Add directories first, then files, both filtered
	for dir := range dirs {
		// Only filter on the last segment after the last slash
		if filter == "" {
			values = append(values, dir)
		} else {
			last := dir
			if idx := strings.LastIndex(strings.TrimRight(dir, "/"), "/"); idx >= 0 {
				last = dir[idx+1:]
			}
			if strings.HasPrefix(last, filter) {
				values = append(values, dir)
			}
		}
	}
	for file := range files {
		if filter == "" {
			values = append(values, file)
		} else {
			last := file
			if idx := strings.LastIndex(file, "/"); idx >= 0 {
				last = file[idx+1:]
			}
			if strings.HasPrefix(last, filter) {
				values = append(values, file)
			}
		}
	}

	if len(values) > 100 {
		values = values[:100]
	}
	return values, nil
}
