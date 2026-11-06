package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	gogithub "github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

func newGranularPullRequestTool[In, Out any](
	toolset inventory.ToolsetMetadata,
	tool mcp.Tool,
	scopeAccess inventory.ScopeAccess,
	handler func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, Out, error),
	inputNormalizers ...inventory.InputNormalizer,
) inventory.ServerTool {
	inputSchema, ok := tool.InputSchema.(*jsonschema.Schema)
	if !ok {
		panic("granular pull request tool input schema must be a JSON Schema")
	}
	return NewToolWithSchemaOptions(
		toolset,
		tool,
		scopeAccess,
		inventory.TypedSchemaOptions{
			ValidationInputSchema: granularPullRequestValidationSchema(inputSchema),
		},
		handler,
		inputNormalizers...,
	)
}

// prUpdateTool is a helper to create single-field pull request update tools via REST.
func prUpdateTool[In interface {
	pullRequestCoordinate() GranularPullRequestCoordinate
}](
	t translations.TranslationHelperFunc,
	name, description, title string,
	extraProps map[string]*jsonschema.Schema,
	extraRequired []string,
	buildRequest func(args In) *gogithub.PullRequest,
	normalizer inventory.InputNormalizer,
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
			Minimum:     new(1.0),
		},
	}
	maps.Copy(props, extraProps)

	required := append([]string{"owner", "repo", "pullNumber"}, extraRequired...)

	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        name,
			Description: t("TOOL_"+strings.ToUpper(name)+"_DESCRIPTION", description),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_"+strings.ToUpper(name)+"_USER_TITLE", title),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: props,
				Required:   required,
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args In) (*mcp.CallToolResult, *MinimalResponse, error) {
			coordinate := args.pullRequestCoordinate()
			owner, repo, pullNumber := coordinate.Owner, coordinate.Repo, coordinate.PullNumber
			prReq := buildRequest(args)

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			pr, resp, err := client.PullRequests.Edit(ctx, owner, repo, pullNumber, prReq)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update pull request", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			output := &MinimalResponse{
				ID:  fmt.Sprintf("%d", pr.GetID()),
				URL: pr.GetHTMLURL(),
			}
			r, err := json.Marshal(output)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), output, nil
		},
		normalizer,
	)
	st.FeatureRule = pullRequestsGranularFeatureRule
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
		func(args GranularPullRequestTitleInput) *gogithub.PullRequest {
			return &gogithub.PullRequest{Title: &args.Title}
		},
		normalizeGranularPullRequestArguments("title"),
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
		func(args GranularPullRequestBodyInput) *gogithub.PullRequest {
			return &gogithub.PullRequest{Body: &args.Body}
		},
		normalizeGranularPullRequestArguments("body"),
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
				Enum:        GranularPullRequestState("").JSONSchema().Enum,
			},
		},
		[]string{"state"},
		func(args GranularPullRequestStateInput) *gogithub.PullRequest {
			state := string(args.State)
			return &gogithub.PullRequest{State: &state}
		},
		normalizeGranularPullRequestArguments("state"),
	)
}

// GranularUpdatePullRequestDraftState creates a tool to toggle draft state.
func GranularUpdatePullRequestDraftState(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "update_pull_request_draft_state",
			Description: t("TOOL_UPDATE_PULL_REQUEST_DRAFT_STATE_DESCRIPTION", "Mark a pull request as draft or ready for review."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_PULL_REQUEST_DRAFT_STATE_USER_TITLE", "Update Pull Request Draft State"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":      {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":       {Type: "string", Description: "Repository name"},
					"pullNumber": {Type: "number", Description: "The pull request number", Minimum: new(1.0)},
					"draft":      {Type: "boolean", Description: "Set to true to convert to draft, false to mark as ready for review"},
				},
				Required: []string{"owner", "repo", "pullNumber", "draft"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularPullRequestDraftInput) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
			owner, repo, pullNumber, draft := args.Owner, args.Repo, args.PullNumber, args.Draft

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
				return pullRequestMessageResult(utils.NewToolResultText("pull request converted to draft"), nil)
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
			return pullRequestMessageResult(utils.NewToolResultText("pull request marked as ready for review"), nil)
		},
		normalizeGranularPullRequestArguments("draft"),
	)
	st.FeatureRule = pullRequestsGranularFeatureRule
	return st
}

// GranularRequestPullRequestReviewers creates a tool to request reviewers.
func GranularRequestPullRequestReviewers(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "request_pull_request_reviewers",
			Description: t("TOOL_REQUEST_PULL_REQUEST_REVIEWERS_DESCRIPTION", "Request reviewers for a pull request."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REQUEST_PULL_REQUEST_REVIEWERS_USER_TITLE", "Request Pull Request Reviewers"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":      {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":       {Type: "string", Description: "Repository name"},
					"pullNumber": {Type: "number", Description: "The pull request number", Minimum: new(1.0)},
					"reviewers": {
						Type:        "array",
						Description: "GitHub usernames or ORG/team-slug team reviewers to request reviews from",
						Items:       &jsonschema.Schema{Type: "string"},
					},
				},
				Required: []string{"owner", "repo", "pullNumber", "reviewers"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularPullRequestReviewersInput) (*mcp.CallToolResult, *MinimalResponse, error) {
			owner, repo, pullNumber := args.Owner, args.Repo, args.PullNumber
			userReviewers, teamReviewers := splitPullRequestReviewers(args.Reviewers)

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			pr, resp, err := client.PullRequests.RequestReviewers(ctx, owner, repo, pullNumber, gogithub.ReviewersRequest{
				Reviewers:     userReviewers,
				TeamReviewers: teamReviewers,
			})
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to request reviewers", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			output := &MinimalResponse{
				ID:  fmt.Sprintf("%d", pr.GetID()),
				URL: pr.GetHTMLURL(),
			}
			r, err := json.Marshal(output)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), output, nil
		},
		normalizeGranularPullRequestArguments("reviewers"),
	)
	st.FeatureRule = pullRequestsGranularFeatureRule
	return st
}

func splitPullRequestReviewers(reviewers []string) ([]string, []string) {
	userReviewers := make([]string, 0, len(reviewers))
	teamReviewers := make([]string, 0)

	for _, reviewer := range reviewers {
		org, team, ok := strings.Cut(reviewer, "/")
		if ok && org != "" && team != "" && !strings.Contains(team, "/") {
			teamReviewers = append(teamReviewers, team)
			continue
		}
		userReviewers = append(userReviewers, reviewer)
	}

	return userReviewers, teamReviewers
}

// GranularCreatePullRequestReview creates a tool to create a PR review.
func GranularCreatePullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "create_pull_request_review",
			Description: t("TOOL_CREATE_PULL_REQUEST_REVIEW_DESCRIPTION", "Create a review on a pull request. If event is provided, the review is submitted immediately; otherwise a pending review is created."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_CREATE_PULL_REQUEST_REVIEW_USER_TITLE", "Create Pull Request Review"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":      {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":       {Type: "string", Description: "Repository name"},
					"pullNumber": {Type: "number", Description: "The pull request number", Minimum: new(1.0)},
					"body":       {Type: "string", Description: "The review body text (optional)"},
					"event":      {Type: "string", Description: "The review action to perform. If omitted, creates a pending review.", Enum: GranularPullRequestReviewEvent("").JSONSchema().Enum},
					"commitID":   {Type: "string", Description: "The SHA of the commit to review (optional, defaults to latest)"},
				},
				Required: []string{"owner", "repo", "pullNumber"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularCreatePullRequestReviewInput) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
			owner, repo, pullNumber := args.Owner, args.Repo, args.PullNumber
			body, event, commitID := args.Body, string(args.Event), args.CommitID

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
			return pullRequestMessageResult(result, err)
		},
		normalizeGranularPullRequestArguments("create_review"),
	)
	st.FeatureRule = pullRequestsGranularFeatureRule
	return st
}

// GranularSubmitPendingPullRequestReview creates a tool to submit a pending review.
func GranularSubmitPendingPullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "submit_pending_pull_request_review",
			Description: t("TOOL_SUBMIT_PENDING_PULL_REQUEST_REVIEW_DESCRIPTION", "Submit a pending pull request review."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_SUBMIT_PENDING_PULL_REQUEST_REVIEW_USER_TITLE", "Submit Pending Pull Request Review"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":      {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":       {Type: "string", Description: "Repository name"},
					"pullNumber": {Type: "number", Description: "The pull request number", Minimum: new(1.0)},
					"event":      {Type: "string", Description: "The review action to perform", Enum: GranularPullRequestReviewEvent("").JSONSchema().Enum},
					"body":       {Type: "string", Description: "The review body text (optional)"},
				},
				Required: []string{"owner", "repo", "pullNumber", "event"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularSubmitPullRequestReviewInput) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
			owner, repo, pullNumber, event, body := args.Owner, args.Repo, args.PullNumber, string(args.Event), args.Body

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
			return pullRequestMessageResult(result, err)
		},
		normalizeGranularPullRequestArguments("submit_review"),
	)
	st.FeatureRule = pullRequestsGranularFeatureRule
	return st
}

// GranularDeletePendingPullRequestReview creates a tool to delete a pending review.
func GranularDeletePendingPullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "delete_pending_pull_request_review",
			Description: t("TOOL_DELETE_PENDING_PULL_REQUEST_REVIEW_DESCRIPTION", "Delete a pending pull request review."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_DELETE_PENDING_PULL_REQUEST_REVIEW_USER_TITLE", "Delete Pending Pull Request Review"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":      {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":       {Type: "string", Description: "Repository name"},
					"pullNumber": {Type: "number", Description: "The pull request number", Minimum: new(1.0)},
				},
				Required: []string{"owner", "repo", "pullNumber"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularPullRequestCoordinate) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
			owner, repo, pullNumber := args.Owner, args.Repo, args.PullNumber

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			result, err := DeletePendingPullRequestReview(ctx, gqlClient, PullRequestReviewWriteParams{
				Owner:      owner,
				Repo:       repo,
				PullNumber: int32(pullNumber), // #nosec G115 - PR numbers are always small positive integers
			})
			return pullRequestMessageResult(result, err)
		},
		normalizeGranularPullRequestArguments("delete_review"),
	)
	st.FeatureRule = pullRequestsGranularFeatureRule
	return st
}

// GranularAddPullRequestReviewComment creates a tool to add a review comment.
func GranularAddPullRequestReviewComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "add_pull_request_review_comment",
			Description: t("TOOL_ADD_PULL_REQUEST_REVIEW_COMMENT_DESCRIPTION", "Add a review comment to the current user's pending pull request review."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ADD_PULL_REQUEST_REVIEW_COMMENT_USER_TITLE", "Add Pull Request Review Comment"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":       {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":        {Type: "string", Description: "Repository name"},
					"pullNumber":  {Type: "number", Description: "The pull request number", Minimum: new(1.0)},
					"path":        {Type: "string", Description: "The relative path of the file to comment on"},
					"body":        {Type: "string", Description: "The comment body"},
					"subjectType": {Type: "string", Description: "The subject type of the comment", Enum: GranularPullRequestCommentSubject("").JSONSchema().Enum},
					"line":        {Type: "number", Description: "The line number in the diff to comment on (optional)"},
					"side":        {Type: "string", Description: "The side of the diff to comment on (optional)", Enum: GranularPullRequestDiffSide("").JSONSchema().Enum},
					"startLine":   {Type: "number", Description: "The start line of a multi-line comment (optional)"},
					"startSide":   {Type: "string", Description: "The start side of a multi-line comment (optional)", Enum: GranularPullRequestDiffSide("").JSONSchema().Enum},
				},
				Required: []string{"owner", "repo", "pullNumber", "path", "body", "subjectType"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularPullRequestReviewCommentInput) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
			owner, repo, pullNumber := args.Owner, args.Repo, args.PullNumber

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			// Convert optional int params to *int32 for the helper
			var linePtr, startLinePtr *int32
			if args.Line != nil && *args.Line != 0 {
				l := int32(*args.Line) // #nosec G115
				linePtr = &l
			}
			if args.StartLine != nil && *args.StartLine != 0 {
				sl := int32(*args.StartLine) // #nosec G115
				startLinePtr = &sl
			}

			// Convert optional string params: pass nil (not empty string) when absent
			var sidePtr, startSidePtr *string
			if args.Side != nil && *args.Side != "" {
				side := string(*args.Side)
				sidePtr = &side
			}
			if args.StartSide != nil && *args.StartSide != "" {
				startSide := string(*args.StartSide)
				startSidePtr = &startSide
			}

			result, err := AddCommentToPendingReviewCall(ctx, gqlClient, AddCommentToPendingReviewParams{
				Owner:       owner,
				Repo:        repo,
				PullNumber:  int32(pullNumber), // #nosec G115 - PR numbers are always small positive integers
				Path:        args.Path,
				Body:        args.Body,
				SubjectType: string(args.SubjectType),
				Line:        linePtr,
				Side:        sidePtr,
				StartLine:   startLinePtr,
				StartSide:   startSidePtr,
			})
			return pullRequestMessageResult(result, err)
		},
		normalizeGranularPullRequestArguments("comment"),
	)
	st.FeatureRule = pullRequestsGranularFeatureRule
	return st
}

// GranularResolveReviewThread creates a tool to resolve a review thread.
func GranularResolveReviewThread(t translations.TranslationHelperFunc) inventory.ServerTool {
	return granularResolveReviewThread(t, false, toolConfig{})
}

// GranularResolveReviewThreadWithResolutionReason creates the feature-gated variant with resolution reasons.
func GranularResolveReviewThreadWithResolutionReason(t translations.TranslationHelperFunc, opts ...ToolOption) inventory.ServerTool {
	cfg := newToolConfig(opts)
	st := granularResolveReviewThread(t, true, cfg)
	if cfg.hostType == utils.HostTypeGHES {
		st.Enabled = func(context.Context) (bool, error) { return false, nil }
	}
	return st
}

func granularResolveReviewThread(t translations.TranslationHelperFunc, withResolutionReason bool, cfg toolConfig) inventory.ServerTool {
	withResolutionReason = withResolutionReason && cfg.hostType != utils.HostTypeGHES

	properties := map[string]*jsonschema.Schema{
		"threadID": {
			Type:        "string",
			Description: "The node ID of the review thread to resolve (e.g., PRRT_kwDOxxx)",
		},
	}
	if withResolutionReason {
		properties["resolutionReason"] = &jsonschema.Schema{
			Type:        "string",
			Description: "Optional reason for resolving a Copilot code review thread: addressed, wont-fix, or invalid.",
		}
	}

	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "resolve_review_thread",
			Description: t("TOOL_RESOLVE_REVIEW_THREAD_DESCRIPTION", "Resolve a review thread on a pull request. Resolving an already-resolved thread is a no-op."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_RESOLVE_REVIEW_THREAD_USER_TITLE", "Resolve Review Thread"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: properties,
				Required:   []string{"threadID"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularResolveReviewThreadInput) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
			threadID := args.ThreadID
			var resolutionReasonPtr *string
			if withResolutionReason {
				resolutionReasonPtr = args.ResolutionReason
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			if !withResolutionReason {
				result, err := ResolveReviewThread(ctx, gqlClient, threadID, true)
				return pullRequestMessageResult(result, err)
			}
			result, err := ResolveReviewThreadWithReason(ctx, gqlClient, threadID, resolutionReasonPtr, true)
			return pullRequestMessageResult(result, err)
		},
		normalizeGranularResolveReviewThreadArguments(withResolutionReason),
	)
	switch {
	case withResolutionReason:
		st.FeatureRule = inventory.NewFeatureRule(
			[]inventory.FeatureFlag{inventory.FeatureFlag(FeatureFlagPullRequestsGranular), FeatureFlagThreadResolutionReason},
			func(featureAsBool inventory.FeatureResolver) bool {
				return featureAsBool(inventory.FeatureFlag(FeatureFlagPullRequestsGranular)) &&
					featureAsBool(FeatureFlagThreadResolutionReason)
			},
		)
	case cfg.hostType == utils.HostTypeGHES:
		st.FeatureRule = pullRequestsGranularFeatureRule
	default:
		st.FeatureRule = inventory.NewFeatureRule(
			[]inventory.FeatureFlag{inventory.FeatureFlag(FeatureFlagPullRequestsGranular), FeatureFlagThreadResolutionReason},
			func(featureAsBool inventory.FeatureResolver) bool {
				return featureAsBool(inventory.FeatureFlag(FeatureFlagPullRequestsGranular)) &&
					!featureAsBool(FeatureFlagThreadResolutionReason)
			},
		)
	}
	return st
}

// GranularUnresolveReviewThread creates a tool to unresolve a review thread.
func GranularUnresolveReviewThread(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "unresolve_review_thread",
			Description: t("TOOL_UNRESOLVE_REVIEW_THREAD_DESCRIPTION", "Unresolve a previously resolved review thread on a pull request. Unresolving an already-unresolved thread is a no-op."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UNRESOLVE_REVIEW_THREAD_USER_TITLE", "Unresolve Review Thread"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"threadID": {
						Type:        "string",
						Description: "The node ID of the review thread to unresolve (e.g., PRRT_kwDOxxx)",
					},
				},
				Required: []string{"threadID"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularReviewThreadInput) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
			threadID := args.ThreadID

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			result, err := ResolveReviewThread(ctx, gqlClient, threadID, false)
			return pullRequestMessageResult(result, err)
		},
		normalizeGranularPullRequestArguments("unresolve"),
	)
	st.FeatureRule = pullRequestsGranularFeatureRule
	return st
}

// GranularAddPullRequestReviewCommentReaction adds a reaction to a pull request review comment.
func GranularAddPullRequestReviewCommentReaction(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "add_pull_request_review_comment_reaction",
			OutputSchema: minimalPullRequestCommentReactionSchema(),
			Description:  t("TOOL_ADD_PULL_REQUEST_REVIEW_COMMENT_REACTION_DESCRIPTION", "Add a reaction to a pull request review comment."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ADD_PULL_REQUEST_REVIEW_COMMENT_REACTION_USER_TITLE", "Add Pull Request Review Comment Reaction"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"comment_id": {
						Type:        "number",
						Description: "The numeric pull request review comment ID. Use the number from a #discussion_r... anchor, not the GraphQL thread node ID (PRRT_...).",
						Minimum:     new(1.0),
					},
					"content": {
						Type:        "string",
						Description: "The emoji reaction type",
						Enum:        PullRequestCommentReactionType("").JSONSchema().Enum,
					},
				},
				Required: []string{"owner", "repo", "comment_id", "content"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularAddPullRequestCommentReactionInput) (*mcp.CallToolResult, *MinimalPullRequestCommentReaction, error) {
			owner, repo, commentID, content := args.Owner, args.Repo, args.CommentID, args.Content

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			reaction, resp, err := client.Reactions.CreatePullRequestCommentReaction(ctx, owner, repo, commentID, string(content))
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to add reaction to pull request review comment", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			output := &MinimalResponse{
				ID:  fmt.Sprintf("%d", reaction.GetID()),
				URL: fmt.Sprintf("%srepos/%s/%s/pulls/comments/%d/reactions/%d", client.BaseURL(), owner, repo, commentID, reaction.GetID()),
			}
			r, err := json.Marshal(output)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), output, nil
		},
		normalizeGranularPullRequestArguments("add_reaction"),
	)
	st.FeatureRule = pullRequestsGranularFeatureRule
	return st
}

// GranularRemovePullRequestReviewCommentReaction removes a reaction from a pull request review comment.
func GranularRemovePullRequestReviewCommentReaction(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := newGranularPullRequestTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "remove_pull_request_review_comment_reaction",
			Description: t("TOOL_REMOVE_PULL_REQUEST_REVIEW_COMMENT_REACTION_DESCRIPTION", "Remove a reaction from a pull request review comment."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REMOVE_PULL_REQUEST_REVIEW_COMMENT_REACTION_USER_TITLE", "Remove Pull Request Review Comment Reaction"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"comment_id": {
						Type:        "number",
						Description: "The numeric pull request review comment ID. Use the number from a #discussion_r... anchor, not the GraphQL thread node ID (PRRT_...).",
						Minimum:     new(1.0),
					},
					"reaction_id": {
						Type:        "number",
						Description: "The reaction ID to remove",
						Minimum:     new(1.0),
					},
				},
				Required: []string{"owner", "repo", "comment_id", "reaction_id"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args GranularRemovePullRequestCommentReactionInput) (*mcp.CallToolResult, *RepositoryMessageOutput, error) {
			owner, repo, commentID, reactionID := args.Owner, args.Repo, args.CommentID, args.ReactionID

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			resp, err := client.Reactions.DeletePullRequestCommentReaction(ctx, owner, repo, commentID, reactionID)
			if resp != nil && resp.Body != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to remove reaction from pull request review comment", resp, err), nil, nil
			}

			return pullRequestMessageResult(utils.NewToolResultText("reaction successfully removed from pull request review comment"), nil)
		},
		normalizeGranularPullRequestArguments("remove_reaction"),
	)
	st.FeatureRule = pullRequestsGranularFeatureRule
	return st
}

// GranularHidePullRequestReviewComment hides (minimizes) an inline pull request review comment.
func GranularHidePullRequestReviewComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return commentVisibilityTool(t, pullRequestReviewCommentVisibilityTarget, true)
}

// GranularUnhidePullRequestReviewComment unhides (unminimizes) an inline pull request review comment.
func GranularUnhidePullRequestReviewComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return commentVisibilityTool(t, pullRequestReviewCommentVisibilityTarget, false)
}

// GranularHidePullRequestReview hides (minimizes) the body of a pull request review.
func GranularHidePullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	return commentVisibilityTool(t, pullRequestReviewVisibilityTarget, true)
}

// GranularUnhidePullRequestReview unhides (unminimizes) the body of a pull request review.
func GranularUnhidePullRequestReview(t translations.TranslationHelperFunc) inventory.ServerTool {
	return commentVisibilityTool(t, pullRequestReviewVisibilityTarget, false)
}
