package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

const (
	CommentTypeIssueComment             = "issue_comment"
	CommentTypePullRequestReviewComment = "pull_request_review_comment"
	CommentTypePullRequestReview        = "pull_request_review"
)

// MinimizeCommentResult is the response returned by the hide_comment and unhide_comment tools.
type MinimizeCommentResult struct {
	NodeID          string `json:"node_id"`
	IsMinimized     bool   `json:"is_minimized"`
	MinimizedReason string `json:"minimized_reason,omitempty"`
}

var commentClassifiers = []any{"SPAM", "ABUSE", "OFF_TOPIC", "OUTDATED", "DUPLICATE", "RESOLVED", "LOW_QUALITY"}

const commentVisibilityDescriptionSuffix = "Supports issue and pull request conversation comments, pull request review comments, and pull request review bodies. " +
	"Requires triage or write access to the repository, or authorship of the comment."

// commentTargetProperties returns the schema properties that identify the comment to hide or unhide.
func commentTargetProperties() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"owner": {
			Type:        "string",
			Description: "Repository owner",
		},
		"repo": {
			Type:        "string",
			Description: "Repository name",
		},
		"comment_type": {
			Type: "string",
			Description: "The kind of comment:\n" +
				"- 'issue_comment' - a comment on an issue, or a conversation comment on a pull request.\n" +
				"- 'pull_request_review_comment' - an inline comment on a pull request diff.\n" +
				"- 'pull_request_review' - the body of a pull request review. Requires 'pull_number'.",
			Enum: []any{CommentTypeIssueComment, CommentTypePullRequestReviewComment, CommentTypePullRequestReview},
		},
		"comment_id": {
			Type:        "integer",
			Description: "The numeric ID of the comment, or of the review when comment_type is 'pull_request_review'",
			Minimum:     jsonschema.Ptr(1.0),
		},
		"pull_number": {
			Type:        "number",
			Description: "Pull request number. Required when comment_type is 'pull_request_review'.",
		},
	}
}

// setCommentVisibility resolves the comment identified by args and hides or unhides it.
func setCommentVisibility(ctx context.Context, deps ToolDependencies, args map[string]any, hide bool, classifier string) *mcp.CallToolResult {
	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return utils.NewToolResultError(err.Error())
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error())
	}
	commentType, err := RequiredParam[string](args, "comment_type")
	if err != nil {
		return utils.NewToolResultError(err.Error())
	}
	commentID, err := RequiredBigInt(args, "comment_id")
	if err != nil {
		return utils.NewToolResultError(err.Error())
	}
	pullNumber, err := OptionalIntParam(args, "pull_number")
	if err != nil {
		return utils.NewToolResultError(err.Error())
	}

	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err)
	}

	nodeID, errResult := resolveCommentNodeID(ctx, client, owner, repo, commentType, commentID, pullNumber)
	if errResult != nil {
		return errResult
	}

	gqlClient, err := deps.GetGQLClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err)
	}

	var result MinimizeCommentResult
	if hide {
		result, errResult = minimizeComment(ctx, gqlClient, nodeID, strings.ToUpper(classifier))
	} else {
		result, errResult = unminimizeComment(ctx, gqlClient, nodeID)
	}
	if errResult != nil {
		return errResult
	}

	r, err := json.Marshal(result)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal response", err)
	}
	return utils.NewToolResultText(string(r))
}

// resolveCommentNodeID looks up the GraphQL node ID for a comment identified by its REST ID,
// since the minimize mutations only accept node IDs.
func resolveCommentNodeID(ctx context.Context, client *github.Client, owner, repo, commentType string, commentID int64, pullNumber int) (string, *mcp.CallToolResult) {
	var (
		nodeID string
		resp   *github.Response
		err    error
	)

	switch commentType {
	case CommentTypeIssueComment:
		var comment *github.IssueComment
		comment, resp, err = client.Issues.GetComment(ctx, owner, repo, commentID)
		nodeID = comment.GetNodeID()
	case CommentTypePullRequestReviewComment:
		var comment *github.PullRequestComment
		comment, resp, err = client.PullRequests.GetComment(ctx, owner, repo, commentID)
		nodeID = comment.GetNodeID()
	case CommentTypePullRequestReview:
		if pullNumber <= 0 {
			return "", utils.NewToolResultError("pull_number is required when comment_type is 'pull_request_review'")
		}
		var review *github.PullRequestReview
		review, resp, err = client.PullRequests.GetReview(ctx, owner, repo, pullNumber, commentID)
		nodeID = review.GetNodeID()
	default:
		return "", utils.NewToolResultError(fmt.Sprintf("unknown comment_type: %s", commentType))
	}

	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return "", ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get comment", resp, err)
	}
	if nodeID == "" {
		return "", utils.NewToolResultError("comment has no node ID")
	}
	return nodeID, nil
}

func minimizeComment(ctx context.Context, client *githubv4.Client, nodeID, classifier string) (MinimizeCommentResult, *mcp.CallToolResult) {
	var mutation struct {
		MinimizeComment struct {
			MinimizedComment struct {
				IsMinimized     githubv4.Boolean
				MinimizedReason githubv4.String
			}
		} `graphql:"minimizeComment(input: $input)"`
	}

	input := githubv4.MinimizeCommentInput{
		SubjectID:  githubv4.ID(nodeID),
		Classifier: githubv4.ReportedContentClassifiers(classifier),
	}
	if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
		return MinimizeCommentResult{}, ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to minimize comment", err)
	}

	return MinimizeCommentResult{
		NodeID:          nodeID,
		IsMinimized:     bool(mutation.MinimizeComment.MinimizedComment.IsMinimized),
		MinimizedReason: string(mutation.MinimizeComment.MinimizedComment.MinimizedReason),
	}, nil
}

func unminimizeComment(ctx context.Context, client *githubv4.Client, nodeID string) (MinimizeCommentResult, *mcp.CallToolResult) {
	var mutation struct {
		UnminimizeComment struct {
			UnminimizedComment struct {
				IsMinimized githubv4.Boolean
			}
		} `graphql:"unminimizeComment(input: $input)"`
	}

	input := githubv4.UnminimizeCommentInput{
		SubjectID: githubv4.ID(nodeID),
	}
	if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
		return MinimizeCommentResult{}, ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to unminimize comment", err)
	}

	return MinimizeCommentResult{
		NodeID:      nodeID,
		IsMinimized: bool(mutation.UnminimizeComment.UnminimizedComment.IsMinimized),
	}, nil
}
