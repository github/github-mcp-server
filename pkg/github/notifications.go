package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// getNotifications creates a tool to list notifications for the current user.
func GetNotifications(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_notifications",
			mcp.WithDescription(t("TOOL_GET_NOTIFICATIONS_DESCRIPTION", "Get notifications for the authenticated GitHub user")),
			mcp.WithBoolean("all",
				mcp.Description("If true, show notifications marked as read. Default: false"),
			),
			mcp.WithBoolean("participating",
				mcp.Description("If true, only shows notifications in which the user is directly participating or mentioned. Default: false"),
			),
			mcp.WithString("since",
				mcp.Description("Only show notifications updated after the given time (ISO 8601 format)"),
			),
			mcp.WithString("before",
				mcp.Description("Only show notifications updated before the given time (ISO 8601 format)"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Results per page (max 100). Default: 30"),
			),
			mcp.WithNumber("page",
				mcp.Description("Page number of the results to fetch. Default: 1"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Extract optional parameters with defaults
			all, err := OptionalBoolParamWithDefault(request, "all", false)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			participating, err := OptionalBoolParamWithDefault(request, "participating", false)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			since, err := OptionalStringParamWithDefault(request, "since", "")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			before, err := OptionalStringParam(request, "before")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			perPage, err := OptionalIntParamWithDefault(request, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			page, err := OptionalIntParamWithDefault(request, "page", 1)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Build options
			opts := &github.NotificationListOptions{
				All:           all,
				Participating: participating,
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: perPage,
				},
			}

			// Parse time parameters if provided
			if since != "" {
				sinceTime, err := time.Parse(time.RFC3339, since)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid since time format, should be RFC3339/ISO8601: %v", err)), nil
				}
				opts.Since = sinceTime
			}

			if before != "" {
				beforeTime, err := time.Parse(time.RFC3339, before)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid before time format, should be RFC3339/ISO8601: %v", err)), nil
				}
				opts.Before = beforeTime
			}

			// Call GitHub API
			notifications, resp, err := client.Activity.ListNotifications(ctx, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to get notifications: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get notifications: %s", string(body))), nil
			}

			// Marshal response to JSON
			r, err := json.Marshal(notifications)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// markNotificationRead creates a tool to mark a notification as read.
func MarkNotificationRead(getclient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("mark_notification_read",
			mcp.WithDescription(t("TOOL_MARK_NOTIFICATION_READ_DESCRIPTION", "Mark a notification as read")),
			mcp.WithString("threadID",
				mcp.Required(),
				mcp.Description("The ID of the notification thread"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getclient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			threadID, err := requiredParam[string](request, "threadID")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			resp, err := client.Activity.MarkThreadRead(ctx, threadID)
			if err != nil {
				return nil, fmt.Errorf("failed to mark notification as read: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusResetContent && resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to mark notification as read: %s", string(body))), nil
			}

			return mcp.NewToolResultText("Notification marked as read"), nil
		}
}

// MarkAllNotificationsRead creates a tool to mark all notifications as read.
func MarkAllNotificationsRead(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("mark_all_notifications_read",
			mcp.WithDescription(t("TOOL_MARK_ALL_NOTIFICATIONS_READ_DESCRIPTION", "Mark all notifications as read")),
			mcp.WithString("lastReadAt",
				mcp.Description("Describes the last point that notifications were checked (optional). Default: Now"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			lastReadAt, err := OptionalStringParam(request, "lastReadAt")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var markReadOptions github.Timestamp
			if lastReadAt != "" {
				lastReadTime, err := time.Parse(time.RFC3339, lastReadAt)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid lastReadAt time format, should be RFC3339/ISO8601: %v", err)), nil
				}
				markReadOptions = github.Timestamp{
					Time: lastReadTime,
				}
			}

			resp, err := client.Activity.MarkNotificationsRead(ctx, markReadOptions)
			if err != nil {
				return nil, fmt.Errorf("failed to mark all notifications as read: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusResetContent && resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to mark all notifications as read: %s", string(body))), nil
			}

			return mcp.NewToolResultText("All notifications marked as read"), nil
		}
}

// GetNotificationThread creates a tool to get a specific notification thread.
func GetNotificationThread(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_notification_thread",
			mcp.WithDescription(t("TOOL_GET_NOTIFICATION_THREAD_DESCRIPTION", "Get a specific notification thread")),
			mcp.WithString("threadID",
				mcp.Required(),
				mcp.Description("The ID of the notification thread"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			threadID, err := requiredParam[string](request, "threadID")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			thread, resp, err := client.Activity.GetThread(ctx, threadID)
			if err != nil {
				return nil, fmt.Errorf("failed to get notification thread: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get notification thread: %s", string(body))), nil
			}

			r, err := json.Marshal(thread)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// markNotificationDone creates a tool to mark a notification as done.
func MarkNotificationDone(getclient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("mark_notification_done",
			mcp.WithDescription(t("TOOL_MARK_NOTIFICATION_DONE_DESCRIPTION", "Mark a notification as done")),
			mcp.WithString("threadID",
				mcp.Required(),
				mcp.Description("The ID of the notification thread"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getclient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			threadIDStr, err := requiredParam[string](request, "threadID")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			threadID, err := strconv.ParseInt(threadIDStr, 10, 64)
			if err != nil {
				return mcp.NewToolResultError("Invalid threadID: must be a numeric value"), nil
			}

			resp, err := client.Activity.MarkThreadDone(ctx, threadID)
			if err != nil {
				return nil, fmt.Errorf("failed to mark notification as done: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusResetContent && resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to mark notification as done: %s", string(body))), nil
			}

			return mcp.NewToolResultText("Notification marked as done"), nil
		}
}
