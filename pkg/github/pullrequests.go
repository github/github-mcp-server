package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/octicons"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
)

// PullRequestRead creates a tool to get details of a specific pull request.
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
 6. get_review_comments - Get review threads on a pull request. Each thread contains logically grouped review comments made on the same code location during pull request reviews. Returns thread metadata and comments with nullable current and original line-range coordinates (line, start_line, original_line, original_start_line). Current coordinates are omitted when unavailable, such as for outdated comments. Use cursor-based pagination (perPage, after) to control results.
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
	// get_review_comments uses GraphQL cursor-based pagination and accepts the
	// `after` cursor. Other methods rely on the `page`/`perPage` parameters
	// added by WithPagination and ignore `after`.
	schema.Properties["after"] = &jsonschema.Schema{
		Type:        "string",
		Description: "Cursor for pagination, used only by the get_review_comments method. Pass the endCursor from the previous page's PageInfo to fetch the next page.",
	}

	return NewTool[PullRequestReadInput, *PullRequestReadOutput](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "pull_request_read",
			OutputSchema: pullRequestReadOutputSchema(),
			Description:  t("TOOL_PULL_REQUEST_READ_DESCRIPTION", "Get information on a specific pull request in GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_PULL_REQUEST_USER_TITLE", "Get details for a single pull request"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input PullRequestReadInput) (*mcp.CallToolResult, *PullRequestReadOutput, error) {
			args, err := discussionNotificationArguments(input)
			if err != nil {
				return nil, nil, err
			}
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

			// attachIFC adds the IFC label to a successful tool result when
			// IFC labels are enabled. Pull request content (descriptions,
			// diffs, comments, reviews) is user-authored and therefore
			// untrusted; confidentiality follows repo visibility. If the
			// visibility lookup fails the label is omitted rather than
			// misclassifying the result.
			attachIFC := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelRepoUserContent)
			}

			switch method {
			case "get":
				result, output, err := getPullRequest(ctx, client, deps, owner, repo, pullNumber)
				return attachIFC(result), output, err
			case "get_diff":
				result, output, err := getPullRequestDiff(ctx, client, deps, owner, repo, pullNumber)
				return attachIFC(result), output, err
			case "get_status":
				result, output, err := getPullRequestStatus(ctx, client, owner, repo, pullNumber)
				return attachIFC(result), output, err
			case "get_files":
				result, output, err := getPullRequestFiles(ctx, client, deps, owner, repo, pullNumber, pagination)
				return attachIFC(result), output, err
			case "get_commits":
				result, output, err := getPullRequestCommits(ctx, client, deps, owner, repo, pullNumber, pagination)
				return attachIFC(result), output, err
			case "get_review_comments":
				gqlClient, err := deps.GetGQLClient(ctx)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to get GitHub GQL client", err), nil, nil
				}
				cursorPagination, err := OptionalCursorPaginationParams(args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				result, output, err := getPullRequestReviewComments(ctx, gqlClient, deps, owner, repo, pullNumber, cursorPagination)
				return attachIFC(result), output, err
			case "get_reviews":
				result, output, err := getPullRequestReviews(ctx, client, deps, owner, repo, pullNumber, pagination)
				return attachIFC(result), output, err
			case "get_comments":
				result, output, err := getIssueComments(ctx, client, deps, owner, repo, pullNumber, pagination)
				return attachIFC(result), pullRequestCommentsOutput(output), err
			case "get_check_runs":
				result, output, err := getPullRequestCheckRuns(ctx, client, owner, repo, pullNumber, pagination)
				return attachIFC(result), output, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		}, normalizePullRequestArguments("read"))
}

func GetPullRequest(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int) (*mcp.CallToolResult, error) {
	result, _, err := getPullRequest(ctx, client, deps, owner, repo, pullNumber)
	return result, err
}

func getPullRequest(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int) (*mcp.CallToolResult, *PullRequestReadOutput, error) {
	cache, err := deps.GetRepoAccessCache(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repo access cache: %w", err)
	}
	ff := deps.GetFlags(ctx)

	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request", resp, body), nil, nil
	}

	if pr == nil {
		return utils.NewToolResultText("null"), &PullRequestReadOutput{}, nil
	}

	if ff.LockdownMode {
		if restricted, err := authorLockdownResult(ctx, cache, owner, repo, pr.GetUser().GetLogin(), lockdownPullRequestRestrictedMessage); restricted != nil || err != nil {
			return restricted, nil, err
		}
	}

	minimalPR := convertToMinimalPullRequest(pr)

	return MarshalledTextResult(minimalPR), &PullRequestReadOutput{PullRequest: &minimalPR}, nil
}

// enforcePullRequestLockdown returns a restricted tool result when lockdown mode is
// enabled and the pull request author is not a safe content source for owner/repo,
// and (nil, nil) otherwise. It fetches the pull request to resolve the author and is
// a no-op that performs no request when lockdown mode is disabled.
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
	result, _, err := getPullRequestDiff(ctx, client, deps, owner, repo, pullNumber)
	return result, err
}

func getPullRequestDiff(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int) (*mcp.CallToolResult, *PullRequestReadOutput, error) {
	if restricted, err := enforcePullRequestLockdown(ctx, client, deps, owner, repo, pullNumber); restricted != nil || err != nil {
		return restricted, nil, err
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
		), nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request diff", resp, body), nil, nil
	}

	defer func() { _ = resp.Body.Close() }()

	// Return the raw response
	return utils.NewToolResultText(string(raw)), &PullRequestReadOutput{Diff: &PullRequestDiffOutput{Diff: string(raw)}}, nil
}

func GetPullRequestStatus(ctx context.Context, client *github.Client, owner, repo string, pullNumber int) (*mcp.CallToolResult, error) {
	result, _, err := getPullRequestStatus(ctx, client, owner, repo, pullNumber)
	return result, err
}

func getPullRequestStatus(ctx context.Context, client *github.Client, owner, repo string, pullNumber int) (*mcp.CallToolResult, *PullRequestReadOutput, error) {
	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request", resp, body), nil, nil
	}

	// Get combined status for the head SHA
	status, resp, err := client.Repositories.GetCombinedStatus(ctx, owner, repo, *pr.Head.SHA, nil)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get combined status",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get combined status", resp, body), nil, nil
	}

	minimalStatus := convertToMinimalCombinedStatus(status)
	r, err := json.Marshal(minimalStatus)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &PullRequestReadOutput{Status: &minimalStatus}, nil
}

func GetPullRequestCheckRuns(ctx context.Context, client *github.Client, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	result, _, err := getPullRequestCheckRuns(ctx, client, owner, repo, pullNumber, pagination)
	return result, err
}

func getPullRequestCheckRuns(ctx context.Context, client *github.Client, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, *PullRequestReadOutput, error) {
	// First get the PR to get the head SHA
	pr, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request",
			resp,
			err,
		), nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request", resp, body), nil, nil
	}

	// Get check runs for the head SHA
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
		), nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get check runs", resp, body), nil, nil
	}

	// Convert to minimal check runs to reduce context usage
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
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), &PullRequestReadOutput{CheckRuns: &minimalResult}, nil
}

func GetPullRequestFiles(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	result, _, err := getPullRequestFiles(ctx, client, deps, owner, repo, pullNumber, pagination)
	return result, err
}

func getPullRequestFiles(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, *PullRequestReadOutput, error) {
	if restricted, err := enforcePullRequestLockdown(ctx, client, deps, owner, repo, pullNumber); restricted != nil || err != nil {
		return restricted, nil, err
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
		), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request files", resp, body), nil, nil
	}

	minimalFiles := convertToMinimalPRFiles(files)

	return MarshalledTextResult(minimalFiles), &PullRequestReadOutput{Files: &minimalFiles}, nil
}

// GetPullRequestCommits returns the commits on a pull request. Under lockdown
// mode it checks the PR author once rather than per commit, since every
// commit on the PR belongs to the same untrusted head branch.
func GetPullRequestCommits(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	result, _, err := getPullRequestCommits(ctx, client, deps, owner, repo, pullNumber, pagination)
	return result, err
}

func getPullRequestCommits(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, *PullRequestReadOutput, error) {
	if restricted, err := enforcePullRequestLockdown(ctx, client, deps, owner, repo, pullNumber); restricted != nil || err != nil {
		return restricted, nil, err
	}

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
		), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request commits", resp, body), nil, nil
	}

	minimalCommits := convertToMinimalPullRequestCommits(commits)

	return MarshalledTextResult(minimalCommits), &PullRequestReadOutput{Commits: &minimalCommits}, nil
}

// GraphQL types for review threads query
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
	ID                githubv4.ID
	Body              githubv4.String
	Path              githubv4.String
	Line              *githubv4.Int
	OriginalLine      *githubv4.Int
	StartLine         *githubv4.Int
	OriginalStartLine *githubv4.Int
	Author            struct {
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
	result, _, err := getPullRequestReviewComments(ctx, gqlClient, deps, owner, repo, pullNumber, pagination)
	return result, err
}

func getPullRequestReviewComments(ctx context.Context, gqlClient *githubv4.Client, deps ToolDependencies, owner, repo string, pullNumber int, pagination CursorPaginationParams) (*mcp.CallToolResult, *PullRequestReadOutput, error) {
	cache, err := deps.GetRepoAccessCache(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repo access cache: %w", err)
	}
	ff := deps.GetFlags(ctx)

	// Convert pagination parameters to GraphQL format
	gqlParams, err := pagination.ToGraphQLParams()
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("invalid pagination parameters: %v", err)), nil, nil
	}

	// Build variables for GraphQL query
	vars := map[string]any{
		"owner":             githubv4.String(owner),
		"repo":              githubv4.String(repo),
		"prNum":             githubv4.Int(int32(pullNumber)), //nolint:gosec // pullNumber is controlled by user input validation
		"first":             githubv4.Int(*gqlParams.First),
		"commentsPerThread": githubv4.Int(100),
	}

	// Add cursor if provided
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
		), nil, nil
	}

	// Lockdown mode filtering
	if ff.LockdownMode {
		if cache == nil {
			return nil, nil, fmt.Errorf("lockdown cache is not configured")
		}

		// Iterate through threads and filter comments
		for i := range query.Repository.PullRequest.ReviewThreads.Nodes {
			thread := &query.Repository.PullRequest.ReviewThreads.Nodes[i]
			filteredComments := make([]reviewCommentNode, 0, len(thread.Comments.Nodes))

			for _, comment := range thread.Comments.Nodes {
				login := string(comment.Author.Login)
				if login != "" {
					isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
					if err != nil {
						return nil, nil, fmt.Errorf("failed to check lockdown mode: %w", err)
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

	threads := convertToMinimalReviewThreadsResponse(query)
	return MarshalledTextResult(threads), &PullRequestReadOutput{ReviewThreads: &threads}, nil
}

func GetPullRequestReviews(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	result, _, err := getPullRequestReviews(ctx, client, deps, owner, repo, pullNumber, pagination)
	return result, err
}

func getPullRequestReviews(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, *PullRequestReadOutput, error) {
	cache, err := deps.GetRepoAccessCache(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repo access cache: %w", err)
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
		), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get pull request reviews", resp, body), nil, nil
	}

	if ff.LockdownMode {
		if cache == nil {
			return nil, nil, fmt.Errorf("lockdown cache is not configured")
		}
		filteredReviews := make([]*github.PullRequestReview, 0, len(reviews))
		for _, review := range reviews {
			login := review.GetUser().GetLogin()
			if login == "" {
				continue
			}
			isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to check lockdown mode: %w", err)
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

	return MarshalledTextResult(minimalReviews), &PullRequestReadOutput{Reviews: &minimalReviews}, nil
}

// PullRequestWriteUIResourceURI is the URI for the create_pull_request tool's MCP App UI resource.
const PullRequestWriteUIResourceURI = "ui://github-mcp-server/pr-write"

// PullRequestEditUIResourceURI is the URI for the update_pull_request tool's MCP App UI resource.
const PullRequestEditUIResourceURI = "ui://github-mcp-server/pr-edit"

// pullRequestWriteFormParams are the parameters the create_pull_request MCP App
// form collects and re-sends on submit. Any other parameter present on a call
// cannot be represented by the form.
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

// CreatePullRequest creates a tool to create a new pull request.
func CreatePullRequest(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[CreatePullRequestInput, *PullRequestWriteOutput](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "create_pull_request",
			OutputSchema: pullRequestWriteOutputSchema(),
			Description:  t("TOOL_CREATE_PULL_REQUEST_DESCRIPTION", "Create a new pull request in a GitHub repository."),
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
		publicRepositoryWriteScopeAccess(),
		func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, input CreatePullRequestInput) (*mcp.CallToolResult, *PullRequestWriteOutput, error) {
			args, err := discussionNotificationArguments(input)
			if err != nil {
				return nil, nil, err
			}
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Hand off to the interactive MCP App form unless this call must
			// execute now (see shouldDeferToForm).
			formArgs, err := pullRequestFormArguments(req, args)
			if err != nil {
				return nil, nil, err
			}
			if shouldDeferToForm(ctx, deps, req, formArgs, pullRequestWriteFormParams) {
				return pullRequestAwaitingFormResult(fmt.Sprintf(
					"An interactive form has been shown to the user for creating a new pull request in %s/%s. "+
						"STOP — do not call any other tools, do not respond as if the pull request was created, "+
						"and do not claim the operation succeeded. The pull request has NOT been created yet; "+
						"only the form was rendered. Wait silently for the user to review and click Submit. "+
						"When they do, the real result will be delivered to your context automatically.",
					owner, repo,
				))
			}

			// When creating PR, title/head/base are required
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
				Title: new(title),
				Head:  head,
				Base:  base,
			}

			if body != "" {
				newPR.Body = new(body)
			}

			newPR.Draft = new(draft)
			newPR.MaintainerCanModify = new(maintainerCanModify)

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

			// Return minimal response with just essential information
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", pr.GetID()),
				URL: pr.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), &PullRequestWriteOutput{PullRequest: pullRequestMutationReference(minimalResponse)}, nil
		}, normalizePullRequestArguments("create"))
}

// UpdatePullRequest creates a tool to update an existing pull request.
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

	st := NewTool[UpdatePullRequestInput, *PullRequestWriteOutput](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "update_pull_request",
			OutputSchema: pullRequestWriteOutputSchema(),
			Description:  t("TOOL_UPDATE_PULL_REQUEST_DESCRIPTION", "Update an existing pull request in a GitHub repository."),
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
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, input UpdatePullRequestInput) (*mcp.CallToolResult, *PullRequestWriteOutput, error) {
			args, err := discussionNotificationArguments(input)
			if err != nil {
				return nil, nil, err
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

			// Hand off to the interactive MCP App form unless this call must
			// execute now (see shouldDeferToForm).
			formArgs, err := pullRequestFormArguments(req, args)
			if err != nil {
				return nil, nil, err
			}
			if shouldDeferToForm(ctx, deps, req, formArgs, pullRequestUpdateFormParams) {
				return pullRequestAwaitingFormResult(fmt.Sprintf(
					"An interactive form has been shown to the user for editing pull request #%d in %s/%s. "+
						"STOP — do not call any other tools, do not respond as if the pull request was updated, "+
						"and do not claim the operation succeeded. The pull request has NOT been updated yet; "+
						"only the form was rendered. Wait silently for the user to review and click Submit. "+
						"When they do, the real result will be delivered to your context automatically.",
					pullNumber, owner, repo,
				))
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
				update.Title = new(title)
				restUpdateNeeded = true
			}

			if body, ok, err := OptionalParamOK[string](args, "body"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				update.Body = new(body)
				restUpdateNeeded = true
			}

			if state, ok, err := OptionalParamOK[string](args, "state"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				update.State = new(state)
				restUpdateNeeded = true
			}

			if base, ok, err := OptionalParamOK[string](args, "base"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				update.Base = &github.PullRequestBranch{Ref: new(base)}
				restUpdateNeeded = true
			}

			if maintainerCanModify, ok, err := OptionalParamOK[bool](args, "maintainer_can_modify"); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			} else if ok {
				update.MaintainerCanModify = new(maintainerCanModify)
				restUpdateNeeded = true
			}

			// Handle reviewers separately
			reviewers, err := OptionalStringArrayParam(args, "reviewers")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// If no updates, no draft change, and no reviewers, return error early
			if !restUpdateNeeded && !draftProvided && len(reviewers) == 0 {
				return utils.NewToolResultError("No update parameters provided."), nil, nil
			}

			// Handle REST API updates (title, body, state, base, maintainer_can_modify)
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
					"prNum": githubv4.Int(pullNumber), // #nosec G115 - pull request numbers are always small positive integers
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
						// Mark as ready for review
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

			// Handle reviewer requests
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

			// Get the final state of the PR to return
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

			// Return minimal response with just essential information
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", finalPR.GetID()),
				URL: finalPR.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return utils.NewToolResultErrorFromErr("Failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), &PullRequestWriteOutput{PullRequest: pullRequestMutationReference(minimalResponse)}, nil
		}, normalizePullRequestArguments("update"))
	st.FeatureRule = pullRequestsConsolidatedRule
	return st
}

// AddReplyToPullRequestComment creates a tool to add a reply or reaction to an existing pull request comment.
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
				Minimum:     new(1.0),
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

	return NewTool[AddReplyToPullRequestCommentInput, *PullRequestCommentReplyOutput](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "add_reply_to_pull_request_comment",
			OutputSchema: pullRequestCommentReplyOutputSchema(),
			Description:  t("TOOL_ADD_REPLY_TO_PULL_REQUEST_COMMENT_DESCRIPTION", "Add a reply and/or reaction to an existing pull request comment. This can create a new comment linked as a reply to the specified comment, add an emoji reaction to the specified comment, or do both. At least one of body or reaction is required."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ADD_REPLY_TO_PULL_REQUEST_COMMENT_USER_TITLE", "Add reply to pull request comment"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input AddReplyToPullRequestCommentInput) (*mcp.CallToolResult, *PullRequestCommentReplyOutput, error) {
			args, err := discussionNotificationArguments(input)
			if err != nil {
				return nil, nil, err
			}
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

			var commentResponse *MinimalResponse
			if hasBody {
				comment, resp, err := client.PullRequests.CreateCommentInReplyTo(ctx, owner, repo, pullNumber, body, commentID)
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

				commentResponse = &MinimalResponse{
					ID:  fmt.Sprintf("%d", comment.GetID()),
					URL: comment.GetHTMLURL(),
				}
			}

			var output PullRequestCommentReplyOutput
			var r []byte
			switch {
			case hasBody && hasReaction:
				output.ReplyAndReaction = &PullRequestReplyAndReactionOutput{
					Comment:  *pullRequestMutationReference(*commentResponse),
					Reaction: PullRequestMutationReference{ID: reactionResponse.ID},
				}
				r, err = json.Marshal(map[string]*MinimalResponse{"comment": commentResponse, "reaction": reactionResponse})
			case hasReaction:
				output.Response = &PullRequestMutationReference{ID: reactionResponse.ID}
				r, err = json.Marshal(reactionResponse)
			default:
				output.Response = pullRequestMutationReference(*commentResponse)
				r, err = json.Marshal(commentResponse)
			}

			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), &output, nil
		}, normalizePullRequestArguments("reply"))
}

// ListPullRequests creates a tool to list pull requests in a GitHub repository.
type ListPullRequestsInput struct {
	Owner     string   `json:"owner"`
	Repo      string   `json:"repo"`
	State     string   `json:"state,omitempty"`
	Head      string   `json:"head,omitempty"`
	Base      string   `json:"base,omitempty"`
	Sort      string   `json:"sort,omitempty"`
	Direction string   `json:"direction,omitempty"`
	Fields    []string `json:"fields,omitempty"`
	Page      *int     `json:"page,omitempty"`
	PerPage   *int     `json:"perPage,omitempty"`
}

type ListPullRequestOutput struct {
	Number             *int             `json:"number,omitempty"`
	Title              *string          `json:"title,omitempty"`
	Body               *string          `json:"body,omitempty"`
	State              *string          `json:"state,omitempty" jsonschema:"Pull request state: open or closed."`
	Draft              *bool            `json:"draft,omitempty"`
	Merged             *bool            `json:"merged,omitempty"`
	MergeableState     *string          `json:"mergeable_state,omitempty"`
	HTMLURL            *string          `json:"html_url,omitempty"`
	User               *MinimalUser     `json:"user,omitempty"`
	Labels             *[]string        `json:"labels,omitempty"`
	Assignees          *[]string        `json:"assignees,omitempty"`
	RequestedReviewers *[]string        `json:"requested_reviewers,omitempty"`
	MergedBy           *string          `json:"merged_by,omitempty"`
	Head               *MinimalPRBranch `json:"head,omitempty"`
	Base               *MinimalPRBranch `json:"base,omitempty"`
	Additions          *int             `json:"additions,omitempty" jsonschema:"Number of lines added."`
	Deletions          *int             `json:"deletions,omitempty" jsonschema:"Number of lines removed."`
	ChangedFiles       *int             `json:"changed_files,omitempty" jsonschema:"Number of files changed."`
	Commits            *int             `json:"commits,omitempty" jsonschema:"Number of commits in the pull request."`
	Comments           *int             `json:"comments,omitempty" jsonschema:"Number of comments on the pull request."`
	CreatedAt          *string          `json:"created_at,omitempty" jsonschema:"Creation time in RFC 3339 format."`
	UpdatedAt          *string          `json:"updated_at,omitempty" jsonschema:"Last update time in RFC 3339 format."`
	ClosedAt           *string          `json:"closed_at,omitempty" jsonschema:"Closing time in RFC 3339 format."`
	MergedAt           *string          `json:"merged_at,omitempty" jsonschema:"Merge time in RFC 3339 format."`
	Milestone          *string          `json:"milestone,omitempty"`
}

func structuredListPullRequestsOutput(pullRequests []MinimalPullRequest, fields []string) []ListPullRequestOutput {
	output := make([]ListPullRequestOutput, 0, len(pullRequests))
	selected := func(field string) bool {
		return len(fields) == 0 || slices.Contains(fields, field)
	}
	for _, pr := range pullRequests {
		item := ListPullRequestOutput{}
		if selected("number") {
			item.Number = new(pr.Number)
		}
		if selected("title") {
			item.Title = new(pr.Title)
		}
		if selected("body") && pr.Body != "" {
			item.Body = new(pr.Body)
		}
		if selected("state") {
			item.State = new(pr.State)
		}
		if selected("draft") {
			item.Draft = new(pr.Draft)
		}
		if selected("merged") {
			item.Merged = new(pr.Merged)
		}
		if selected("mergeable_state") && pr.MergeableState != "" {
			item.MergeableState = new(pr.MergeableState)
		}
		if selected("html_url") {
			item.HTMLURL = new(pr.HTMLURL)
		}
		if selected("user") && pr.User != nil {
			item.User = pr.User
		}
		if selected("labels") && len(pr.Labels) > 0 {
			item.Labels = &pr.Labels
		}
		if selected("assignees") && len(pr.Assignees) > 0 {
			item.Assignees = &pr.Assignees
		}
		if selected("requested_reviewers") && len(pr.RequestedReviewers) > 0 {
			item.RequestedReviewers = &pr.RequestedReviewers
		}
		if selected("merged_by") && pr.MergedBy != "" {
			item.MergedBy = new(pr.MergedBy)
		}
		if selected("head") && pr.Head != nil {
			item.Head = pr.Head
		}
		if selected("base") && pr.Base != nil {
			item.Base = pr.Base
		}
		if selected("additions") && pr.Additions != 0 {
			item.Additions = new(pr.Additions)
		}
		if selected("deletions") && pr.Deletions != 0 {
			item.Deletions = new(pr.Deletions)
		}
		if selected("changed_files") && pr.ChangedFiles != 0 {
			item.ChangedFiles = new(pr.ChangedFiles)
		}
		if selected("commits") && pr.Commits != 0 {
			item.Commits = new(pr.Commits)
		}
		if selected("comments") && pr.Comments != 0 {
			item.Comments = new(pr.Comments)
		}
		if selected("created_at") && pr.CreatedAt != "" {
			item.CreatedAt = new(pr.CreatedAt)
		}
		if selected("updated_at") && pr.UpdatedAt != "" {
			item.UpdatedAt = new(pr.UpdatedAt)
		}
		if selected("closed_at") && pr.ClosedAt != "" {
			item.ClosedAt = new(pr.ClosedAt)
		}
		if selected("merged_at") && pr.MergedAt != "" {
			item.MergedAt = new(pr.MergedAt)
		}
		if selected("milestone") && pr.Milestone != "" {
			item.Milestone = new(pr.Milestone)
		}
		output = append(output, item)
	}
	return output
}

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

	return NewTool[ListPullRequestsInput, []ListPullRequestOutput](
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
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input ListPullRequestsInput) (*mcp.CallToolResult, []ListPullRequestOutput, error) {
			if input.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if input.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}
			pagination := PaginationParams{Page: 1, PerPage: 30}
			if input.Page != nil {
				pagination.Page = *input.Page
			}
			if input.PerPage != nil {
				pagination.PerPage = *input.PerPage
			}

			opts := &github.PullRequestListOptions{
				State:     input.State,
				Head:      input.Head,
				Base:      input.Base,
				Sort:      input.Sort,
				Direction: input.Direction,
				ListOptions: github.ListOptions{
					PerPage: pagination.PerPage,
					Page:    pagination.Page,
				},
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			prs, resp, err := client.PullRequests.List(ctx, input.Owner, input.Repo, opts)
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

			minimalPRs := make([]MinimalPullRequest, 0, len(prs))
			for _, pr := range prs {
				if pr != nil {
					minimalPRs = append(minimalPRs, convertToMinimalPullRequest(pr))
				}
			}

			filtered := false
			var payload any = minimalPRs
			if len(input.Fields) > 0 {
				filteredPRs, err := filterEachField(minimalPRs, input.Fields)
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
			// Pull request titles/bodies are user-authored (untrusted);
			// confidentiality follows repo visibility.
			result = attachRepoVisibilityIFCLabel(ctx, deps, client, input.Owner, input.Repo, result, ifc.LabelRepoUserContent)
			return result, structuredListPullRequestsOutput(minimalPRs, input.Fields), nil
		},
		normalizeTypedReadArguments(nil, false),
	)
}

// MergePullRequest creates a tool to merge a pull request.
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
			"expectedHeadSha": {
				Type:        "string",
				Description: "The expected SHA of the pull request's HEAD ref",
			},
		},
		Required: []string{"owner", "repo", "pullNumber"},
	}

	return NewTool[MergePullRequestInput, *PullRequestMergeOutput](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "merge_pull_request",
			OutputSchema: pullRequestMergeOutputSchema(),
			Description:  t("TOOL_MERGE_PULL_REQUEST_DESCRIPTION", "Merge a pull request in a GitHub repository."),
			Icons:        octicons.Icons("git-merge"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_MERGE_PULL_REQUEST_USER_TITLE", "Merge pull request"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input MergePullRequestInput) (*mcp.CallToolResult, *PullRequestMergeOutput, error) {
			args, err := discussionNotificationArguments(input)
			if err != nil {
				return nil, nil, err
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
			expectedHeadSHA, err := OptionalParam[string](args, "expectedHeadSha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			options := &github.PullRequestOptions{
				CommitTitle: commitTitle,
				SHA:         expectedHeadSHA,
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

			output := &PullRequestMergeOutput{}
			if result != nil {
				output.Result = &PullRequestMergeResult{SHA: result.SHA, Merged: result.Merged, Message: result.Message}
			}
			return utils.NewToolResultText(string(r)), output, nil
		}, normalizePullRequestArguments("merge"))
}

// SearchPullRequests creates a tool to search for pull requests.
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

	return NewTool[SearchIssuesInput, SearchIssuesOutput](
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
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input SearchIssuesInput) (*mcp.CallToolResult, SearchIssuesOutput, error) {
			options := []searchOption{ifcSearchPostProcessOption(ctx, deps)}
			options = append(options, withFieldsFiltering(deps, "search_pull_requests", input.Fields))
			result, response, err := searchHandler(ctx, deps.GetClient, input, "pr", "failed to search pull requests", options...)
			return result, structuredSearchIssuesOutput(response, input.Fields), err
		},
		normalizeTypedReadArguments(nil, false),
	)
}

// UpdatePullRequestBranch creates a tool to update a pull request branch with the latest changes from the base branch.
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

	return NewTool[UpdatePullRequestBranchInput, *PullRequestBranchUpdateOutput](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "update_pull_request_branch",
			OutputSchema: pullRequestBranchUpdateOutputSchema(),
			Description:  t("TOOL_UPDATE_PULL_REQUEST_BRANCH_DESCRIPTION", "Update the branch of a pull request with the latest changes from the base branch."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_PULL_REQUEST_BRANCH_USER_TITLE", "Update pull request branch"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input UpdatePullRequestBranchInput) (*mcp.CallToolResult, *PullRequestBranchUpdateOutput, error) {
			args, err := discussionNotificationArguments(input)
			if err != nil {
				return nil, nil, err
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
			expectedHeadSHA, err := OptionalParam[string](args, "expectedHeadSha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			opts := &github.PullRequestBranchUpdateOptions{}
			if expectedHeadSHA != "" {
				opts.ExpectedHeadSHA = new(expectedHeadSHA)
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			result, resp, err := client.PullRequests.UpdateBranch(ctx, owner, repo, pullNumber, opts)
			if err != nil {
				// Check if it's an acceptedError. An acceptedError indicates that the update is in progress,
				// and it's not a real error.
				if resp != nil && resp.StatusCode == http.StatusAccepted && isAcceptedError(err) {
					message := "Pull request branch update is in progress"
					return utils.NewToolResultText(message), &PullRequestBranchUpdateOutput{Result: &PullRequestBranchUpdateResult{Message: &message}}, nil
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

			output := &PullRequestBranchUpdateOutput{}
			if result != nil {
				output.Result = &PullRequestBranchUpdateResult{Message: result.Message}
			}
			return utils.NewToolResultText(string(r)), output, nil
		}, normalizePullRequestArguments("branch"))
}

type PullRequestReviewWriteParams struct {
	Method           string
	Owner            string
	Repo             string
	PullNumber       int32
	Body             string
	Event            string
	CommitID         *string
	ThreadID         string
	ResolutionReason *string
}

func PullRequestReviewWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return pullRequestReviewWrite(t, false, toolConfig{})
}

// PullRequestReviewWriteWithResolutionReason creates the feature-gated review write variant with resolution reasons.
func PullRequestReviewWriteWithResolutionReason(t translations.TranslationHelperFunc, opts ...ToolOption) inventory.ServerTool {
	cfg := newToolConfig(opts)
	st := pullRequestReviewWrite(t, true, cfg)
	if cfg.hostType == utils.HostTypeGHES {
		st.Enabled = func(context.Context) (bool, error) { return false, nil }
	}
	return st
}

func pullRequestReviewWrite(t translations.TranslationHelperFunc, withResolutionReason bool, cfg toolConfig) inventory.ServerTool {
	withResolutionReason = withResolutionReason && cfg.hostType != utils.HostTypeGHES

	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			// Either we need the PR GQL Id directly, or we need owner, repo and PR number to look it up.
			// Since our other Pull Request tools are working with the REST Client, will handle the lookup
			// internally for now.
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
	if withResolutionReason {
		schema.Properties["resolutionReason"] = &jsonschema.Schema{
			Type:        "string",
			Description: "Optional reason for resolving a Copilot code review thread: addressed, wont-fix, or invalid.",
		}
	}

	st := NewTool[PullRequestReviewWriteInput, *RepositoryMessageOutput](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "pull_request_review_write",
			OutputSchema: pullRequestOutputSchema[RepositoryMessageOutput](),
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
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input PullRequestReviewWriteInput) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
			args, err := discussionNotificationArguments(input)
			if err != nil {
				return nil, nil, err
			}
			var params PullRequestReviewWriteParams
			if err := mapstructure.WeakDecode(args, &params); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Given our owner, repo and PR number, lookup the GQL ID of the PR.
			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			var result *mcp.CallToolResult

			switch params.Method {
			case "create":
				result, err = CreatePullRequestReview(ctx, client, params)
				return pullRequestMessageResult(result, err)
			case "submit_pending":
				result, err = SubmitPendingPullRequestReview(ctx, client, params)
				return pullRequestMessageResult(result, err)
			case "delete_pending":
				result, err = DeletePendingPullRequestReview(ctx, client, params)
				return pullRequestMessageResult(result, err)
			case "resolve_thread":
				if !withResolutionReason {
					result, err = ResolveReviewThread(ctx, client, params.ThreadID, true)
					return pullRequestMessageResult(result, err)
				}
				result, err = ResolveReviewThreadWithReason(ctx, client, params.ThreadID, params.ResolutionReason, true)
				return pullRequestMessageResult(result, err)
			case "unresolve_thread":
				result, err = ResolveReviewThread(ctx, client, params.ThreadID, false)
				return pullRequestMessageResult(result, err)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", params.Method)), nil, nil
			}
		}, normalizePullRequestReviewWriteArguments)
	if withResolutionReason {
		st.FeatureRule = inventory.NewFeatureRule(
			[]inventory.FeatureFlag{FeatureFlagThreadResolutionReason, inventory.FeatureFlag(FeatureFlagPullRequestsGranular)},
			func(featureAsBool inventory.FeatureResolver) bool {
				return featureAsBool(FeatureFlagThreadResolutionReason) &&
					!featureAsBool(inventory.FeatureFlag(FeatureFlagPullRequestsGranular))
			},
		)
	} else {
		if cfg.hostType == utils.HostTypeGHES {
			st.FeatureRule = pullRequestsConsolidatedRule
		} else {
			st.FeatureRule = inventory.NewFeatureRule(
				[]inventory.FeatureFlag{FeatureFlagThreadResolutionReason, inventory.FeatureFlag(FeatureFlagPullRequestsGranular)},
				func(featureAsBool inventory.FeatureResolver) bool {
					return !featureAsBool(FeatureFlagThreadResolutionReason) &&
						!featureAsBool(inventory.FeatureFlag(FeatureFlagPullRequestsGranular))
				},
			)
		}
	}
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

	// Now we have the GQL ID, we can create a review
	var addPullRequestReviewMutation struct {
		AddPullRequestReview struct {
			PullRequestReview struct {
				ID githubv4.ID // We don't need this, but a selector is required or GQL complains.
			}
		} `graphql:"addPullRequestReview(input: $input)"`
	}

	addPullRequestReviewInput := githubv4.AddPullRequestReviewInput{
		PullRequestID: getPullRequestQuery.Repository.PullRequest.ID,
		CommitOID:     newGQLStringlikePtr[githubv4.GitObjectID](params.CommitID),
	}

	// Event and Body are provided if we submit a review
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

	// Return nothing interesting, just indicate success for the time being.
	// In future, we may want to return the review ID, but for the moment, we're not leaking
	// API implementation details to the LLM.
	if params.Event == "" {
		return utils.NewToolResultText("pending pull request created"), nil
	}
	return utils.NewToolResultText("pull request review submitted successfully"), nil
}

func SubmitPendingPullRequestReview(ctx context.Context, client *githubv4.Client, params PullRequestReviewWriteParams) (*mcp.CallToolResult, error) {
	review, result := getPendingPullRequestReviewForViewer(ctx, client, params.Owner, params.Repo, params.PullNumber)
	if result != nil {
		return result, nil
	}

	// Prepare the mutation
	var submitPullRequestReviewMutation struct {
		SubmitPullRequestReview struct {
			PullRequestReview struct {
				ID githubv4.ID // We don't need this, but a selector is required or GQL complains.
			}
		} `graphql:"submitPullRequestReview(input: $input)"`
	}

	if err := client.Mutate(
		ctx,
		&submitPullRequestReviewMutation,
		githubv4.SubmitPullRequestReviewInput{
			PullRequestReviewID: review,
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

	// Return nothing interesting, just indicate success for the time being.
	// In future, we may want to return the review ID, but for the moment, we're not leaking
	// API implementation details to the LLM.
	return utils.NewToolResultText("pending pull request review successfully submitted"), nil
}

func DeletePendingPullRequestReview(ctx context.Context, client *githubv4.Client, params PullRequestReviewWriteParams) (*mcp.CallToolResult, error) {
	review, result := getPendingPullRequestReviewForViewer(ctx, client, params.Owner, params.Repo, params.PullNumber)
	if result != nil {
		return result, nil
	}

	// Prepare the mutation
	var deletePullRequestReviewMutation struct {
		DeletePullRequestReview struct {
			PullRequestReview struct {
				ID githubv4.ID // We don't need this, but a selector is required or GQL complains.
			}
		} `graphql:"deletePullRequestReview(input: $input)"`
	}

	if err := client.Mutate(
		ctx,
		&deletePullRequestReviewMutation,
		githubv4.DeletePullRequestReviewInput{
			PullRequestReviewID: review,
		},
		nil,
	); err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	// Return nothing interesting, just indicate success for the time being.
	// In future, we may want to return the review ID, but for the moment, we're not leaking
	// API implementation details to the LLM.
	return utils.NewToolResultText("pending pull request review successfully deleted"), nil
}

// ResolveReviewThreadInput is the GraphQL input for resolving a review thread.
type ResolveReviewThreadInput struct {
	ThreadID         githubv4.ID      `json:"threadId"`
	ResolutionReason *githubv4.String `json:"resolutionReason,omitempty"`
}

// ResolveReviewThread resolves or unresolves a PR review thread using GraphQL mutations.
func ResolveReviewThread(ctx context.Context, client *githubv4.Client, threadID string, resolve bool) (*mcp.CallToolResult, error) {
	return ResolveReviewThreadWithReason(ctx, client, threadID, nil, resolve)
}

// ResolveReviewThreadWithReason resolves or unresolves a PR review thread with an optional resolution reason.
func ResolveReviewThreadWithReason(ctx context.Context, client *githubv4.Client, threadID string, resolutionReason *string, resolve bool) (*mcp.CallToolResult, error) {
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

		input := ResolveReviewThreadInput{
			ThreadID:         githubv4.ID(threadID),
			ResolutionReason: newGQLStringlikePtr[githubv4.String](resolutionReason),
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

// AddCommentToPendingReviewParams contains the parameters for adding a comment to a pending review.
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

// AddCommentToPendingReviewCall adds a review comment to the viewer's pending pull request review.
func AddCommentToPendingReviewCall(ctx context.Context, client *githubv4.Client, params AddCommentToPendingReviewParams) (*mcp.CallToolResult, error) {
	review, result := getPendingPullRequestReviewForViewer(ctx, client, params.Owner, params.Repo, params.PullNumber)
	if result != nil {
		return result, nil
	}

	// Create a new review thread comment on the review.
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
			Path:                newGQLStringlikePtr[githubv4.String](&params.Path),
			Body:                githubv4.String(params.Body),
			SubjectType:         newGQLStringlikePtr[githubv4.PullRequestReviewThreadSubjectType](&params.SubjectType),
			Line:                newGQLIntPtr(params.Line),
			Side:                newGQLStringlikePtr[githubv4.DiffSide](params.Side),
			StartLine:           newGQLIntPtr(params.StartLine),
			StartSide:           newGQLStringlikePtr[githubv4.DiffSide](params.StartSide),
			PullRequestReviewID: review,
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

type pendingReviewAuthor struct {
	Bot struct {
		ID githubv4.ID `graphql:"botId: id"`
	} `graphql:"... on Bot"`
	EnterpriseUserAccount struct {
		ID githubv4.ID `graphql:"enterpriseUserAccountId: id"`
	} `graphql:"... on EnterpriseUserAccount"`
	Mannequin struct {
		ID githubv4.ID `graphql:"mannequinId: id"`
	} `graphql:"... on Mannequin"`
	Organization struct {
		ID githubv4.ID `graphql:"organizationId: id"`
	} `graphql:"... on Organization"`
	User struct {
		ID githubv4.ID `graphql:"userId: id"`
	} `graphql:"... on User"`
}

func (a pendingReviewAuthor) id() githubv4.ID {
	for _, id := range []githubv4.ID{
		a.Bot.ID,
		a.EnterpriseUserAccount.ID,
		a.Mannequin.ID,
		a.Organization.ID,
		a.User.ID,
	} {
		if id != nil {
			return id
		}
	}
	return nil
}

func getPendingPullRequestReviewForViewer(ctx context.Context, client *githubv4.Client, owner, repo string, pullNumber int32) (*githubv4.ID, *mcp.CallToolResult) {
	var getViewerQuery struct {
		Viewer struct {
			ID githubv4.ID
		}
	}

	if err := client.Query(ctx, &getViewerQuery, nil); err != nil {
		return nil, ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get current user",
			err,
		)
	}

	vars := map[string]any{
		"after":  (*githubv4.String)(nil),
		"owner":  githubv4.String(owner),
		"name":   githubv4.String(repo),
		"prNum":  githubv4.Int(pullNumber),
		"states": []githubv4.PullRequestReviewState{githubv4.PullRequestReviewStatePending},
	}

	for {
		var getPendingReviewsQuery struct {
			Repository struct {
				PullRequest struct {
					Reviews struct {
						Nodes []struct {
							ID     githubv4.ID
							Author pendingReviewAuthor
						}
						PageInfo struct {
							HasNextPage githubv4.Boolean
							EndCursor   githubv4.String
						}
					} `graphql:"reviews(first: 100, after: $after, states: $states)"`
				} `graphql:"pullRequest(number: $prNum)"`
			} `graphql:"repository(owner: $owner, name: $name)"`
		}

		if err := client.Query(ctx, &getPendingReviewsQuery, vars); err != nil {
			return nil, ghErrors.NewGitHubGraphQLErrorResponse(ctx,
				"failed to get pending pull request reviews",
				err,
			)
		}

		for _, review := range getPendingReviewsQuery.Repository.PullRequest.Reviews.Nodes {
			if review.Author.id() == getViewerQuery.Viewer.ID {
				reviewID := review.ID
				return &reviewID, nil
			}
		}

		if !getPendingReviewsQuery.Repository.PullRequest.Reviews.PageInfo.HasNextPage {
			return nil, utils.NewToolResultError("No pending review found for the viewer")
		}
		vars["after"] = githubv4.NewString(getPendingReviewsQuery.Repository.PullRequest.Reviews.PageInfo.EndCursor)
	}
}

// AddCommentToPendingReview creates a tool to add a comment to a pull request review.
func AddCommentToPendingReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			// Ideally, for performance sake this would just accept the pullRequestReviewID. However, we would need to
			// add a new tool to get that ID for clients that aren't in the same context as the original pending review
			// creation. So for now, we'll just accept the owner, repo and pull number and assume this is adding a comment
			// the latest review from a user, since only one can be active at a time. It can later be extended with
			// a pullRequestReviewID parameter if targeting other reviews is desired:
			// mcp.WithString("pullRequestReviewID",
			// 	mcp.Required(),
			// 	mcp.Description("The ID of the pull request review to add a comment to"),
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

	st := NewTool[AddCommentToPendingReviewInput, *RepositoryMessageOutput](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "add_comment_to_pending_review",
			OutputSchema: pullRequestOutputSchema[RepositoryMessageOutput](),
			Description:  t("TOOL_ADD_COMMENT_TO_PENDING_REVIEW_DESCRIPTION", "Add review comment to the requester's latest pending pull request review. A pending review needs to already exist to call this (check with the user if not sure)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ADD_COMMENT_TO_PENDING_REVIEW_USER_TITLE", "Add review comment to the requester's latest pending pull request review"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input AddCommentToPendingReviewInput) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
			args, err := discussionNotificationArguments(input)
			if err != nil {
				return nil, nil, err
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
				PullNumber:  int32(pullNumber), // #nosec G115 - PR numbers are always small positive integers
				Path:        path,
				Body:        body,
				SubjectType: subjectType,
				Line:        linePtr,
				Side:        sidePtr,
				StartLine:   startLinePtr,
				StartSide:   startSidePtr,
			})
			return pullRequestMessageResult(result, err)
		}, normalizePullRequestArguments("pending_comment"))
	st.FeatureRule = pullRequestsConsolidatedRule
	return st
}

// newGQLString like takes something that approximates a string (of which there are many types in shurcooL/githubv4)
// and constructs a pointer to it, or nil if the string is empty. This is extremely useful because when we parse
// params from the MCP request, we need to convert them to types that are pointers of type def strings and it's
// not possible to take a pointer of an anonymous value e.g. &githubv4.String("foo").
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
