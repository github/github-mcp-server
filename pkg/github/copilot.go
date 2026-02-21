package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// mvpDescription is an MVP idea for generating tool descriptions from structured data in a shared format.
// It is not intended for widespread usage and is not a complete implementation.
type mvpDescription struct {
	summary        string
	outcomes       []string
	referenceLinks []string
}

func (d *mvpDescription) String() string {
	var sb strings.Builder
	sb.WriteString(d.summary)
	if len(d.outcomes) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString("This tool can help with the following outcomes:\n")
		for _, outcome := range d.outcomes {
			sb.WriteString(fmt.Sprintf("- %s\n", outcome))
		}
	}

	if len(d.referenceLinks) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString("More information can be found at:\n")
		for _, link := range d.referenceLinks {
			sb.WriteString(fmt.Sprintf("- %s\n", link))
		}
	}

	return sb.String()
}

// linkedPullRequest represents a PR linked to an issue by Copilot.
type linkedPullRequest struct {
	Number    int
	URL       string
	Title     string
	State     string
	CreatedAt time.Time
}

// pollConfigKey is a context key for polling configuration.
type pollConfigKey struct{}

// PollConfig configures the PR polling behavior.
type PollConfig struct {
	MaxAttempts int
	Delay       time.Duration
}

// ContextWithPollConfig returns a context with polling configuration.
// Use this in tests to reduce or disable polling.
func ContextWithPollConfig(ctx context.Context, config PollConfig) context.Context {
	return context.WithValue(ctx, pollConfigKey{}, config)
}

// getPollConfig returns the polling configuration from context, or defaults.
func getPollConfig(ctx context.Context) PollConfig {
	if config, ok := ctx.Value(pollConfigKey{}).(PollConfig); ok {
		return config
	}
	// Default: 9 attempts with 1s delay = 8s max wait
	// Based on observed latency in remote server: p50 ~5s, p90 ~7s
	return PollConfig{MaxAttempts: 9, Delay: 1 * time.Second}
}

// findLinkedCopilotPR searches for a PR created by the copilot-swe-agent bot that references the given issue.
// It queries the issue's timeline for CrossReferencedEvent items from PRs authored by copilot-swe-agent.
// The createdAfter parameter filters to only return PRs created after the specified time.
func findLinkedCopilotPR(ctx context.Context, client *githubv4.Client, owner, repo string, issueNumber int, createdAfter time.Time) (*linkedPullRequest, error) {
	// Query timeline items looking for CrossReferencedEvent from PRs by copilot-swe-agent
	var query struct {
		Repository struct {
			Issue struct {
				TimelineItems struct {
					Nodes []struct {
						TypeName             string `graphql:"__typename"`
						CrossReferencedEvent struct {
							Source struct {
								PullRequest struct {
									Number    int
									URL       string
									Title     string
									State     string
									CreatedAt githubv4.DateTime
									Author    struct {
										Login string
									}
								} `graphql:"... on PullRequest"`
							}
						} `graphql:"... on CrossReferencedEvent"`
					}
				} `graphql:"timelineItems(first: 20, itemTypes: [CROSS_REFERENCED_EVENT])"`
			} `graphql:"issue(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]any{
		"owner":  githubv4.String(owner),
		"name":   githubv4.String(repo),
		"number": githubv4.Int(issueNumber), //nolint:gosec // Issue numbers are always small positive integers
	}

	if err := client.Query(ctx, &query, variables); err != nil {
		return nil, err
	}

	// Look for a PR from copilot-swe-agent created after the assignment time
	for _, node := range query.Repository.Issue.TimelineItems.Nodes {
		if node.TypeName != "CrossReferencedEvent" {
			continue
		}
		pr := node.CrossReferencedEvent.Source.PullRequest
		if pr.Number > 0 && pr.Author.Login == "copilot-swe-agent" {
			// Only return PRs created after the assignment time
			if pr.CreatedAt.Time.After(createdAfter) {
				return &linkedPullRequest{
					Number:    pr.Number,
					URL:       pr.URL,
					Title:     pr.Title,
					State:     pr.State,
					CreatedAt: pr.CreatedAt.Time,
				}, nil
			}
		}
	}

	return nil, nil
}

func AssignCopilotToIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	description := mvpDescription{
		summary: "Assign Copilot to a specific issue in a GitHub repository.",
		outcomes: []string{
			"a Pull Request created with source code changes to resolve the issue",
		},
		referenceLinks: []string{
			"https://docs.github.com/en/copilot/using-github-copilot/using-copilot-coding-agent-to-work-on-tasks/about-assigning-tasks-to-copilot",
		},
	}

	return NewTool(
		ToolsetMetadataCopilot,
		mcp.Tool{
			Name:        "assign_copilot_to_issue",
			Description: t("TOOL_ASSIGN_COPILOT_TO_ISSUE_DESCRIPTION", description.String()),
			Icons:       octicons.Icons("copilot"),
			Annotations: &mcp.ToolAnnotations{
				Title:          t("TOOL_ASSIGN_COPILOT_TO_ISSUE_USER_TITLE", "Assign Copilot to issue"),
				ReadOnlyHint:   false,
				IdempotentHint: true,
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
					"issue_number": {
						Type:        "number",
						Description: "Issue number",
					},
					"base_ref": {
						Type:        "string",
						Description: "Git reference (e.g., branch) that the agent will start its work from. If not specified, defaults to the repository's default branch",
					},
					"custom_instructions": {
						Type:        "string",
						Description: "Optional custom instructions to guide the agent beyond the issue body. Use this to provide additional context, constraints, or guidance that is not captured in the issue description",
					},
				},
				Required: []string{"owner", "repo", "issue_number"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, request *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			var params struct {
				Owner              string `mapstructure:"owner"`
				Repo               string `mapstructure:"repo"`
				IssueNumber        int32  `mapstructure:"issue_number"`
				BaseRef            string `mapstructure:"base_ref"`
				CustomInstructions string `mapstructure:"custom_instructions"`
			}
			if err := mapstructure.Decode(args, &params); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Firstly, we try to find the copilot bot in the suggested actors for the repository.
			// Although as I write this, we would expect copilot to be at the top of the list, in future, maybe
			// it will not be on the first page of responses, thus we will keep paginating until we find it.
			type botAssignee struct {
				ID       githubv4.ID
				Login    string
				TypeName string `graphql:"__typename"`
			}

			type suggestedActorsQuery struct {
				Repository struct {
					SuggestedActors struct {
						Nodes []struct {
							Bot botAssignee `graphql:"... on Bot"`
						}
						PageInfo struct {
							HasNextPage bool
							EndCursor   string
						}
					} `graphql:"suggestedActors(first: 100, after: $endCursor, capabilities: CAN_BE_ASSIGNED)"`
				} `graphql:"repository(owner: $owner, name: $name)"`
			}

			variables := map[string]any{
				"owner":     githubv4.String(params.Owner),
				"name":      githubv4.String(params.Repo),
				"endCursor": (*githubv4.String)(nil),
			}

			var copilotAssignee *botAssignee
			for {
				var query suggestedActorsQuery
				err := client.Query(ctx, &query, variables)
				if err != nil {
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get suggested actors", err), nil, nil
				}

				// Iterate all the returned nodes looking for the copilot bot, which is supposed to have the
				// same name on each host. We need this in order to get the ID for later assignment.
				for _, node := range query.Repository.SuggestedActors.Nodes {
					if node.Bot.Login == "copilot-swe-agent" {
						copilotAssignee = &node.Bot
						break
					}
				}

				if !query.Repository.SuggestedActors.PageInfo.HasNextPage {
					break
				}
				variables["endCursor"] = githubv4.String(query.Repository.SuggestedActors.PageInfo.EndCursor)
			}

			// If we didn't find the copilot bot, we can't proceed any further.
			if copilotAssignee == nil {
				// The e2e tests depend upon this specific message to skip the test.
				return utils.NewToolResultError("copilot isn't available as an assignee for this issue. Please inform the user to visit https://docs.github.com/en/copilot/using-github-copilot/using-copilot-coding-agent-to-work-on-tasks/about-assigning-tasks-to-copilot for more information."), nil, nil
			}

			// Next, get the issue ID and repository ID
			var getIssueQuery struct {
				Repository struct {
					ID    githubv4.ID
					Issue struct {
						ID        githubv4.ID
						Assignees struct {
							Nodes []struct {
								ID githubv4.ID
							}
						} `graphql:"assignees(first: 100)"`
					} `graphql:"issue(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $name)"`
			}

			variables = map[string]any{
				"owner":  githubv4.String(params.Owner),
				"name":   githubv4.String(params.Repo),
				"number": githubv4.Int(params.IssueNumber),
			}

			if err := client.Query(ctx, &getIssueQuery, variables); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get issue ID", err), nil, nil
			}

			// Build the assignee IDs list including copilot
			actorIDs := make([]githubv4.ID, len(getIssueQuery.Repository.Issue.Assignees.Nodes)+1)
			for i, node := range getIssueQuery.Repository.Issue.Assignees.Nodes {
				actorIDs[i] = node.ID
			}
			actorIDs[len(getIssueQuery.Repository.Issue.Assignees.Nodes)] = copilotAssignee.ID

			// Prepare agent assignment input
			emptyString := githubv4.String("")
			agentAssignment := &AgentAssignmentInput{
				CustomAgent:        &emptyString,
				CustomInstructions: &emptyString,
				TargetRepositoryID: getIssueQuery.Repository.ID,
			}

			// Add base ref if provided
			if params.BaseRef != "" {
				baseRef := githubv4.String(params.BaseRef)
				agentAssignment.BaseRef = &baseRef
			}

			// Add custom instructions if provided
			if params.CustomInstructions != "" {
				customInstructions := githubv4.String(params.CustomInstructions)
				agentAssignment.CustomInstructions = &customInstructions
			}

			// Execute the updateIssue mutation with the GraphQL-Features header
			// This header is required for the agent assignment API which is not GA yet
			var updateIssueMutation struct {
				UpdateIssue struct {
					Issue struct {
						ID     githubv4.ID
						Number githubv4.Int
						URL    githubv4.String
					}
				} `graphql:"updateIssue(input: $input)"`
			}

			// Add the GraphQL-Features header for the agent assignment API
			// The header will be read by the HTTP transport if it's configured to do so
			ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "issues_copilot_assignment_api_support")

			// Capture the time before assignment to filter out older PRs during polling
			assignmentTime := time.Now().UTC()

			if err := client.Mutate(
				ctxWithFeatures,
				&updateIssueMutation,
				UpdateIssueInput{
					ID:              getIssueQuery.Repository.Issue.ID,
					AssigneeIDs:     actorIDs,
					AgentAssignment: agentAssignment,
				},
				nil,
			); err != nil {
				return nil, nil, fmt.Errorf("failed to update issue with agent assignment: %w", err)
			}

			// Poll for a linked PR created by Copilot after the assignment
			pollConfig := getPollConfig(ctx)

			// Get progress token from request for sending progress notifications
			progressToken := request.Params.GetProgressToken()

			// Send initial progress notification that assignment succeeded and polling is starting
			if progressToken != nil && request.Session != nil && pollConfig.MaxAttempts > 0 {
				_ = request.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
					ProgressToken: progressToken,
					Progress:      0,
					Total:         float64(pollConfig.MaxAttempts),
					Message:       "Copilot assigned to issue, waiting for PR creation...",
				})
			}

			var linkedPR *linkedPullRequest
			for attempt := range pollConfig.MaxAttempts {
				if attempt > 0 {
					time.Sleep(pollConfig.Delay)
				}

				// Send progress notification if progress token is available
				if progressToken != nil && request.Session != nil {
					_ = request.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
						ProgressToken: progressToken,
						Progress:      float64(attempt + 1),
						Total:         float64(pollConfig.MaxAttempts),
						Message:       fmt.Sprintf("Waiting for Copilot to create PR... (attempt %d/%d)", attempt+1, pollConfig.MaxAttempts),
					})
				}

				pr, err := findLinkedCopilotPR(ctx, client, params.Owner, params.Repo, int(params.IssueNumber), assignmentTime)
				if err != nil {
					// Polling errors are non-fatal, continue to next attempt
					continue
				}
				if pr != nil {
					linkedPR = pr
					break
				}
			}

			// Build the result
			result := map[string]any{
				"message":      "successfully assigned copilot to issue",
				"issue_number": int(updateIssueMutation.UpdateIssue.Issue.Number),
				"issue_url":    string(updateIssueMutation.UpdateIssue.Issue.URL),
				"owner":        params.Owner,
				"repo":         params.Repo,
			}

			// Add PR info if found during polling
			if linkedPR != nil {
				result["pull_request"] = map[string]any{
					"number": linkedPR.Number,
					"url":    linkedPR.URL,
					"title":  linkedPR.Title,
					"state":  linkedPR.State,
				}
				result["message"] = "successfully assigned copilot to issue - pull request created"
			} else {
				result["message"] = "successfully assigned copilot to issue - pull request pending"
				result["note"] = "The pull request may still be in progress. Once created, the PR number can be used to check job status, or check the issue timeline for updates."
			}

			r, err := json.Marshal(result)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to marshal response: %s", err)), nil, nil
			}

			return utils.NewToolResultText(string(r)), result, nil
		})
}

type ReplaceActorsForAssignableInput struct {
	AssignableID githubv4.ID   `json:"assignableId"`
	ActorIDs     []githubv4.ID `json:"actorIds"`
}

// AgentAssignmentInput represents the input for assigning an agent to an issue.
type AgentAssignmentInput struct {
	BaseRef            *githubv4.String `json:"baseRef,omitempty"`
	CustomAgent        *githubv4.String `json:"customAgent,omitempty"`
	CustomInstructions *githubv4.String `json:"customInstructions,omitempty"`
	TargetRepositoryID githubv4.ID      `json:"targetRepositoryId"`
}

// UpdateIssueInput represents the input for updating an issue with agent assignment.
type UpdateIssueInput struct {
	ID              githubv4.ID           `json:"id"`
	AssigneeIDs     []githubv4.ID         `json:"assigneeIds"`
	AgentAssignment *AgentAssignmentInput `json:"agentAssignment,omitempty"`
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
		ToolsetMetadataCopilot,
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

func AssignCodingAgentPrompt(t translations.TranslationHelperFunc) inventory.ServerPrompt {
	return inventory.NewServerPrompt(
		ToolsetMetadataIssues,
		mcp.Prompt{
			Name:        "AssignCodingAgent",
			Description: t("PROMPT_ASSIGN_CODING_AGENT_DESCRIPTION", "Assign GitHub Coding Agent to multiple tasks in a GitHub repository."),
			Arguments: []*mcp.PromptArgument{
				{
					Name:        "repo",
					Description: "The repository to assign tasks in (owner/repo).",
					Required:    true,
				},
			},
		},
		func(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			repo := request.Params.Arguments["repo"]

			messages := []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "You are a personal assistant for GitHub the Copilot GitHub Coding Agent. Your task is to help the user assign tasks to the Coding Agent based on their open GitHub issues. You can use `assign_copilot_to_issue` tool to assign the Coding Agent to issues that are suitable for autonomous work, and `search_issues` tool to find issues that match the user's criteria. You can also use `list_issues` to get a list of issues in the repository.",
					},
				},
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: fmt.Sprintf("Please go and get a list of the most recent 10 issues from the %s GitHub repository", repo),
					},
				},
				{
					Role: "assistant",
					Content: &mcp.TextContent{
						Text: fmt.Sprintf("Sure! I will get a list of the 10 most recent issues for the repo %s.", repo),
					},
				},
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "For each issue, please check if it is a clearly defined coding task with acceptance criteria and a low to medium complexity to identify issues that are suitable for an AI Coding Agent to work on. Then assign each of the identified issues to Copilot.",
					},
				},
				{
					Role: "assistant",
					Content: &mcp.TextContent{
						Text: "Certainly! Let me carefully check which ones are clearly scoped issues that are good to assign to the coding agent, and I will summarize and assign them now.",
					},
				},
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "Great, if you are unsure if an issue is good to assign, ask me first, rather than assigning copilot. If you are certain the issue is clear and suitable you can assign it to Copilot without asking.",
					},
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		},
	)
}
