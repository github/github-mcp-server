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

// ManageNotifications creates a tool to manage notifications (mark as read, mark all as read, or mark as done).
func ManageNotifications(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("manage_notifications",
			mcp.WithDescription(t("TOOL_MANAGE_NOTIFICATIONS_DESCRIPTION", "Manage notifications (mark as read, mark all as read, or mark as done)")),
			mcp.WithString("action",
				mcp.Required(),
				mcp.Description("The action to perform: 'mark_read', 'mark_all_read', or 'mark_done'"),
			),
			mcp.WithString("threadID",
				mcp.Description("The ID of the notification thread (required for 'mark_read' and 'mark_done')"),
			),
			mcp.WithString("lastReadAt",
				mcp.Description("Describes the last point that notifications were checked (optional, for 'mark_all_read'). Default: Now"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			action, err := requiredParam[string](request, "action")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			switch action {
			case "mark_read":
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

			case "mark_done":
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
					return mcp.NewToolResultError(fmt.Sprintf("failed to mark all notifications as read: %s", string(body))), nil
				}

				return mcp.NewToolResultText("Notification marked as done"), nil

			case "mark_all_read":
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

			default:
				return mcp.NewToolResultError("Invalid action: must be 'mark_read', 'mark_all_read', or 'mark_done'"), nil
			}
		}
}
