package github

import (
	"context"
	"encoding/json"
	"fmt"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
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

type userListLookupQuery struct {
	Viewer struct {
		Lists struct {
			Nodes []struct {
				ID   githubv4.ID
				Name githubv4.String
			}
			PageInfo struct {
				HasNextPage bool
				EndCursor   string
			}
		} `graphql:"lists(first: 100, after: $after)"`
	}
}

type userListPageQuery struct {
	Viewer struct {
		Lists struct {
			Nodes []struct {
				ID          githubv4.ID
				Name        githubv4.String
				Description githubv4.String
				IsPrivate   githubv4.Boolean
			}
			PageInfo struct {
				HasNextPage bool
				EndCursor   string
			}
			TotalCount githubv4.Int
		} `graphql:"lists(first: 100, after: $after)"`
	}
}

type userListPageWithItemsQuery struct {
	Viewer struct {
		Lists struct {
			Nodes []struct {
				ID          githubv4.ID
				Name        githubv4.String
				Description githubv4.String
				IsPrivate   githubv4.Boolean
				Items       struct {
					Nodes []struct {
						Repository struct {
							NameWithOwner githubv4.String
						} `graphql:"... on Repository"`
					}
					PageInfo struct {
						HasNextPage bool
						EndCursor   string
					}
				} `graphql:"items(first: 100)"`
			}
			PageInfo struct {
				HasNextPage bool
				EndCursor   string
			}
			TotalCount githubv4.Int
		} `graphql:"lists(first: 100, after: $after)"`
	}
}

// getUserListID resolves the authenticated user's list with the given name to
// its node ID. It returns an error when no list matches the name.
func getUserListID(ctx context.Context, client *githubv4.Client, name string) (githubv4.ID, error) {
	var after *githubv4.String
	for {
		var query userListLookupQuery
		vars := map[string]any{"after": after}
		if err := client.Query(ctx, &query, vars); err != nil {
			return "", err
		}
		for _, node := range query.Viewer.Lists.Nodes {
			if string(node.Name) == name {
				return node.ID, nil
			}
		}
		if !query.Viewer.Lists.PageInfo.HasNextPage {
			break
		}
		cursor := githubv4.String(query.Viewer.Lists.PageInfo.EndCursor)
		after = &cursor
	}
	return "", fmt.Errorf("list '%s' not found", name)
}

// listUserLists returns the authenticated user's star lists, optionally
// including the repositories each list contains.
func listUserLists(ctx context.Context, client *githubv4.Client, includeItems bool) ([]userList, int, error) {
	lists := make([]userList, 0)
	totalCount := 0
	var after *githubv4.String
	for {
		if includeItems {
			var query userListPageWithItemsQuery
			vars := map[string]any{"after": after}
			if err := client.Query(ctx, &query, vars); err != nil {
				return nil, 0, err
			}
			totalCount = int(query.Viewer.Lists.TotalCount)
			for _, node := range query.Viewer.Lists.Nodes {
				items := make([]userListItem, 0, len(node.Items.Nodes))
				for _, item := range node.Items.Nodes {
					items = append(items, userListItem{Repository: string(item.Repository.NameWithOwner)})
				}
				if node.Items.PageInfo.HasNextPage {
					cursor := githubv4.String(node.Items.PageInfo.EndCursor)
					remaining, err := listUserListItems(ctx, client, node.ID, &cursor)
					if err != nil {
						return nil, 0, err
					}
					items = append(items, remaining...)
				}
				lists = append(lists, userList{
					ID:          node.ID,
					Name:        string(node.Name),
					Description: string(node.Description),
					IsPrivate:   bool(node.IsPrivate),
					Items:       items,
				})
			}
			if !query.Viewer.Lists.PageInfo.HasNextPage {
				break
			}
			cursor := githubv4.String(query.Viewer.Lists.PageInfo.EndCursor)
			after = &cursor
			continue
		}

		var query userListPageQuery
		vars := map[string]any{"after": after}
		if err := client.Query(ctx, &query, vars); err != nil {
			return nil, 0, err
		}
		totalCount = int(query.Viewer.Lists.TotalCount)
		for _, node := range query.Viewer.Lists.Nodes {
			list := userList{
				ID:          node.ID,
				Name:        string(node.Name),
				Description: string(node.Description),
				IsPrivate:   bool(node.IsPrivate),
			}
			lists = append(lists, list)
		}
		if !query.Viewer.Lists.PageInfo.HasNextPage {
			break
		}
		cursor := githubv4.String(query.Viewer.Lists.PageInfo.EndCursor)
		after = &cursor
	}
	return lists, totalCount, nil
}

// listUserListItems returns the repositories held by a single list, following
// the items connection's cursor until every page has been consumed.
func listUserListItems(ctx context.Context, client *githubv4.Client, listID githubv4.ID, after *githubv4.String) ([]userListItem, error) {
	items := make([]userListItem, 0)
	for {
		var query struct {
			Node struct {
				UserList struct {
					Items struct {
						Nodes []struct {
							Repository struct {
								NameWithOwner githubv4.String
							} `graphql:"... on Repository"`
						}
						PageInfo struct {
							HasNextPage bool
							EndCursor   string
						}
					} `graphql:"items(first: 100, after: $after)"`
				} `graphql:"... on UserList"`
			} `graphql:"node(id: $id)"`
		}
		vars := map[string]any{
			"id":    listID,
			"after": after,
		}
		if err := client.Query(ctx, &query, vars); err != nil {
			return nil, err
		}
		for _, node := range query.Node.UserList.Items.Nodes {
			items = append(items, userListItem{Repository: string(node.Repository.NameWithOwner)})
		}
		if !query.Node.UserList.Items.PageInfo.HasNextPage {
			break
		}
		cursor := githubv4.String(query.Node.UserList.Items.PageInfo.EndCursor)
		after = &cursor
	}
	return items, nil
}

// repoInList reports whether the repository identified by repoID belongs to the
// list identified by listID, paging through the list's items until a match is
// found or the connection is exhausted.
func repoInList(ctx context.Context, client *githubv4.Client, listID, repoID githubv4.ID, after *githubv4.String) (bool, error) {
	for {
		var query struct {
			Node struct {
				UserList struct {
					Items struct {
						Nodes []struct {
							Repository struct {
								ID githubv4.ID
							} `graphql:"... on Repository"`
						}
						PageInfo struct {
							HasNextPage bool
							EndCursor   string
						}
					} `graphql:"items(first: 100, after: $after)"`
				} `graphql:"... on UserList"`
			} `graphql:"node(id: $id)"`
		}
		vars := map[string]any{
			"id":    listID,
			"after": after,
		}
		if err := client.Query(ctx, &query, vars); err != nil {
			return false, err
		}
		for _, node := range query.Node.UserList.Items.Nodes {
			if node.Repository.ID == repoID {
				return true, nil
			}
		}
		if !query.Node.UserList.Items.PageInfo.HasNextPage {
			return false, nil
		}
		cursor := githubv4.String(query.Node.UserList.Items.PageInfo.EndCursor)
		after = &cursor
	}
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
// optional changes. description is nil when the field was omitted (leave
// unchanged) and non-nil when supplied (including an explicit empty string,
// which clears the description).
func updateUserList(ctx context.Context, client *githubv4.Client, name, newName string, description *string, isPrivate *bool) (string, error) {
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
	if description != nil {
		d := githubv4.String(*description)
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
//
// GitHub's schema has no reverse lookup from a repository to its lists (there
// is no `lists` field on Repository). Membership is instead derived by walking
// the viewer's lists and checking each list's items for the repository's node
// ID. Both the lists and each list's items are fully paginated so a repository
// beyond the first 100 items of a list is still counted; omitting it here would
// silently drop the repository from that list on the subsequent mutation.
func setRepoListMemberships(ctx context.Context, client *githubv4.Client, owner, repo, listName string, add bool) error {
	listID, err := getUserListID(ctx, client, listName)
	if err != nil {
		return err
	}

	repoID, err := getRepositoryID(ctx, client, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to find repository: %w", err)
	}

	// Walk every list and, for each, every page of items to determine which
	// lists currently contain the repository.
	var listIDs []githubv4.ID
	var listsAfter *githubv4.String
	for {
		var query struct {
			Viewer struct {
				Lists struct {
					Nodes []struct {
						ID    githubv4.ID
						Items struct {
							Nodes []struct {
								Repository struct {
									ID githubv4.ID
								} `graphql:"... on Repository"`
							}
							PageInfo struct {
								HasNextPage bool
								EndCursor   string
							}
						} `graphql:"items(first: 100)"`
					}
					PageInfo struct {
						HasNextPage bool
						EndCursor   string
					}
				} `graphql:"lists(first: 100, after: $listsAfter)"`
			}
		}
		vars := map[string]any{
			"listsAfter": listsAfter,
		}
		if err := client.Query(ctx, &query, vars); err != nil {
			return err
		}
		for _, list := range query.Viewer.Lists.Nodes {
			contains := false
			for _, item := range list.Items.Nodes {
				if item.Repository.ID == repoID {
					contains = true
					break
				}
			}
			// If the first page didn't contain the repository but the list has
			// more than 100 items, keep paging until we know for certain.
			if !contains && list.Items.PageInfo.HasNextPage {
				cursor := githubv4.String(list.Items.PageInfo.EndCursor)
				var err error
				contains, err = repoInList(ctx, client, list.ID, repoID, &cursor)
				if err != nil {
					return err
				}
			}
			if contains {
				listIDs = append(listIDs, list.ID)
			}
		}

		if !query.Viewer.Lists.PageInfo.HasNextPage {
			break
		}
		cursor := githubv4.String(query.Viewer.Lists.PageInfo.EndCursor)
		listsAfter = &cursor
	}

	present := false
	for _, id := range listIDs {
		if id == listID {
			present = true
			break
		}
	}
	if add == present {
		return nil
	}

	result := make([]githubv4.ID, 0, len(listIDs)+1)
	if add {
		for _, id := range listIDs {
			result = append(result, id)
		}
		if !present {
			result = append(result, listID)
		}
	} else {
		for _, id := range listIDs {
			if id != listID {
				result = append(result, id)
			}
		}
	}

	input := githubv4.UpdateUserListsForItemInput{
		ItemID:  repoID,
		ListIDs: result,
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
		userListReadScopeAccess(),
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
			result := utils.NewToolResultText(string(out))
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelUserList())
			return result, nil, nil
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
			description, descPresent, err := OptionalParamOK[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			isPrivate, present, err := OptionalParamOK[bool](args, "is_private")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if newName == "" && !descPresent && !present {
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
			var descriptionPtr *string
			if descPresent {
				descriptionPtr = &description
			}
			updatedName, err := updateUserList(ctx, client, name, newName, descriptionPtr, isPrivatePtr)
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
				IdempotentHint:  true,
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
		scopes.RequireAll(scopes.User, scopes.Repo),
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
				DestructiveHint: jsonschema.Ptr(true),
				IdempotentHint:  true,
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
		scopes.RequireAll(scopes.User, scopes.Repo),
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
