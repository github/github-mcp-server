package github

import (
	"context"
	"fmt"
	"strings"

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

// GetProject defines a tool that retrieves detailed information about a specific GitHub ProjectV2.
// It takes a project number or name and owner as input and works for both organizations and users.
func GetProject(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
    return mcp.NewTool("get_project",
            mcp.WithDescription(t("TOOL_GET_PROJECT_DESCRIPTION", "Get details for a specific project using its number or name")),
            mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_GET_PROJECT_TITLE", "Get project details"), ReadOnlyHint: ToBoolPtr(true)}),
            mcp.WithString("owner", mcp.Required(), mcp.Description("Owner login (user or organization)")),
            mcp.WithNumber("number", mcp.Description("Project number (either number or name must be provided)")),
            mcp.WithString("name", mcp.Description("Project name (either number or name must be provided)")),
            mcp.WithString("owner_type", mcp.Description("Owner type"), mcp.Enum("user", "organization")),
        ), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get optional parameters
			number, numberErr := OptionalParam[float64](req, "number")
			name, nameErr := OptionalParam[string](req, "name")

			// Check if parameters were actually provided (not just no error)
			nameProvided := nameErr == nil && name != ""
			numberProvided := numberErr == nil && number != 0

			// CORRECTED VALIDATION:
			// 1. Check if both were provided
			if nameProvided && numberProvided {
				return mcp.NewToolResultError("Cannot provide both 'number' and 'name' parameters. Please use only one."), nil
			}
			// 2. Check if neither was provided
			if !nameProvided && !numberProvided {
				return mcp.NewToolResultError("Either the 'number' or 'name' parameter must be provided."), nil
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

			// Route to the correct helper function based on which parameter was provided
			if nameProvided {
				return getProjectByName(ctx, client, owner, name, ownerType)
			}

			// If it wasn't name, it must be number
			projectNumber := int(number)
			return getProjectByNumber(ctx, client, owner, projectNumber, ownerType)
        }
}

// Helper function to get project by number
func getProjectByNumber(ctx context.Context, client interface{}, owner string, number int, ownerType string) (*mcp.CallToolResult, error) {
    type GraphQLClient interface {
        Query(ctx context.Context, q interface{}, variables map[string]interface{}) error
    }
    
    gqlClient := client.(GraphQLClient)
    
    if ownerType == "user" {
        var q struct {
            User struct {
                ProjectV2 struct {
                    ID     githubv4.ID
                    Title  githubv4.String
                    Number githubv4.Int
                    Readme githubv4.String
                    URL    githubv4.URI
                } `graphql:"projectV2(number: $projectNumber)"`
            } `graphql:"user(login: $owner)"`
        }
        
        variables := map[string]any{
            "owner":         githubv4.String(owner),
            "projectNumber": githubv4.Int(number),
        }

        if err := gqlClient.Query(ctx, &q, variables); err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }

        // Check if the project was found
        if q.User.ProjectV2.Title == "" {
            return mcp.NewToolResultError(fmt.Sprintf("Could not find project number %d for user '%s'.", number, owner)), nil
        }

        return MarshalledTextResult(q.User.ProjectV2), nil
    } else {
        var q struct {
            Organization struct {
                ProjectV2 struct {
                    ID     githubv4.ID
                    Title  githubv4.String
                    Number githubv4.Int
                    Readme githubv4.String
                    URL    githubv4.URI
                } `graphql:"projectV2(number: $projectNumber)"`
            } `graphql:"organization(login: $owner)"`
        }
        
        variables := map[string]any{
            "owner":         githubv4.String(owner),
            "projectNumber": githubv4.Int(number),
        }

        if err := gqlClient.Query(ctx, &q, variables); err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }

        // Check if the project was found
        if q.Organization.ProjectV2.Title == "" {
            return mcp.NewToolResultError(fmt.Sprintf("Could not find project number %d for organization '%s'.", number, owner)), nil
        }

        return MarshalledTextResult(q.Organization.ProjectV2), nil
    }
}

// Helper function to get project by name with pagination support
func getProjectByName(ctx context.Context, client interface{}, owner string, name string, ownerType string) (*mcp.CallToolResult, error) {
    type GraphQLClient interface {
        Query(ctx context.Context, q interface{}, variables map[string]interface{}) error
    }
    
    gqlClient := client.(GraphQLClient)
    
    if ownerType == "user" {
        var cursor *githubv4.String
        
        for {
            var q struct {
                User struct {
                    Projects struct {
                        Nodes []struct {
                            ID     githubv4.ID
                            Title  githubv4.String
                            Number githubv4.Int
                            Readme githubv4.String
                            URL    githubv4.URI
                        }
                        PageInfo struct {
                            HasNextPage bool
                            EndCursor   githubv4.String
                        }
                    } `graphql:"projectsV2(first: 100, after: $cursor)"`
                } `graphql:"user(login: $login)"`
            }
            
            variables := map[string]any{
                "login":  githubv4.String(owner),
                "cursor": cursor,
            }
            
            if err := gqlClient.Query(ctx, &q, variables); err != nil {
                return mcp.NewToolResultError(err.Error()), nil
            }
            
            // Search for project by name (case-insensitive exact match first)
            for _, project := range q.User.Projects.Nodes {
                if strings.EqualFold(string(project.Title), name) {
                    return MarshalledTextResult(project), nil
                }
            }
            
            // Check if we should continue to next page
            if !q.User.Projects.PageInfo.HasNextPage {
                break
            }
            cursor = &q.User.Projects.PageInfo.EndCursor
        }
        
        // If exact match not found, do a second pass with partial matching
        cursor = nil
        for {
            var q struct {
                User struct {
                    Projects struct {
                        Nodes []struct {
                            ID     githubv4.ID
                            Title  githubv4.String
                            Number githubv4.Int
                            Readme githubv4.String
                            URL    githubv4.URI
                        }
                        PageInfo struct {
                            HasNextPage bool
                            EndCursor   githubv4.String
                        }
                    } `graphql:"projectsV2(first: 100, after: $cursor)"`
                } `graphql:"user(login: $login)"`
            }
            
            variables := map[string]any{
                "login":  githubv4.String(owner),
                "cursor": cursor,
            }
            
            if err := gqlClient.Query(ctx, &q, variables); err != nil {
                return mcp.NewToolResultError(err.Error()), nil
            }
            
            // Search for project by partial name match
            for _, project := range q.User.Projects.Nodes {
                if strings.Contains(strings.ToLower(string(project.Title)), strings.ToLower(name)) {
                    return MarshalledTextResult(project), nil
                }
            }
            
            // Check if we should continue to next page
            if !q.User.Projects.PageInfo.HasNextPage {
                break
            }
            cursor = &q.User.Projects.PageInfo.EndCursor
        }
        
        return mcp.NewToolResultError(fmt.Sprintf("Could not find project with name '%s' for user '%s'.", name, owner)), nil
    } else {
        var cursor *githubv4.String
        
        // First pass: exact match
        for {
            var q struct {
                Organization struct {
                    Projects struct {
                        Nodes []struct {
                            ID     githubv4.ID
                            Title  githubv4.String
                            Number githubv4.Int
                            Readme githubv4.String
                            URL    githubv4.URI
                        }
                        PageInfo struct {
                            HasNextPage bool
                            EndCursor   githubv4.String
                        }
                    } `graphql:"projectsV2(first: 100, after: $cursor)"`
                } `graphql:"organization(login: $login)"`
            }
            
            variables := map[string]any{
                "login":  githubv4.String(owner),
                "cursor": cursor,
            }
            
            if err := gqlClient.Query(ctx, &q, variables); err != nil {
                return mcp.NewToolResultError(err.Error()), nil
            }
            
            // Search for project by name (case-insensitive exact match first)
            for _, project := range q.Organization.Projects.Nodes {
                if strings.EqualFold(string(project.Title), name) {
                    return MarshalledTextResult(project), nil
                }
            }
            
            // Check if we should continue to next page
            if !q.Organization.Projects.PageInfo.HasNextPage {
                break
            }
            cursor = &q.Organization.Projects.PageInfo.EndCursor
        }
        
        // Second pass: partial match
        cursor = nil
        for {
            var q struct {
                Organization struct {
                    Projects struct {
                        Nodes []struct {
                            ID     githubv4.ID
                            Title  githubv4.String
                            Number githubv4.Int
                            Readme githubv4.String
                            URL    githubv4.URI
                        }
                        PageInfo struct {
                            HasNextPage bool
                            EndCursor   githubv4.String
                        }
                    } `graphql:"projectsV2(first: 100, after: $cursor)"`
                } `graphql:"organization(login: $login)"`
            }
            
            variables := map[string]any{
                "login":  githubv4.String(owner),
                "cursor": cursor,
            }
            
            if err := gqlClient.Query(ctx, &q, variables); err != nil {
                return mcp.NewToolResultError(err.Error()), nil
            }
            
            // Search for project by partial name match
            for _, project := range q.Organization.Projects.Nodes {
                if strings.Contains(strings.ToLower(string(project.Title)), strings.ToLower(name)) {
                    return MarshalledTextResult(project), nil
                }
            }
            
            // Check if we should continue to next page
            if !q.Organization.Projects.PageInfo.HasNextPage {
                break
            }
            cursor = &q.Organization.Projects.PageInfo.EndCursor
        }
        
        return mcp.NewToolResultError(fmt.Sprintf("Could not find project with name '%s' for organization '%s'.", name, owner)), nil
    }
}

// GetProjectStatuses retrieves the Status field options for a specific GitHub ProjectV2.
// It returns the status options with their IDs, names, and descriptions.
func GetProjectStatuses(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
    return mcp.NewTool("get_project_statuses",
            mcp.WithDescription(t("TOOL_GET_PROJECT_STATUSES_DESCRIPTION", "Get status field options for a project")),
            mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_GET_PROJECT_STATUSES_TITLE", "Get project statuses"), ReadOnlyHint: ToBoolPtr(true)}),
            mcp.WithString("project_id", mcp.Required(), mcp.Description("The global node ID of the project (e.g., 'PVT_kwDOA_dmc84A7u-a')")),
        ), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            projectID, err := RequiredParam[string](req, "project_id")
            if err != nil {
                return mcp.NewToolResultError(err.Error()), nil
            }

            client, err := getClient(ctx)
            if err != nil {
                return mcp.NewToolResultError(err.Error()), nil
            }

            // This struct defines the shape of the GraphQL query.
            // It fetches the Status field for a ProjectV2 by ID.
            var q struct {
                Node struct {
                    ProjectV2 struct {
                        Field struct {
                            ProjectV2SingleSelectField struct {
                                ID   githubv4.ID
                                Name githubv4.String
                                Options []struct {
                                    ID          githubv4.ID
                                    Name        githubv4.String
                                    Description githubv4.String
                                }
                            } `graphql:"... on ProjectV2SingleSelectField"`
                        } `graphql:"field(name: \"Status\")"`
                    } `graphql:"... on ProjectV2"`
                } `graphql:"node(id: $projectId)"`
            }

            variables := map[string]any{
                "projectId": githubv4.ID(projectID),
            }

            if err := client.Query(ctx, &q, variables); err != nil {
                // Provide a more helpful error message if the ID is malformed.
                if err.Error() == "Could not resolve to a node with the global id of '"+projectID+"'" {
                    return mcp.NewToolResultError(fmt.Sprintf("Invalid project_id: '%s'. Please provide a valid global node ID for a project.", projectID)), nil
                }
                return mcp.NewToolResultError(err.Error()), nil
            }

            // Check if the Status field exists and has options
            statusField := q.Node.ProjectV2.Field.ProjectV2SingleSelectField
            if statusField.Name == "" {
                return mcp.NewToolResultError(fmt.Sprintf("Could not find a Status field for project with ID '%s'. The project might not have a Status field configured.", projectID)), nil
            }

            return MarshalledTextResult(statusField), nil
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
									ProjectV2Field struct {
										ID       githubv4.ID
										Name     githubv4.String
										DataType githubv4.String
									} `graphql:"... on ProjectV2Field"`
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
								ProjectV2Field struct {
									ID       githubv4.ID
									Name     githubv4.String
									DataType githubv4.String
								} `graphql:"... on ProjectV2Field"`
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

// FieldNameFragment defines the fields we want from a ProjectV2FieldCommon interface.
type FieldNameFragment struct {
	Name githubv4.String
}

// Field represents the 'field' interface on a project item's field value.
// It uses an embedded struct with a graphql tag to act as an inline fragment.
type Field struct {
	OnProjectV2FieldCommon FieldNameFragment `graphql:"... on ProjectV2FieldCommon"`
}

// ProjectItem defines the structure of a single item within a project,
// including its field values and content.
type ProjectItem struct {
	ID          githubv4.ID
	FieldValues struct {
		Nodes []struct {
			TypeName string `graphql:"__typename"`
			// Fragment for Text values
			OnTextValue struct {
				Text  githubv4.String
				Field Field
			} `graphql:"... on ProjectV2ItemFieldTextValue"`
			// Fragment for Date values
			OnDateValue struct {
				Date  githubv4.DateTime
				Field Field
			} `graphql:"... on ProjectV2ItemFieldDateValue"`
			// Fragment for Single Select values
			OnSingleSelectValue struct {
				Name  githubv4.String
				Field Field
			} `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
		}
	} `graphql:"fieldValues(first: 8)"`
	Content struct {
		TypeName string `graphql:"__typename"`
		OnDraftIssue struct {
			Title githubv4.String
			Body  githubv4.String
		} `graphql:"... on DraftIssue"`
		OnIssue struct {
			Title     githubv4.String
			Assignees struct {
				Nodes []struct {
					Login githubv4.String
				}
			} `graphql:"assignees(first: 10)"`
		} `graphql:"... on Issue"`
		OnPullRequest struct {
			Title     githubv4.String
			Assignees struct {
				Nodes []struct {
					Login githubv4.String
				}
			} `graphql:"assignees(first: 10)"`
		} `graphql:"... on PullRequest"`
	}
}

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

		variables := map[string]any{
			"login":  githubv4.String(owner),
			"number": githubv4.Int(number),
		}

		if ownerType == "user" {
			var q struct {
				User struct {
					Project struct {
						Items struct {
							Nodes []ProjectItem
						} `graphql:"items(first: 100)"`
					} `graphql:"projectV2(number: $number)"`
				} `graphql:"user(login: $login)"`
			}
			if err := client.Query(ctx, &q, variables); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return MarshalledTextResult(q.User.Project.Items), nil
		}

		// This code is now reachable and syntactically correct.
		var q struct {
			Organization struct {
				Project struct {
					Items struct {
						Nodes []ProjectItem
					} `graphql:"items(first: 100)"`
				} `graphql:"projectV2(number: $number)"`
			} `graphql:"organization(login: $login)"`
		}
		if err := client.Query(ctx, &q, variables); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return MarshalledTextResult(q.Organization.Project.Items), nil
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

// UpdateProjectItemStatus updates the status field of a project item, allowing items to be moved between columns.
func UpdateProjectItemStatus(getClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	return mcp.NewTool("update_project_item_status",
		mcp.WithDescription(t("TOOL_UPDATE_PROJECT_ITEM_STATUS_DESCRIPTION", "Update a project item's status to move it between columns")),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{Title: t("TOOL_UPDATE_PROJECT_ITEM_STATUS_USER_TITLE", "Update project item status"), ReadOnlyHint: ToBoolPtr(false)}),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
		mcp.WithString("item_id", mcp.Required(), mcp.Description("Item ID")),
		mcp.WithString("status_option_id", mcp.Required(), mcp.Description("Status option ID (use get_project_statuses to find available options)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID, err := RequiredParam[string](req, "project_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		itemID, err := RequiredParam[string](req, "item_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		statusOptionID, err := RequiredParam[string](req, "status_option_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, err := getClient(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var statusFieldQuery struct {
			Node struct {
				ProjectV2 struct {
					Field struct {
						ProjectV2SingleSelectField struct {
							ID      githubv4.ID
							Name    githubv4.String
							Options []struct {
								ID   githubv4.ID
								Name githubv4.String
							}
						} `graphql:"... on ProjectV2SingleSelectField"`
					} `graphql:"field(name: \"Status\")"`
				} `graphql:"... on ProjectV2"`
			} `graphql:"node(id: $projectId)"`
		}

		variables := map[string]any{
			"projectId": githubv4.ID(projectID),
		}

		if err := client.Query(ctx, &statusFieldQuery, variables); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get Status field for project: %s", err.Error())), nil
		}

		statusField := statusFieldQuery.Node.ProjectV2.Field.ProjectV2SingleSelectField
		if statusField.ID == "" {
			return mcp.NewToolResultError(fmt.Sprintf("Could not find a Status field for project with ID '%s'. The project might not have a Status field configured.", projectID)), nil
		}

		// Validate that the provided status option ID exists
		var validOption bool
		var optionName string
		for _, option := range statusField.Options {
			if option.ID.(string) == statusOptionID {
				validOption = true
				optionName = string(option.Name)
				break
			}
		}

		if !validOption {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid status_option_id '%s' for project '%s'. Use get_project_statuses to see available options.", statusOptionID, projectID)), nil
		}

		val := githubv4.ProjectV2FieldValue{
			SingleSelectOptionID: githubv4.NewString(githubv4.String(statusOptionID)),
		}

		var mut struct {
			UpdateProjectV2ItemFieldValue struct {
				ProjectV2Item struct {
					ID githubv4.ID
				} `graphql:"projectV2Item"`
			} `graphql:"updateProjectV2ItemFieldValue(input: $input)"`
		}

		input := githubv4.UpdateProjectV2ItemFieldValueInput{
			ProjectID: githubv4.ID(projectID),
			ItemID:    githubv4.ID(itemID),
			FieldID:   statusField.ID,
			Value:     val,
		}

		if err := client.Mutate(ctx, &mut, input, nil); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update project item status: %s", err.Error())), nil
		}

		result := map[string]interface{}{
			"success":          true,
			"message":          fmt.Sprintf("Successfully updated item status to '%s'", optionName),
			"project_id":       projectID,
			"item_id":          itemID,
			"new_status":       optionName,
			"status_option_id": statusOptionID,
		}

		return MarshalledTextResult(result), nil
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

		input := githubv4.AddProjectV2DraftIssueInput{
			ProjectID: githubv4.ID(projectID),
			Title:     githubv4.String(title),
		}
		if body != "" {
			input.Body = githubv4.NewString(githubv4.String(body))
		}

		// CORRECTED: The payload field is 'projectItem', not 'item'.
		var mut struct {
			AddProjectV2DraftIssue struct {
				ProjectItem struct {
					ID githubv4.ID
				}
			} `graphql:"addProjectV2DraftIssue(input: $input)"`
		}

		// The library requires a pointer to the mutation struct, the input, and variables (nil in this case).
		if err := client.Mutate(ctx, &mut, input, nil); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return MarshalledTextResult(mut.AddProjectV2DraftIssue.ProjectItem), nil
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
			DeleteProjectV2Item struct {
				DeletedItemID githubv4.ID `graphql:"deletedItemId"`
			} `graphql:"deleteProjectV2Item(input: $input)"`
		}

		input := githubv4.DeleteProjectV2ItemInput{
			ProjectID: githubv4.ID(projectID),
			ItemID:    githubv4.ID(itemID),
		}

		if err := client.Mutate(ctx, &mut, input, nil); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return MarshalledTextResult(mut.DeleteProjectV2Item), nil
	}
}

