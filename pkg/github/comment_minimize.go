package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// MinimizeCommentResult is the response returned by the hide and unhide comment tools.
type MinimizeCommentResult struct {
	NodeID          string `json:"node_id"`
	IsMinimized     bool   `json:"is_minimized"`
	MinimizedReason string `json:"minimized_reason,omitempty" jsonschema:"GitHub's lowercase, hyphenated reason (for example off-topic); omitted when unhidden."`
}

// CommentVisibilityInput identifies the comment or review whose visibility changes.
// The factory's target-specific schema determines which identifiers are required.
type CommentVisibilityInput struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	CommentID  int64  `json:"comment_id,omitempty"`
	PullNumber int    `json:"pullNumber,omitempty"`
	ReviewID   int64  `json:"review_id,omitempty"`
	Classifier string `json:"classifier,omitempty"`
}

var commentClassifiers = []any{"SPAM", "ABUSE", "OFF_TOPIC", "OUTDATED", "DUPLICATE", "RESOLVED", "LOW_QUALITY"}

const commentVisibilityPermissionNote = " Requires triage or write access to the repository, or being its author."

// commentVisibilityTarget describes one kind of hideable object: how the tools that
// hide and unhide it are named and described, and how to find its GraphQL node ID.
type commentVisibilityTarget struct {
	toolset     inventory.ToolsetMetadata
	featureRule inventory.FeatureRule
	// name is appended to "hide_" and "unhide_" to form the tool names.
	name              string
	title             string
	hideDescription   string
	unhideDescription string
	properties        func() map[string]*jsonschema.Schema
	required          []string
	resolveNodeID     func(ctx context.Context, client *github.Client, input CommentVisibilityInput) (string, *mcp.CallToolResult)
}

var issueCommentVisibilityTarget = commentVisibilityTarget{
	toolset:           ToolsetMetadataIssues,
	featureRule:       issuesGranularFeatureRule,
	name:              "issue_comment",
	title:             "Issue Comment",
	hideDescription:   "Hide (minimize) a comment on an issue, or a conversation comment on a pull request.",
	unhideDescription: "Unhide (unminimize) a previously hidden comment on an issue, or conversation comment on a pull request.",
	properties: func() map[string]*jsonschema.Schema {
		return map[string]*jsonschema.Schema{
			"comment_id": {
				Type:        "number",
				Description: "The numeric ID of the issue or pull request conversation comment",
				Minimum:     new(1.0),
			},
		}
	},
	required: []string{"comment_id"},
	resolveNodeID: func(ctx context.Context, client *github.Client, input CommentVisibilityInput) (string, *mcp.CallToolResult) {
		comment, resp, err := client.Issues.GetComment(ctx, input.Owner, input.Repo, input.CommentID)
		return nodeIDFromResponse(ctx, "failed to get issue comment", comment.GetNodeID(), resp, err)
	},
}

var pullRequestReviewCommentVisibilityTarget = commentVisibilityTarget{
	toolset:           ToolsetMetadataPullRequests,
	featureRule:       pullRequestsGranularFeatureRule,
	name:              "pull_request_review_comment",
	title:             "Pull Request Review Comment",
	hideDescription:   "Hide (minimize) an inline review comment on a pull request diff.",
	unhideDescription: "Unhide (unminimize) a previously hidden inline review comment on a pull request diff.",
	properties: func() map[string]*jsonschema.Schema {
		return map[string]*jsonschema.Schema{
			"comment_id": {
				Type:        "number",
				Description: "The numeric pull request review comment ID. Use the number from a #discussion_r... anchor, not the GraphQL thread node ID (PRRT_...).",
				Minimum:     new(1.0),
			},
		}
	},
	required: []string{"comment_id"},
	resolveNodeID: func(ctx context.Context, client *github.Client, input CommentVisibilityInput) (string, *mcp.CallToolResult) {
		comment, resp, err := client.PullRequests.GetComment(ctx, input.Owner, input.Repo, input.CommentID)
		return nodeIDFromResponse(ctx, "failed to get pull request review comment", comment.GetNodeID(), resp, err)
	},
}

var pullRequestReviewVisibilityTarget = commentVisibilityTarget{
	toolset:           ToolsetMetadataPullRequests,
	featureRule:       pullRequestsGranularFeatureRule,
	name:              "pull_request_review",
	title:             "Pull Request Review",
	hideDescription:   "Hide (minimize) the body of a submitted pull request review.",
	unhideDescription: "Unhide (unminimize) the previously hidden body of a submitted pull request review.",
	properties: func() map[string]*jsonschema.Schema {
		return map[string]*jsonschema.Schema{
			"pullNumber": {
				Type:        "number",
				Description: "The pull request number",
				Minimum:     new(1.0),
			},
			"review_id": {
				Type:        "number",
				Description: "The numeric ID of the pull request review",
				Minimum:     new(1.0),
			},
		}
	},
	required: []string{"pullNumber", "review_id"},
	resolveNodeID: func(ctx context.Context, client *github.Client, input CommentVisibilityInput) (string, *mcp.CallToolResult) {
		review, resp, err := client.PullRequests.GetReview(ctx, input.Owner, input.Repo, input.PullNumber, input.ReviewID)
		return nodeIDFromResponse(ctx, "failed to get pull request review", review.GetNodeID(), resp, err)
	},
}

// commentVisibilityTool builds the hide_<target> tool when hide is true, and the unhide_<target> tool otherwise.
func commentVisibilityTool(t translations.TranslationHelperFunc, target commentVisibilityTarget, hide bool) inventory.ServerTool {
	action, titleAction, description := "unhide", "Unhide", target.unhideDescription
	if hide {
		action, titleAction, description = "hide", "Hide", target.hideDescription
	}
	name := action + "_" + target.name

	properties := map[string]*jsonschema.Schema{
		"owner": {
			Type:        "string",
			Description: "Repository owner (username or organization)",
		},
		"repo": {
			Type:        "string",
			Description: "Repository name",
		},
	}
	maps.Copy(properties, target.properties())
	required := append([]string{"owner", "repo"}, target.required...)
	if hide {
		properties["classifier"] = &jsonschema.Schema{
			Type:        "string",
			Description: "The reason for hiding the comment",
			Enum:        commentClassifiers,
		}
		required = append(required, "classifier")
	}

	outputSchema, err := inventory.CachedSchemaFor[MinimizeCommentResult](nil)
	if err != nil {
		panic(fmt.Sprintf("failed to generate comment visibility output schema: %v", err))
	}
	st := NewTool[CommentVisibilityInput, MinimizeCommentResult](
		target.toolset,
		mcp.Tool{
			Name:        name,
			Description: t("TOOL_"+strings.ToUpper(name)+"_DESCRIPTION", description+commentVisibilityPermissionNote),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_"+strings.ToUpper(name)+"_USER_TITLE", titleAction+" "+target.title),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: properties,
				Required:   required,
			},
			OutputSchema: outputSchema,
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input CommentVisibilityInput) (*mcp.CallToolResult, MinimizeCommentResult, error) {
			result, output := setCommentVisibility(ctx, deps, target, input, hide)
			return result, output, nil
		},
		normalizeCommentVisibilityArguments(target, hide),
	)
	st.FeatureRule = target.featureRule
	return st
}

func normalizeCommentVisibilityArguments(target commentVisibilityTarget, hide bool) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		if args == nil {
			return raw, nil
		}
		input, err := normalizeCommentVisibilityInput(args, target, hide)
		if err != nil {
			return nil, err
		}
		normalized, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("marshal normalized comment visibility arguments: %w", err)
		}
		return normalized, nil
	}
}

func normalizeCommentVisibilityInput(args map[string]any, target commentVisibilityTarget, hide bool) (CommentVisibilityInput, error) {
	var input CommentVisibilityInput
	var err error
	input.Owner, err = RequiredParam[string](args, "owner")
	if err != nil {
		return input, err
	}
	input.Repo, err = RequiredParam[string](args, "repo")
	if err != nil {
		return input, err
	}
	if hide {
		input.Classifier, err = RequiredParam[string](args, "classifier")
		if err != nil {
			return input, err
		}
		input.Classifier = strings.ToUpper(input.Classifier)
		if !slices.Contains(commentClassifiers, any(input.Classifier)) {
			return input, fmt.Errorf("invalid classifier %q: must be one of %v", input.Classifier, commentClassifiers)
		}
	}
	for _, name := range target.required {
		if name == "pullNumber" {
			value, err := RequiredInt(args, name)
			if err != nil {
				return input, err
			}
			if value < 1 {
				return input, fmt.Errorf("pullNumber must be greater than 0")
			}
			input.PullNumber = value
			continue
		}
		value, err := requiredPositiveBigInt(args, name)
		if err != nil {
			return input, err
		}
		if name == "comment_id" {
			input.CommentID = value
		} else {
			input.ReviewID = value
		}
	}
	return input, nil
}

// setCommentVisibility resolves the object identified by input and hides or unhides it.
func setCommentVisibility(ctx context.Context, deps ToolDependencies, target commentVisibilityTarget, input CommentVisibilityInput, hide bool) (*mcp.CallToolResult, MinimizeCommentResult) {
	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), MinimizeCommentResult{}
	}

	nodeID, errResult := target.resolveNodeID(ctx, client, input)
	if errResult != nil {
		return errResult, MinimizeCommentResult{}
	}

	gqlClient, err := deps.GetGQLClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), MinimizeCommentResult{}
	}

	var result MinimizeCommentResult
	if hide {
		result, errResult = minimizeComment(ctx, gqlClient, nodeID, input.Classifier)
	} else {
		result, errResult = unminimizeComment(ctx, gqlClient, nodeID)
	}
	if errResult != nil {
		return errResult, MinimizeCommentResult{}
	}

	r, err := json.Marshal(result)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal response", err), MinimizeCommentResult{}
	}
	return utils.NewToolResultText(string(r)), result
}

// requiredPositiveBigInt validates legacy numeric IDs before SDK schema validation.
func requiredPositiveBigInt(args map[string]any, p string) (int64, error) {
	v, err := RequiredBigInt(args, p)
	if err != nil {
		return 0, err
	}
	if v < 1 {
		return 0, fmt.Errorf("%s must be greater than 0", p)
	}
	return v, nil
}

// nodeIDFromResponse turns the result of a REST lookup into a GraphQL node ID, since the
// minimize mutations only accept node IDs.
func nodeIDFromResponse(ctx context.Context, errMessage, nodeID string, resp *github.Response, err error) (string, *mcp.CallToolResult) {
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return "", ghErrors.NewGitHubAPIErrorResponse(ctx, errMessage, resp, err)
	}
	if nodeID == "" {
		return "", utils.NewToolResultError(errMessage + ": response has no node ID")
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
