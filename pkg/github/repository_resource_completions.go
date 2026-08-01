package github

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CompleteHandler defines 函数 signature f或completion 处理器s
type CompleteHandler func(ctx context.Context, client *github.Client, resolved map[string]string, argValue string) ([]string, error)

// RepositoryResourceArgumentResolvers is 一个map of 参数 names to their completion 处理器s
var RepositoryResourceArgumentResolvers = map[string]CompleteHandler{
	"owner":    completeOwner,
	"repo":     completeRepo,
	"branch":   completeBranch,
	"sha":      completeSHA,
	"tag":      completeTag,
	"prNumber": completePRNumber,
	"path":     completePath,
}

// RepositoryResourceCompletionHandler 返回 一个CompletionHandlerFunc f或仓库 资源 completions.
func RepositoryResourceCompletionHandler(getClient GetClientFn) func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		if req.Params.Ref.Type != "ref/resource" {
			return nil, nil // 不a 资源 completion
		}

		argName := req.Params.Argument.Name
		argValue := req.Params.Argument.Value
		var resolved map[string]string
		if req.Params.Context != nil && req.Params.Context.Arguments != nil {
			resolved = req.Params.Context.Arguments
		} else {
			resolved = map[string]string{}
		}

		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}

		// Argument resolver 函数s
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

// --- Per-参数 resolver 函数s ---

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

	// 筛选 值 based on argValue 和replace 值 slice
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
		return values, nil // Limit to 100 结果
	}
	// Else 也do 一个客户端.Search.Users()
	if argValue == "" {
		return values, nil // No need to search 如果没有argValue
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
	// 筛选 repos based on argValue
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

	// Determine prefix to complete (directory 路径 或文件 路径)
	prefix := argValue
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		lastSlash := strings.LastIndex(prefix, "/")
		if lastSlash >= 0 {
			prefix = prefix[:lastSlash+1]
		} else {
			prefix = ""
		}
	}

	// Get tree 用于ref (recursive)
	tree, _, err := client.Git.GetTree(ctx, owner, repo, refVal, true)
	if err != nil || tree == nil {
		return nil, errors.New("failed to get file tree")
	}

	// Collect immediate children 的prefix (文件s 和directories, no duplicates)
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
		// 仅immediate children
		slashIdx := strings.Index(rel, "/")
		if slashIdx >= 0 {
			// Directory: 仅add directory name (with trailing slash), prefixed with full 路径
			dirName := prefix + rel[:slashIdx+1]
			dirs[dirName] = struct{}{}
		} else if entry.GetType() == "blob" {
			// File: add as-is, prefixed with full 路径
			fileName := prefix + rel
			files[fileName] = struct{}{}
		}
	}

	// Optionally 筛选 by argValue (if user is typing after 最后一个 slash)
	var filter string
	if argValue != "" {
		if lastSlash := strings.LastIndex(argValue, "/"); lastSlash >= 0 {
			filter = argValue[lastSlash+1:]
		} else {
			filter = argValue
		}
	}

	var values []string
	// Add directories 第一个, 然后文件s, both 筛选ed
	for dir := range dirs {
		// 仅筛选 在最后一个 segment after 最后一个 slash
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
