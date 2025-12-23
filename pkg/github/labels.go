package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// GetLabel retrieves a specific label by name from a GitHub repository
func GetLabel(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "get_label",
			Description: t("TOOL_GET_LABEL_DESCRIPTION", "Get a specific label from a repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_LABEL_TITLE", "Get a specific label from a repository."),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"name": {
						Type:        "string",
						Description: "Label name.",
					},
				},
				Required: []string{"owner", "repo", "name"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			name, err := RequiredParam[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			if err := client.Query(ctx, &query, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to find label", err), nil, nil
			}

			if query.Repository.Label.Name == "" {
				return utils.NewToolResultError(fmt.Sprintf("label '%s' not found in %s/%s", name, owner, repo)), nil, nil
			}

			label := map[string]any{
				"id":          fmt.Sprintf("%v", query.Repository.Label.ID),
				"name":        string(query.Repository.Label.Name),
				"color":       string(query.Repository.Label.Color),
				"description": string(query.Repository.Label.Description),
			}

			out, err := json.Marshal(label)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal label: %w", err)
			}

			return utils.NewToolResultText(string(out)), nil, nil
		},
	)
}

// GetLabelForLabelsToolset returns the same GetLabel tool but registered in the labels toolset.
// This provides conformance with the original behavior where get_label was in both toolsets.
func GetLabelForLabelsToolset(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := GetLabel(t)
	tool.Toolset = ToolsetLabels
	return tool
}

// ListLabels lists labels from a repository
func ListLabels(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetLabels,
		mcp.Tool{
			Name:        "list_label",
			Description: t("TOOL_LIST_LABEL_DESCRIPTION", "List labels from a repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_LABEL_DESCRIPTION", "List labels from a repository."),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name) - required for all operations",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name - required for all operations",
					},
				},
				Required: []string{"owner", "repo"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to list labels", err), nil, nil
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
				return nil, nil, fmt.Errorf("failed to marshal labels: %w", err)
			}

			return utils.NewToolResultText(string(out)), nil, nil
		},
	)
}

// LabelWrite handles create, update, and delete operations for GitHub labels
func LabelWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetLabels,
		mcp.Tool{
			Name:        "label_write",
			Description: t("TOOL_LABEL_WRITE_DESCRIPTION", "Perform write operations on repository labels. To set labels on issues, use the 'update_issue' tool."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LABEL_WRITE_TITLE", "Write operations on repository labels."),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "Operation to perform: 'create', 'update', or 'delete'",
						Enum:        []any{"create", "update", "delete"},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner (username or organization name)",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"name": {
						Type:        "string",
						Description: "Label name - required for all operations",
					},
					"new_name": {
						Type:        "string",
						Description: "New name for the label (used only with 'update' method to rename)",
					},
					"color": {
						Type:        "string",
						Description: "Label color as 6-character hex code without '#' prefix (e.g., 'f29513'). Required for 'create', optional for 'update'.",
					},
					"description": {
						Type:        "string",
						Description: "Label description text. Optional for 'create' and 'update'.",
					},
				},
				Required: []string{"method", "owner", "repo", "name"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			// Get and validate required parameters
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			method = strings.ToLower(method)

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			name, err := RequiredParam[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get optional parameters
			newName, _ := OptionalParam[string](args, "new_name")
			color, _ := OptionalParam[string](args, "color")
			description, _ := OptionalParam[string](args, "description")

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch method {
			case "create":
				// Validate required params for create
				if color == "" {
					return utils.NewToolResultError("color is required for create"), nil, nil
				}

				// Get repository ID
				repoID, err := getRepositoryID(ctx, client, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to find repository", err), nil, nil
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
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to create label", err), nil, nil
				}

				return utils.NewToolResultText(fmt.Sprintf("label '%s' created successfully", mutation.CreateLabel.Label.Name)), nil, nil

			case "update":
				// Validate required params for update
				if newName == "" && color == "" && description == "" {
					return utils.NewToolResultError("at least one of new_name, color, or description must be provided for update"), nil, nil
				}

				// Get the label ID
				labelID, err := getLabelID(ctx, client, owner, repo, name)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
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
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to update label", err), nil, nil
				}

				return utils.NewToolResultText(fmt.Sprintf("label '%s' updated successfully", mutation.UpdateLabel.Label.Name)), nil, nil

			case "delete":
				// Get the label ID
				labelID, err := getLabelID(ctx, client, owner, repo, name)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
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
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to delete label", err), nil, nil
				}

				return utils.NewToolResultText(fmt.Sprintf("label '%s' deleted successfully", name)), nil, nil

			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s. Supported methods are: create, update, delete", method)), nil, nil
			}
		},
	)
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
