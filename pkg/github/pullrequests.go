package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/go-github/v76/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/sanitize"
	"github.com/github/github-mcp-server/pkg/translations"
)

// GetPullRequest creates a tool to get details of a specific pull request.
func PullRequestRead(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("pull_request_read",
			mcp.WithDescription(t("TOOL_PULL_REQUEST_READ_DESCRIPTION", "Get information on a specific pull request in GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PULL_REQUEST_USER_TITLE", "Get details for a single pull request"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("method",
				mcp.Required(),
				mcp.Description(`Action to specify what pull request data needs to be retrieved from GitHub. 
Possible options: 
 1. get - Get details of a specific pull request.
 2. get_diff - Get the diff of a pull request.
 3. get_status - Get status of a head commit in a pull request. This reflects status of builds and checks.
 4. get_files - Get the list of files changed in a pull request. Use with pagination parameters to control the number of results returned.
 5. get_review_comments - Get the review comments on a pull request. They are comments made on a portion of the unified diff during a pull request review. Use with pagination parameters to control the number of results returned.
 6. get_reviews - Get the reviews on a pull request. When asked for review comments, use get_review_comments method.
 7. get_comments - Get comments on a pull request. Use this if user doesn't specifically want review comments. Use with pagination parameters to control the number of results returned.
`),

				mcp.Enum("get", "get_diff", "get_status", "get_files", "get_review_comments", "get_reviews", "get_comments"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			method, err := RequiredParam[string](request, "method")
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
			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {

			case "get":
				return GetPullRequest(ctx, client, owner, repo, pullNumber)
			case "get_diff":
				return GetPullRequestDiff(ctx, client, owner, repo, pullNumber)
			case "get_status":
				return GetPullRequestStatus(ctx, client, owner, repo, pullNumber)
			case "get_files":
				return GetPullRequestFiles(ctx, client, owner, repo, pullNumber, pagination)
			case "get_review_comments":
				return GetPullRequestReviewComments(ctx, client, owner, repo, pullNumber, pagination)
			case "get_reviews":
				return GetPullRequestReviews(ctx, client, owner, repo, pullNumber)
			case "get_comments":
				return GetIssueComments(ctx, client, owner, repo, pullNumber, pagination)
			default:
				return nil, fmt.Errorf("unknown method: %s", method)
			}
		}
}

func GetPullRequest(ctx context.Context, client *github.Client, owner, repo string, pullNumber int) (*mcp.CallToolResult, error) {
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
		return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request: %s", string(body))), nil
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

	r, err := json.Marshal(pr)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request diff: %s", string(body))), nil
	}

	defer func() { _ = resp.Body.Close() }()

	// Return the raw response
	return mcp.NewToolResultText(string(raw)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request: %s", string(body))), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("failed to get combined status: %s", string(body))), nil
	}

	r, err := json.Marshal(status)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
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
		return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request files: %s", string(body))), nil
	}

	r, err := json.Marshal(files)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func GetPullRequestReviewComments(ctx context.Context, client *github.Client, owner, repo string, pullNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	opts := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{
			PerPage: pagination.PerPage,
			Page:    pagination.Page,
		},
	}

	comments, resp, err := client.PullRequests.ListComments(ctx, owner, repo, pullNumber, opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get pull request review comments",
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
		return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request review comments: %s", string(body))), nil
	}

	r, err := json.Marshal(comments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func GetPullRequestReviews(ctx context.Context, client *github.Client, owner, repo string, pullNumber int) (*mcp.CallToolResult, error) {
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
		return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request reviews: %s", string(body))), nil
	}

	r, err := json.Marshal(reviews)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// CreatePullRequest creates a tool to create a new pull request.
func CreatePullRequest(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("create_pull_request",
			mcp.WithDescription(t("TOOL_CREATE_PULL_REQUEST_DESCRIPTION", "Create a new pull request in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_PULL_REQUEST_USER_TITLE", "Open new pull request"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("title",
				mcp.Required(),
				mcp.Description("PR title"),
			),
			mcp.WithString("body",
				mcp.Description("PR description"),
			),
			mcp.WithString("head",
				mcp.Required(),
				mcp.Description("Branch containing changes"),
			),
			mcp.WithString("base",
				mcp.Required(),
				mcp.Description("Branch to merge into"),
			),
			mcp.WithBoolean("draft",
				mcp.Description("Create as draft PR"),
			),
			mcp.WithBoolean("maintainer_can_modify",
				mcp.Description("Allow maintainer edits"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			title, err := RequiredParam[string](request, "title")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			head, err := RequiredParam[string](request, "head")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			base, err := RequiredParam[string](request, "base")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			body, err := OptionalParam[string](request, "body")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			draft, err := OptionalParam[bool](request, "draft")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			maintainerCanModify, err := OptionalParam[bool](request, "maintainer_can_modify")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			pr, resp, err := client.PullRequests.Create(ctx, owner, repo, newPR)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to create pull request",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to create pull request: %s", string(body))), nil
			}

			// Return minimal response with just essential information
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", pr.GetID()),
				URL: pr.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// UpdatePullRequest creates a tool to update an existing pull request.
func UpdatePullRequest(getClient GetClientFn, getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("update_pull_request",
			mcp.WithDescription(t("TOOL_UPDATE_PULL_REQUEST_DESCRIPTION", "Update an existing pull request in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_UPDATE_PULL_REQUEST_USER_TITLE", "Edit pull request"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number to update"),
			),
			mcp.WithString("title",
				mcp.Description("New title"),
			),
			mcp.WithString("body",
				mcp.Description("New description"),
			),
			mcp.WithString("state",
				mcp.Description("New state"),
				mcp.Enum("open", "closed"),
			),
			mcp.WithBoolean("draft",
				mcp.Description("Mark pull request as draft (true) or ready for review (false)"),
			),
			mcp.WithString("base",
				mcp.Description("New base branch name"),
			),
			mcp.WithBoolean("maintainer_can_modify",
				mcp.Description("Allow maintainer edits"),
			),
			mcp.WithArray("reviewers",
				mcp.Description("GitHub usernames to request reviews from"),
				mcp.Items(map[string]interface{}{
					"type": "string",
				}),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Check if draft parameter is provided
			draftProvided := request.GetArguments()["draft"] != nil
			var draftValue bool
			if draftProvided {
				draftValue, err = OptionalParam[bool](request, "draft")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}

			// Build the update struct only with provided fields
			update := &github.PullRequest{}
			restUpdateNeeded := false

			if title, ok, err := OptionalParamOK[string](request, "title"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if ok {
				update.Title = github.Ptr(title)
				restUpdateNeeded = true
			}

			if body, ok, err := OptionalParamOK[string](request, "body"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if ok {
				update.Body = github.Ptr(body)
				restUpdateNeeded = true
			}

			if state, ok, err := OptionalParamOK[string](request, "state"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if ok {
				update.State = github.Ptr(state)
				restUpdateNeeded = true
			}

			if base, ok, err := OptionalParamOK[string](request, "base"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if ok {
				update.Base = &github.PullRequestBranch{Ref: github.Ptr(base)}
				restUpdateNeeded = true
			}

			if maintainerCanModify, ok, err := OptionalParamOK[bool](request, "maintainer_can_modify"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			} else if ok {
				update.MaintainerCanModify = github.Ptr(maintainerCanModify)
				restUpdateNeeded = true
			}

			// Handle reviewers separately
			reviewers, err := OptionalStringArrayParam(request, "reviewers")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// If no updates, no draft change, and no reviewers, return error early
			if !restUpdateNeeded && !draftProvided && len(reviewers) == 0 {
				return mcp.NewToolResultError("No update parameters provided."), nil
			}

			// Handle REST API updates (title, body, state, base, maintainer_can_modify)
			if restUpdateNeeded {
				client, err := getClient(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to get GitHub client: %w", err)
				}

				_, resp, err := client.PullRequests.Edit(ctx, owner, repo, pullNumber, update)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx,
						"failed to update pull request",
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
					return mcp.NewToolResultError(fmt.Sprintf("failed to update pull request: %s", string(body))), nil
				}
			}

			// Handle draft status changes using GraphQL
			if draftProvided {
				gqlClient, err := getGQLClient(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to get GitHub GraphQL client: %w", err)
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
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to find pull request", err), nil
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
							return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to convert pull request to draft", err), nil
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
							return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to mark pull request ready for review", err), nil
						}
					}
				}
			}

			// Handle reviewer requests
			if len(reviewers) > 0 {
				client, err := getClient(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
					), nil
				}
				defer func() {
					if resp != nil && resp.Body != nil {
						_ = resp.Body.Close()
					}
				}()

				if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return nil, fmt.Errorf("failed to read response body: %w", err)
					}
					return mcp.NewToolResultError(fmt.Sprintf("failed to request reviewers: %s", string(body))), nil
				}
			}

			// Get the final state of the PR to return
			client, err := getClient(ctx)
			if err != nil {
				return nil, err
			}

			finalPR, resp, err := client.PullRequests.Get(ctx, owner, repo, pullNumber)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "Failed to get pull request", resp, err), nil
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
				return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal response: %v", err)), nil
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListPullRequests creates a tool to list and filter repository pull requests.
func ListPullRequests(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("list_pull_requests",
			mcp.WithDescription(t("TOOL_LIST_PULL_REQUESTS_DESCRIPTION", "List pull requests in a GitHub repository. If the user specifies an author, then DO NOT use this tool and use the search_pull_requests tool instead.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_PULL_REQUESTS_USER_TITLE", "List pull requests"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("state",
				mcp.Description("Filter by state"),
				mcp.Enum("open", "closed", "all"),
			),
			mcp.WithString("head",
				mcp.Description("Filter by head user/org and branch"),
			),
			mcp.WithString("base",
				mcp.Description("Filter by base branch"),
			),
			mcp.WithString("sort",
				mcp.Description("Sort by"),
				mcp.Enum("created", "updated", "popularity", "long-running"),
			),
			mcp.WithString("direction",
				mcp.Description("Sort direction"),
				mcp.Enum("asc", "desc"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			state, err := OptionalParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			head, err := OptionalParam[string](request, "head")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			base, err := OptionalParam[string](request, "base")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sort, err := OptionalParam[string](request, "sort")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			direction, err := OptionalParam[string](request, "direction")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
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

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			prs, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list pull requests",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to list pull requests: %s", string(body))), nil
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
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// MergePullRequest creates a tool to merge a pull request.
func MergePullRequest(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("merge_pull_request",
			mcp.WithDescription(t("TOOL_MERGE_PULL_REQUEST_DESCRIPTION", "Merge a pull request in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_MERGE_PULL_REQUEST_USER_TITLE", "Merge pull request"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
			),
			mcp.WithString("commit_title",
				mcp.Description("Title for merge commit"),
			),
			mcp.WithString("commit_message",
				mcp.Description("Extra detail for merge commit"),
			),
			mcp.WithString("merge_method",
				mcp.Description("Merge method"),
				mcp.Enum("merge", "squash", "rebase"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			commitTitle, err := OptionalParam[string](request, "commit_title")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			commitMessage, err := OptionalParam[string](request, "commit_message")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			mergeMethod, err := OptionalParam[string](request, "merge_method")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			options := &github.PullRequestOptions{
				CommitTitle: commitTitle,
				MergeMethod: mergeMethod,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			result, resp, err := client.PullRequests.Merge(ctx, owner, repo, pullNumber, commitMessage, options)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to merge pull request",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to merge pull request: %s", string(body))), nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// SearchPullRequests creates a tool to search for pull requests.
func SearchPullRequests(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_pull_requests",
			mcp.WithDescription(t("TOOL_SEARCH_PULL_REQUESTS_DESCRIPTION", "Search for pull requests in GitHub repositories using issues search syntax already scoped to is:pr")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_SEARCH_PULL_REQUESTS_USER_TITLE", "Search pull requests"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query using GitHub pull request search syntax"),
			),
			mcp.WithString("owner",
				mcp.Description("Optional repository owner. If provided with repo, only pull requests for this repository are listed."),
			),
			mcp.WithString("repo",
				mcp.Description("Optional repository name. If provided with owner, only pull requests for this repository are listed."),
			),
			mcp.WithString("sort",
				mcp.Description("Sort field by number of matches of categories, defaults to best match"),
				mcp.Enum(
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
				),
			),
			mcp.WithString("order",
				mcp.Description("Sort order"),
				mcp.Enum("asc", "desc"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return searchHandler(ctx, getClient, request, "pr", "failed to search pull requests")
		}
}

// UpdatePullRequestBranch creates a tool to update a pull request branch with the latest changes from the base branch.
func UpdatePullRequestBranch(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("update_pull_request_branch",
			mcp.WithDescription(t("TOOL_UPDATE_PULL_REQUEST_BRANCH_DESCRIPTION", "Update the branch of a pull request with the latest changes from the base branch.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_UPDATE_PULL_REQUEST_BRANCH_USER_TITLE", "Update pull request branch"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
			),
			mcp.WithString("expectedHeadSha",
				mcp.Description("The expected SHA of the pull request's HEAD ref"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			expectedHeadSHA, err := OptionalParam[string](request, "expectedHeadSha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			opts := &github.PullRequestBranchUpdateOptions{}
			if expectedHeadSHA != "" {
				opts.ExpectedHeadSHA = github.Ptr(expectedHeadSHA)
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			result, resp, err := client.PullRequests.UpdateBranch(ctx, owner, repo, pullNumber, opts)
			if err != nil {
				// Check if it's an acceptedError. An acceptedError indicates that the update is in progress,
				// and it's not a real error.
				if resp != nil && resp.StatusCode == http.StatusAccepted && isAcceptedError(err) {
					return mcp.NewToolResultText("Pull request branch update is in progress"), nil
				}
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to update pull request branch",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusAccepted {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to update pull request branch: %s", string(body))), nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
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

func PullRequestReviewWrite(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("pull_request_review_write",
			mcp.WithDescription(t("TOOL_PULL_REQUEST_REVIEW_WRITE_DESCRIPTION", `Create and/or submit, delete review of a pull request.

Available methods:
- create: Create a new review of a pull request. If "event" parameter is provided, the review is submitted. If "event" is omitted, a pending review is created.
- submit_pending: Submit an existing pending review of a pull request. This requires that a pending review exists for the current user on the specified pull request. The "body" and "event" parameters are used when submitting the review.
- delete_pending: Delete an existing pending review of a pull request. This requires that a pending review exists for the current user on the specified pull request.
`)),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_PULL_REQUEST_REVIEW_WRITE_USER_TITLE", "Write operations (create, submit, delete) on pull request reviews."),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			// Either we need the PR GQL Id directly, or we need owner, repo and PR number to look it up.
			// Since our other Pull Request tools are working with the REST Client, will handle the lookup
			// internally for now.
			mcp.WithString("method",
				mcp.Required(),
				mcp.Description("The write operation to perform on pull request review."),
				mcp.Enum("create", "submit_pending", "delete_pending"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
			),
			mcp.WithString("body",
				mcp.Description("Review comment text"),
			),
			mcp.WithString("event",
				mcp.Description("Review action to perform."),
				mcp.Enum("APPROVE", "REQUEST_CHANGES", "COMMENT"),
			),
			mcp.WithString("commitID",
				mcp.Description("SHA of commit to review"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var params PullRequestReviewWriteParams
			if err := mapstructure.Decode(request.Params.Arguments, &params); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Given our owner, repo and PR number, lookup the GQL ID of the PR.
			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil
			}

			switch params.Method {
			case "create":
				return CreatePullRequestReview(ctx, client, params)
			case "submit_pending":
				return SubmitPendingPullRequestReview(ctx, client, params)
			case "delete_pending":
				return DeletePendingPullRequestReview(ctx, client, params)
			default:
				return mcp.NewToolResultError(fmt.Sprintf("unknown method: %s", params.Method)), nil
			}
		}
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
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Return nothing interesting, just indicate success for the time being.
	// In future, we may want to return the review ID, but for the moment, we're not leaking
	// API implementation details to the LLM.
	if params.Event == "" {
		return mcp.NewToolResultText("pending pull request created"), nil
	}
	return mcp.NewToolResultText("pull request review submitted successfully"), nil
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
		return mcp.NewToolResultError("No pending review found for the viewer"), nil
	}

	review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
	if review.State != githubv4.PullRequestReviewStatePending {
		errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
		return mcp.NewToolResultError(errText), nil
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
	return mcp.NewToolResultText("pending pull request review successfully submitted"), nil
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
		return mcp.NewToolResultError("No pending review found for the viewer"), nil
	}

	review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
	if review.State != githubv4.PullRequestReviewStatePending {
		errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
		return mcp.NewToolResultError(errText), nil
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
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Return nothing interesting, just indicate success for the time being.
	// In future, we may want to return the review ID, but for the moment, we're not leaking
	// API implementation details to the LLM.
	return mcp.NewToolResultText("pending pull request review successfully deleted"), nil
}

// AddCommentToPendingReview creates a tool to add a comment to a pull request review.
func AddCommentToPendingReview(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("add_comment_to_pending_review",
			mcp.WithDescription(t("TOOL_ADD_COMMENT_TO_PENDING_REVIEW_DESCRIPTION", "Add review comment to the requester's latest pending pull request review. A pending review needs to already exist to call this (check with the user if not sure).")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_ADD_COMMENT_TO_PENDING_REVIEW_USER_TITLE", "Add review comment to the requester's latest pending pull request review"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			// Ideally, for performance sake this would just accept the pullRequestReviewID. However, we would need to
			// add a new tool to get that ID for clients that aren't in the same context as the original pending review
			// creation. So for now, we'll just accept the owner, repo and pull number and assume this is adding a comment
			// the latest review from a user, since only one can be active at a time. It can later be extended with
			// a pullRequestReviewID parameter if targeting other reviews is desired:
			// mcp.WithString("pullRequestReviewID",
			// 	mcp.Required(),
			// 	mcp.Description("The ID of the pull request review to add a comment to"),
			// ),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
			),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("The relative path to the file that necessitates a comment"),
			),
			mcp.WithString("body",
				mcp.Required(),
				mcp.Description("The text of the review comment"),
			),
			mcp.WithString("subjectType",
				mcp.Required(),
				mcp.Description("The level at which the comment is targeted"),
				mcp.Enum("FILE", "LINE"),
			),
			mcp.WithNumber("line",
				mcp.Description("The line of the blob in the pull request diff that the comment applies to. For multi-line comments, the last line of the range"),
			),
			mcp.WithString("side",
				mcp.Description("The side of the diff to comment on. LEFT indicates the previous state, RIGHT indicates the new state"),
				mcp.Enum("LEFT", "RIGHT"),
			),
			mcp.WithNumber("startLine",
				mcp.Description("For multi-line comments, the first line of the range that the comment applies to"),
			),
			mcp.WithString("startSide",
				mcp.Description("For multi-line comments, the starting side of the diff that the comment applies to. LEFT indicates the previous state, RIGHT indicates the new state"),
				mcp.Enum("LEFT", "RIGHT"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			if err := mapstructure.Decode(request.Params.Arguments, &params); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub GQL client: %w", err)
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
				return mcp.NewToolResultError("No pending review found for the viewer"), nil
			}

			review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
			if review.State != githubv4.PullRequestReviewStatePending {
				errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
				return mcp.NewToolResultError(errText), nil
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
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Return nothing interesting, just indicate success for the time being.
			// In future, we may want to return the review ID, but for the moment, we're not leaking
			// API implementation details to the LLM.
			return mcp.NewToolResultText("pull request review comment successfully added to pending review"), nil
		}
}

// RequestCopilotReview creates a tool to request a Copilot review for a pull request.
// Note that this tool will not work on GHES where this feature is unsupported. In future, we should not expose this
// tool if the configured host does not support it.
func RequestCopilotReview(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("request_copilot_review",
			mcp.WithDescription(t("TOOL_REQUEST_COPILOT_REVIEW_DESCRIPTION", "Request a GitHub Copilot code review for a pull request. Use this for automated feedback on pull requests, usually before requesting a human reviewer.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_REQUEST_COPILOT_REVIEW_USER_TITLE", "Request Copilot review"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
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
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to request copilot review: %s", string(body))), nil
			}

			// Return nothing on success, as there's not much value in returning the Pull Request itself
			return mcp.NewToolResultText(""), nil
		}
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
