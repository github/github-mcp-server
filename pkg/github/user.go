package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ListUsersPublicSSHKeys creates a tool to list public ssh keys for user
func ListUsersPublicSSHKeys(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_users_public_ssh_keys",
			mcp.WithDescription(t("TOOL_LIST_USERS_PUBLIC_SSH_KEYS", "Lists the public SSH keys for the authenticated user's GitHub account")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_USERS_PUBLIC_SSH_KEYS_USER_TITLE", "List users public ssh keys"),
				ReadOnlyHint: toBoolPtr(true),
			}),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ListOptions{
				Page:    pagination.page,
				PerPage: pagination.perPage,
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			result, resp, err := client.Users.ListKeys(ctx, "", opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list users ssh keys: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list users ssh keys: %s", string(body))), nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetUsersPublicSSHKey creates a tool to get public ssh key for user
func GetUsersPublicSSHKey(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_users_public_ssh_key",
			mcp.WithDescription(t("TOOL_GET_USERS_PUBLIC_SSH_KEY", "View extended details for a single public SSH key")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_USERS_PUBLIC_SSH_KEY_USER_TITLE", "Get public ssh key details"),
				ReadOnlyHint: toBoolPtr(true),
			}),
			mcp.WithNumber("key_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the key"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			keyId, err := RequiredInt(request, "key_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result, resp, err := client.Users.GetKey(ctx, int64(keyId))
			if err != nil {
				return nil, fmt.Errorf("failed to get ssh key details: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get ssh key details: %s", string(body))), nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// AddPublicSSHKey Adds a public SSH key to the authenticated user's GitHub account
func AddUsersPublicSSHKey(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("add_users_public_ssh_key",
			mcp.WithDescription(t("TOOL_ADD_USERS_PUBLIC_SSH_KEY", "Adds a public SSH key to the authenticated user's GitHub account")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_ADD_USERS_PUBLIC_SSH_KEY_USER_TITLE", "Add users public ssh key"),
				ReadOnlyHint: toBoolPtr(true),
			}),
			mcp.WithString("title",
				mcp.Description("A descriptive name for the new key"),
			),
			mcp.WithString("key",
				mcp.Required(),
				mcp.Description("The public SSH key to add to your GitHub account"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			title, err := OptionalParam[string](request, "title")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			key, err := requiredParam[string](request, "key")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			githubKey := &github.Key{
				Title: &title,
				Key:   &key,
			}
			result, resp, err := client.Users.CreateKey(ctx, githubKey)
			if err != nil {
				return nil, fmt.Errorf("failed to add ssh key: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 201 {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to add ssh key: %s", string(body))), nil
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
