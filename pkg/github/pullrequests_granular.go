package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// prUpdateTool is a helper to create single-field pull request update tools via REST.
func prUpdateTool(
	t translations.TranslationHelperFunc,
	name, description, title string,
	extraProps map[string]*jsonschema.Schema,
	extraRequired []string,
	buildRequest func(args map[string]any) (*gogithub.PullRequest, error),
) inventory.ServerTool {
	props := map[string]*jsonschema.Schema{
		"owner": {
			Type:        "string",
			Description: "Repository owner (username or organization)",
		},
		"repo": {
			Type:        "string",
			Description: "Repository name",
		},
		"pullNumber": {
			Type:        "number",
			Description: "The pull request number",
			Minimum:     jsonschema.Ptr(1.0),
		},
	}
	maps.Copy(props, extraProps)

	required := append([]string{"owner", "repo", "pullNumber"}, extraRequired...)

	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        name,
			Description: t("TOOL_"+name+"_DESCRIPTION", description),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_"+name+"_USER_TITLE", title),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: props,
				Required:   required,
			},
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

			prReq, err := buildRequest(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			pr, resp, err := client.PullRequests.Edit(ctx, owner, repo, pullNumber, prReq)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update pull request", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(MinimalResponse{
				ID:  fmt.Sprintf("%d", pr.GetID()),
				URL: pr.GetHTMLURL(),
			})
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	st.FeatureFlagEnable = FeatureFlagPullRequestsGranular
	return st
}

// GranularUpdatePullRequestTitle creates a tool to update a PR's title.
func GranularUpdatePullRequestTitle(t translations.TranslationHelperFunc) inventory.ServerTool {
	return prUpdateTool(t,
		"update_pull_request_title",
		"Update the title of an existing pull request.",
		"Update Pull Request Title",
		map[string]*jsonschema.Schema{
			"title": {Type: "string", Description: "The new title for the pull request"},
		},
		[]string{"title"},
		func(args map[string]any) (*gogithub.PullRequest, error) {
			title, err := RequiredParam[string](args, "title")
			if err != nil {
				return nil, err
			}
			return &gogithub.PullRequest{Title: &title}, nil
		},
	)
}

// GranularUpdatePullRequestBody creates a tool to update a PR's body.
func GranularUpdatePullRequestBody(t translations.TranslationHelperFunc) inventory.ServerTool {
	return prUpdateTool(t,
		"update_pull_request_body",
		"Update the body description of an existing pull request.",
		"Update Pull Request Body",
		map[string]*jsonschema.Schema{
			"body": {Type: "string", Description: "The new body content for the pull request"},
		},
		[]string{"body"},
		func(args map[string]any) (*gogithub.PullRequest, error) {
			body, err := RequiredParam[string](args, "body")
			if err != nil {
				return nil, err
			}
			return &gogithub.PullRequest{Body: &body}, nil
		},
	)
}

// GranularUpdatePullRequestState creates a tool to update a PR's state.
func GranularUpdatePullRequestState(t translations.TranslationHelperFunc) inventory.ServerTool {
	return prUpdateTool(t,
		"update_pull_request_state",
		"Update the state of an existing pull request (open or closed).",
		"Update Pull Request State",
		map[string]*jsonschema.Schema{
			"state": {
				Type:        "string",
				Description: "The new state for the pull request",
				Enum:        []any{"open", "closed"},
			},
		},
		[]string{"state"},
		func(args map[string]any) (*gogithub.PullRequest, error) {
			state, err := RequiredParam[string](args, "state")
			if err != nil {
				return nil, err
			}
			return &gogithub.PullRequest{State: &state}, nil
		},
	)
}

// GranularUpdatePullRequestDraftState creates a tool to toggle draft state.
func GranularUpdatePullRequestDraftState(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "update_pull_request_draft_state",
			Description: t("TOOL_UPDATE_PULL_REQUEST_DRAFT_STATE_DESCRIPTION", "Mark a pull request as draft or ready for review."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_PULL_REQUEST_DRAFT_STATE_USER_TITLE", "Update Pull Request Draft State"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":      {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":       {Type: "string", Description: "Repository name"},
					"pullNumber": {Type: "number", Description: "The pull request number", Minimum: jsonschema.Ptr(1.0)},
					"draft":      {Type: "boolean", Description: "Set to true to convert to draft, false to mark as ready for review"},
				},
				Required: []string{"owner", "repo", "pullNumber", "draft"},
			},
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
			// Use presence check + OptionalParam since RequiredParam rejects false (zero-value for bool)
			if _, ok := args["draft"]; !ok {
				return utils.NewToolResultError("missing required parameter: draft"), nil, nil
			}
			draft, err := OptionalParam[bool](args, "draft")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			// Get PR node ID
			var prQuery struct {
				Repository struct {
					PullRequest struct {
						ID githubv4.ID
					} `graphql:"pullRequest(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $name)"`
			}
			if err := gqlClient.Query(ctx, &prQuery, map[string]any{
				"owner":  githubv4.String(owner),
				"name":   githubv4.String(repo),
				"number": githubv4.Int(pullNumber), // #nosec G115 - PR numbers are always small positive integers
			}); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get pull request", err), nil, nil
			}

			if draft {
				var mutation struct {
					ConvertPullRequestToDraft struct {
						PullRequest struct {
							ID      githubv4.ID
							IsDraft githubv4.Boolean
						}
					} `graphql:"convertPullRequestToDraft(input: $input)"`
				}
				if err := gqlClient.Mutate(ctx, &mutation, githubv4.ConvertPullRequestToDraftInput{
					PullRequestID: prQuery.Repository.PullRequest.ID,
				}, nil); err != nil {
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to convert to draft", err), nil, nil
				}
				return utils.NewToolResultText("pull request converted to draft"), nil, nil
			}

			var mutation struct {
				MarkPullRequestReadyForReview struct {
					PullRequest struct {
						ID      githubv4.ID
						IsDraft githubv4.Boolean
					}
				} `graphql:"markPullRequestReadyForReview(input: $input)"`
			}
			if err := gqlClient.Mutate(ctx, &mutation, githubv4.MarkPullRequestReadyForReviewInput{
				PullRequestID: prQuery.Repository.PullRequest.ID,
			}, nil); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to mark ready for review", err), nil, nil
			}
			return utils.NewToolResultText("pull request marked as ready for review"), nil, nil
		},
	)
	st.FeatureFlagEnable = FeatureFlagPullRequestsGranular
	return st
}

// GranularRequestPullRequestReviewers creates a tool to request reviewers.
func GranularRequestPullRequestReviewers(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "request_pull_request_reviewers",
			Description: t("TOOL_REQUEST_PULL_REQUEST_REVIEWERS_DESCRIPTION", "Request reviewers for a pull request."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REQUEST_PULL_REQUEST_REVIEWERS_USER_TITLE", "Request Pull Request Reviewers"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":      {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":       {Type: "string", Description: "Repository name"},
					"pullNumber": {Type: "number", Description: "The pull request number", Minimum: jsonschema.Ptr(1.0)},
					"reviewers": {
						Type:        "array",
						Description: "GitHub usernames to request reviews from",
						Items:       &jsonschema.Schema{Type: "string"},
					},
				},
				Required: []string{"owner", "repo", "pullNumber", "reviewers"},
			},
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
			reviewers, err := OptionalStringArrayParam(args, "reviewers")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if len(reviewers) == 0 {
				return utils.NewToolResultError("missing required parameter: reviewers"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			pr, resp, err := client.PullRequests.RequestReviewers(ctx, owner, repo, pullNumber, gogithub.ReviewersRequest{Reviewers: reviewers})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to request reviewers", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(MinimalResponse{
				ID:  fmt.Sprintf("%d", pr.GetID()),
				URL: pr.GetHTMLURL(),
			})
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
	st.FeatureFlagEnable = FeatureFlagPullRequestsGranular
	return st
}

// GranularCreatePullRequestReview creates a tool to create a PR review.
func GranularCreatePullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "create_pull_request_review",
			Description: t("TOOL_CREATE_PULL_REQUEST_REVIEW_DESCRIPTION", "Create a review on a pull request. If event is provided, the review is submitted immediately; otherwise a pending review is created."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_CREATE_PULL_REQUEST_REVIEW_USER_TITLE", "Create Pull Request Review"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":      {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":       {Type: "string", Description: "Repository name"},
					"pullNumber": {Type: "number", Description: "The pull request number", Minimum: jsonschema.Ptr(1.0)},
					"body":       {Type: "string", Description: "The review body text (optional)"},
					"event":      {Type: "string", Description: "The review action to perform. If omitted, creates a pending review.", Enum: []any{"APPROVE", "REQUEST_CHANGES", "COMMENT"}},
					"commitID":   {Type: "string", Description: "The SHA of the commit to review (optional, defaults to latest)"},
				},
				Required: []string{"owner", "repo", "pullNumber"},
			},
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
			body, _ := OptionalParam[string](args, "body")
			event, _ := OptionalParam[string](args, "event")
			commitID, _ := OptionalParam[string](args, "commitID")

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			var commitIDPtr *string
			if commitID != "" {
				commitIDPtr = &commitID
			}

			result, err := CreatePullRequestReview(ctx, gqlClient, PullRequestReviewWriteParams{
				Owner:      owner,
				Repo:       repo,
				PullNumber: int32(pullNumber), // #nosec G115 - PR numbers are always small positive integers
				Body:       body,
				Event:      event,
				CommitID:   commitIDPtr,
			})
			return result, nil, err
		},
	)
	st.FeatureFlagEnable = FeatureFlagPullRequestsGranular
	return st
}

// GranularSubmitPendingPullRequestReview creates a tool to submit a pending review.
func GranularSubmitPendingPullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "submit_pending_pull_request_review",
			Description: t("TOOL_SUBMIT_PENDING_PULL_REQUEST_REVIEW_DESCRIPTION", "Submit a pending pull request review."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_SUBMIT_PENDING_PULL_REQUEST_REVIEW_USER_TITLE", "Submit Pending Pull Request Review"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":      {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":       {Type: "string", Description: "Repository name"},
					"pullNumber": {Type: "number", Description: "The pull request number", Minimum: jsonschema.Ptr(1.0)},
					"event":      {Type: "string", Description: "The review action to perform", Enum: []any{"APPROVE", "REQUEST_CHANGES", "COMMENT"}},
					"body":       {Type: "string", Description: "The review body text (optional)"},
				},
				Required: []string{"owner", "repo", "pullNumber", "event"},
			},
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
			event, err := RequiredParam[string](args, "event")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			body, _ := OptionalParam[string](args, "body")

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			result, err := SubmitPendingPullRequestReview(ctx, gqlClient, PullRequestReviewWriteParams{
				Owner:      owner,
				Repo:       repo,
				PullNumber: int32(pullNumber), // #nosec G115 - PR numbers are always small positive integers
				Event:      event,
				Body:       body,
			})
			return result, nil, err
		},
	)
	st.FeatureFlagEnable = FeatureFlagPullRequestsGranular
	return st
}

// GranularDeletePendingPullRequestReview creates a tool to delete a pending review.
func GranularDeletePendingPullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "delete_pending_pull_request_review",
			Description: t("TOOL_DELETE_PENDING_PULL_REQUEST_REVIEW_DESCRIPTION", "Delete a pending pull request review."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_DELETE_PENDING_PULL_REQUEST_REVIEW_USER_TITLE", "Delete Pending Pull Request Review"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":      {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":       {Type: "string", Description: "Repository name"},
					"pullNumber": {Type: "number", Description: "The pull request number", Minimum: jsonschema.Ptr(1.0)},
				},
				Required: []string{"owner", "repo", "pullNumber"},
			},
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

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			result, err := DeletePendingPullRequestReview(ctx, gqlClient, PullRequestReviewWriteParams{
				Owner:      owner,
				Repo:       repo,
				PullNumber: int32(pullNumber), // #nosec G115 - PR numbers are always small positive integers
			})
			return result, nil, err
		},
	)
	st.FeatureFlagEnable = FeatureFlagPullRequestsGranular
	return st
}

// GranularAddPullRequestReviewComment creates a tool to add a review comment.
func GranularAddPullRequestReviewComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "add_pull_request_review_comment",
			Description: t("TOOL_ADD_PULL_REQUEST_REVIEW_COMMENT_DESCRIPTION", "Add a review comment to the current user's pending pull request review."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ADD_PULL_REQUEST_REVIEW_COMMENT_USER_TITLE", "Add Pull Request Review Comment"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":       {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":        {Type: "string", Description: "Repository name"},
					"pullNumber":  {Type: "number", Description: "The pull request number", Minimum: jsonschema.Ptr(1.0)},
					"path":        {Type: "string", Description: "The relative path of the file to comment on"},
					"body":        {Type: "string", Description: "The comment body"},
					"subjectType": {Type: "string", Description: "The subject type of the comment", Enum: []any{"FILE", "LINE"}},
					"line":        {Type: "number", Description: "The line number in the diff to comment on (optional)"},
					"side":        {Type: "string", Description: "The side of the diff to comment on (optional)", Enum: []any{"LEFT", "RIGHT"}},
					"startLine":   {Type: "number", Description: "The start line of a multi-line comment (optional)"},
					"startSide":   {Type: "string", Description: "The start side of a multi-line comment (optional)", Enum: []any{"LEFT", "RIGHT"}},
				},
				Required: []string{"owner", "repo", "pullNumber", "path", "body", "subjectType"},
			},
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

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			// Get the current user to find their pending review
			var getViewerQuery struct {
				Viewer struct {
					Login githubv4.String
				}
			}
			if err := gqlClient.Query(ctx, &getViewerQuery, nil); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get current user", err), nil, nil
			}

			// Find the viewer's latest review (must be pending)
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
				"owner":  githubv4.String(owner),
				"name":   githubv4.String(repo),
				"prNum":  githubv4.Int(pullNumber), // #nosec G115 - PR numbers are always small positive integers
			}

			if err := gqlClient.Query(ctx, &getLatestReviewForViewerQuery, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get latest review for current user", err), nil, nil
			}

			if len(getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes) == 0 {
				return utils.NewToolResultError("No pending review found for the viewer"), nil, nil
			}

			review := getLatestReviewForViewerQuery.Repository.PullRequest.Reviews.Nodes[0]
			if review.State != githubv4.PullRequestReviewStatePending {
				errText := fmt.Sprintf("The latest review, found at %s is not pending", review.URL)
				return utils.NewToolResultError(errText), nil, nil
			}

			// Add review thread comment to the pending review
			var addPullRequestReviewThreadMutation struct {
				AddPullRequestReviewThread struct {
					Thread struct {
						ID githubv4.ID
					}
				} `graphql:"addPullRequestReviewThread(input: $input)"`
			}

			// Convert optional int params to *int32 for GraphQL helper
			var linePtr, startLinePtr *int32
			if line != 0 {
				l := int32(line) // #nosec G115
				linePtr = &l
			}
			if startLine != 0 {
				sl := int32(startLine) // #nosec G115
				startLinePtr = &sl
			}

			// Convert optional string params: pass nil (not empty string) when absent
			var sidePtr, startSidePtr *string
			if side != "" {
				sidePtr = &side
			}
			if startSide != "" {
				startSidePtr = &startSide
			}

			if err := gqlClient.Mutate(
				ctx,
				&addPullRequestReviewThreadMutation,
				githubv4.AddPullRequestReviewThreadInput{
					Path:                githubv4.String(path),
					Body:                githubv4.String(body),
					SubjectType:         newGQLStringlikePtr[githubv4.PullRequestReviewThreadSubjectType](&subjectType),
					Line:                newGQLIntPtr(linePtr),
					Side:                newGQLStringlikePtr[githubv4.DiffSide](sidePtr),
					StartLine:           newGQLIntPtr(startLinePtr),
					StartSide:           newGQLStringlikePtr[githubv4.DiffSide](startSidePtr),
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

			return utils.NewToolResultText("pull request review comment successfully added to pending review"), nil, nil
		},
	)
	st.FeatureFlagEnable = FeatureFlagPullRequestsGranular
	return st
}
