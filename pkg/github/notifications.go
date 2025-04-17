package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ListNotifications creates a tool to list notifications for a GitHub user.
func ListNotifications(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_notifications",
			mcp.WithDescription(t("TOOL_LIST_NOTIFICATIONS_DESCRIPTION", "List notifications for a GitHub user")),
			mcp.WithNumber("page",
				mcp.Description("Page number"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Number of records per page"),
			),
			mcp.WithBoolean("all",
				mcp.Description("Whether to fetch all notifications, including read ones"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			page, err := OptionalIntParamWithDefault(request, "page", 1)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			perPage, err := OptionalIntParamWithDefault(request, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			all, err := OptionalBoolParamWithDefault(request, "all", false) // Default to false unless specified
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if request.Params.Arguments["all"] == true {
				all = true // Set to true if user explicitly asks for all notifications
			}

			opts := &github.NotificationListOptions{
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: perPage,
				},
				All: all, // Include all notifications, even those already read.
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			notifications, resp, err := client.Activity.ListNotifications(ctx, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list notifications: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list notifications: %s", string(body))), nil
			}

			// Extract the notification title in addition to reason, url, and timestamp.
			var extractedNotifications []map[string]interface{}
			for _, notification := range notifications {
				extractedNotifications = append(extractedNotifications, map[string]interface{}{
					"title":     notification.GetSubject().GetTitle(),
					"reason":    notification.GetReason(),
					"url":       notification.GetURL(),
					"timestamp": notification.GetUpdatedAt(),
				})
			}

			r, err := json.Marshal(extractedNotifications)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal notifications: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
