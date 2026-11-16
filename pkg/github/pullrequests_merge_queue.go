package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/octicons"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// mergeQueueEntryNode is the merge queue entry selection shared by the
// enqueue, dequeue and status queries.
type mergeQueueEntryNode struct {
	ID                   githubv4.ID
	Position             githubv4.Int
	State                githubv4.MergeQueueEntryState
	EnqueuedAt           githubv4.DateTime
	EstimatedTimeToMerge *githubv4.Int
	Jump                 githubv4.Boolean
	Solo                 githubv4.Boolean
}

// MinimalMergeQueueEntry is a pull request's entry in its base branch's merge queue.
type MinimalMergeQueueEntry struct {
	ID                   string `json:"id"`
	Position             int    `json:"position"`
	State                string `json:"state"`
	EnqueuedAt           string `json:"enqueued_at"`
	EstimatedTimeToMerge *int   `json:"estimated_time_to_merge,omitempty"`
	Jump                 bool   `json:"jump"`
	Solo                 bool   `json:"solo"`
}

// MinimalMergeQueueStatus reports whether a pull request's base branch uses a
// merge queue and, if the pull request is queued, its entry.
type MinimalMergeQueueStatus struct {
	MergeQueueEnabled bool                    `json:"merge_queue_enabled"`
	InMergeQueue      bool                    `json:"in_merge_queue"`
	Entry             *MinimalMergeQueueEntry `json:"entry,omitempty"`
}

func convertToMinimalMergeQueueEntry(node *mergeQueueEntryNode) *MinimalMergeQueueEntry {
	if node == nil {
		return nil
	}
	entry := &MinimalMergeQueueEntry{
		ID:       fmt.Sprintf("%v", node.ID),
		Position: int(node.Position),
		State:    string(node.State),
		Jump:     bool(node.Jump),
		Solo:     bool(node.Solo),
	}
	if !node.EnqueuedAt.IsZero() {
		entry.EnqueuedAt = node.EnqueuedAt.UTC().Format(time.RFC3339)
	}
	if node.EstimatedTimeToMerge != nil {
		seconds := int(*node.EstimatedTimeToMerge)
		entry.EstimatedTimeToMerge = &seconds
	}
	return entry
}

// PullRequestMergeQueueEntryOutput is the merge queue entry an enqueue or
// dequeue mutation returned; null when GitHub returned none.
type PullRequestMergeQueueEntryOutput struct {
	Entry *MinimalMergeQueueEntry
}

func (out PullRequestMergeQueueEntryOutput) MarshalJSON() ([]byte, error) {
	return json.Marshal(out.Entry)
}

func pullRequestMergeQueueEntryOutputSchema() *jsonschema.Schema {
	return nullableRepositoryOutputSchema[MinimalMergeQueueEntry]()
}

type EnqueuePullRequestInput struct {
	Owner           string  `json:"owner"`
	Repo            string  `json:"repo"`
	PullNumber      int     `json:"pullNumber"`
	Jump            *bool   `json:"jump,omitempty"`
	ExpectedHeadSHA *string `json:"expectedHeadSha,omitempty"`
}

type DequeuePullRequestInput struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	PullNumber int    `json:"pullNumber"`
}

func mergeQueuePullRequestProperties() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
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
	}
}

// EnqueuePullRequest creates a tool that adds a pull request to its base branch's merge queue.
func EnqueuePullRequest(t translations.TranslationHelperFunc) inventory.ServerTool {
	properties := mergeQueuePullRequestProperties()
	properties["jump"] = &jsonschema.Schema{
		Type:        "boolean",
		Description: "Add the pull request to the front of the queue instead of the back",
	}
	properties["expectedHeadSha"] = &jsonschema.Schema{
		Type:        "string",
		Description: "The expected SHA of the pull request's HEAD ref. The pull request is only queued if its head still matches.",
	}

	return NewTool[EnqueuePullRequestInput, *PullRequestMergeQueueEntryOutput](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "enqueue_pull_request",
			OutputSchema: pullRequestMergeQueueEntryOutputSchema(),
			Description: t("TOOL_ENQUEUE_PULL_REQUEST_DESCRIPTION",
				"Add a pull request to the merge queue of its base branch. Use this instead of merge_pull_request when the base branch requires a merge queue (see the get_merge_queue_status method of pull_request_read); the queue merges the pull request once its checks pass. Returns the merge queue entry."),
			Icons: octicons.Icons("git-merge"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ENQUEUE_PULL_REQUEST_USER_TITLE", "Add pull request to merge queue"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: properties,
				Required:   []string{"owner", "repo", "pullNumber"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input EnqueuePullRequestInput) (*mcp.CallToolResult, *PullRequestMergeQueueEntryOutput, error) {
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
			jump, err := OptionalParam[bool](args, "jump")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			expectedHeadSHA, err := OptionalParam[string](args, "expectedHeadSha")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}
			pullRequestID, err := resolvePullRequestNodeID(ctx, gqlClient, owner, repo, pullNumber)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to find pull request", err), nil, nil
			}

			mutationInput := githubv4.EnqueuePullRequestInput{PullRequestID: pullRequestID}
			if jump {
				mutationInput.Jump = githubv4.NewBoolean(githubv4.Boolean(true))
			}
			if expectedHeadSHA != "" {
				mutationInput.ExpectedHeadOid = githubv4.NewGitObjectID(githubv4.GitObjectID(expectedHeadSHA))
			}

			var mutation struct {
				EnqueuePullRequest struct {
					MergeQueueEntry *mergeQueueEntryNode
				} `graphql:"enqueuePullRequest(input: $input)"`
			}
			if err := gqlClient.Mutate(ctx, &mutation, mutationInput, nil); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to add pull request to the merge queue", err), nil, nil
			}

			entry := convertToMinimalMergeQueueEntry(mutation.EnqueuePullRequest.MergeQueueEntry)
			return MarshalledTextResult(entry), &PullRequestMergeQueueEntryOutput{Entry: entry}, nil
		}, normalizePullRequestArguments("enqueue"))
}

// DequeuePullRequest creates a tool that removes a pull request from its base branch's merge queue.
func DequeuePullRequest(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[DequeuePullRequestInput, *PullRequestMergeQueueEntryOutput](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:         "dequeue_pull_request",
			OutputSchema: pullRequestMergeQueueEntryOutputSchema(),
			Description: t("TOOL_DEQUEUE_PULL_REQUEST_DESCRIPTION",
				"Remove a pull request from the merge queue of its base branch. Returns the removed merge queue entry."),
			Icons: octicons.Icons("git-merge"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_DEQUEUE_PULL_REQUEST_USER_TITLE", "Remove pull request from merge queue"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: mergeQueuePullRequestProperties(),
				Required:   []string{"owner", "repo", "pullNumber"},
			},
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input DequeuePullRequestInput) (*mcp.CallToolResult, *PullRequestMergeQueueEntryOutput, error) {
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

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}
			pullRequestID, err := resolvePullRequestNodeID(ctx, gqlClient, owner, repo, pullNumber)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to find pull request", err), nil, nil
			}

			var mutation struct {
				DequeuePullRequest struct {
					MergeQueueEntry *mergeQueueEntryNode
				} `graphql:"dequeuePullRequest(input: $input)"`
			}
			if err := gqlClient.Mutate(ctx, &mutation, githubv4.DequeuePullRequestInput{ID: pullRequestID}, nil); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to remove pull request from the merge queue", err), nil, nil
			}

			entry := convertToMinimalMergeQueueEntry(mutation.DequeuePullRequest.MergeQueueEntry)
			return MarshalledTextResult(entry), &PullRequestMergeQueueEntryOutput{Entry: entry}, nil
		}, normalizePullRequestArguments("dequeue"))
}

// getPullRequestMergeQueueStatus implements the get_merge_queue_status method of pull_request_read.
func getPullRequestMergeQueueStatus(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, pullNumber int) (*mcp.CallToolResult, *PullRequestReadOutput, error) {
	var query struct {
		Repository struct {
			PullRequest struct {
				IsMergeQueueEnabled githubv4.Boolean
				IsInMergeQueue      githubv4.Boolean
				MergeQueueEntry     *mergeQueueEntryNode
			} `graphql:"pullRequest(number: $prNum)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	vars := map[string]any{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
		"prNum": githubv4.Int(int32(pullNumber)), //nolint:gosec // pullNumber is controlled by user input validation
	}
	if err := gqlClient.Query(ctx, &query, vars); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get pull request merge queue status", err), nil, nil
	}

	pr := query.Repository.PullRequest
	status := MinimalMergeQueueStatus{
		MergeQueueEnabled: bool(pr.IsMergeQueueEnabled),
		InMergeQueue:      bool(pr.IsInMergeQueue),
		Entry:             convertToMinimalMergeQueueEntry(pr.MergeQueueEntry),
	}
	return MarshalledTextResult(status), &PullRequestReadOutput{MergeQueueStatus: &status}, nil
}
