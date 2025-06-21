package github

import (
	"context"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/go-viper/mapstructure/v2"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"
)

// ListProjects lists projects for a given user or organization.
func ListProjects(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("list_projects",
			mcp.WithDescription(t("TOOL_LIST_PROJECTS_DESCRIPTION", "List Projects for a user or organization")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_LIST_PROJECTS_USER_TITLE", "List projects"), ReadOnlyHint: ToBoolPtr(true)}),
			mcp.WithString("owner", mcp.Required(), mcp.Description("Owner login (user or organization)")),
			mcp.WithString("owner_type", mcp.Description("Owner type"), mcp.Enum("user", "organization")),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ownerType, err := OptionalParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if ownerType == "" {
				ownerType = "organization"
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if ownerType == "user" {
				var q struct {
					User struct {
						Projects struct {
							Nodes []struct {
								ID     githubv4.ID
								Title  githubv4.String
								Number githubv4.Int
							}
						} `graphql:"projectsV2(first: 100)"`
					} `graphql:"user(login: $login)"`
				}
				if err := client.Query(ctx, &q, map[string]any{"login": githubv4.String(owner)}); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return MarshalledTextResult(q), nil
			}
			var q struct {
				Organization struct {
					Projects struct {
						Nodes []struct {
							ID     githubv4.ID
							Title  githubv4.String
							Number githubv4.Int
						}
					} `graphql:"projectsV2(first: 100)"`
				} `graphql:"organization(login: $login)"`
			}
			if err := client.Query(ctx, &q, map[string]any{"login": githubv4.String(owner)}); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return MarshalledTextResult(q), nil
		}
}

// GetProjectFields lists fields for a project.
func GetProjectFields(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_project_fields",
			mcp.WithDescription(t("TOOL_GET_PROJECT_FIELDS_DESCRIPTION", "Get fields for a project")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_GET_PROJECT_FIELDS_USER_TITLE", "Get project fields"), ReadOnlyHint: ToBoolPtr(true)}),
			mcp.WithString("owner", mcp.Required(), mcp.Description("Owner login")),
			mcp.WithString("owner_type", mcp.Description("Owner type"), mcp.Enum("user", "organization")),
			mcp.WithNumber("number", mcp.Required(), mcp.Description("Project number")),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			number, err := RequiredInt(req, "number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ownerType, err := OptionalParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if ownerType == "" {
				ownerType = "organization"
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if ownerType == "user" {
				var q struct {
					User struct {
						Project struct {
							Fields struct {
								Nodes []struct {
									ID       githubv4.ID
									Name     githubv4.String
									DataType githubv4.String
								}
							} `graphql:"fields(first: 100)"`
						} `graphql:"projectV2(number: $number)"`
					} `graphql:"user(login: $login)"`
				}
				if err := client.Query(ctx, &q, map[string]any{"login": githubv4.String(owner), "number": githubv4.Int(number)}); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return MarshalledTextResult(q), nil
			}
			var q struct {
				Organization struct {
					Project struct {
						Fields struct {
							Nodes []struct {
								ID       githubv4.ID
								Name     githubv4.String
								DataType githubv4.String
							}
						} `graphql:"fields(first: 100)"`
					} `graphql:"projectV2(number: $number)"`
				} `graphql:"organization(login: $login)"`
			}
			if err := client.Query(ctx, &q, map[string]any{"login": githubv4.String(owner), "number": githubv4.Int(number)}); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return MarshalledTextResult(q), nil
		}
}

// GetProjectItems lists items for a project.
func GetProjectItems(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("get_project_items",
			mcp.WithDescription(t("TOOL_GET_PROJECT_ITEMS_DESCRIPTION", "Get items for a project")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_GET_PROJECT_ITEMS_USER_TITLE", "Get project items"), ReadOnlyHint: ToBoolPtr(true)}),
			mcp.WithString("owner", mcp.Required(), mcp.Description("Owner login")),
			mcp.WithString("owner_type", mcp.Description("Owner type"), mcp.Enum("user", "organization")),
			mcp.WithNumber("number", mcp.Required(), mcp.Description("Project number")),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			number, err := RequiredInt(req, "number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ownerType, err := OptionalParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if ownerType == "" {
				ownerType = "organization"
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if ownerType == "user" {
				var q struct {
					User struct {
						Project struct {
							Items struct {
								Nodes []struct {
									ID githubv4.ID
								}
							} `graphql:"items(first: 100)"`
						} `graphql:"projectV2(number: $number)"`
					} `graphql:"user(login: $login)"`
				}
				if err := client.Query(ctx, &q, map[string]any{"login": githubv4.String(owner), "number": githubv4.Int(number)}); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return MarshalledTextResult(q), nil
			}
			var q struct {
				Organization struct {
					Project struct {
						Items struct {
							Nodes []struct{ ID githubv4.ID }
						} `graphql:"items(first: 100)"`
					} `graphql:"projectV2(number: $number)"`
				} `graphql:"organization(login: $login)"`
			}
			if err := client.Query(ctx, &q, map[string]any{"login": githubv4.String(owner), "number": githubv4.Int(number)}); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return MarshalledTextResult(q), nil
		}
}

// CreateIssue creates an issue in a repository.
func CreateProjectIssue(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("create_project_issue",
			mcp.WithDescription(t("TOOL_CREATE_PROJECT_ISSUE_DESCRIPTION", "Create a new issue")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_CREATE_PROJECT_ISSUE_USER_TITLE", "Create issue"), ReadOnlyHint: ToBoolPtr(false)}),
			mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner")),
			mcp.WithString("repo", mcp.Required(), mcp.Description("Repository name")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Issue title")),
			mcp.WithString("body", mcp.Description("Issue body")),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var params struct{ Owner, Repo, Title, Body string }
			if err := mapstructure.Decode(req.Params.Arguments, &params); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			var repoQ struct {
				Repository struct{ ID githubv4.ID } `graphql:"repository(owner: $owner, name: $name)"`
			}
			if err := client.Query(ctx, &repoQ, map[string]any{"owner": githubv4.String(params.Owner), "name": githubv4.String(params.Repo)}); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			input := githubv4.CreateIssueInput{RepositoryID: repoQ.Repository.ID, Title: githubv4.String(params.Title)}
			if params.Body != "" {
				input.Body = githubv4.NewString(githubv4.String(params.Body))
			}
			var mut struct {
				CreateIssue struct{ Issue struct{ ID githubv4.ID } } `graphql:"createIssue(input: $input)"`
			}
			if err := client.Mutate(ctx, &mut, input, nil); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return MarshalledTextResult(mut), nil
		}
}

// AddIssueToProject adds an issue to a project by ID.
func AddIssueToProject(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("add_issue_to_project",
			mcp.WithDescription(t("TOOL_ADD_ISSUE_TO_PROJECT_DESCRIPTION", "Add an issue to a project")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_ADD_ISSUE_TO_PROJECT_USER_TITLE", "Add issue to project"), ReadOnlyHint: ToBoolPtr(false)}),
			mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
			mcp.WithString("issue_id", mcp.Required(), mcp.Description("Issue node ID")),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			projectID, err := RequiredParam[string](req, "project_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueID, err := RequiredParam[string](req, "issue_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			var mut struct {
				AddProjectV2ItemById struct {
					Item struct{ ID githubv4.ID }
				} `graphql:"addProjectV2ItemById(input: $input)"`
			}
			input := githubv4.AddProjectV2ItemByIdInput{ProjectID: githubv4.ID(projectID), ContentID: githubv4.ID(issueID)}
			if err := client.Mutate(ctx, &mut, input, nil); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return MarshalledTextResult(mut), nil
		}
}

// UpdateProjectItemField updates a field value on a project item.
func UpdateProjectItemField(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("update_project_item_field",
			mcp.WithDescription(t("TOOL_UPDATE_PROJECT_ITEM_FIELD_DESCRIPTION", "Update a project item field")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_UPDATE_PROJECT_ITEM_FIELD_USER_TITLE", "Update project item field"), ReadOnlyHint: ToBoolPtr(false)}),
			mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
			mcp.WithString("item_id", mcp.Required(), mcp.Description("Item ID")),
			mcp.WithString("field_id", mcp.Required(), mcp.Description("Field ID")),
			mcp.WithString("text_value", mcp.Description("Text value")),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			projectID, err := RequiredParam[string](req, "project_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			itemID, err := RequiredParam[string](req, "item_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fieldID, err := RequiredParam[string](req, "field_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			textValue, err := OptionalParam[string](req, "text_value")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			val := githubv4.ProjectV2FieldValue{}
			if textValue != "" {
				val.Text = githubv4.NewString(githubv4.String(textValue))
			}
			var mut struct {
				UpdateProjectV2ItemFieldValue struct{ Typename githubv4.String } `graphql:"updateProjectV2ItemFieldValue(input: $input)"`
			}
			input := githubv4.UpdateProjectV2ItemFieldValueInput{ProjectID: githubv4.ID(projectID), ItemID: githubv4.ID(itemID), FieldID: githubv4.ID(fieldID), Value: val}
			if err := client.Mutate(ctx, &mut, input, nil); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return MarshalledTextResult(mut), nil
		}
}

// CreateDraftIssue creates a draft issue in a project.
func CreateDraftIssue(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("create_draft_issue",
			mcp.WithDescription(t("TOOL_CREATE_DRAFT_ISSUE_DESCRIPTION", "Create a draft issue in a project")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_CREATE_DRAFT_ISSUE_USER_TITLE", "Create draft issue"), ReadOnlyHint: ToBoolPtr(false)}),
			mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Issue title")),
			mcp.WithString("body", mcp.Description("Issue body")),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			projectID, err := RequiredParam[string](req, "project_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			title, err := RequiredParam[string](req, "title")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body, err := OptionalParam[string](req, "body")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			input := githubv4.AddProjectV2DraftIssueInput{ProjectID: githubv4.ID(projectID), Title: githubv4.String(title)}
			if body != "" {
				input.Body = githubv4.NewString(githubv4.String(body))
			}
			var mut struct {
				AddProjectV2DraftIssue struct{ Item struct{ ID githubv4.ID } } `graphql:"addProjectV2DraftIssue(input: $input)"`
			}
			if err := client.Mutate(ctx, &mut, input, nil); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return MarshalledTextResult(mut), nil
		}
}

// DeleteProjectItem removes an item from a project.
func DeleteProjectItem(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("delete_project_item",
			mcp.WithDescription(t("TOOL_DELETE_PROJECT_ITEM_DESCRIPTION", "Delete a project item")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_DELETE_PROJECT_ITEM_USER_TITLE", "Delete project item"), ReadOnlyHint: ToBoolPtr(false)}),
			mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
			mcp.WithString("item_id", mcp.Required(), mcp.Description("Item ID")),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			projectID, err := RequiredParam[string](req, "project_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			itemID, err := RequiredParam[string](req, "item_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			var mut struct {
				DeleteProjectV2Item struct{ Typename githubv4.String } `graphql:"deleteProjectV2Item(input: $input)"`
			}
			input := githubv4.DeleteProjectV2ItemInput{ProjectID: githubv4.ID(projectID), ItemID: githubv4.ID(itemID)}
			if err := client.Mutate(ctx, &mut, input, nil); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return MarshalledTextResult(mut), nil
		}
}
