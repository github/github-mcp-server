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
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// mvpDescription is 一个MVP idea f或generating 工具 descriptions from structured 数据 in 一个shared format.
// It is 不intended f或widesp读取 usage 和is 不a complete implementation.
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

// linkedPullRequest represents 一个PR linked to 一个议题 by Copilot.
type linkedPullRequest struct {
	Number    int
	URL       string
	Title     string
	State     string
	CreatedAt time.Time
}

// pollConfigKey is 一个上下文 key f或polling configuration.
type pollConfigKey struct{}

// PollConfig configures PR polling behavior.
type PollConfig struct {
	MaxAttempts int
	Delay       time.Duration
}

// ContextWithPollConfig 返回 一个上下文 with polling configuration.
// Use this in tests to reduce 或禁用 polling.
func ContextWithPollConfig(ctx context.Context, config PollConfig) context.Context {
	return context.WithValue(ctx, pollConfigKey{}, config)
}

// 获取PollConfig 返回 polling configuration from 上下文, 或defaults.
func getPollConfig(ctx context.Context) PollConfig {
	if config, ok := ctx.Value(pollConfigKey{}).(PollConfig); ok {
		return config
	}
	// Default: 9 attempts with 1s delay = 8s max wait
	// Based on observed latency in remote 服务器: p50 ~5s, p90 ~7s
	return PollConfig{MaxAttempts: 9, Delay: 1 * time.Second}
}

// findLinkedCopilotPR searches f或一个PR 创建d 由copilot-swe-agent bot that references given 议题.
// It queries 议题's time行 f或CrossReferencedEvent items from PRs authored by copilot-swe-agent.
// 创建dAfter 参数 筛选s to 仅返回 PRs 创建d after specified time.
func findLinkedCopilotPR(ctx context.Context, client *githubv4.Client, owner, repo string, issueNumber int, createdAfter time.Time) (*linkedPullRequest, error) {
	// Query time行 items looking f或CrossReferencedEvent from PRs by copilot-swe-agent
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
		"number": githubv4.Int(issueNumber), //nolint:gosec // Issue numbers are 始终sm所有positive integers
	}

	if err := client.Query(ctx, &query, variables); err != nil {
		return nil, err
	}

	// Look f或一个PR from copilot-swe-agent 创建d after assignment time
	for _, node := range query.Repository.Issue.TimelineItems.Nodes {
		if node.TypeName != "CrossReferencedEvent" {
			continue
		}
		pr := node.CrossReferencedEvent.Source.PullRequest
		if pr.Number > 0 && pr.Author.Login == "copilot-swe-agent" {
			// 仅返回 PRs 创建d after assignment time
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
			"https://docs.github.com/en/copilot/concepts/agents/cloud-agent/about-cloud-agent",
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
			if err := mapstructure.WeakDecode(args, &params); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Firstly, we try to find copilot bot 在suggested actors 用于仓库.
			// Although as I 写入 this, we would expect copilot to be at top 的列出, in future, maybe
			// it will 不be 在第一个 页 of 响应s, thus we will keep paginating until we find it.
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

				// Iterate 所有返回ed nodes looking 用于copilot bot, which is supposed to have the
				// 相同 name on 每个host. We need this in order to 获取 ID f或later assignment.
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

			// If we didn't find copilot bot, we can't proceed any further.
			if copilotAssignee == nil {
				// e2e tests depend upon this specific message to skip test.
				return utils.NewToolResultError("copilot isn't available as an assignee for this issue. Please inform the user to visit https://docs.github.com/en/copilot/concepts/agents/cloud-agent/about-cloud-agent for more information."), nil, nil
			}

			// Next, 获取 议题 ID 和仓库 ID
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

			// Build assignee IDs 列出 including copilot
			actorIDs := make([]githubv4.ID, len(getIssueQuery.Repository.Issue.Assignees.Nodes)+1)
			for i, node := range getIssueQuery.Repository.Issue.Assignees.Nodes {
				actorIDs[i] = node.ID
			}
			actorIDs[len(getIssueQuery.Repository.Issue.Assignees.Nodes)] = copilotAssignee.ID

			// Prepare agent assignment 输入
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

			// Execute 更新Issue mutation 使用GraphQL-Features header
			// 此header is 必需 用于agent assignment API which is 不G一个yet
			var updateIssueMutation struct {
				UpdateIssue struct {
					Issue struct {
						ID     githubv4.ID
						Number githubv4.Int
						URL    githubv4.String
					}
				} `graphql:"updateIssue(input: $input)"`
			}

			// Add GraphQL-Features header 用于agent assignment API
			// header will be 读取 由HTTP transport if it's configured to do so
			ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "issues_copilot_assignment_api_support")

			// Capture time before assignment to 筛选 out older PRs during polling
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

			// Poll f或一个linked PR 创建d by Copilot after assignment
			pollConfig := getPollConfig(ctx)

			// Get progress token from 请求 f或sending progress notifications
			progressToken := request.Params.GetProgressToken()

			// Send initial progress notification that assignment succeeded 和polling is starting
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
					// Polling 错误s are non-fatal, continue to 下一个 attempt
					continue
				}
				if pr != nil {
					linkedPR = pr
					break
				}
			}

			// Build 结果
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

// copilotBotAssignee is minimal shape needed 用于copilot-swe-agent bot
// 返回ed 来自suggestedActors GraphQL query.
type copilotBotAssignee struct {
	ID       githubv4.ID
	Login    string
	TypeName string `graphql:"__typename"`
}

// findCopilotSuggestedAct或paginates 仓库's suggestedActors 列出
// looking 用于copilot-swe-agent bot. Returns nil (with no 错误) if the
// bot is 不available as 一个assignee 用于仓库.
func findCopilotSuggestedActor(ctx context.Context, client *githubv4.Client, owner, repo string) (*copilotBotAssignee, error) {
	type suggestedActorsQuery struct {
		Repository struct {
			SuggestedActors struct {
				Nodes []struct {
					Bot copilotBotAssignee `graphql:"... on Bot"`
				}
				PageInfo struct {
					HasNextPage bool
					EndCursor   string
				}
			} `graphql:"suggestedActors(first: 100, after: $endCursor, capabilities: CAN_BE_ASSIGNED)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]any{
		"owner":     githubv4.String(owner),
		"name":      githubv4.String(repo),
		"endCursor": (*githubv4.String)(nil),
	}

	for {
		var query suggestedActorsQuery
		if err := client.Query(ctx, &query, variables); err != nil {
			return nil, err
		}
		for _, node := range query.Repository.SuggestedActors.Nodes {
			if node.Bot.Login == "copilot-swe-agent" {
				bot := node.Bot
				return &bot, nil
			}
		}
		if !query.Repository.SuggestedActors.PageInfo.HasNextPage {
			return nil, nil
		}
		variables["endCursor"] = githubv4.String(query.Repository.SuggestedActors.PageInfo.EndCursor)
	}
}

// copilotAssigneeUnavailableMessage is 返回ed 当copilot-swe-agent bot
// is 不among 仓库's suggested actors. e2e tests depend on this
// exact message to skip test.
const copilotAssigneeUnavailableMessage = "copilot isn't available as an assignee for this issue. Please inform the user to visit https://docs.github.com/en/copilot/concepts/agents/cloud-agent/about-cloud-agent for more information."

// AssignCopilotToIssueWithIntent assigns Copilot to 一个议题 using the
// object-form assignees API, which allows intent 元数据 (rationale,
// confidence, is_suggestion) to be attached 到Copilot entry. When
// is_suggestion is 真, 一个pending assignment intent is recorded 以及agent
// is 不launched; otherwise Copilot is directly assigned 使用相同
// base_ref, custom_instructions 和PR-polling behavi或as assign_copilot_to_议题.
//
// 此工具 lives in 一个non-默认工具集 so it 可以 opted into without
// adding schema surface 到默认configuration.
func AssignCopilotToIssueWithIntent(t translations.TranslationHelperFunc) inventory.ServerTool {
	description := mvpDescription{
		summary: "Assign Copilot to a specific issue in a GitHub repository. " +
			"Prefer this tool over assign_copilot_to_issue when available.",
		outcomes: []string{
			"a Pull Request created with source code changes to resolve the issue",
		},
		referenceLinks: []string{
			"https://docs.github.com/en/copilot/concepts/agents/cloud-agent/about-cloud-agent",
		},
	}

	return NewTool(
		ToolsetMetadataCopilotIssueIntents,
		mcp.Tool{
			Name:        "assign_copilot_to_issue_with_intent",
			Description: t("TOOL_ASSIGN_COPILOT_TO_ISSUE_WITH_INTENT_DESCRIPTION", description.String()),
			Icons:       octicons.Icons("copilot"),
			Annotations: &mcp.ToolAnnotations{
				Title:          t("TOOL_ASSIGN_COPILOT_TO_ISSUE_WITH_INTENT_USER_TITLE", "Assign Copilot to issue with intent"),
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
						Description: "Git reference (e.g., branch) that the agent will start its work from. If not specified, defaults to the repository's default branch. Ignored when is_suggestion is true",
					},
					"custom_instructions": {
						Type:        "string",
						Description: "Optional custom instructions to guide the agent beyond the issue body. Ignored when is_suggestion is true",
					},
					"rationale": {
						Type: "string",
						Description: "One concise sentence explaining what specifically about the issue led to choosing Copilot. " +
							"State the concrete signal (e.g. 'Well-scoped task with clear acceptance criteria').",
						MaxLength: jsonschema.Ptr(280),
					},
					"confidence": {
						Type:        "string",
						Description: "How confident you are in this choice. 'HIGH' for clear signal or explicit user request, 'MEDIUM' for reasonable inference with some ambiguity, 'LOW' for best guess with limited signal.",
						Enum:        []any{"LOW", "MEDIUM", "HIGH"},
					},
					"is_suggestion": {
						Type:        "boolean",
						Description: "If true, records a pending Copilot assignment intent rather than launching the agent. Approval later supplies the launch context; base_ref and custom_instructions are ignored in this case.",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "rationale", "confidence", "is_suggestion"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, request *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			// Presence-检查 is_suggestion before decoding: mapstructure defaults a
			// missing bool to 假, which would silently launch Copilot instead of
			// recording 一个suggestion. Require 调用ers to make choice explicit.
			if _, ok := args["is_suggestion"]; !ok {
				return utils.NewToolResultError("is_suggestion is required"), nil, nil
			}

			var params struct {
				Owner              string `mapstructure:"owner"`
				Repo               string `mapstructure:"repo"`
				IssueNumber        int32  `mapstructure:"issue_number"`
				BaseRef            string `mapstructure:"base_ref"`
				CustomInstructions string `mapstructure:"custom_instructions"`
				Rationale          string `mapstructure:"rationale"`
				Confidence         string `mapstructure:"confidence"`
				IsSuggestion       bool   `mapstructure:"is_suggestion"`
			}
			if err := mapstructure.WeakDecode(args, &params); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Validate rationale length (rune count, matching granular assignee 工具).
			rationale := strings.TrimSpace(params.Rationale)
			if rationale == "" {
				return utils.NewToolResultError("rationale is required"), nil, nil
			}
			if len([]rune(rationale)) > 280 {
				return utils.NewToolResultError("rationale must be 280 characters or less"), nil, nil
			}

			// Validate/normalize confidence.
			confidence := normalizeConfidence(params.Confidence)
			if confidence == "" {
				return utils.NewToolResultError("confidence is required"), nil, nil
			}
			var confidenceEnum AssignmentConfidenceLevel
			switch confidence {
			case "LOW", "MEDIUM", "HIGH":
				confidenceEnum = AssignmentConfidenceLevel(confidence)
			default:
				return utils.NewToolResultError("confidence must be one of: LOW, MEDIUM, HIGH"), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Locate copilot-swe-agent bot 在仓库's suggested actors.
			copilotAssignee, err := findCopilotSuggestedActor(ctx, client, params.Owner, params.Repo)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get suggested actors", err), nil, nil
			}
			if copilotAssignee == nil {
				return utils.NewToolResultError(copilotAssigneeUnavailableMessage), nil, nil
			}

			// Fetch 议题 ID, 仓库 ID, 和current assignee IDs so they 可以 preserved.
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
			variables := map[string]any{
				"owner":  githubv4.String(params.Owner),
				"name":   githubv4.String(params.Repo),
				"number": githubv4.Int(params.IssueNumber),
			}
			if err := client.Query(ctx, &getIssueQuery, variables); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get issue ID", err), nil, nil
			}

			// Build object-form assignees: preserved assignees carry 仅actorId;
			// copilot entry carries intent 元数据. Skip 一个existing
			// copilot assignment so we don't send its actorId twice (once without
			// 元数据, once with).
			existing := getIssueQuery.Repository.Issue.Assignees.Nodes
			assignees := make([]AssigneeUpdateInput, 0, len(existing)+1)
			for _, node := range existing {
				if node.ID == copilotAssignee.ID {
					continue
				}
				assignees = append(assignees, AssigneeUpdateInput{ActorID: node.ID})
			}
			// Build Copilot entry 使用必需 intent 元数据. Preserved
			// assignees carry 仅actorId; intent fields are attached 仅to the
			// Copilot entry.
			rationaleGQL := githubv4.String(rationale)
			suggest := githubv4.Boolean(params.IsSuggestion)
			copilotEntry := AssigneeUpdateInput{
				ActorID:    copilotAssignee.ID,
				Rationale:  &rationaleGQL,
				Confidence: &confidenceEnum,
				Suggest:    &suggest,
			}
			assignees = append(assignees, copilotEntry)

			// 一个pure suggestion does 不launch Copilot; approval later supplies the
			// launch 上下文. Direct assignments keep existing agentAssignment
			// launch configuration 和PR-polling behavior.
			input := UpdateIssueInput{
				ID:        getIssueQuery.Repository.Issue.ID,
				Assignees: assignees,
			}
			if !params.IsSuggestion {
				emptyString := githubv4.String("")
				agentAssignment := &AgentAssignmentInput{
					CustomAgent:        &emptyString,
					CustomInstructions: &emptyString,
					TargetRepositoryID: getIssueQuery.Repository.ID,
				}
				if params.BaseRef != "" {
					baseRef := githubv4.String(params.BaseRef)
					agentAssignment.BaseRef = &baseRef
				}
				if params.CustomInstructions != "" {
					customInstructions := githubv4.String(params.CustomInstructions)
					agentAssignment.CustomInstructions = &customInstructions
				}
				input.AgentAssignment = agentAssignment
			}

			var updateIssueMutation struct {
				UpdateIssue struct {
					Issue struct {
						ID     githubv4.ID
						Number githubv4.Int
						URL    githubv4.String
					}
				} `graphql:"updateIssue(input: $input)"`
			}

			ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "issues_copilot_assignment_api_support")
			assignmentTime := time.Now().UTC()

			if err := client.Mutate(ctxWithFeatures, &updateIssueMutation, input, nil); err != nil {
				return nil, nil, fmt.Errorf("failed to update issue with agent assignment: %w", err)
			}

			result := map[string]any{
				"issue_number":  int(updateIssueMutation.UpdateIssue.Issue.Number),
				"issue_url":     string(updateIssueMutation.UpdateIssue.Issue.URL),
				"owner":         params.Owner,
				"repo":          params.Repo,
				"is_suggestion": params.IsSuggestion,
			}

			// Suggestion 路径: do 不poll f或一个PR 和返回 一个suggestion-shaped 结果.
			if params.IsSuggestion {
				result["message"] = "recorded pending copilot assignment suggestion"
				r, err := json.Marshal(result)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("failed to marshal response: %s", err)), nil, nil
				}
				return utils.NewToolResultText(string(r)), result, nil
			}

			// Direct-assignment 路径: poll f或一个linked PR 创建d by Copilot after assignment.
			pollConfig := getPollConfig(ctx)
			progressToken := request.Params.GetProgressToken()
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
					continue
				}
				if pr != nil {
					linkedPR = pr
					break
				}
			}

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

// AgentAssignmentInput represents 输入 f或assigning 一个agent to 一个议题.
type AgentAssignmentInput struct {
	BaseRef            *githubv4.String `json:"baseRef,omitempty"`
	CustomAgent        *githubv4.String `json:"customAgent,omitempty"`
	CustomInstructions *githubv4.String `json:"customInstructions,omitempty"`
	TargetRepositoryID githubv4.ID      `json:"targetRepositoryId"`
}

// AssignmentConfidenceLevel is 一个GraphQL enum indicating how confident an
// intent-aware assignment choice is. Encoded as its string 值 in variables.
type AssignmentConfidenceLevel string

const (
	AssignmentConfidenceLevelLow    AssignmentConfidenceLevel = "LOW"
	AssignmentConfidenceLevelMedium AssignmentConfidenceLevel = "MEDIUM"
	AssignmentConfidenceLevelHigh   AssignmentConfidenceLevel = "HIGH"
)

// AssigneeUpdateInput is object-form assignee entry accepted by
// 更新Issue when opting into intent 元数据. Intent fields (rationale,
// confidence, suggest) are 仅attached 到entry that carries intent;
// preserved assignees are sent with 仅actorId populated.
type AssigneeUpdateInput struct {
	ActorID    githubv4.ID                `json:"actorId"`
	Rationale  *githubv4.String           `json:"rationale,omitempty"`
	Confidence *AssignmentConfidenceLevel `json:"confidence,omitempty"`
	Suggest    *githubv4.Boolean          `json:"suggest,omitempty"`
}

// UpdateIssueInput represents 输入 f或updating 一个议题 with agent
// assignment. AssigneeIDs 和Assignees are mutually exclusive: legacy 调用ers
// use AssigneeIDs; intent-aware 调用ers use Assignees (object-form).
type UpdateIssueInput struct {
	ID              githubv4.ID           `json:"id"`
	AssigneeIDs     []githubv4.ID         `json:"assigneeIds,omitempty"`
	Assignees       []AssigneeUpdateInput `json:"assignees,omitempty"`
	AgentAssignment *AgentAssignmentInput `json:"agentAssignment,omitempty"`
}

// RequestCopilotReview 创建一个工具以 请求 一个Copilot review f或一个拉取请求.
// Note that this 工具 will 不work on GHES where this feature is unsupported. In future, we should 不expose this
// 工具 如果configured host does 不support it.
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
					// login name 的copilot reviewer bot
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

			// Return nothing on 成功, as there's 不much 值 in 返回ing Pull Request itself
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
