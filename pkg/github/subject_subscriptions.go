package github

import (
	"context"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

type SubjectSubscriptionInput struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	SubjectType string `json:"subjectType"`
	Number      int32  `json:"number"`
}

type ManageSubjectSubscriptionInput struct {
	SubjectSubscriptionInput
	State string `json:"state"`
}

// SubjectSubscriptionOutput is the authenticated viewer's subscription, not public repository data.
type SubjectSubscriptionOutput struct {
	ID           string  `json:"id"`
	SubjectType  string  `json:"subject_type"`
	Number       int32   `json:"number"`
	URL          string  `json:"url"`
	State        *string `json:"state"`
	CanSubscribe bool    `json:"can_subscribe"`
}

func subjectSubscriptionInputSchema(write bool) *jsonschema.Schema {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {Type: "string", Description: "Repository owner"},
			"repo":  {Type: "string", Description: "Repository name"},
			"subjectType": {
				Type: "string", Description: "Type of the subject to subscribe to",
				Enum: []any{"issue", "pull_request", "discussion"},
			},
			"number": {
				Type: "integer", Description: "Issue, pull request, or discussion number",
				Minimum: new(float64(1)), Maximum: new(float64(2147483647)),
			},
		},
		Required: []string{"owner", "repo", "subjectType", "number"},
	}
	if write {
		schema.Properties["state"] = &jsonschema.Schema{
			Type:        "string",
			Description: "subscribed: receive all updates; unsubscribed: only participating or mentions; ignored: receive no notifications",
			Enum:        []any{"subscribed", "unsubscribed", "ignored"},
		}
		schema.Required = append(schema.Required, "state")
	}
	return schema
}

func GetSubjectSubscription(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[SubjectSubscriptionInput, *SubjectSubscriptionOutput](
		ToolsetMetadataNotifications,
		mcp.Tool{
			Name:        "get_subject_subscription",
			Description: t("TOOL_GET_SUBJECT_SUBSCRIPTION_DESCRIPTION", "Get the authenticated user's notification subscription state for a specific issue, pull request, or discussion by repository and number. Does not require an existing notification thread."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_SUBJECT_SUBSCRIPTION_USER_TITLE", "Get subject subscription"),
				ReadOnlyHint: true,
			},
			InputSchema: subjectSubscriptionInputSchema(false),
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input SubjectSubscriptionInput) (*mcp.CallToolResult, *SubjectSubscriptionOutput, error) {
			if err := validateSubjectSubscriptionInput(input); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}
			subject, err := querySubscriptionSubject(ctx, client, input)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get subject subscription", err), nil, nil
			}
			if subject.ID == nil || subject.ID == "" {
				return utils.NewToolResultError("subscription subject not found"), nil, nil
			}
			return subjectSubscriptionResult(ctx, deps, input, subject)
		},
	)
}

func ManageSubjectSubscription(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[ManageSubjectSubscriptionInput, *SubjectSubscriptionOutput](
		ToolsetMetadataNotifications,
		mcp.Tool{
			Name:        "manage_subject_subscription",
			Description: t("TOOL_MANAGE_SUBJECT_SUBSCRIPTION_DESCRIPTION", "Subscribe, unsubscribe, or ignore a specific issue, pull request, or discussion for the authenticated user by repository and number. Does not require an existing notification thread and does not watch the whole repository. Use get_subject_subscription to check the current state."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_MANAGE_SUBJECT_SUBSCRIPTION_USER_TITLE", "Manage subject subscription"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
				IdempotentHint:  true,
			},
			InputSchema: subjectSubscriptionInputSchema(true),
		},
		subjectSubscriptionWriteScopeAccess(),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input ManageSubjectSubscriptionInput) (*mcp.CallToolResult, *SubjectSubscriptionOutput, error) {
			if err := validateSubjectSubscriptionInput(input.SubjectSubscriptionInput); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			switch input.State {
			case "subscribed", "unsubscribed", "ignored":
			default:
				return utils.NewToolResultError("state must be one of: subscribed, unsubscribed, ignored"), nil, nil
			}
			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}
			subject, err := querySubscriptionSubject(ctx, client, input.SubjectSubscriptionInput)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get subject subscription", err), nil, nil
			}
			if subject.ID == nil || subject.ID == "" {
				return utils.NewToolResultError("subscription subject not found"), nil, nil
			}
			desiredState := githubv4.SubscriptionState(strings.ToUpper(input.State))
			if subject.ViewerSubscription != nil && *subject.ViewerSubscription == desiredState {
				return subjectSubscriptionResult(ctx, deps, input.SubjectSubscriptionInput, subject)
			}
			if !subject.ViewerCanSubscribe {
				return utils.NewToolResultError("the authenticated user cannot subscribe to this subject"), nil, nil
			}
			var mutation struct {
				UpdateSubscription struct {
					Subscribable struct {
						ID                 githubv4.ID
						ViewerSubscription *githubv4.SubscriptionState
						ViewerCanSubscribe githubv4.Boolean
					}
				} `graphql:"updateSubscription(input: $input)"`
			}
			err = client.Mutate(ctx, &mutation, githubv4.UpdateSubscriptionInput{
				SubscribableID: subject.ID,
				State:          desiredState,
			}, nil)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to update subject subscription", err), nil, nil
			}
			updated := mutation.UpdateSubscription.Subscribable
			if updated.ID != subject.ID || updated.ViewerSubscription == nil {
				return utils.NewToolResultError("GitHub did not return the requested subject subscription; check its state before retrying"), nil, nil
			}
			if *updated.ViewerSubscription != desiredState {
				return utils.NewToolResultError(fmt.Sprintf("GitHub returned subscription state %s instead of requested %s; use get_subject_subscription to check its state before retrying", strings.ToLower(string(*updated.ViewerSubscription)), input.State)), nil, nil
			}
			// Subscribable exposes no URL field. Retain the URL from the concrete subject lookup.
			subject.ViewerSubscription = updated.ViewerSubscription
			subject.ViewerCanSubscribe = updated.ViewerCanSubscribe
			return subjectSubscriptionResult(ctx, deps, input.SubjectSubscriptionInput, subject)
		},
	)
}

func subjectSubscriptionWriteScopeAccess() inventory.ScopeAccess {
	access := scopes.RequireAll(scopes.Repo, scopes.Notifications)
	// Notifications-only PATs can still target public subjects. OAuth must be
	// able to request repository access for the preliminary private subject lookup.
	access.Visible = scopes.RequireAll(scopes.Notifications).Visible
	access.Challenge = func(_ map[string]any, activeScopes []string) []string {
		return append(scopes.ChallengeAll(activeScopes, scopes.Repo), scopes.ChallengeAll(activeScopes, scopes.Notifications)...)
	}
	return access
}

func validateSubjectSubscriptionInput(input SubjectSubscriptionInput) error {
	if strings.TrimSpace(input.Owner) == "" {
		return fmt.Errorf("missing required parameter: owner")
	}
	if strings.TrimSpace(input.Repo) == "" {
		return fmt.Errorf("missing required parameter: repo")
	}
	if input.Number < 1 {
		return fmt.Errorf("number must be between 1 and 2147483647")
	}
	switch input.SubjectType {
	case "issue", "pull_request", "discussion":
		return nil
	default:
		return fmt.Errorf("subjectType must be one of: issue, pull_request, discussion")
	}
}

type subscriptionSubject struct {
	ID                 githubv4.ID
	URL                githubv4.String `graphql:"url"`
	ViewerSubscription *githubv4.SubscriptionState
	ViewerCanSubscribe githubv4.Boolean
}

func querySubscriptionSubject(ctx context.Context, client *githubv4.Client, input SubjectSubscriptionInput) (subscriptionSubject, error) {
	vars := map[string]any{
		"owner":  githubv4.String(input.Owner),
		"repo":   githubv4.String(input.Repo),
		"number": githubv4.Int(input.Number),
	}
	switch input.SubjectType {
	case "issue":
		var query struct {
			Repository struct {
				Subject subscriptionSubject `graphql:"issue(number: $number)"`
			} `graphql:"repository(owner: $owner, name: $repo)"`
		}
		err := client.Query(ctx, &query, vars)
		return query.Repository.Subject, err
	case "pull_request":
		var query struct {
			Repository struct {
				Subject subscriptionSubject `graphql:"pullRequest(number: $number)"`
			} `graphql:"repository(owner: $owner, name: $repo)"`
		}
		err := client.Query(ctx, &query, vars)
		return query.Repository.Subject, err
	default: // subjectType has already been validated.
		var query struct {
			Repository struct {
				Subject subscriptionSubject `graphql:"discussion(number: $number)"`
			} `graphql:"repository(owner: $owner, name: $repo)"`
		}
		err := client.Query(ctx, &query, vars)
		return query.Repository.Subject, err
	}
}

func subjectSubscriptionResult(ctx context.Context, deps ToolDependencies, input SubjectSubscriptionInput, subject subscriptionSubject) (*mcp.CallToolResult, *SubjectSubscriptionOutput, error) {
	var state *string
	if subject.ViewerSubscription != nil {
		normalized := strings.ToLower(string(*subject.ViewerSubscription))
		state = &normalized
	}
	output := &SubjectSubscriptionOutput{
		ID: fmt.Sprint(subject.ID), SubjectType: input.SubjectType,
		Number: input.Number, URL: string(subject.URL),
		State: state, CanSubscribe: bool(subject.ViewerCanSubscribe),
	}
	// A viewer's subscription state is private even when its subject is public.
	return attachStaticIFCLabel(ctx, deps, MarshalledTextResult(output), ifc.PrivateTrusted()), output, nil
}
