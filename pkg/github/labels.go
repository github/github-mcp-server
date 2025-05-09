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

// ListLabels creates a tool to list labels in a GitHub repository.
func ListLabels(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_labels",
			mcp.WithDescription(t("TOOL_LIST_LABELS_DESCRIPTION", "List labels for a repository")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
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
			labels, resp, err := client.Issues.ListLabels(ctx, owner, repo, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list labels: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list labels: %s", string(body))), nil
			}

			r, err := json.Marshal(labels)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetLabel creates a tool to get a specific label in a GitHub repository.
func GetLabel(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_label",
			mcp.WithDescription(t("TOOL_GET_LABEL_DESCRIPTION", "Get a specific label from a repository")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Label name"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := requiredParam[string](request, "name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			label, resp, err := client.Issues.GetLabel(ctx, owner, repo, name)
			if err != nil {
				return nil, fmt.Errorf("failed to get label: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get label: %s", string(body))), nil
			}

			r, err := json.Marshal(label)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// CreateLabel creates a tool to create a new label in a GitHub repository.
func CreateLabel(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_label",
			mcp.WithDescription(t("TOOL_CREATE_LABEL_DESCRIPTION", "Create a label in a GitHub repository")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Label name"),
			),
			mcp.WithString("color",
				mcp.Required(),
				mcp.Description("The hexadecimal color code for the label, without the leading #"),
			),
			mcp.WithString("description",
				mcp.Description("A short description of the label"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := requiredParam[string](request, "name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			color, err := requiredParam[string](request, "color")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			description, err := OptionalParam[string](request, "description")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Create the label request
			labelRequest := &github.Label{
				Name:        github.Ptr(name),
				Color:       github.Ptr(color),
				Description: github.Ptr(description),
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			label, resp, err := client.Issues.CreateLabel(ctx, owner, repo, labelRequest)
			if err != nil {
				return nil, fmt.Errorf("failed to create label: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to create label: %s", string(body))), nil
			}

			r, err := json.Marshal(label)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// UpdateLabel creates a tool to update an existing label in a GitHub repository.
func UpdateLabel(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("update_label",
			mcp.WithDescription(t("TOOL_UPDATE_LABEL_DESCRIPTION", "Update a label in a GitHub repository")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Current label name"),
			),
			mcp.WithString("new_name",
				mcp.Description("New label name"),
			),
			mcp.WithString("color",
				mcp.Description("The hexadecimal color code for the label, without the leading #"),
			),
			mcp.WithString("description",
				mcp.Description("A short description of the label"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := requiredParam[string](request, "name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Create the label update
			labelRequest := &github.Label{}
			updateNeeded := false

			newName, err := OptionalParam[string](request, "new_name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if newName != "" {
				labelRequest.Name = github.Ptr(newName)
				updateNeeded = true
			}

			color, err := OptionalParam[string](request, "color")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if color != "" {
				labelRequest.Color = github.Ptr(color)
				updateNeeded = true
			}

			description, err := OptionalParam[string](request, "description")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if description != "" {
				labelRequest.Description = github.Ptr(description)
				updateNeeded = true
			}

			if !updateNeeded {
				return mcp.NewToolResultError("No update parameters provided."), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			label, resp, err := client.Issues.EditLabel(ctx, owner, repo, name, labelRequest)
			if err != nil {
				return nil, fmt.Errorf("failed to update label: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to update label: %s", string(body))), nil
			}

			r, err := json.Marshal(label)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// DeleteLabel creates a tool to delete a label from a GitHub repository.
func DeleteLabel(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("delete_label",
			mcp.WithDescription(t("TOOL_DELETE_LABEL_DESCRIPTION", "Delete a label from a GitHub repository")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Label name"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := requiredParam[string](request, "name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			resp, err := client.Issues.DeleteLabel(ctx, owner, repo, name)
			if err != nil {
				return nil, fmt.Errorf("failed to delete label: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusNoContent {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to delete label: %s", string(body))), nil
			}

			result := fmt.Sprintf("Label '%s' successfully deleted from %s/%s", name, owner, repo)
			return mcp.NewToolResultText(result), nil
		}
}

// ListLabelsForIssue creates a tool to list labels on an issue.
func ListLabelsForIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_labels_for_issue",
			mcp.WithDescription(t("TOOL_LIST_LABELS_FOR_ISSUE_DESCRIPTION", "List labels for an issue")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue number"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
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
			labels, resp, err := client.Issues.ListLabelsByIssue(ctx, owner, repo, issueNumber, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to list labels for issue: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list labels for issue: %s", string(body))), nil
			}

			r, err := json.Marshal(labels)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// AddLabelsToIssue creates a tool to add labels to an issue.
func AddLabelsToIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("add_labels_to_issue",
			mcp.WithDescription(t("TOOL_ADD_LABELS_TO_ISSUE_DESCRIPTION", "Add labels to an issue")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue number"),
			),
			mcp.WithArray("labels",
				mcp.Required(),
				mcp.Description("Labels to add to the issue"),
				mcp.Items(
					map[string]interface{}{
						"type": "string",
					},
				),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get labels
			labels, err := RequiredStringArrayParam(request, "labels")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			addedLabels, resp, err := client.Issues.AddLabelsToIssue(ctx, owner, repo, issueNumber, labels)
			if err != nil {
				return nil, fmt.Errorf("failed to add labels to issue: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to add labels to issue: %s", string(body))), nil
			}

			r, err := json.Marshal(addedLabels)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// RemoveLabelFromIssue creates a tool to remove a label from an issue.
func RemoveLabelFromIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("remove_label_from_issue",
			mcp.WithDescription(t("TOOL_REMOVE_LABEL_FROM_ISSUE_DESCRIPTION", "Remove a label from an issue")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue number"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Label name"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			name, err := requiredParam[string](request, "name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			resp, err := client.Issues.RemoveLabelForIssue(ctx, owner, repo, issueNumber, name)
			if err != nil {
				return nil, fmt.Errorf("failed to remove label from issue: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to remove label from issue: %s", string(body))), nil
			}

			result := fmt.Sprintf("Label '%s' successfully removed from issue #%d in %s/%s", name, issueNumber, owner, repo)
			return mcp.NewToolResultText(result), nil
		}
}
