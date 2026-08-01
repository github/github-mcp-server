package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/sanitize"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
)

// PullRequestRead 创建一个工具以 获取 details of 一个specific 拉取请求.
func PullRequestRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"method": {
				Type: "string",
				Description: `Action to specify what pull request data needs to be retrieved from GitHub. 
Possible options: 
 1. get - Get details of a specific pull request.
 2. get_diff - Get the diff of a pull request.
 3. get_status - Get combined commit status of a head commit in a pull request.
 4. get_files - Get the list of files changed in a pull request. Use with pagination parameters to control the number of results returned.
 5. get_commits - Get the list of commits on a pull request. Use with pagination parameters to control the number of results returned.
 6. get_review_comments - Get review threads on a pull request. Each thread contains logically grouped review comments made on the same code location during pull request reviews. Returns threads with metadata (isResolved, isOutdated, isCollapsed) and their associated comments. Use cursor-based pagination (perPage, after) to control results.
 7. get_reviews - Get the reviews on a pull request. When asked for review comments, use get_review_comments method. Use with pagination parameters to control the number of results returned.
 8. get_comments - Get comments on a pull request. Use this if user doesn't specifically want review comments. Use with pagination parameters to control the number of results returned.
 9. get_check_runs - Get check runs for the head commit of a pull request. Check runs are the individual CI/CD jobs and checks that run on the PR.
`,
				Enum: []any{"get", "get_diff", "get_status", "get_files", "get_commits", "get_review_comments", "get_reviews", "get_comments", "get_check_runs"},
			},
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"pullNumber": {
				Type:        "number",
				Description: "Pull request number",
			},
		},
		Required: []string{"method", "owner", "repo", "pullNumber"},
	}
	WithPagination(schema)
	// 获取_review_comments uses GraphQL cursor-based pagination 和accepts the
	// `after` cursor. Other methods rely 在`页`/`perPage` 参数
	// added by WithPagination 和ignore `after`.
	schema.Properties["after"] = &jsonschema.Schema{
		Type:        "string",
		Description: "Cursor for pagination, used only by the get_review_comments method. Pass the endCursor from the previous page's PageInfo to fetch the next page.",
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "pull_request_read",
			Description: t("TOOL_PULL_REQUEST_READ_DESCRIPTION", "Get information on a specific pull request in GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_PULL_REQUEST_USER_TITLE", "Get details for a single pull request"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pullNumber, err := RequiredInt(args, "pullNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			// attachIFC adds IFC label to 一个成功ful 工具 结果 when
			// IFC labels are 启用. Pull 请求 内容 (descriptions,
			// diffs, comments, reviews) is user-authored 和therefore
			// 不受信任; confidentiality follows repo visibility. If the
			// visibility lookup fails label is omitted rather than
			// misclassifying 结果.
			attachIFC := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelRepoUserContent)
			}

			switch method {
			case "get":
				result, err := GetPullRequest(ctx, client, deps, owner, repo, pullNumber)
				return attachIFC(result), nil, err
			case "get_diff":
				result, err := GetPullRequestDiff(ctx, client, deps, owner, repo, pullNumber)
				return attachIFC(result), nil, err
			case "get_status":
				result, err := GetPullRequestStatus(ctx, client, owner, repo, pullNumber)
				return attachIFC(result), nil, err
			case "get_files":
				result, err := GetPullRequestFiles(ctx, client, deps, owner, repo, pullNumber, pagination)
				return attachIFC(result), nil, err
			case "get_commits":
				result, err := GetPullRequestCommits(ctx, client, owner, repo, pullNumber, pagination)
				return attachIFC(result), nil, err
			case "get_review_comments":
				gqlClient, err := deps.GetGQLClient(ctx)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to get GitHub GQL client", err), nil, nil
				}
				cursorPagination, err := OptionalCursorPaginationParams(args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				result, err := GetPullRequestReviewComments(ctx, gqlClient, deps, owner, repo, pullNumber, cursorPagination)
				return attachIFC(result), nil, err
			case "get_reviews":
				result, err := GetPullRequestReviews(ctx, client, deps, owner, repo, pullNumber, pagination)
				return attachIFC(result), nil, err
			case "get_comments":
				result, err := GetIssueComments(ctx, client, deps, owner, repo, pullNumber, pagination)
				return attachIFC(result), nil, err
			case "get_check_runs":
				result, err := GetPullRequestCheckRuns(ctx, client, owner, repo, pullNumber, pagination)
				return attachIFC(result), nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		})
}

func GetPullRequest(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int) (*mcp.CallToolResult, error) {
	cache, err := deps.GetRepoAccessCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo access cache: %w", err)
	}
	ff := deps.GetFlags(ctx)

	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request", resp, body), nil
	}

	// sanitize title/body on 响应
	if pr != nil {
		if pr.Title != nil {
			pr.Title = github.Ptr(sanitize.Sanitize(*pr.Title))
		}
		if pr.Body != nil {
			pr.Body = github.Ptr(sanitize.Sanitize(*pr.Body))
		}
	}

	if ff.LockdownMode {
		if restricted, err := authorLockdownResult(ctx, cache, owner, repo, pr.GetUser().GetLogin(), lockdownPullRequestRestrictedMessage); restricted != nil || err != nil {
			return restricted, err
		}
	}

	minimalPR := convertToMinimalPullRequest(pr)

	return MarshalledTextResult(minimalPR), nil
}

// enforcePullRequestLockdown 返回 一个restricted 工具 结果 when lockdown mode is
// 启用 以及拉取请求 auth或is 不a safe 内容 source f或owner/repo,
// 和(nil, nil) otherwise. It fetches 拉取请求 to resolve auth或和is
// 一个no-op that performs no 请求 when lockdown mode is 禁用.
func enforcePullRequestLockdown(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int) (*mcp.CallToolResult, error) {
	if !deps.GetFlags(ctx).LockdownMode {
		return nil, nil
	}
	cache, err := deps.GetRepoAccessCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo access cache: %w", err)
	}
	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get pull request", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request", resp, body), nil
	}

	return authorLockdownResult(ctx, cache, owner, repo, pr.GetUser().GetLogin(), lockdownPullRequestRestrictedMessage)
}

func GetPullRequestDiff(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int) (*mcp.CallToolResult, error) {
	if restricted, err := enforcePullRequestLockdown(ctx, client, deps, owner, repo, pullNumber); restricted != nil || err != nil {
		return restricted, err
	}

	raw, resp, err := client.PullRequests.GetRaw(
		ctx,
		owner,
		repo,
		pullNumber,
		github.RawOptions{Type: github.Diff},
	)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request diff",
			resp,
			err,
		), nil
	}

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request diff", resp, body), nil
	}

	defer func() { _ = resp.Body.Close() }()

	// Return raw 响应
	return utils.NewToolResultText(string(raw)), nil
}

func GetPullRequestStatus(ctx context.Context, client *github.Client, owner, repo string, pullNumber int) (*mcp.CallToolResult, error) {
	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request", resp, body), nil
	}

	// Get combined status 用于head SHA
	status, resp, err := client.Repositories.GetCombinedStatus(ctx, owner, repo, *pr.Head.SHA, nil)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get combined status",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get combined status", resp, body), nil
	}

	r, err := json.Marshal(status)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func GetPullRequestCheckRuns(ctx context.Context, client *github.Client, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	// First 获取 PR to 获取 head SHA
	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request",
			resp,
			err,
		), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request", resp, body), nil
	}

	// Get 检查 runs 用于head SHA
	opts := &github.ListCheckRunsOptions{
		ListOptions: github.ListOptions{
			PerPage: pagination.PerPage,
			Page:    pagination.Page,
		},
	}

	checkRuns, resp, err := client.Checks.ListCheckRunsForRef(ctx, owner, repo, *pr.Head.SHA, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get check runs",
			resp,
			err,
		), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get check runs", resp, body), nil
	}

	// Convert to minimal 检查 runs to reduce 上下文 usage
	minimalCheckRuns := make([]MinimalCheckRun, 0, len(checkRuns.CheckRuns))
	for _, checkRun := range checkRuns.CheckRuns {
		minimalCheckRuns = append(minimalCheckRuns, convertToMinimalCheckRun(checkRun))
	}

	minimalResult := MinimalCheckRunsResult{
		TotalCount: checkRuns.GetTotal(),
		CheckRuns:  minimalCheckRuns,
	}

	r, err := json.Marshal(minimalResult)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func GetPullRequestFiles(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	if restricted, err := enforcePullRequestLockdown(ctx, client, deps, owner, repo, pullNumber); restricted != nil || err != nil {
		return restricted, err
	}

	opts := &github.ListOptions{
		PerPage: pagination.PerPage,
		Page:    pagination.Page,
	}
	files, resp, err := client.PullRequests.ListFiles(ctx, owner, repo, pullNumber, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request files",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request files", resp, body), nil
	}

	minimalFiles := convertToMinimalPRFiles(files)

	return MarshalledTextResult(minimalFiles), nil
}

func GetPullRequestCommits(ctx context.Context, client *github.Client, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	opts := &github.ListOptions{
		PerPage: pagination.PerPage,
		Page:    pagination.Page,
	}
	commits, resp, err := client.PullRequests.ListCommits(ctx, owner, repo, pullNumber, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request commits",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request commits", resp, body), nil
	}

	minimalCommits := convertToMinimalPullRequestCommits(commits)

	return MarshalledTextResult(minimalCommits), nil
}

// GraphQL types f或review th读取s query
type reviewThreadsQuery struct {
	Repository struct {
		PullRequest struct {
			ReviewThreads struct {
				Nodes      []reviewThreadNode
				PageInfo   pageInfoFragment
				TotalCount githubv4.Int
			} `graphql:"reviewThreads(first: $first, after: $after)"`
		} `graphql:"pullRequest(number: $prNum)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type reviewThreadNode struct {
	ID          githubv4.ID
	IsResolved  githubv4.Boolean
	IsOutdated  githubv4.Boolean
	IsCollapsed githubv4.Boolean
	Comments    struct {
		Nodes      []reviewCommentNode
		TotalCount githubv4.Int
	} `graphql:"comments(first: $commentsPerThread)"`
}

type reviewCommentNode struct {
	ID     githubv4.ID
	Body   githubv4.String
	Path   githubv4.String
	Line   *githubv4.Int
	Author struct {
		Login githubv4.String
	}
	CreatedAt githubv4.DateTime
	UpdatedAt githubv4.DateTime
	URL       githubv4.URI
}

type pageInfoFragment struct {
	HasNextPage     githubv4.Boolean
	HasPreviousPage githubv4.Boolean
	StartCursor     githubv4.String
	EndCursor       githubv4.String
}

func GetPullRequestReviewComments(ctx context.Context, gqlClient *githubv4.Client, deps ToolDependencies, owner, repo string, pullNumber int, pagination CursorPaginationParams) (*mcp.CallToolResult, error) {
	cache, err := deps.GetRepoAccessCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo access cache: %w", err)
	}
	ff := deps.GetFlags(ctx)

	// Convert pagination 参数 to GraphQL format
	gqlParams, err := pagination.ToGraphQLParams()
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("invalid pagination parameters: %v", err)), nil
	}

	// Build variables f或GraphQL query
	vars := map[string]any{
		"owner":             githubv4.String(owner),
		"repo":              githubv4.String(repo),
		"prNum":             githubv4.Int(int32(pullNumber)), //nolint:gosec // pullNumber is controlled by user 输入 validation
		"first":             githubv4.Int(*gqlParams.First),
		"commentsPerThread": githubv4.Int(100),
	}

	// Add curs或if provided
	if gqlParams.After != nil {
		vars["after"] = githubv4.String(*gqlParams.After)
	} else {
		vars["after"] = (*githubv4.String)(nil)
	}

	// Execute GraphQL query
	var query reviewThreadsQuery
	if err := gqlClient.Query(ctx, &query, vars); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get pull request review threads",
			err,
		), nil
	}

	// Lockdown mode 筛选ing
	if ff.LockdownMode {
		if cache == nil {
			return nil, fmt.Errorf("lockdown cache is not configured")
		}

		// Iterate through th读取s 和筛选 comments
		for i := range query.Repository.PullRequest.ReviewThreads.Nodes {
			thread := &query.Repository.PullRequest.ReviewThreads.Nodes[i]
			filteredComments := make([]reviewCommentNode, 0, len(thread.Comments.Nodes))

			for _, comment := range thread.Comments.Nodes {
				login := string(comment.Author.Login)
				if login != "" {
					isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
					if err != nil {
						return nil, fmt.Errorf("failed to check lockdown mode: %w", err)
					}
					if isSafeContent {
						filteredComments = append(filteredComments, comment)
					}
				}
			}

			thread.Comments.Nodes = filteredComments
			thread.Comments.TotalCount = githubv4.Int(int32(len(filteredComments))) //nolint:gosec // comment count is bounded by API limits
		}
	}

	return MarshalledTextResult(convertToMinimalReviewThreadsResponse(query)), nil
}

func GetPullRequestReviews(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	cache, err := deps.GetRepoAccessCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo access cache: %w", err)
	}
	ff := deps.GetFlags(ctx)

	reviews, resp, err := client.PullRequests.ListReviews(ctx, owner, repo, pullNumber, &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	})
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request reviews",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request reviews", resp, body), nil
	}

	if ff.LockdownMode {
		if cache == nil {
			return nil, fmt.Errorf("lockdown cache is not configured")
		}
		filteredReviews := make([]*github.PullRequestReview, 0, len(reviews))
		for _, review := range reviews {
			login := review.GetUser().GetLogin()
			if login == "" {
				continue
			}
			isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
			if err != nil {
				return nil, fmt.Errorf("failed to check lockdown mode: %w", err)
			}
			if isSafeContent {
				filteredReviews = append(filteredReviews, review)
			}
		}
		reviews = filteredReviews
	}

	minimalReviews := make([]MinimalPullRequestReview, 0, len(reviews))
	for _, review := range reviews {
		minimalReviews = append(minimalReviews, convertToMinimalPullRequestReview(review))
	}

	return MarshalledTextResult(minimalReviews), nil
}

// PullRequestWriteUIResourceURI is URI 用于创建_pull_请求 工具's MCP App UI 资源.
const PullRequestWriteUIResourceURI = "ui://github-mcp-server/pr-write"

// PullRequestEditUIResourceURI is URI 用于更新_pull_请求 工具's MCP App UI 资源.
const PullRequestEditUIResourceURI = "ui://github-mcp-server/pr-edit"

// pullRequestWriteFormParams are 参数 创建_pull_请求 MCP App
// form collects 和re-sends on submit. Any other 参数 present on 一个调用
// can不be represented 由form.
var pullRequestWriteFormParams = map[string]struct{}{
	"owner":                 {},
	"repo":                  {},
	"title":                 {},
	"body":                  {},
	"head":                  {},
	"base":                  {},
	"draft":                 {},
	"maintainer_can_modify": {},
	"reviewers":             {},
	"_ui_submitted":         {},
}

var pullRequestUpdateFormParams = map[string]struct{}{
	"owner":                 {},
	"repo":                  {},
	"pullNumber":            {},
	"title":                 {},
	"body":                  {},
	"state":                 {},
	"draft":                 {},
	"base":                  {},
	"maintainer_can_modify": {},
	"reviewers":             {},
	"_ui_submitted":         {},
}

// CreatePullRequest 创建一个工具以 创建 一个新的 拉取请求.
func CreatePullRequest(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "create_pull_request",
			Description: t("TOOL_CREATE_PULL_REQUEST_DESCRIPTION", "Create a new pull request in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_PULL_REQUEST_USER_TITLE", "Open new pull request"),
				ReadOnlyHint: false,
			},
			Meta: mcp.Meta{
				"ui": map[string]any{
					"resourceUri": PullRequestWriteUIResourceURI,
					"visibility":  []string{"model", "app"},
				},
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
					"title": {
						Type:        "string",
						Description: "PR title",
					},
					"body": {
						Type:        "string",
						Description: "PR description",
					},
					"head": {
						Type:        "string",
						Description: "Branch containing changes",
					},
					"base": {
						Type:        "string",
						Description: "Branch to merge into",
					},
					"draft": {
						Type:        "boolean",
						Description: "Create as draft PR",
					},
					"maintainer_can_modify": {
						Type:        "boolean",
						Description: "Allow maintainer edits",
					},
					"reviewers": {
						Type:        "array",
						Description: "GitHub usernames or ORG/team-slug team reviewers to request reviews from",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
				},
				Required: []string{"owner", "repo", "title", "head", "base"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// H和off 到interactive MCP App form unless this c所有must
			// execute now (see shouldDeferToForm).
			if shouldDeferToForm(ctx, deps, req, args, pullRequestWriteFormParams) {
				return utils.NewToolResultAwaitingFormSubmission(fmt.Sprintf(
					"An interactive form has been shown to the user for creating a new pull request in %s/%s. "+
						"STOP — do not call any other tools, do not respond as if the pull request was created, "+
						"and do not claim the operation succeeded. The pull request has NOT been created yet; "+
						"only the form was rendered. Wait silently for the user to review and click Submit. "+
						"When they do, the real result will be delivered to your context automatically.",
					owner, repo,
				)), nil, nil
			}

			// When creating PR, title/head/base are 必需
			title, err := OptionalParam[string](args, "title")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			head, err := OptionalParam[string](args, "head")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			base, err := OptionalParam[string](args, "base")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if title == "" {
				return utils.NewToolResultError("missing required parameter: title"), nil, nil
			}
			if head == "" {
				return utils.NewToolResultError("missing required parameter: head"), nil, nil
			}
			if base == "" {
				return utils.NewToolResultError("missing required parameter: base"), nil, nil
			}

			body, err := OptionalParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			draft, err := OptionalParam[bool](args, "draft")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			maintainerCanModify, err := OptionalParam[bool](args, "maintainer_can_modify")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			reviewers, err := OptionalStringArrayParam(args, "reviewers")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			newPR := &github.CreatePullRequest{
				Title: github.Ptr(title),
				Head:  head,
				Base:  base,
			}

			if body != "" {
				newPR.Body = github.Ptr(body)
			}

			newPR.Draft = github.Ptr(draft)
			newPR.MaintainerCanModify = github.Ptr(maintainerCanModify)

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			pr, resp, err := client.PullRequests.Create(ctx, owner, repo, *newPR)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create pull request",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create pull request", resp, bodyBytes), nil, nil
			}

			if len(reviewers) > 0 {
				userReviewers, teamReviewers := splitPullRequestReviewers(reviewers)
				reviewersRequest := github.ReviewersRequest{
					Reviewers:     userReviewers,
					TeamReviewers: teamReviewers,
				}

				_, reviewerResp, err := client.PullRequests.RequestReviewers(ctx, owner, repo, pr.GetNumber(), reviewersRequest)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to request reviewers",
						reviewerResp,
						err,
					), nil, nil
				}
				defer func() {
					if reviewerResp != nil && reviewerResp.Body != nil {
						_ = reviewerResp.Body.Close()
					}
				}()

				if reviewerResp.StatusCode != http.StatusCreated && reviewerResp.StatusCode != http.StatusOK {
					bodyBytes, err := io.ReadAll(reviewerResp.Body)
					if err != nil {
						return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
					}
					return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to request reviewers", reviewerResp, bodyBytes), nil, nil
				}
			}

			// Return minimal 响应 with just essential 信息
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", pr.GetID()),
				URL: pr.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
}

// UpdatePullRequest 创建一个工具以 更新 一个existing 拉取请求.
func UpdatePullRequest(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
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
			"pullNumber": {
				Type:        "number",
				Description: "Pull request number to update",
			},
			"title": {
				Type:        "string",
				Description: "New title",
			},
			"body": {
				Type:        "string",
				Description: "New description",
			},
			"state": {
				Type:        "string",
				Description: "New state",
				Enum:        []any{"open", "closed"},
			},
			"draft": {
				Type:        "boolean",
				Description: "Mark pull request as draft (true) or ready for review (false)",
			},
			"base": {
				Type:        "string",
				Description: "New base branch name",
			},
			"maintainer_can_modify": {
				Type:        "boolean",
				Description: "Allow maintainer edits",
			},
			"reviewers": {
				Type:        "array",
				Description: "GitHub usernames or ORG/team-slug team reviewers to request reviews from",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
		},
		Required: []string{"owner", "repo", "pullNumber"},
	}

	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "update_pull_request",
			Description: t("TOOL_UPDATE_PULL_REQUEST_DESCRIPTION", "Update an existing pull request in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_PULL_REQUEST_USER_TITLE", "Edit pull request"),
				ReadOnlyHint: false,
			},
			Meta: mcp.Meta{
				"ui": map[string]any{
					"resourceUri": PullRequestEditUIResourceURI,
					"visibility":  []string{"model", "app"},
				},
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pullNumber, err := RequiredInt(args, "pullNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// H和off 到interactive MCP App form unless this c所有must
			// execute now (see shouldDeferToForm).
			if shouldDeferToForm(ctx, deps, req, args, pullRequestUpdateFormParams) {
				return utils.NewToolResultAwaitingFormSubmission(fmt.Sprintf(
					"An interactive form has been shown to the user for editing pull request #%d in %s/%s. "+
						"STOP — do not call any other tools, do not respond as if the pull request was updated, "+
						"and do not claim the operation succeeded. The pull request has NOT been updated yet; "+
						"only the form was rendered. Wait silently for the user to review and click Submit. "+
						"When they do, the real result will be delivered to your context automatically.",
					pullNumber, owner, repo,
				)), nil, nil
			}

			_, draftProvided := args["draft"]
			var draftValue bool
			if draftProvided {
				draftValue, err = OptionalParam[bool](args, "draft")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
			}

			update := &github.PullRequest{}
			restUpdateNeeded := false

			if title, ok, err := OptionalParamOK[string](args, "title"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				update.Title = github.Ptr(title)
				restUpdateNeeded = true
			}

			if body, ok, err := OptionalParamOK[string](args, "body"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				update.Body = github.Ptr(body)
				restUpdateNeeded = true
			}

			if state, ok, err := OptionalParamOK[string](args, "state"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				update.State = github.Ptr(state)
				restUpdateNeeded = true
			}

			if base, ok, err := OptionalParamOK[string](args, "base"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				update.Base = &github.PullRequestBranch{Ref: github.Ptr(base)}
				restUpdateNeeded = true
			}

			if maintainerCanModify, ok, err := OptionalParamOK[bool](args, "maintainer_can_modify"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				update.MaintainerCanModify = github.Ptr(maintainerCanModify)
				restUpdateNeeded = true
			}

			// Handle reviewers separately
			reviewers, err := OptionalStringArrayParam(args, "reviewers")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// 如果没有更新s, no draft change, 和no reviewers, 返回 错误 early
			if !restUpdateNeeded && !draftProvided && len(reviewers) == 0 {
				return utils.NewToolResultError("No update parameters provided."), nil, nil
			}

			// Handle REST API 更新s (title, body, state, base, maintainer_can_modify)
			if restUpdateNeeded {
				client, err := deps.GetClient(ctx)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
				}

				_, resp, err := client.PullRequests.Edit(ctx, owner, repo, pullNumber, update)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to update pull request",
						resp,
						err,
					), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				if resp.StatusCode != http.StatusOK {
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
					}
					return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to update pull request", resp, bodyBytes), nil, nil
				}
			}

			// Handle draft status changes using GraphQL
			if draftProvided {
				gqlClient, err := deps.GetGQLClient(ctx)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
				}

				var prQuery struct {
					Repository struct {
						PullRequest struct {
							ID      githubv4.ID
							IsDraft githubv4.Boolean
						} `graphql:"pullRequest(number: $prNum)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}

				err = gqlClient.Query(ctx, &prQuery, map[string]any{
					"owner": githubv4.String(owner),
					"repo":  githubv4.String(repo),
					"prNum": githubv4.Int(pullNumber), // #nosec G115 - 拉取请求 numbers are 始终sm所有positive integers
				})
				if err != nil {
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to find pull request", err), nil, nil
				}

				currentIsDraft := bool(prQuery.Repository.PullRequest.IsDraft)

				if currentIsDraft != draftValue {
					if draftValue {
						// Convert to draft
						var mutation struct {
							ConvertPullRequestToDraft struct {
								PullRequest struct {
									ID      githubv4.ID
									IsDraft githubv4.Boolean
								}
							} `graphql:"convertPullRequestToDraft(input: $input)"`
						}

						err = gqlClient.Mutate(ctx, &mutation, githubv4.ConvertPullRequestToDraftInput{
							PullRequestID: prQuery.Repository.PullRequest.ID,
						}, nil)
						if err != nil {
							return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to convert pull request to draft", err), nil, nil
						}
					} else {
						// Mark as 读取y f或review
						var mutation struct {
							MarkPullRequestReadyForReview struct {
								PullRequest struct {
									ID      githubv4.ID
									IsDraft githubv4.Boolean
								}
							} `graphql:"markPullRequestReadyForReview(input: $input)"`
						}

						err = gqlClient.Mutate(ctx, &mutation, githubv4.MarkPullRequestReadyForReviewInput{
							PullRequestID: prQuery.Repository.PullRequest.ID,
						}, nil)
						if err != nil {
							return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to mark pull request ready for review", err), nil, nil
						}
					}
				}
			}

			// Handle reviewer 请求s
			if len(reviewers) > 0 {
				client, err := deps.GetClient(ctx)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
				}

				userReviewers, teamReviewers := splitPullRequestReviewers(reviewers)
				reviewersRequest := github.ReviewersRequest{
					Reviewers:     userReviewers,
					TeamReviewers: teamReviewers,
				}

				_, resp, err := client.PullRequests.RequestReviewers(ctx, owner, repo, pullNumber, reviewersRequest)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to request reviewers",
						resp,
						err,
					), nil, nil
				}
				defer func() {
					if resp != nil && resp.Body != nil {
						_ = resp.Body.Close()
					}
				}()

				if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
					}
					return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to request reviewers", resp, bodyBytes), nil, nil
				}
			}

			// Get final state 的PR to 返回
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			finalPR, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "Failed to get pull request", resp, err), nil, nil
			}
			defer func() {
				if resp != nil && resp.Body != nil {
					_ = resp.Body.Close()
				}
			}()

			// Return minimal 响应 with just essential 信息
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", finalPR.GetID()),
				URL: finalPR.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return utils.NewToolResultErrorFromErr("Failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
	st.FeatureFlagDisable = []string{FeatureFlagPullRequestsGranular}
	return st
}

// AddReplyToPullRequestComment 创建一个工具以 add 一个reply 或reaction to 一个existing 拉取请求 comment.
func AddReplyToPullRequestComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
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
			"pullNumber": {
				Type:        "number",
				Description: "Pull request number. Required when body is provided.",
			},
			"commentId": {
				Type:        "number",
				Description: "The numeric ID of the pull request review comment to reply or react to. Use the number from a #discussion_r... anchor, not the GraphQL thread node ID (PRRT_...).",
				Minimum:     jsonschema.Ptr(1.0),
			},
			"body": {
				Type:        "string",
				Description: "The text of the reply. Required unless reaction is provided.",
			},
			"reaction": {
				Type:        "string",
				Description: "Emoji reaction to add. Required unless body is provided.",
				Enum:        []any{"+1", "-1", "laugh", "confused", "heart", "hooray", "rocket", "eyes"},
			},
		},
		Required: []string{"owner", "repo", "commentId"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "add_reply_to_pull_request_comment",
			Description: t("TOOL_ADD_REPLY_TO_PULL_REQUEST_COMMENT_DESCRIPTION", "Add a reply and/or reaction to an existing pull request comment. This can create a new comment linked as a reply to the specified comment, add an emoji reaction to the specified comment, or do both. At least one of body or reaction is required."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ADD_REPLY_TO_PULL_REQUEST_COMMENT_USER_TITLE", "Add reply to pull request comment"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			commentID, err := RequiredBigInt(args, "commentId")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if commentID < 1 {
				return utils.NewToolResultError("commentId must be greater than 0"), nil, nil
			}
			body, hasBody, err := OptionalParamOK[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			reactionContent, hasReaction, err := OptionalParamOK[string](args, "reaction")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if !hasBody && !hasReaction {
				return utils.NewToolResultError("at least one of body or reaction is required"), nil, nil
			}
			if hasBody && body == "" {
				return utils.NewToolResultError("body cannot be empty when provided"), nil, nil
			}
			if hasReaction && reactionContent == "" {
				return utils.NewToolResultError("reaction cannot be empty when provided"), nil, nil
			}
			var pullNumber int
			if hasBody {
				pullNumber, err = RequiredInt(args, "pullNumber")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			var reactionResponse *MinimalResponse
			if hasReaction {
				reaction, resp, err := client.Reactions.CreatePullRequestCommentReaction(ctx, owner, repo, commentID, reactionContent)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to add reaction to pull request review comment", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				reactionResponse = &MinimalResponse{
					ID:  fmt.Sprintf("%d", reaction.GetID()),
					URL: fmt.Sprintf("%srepos/%s/%s/pulls/comments/%d/reactions/%d", client.BaseURL(), owner, repo, commentID, reaction.GetID()),
				}
			}

			var comment *github.PullRequestComment
			if hasBody {
				var resp *github.Response
				comment, resp, err = client.PullRequests.CreateCommentInReplyTo(ctx, owner, repo, pullNumber, body, commentID)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to add reply to pull request comment", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				if resp.StatusCode != http.StatusCreated {
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
					}
					return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to add reply to pull request comment", resp, bodyBytes), nil, nil
				}
			}

			var result any
			switch {
			case hasBody && hasReaction:
				result = map[string]any{
					"comment":  comment,
					"reaction": reactionResponse,
				}
			case hasReaction:
				result = reactionResponse
			default:
				result = comment
			}

			r, err := json.Marshal(result)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
}

// ListPullRequests 创建一个工具以 列出 拉取请求 in 一个GitHub 仓库.
func ListPullRequests(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
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
			"state": {
				Type:        "string",
				Description: "Filter by state",
				Enum:        []any{"open", "closed", "all"},
			},
			"head": {
				Type:        "string",
				Description: "Filter by head user/org and branch",
			},
			"base": {
				Type:        "string",
				Description: "Filter by base branch",
			},
			"sort": {
				Type:        "string",
				Description: "Sort by",
				Enum:        []any{"created", "updated", "popularity", "long-running"},
			},
			"direction": {
				Type:        "string",
				Description: "Sort direction",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"owner", "repo"},
	}
	schema.Properties["fields"] = fieldsSchemaProperty(
		"Subset of fields to return for each pull request. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body' in particular drops the largest per-result data.",
		listPullRequestsItemFieldEnum,
	)
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "list_pull_requests",
			Description: t("TOOL_LIST_PULL_REQUESTS_DESCRIPTION", "List pull requests in a GitHub repository. If the user specifies an author, then DO NOT use this tool and use the search_pull_requests tool instead."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_PULL_REQUESTS_USER_TITLE", "List pull requests"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			head, err := OptionalParam[string](args, "head")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			base, err := OptionalParam[string](args, "base")
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
			fields, err := OptionalStringArrayParam(args, "fields")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			opts := &github.PullRequestListOptions{
				State:     state,
				Head:      head,
				Base:      base,
				Sort:      sort,
				Direction: direction,
				ListOptions: github.ListOptions{
					PerPage: pagination.PerPage,
					Page:    pagination.Page,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			prs, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list pull requests",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list pull requests", resp, bodyBytes), nil, nil
			}

			// sanitize title/body on 每个PR
			for _, pr := range prs {
				if pr == nil {
					continue
				}
				if pr.Title != nil {
					pr.Title = github.Ptr(sanitize.Sanitize(*pr.Title))
				}
				if pr.Body != nil {
					pr.Body = github.Ptr(sanitize.Sanitize(*pr.Body))
				}
			}

			minimalPRs := make([]MinimalPullRequest, 0, len(prs))
			for _, pr := range prs {
				if pr != nil {
					minimalPRs = append(minimalPRs, convertToMinimalPullRequest(pr))
				}
			}

			filtered := false
			var payload any = minimalPRs
			if len(fields) > 0 {
				filteredPRs, err := filterEachField(minimalPRs, fields)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to filter pull requests", err), nil, nil
				}
				payload = filteredPRs
				filtered = true
			}

			r, err := json.Marshal(payload)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			recordFieldsUsageFor(ctx, deps, "list_pull_requests", minimalPRs, filtered, len(r))

			result := utils.NewToolResultText(string(r))
			// Pull 请求 titles/bodies are user-authored (不受信任);
			// confidentiality follows repo visibility.
			result = attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelRepoUserContent)
			return result, nil, nil
		})
}

// MergePullRequest 创建一个工具以 merge 一个拉取请求.
func MergePullRequest(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
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
			"pullNumber": {
				Type:        "number",
				Description: "Pull request number",
			},
			"commit_title": {
				Type:        "string",
				Description: "Title for merge commit",
			},
			"commit_message": {
				Type:        "string",
				Description: "Extra detail for merge commit",
			},
			"merge_method": {
				Type:        "string",
				Description: "Merge method",
				Enum:        []any{"merge", "squash", "rebase"},
			},
		},
		Required: []string{"owner", "repo", "pullNumber"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "merge_pull_request",
			Description: t("TOOL_MERGE_PULL_REQUEST_DESCRIPTION", "Merge a pull request in a GitHub repository."),
			Icons:       octicons.Icons("git-merge"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_MERGE_PULL_REQUEST_USER_TITLE", "Merge pull request"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pullNumber, err := RequiredInt(args, "pullNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			commitTitle, err := OptionalParam[string](args, "commit_title")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			commitMessage, err := OptionalParam[string](args, "commit_message")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			mergeMethod, err := OptionalParam[string](args, "merge_method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			options := &github.PullRequestOptions{
				CommitTitle: commitTitle,
				MergeMethod: mergeMethod,
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			result, resp, err := client.PullRequests.Merge(ctx, owner, repo, pullNumber, commitMessage, options)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to merge pull request",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to merge pull request", resp, bodyBytes), nil, nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
}

// SearchPullRequests 创建一个工具以 search f或拉取请求.
func SearchPullRequests(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Search query using GitHub pull request search syntax",
			},
			"owner": {
				Type:        "string",
				Description: "Optional repository owner. If provided with repo, only pull requests for this repository are listed.",
			},
			"repo": {
				Type:        "string",
				Description: "Optional repository name. If provided with owner, only pull requests for this repository are listed.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort field by number of matches of categories, defaults to best match",
				Enum: []any{
					"comments",
					"reactions",
					"reactions-+1",
					"reactions--1",
					"reactions-smile",
					"reactions-thinking_face",
					"reactions-heart",
					"reactions-tada",
					"interactions",
					"created",
					"updated",
				},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	schema.Properties["fields"] = fieldsSchemaProperty(
		"Subset of fields to return for each pull request result. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body', 'reactions', and 'labels' in particular drops the largest per-result data.",
		searchPullRequestsItemFieldEnum,
	)
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "search_pull_requests",
			Description: t("TOOL_SEARCH_PULL_REQUESTS_DESCRIPTION", "Search for pull requests in GitHub repositories using issues search syntax already scoped to is:pr"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_PULL_REQUESTS_USER_TITLE", "Search pull requests"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			options := []searchOption{ifcSearchPostProcessOption(ctx, deps)}
			fields, err := OptionalStringArrayParam(args, "fields")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			options = append(options, withFieldsFiltering(deps, "search_pull_requests", fields))
			result, err := searchHandler(ctx, deps.GetClient, args, "pr", "failed to search pull requests", options...)
			return result, nil, err
		})
}

// UpdatePullRequestBranch 创建一个工具以 更新 一个拉取请求 分支 使用latest changes 来自base 分支.
func UpdatePullRequestBranch(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
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
			"pullNumber": {
				Type:        "number",
				Description: "Pull request number",
			},
			"expectedHeadSha": {
				Type:        "string",
				Description: "The expected SHA of the pull request's HEAD ref",
			},
		},
		Required: []string{"owner", "repo", "pullNumber"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "update_pull_request_branch",
			Description: t("TOOL_UPDATE_PULL_REQUEST_BRANCH_DESCRIPTION", "Update the branch of a pull request with the latest changes from the base branch."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_PULL_REQUEST_BRANCH_USER_TITLE", "Update pull request branch"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pullNumber, err := RequiredInt(args, "pullNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			expectedHeadSHA, err := OptionalParam[string](args, "expectedHeadSha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			opts := &github.PullRequestBranchUpdateOptions{}
			if expectedHeadSHA != "" {
				opts.ExpectedHeadSHA = github.Ptr(expectedHeadSHA)
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			result, resp, err := client.PullRequests.UpdateBranch(ctx, owner, repo, pullNumber, opts)
			if err != nil {
				// Check if it's 一个acceptedError. 一个acceptedErr或indicates that 更新 is in progress,
				// 和it's 不a real 错误.
				if resp != nil && resp.StatusCode == http.StatusAccepted && isAcceptedError(err) {
					return utils.NewToolResultText("Pull request branch update is in progress"), nil, nil
				}
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to update pull request branch",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusAccepted {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to update pull request branch", resp, bodyBytes), nil, nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
}

type PullRequestReviewWriteParams struct {
	Method     string
	Owner      string
	Repo       string
	PullNumber int32
	Body       string
	Event      string
	CommitID   *string
	ThreadID   string
}

func PullRequestReviewWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			// Either we need PR GQL Id directly, 或we need owner, repo 和PR number to look it up.
			// Since our other Pull Request 工具 are working 使用REST Client, will handle lookup
			// internally f或now.
			"method": {
				Type:        "string",
				Description: `The write operation to perform on pull request review.`,
				Enum:        []any{"create", "submit_pending", "delete_pending", "resolve_thread", "unresolve_thread"},
			},
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"pullNumber": {
				Type:        "number",
				Description: "Pull request number",
			},
			"body": {
				Type:        "string",
				Description: "Review comment text",
			},
			"event": {
				Type:        "string",
				Description: "Review action to perform.",
				Enum:        []any{"APPROVE", "REQUEST_CHANGES", "COMMENT"},
			},
			"commitID": {
				Type:        "string",
				Description: "SHA of commit to review",
			},
			"threadId": {
				Type:        "string",
				Description: "The node ID of the review thread (e.g., PRRT_kwDOxxx). Required for resolve_thread and unresolve_thread methods. Get thread IDs from pull_request_read with method get_review_comments.",
			},
		},
		Required: []string{"method", "owner", "repo", "pullNumber"},
	}

	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name: "pull_request_review_write",
			Description: t("TOOL_PULL_REQUEST_REVIEW_WRITE_DESCRIPTION", `Create and/or submit, delete review of a pull request.

Available methods:
- create: Create a new review of a pull request. If "event" parameter is provided, the review is submitted. If "event" is omitted, a pending review is created.
- submit_pending: Submit an existing pending review of a pull request. This requires that a pending review exists for the current user on the specified pull request. The "body" and "event" parameters are used when submitting the review.
- delete_pending: Delete an existing pending review of a pull request. This requires that a pending review exists for the current user on the specified pull request.
- resolve_thread: Resolve a review thread. Requires only "threadId" parameter with the thread's node ID (e.g., PRRT_kwDOxxx). The owner, repo, and pullNumber parameters are not used for this method. Resolving an already-resolved thread is a no-op.
- unresolve_thread: Unresolve a previously resolved review thread. Requires only "threadId" parameter. The owner, repo, and pullNumber parameters are not used for this method. Unresolving an already-unresolved thread is a no-op.
`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_PULL_REQUEST_REVIEW_WRITE_USER_TITLE", "Write operations (create, submit, delete) on pull request reviews"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			var params PullRequestReviewWriteParams
			if err := mapstructure.WeakDecode(args, &params); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Given our owner, repo 和PR number, lookup GQL ID 的PR.
			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			switch params.Method {
			case "create":
				result, err := CreatePullRequestReview(ctx, client, params)
				return result, nil, err
			case "submit_pending":
				result, err := SubmitPendingPullRequestReview(ctx, client, params)
				return result, nil, err
			case "delete_pending":
				result, err := DeletePendingPullRequestReview(ctx, client, params)
				return result, nil, err
			case "resolve_thread":
				result, err := ResolveReviewThread(ctx, client, params.ThreadID, true)
				return result, nil, err
			case "unresolve_thread":
				result, err := ResolveReviewThread(ctx, client, params.ThreadID, false)
				return result, nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", params.Method)), nil, nil
			}
		})
	st.FeatureFlagDisable = []string{FeatureFlagPullRequestsGranular}
	return st
}

func CreatePullRequestReview(ctx context.Context, client *githubv4.Client, params PullRequestReviewWriteParams) (*mcp.CallToolResult, error) {
	var getPullRequestQuery struct {
		Repository struct {
			PullRequest struct {
				ID githubv4.ID
			} `graphql:"pullRequest(number: $prNum)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	if err := client.Query(ctx, &getPullRequestQuery, map[string]any{
		"owner": githubv4.String(params.Owner),
		"repo":  githubv4.String(params.Repo),
		"prNum": githubv4.Int(params.PullNumber),
	}); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get pull request",
			err,
		), nil
	}

	// Now we have GQL ID, we can 创建 一个review
	var addPullRequestReviewMutation struct {
		AddPullRequestReview struct {
			PullRequestReview struct {
				ID githubv4.ID // We don't need this, 但a select或is 必需 或GQL complains.
			}
		} `graphql:"addPullRequestReview(input: $input)"`
	}

	addPullRequestReviewInput := githubv4.AddPullRequestReviewInput{
		PullRequestID: getPullRequestQuery.Repository.PullRequest.ID,
		CommitOID:     newGQLStringlikePtr[githubv4.GitObjectID](params.CommitID),
	}

	// Event 和Body are provided if we submit 一个review
	if params.Event != "" {
		addPullRequestReviewInput.Event = newGQLStringlike[githubv4.PullRequestReviewEvent](params.Event)
		addPullRequestReviewInput.Body = githubv4.NewString(githubv4.String(params.Body))
	}

	if err := client.Mutate(
		ctx,
		&addPullRequestReviewMutation,
		addPullRequestReviewInput,
		nil,
	); err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	// Return nothing interesting, just indicate 成功 用于time being.
	// In future, we may want to 返回 review ID, 但用于moment, we're 不leaking
	// API implementation details 到LLM.
	if params.Event == "" {
		return utils.NewToolResultText("pending pull request created"), nil
	}
	return utils.NewToolResultText("pull request review submitted successfully"), nil
}

func SubmitPendingPullRequestReview(ctx context.Context, client *githubv4.Client, params PullRequestReviewWriteParams) (*mcp.CallToolResult, error) {
	// First we'll 获取 current user
	var getViewerQuery struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	if err := client.Query(ctx, &getViewerQuery, nil); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get current user",
			err,
		), nil
	}

	var getLatestReviewForViewerQuery struct {
		Repository struct {
			PullRequest struct {
				Reviews struct {
					Nodes []struct {
						ID    githubv4.ID
						State githubv4.PullRequestReviewState
						URL   githubv4.URI
					}
				} `graphql:"reviews(first: 1, author: $author)"`
			} `graphql:"pullRequest(number: $prNum)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	vars := map[string]any{
		"author": githubv4.String(getViewerQuery.Viewer.Login),
		"owner":  githubv4.String(params.Owner),
		"name":   githubv4.String(params.Repo),
		"prNum":  githubv4.Int(params.PullNumber),
	}

	if err := client.Query(ctx, &getLatestReviewForViewerQuery, vars); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get latest review for current user",
			err,
		), nil
	}

	// Validate there is one review 以及state is pending
	if len(getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes) == 0 {
		return utils.NewToolResultError("No pending review found for the viewer"), nil
	}

	review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
	if review.State != githubv4.PullRequestReviewStatePending {
		errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
		return utils.NewToolResultError(errText), nil
	}

	// Prepare mutation
	var submitPullRequestReviewMutation struct {
		SubmitPullRequestReview struct {
			PullRequestReview struct {
				ID githubv4.ID // We don't need this, 但a select或is 必需 或GQL complains.
			}
		} `graphql:"submitPullRequestReview(input: $input)"`
	}

	if err := client.Mutate(
		ctx,
		&submitPullRequestReviewMutation,
		githubv4.SubmitPullRequestReviewInput{
			PullRequestReviewID: &review.ID,
			Event:               githubv4.PullRequestReviewEvent(params.Event),
			Body:                newGQLStringlikePtr[githubv4.String](&params.Body),
		},
		nil,
	); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to submit pull request review",
			err,
		), nil
	}

	// Return nothing interesting, just indicate 成功 用于time being.
	// In future, we may want to 返回 review ID, 但用于moment, we're 不leaking
	// API implementation details 到LLM.
	return utils.NewToolResultText("pending pull request review successfully submitted"), nil
}

func DeletePendingPullRequestReview(ctx context.Context, client *githubv4.Client, params PullRequestReviewWriteParams) (*mcp.CallToolResult, error) {
	// First we'll 获取 current user
	var getViewerQuery struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	if err := client.Query(ctx, &getViewerQuery, nil); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get current user",
			err,
		), nil
	}

	var getLatestReviewForViewerQuery struct {
		Repository struct {
			PullRequest struct {
				Reviews struct {
					Nodes []struct {
						ID    githubv4.ID
						State githubv4.PullRequestReviewState
						URL   githubv4.URI
					}
				} `graphql:"reviews(first: 1, author: $author)"`
			} `graphql:"pullRequest(number: $prNum)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	vars := map[string]any{
		"author": githubv4.String(getViewerQuery.Viewer.Login),
		"owner":  githubv4.String(params.Owner),
		"name":   githubv4.String(params.Repo),
		"prNum":  githubv4.Int(params.PullNumber),
	}

	if err := client.Query(ctx, &getLatestReviewForViewerQuery, vars); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get latest review for current user",
			err,
		), nil
	}

	// Validate there is one review 以及state is pending
	if len(getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes) == 0 {
		return utils.NewToolResultError("No pending review found for the viewer"), nil
	}

	review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
	if review.State != githubv4.PullRequestReviewStatePending {
		errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
		return utils.NewToolResultError(errText), nil
	}

	// Prepare mutation
	var deletePullRequestReviewMutation struct {
		DeletePullRequestReview struct {
			PullRequestReview struct {
				ID githubv4.ID // We don't need this, 但a select或is 必需 或GQL complains.
			}
		} `graphql:"deletePullRequestReview(input: $input)"`
	}

	if err := client.Mutate(
		ctx,
		&deletePullRequestReviewMutation,
		githubv4.DeletePullRequestReviewInput{
			PullRequestReviewID: &review.ID,
		},
		nil,
	); err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	// Return nothing interesting, just indicate 成功 用于time being.
	// In future, we may want to 返回 review ID, 但用于moment, we're 不leaking
	// API implementation details 到LLM.
	return utils.NewToolResultText("pending pull request review successfully deleted"), nil
}

// ResolveReviewTh读取 resolves 或unresolves 一个PR review th读取 using GraphQL mutations.
func ResolveReviewThread(ctx context.Context, client *githubv4.Client, threadID string, resolve bool) (*mcp.CallToolResult, error) {
	if threadID == "" {
		return utils.NewToolResultError("threadId is required for resolve_thread and unresolve_thread methods"), nil
	}

	if resolve {
		var mutation struct {
			ResolveReviewThread struct {
				Thread struct {
					ID         githubv4.ID
					IsResolved githubv4.Boolean
				}
			} `graphql:"resolveReviewThread(input: $input)"`
		}

		input := githubv4.ResolveReviewThreadInput{
			ThreadID: githubv4.ID(threadID),
		}

		if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
				"failed to resolve review thread",
				err,
			), nil
		}

		return utils.NewToolResultText("review thread resolved successfully"), nil
	}

	// Unresolve
	var mutation struct {
		UnresolveReviewThread struct {
			Thread struct {
				ID         githubv4.ID
				IsResolved githubv4.Boolean
			}
		} `graphql:"unresolveReviewThread(input: $input)"`
	}

	input := githubv4.UnresolveReviewThreadInput{
		ThreadID: githubv4.ID(threadID),
	}

	if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to unresolve review thread",
			err,
		), nil
	}

	return utils.NewToolResultText("review thread unresolved successfully"), nil
}

// AddCommentToPendingReviewParams contains 参数 f或adding 一个comment to 一个pending review.
type AddCommentToPendingReviewParams struct {
	Owner       string
	Repo        string
	PullNumber  int32
	Path        string
	Body        string
	SubjectType string
	Line        *int32
	Side        *string
	StartLine   *int32
	StartSide   *string
}

// AddCommentToPendingReviewC所有adds 一个review comment 到viewer's pending 拉取请求 review.
func AddCommentToPendingReviewCall(ctx context.Context, client *githubv4.Client, params AddCommentToPendingReviewParams) (*mcp.CallToolResult, error) {
	// Get current user
	var getViewerQuery struct {
		Viewer struct {
			Login githubv4.String
		}
	}

	if err := client.Query(ctx, &getViewerQuery, nil); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get current user",
			err,
		), nil
	}

	var getLatestReviewForViewerQuery struct {
		Repository struct {
			PullRequest struct {
				Reviews struct {
					Nodes []struct {
						ID    githubv4.ID
						State githubv4.PullRequestReviewState
						URL   githubv4.URI
					}
				} `graphql:"reviews(first: 1, author: $author)"`
			} `graphql:"pullRequest(number: $prNum)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	vars := map[string]any{
		"author": githubv4.String(getViewerQuery.Viewer.Login),
		"owner":  githubv4.String(params.Owner),
		"name":   githubv4.String(params.Repo),
		"prNum":  githubv4.Int(params.PullNumber),
	}

	if err := client.Query(ctx, &getLatestReviewForViewerQuery, vars); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get latest review for current user",
			err,
		), nil
	}

	// Validate there is one review 以及state is pending
	if len(getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes) == 0 {
		return utils.NewToolResultError("No pending review found for the viewer"), nil
	}

	review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
	if review.State != githubv4.PullRequestReviewStatePending {
		errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
		return utils.NewToolResultError(errText), nil
	}

	// Create 一个新的 review th读取 comment 在review.
	var addPullRequestReviewThreadMutation struct {
		AddPullRequestReviewThread struct {
			Thread struct {
				ID githubv4.ID
			}
		} `graphql:"addPullRequestReviewThread(input: $input)"`
	}

	if err := client.Mutate(
		ctx,
		&addPullRequestReviewThreadMutation,
		githubv4.AddPullRequestReviewThreadInput{
			Path:                githubv4.String(params.Path),
			Body:                githubv4.String(params.Body),
			SubjectType:         newGQLStringlikePtr[githubv4.PullRequestReviewThreadSubjectType](&params.SubjectType),
			Line:                newGQLIntPtr(params.Line),
			Side:                newGQLStringlikePtr[githubv4.DiffSide](params.Side),
			StartLine:           newGQLIntPtr(params.StartLine),
			StartSide:           newGQLStringlikePtr[githubv4.DiffSide](params.StartSide),
			PullRequestReviewID: &review.ID,
		},
		nil,
	); err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	if addPullRequestReviewThreadMutation.AddPullRequestReviewThread.Thread.ID == nil {
		return utils.NewToolResultError(`Failed to add comment to pending review. Possible reasons:
	- The line number doesn't exist in the pull request diff
	- The file path is incorrect
	- The side (LEFT/RIGHT) is invalid for the specified line
`), nil
	}

	return utils.NewToolResultText("pull request review comment successfully added to pending review"), nil
}

// AddCommentToPendingReview 创建一个工具以 add 一个comment to 一个拉取请求 review.
func AddCommentToPendingReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			// Ideally, f或performance sake this would just accept pullRequestReviewID. However, we would need to
			// add 一个新的 工具 to 获取 that ID f或客户端s that aren't 在相同 上下文 as original pending review
			// creation. So f或now, we'll just accept owner, repo 和pull number 和assume this is adding 一个comment
			// latest review from 一个user, since 仅one 可以 active at 一个time. It can later be extended with
			// 一个pullRequestReviewID 参数 if tar获取ing other reviews is desired:
			// mcp.WithString("pullRequestReviewID",
			// 	mcp.Required(),
			// 	mcp.Description("ID 的拉取请求 review to add 一个comment to"),
			// ),
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"pullNumber": {
				Type:        "number",
				Description: "Pull request number",
			},
			"path": {
				Type:        "string",
				Description: "The relative path to the file that necessitates a comment",
			},
			"body": {
				Type:        "string",
				Description: "The text of the review comment",
			},
			"subjectType": {
				Type:        "string",
				Description: "The level at which the comment is targeted",
				Enum:        []any{"FILE", "LINE"},
			},
			"line": {
				Type:        "number",
				Description: "The line of the blob in the pull request diff that the comment applies to. For multi-line comments, the last line of the range",
			},
			"side": {
				Type:        "string",
				Description: "The side of the diff to comment on. LEFT indicates the previous state, RIGHT indicates the new state",
				Enum:        []any{"LEFT", "RIGHT"},
			},
			"startLine": {
				Type:        "number",
				Description: "For multi-line comments, the first line of the range that the comment applies to",
			},
			"startSide": {
				Type:        "string",
				Description: "For multi-line comments, the starting side of the diff that the comment applies to. LEFT indicates the previous state, RIGHT indicates the new state",
				Enum:        []any{"LEFT", "RIGHT"},
			},
		},
		Required: []string{"owner", "repo", "pullNumber", "path", "body", "subjectType"},
	}

	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "add_comment_to_pending_review",
			Description: t("TOOL_ADD_COMMENT_TO_PENDING_REVIEW_DESCRIPTION", "Add review comment to the requester's latest pending pull request review. A pending review needs to already exist to call this (check with the user if not sure)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ADD_COMMENT_TO_PENDING_REVIEW_USER_TITLE", "Add review comment to the requester's latest pending pull request review"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pullNumber, err := RequiredInt(args, "pullNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			path, err := RequiredParam[string](args, "path")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			body, err := RequiredParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			subjectType, err := RequiredParam[string](args, "subjectType")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			line, err := OptionalIntParam(args, "line")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			side, _ := OptionalParam[string](args, "side")
			startLine, err := OptionalIntParam(args, "startLine")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			startSide, _ := OptionalParam[string](args, "startSide")

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GQL client", err), nil, nil
			}

			var linePtr, startLinePtr *int32
			if line != 0 {
				l := int32(line) // #nosec G115
				linePtr = &l
			}
			if startLine != 0 {
				sl := int32(startLine) // #nosec G115
				startLinePtr = &sl
			}
			var sidePtr, startSidePtr *string
			if side != "" {
				sidePtr = &side
			}
			if startSide != "" {
				startSidePtr = &startSide
			}

			result, err := AddCommentToPendingReviewCall(ctx, client, AddCommentToPendingReviewParams{
				Owner:       owner,
				Repo:        repo,
				PullNumber:  int32(pullNumber), // #nosec G115 - PR numbers are 始终sm所有positive integers
				Path:        path,
				Body:        body,
				SubjectType: subjectType,
				Line:        linePtr,
				Side:        sidePtr,
				StartLine:   startLinePtr,
				StartSide:   startSidePtr,
			})
			return result, nil, err
		})
	st.FeatureFlagDisable = []string{FeatureFlagPullRequestsGranular}
	return st
}

// 新的GQLString like takes something that approximates 一个string (of which there are many types in shurcooL/githubv4)
// 和constructs 一个pointer to it, 或nil 如果string is 空. 此is extremely useful 因为when we parse
// params 来自MCP 请求, we need to convert them to types that are pointers of type def strings 和it's
// 不possible to take 一个pointer of 一个anonymous 值 e.g. &githubv4.String("foo").
func newGQLStringlike[T ~string](s string) *T {
	if s == "" {
		return nil
	}
	stringlike := T(s)
	return &stringlike
}

func newGQLStringlikePtr[T ~string](s *string) *T {
	if s == nil {
		return nil
	}
	stringlike := T(*s)
	return &stringlike
}

func newGQLIntPtr(i *int32) *githubv4.Int {
	if i == nil {
		return nil
	}
	gi := githubv4.Int(*i)
	return &gi
}
