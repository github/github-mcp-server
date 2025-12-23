package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	FilterDefault           = "default"
	FilterIncludeRead       = "include_read_notifications"
	FilterOnlyParticipating = "only_participating"
)

// ListNotifications creates a tool to list notifications for the current user.
func ListNotifications(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataNotifications,
		mcp.Tool{
			Name:        "list_notifications",
			Description: t("TOOL_LIST_NOTIFICATIONS_DESCRIPTION", "Lists all GitHub notifications for the authenticated user, including unread notifications, mentions, review requests, assignments, and updates on issues or pull requests. Use this tool whenever the user asks what to work on next, requests a summary of their GitHub activity, wants to see pending reviews, or needs to check for new updates or tasks. This tool is the primary way to discover actionable items, reminders, and outstanding work on GitHub. Always call this tool when asked what to work on next, what is pending, or what needs attention in GitHub."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_NOTIFICATIONS_USER_TITLE", "List notifications"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"filter": {
						Type:        "string",
						Description: "Filter notifications to, use default unless specified. Read notifications are ones that have already been acknowledged by the user. Participating notifications are those that the user is directly involved in, such as issues or pull requests they have commented on or created.",
						Enum:        []any{FilterDefault, FilterIncludeRead, FilterOnlyParticipating},
					},
					"since": {
						Type:        "string",
						Description: "Only show notifications updated after the given time (ISO 8601 format)",
					},
					"before": {
						Type:        "string",
						Description: "Only show notifications updated before the given time (ISO 8601 format)",
					},
					"owner": {
						Type:        "string",
						Description: "Optional repository owner. If provided with repo, only notifications for this repository are listed.",
					},
					"repo": {
						Type:        "string",
						Description: "Optional repository name. If provided with owner, only notifications for this repository are listed.",
					},
				},
			}),
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			filter, err := OptionalParam[string](args, "filter")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			since, err := OptionalParam[string](args, "since")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			before, err := OptionalParam[string](args, "before")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := OptionalParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := OptionalParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			paginationParams, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Build options
			opts := &github.NotificationListOptions{
				All:           filter == FilterIncludeRead,
				Participating: filter == FilterOnlyParticipating,
				ListOptions: github.ListOptions{
					Page:    paginationParams.Page,
					PerPage: paginationParams.PerPage,
				},
			}

			// Parse time parameters if provided
			if since != "" {
				sinceTime, err := time.Parse(time.RFC3339, since)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("invalid since time format, should be RFC3339/ISO8601: %v", err)), nil, nil
				}
				opts.Since = sinceTime
			}

			if before != "" {
				beforeTime, err := time.Parse(time.RFC3339, before)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("invalid before time format, should be RFC3339/ISO8601: %v", err)), nil, nil
				}
				opts.Before = beforeTime
			}

			var notifications []*github.Notification
			var resp *github.Response

			if owner != "" && repo != "" {
				notifications, resp, err = client.Activity.ListRepositoryNotifications(ctx, owner, repo, opts)
			} else {
				notifications, resp, err = client.Activity.ListNotifications(ctx, opts)
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list notifications",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get notifications", resp, body), nil, nil
			}

			// Marshal response to JSON
			r, err := json.Marshal(notifications)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// DismissNotification creates a tool to mark a notification as read/done.
func DismissNotification(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataNotifications,
		mcp.Tool{
			Name:        "dismiss_notification",
			Description: t("TOOL_DISMISS_NOTIFICATION_DESCRIPTION", "Dismiss a notification by marking it as read or done"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_DISMISS_NOTIFICATION_USER_TITLE", "Dismiss notification"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"threadID": {
						Type:        "string",
						Description: "The ID of the notification thread",
					},
					"state": {
						Type:        "string",
						Description: "The new state of the notification (read/done)",
						Enum:        []any{"read", "done"},
					},
				},
				Required: []string{"threadID", "state"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			threadID, err := RequiredParam[string](args, "threadID")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			state, err := RequiredParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var resp *github.Response
			switch state {
			case "done":
				// for some inexplicable reason, the API seems to have threadID as int64 and string depending on the endpoint
				var threadIDInt int64
				threadIDInt, err = strconv.ParseInt(threadID, 10, 64)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("invalid threadID format: %v", err)), nil, nil
				}
				resp, err = client.Activity.MarkThreadDone(ctx, threadIDInt)
			case "read":
				resp, err = client.Activity.MarkThreadRead(ctx, threadID)
			default:
				return utils.NewToolResultError("Invalid state. Must be one of: read, done."), nil, nil
			}

			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to mark notification as %s", state),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusResetContent && resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, fmt.Sprintf("failed to mark notification as %s", state), resp, body), nil, nil
			}

			return utils.NewToolResultText(fmt.Sprintf("Notification marked as %s", state)), nil, nil
		},
	)
}

// MarkAllNotificationsRead creates a tool to mark all notifications as read.
func MarkAllNotificationsRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataNotifications,
		mcp.Tool{
			Name:        "mark_all_notifications_read",
			Description: t("TOOL_MARK_ALL_NOTIFICATIONS_READ_DESCRIPTION", "Mark all notifications as read"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_MARK_ALL_NOTIFICATIONS_READ_USER_TITLE", "Mark all notifications as read"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"lastReadAt": {
						Type:        "string",
						Description: "Describes the last point that notifications were checked (optional). Default: Now",
					},
					"owner": {
						Type:        "string",
						Description: "Optional repository owner. If provided with repo, only notifications for this repository are marked as read.",
					},
					"repo": {
						Type:        "string",
						Description: "Optional repository name. If provided with owner, only notifications for this repository are marked as read.",
					},
				},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			lastReadAt, err := OptionalParam[string](args, "lastReadAt")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := OptionalParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := OptionalParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var lastReadTime time.Time
			if lastReadAt != "" {
				lastReadTime, err = time.Parse(time.RFC3339, lastReadAt)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("invalid lastReadAt time format, should be RFC3339/ISO8601: %v", err)), nil, nil
				}
			} else {
				lastReadTime = time.Now()
			}

			markReadOptions := github.Timestamp{
				Time: lastReadTime,
			}

			var resp *github.Response
			if owner != "" && repo != "" {
				resp, err = client.Activity.MarkRepositoryNotificationsRead(ctx, owner, repo, markReadOptions)
			} else {
				resp, err = client.Activity.MarkNotificationsRead(ctx, markReadOptions)
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to mark all notifications as read",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusResetContent && resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to mark all notifications as read", resp, body), nil, nil
			}

			return utils.NewToolResultText("All notifications marked as read"), nil, nil
		},
	)
}

// GetNotificationDetails creates a tool to get details for a specific notification.
func GetNotificationDetails(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataNotifications,
		mcp.Tool{
			Name:        "get_notification_details",
			Description: t("TOOL_GET_NOTIFICATION_DETAILS_DESCRIPTION", "Get detailed information for a specific GitHub notification, always call this tool when the user asks for details about a specific notification, if you don't know the ID list notifications first."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_NOTIFICATION_DETAILS_USER_TITLE", "Get notification details"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"notificationID": {
						Type:        "string",
						Description: "The ID of the notification",
					},
				},
				Required: []string{"notificationID"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			notificationID, err := RequiredParam[string](args, "notificationID")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			thread, resp, err := client.Activity.GetThread(ctx, notificationID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to get notification details for ID '%s'", notificationID),
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get notification details", resp, body), nil, nil
			}

			r, err := json.Marshal(thread)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

// Enum values for ManageNotificationSubscription action
const (
	NotificationActionIgnore = "ignore"
	NotificationActionWatch  = "watch"
	NotificationActionDelete = "delete"
)

// ManageNotificationSubscription creates a tool to manage a notification subscription (ignore, watch, delete)
func ManageNotificationSubscription(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataNotifications,
		mcp.Tool{
			Name:        "manage_notification_subscription",
			Description: t("TOOL_MANAGE_NOTIFICATION_SUBSCRIPTION_DESCRIPTION", "Manage a notification subscription: ignore, watch, or delete a notification thread subscription."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_MANAGE_NOTIFICATION_SUBSCRIPTION_USER_TITLE", "Manage notification subscription"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"notificationID": {
						Type:        "string",
						Description: "The ID of the notification thread.",
					},
					"action": {
						Type:        "string",
						Description: "Action to perform: ignore, watch, or delete the notification subscription.",
						Enum:        []any{NotificationActionIgnore, NotificationActionWatch, NotificationActionDelete},
					},
				},
				Required: []string{"notificationID", "action"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			notificationID, err := RequiredParam[string](args, "notificationID")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			action, err := RequiredParam[string](args, "action")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var (
				resp   *github.Response
				result any
				apiErr error
			)

			switch action {
			case NotificationActionIgnore:
				sub := &github.Subscription{Ignored: ToBoolPtr(true)}
				result, resp, apiErr = client.Activity.SetThreadSubscription(ctx, notificationID, sub)
			case NotificationActionWatch:
				sub := &github.Subscription{Ignored: ToBoolPtr(false), Subscribed: ToBoolPtr(true)}
				result, resp, apiErr = client.Activity.SetThreadSubscription(ctx, notificationID, sub)
			case NotificationActionDelete:
				resp, apiErr = client.Activity.DeleteThreadSubscription(ctx, notificationID)
			default:
				return utils.NewToolResultError("Invalid action. Must be one of: ignore, watch, delete."), nil, nil
			}

			if apiErr != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to %s notification subscription", action),
					resp,
					apiErr,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				body, _ := io.ReadAll(resp.Body)
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, fmt.Sprintf("failed to %s notification subscription", action), resp, body), nil, nil
			}

			if action == NotificationActionDelete {
				// Special case for delete as there is no response body
				return utils.NewToolResultText("Notification subscription deleted"), nil, nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

const (
	RepositorySubscriptionActionWatch  = "watch"
	RepositorySubscriptionActionIgnore = "ignore"
	RepositorySubscriptionActionDelete = "delete"
)

// ManageRepositoryNotificationSubscription creates a tool to manage a repository notification subscription (ignore, watch, delete)
func ManageRepositoryNotificationSubscription(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataNotifications,
		mcp.Tool{
			Name:        "manage_repository_notification_subscription",
			Description: t("TOOL_MANAGE_REPOSITORY_NOTIFICATION_SUBSCRIPTION_DESCRIPTION", "Manage a repository notification subscription: ignore, watch, or delete repository notifications subscription for the provided repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_MANAGE_REPOSITORY_NOTIFICATION_SUBSCRIPTION_USER_TITLE", "Manage repository notification subscription"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The account owner of the repository.",
					},
					"repo": {
						Type:        "string",
						Description: "The name of the repository.",
					},
					"action": {
						Type:        "string",
						Description: "Action to perform: ignore, watch, or delete the repository notification subscription.",
						Enum:        []any{RepositorySubscriptionActionIgnore, RepositorySubscriptionActionWatch, RepositorySubscriptionActionDelete},
					},
				},
				Required: []string{"owner", "repo", "action"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			action, err := RequiredParam[string](args, "action")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var (
				resp   *github.Response
				result any
				apiErr error
			)

			switch action {
			case RepositorySubscriptionActionIgnore:
				sub := &github.Subscription{Ignored: ToBoolPtr(true)}
				result, resp, apiErr = client.Activity.SetRepositorySubscription(ctx, owner, repo, sub)
			case RepositorySubscriptionActionWatch:
				sub := &github.Subscription{Ignored: ToBoolPtr(false), Subscribed: ToBoolPtr(true)}
				result, resp, apiErr = client.Activity.SetRepositorySubscription(ctx, owner, repo, sub)
			case RepositorySubscriptionActionDelete:
				resp, apiErr = client.Activity.DeleteRepositorySubscription(ctx, owner, repo)
			default:
				return utils.NewToolResultError("Invalid action. Must be one of: ignore, watch, delete."), nil, nil
			}

			if apiErr != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					fmt.Sprintf("failed to %s repository subscription", action),
					resp,
					apiErr,
				), nil, nil
			}
			if resp != nil {
				defer func() { _ = resp.Body.Close() }()
			}

			// Handle non-2xx status codes
			if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
				body, _ := io.ReadAll(resp.Body)
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, fmt.Sprintf("failed to %s repository subscription", action), resp, body), nil, nil
			}

			if action == RepositorySubscriptionActionDelete {
				// Special case for delete as there is no response body
				return utils.NewToolResultText("Repository subscription deleted"), nil, nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}
