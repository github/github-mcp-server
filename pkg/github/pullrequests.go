package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/sanitize"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
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
 3. get_status - Get status of a head commit in a pull request. This reflects status of builds and checks.
 4. get_files - Get the list of files changed in a pull request. Use with pagination parameters to control the number of results returned.
 5. get_review_comments - Get review threads on a pull request. Each thread contains logically grouped review comments made on the same code location during pull request reviews. Returns threads with metadata (isResolved, isOutdated, isCollapsed) and their associated comments. Use cursor-based pagination (perPage, after) to control results.
 6. get_reviews - Get the reviews on a pull request. When asked for review comments, use get_review_comments method.
 7. get_comments - Get comments on a pull request. Use this if user doesn't specifically want review comments. Use with pagination parameters to control the number of results returned.
`,
				Enum: []any{"get", "get_diff", "get_status", "get_files", "get_review_comments", "get_reviews", "get_comments"},
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

			switch method {
			case "get":
				result, err := GetPullRequest(ctx, client, deps.GetRepoAccessCache(), owner, repo, pullNumber, deps.GetFlags())
				return result, nil, err
			case "get_diff":
				result, err := GetPullRequestDiff(ctx, client, owner, repo, pullNumber)
				return result, nil, err
			case "get_status":
				result, err := GetPullRequestStatus(ctx, client, owner, repo, pullNumber)
				return result, nil, err
			case "get_files":
				result, err := GetPullRequestFiles(ctx, client, owner, repo, pullNumber, pagination)
				return result, nil, err
			case "get_review_comments":
				gqlClient, err := deps.GetGQLClient(ctx)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to get GitHub GQL client", err), nil, nil
				}
				cursorPagination, err := OptionalCursorPaginationParams(args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				result, err := GetPullRequestReviewComments(ctx, gqlClient, deps.GetRepoAccessCache(), owner, repo, pullNumber, cursorPagination, deps.GetFlags())
				return result, nil, err
			case "get_reviews":
				result, err := GetPullRequestReviews(ctx, client, deps.GetRepoAccessCache(), owner, repo, pullNumber, deps.GetFlags())
				return result, nil, err
			case "get_comments":
				result, err := GetIssueComments(ctx, client, deps.GetRepoAccessCache(), owner, repo, pullNumber, pagination, deps.GetFlags())
				return result, nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		})
}

func GetPullRequest(ctx context.Context, client *github.Client, cache *lockdown.RepoAccessCache, owner, repo string, pullNumber int, ff FeatureFlags) (*mcp.CallToolResult, error) {
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

	// sanitize title/body on response
	if pr != nil {
		if pr.Title != nil {
			pr.Title = github.Ptr(sanitize.Sanitize(*pr.Title))
		}
		if pr.Body != nil {
			pr.Body = github.Ptr(sanitize.Sanitize(*pr.Body))
		}
	}

	if ff.LockdownMode {
		if cache == nil {
			return nil, fmt.Errorf("lockdown cache is not configured")
		}
		login := pr.GetUser().GetLogin()
		if login != "" {
			isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
			if err != nil {
				return nil, fmt.Errorf("failed to check content removal: %w", err)
			}

			if !isSafeContent {
				return utils.NewToolResultError("access to pull request is restricted by lockdown mode"), nil
			}
		}
	}

	r, err := json.Marshal(pr)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func GetPullRequestDiff(ctx context.Context, client *github.Client, owner, repo string, pullNumber int) (*mcp.CallToolResult, error) {
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

	// Return the raw response
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

	// Get combined status for the head SHA
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

func GetPullRequestFiles(ctx context.Context, client *github.Client, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
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

	r, err := json.Marshal(files)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
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

func GetPullRequestReviewComments(ctx context.Context, gqlClient *githubv4.Client, cache *lockdown.RepoAccessCache, owner, repo string, pullNumber int, pagination CursorPaginationParams, ff FeatureFlags) (*mcp.CallToolResult, error) {
	// Convert pagination parameters to GraphQL format
	gqlParams, err := pagination.ToGraphQLParams()
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("invalid pagination parameters: %v", err)), nil
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
		), nil
	}

	// Lockdown mode filtering
	if ff.LockdownMode {
		if cache == nil {
			return nil, fmt.Errorf("lockdown cache is not configured")
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

	// Build response with review threads and pagination info
	response := map[string]any{
		"reviewThreads": query.Repository.PullRequest.ReviewThreads.Nodes,
		"pageInfo": map[string]any{
			"hasNextPage":     query.Repository.PullRequest.ReviewThreads.PageInfo.HasNextPage,
			"hasPreviousPage": query.Repository.PullRequest.ReviewThreads.PageInfo.HasPreviousPage,
			"startCursor":     string(query.Repository.PullRequest.ReviewThreads.PageInfo.StartCursor),
			"endCursor":       string(query.Repository.PullRequest.ReviewThreads.PageInfo.EndCursor),
		},
		"totalCount": int(query.Repository.PullRequest.ReviewThreads.TotalCount),
	}

	r, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func GetPullRequestReviews(ctx context.Context, client *github.Client, cache *lockdown.RepoAccessCache, owner, repo string, pullNumber int, ff FeatureFlags) (*mcp.CallToolResult, error) {
	reviews, resp, err := client.PullRequests.ListReviews(ctx, owner, repo, pullNumber, nil)
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
			if login != "" {
				isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
				if err != nil {
					return nil, fmt.Errorf("failed to check lockdown mode: %w", err)
				}
				if isSafeContent {
					filteredReviews = append(filteredReviews, review)
				}
				reviews = filteredReviews
			}
		}
	}

	r, err := json.Marshal(reviews)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

// CreatePullRequest creates a tool to create a new pull request.
func CreatePullRequest(t translations.TranslationHelperFunc) inventory.ServerTool {
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
		},
		Required: []string{"owner", "repo", "title", "head", "base"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "create_pull_request",
			Description: t("TOOL_CREATE_PULL_REQUEST_DESCRIPTION", "Create a new pull request in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_PULL_REQUEST_USER_TITLE", "Open new pull request"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
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
			title, err := RequiredParam[string](args, "title")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			head, err := RequiredParam[string](args, "head")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			base, err := RequiredParam[string](args, "base")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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

			newPR := &github.NewPullRequest{
				Title: github.Ptr(title),
				Head:  github.Ptr(head),
				Base:  github.Ptr(base),
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
			pr, resp, err := client.PullRequests.Create(ctx, owner, repo, newPR)
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

			// Return minimal response with just essential information
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
				Description: "GitHub usernames to request reviews from",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
		},
		Required: []string{"owner", "repo", "pullNumber"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "update_pull_request",
			Description: t("TOOL_UPDATE_PULL_REQUEST_DESCRIPTION", "Update an existing pull request in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_PULL_REQUEST_USER_TITLE", "Edit pull request"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
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
			pullNumber, err := RequiredInt(args, "pullNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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

				err = gqlClient.Query(ctx, &prQuery, map[string]interface{}{
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

				reviewersRequest := github.ReviewersRequest{
					Reviewers: reviewers,
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

			return utils.NewToolResultText(string(r)), nil, nil
		})
}

// ListPullRequests creates a tool to list and filter repository pull requests.
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

			// sanitize title/body on each PR
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

			r, err := json.Marshal(prs)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
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
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			result, err := searchHandler(ctx, deps.GetClient, args, "pr", "failed to search pull requests")
			return result, nil, err
		})
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
				// Check if it's an acceptedError. An acceptedError indicates that the update is in progress,
				// and it's not a real error.
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
}

func PullRequestReviewWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			// Either we need the PR GQL Id directly, or we need owner, repo and PR number to look it up.
			// Since our other Pull Request tools are working with the REST Client, will handle the lookup
			// internally for now.
			"method": {
				Type:        "string",
				Description: `The write operation to perform on pull request review.`,
				Enum:        []any{"create", "submit_pending", "delete_pending"},
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
		},
		Required: []string{"method", "owner", "repo", "pullNumber"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name: "pull_request_review_write",
			Description: t("TOOL_PULL_REQUEST_REVIEW_WRITE_DESCRIPTION", `Create and/or submit, delete review of a pull request.

Available methods:
- create: Create a new review of a pull request. If "event" parameter is provided, the review is submitted. If "event" is omitted, a pending review is created.
- submit_pending: Submit an existing pending review of a pull request. This requires that a pending review exists for the current user on the specified pull request. The "body" and "event" parameters are used when submitting the review.
- delete_pending: Delete an existing pending review of a pull request. This requires that a pending review exists for the current user on the specified pull request.
`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_PULL_REQUEST_REVIEW_WRITE_USER_TITLE", "Write operations (create, submit, delete) on pull request reviews."),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			var params PullRequestReviewWriteParams
			if err := mapstructure.Decode(args, &params); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Given our owner, repo and PR number, lookup the GQL ID of the PR.
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
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", params.Method)), nil, nil
			}
		})
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
	// First we'll get the current user
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

	if err := client.Query(context.Background(), &getLatestReviewForViewerQuery, vars); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get latest review for current user",
			err,
		), nil
	}

	// Validate there is one review and the state is pending
	if len(getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes) == 0 {
		return utils.NewToolResultError("No pending review found for the viewer"), nil
	}

	review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
	if review.State != githubv4.PullRequestReviewStatePending {
		errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
		return utils.NewToolResultError(errText), nil
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

	// Return nothing interesting, just indicate success for the time being.
	// In future, we may want to return the review ID, but for the moment, we're not leaking
	// API implementation details to the LLM.
	return utils.NewToolResultText("pending pull request review successfully submitted"), nil
}

func DeletePendingPullRequestReview(ctx context.Context, client *githubv4.Client, params PullRequestReviewWriteParams) (*mcp.CallToolResult, error) {
	// First we'll get the current user
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

	if err := client.Query(context.Background(), &getLatestReviewForViewerQuery, vars); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
			"failed to get latest review for current user",
			err,
		), nil
	}

	// Validate there is one review and the state is pending
	if len(getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes) == 0 {
		return utils.NewToolResultError("No pending review found for the viewer"), nil
	}

	review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
	if review.State != githubv4.PullRequestReviewStatePending {
		errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
		return utils.NewToolResultError(errText), nil
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
			PullRequestReviewID: &review.ID,
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

	return NewTool(
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
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			var params struct {
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
			if err := mapstructure.Decode(args, &params); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GQL client", err), nil, nil
			}

			// First we'll get the current user
			var getViewerQuery struct {
				Viewer struct {
					Login githubv4.String
				}
			}

			if err := client.Query(ctx, &getViewerQuery, nil); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
					"failed to get current user",
					err,
				), nil, nil
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

			if err := client.Query(context.Background(), &getLatestReviewForViewerQuery, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx,
					"failed to get latest review for current user",
					err,
				), nil, nil
			}

			// Validate there is one review and the state is pending
			if len(getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes) == 0 {
				return utils.NewToolResultError("No pending review found for the viewer"), nil, nil
			}

			review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
			if review.State != githubv4.PullRequestReviewStatePending {
				errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
				return utils.NewToolResultError(errText), nil, nil
			}

			// Then we can create a new review thread comment on the review.
			var addPullRequestReviewThreadMutation struct {
				AddPullRequestReviewThread struct {
					Thread struct {
						ID githubv4.ID // We don't need this, but a selector is required or GQL complains.
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
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if addPullRequestReviewThreadMutation.AddPullRequestReviewThread.Thread.ID == nil {
				return utils.NewToolResultError(`Failed to add comment to pending review. Possible reasons:
	- The line number doesn't exist in the pull request diff
	- The file path is incorrect
	- The side (LEFT/RIGHT) is invalid for the specified line
`), nil, nil
			}

			// Return nothing interesting, just indicate success for the time being.
			// In future, we may want to return the review ID, but for the moment, we're not leaking
			// API implementation details to the LLM.
			return utils.NewToolResultText("pull request review comment successfully added to pending review"), nil, nil
		})
}

// RequestCopilotReview creates a tool to request a Copilot review for a pull request.
// Note that this tool will not work on GHES where this feature is unsupported. In future, we should not expose this
// tool if the configured host does not support it.
func RequestCopilotReview(t translations.TranslationHelperFunc) inventory.ServerTool {
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
		},
		Required: []string{"owner", "repo", "pullNumber"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "request_copilot_review",
			Description: t("TOOL_REQUEST_COPILOT_REVIEW_DESCRIPTION", "Request a GitHub Copilot code review for a pull request. Use this for automated feedback on pull requests, usually before requesting a human reviewer."),
			Icons:       octicons.Icons("copilot"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_REQUEST_COPILOT_REVIEW_USER_TITLE", "Request Copilot review"),
				ReadOnlyHint: false,
			},
			InputSchema: schema,
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

			pullNumber, err := RequiredInt(args, "pullNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			_, resp, err := client.PullRequests.RequestReviewers(
				ctx,
				owner,
				repo,
				pullNumber,
				github.ReviewersRequest{
					// The login name of the copilot reviewer bot
					Reviewers: []string{"copilot-pull-request-reviewer[bot]"},
				},
			)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to request copilot review",
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
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to request copilot review", resp, bodyBytes), nil, nil
			}

			// Return nothing on success, as there's not much value in returning the Pull Request itself
			return utils.NewToolResultText(""), nil, nil
		})
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
