package github

import (
	"context"
	"encoding/json"
	"fmt"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// userList represents a GitHub star list (UserList) surfaced through the tools.
type userList struct {
	ID          githubv4.ID    `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	IsPrivate   bool           `json:"is_private"`
	Items       []userListItem `json:"items,omitempty"`
}

type userListItem struct {
	Repository string `json:"repository"`
}

// getUserListID resolves the authenticated user's list with the given name to
// its node ID. It returns an error when no list matches the name.
func getUserListID(ctx context.Context, client *githubv4.Client, name string) (githubv4.ID, error) {
	var query struct {
		Viewer struct {
			Lists struct {
				Nodes []struct {
					ID   githubv4.ID
					Name githubv4.String
				}
			} `graphql:"lists(first: 100)"`
		}
	}
	if err := client.Query(ctx, &query, nil); err != nil {
		return "", err
	}
	for _, node := range query.Viewer.Lists.Nodes {
		if string(node.Name) == name {
			return node.ID, nil
		}
	}
	return "", fmt.Errorf("list '%s' not found", name)
}

// listUserLists returns the authenticated user's star lists, optionally
// including the repositories each list contains.
func listUserLists(ctx context.Context, client *githubv4.Client, includeItems bool) ([]userList, int, error) {
	var query struct {
		Viewer struct {
			Lists struct {
				Nodes []struct {
					ID          githubv4.ID
					Name        githubv4.String
					Description githubv4.String
					IsPrivate   githubv4.Boolean
				}
				TotalCount githubv4.Int
			} `graphql:"lists(first: 100)"`
		}
	}
	if err := client.Query(ctx, &query, nil); err != nil {
		return nil, 0, err
	}

	lists := make([]userList, 0, len(query.Viewer.Lists.Nodes))
	for _, node := range query.Viewer.Lists.Nodes {
		list := userList{
			ID:          node.ID,
			Name:        string(node.Name),
			Description: string(node.Description),
			IsPrivate:   bool(node.IsPrivate),
		}
		if includeItems {
			items, err := listUserListItems(ctx, client, node.ID)
			if err != nil {
				return nil, 0, err
			}
			list.Items = items
		}
		lists = append(lists, list)
	}
	return lists, int(query.Viewer.Lists.TotalCount), nil
}

// listUserListItems returns the repositories held by a single list.
func listUserListItems(ctx context.Context, client *githubv4.Client, listID githubv4.ID) ([]userListItem, error) {
	var query struct {
		Node struct {
			UserList struct {
				Items struct {
					Nodes []struct {
						Repository struct {
							NameWithOwner githubv4.String
						}
					}
				} `graphql:"items(first: 100)"`
			} `graphql:"... on UserList"`
		} `graphql:"node(id: $id)"`
	}
	vars := map[string]any{
		"id": listID,
	}
	if err := client.Query(ctx, &query, vars); err != nil {
		return nil, err
	}
	items := make([]userListItem, 0, len(query.Node.UserList.Items.Nodes))
	for _, node := range query.Node.UserList.Items.Nodes {
		items = append(items, userListItem{Repository: string(node.Repository.NameWithOwner)})
	}
	return items, nil
}

// createUserList creates a new star list for the authenticated user.
func createUserList(ctx context.Context, client *githubv4.Client, name, description string, isPrivate *bool) (string, error) {
	input := githubv4.CreateUserListInput{
		Name: githubv4.String(name),
	}
	if description != "" {
		d := githubv4.String(description)
		input.Description = &d
	}
	if isPrivate != nil {
		p := githubv4.Boolean(*isPrivate)
		input.IsPrivate = &p
	}

	var mutation struct {
		CreateUserList struct {
			List struct {
				ID   githubv4.ID
				Name githubv4.String
			}
		} `graphql:"createUserList(input: $input)"`
	}
	if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
		return "", err
	}
	return string(mutation.CreateUserList.List.Name), nil
}

// updateUserList updates the name, description, and/or privacy of an existing
// star list. name identifies the list; newName, description, and isPrivate are
// optional changes.
func updateUserList(ctx context.Context, client *githubv4.Client, name, newName, description string, isPrivate *bool) (string, error) {
	listID, err := getUserListID(ctx, client, name)
	if err != nil {
		return "", err
	}

	input := githubv4.UpdateUserListInput{
		ListID: listID,
	}
	if newName != "" {
		n := githubv4.String(newName)
		input.Name = &n
	}
	if description != "" {
		d := githubv4.String(description)
		input.Description = &d
	}
	if isPrivate != nil {
		p := githubv4.Boolean(*isPrivate)
		input.IsPrivate = &p
	}

	var mutation struct {
		UpdateUserList struct {
			List struct {
				Name githubv4.String
			}
		} `graphql:"updateUserList(input: $input)"`
	}
	if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
		return "", err
	}
	return string(mutation.UpdateUserList.List.Name), nil
}

// deleteUserList deletes a star list owned by the authenticated user.
func deleteUserList(ctx context.Context, client *githubv4.Client, name string) error {
	listID, err := getUserListID(ctx, client, name)
	if err != nil {
		return err
	}

	input := githubv4.DeleteUserListInput{
		ListID: listID,
	}
	var mutation struct {
		DeleteUserList struct {
			ClientMutationID githubv4.String
		} `graphql:"deleteUserList(input: $input)"`
	}
	if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
		return err
	}
	return nil
}

// setRepoListMemberships adds (add=true) or removes (add=false) a repository
// from the named list. updateUserListsForItem REPLACES the repository's full
// list membership, so the current set is read first, merged/subtracted, and
// resubmitted in full.
func setRepoListMemberships(ctx context.Context, client *githubv4.Client, owner, repo, listName string, add bool) error {
	listID, err := getUserListID(ctx, client, listName)
	if err != nil {
		return err
	}

	repoID, err := getRepositoryID(ctx, client, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to find repository: %w", err)
	}

	var repoQuery struct {
		Repository struct {
			Lists struct {
				Nodes []struct {
					ID githubv4.ID
				}
			} `graphql:"lists(first: 100)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	vars := map[string]any{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
	}
	if err := client.Query(ctx, &repoQuery, vars); err != nil {
		return err
	}

	listIDs := make([]githubv4.ID, 0, len(repoQuery.Repository.Lists.Nodes)+1)
	present := false
	for _, node := range repoQuery.Repository.Lists.Nodes {
		if node.ID == listID {
			present = true
			if !add {
				continue
			}
		}
		listIDs = append(listIDs, node.ID)
	}
	if add && !present {
		listIDs = append(listIDs, listID)
	}

	input := githubv4.UpdateUserListsForItemInput{
		ItemID:  repoID,
		ListIDs: listIDs,
	}
	var mutation struct {
		UpdateUserListsForItem struct {
			ClientMutationID githubv4.String
		} `graphql:"updateUserListsForItem(input: $input)"`
	}
	if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
		return err
	}
	return nil
}

// ListUserLists creates a tool to list the authenticated user's star lists.
func ListUserLists(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataStargazers,
		mcp.Tool{
			Name:        "list_user_lists",
			Description: t("TOOL_LIST_USER_LISTS_DESCRIPTION", "List the authenticated user's star lists (UserLists), optionally including the repositories in each list."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_USER_LISTS_USER_TITLE", "List star lists"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"include_items": {
						Type:        "boolean",
						Description: "Whether to include the repositories in each list.",
					},
				},
			},
		},
		scopes.PublicRead(scopes.ReadUser),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			includeItems, err := OptionalParam[bool](args, "include_items")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			lists, totalCount, err := listUserLists(ctx, client, includeItems)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to list user lists", err), nil, nil
			}

			response := map[string]any{
				"lists":      lists,
				"totalCount": totalCount,
			}
			out, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal user lists: %w", err)
			}
			return utils.NewToolResultText(string(out)), nil, nil
		},
	)
}

// CreateUserList creates a tool to create a new star list.
func CreateUserList(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataStargazers,
		mcp.Tool{
			Name:        "create_user_list",
			Description: t("TOOL_CREATE_USER_LIST_DESCRIPTION", "Create a new star list (UserList) for the authenticated user."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_CREATE_USER_LIST_USER_TITLE", "Create star list"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {
						Type:        "string",
						Description: "The name of the new list.",
					},
					"description": {
						Type:        "string",
						Description: "A description of the list.",
					},
					"is_private": {
						Type:        "boolean",
						Description: "Whether the list is private.",
					},
				},
				Required: []string{"name"},
			},
		},
		scopes.RequireAll(scopes.User),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			name, err := RequiredParam[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			description, err := OptionalParam[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			isPrivate, present, err := OptionalParamOK[bool](args, "is_private")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			var isPrivatePtr *bool
			if present {
				isPrivatePtr = &isPrivate
			}
			createdName, err := createUserList(ctx, client, name, description, isPrivatePtr)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to create user list", err), nil, nil
			}
			return utils.NewToolResultText(fmt.Sprintf("list '%s' created successfully", createdName)), nil, nil
		},
	)
}

// UpdateUserList creates a tool to update an existing star list.
func UpdateUserList(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataStargazers,
		mcp.Tool{
			Name:        "update_user_list",
			Description: t("TOOL_UPDATE_USER_LIST_DESCRIPTION", "Update an existing star list (UserList): rename it, change its description, or change its privacy."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_USER_LIST_USER_TITLE", "Update star list"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {
						Type:        "string",
						Description: "The current name of the list to update.",
					},
					"new_name": {
						Type:        "string",
						Description: "The new name for the list.",
					},
					"description": {
						Type:        "string",
						Description: "The new description for the list.",
					},
					"is_private": {
						Type:        "boolean",
						Description: "Whether the list is private.",
					},
				},
				Required: []string{"name"},
			},
		},
		scopes.RequireAll(scopes.User),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			name, err := RequiredParam[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			newName, err := OptionalParam[string](args, "new_name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			description, err := OptionalParam[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			isPrivate, present, err := OptionalParamOK[bool](args, "is_private")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if newName == "" && description == "" && !present {
				return utils.NewToolResultError("at least one of new_name, description, or is_private must be provided for update"), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			var isPrivatePtr *bool
			if present {
				isPrivatePtr = &isPrivate
			}
			updatedName, err := updateUserList(ctx, client, name, newName, description, isPrivatePtr)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to update user list", err), nil, nil
			}
			return utils.NewToolResultText(fmt.Sprintf("list '%s' updated successfully", updatedName)), nil, nil
		},
	)
}

// DeleteUserList creates a tool to delete a star list.
func DeleteUserList(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataStargazers,
		mcp.Tool{
			Name:        "delete_user_list",
			Description: t("TOOL_DELETE_USER_LIST_DESCRIPTION", "Delete a star list (UserList) owned by the authenticated user."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_DELETE_USER_LIST_USER_TITLE", "Delete star list"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {
						Type:        "string",
						Description: "The name of the list to delete.",
					},
				},
				Required: []string{"name"},
			},
		},
		scopes.RequireAll(scopes.User),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			name, err := RequiredParam[string](args, "name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			if err := deleteUserList(ctx, client, name); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to delete user list", err), nil, nil
			}
			return utils.NewToolResultText(fmt.Sprintf("list '%s' deleted successfully", name)), nil, nil
		},
	)
}

// AddRepositoryToList creates a tool to add a repository to a star list.
func AddRepositoryToList(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataStargazers,
		mcp.Tool{
			Name:        "add_repository_to_list",
			Description: t("TOOL_ADD_REPOSITORY_TO_LIST_DESCRIPTION", "Add a repository to a star list (UserList). List membership is independent of star state."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_ADD_REPOSITORY_TO_LIST_USER_TITLE", "Add repository to star list"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
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
					"list_name": {
						Type:        "string",
						Description: "The name of the star list to add the repository to.",
					},
				},
				Required: []string{"owner", "repo", "list_name"},
			},
		},
		scopes.RequireAll(scopes.User),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			listName, err := RequiredParam[string](args, "list_name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			if err := setRepoListMemberships(ctx, client, owner, repo, listName, true); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to add repository to list", err), nil, nil
			}
			return utils.NewToolResultText(fmt.Sprintf("repository %s/%s added to list '%s'", owner, repo, listName)), nil, nil
		},
	)
}

// RemoveRepositoryFromList creates a tool to remove a repository from a star list.
func RemoveRepositoryFromList(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataStargazers,
		mcp.Tool{
			Name:        "remove_repository_from_list",
			Description: t("TOOL_REMOVE_REPOSITORY_FROM_LIST_DESCRIPTION", "Remove a repository from a star list (UserList). List membership is independent of star state."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_REMOVE_REPOSITORY_FROM_LIST_USER_TITLE", "Remove repository from star list"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
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
					"list_name": {
						Type:        "string",
						Description: "The name of the star list to remove the repository from.",
					},
				},
				Required: []string{"owner", "repo", "list_name"},
			},
		},
		scopes.RequireAll(scopes.User),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			listName, err := RequiredParam[string](args, "list_name")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			if err := setRepoListMemberships(ctx, client, owner, repo, listName, false); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to remove repository from list", err), nil, nil
			}
			return utils.NewToolResultText(fmt.Sprintf("repository %s/%s removed from list '%s'", owner, repo, listName)), nil, nil
		},
	)
}
