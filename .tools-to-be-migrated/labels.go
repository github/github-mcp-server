package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"
)

// GetLabel retrieves a specific label by name from a GitHub repository
func GetLabel(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool(
			"get_label",
			mcp.WithDescription(t("TOOL_GET_LABEL_DESCRIPTION", "Get a specific label from a repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_LABEL_TITLE", "Get a specific label from a repository."),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner (username or organization name)"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Label name."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			name, err := RequiredParam[string](request, "name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var query struct {
				Repository struct {
					Label struct {
						ID          githubv4.ID
						Name        githubv4.String
						Color       githubv4.String
						Description githubv4.String
					} `graphql:"label(name: $name)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}

			vars := map[string]any{
				"owner": githubv4.String(owner),
				"repo":  githubv4.String(repo),
				"name":  githubv4.String(name),
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			if err := client.Query(ctx, &query, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to find label", err), nil
			}

			if query.Repository.Label.Name == "" {
				return mcp.NewToolResultError(fmt.Sprintf("label '%s' not found in %s/%s", name, owner, repo)), nil
			}

			label := map[string]any{
				"id":          fmt.Sprintf("%v", query.Repository.Label.ID),
				"name":        string(query.Repository.Label.Name),
				"color":       string(query.Repository.Label.Color),
				"description": string(query.Repository.Label.Description),
			}

			out, err := json.Marshal(label)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal label: %w", err)
			}

			return mcp.NewToolResultText(string(out)), nil
		}
}

// ListLabels lists labels from a repository
func ListLabels(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool(
			"list_label",
			mcp.WithDescription(t("TOOL_LIST_LABEL_DESCRIPTION", "List labels from a repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_LABEL_DESCRIPTION", "List labels from a repository."),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner (username or organization name) - required for all operations"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name - required for all operations"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			var query struct {
				Repository struct {
					Labels struct {
						Nodes []struct {
							ID          githubv4.ID
							Name        githubv4.String
							Color       githubv4.String
							Description githubv4.String
						}
						TotalCount githubv4.Int
					} `graphql:"labels(first: 100)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}

			vars := map[string]any{
				"owner": githubv4.String(owner),
				"repo":  githubv4.String(repo),
			}

			if err := client.Query(ctx, &query, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to list labels", err), nil
			}

			labels := make([]map[string]any, len(query.Repository.Labels.Nodes))
			for i, labelNode := range query.Repository.Labels.Nodes {
				labels[i] = map[string]any{
					"id":          fmt.Sprintf("%v", labelNode.ID),
					"name":        string(labelNode.Name),
					"color":       string(labelNode.Color),
					"description": string(labelNode.Description),
				}
			}

			response := map[string]any{
				"labels":     labels,
				"totalCount": int(query.Repository.Labels.TotalCount),
			}

			out, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal labels: %w", err)
			}

			return mcp.NewToolResultText(string(out)), nil
		}
}

// LabelWrite handles create, update, and delete operations for GitHub labels
func LabelWrite(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool(
			"label_write",
			mcp.WithDescription(t("TOOL_LABEL_WRITE_DESCRIPTION", "Perform write operations on repository labels. To set labels on issues, use the 'update_issue' tool.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LABEL_WRITE_TITLE", "Write operations on repository labels."),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("method",
				mcp.Required(),
				mcp.Description("Operation to perform: 'create', 'update', or 'delete'"),
				mcp.Enum("create", "update", "delete"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner (username or organization name)"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Label name - required for all operations"),
			),
			mcp.WithString("new_name",
				mcp.Description("New name for the label (used only with 'update' method to rename)"),
			),
			mcp.WithString("color",
				mcp.Description("Label color as 6-character hex code without '#' prefix (e.g., 'f29513'). Required for 'create', optional for 'update'."),
			),
			mcp.WithString("description",
				mcp.Description("Label description text. Optional for 'create' and 'update'."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Get and validate required parameters
			method, err := RequiredParam[string](request, "method")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			method = strings.ToLower(method)

			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			name, err := RequiredParam[string](request, "name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get optional parameters
			newName, _ := OptionalParam[string](request, "new_name")
			color, _ := OptionalParam[string](request, "color")
			description, _ := OptionalParam[string](request, "description")

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case "create":
				// Validate required params for create
				if color == "" {
					return mcp.NewToolResultError("color is required for create"), nil
				}

				// Get repository ID
				repoID, err := getRepositoryID(ctx, client, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to find repository", err), nil
				}

				input := githubv4.CreateLabelInput{
					RepositoryID: repoID,
					Name:         githubv4.String(name),
					Color:        githubv4.String(color),
				}
				if description != "" {
					d := githubv4.String(description)
					input.Description = &d
				}

				var mutation struct {
					CreateLabel struct {
						Label struct {
							Name githubv4.String
							ID   githubv4.ID
						}
					} `graphql:"createLabel(input: $input)"`
				}

				if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to create label", err), nil
				}

				return mcp.NewToolResultText(fmt.Sprintf("label '%s' created successfully", mutation.CreateLabel.Label.Name)), nil

			case "update":
				// Validate required params for update
				if newName == "" && color == "" && description == "" {
					return mcp.NewToolResultError("at least one of new_name, color, or description must be provided for update"), nil
				}

				// Get the label ID
				labelID, err := getLabelID(ctx, client, owner, repo, name)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				input := githubv4.UpdateLabelInput{
					ID: labelID,
				}
				if newName != "" {
					n := githubv4.String(newName)
					input.Name = &n
				}
				if color != "" {
					c := githubv4.String(color)
					input.Color = &c
				}
				if description != "" {
					d := githubv4.String(description)
					input.Description = &d
				}

				var mutation struct {
					UpdateLabel struct {
						Label struct {
							Name githubv4.String
							ID   githubv4.ID
						}
					} `graphql:"updateLabel(input: $input)"`
				}

				if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to update label", err), nil
				}

				return mcp.NewToolResultText(fmt.Sprintf("label '%s' updated successfully", mutation.UpdateLabel.Label.Name)), nil

			case "delete":
				// Get the label ID
				labelID, err := getLabelID(ctx, client, owner, repo, name)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				input := githubv4.DeleteLabelInput{
					ID: labelID,
				}

				var mutation struct {
					DeleteLabel struct {
						ClientMutationID githubv4.String
					} `graphql:"deleteLabel(input: $input)"`
				}

				if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to delete label", err), nil
				}

				return mcp.NewToolResultText(fmt.Sprintf("label '%s' deleted successfully", name)), nil

			default:
				return mcp.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create, update, delete", method)), nil
			}
		}
}

// Helper function to get repository ID
func getRepositoryID(ctx context.Context, client *githubv4.Client, owner, repo string) (githubv4.ID, error) {
	var repoQuery struct {
		Repository struct {
			ID githubv4.ID
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	vars := map[string]any{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
	}
	if err := client.Query(ctx, &repoQuery, vars); err != nil {
		return "", err
	}
	return repoQuery.Repository.ID, nil
}

// Helper function to get label by name
func getLabelID(ctx context.Context, client *githubv4.Client, owner, repo, labelName string) (githubv4.ID, error) {
	var query struct {
		Repository struct {
			Label struct {
				ID   githubv4.ID
				Name githubv4.String
			} `graphql:"label(name: $name)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	vars := map[string]any{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
		"name":  githubv4.String(labelName),
	}
	if err := client.Query(ctx, &query, vars); err != nil {
		return "", err
	}
	if query.Repository.Label.Name == "" {
		return "", fmt.Errorf("label '%s' not found in %s/%s", labelName, owner, repo)
	}
	return query.Repository.Label.ID, nil
}
