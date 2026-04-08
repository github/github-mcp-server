package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

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

	return NewTool(
		ToolsetMetadataPullRequestsGranular,
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

			pr, _, err := client.PullRequests.Edit(ctx, owner, repo, pullNumber, prReq)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to update pull request", err), nil, nil
			}

			r, err := json.Marshal(pr)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
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
	return NewTool(
		ToolsetMetadataPullRequestsGranular,
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
			draft, err := RequiredParam[bool](args, "draft")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			prNodeID, err := getGranularPullRequestNodeID(ctx, gqlClient, owner, repo, pullNumber)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get pull request", err), nil, nil
			}

			if draft {
				var mutation struct {
					ConvertPullRequestToDraft struct {
						PullRequest struct {
							ID, Title, URL string
							IsDraft        bool
						}
					} `graphql:"convertPullRequestToDraft(input: $input)"`
				}
				input := map[string]any{"pullRequestId": githubv4.ID(prNodeID)}
				if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
					return utils.NewToolResultErrorFromErr("failed to convert to draft", err), nil, nil
				}
				r, err := json.Marshal(mutation.ConvertPullRequestToDraft.PullRequest)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
				}
				return utils.NewToolResultText(string(r)), nil, nil
			}

			var mutation struct {
				MarkPullRequestReadyForReview struct {
					PullRequest struct {
						ID, Title, URL string
						IsDraft        bool
					}
				} `graphql:"markPullRequestReadyForReview(input: $input)"`
			}
			input := map[string]any{"pullRequestId": githubv4.ID(prNodeID)}
			if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
				return utils.NewToolResultErrorFromErr("failed to mark ready for review", err), nil, nil
			}
			r, err := json.Marshal(mutation.MarkPullRequestReadyForReview.PullRequest)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GranularRequestPullRequestReviewers creates a tool to request reviewers.
func GranularRequestPullRequestReviewers(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataPullRequestsGranular,
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
			rawReviewers, _ := OptionalParam[[]any](args, "reviewers")
			if len(rawReviewers) == 0 {
				return utils.NewToolResultError("missing required parameter: reviewers"), nil, nil
			}
			reviewers := make([]string, 0, len(rawReviewers))
			for _, v := range rawReviewers {
				if s, ok := v.(string); ok {
					reviewers = append(reviewers, s)
				}
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			pr, _, err := client.PullRequests.RequestReviewers(ctx, owner, repo, pullNumber, gogithub.ReviewersRequest{Reviewers: reviewers})
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to request reviewers", err), nil, nil
			}

			r, err := json.Marshal(pr)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GranularCreatePullRequestReview creates a tool to create a PR review.
func GranularCreatePullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataPullRequestsGranular,
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

			reviewReq := &gogithub.PullRequestReviewRequest{}
			if body != "" {
				reviewReq.Body = &body
			}
			if event != "" {
				reviewReq.Event = &event
			}
			if commitID != "" {
				reviewReq.CommitID = &commitID
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			review, _, err := client.PullRequests.CreateReview(ctx, owner, repo, pullNumber, reviewReq)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create review", err), nil, nil
			}

			r, err := json.Marshal(review)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GranularSubmitPendingPullRequestReview creates a tool to submit a pending review.
func GranularSubmitPendingPullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataPullRequestsGranular,
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

			prNodeID, err := getGranularPullRequestNodeID(ctx, gqlClient, owner, repo, pullNumber)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get pull request", err), nil, nil
			}

			// Find pending review
			var reviewQuery struct {
				Repository struct {
					PullRequest struct {
						Reviews struct {
							Nodes []struct {
								ID, State string
							}
						} `graphql:"reviews(first: 1, states: PENDING)"`
					} `graphql:"pullRequest(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $name)"`
			}

			vars := map[string]any{
				"owner":  githubv4.String(owner),
				"name":   githubv4.String(repo),
				"number": githubv4.Int(pullNumber), // #nosec G115 - PR numbers are always small positive integers
			}
			if err := gqlClient.Query(ctx, &reviewQuery, vars); err != nil {
				return utils.NewToolResultErrorFromErr("failed to find pending review", err), nil, nil
			}

			if len(reviewQuery.Repository.PullRequest.Reviews.Nodes) == 0 {
				return utils.NewToolResultError("no pending review found for the current user"), nil, nil
			}

			reviewID := reviewQuery.Repository.PullRequest.Reviews.Nodes[0].ID

			var mutation struct {
				SubmitPullRequestReview struct {
					PullRequestReview struct {
						ID, State, URL string
					}
				} `graphql:"submitPullRequestReview(input: $input)"`
			}

			submitInput := map[string]any{
				"pullRequestId":       githubv4.ID(prNodeID),
				"pullRequestReviewId": githubv4.ID(reviewID),
				"event":               githubv4.PullRequestReviewEvent(event),
			}
			if body != "" {
				submitInput["body"] = githubv4.String(body)
			}

			if err := gqlClient.Mutate(ctx, &mutation, submitInput, nil); err != nil {
				return utils.NewToolResultErrorFromErr("failed to submit review", err), nil, nil
			}

			r, err := json.Marshal(mutation.SubmitPullRequestReview.PullRequestReview)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// GranularDeletePendingPullRequestReview creates a tool to delete a pending review.
func GranularDeletePendingPullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataPullRequestsGranular,
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

			// Find pending review
			var reviewQuery struct {
				Repository struct {
					PullRequest struct {
						Reviews struct {
							Nodes []struct{ ID string }
						} `graphql:"reviews(first: 1, states: PENDING)"`
					} `graphql:"pullRequest(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $name)"`
			}

			vars := map[string]any{
				"owner":  githubv4.String(owner),
				"name":   githubv4.String(repo),
				"number": githubv4.Int(pullNumber), // #nosec G115 - PR numbers are always small positive integers
			}
			if err := gqlClient.Query(ctx, &reviewQuery, vars); err != nil {
				return utils.NewToolResultErrorFromErr("failed to find pending review", err), nil, nil
			}

			if len(reviewQuery.Repository.PullRequest.Reviews.Nodes) == 0 {
				return utils.NewToolResultError("no pending review found for the current user"), nil, nil
			}

			reviewID := reviewQuery.Repository.PullRequest.Reviews.Nodes[0].ID

			var mutation struct {
				DeletePullRequestReview struct {
					PullRequestReview struct{ ID string }
				} `graphql:"deletePullRequestReview(input: $input)"`
			}

			input := map[string]any{"pullRequestReviewId": githubv4.ID(reviewID)}

			if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
				return utils.NewToolResultErrorFromErr("failed to delete pending review", err), nil, nil
			}

			return utils.NewToolResultText(`{"message": "Pending review deleted successfully"}`), nil, nil
		},
	)
}

// GranularAddPullRequestReviewComment creates a tool to add a review comment.
func GranularAddPullRequestReviewComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataPullRequestsGranular,
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
			line, _ := OptionalParam[float64](args, "line")
			side, _ := OptionalParam[string](args, "side")
			startLine, _ := OptionalParam[float64](args, "startLine")
			startSide, _ := OptionalParam[string](args, "startSide")

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			prNodeID, err := getGranularPullRequestNodeID(ctx, gqlClient, owner, repo, pullNumber)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get pull request", err), nil, nil
			}

			var mutation struct {
				AddPullRequestReviewThread struct {
					Thread struct {
						ID       string
						Comments struct {
							Nodes []struct {
								ID, Body, URL string
							}
						} `graphql:"comments(first: 1)"`
					}
				} `graphql:"addPullRequestReviewThread(input: $input)"`
			}

			input := map[string]any{
				"pullRequestId": githubv4.ID(prNodeID),
				"path":          githubv4.String(path),
				"body":          githubv4.String(body),
				"subjectType":   githubv4.PullRequestReviewThreadSubjectType(subjectType),
			}
			if line != 0 {
				input["line"] = githubv4.Int(int(line)) // #nosec G115
			}
			if side != "" {
				input["side"] = githubv4.DiffSide(side)
			}
			if startLine != 0 {
				input["startLine"] = githubv4.Int(int(startLine)) // #nosec G115
			}
			if startSide != "" {
				input["startSide"] = githubv4.DiffSide(startSide)
			}

			if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
				return utils.NewToolResultErrorFromErr("failed to add review comment", err), nil, nil
			}

			r, err := json.Marshal(mutation.AddPullRequestReviewThread)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// getGranularPullRequestNodeID fetches the GraphQL node ID for a pull request.
func getGranularPullRequestNodeID(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, pullNumber int) (string, error) {
	var query struct {
		Repository struct {
			PullRequest struct {
				ID string
			} `graphql:"pullRequest(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	vars := map[string]any{
		"owner":  githubv4.String(owner),
		"name":   githubv4.String(repo),
		"number": githubv4.Int(pullNumber), // #nosec G115 - PR numbers are always small positive integers
	}

	if err := gqlClient.Query(ctx, &query, vars); err != nil {
		return "", fmt.Errorf("failed to query pull request node ID: %w", err)
	}

	return query.Repository.PullRequest.ID, nil
}
