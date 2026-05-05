package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

const DefaultGraphQLPageSize = 30

// Common interface for all discussion query types
type DiscussionQueryResult interface {
	GetDiscussionFragment() DiscussionFragment
}

// Implement the interface for all query types
func (q *BasicNoOrder) GetDiscussionFragment() DiscussionFragment {
	return q.Repository.Discussions
}

func (q *BasicWithOrder) GetDiscussionFragment() DiscussionFragment {
	return q.Repository.Discussions
}

func (q *WithCategoryAndOrder) GetDiscussionFragment() DiscussionFragment {
	return q.Repository.Discussions
}

func (q *WithCategoryNoOrder) GetDiscussionFragment() DiscussionFragment {
	return q.Repository.Discussions
}

type DiscussionFragment struct {
	Nodes      []NodeFragment
	PageInfo   PageInfoFragment
	TotalCount githubv4.Int
}

type NodeFragment struct {
	Number         githubv4.Int
	Title          githubv4.String
	CreatedAt      githubv4.DateTime
	UpdatedAt      githubv4.DateTime
	Closed         githubv4.Boolean
	IsAnswered     githubv4.Boolean
	AnswerChosenAt *githubv4.DateTime
	Author         struct {
		Login githubv4.String
	}
	Category struct {
		Name githubv4.String
	} `graphql:"category"`
	URL githubv4.String `graphql:"url"`
}

type PageInfoFragment struct {
	HasNextPage     bool
	HasPreviousPage bool
	StartCursor     githubv4.String
	EndCursor       githubv4.String
}

type BasicNoOrder struct {
	Repository struct {
		Discussions DiscussionFragment `graphql:"discussions(first: $first, after: $after)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type BasicWithOrder struct {
	Repository struct {
		Discussions DiscussionFragment `graphql:"discussions(first: $first, after: $after, orderBy: { field: $orderByField, direction: $orderByDirection })"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type WithCategoryAndOrder struct {
	Repository struct {
		Discussions DiscussionFragment `graphql:"discussions(first: $first, after: $after, categoryId: $categoryId, orderBy: { field: $orderByField, direction: $orderByDirection })"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type WithCategoryNoOrder struct {
	Repository struct {
		Discussions DiscussionFragment `graphql:"discussions(first: $first, after: $after, categoryId: $categoryId)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

func fragmentToDiscussion(fragment NodeFragment) *github.Discussion {
	return &github.Discussion{
		Number:    github.Ptr(int(fragment.Number)),
		Title:     github.Ptr(string(fragment.Title)),
		HTMLURL:   github.Ptr(string(fragment.URL)),
		CreatedAt: &github.Timestamp{Time: fragment.CreatedAt.Time},
		UpdatedAt: &github.Timestamp{Time: fragment.UpdatedAt.Time},
		User: &github.User{
			Login: github.Ptr(string(fragment.Author.Login)),
		},
		DiscussionCategory: &github.DiscussionCategory{
			Name: github.Ptr(string(fragment.Category.Name)),
		},
	}
}

func getQueryType(useOrdering bool, categoryID *githubv4.ID) any {
	if categoryID != nil && useOrdering {
		return &WithCategoryAndOrder{}
	}
	if categoryID != nil && !useOrdering {
		return &WithCategoryNoOrder{}
	}
	if categoryID == nil && useOrdering {
		return &BasicWithOrder{}
	}
	return &BasicNoOrder{}
}

func ListDiscussions(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDiscussions,
		mcp.Tool{
			Name:        "list_discussions",
			Description: t("TOOL_LIST_DISCUSSIONS_DESCRIPTION", "List discussions for a repository or organisation."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_DISCUSSIONS_USER_TITLE", "List discussions"),
				ReadOnlyHint: true,
			},
			InputSchema: WithCursorPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name. If not provided, discussions will be queried at the organisation level.",
					},
					"category": {
						Type:        "string",
						Description: "Optional filter by discussion category ID. If provided, only discussions with this category are listed.",
					},
					"orderBy": {
						Type:        "string",
						Description: "Order discussions by field. If provided, the 'direction' also needs to be provided.",
						Enum:        []any{"CREATED_AT", "UPDATED_AT"},
					},
					"direction": {
						Type:        "string",
						Description: "Order direction.",
						Enum:        []any{"ASC", "DESC"},
					},
				},
				Required: []string{"owner"},
			}),
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := OptionalParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			// when not provided, default to the .github repository
			// this will query discussions at the organisation level
			if repo == "" {
				repo = ".github"
			}

			category, err := OptionalParam[string](args, "category")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			orderBy, err := OptionalParam[string](args, "orderBy")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			direction, err := OptionalParam[string](args, "direction")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get pagination parameters and convert to GraphQL format
			pagination, err := OptionalCursorPaginationParams(args)
			if err != nil {
				return nil, nil, err
			}
			paginationParams, err := pagination.ToGraphQLParams()
			if err != nil {
				return nil, nil, err
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			var categoryID *githubv4.ID
			if category != "" {
				id := githubv4.ID(category)
				categoryID = &id
			}

			vars := map[string]any{
				"owner": githubv4.String(owner),
				"repo":  githubv4.String(repo),
				"first": githubv4.Int(*paginationParams.First),
			}
			if paginationParams.After != nil {
				vars["after"] = githubv4.String(*paginationParams.After)
			} else {
				vars["after"] = (*githubv4.String)(nil)
			}

			// this is an extra check in case the tool description is misinterpreted, because
			// we shouldn't use ordering unless both a 'field' and 'direction' are provided
			useOrdering := orderBy != "" && direction != ""
			if useOrdering {
				vars["orderByField"] = githubv4.DiscussionOrderField(orderBy)
				vars["orderByDirection"] = githubv4.OrderDirection(direction)
			}

			if categoryID != nil {
				vars["categoryId"] = *categoryID
			}

			discussionQuery := getQueryType(useOrdering, categoryID)
			if err := client.Query(ctx, discussionQuery, vars); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Extract and convert all discussion nodes using the common interface
			var discussions []*github.Discussion
			var pageInfo PageInfoFragment
			var totalCount githubv4.Int
			if queryResult, ok := discussionQuery.(DiscussionQueryResult); ok {
				fragment := queryResult.GetDiscussionFragment()
				for _, node := range fragment.Nodes {
					discussions = append(discussions, fragmentToDiscussion(node))
				}
				pageInfo = fragment.PageInfo
				totalCount = fragment.TotalCount
			}

			// Create response with pagination info
			response := map[string]any{
				"discussions": discussions,
				"pageInfo": map[string]any{
					"hasNextPage":     pageInfo.HasNextPage,
					"hasPreviousPage": pageInfo.HasPreviousPage,
					"startCursor":     string(pageInfo.StartCursor),
					"endCursor":       string(pageInfo.EndCursor),
				},
				"totalCount": totalCount,
			}

			out, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal discussions: %w", err)
			}
			return utils.NewToolResultText(string(out)), nil, nil
		},
	)
}

func GetDiscussion(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDiscussions,
		mcp.Tool{
			Name:        "get_discussion",
			Description: t("TOOL_GET_DISCUSSION_DESCRIPTION", "Get a specific discussion by ID"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_DISCUSSION_USER_TITLE", "Get discussion"),
				ReadOnlyHint: true,
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
					"discussionNumber": {
						Type:        "number",
						Description: "Discussion Number",
					},
				},
				Required: []string{"owner", "repo", "discussionNumber"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			// Decode params
			var params struct {
				Owner            string
				Repo             string
				DiscussionNumber int32
			}
			if err := mapstructure.WeakDecode(args, &params); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			var q struct {
				Repository struct {
					Discussion struct {
						Number         githubv4.Int
						Title          githubv4.String
						Body           githubv4.String
						CreatedAt      githubv4.DateTime
						Closed         githubv4.Boolean
						IsAnswered     githubv4.Boolean
						AnswerChosenAt *githubv4.DateTime
						URL            githubv4.String `graphql:"url"`
						Category       struct {
							Name githubv4.String
						} `graphql:"category"`
					} `graphql:"discussion(number: $discussionNumber)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}
			vars := map[string]any{
				"owner":            githubv4.String(params.Owner),
				"repo":             githubv4.String(params.Repo),
				"discussionNumber": githubv4.Int(params.DiscussionNumber),
			}
			if err := client.Query(ctx, &q, vars); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			d := q.Repository.Discussion

			// Build response as map to include fields not present in go-github's Discussion struct.
			// The go-github library's Discussion type lacks isAnswered and answerChosenAt fields,
			// so we use map[string]interface{} for the response (consistent with other functions
			// like ListDiscussions and GetDiscussionComments).
			response := map[string]any{
				"number":     int(d.Number),
				"title":      string(d.Title),
				"body":       string(d.Body),
				"url":        string(d.URL),
				"closed":     bool(d.Closed),
				"isAnswered": bool(d.IsAnswered),
				"createdAt":  d.CreatedAt.Time,
				"category": map[string]any{
					"name": string(d.Category.Name),
				},
			}

			// Add optional timestamp fields if present
			if d.AnswerChosenAt != nil {
				response["answerChosenAt"] = d.AnswerChosenAt.Time
			}

			out, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal discussion: %w", err)
			}

			return utils.NewToolResultText(string(out)), nil, nil
		},
	)
}

func GetDiscussionComments(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDiscussions,
		mcp.Tool{
			Name:        "get_discussion_comments",
			Description: t("TOOL_GET_DISCUSSION_COMMENTS_DESCRIPTION", "Get comments from a discussion"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_DISCUSSION_COMMENTS_USER_TITLE", "Get discussion comments"),
				ReadOnlyHint: true,
			},
			InputSchema: WithCursorPagination(&jsonschema.Schema{
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
					"discussionNumber": {
						Type:        "number",
						Description: "Discussion Number",
					},
					"includeReplies": {
						Type:        "boolean",
						Description: "When true, each top-level comment will include its replies nested within it (up to 100 replies per comment, which is the GitHub API maximum). Defaults to false.",
					},
				},
				Required: []string{"owner", "repo", "discussionNumber"},
			}),
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			// Decode params
			var params struct {
				Owner            string
				Repo             string
				DiscussionNumber int32
			}
			if err := mapstructure.WeakDecode(args, &params); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			includeReplies, err := OptionalParam[bool](args, "includeReplies")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get pagination parameters and convert to GraphQL format
			pagination, err := OptionalCursorPaginationParams(args)
			if err != nil {
				return nil, nil, err
			}

			// Check if pagination parameters were explicitly provided
			_, perPageProvided := args["perPage"]
			paginationExplicit := perPageProvided

			paginationParams, err := pagination.ToGraphQLParams()
			if err != nil {
				return nil, nil, err
			}

			// Use default of 30 if pagination was not explicitly provided
			if !paginationExplicit {
				defaultFirst := int32(DefaultGraphQLPageSize)
				paginationParams.First = &defaultFirst
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			vars := map[string]any{
				"owner":            githubv4.String(params.Owner),
				"repo":             githubv4.String(params.Repo),
				"discussionNumber": githubv4.Int(params.DiscussionNumber),
				"first":            githubv4.Int(*paginationParams.First),
			}
			if paginationParams.After != nil {
				vars["after"] = githubv4.String(*paginationParams.After)
			} else {
				vars["after"] = (*githubv4.String)(nil)
			}

			var comments []MinimalDiscussionComment
			var pageInfo struct {
				HasNextPage     githubv4.Boolean
				HasPreviousPage githubv4.Boolean
				StartCursor     githubv4.String
				EndCursor       githubv4.String
			}
			var totalCount int

			if includeReplies {
				var q struct {
					Repository struct {
						Discussion struct {
							Comments struct {
								Nodes []struct {
									ID       githubv4.ID
									Body     githubv4.String
									IsAnswer githubv4.Boolean
									Replies  struct {
										Nodes []struct {
											ID   githubv4.ID
											Body githubv4.String
										}
										TotalCount int
									} `graphql:"replies(first: 100)"`
								}
								PageInfo struct {
									HasNextPage     githubv4.Boolean
									HasPreviousPage githubv4.Boolean
									StartCursor     githubv4.String
									EndCursor       githubv4.String
								}
								TotalCount int
							} `graphql:"comments(first: $first, after: $after)"`
						} `graphql:"discussion(number: $discussionNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}
				if err := client.Query(ctx, &q, vars); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				for _, c := range q.Repository.Discussion.Comments.Nodes {
					comment := MinimalDiscussionComment{
						ID:              fmt.Sprintf("%v", c.ID),
						Body:            string(c.Body),
						IsAnswer:        bool(c.IsAnswer),
						ReplyTotalCount: c.Replies.TotalCount,
					}
					for _, r := range c.Replies.Nodes {
						comment.Replies = append(comment.Replies, MinimalDiscussionComment{
							ID:   fmt.Sprintf("%v", r.ID),
							Body: string(r.Body),
						})
					}
					comments = append(comments, comment)
				}
				pageInfo = q.Repository.Discussion.Comments.PageInfo
				totalCount = q.Repository.Discussion.Comments.TotalCount
			} else {
				var q struct {
					Repository struct {
						Discussion struct {
							Comments struct {
								Nodes []struct {
									ID       githubv4.ID
									Body     githubv4.String
									IsAnswer githubv4.Boolean
								}
								PageInfo struct {
									HasNextPage     githubv4.Boolean
									HasPreviousPage githubv4.Boolean
									StartCursor     githubv4.String
									EndCursor       githubv4.String
								}
								TotalCount int
							} `graphql:"comments(first: $first, after: $after)"`
						} `graphql:"discussion(number: $discussionNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}
				if err := client.Query(ctx, &q, vars); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				for _, c := range q.Repository.Discussion.Comments.Nodes {
					comments = append(comments, MinimalDiscussionComment{
						ID:       fmt.Sprintf("%v", c.ID),
						Body:     string(c.Body),
						IsAnswer: bool(c.IsAnswer),
					})
				}
				pageInfo = q.Repository.Discussion.Comments.PageInfo
				totalCount = q.Repository.Discussion.Comments.TotalCount
			}

			// Create response with pagination info
			response := map[string]any{
				"comments": comments,
				"pageInfo": map[string]any{
					"hasNextPage":     pageInfo.HasNextPage,
					"hasPreviousPage": pageInfo.HasPreviousPage,
					"startCursor":     string(pageInfo.StartCursor),
					"endCursor":       string(pageInfo.EndCursor),
				},
				"totalCount": totalCount,
			}

			out, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal comments: %w", err)
			}

			return utils.NewToolResultText(string(out)), nil, nil
		},
	)
}

func AddDiscussionComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDiscussions,
		mcp.Tool{
			Name:        "add_discussion_comment",
			Description: t("TOOL_ADD_DISCUSSION_COMMENT_DESCRIPTION", "Add a comment to a discussion"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ADD_DISCUSSION_COMMENT_USER_TITLE", "Add discussion comment"),
				ReadOnlyHint: false,
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
					"discussionNumber": {
						Type:        "number",
						Description: "Discussion Number",
					},
					"body": {
						Type:        "string",
						Description: "Comment content",
					},
					"replyToCommentNodeID": {
						Type:        "string",
						Description: "The Node ID of the comment to reply to. If provided, the comment will be posted as a reply.",
					},
				},
				Required: []string{"owner", "repo", "discussionNumber", "body"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			discussionNumber, err := RequiredInt(args, "discussionNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			body, err := RequiredParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			// Get the discussion's node ID using its number
			var q struct {
				Repository struct {
					Discussion struct {
						ID githubv4.ID
					} `graphql:"discussion(number: $discussionNumber)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}
			vars := map[string]any{
				"owner":            githubv4.String(owner),
				"repo":             githubv4.String(repo),
				"discussionNumber": githubv4.Int(discussionNumber), // #nosec G115 - discussion numbers are always small positive integers
			}
			if err := client.Query(ctx, &q, vars); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Add the comment using the discussion's node ID
			input := githubv4.AddDiscussionCommentInput{
				DiscussionID: q.Repository.Discussion.ID,
				Body:         githubv4.String(body),
			}

			replyToCommentNodeID, err := OptionalParam[string](args, "replyToCommentNodeID")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if replyToCommentNodeID != "" {
				if strings.TrimSpace(replyToCommentNodeID) == "" {
					return utils.NewToolResultError("replyToCommentNodeID cannot be blank"), nil, nil
				}
				// The GitHub API silently ignores an invalid ReplyToID and creates a top-level
				// comment instead of returning an error, so we validate upfront that the node
				// exists and is a DiscussionComment to give callers a clear failure.
				var nodeQuery struct {
					Node struct {
						DiscussionComment struct {
							ID githubv4.ID
						} `graphql:"... on DiscussionComment"`
					} `graphql:"node(id: $replyToID)"`
				}
				if err := client.Query(ctx, &nodeQuery, map[string]any{
					"replyToID": githubv4.ID(replyToCommentNodeID),
				}); err != nil {
					return utils.NewToolResultError(fmt.Sprintf("failed to validate replyToCommentNodeID: %v", err)), nil, nil
				}
				if nodeQuery.Node.DiscussionComment.ID == nil || nodeQuery.Node.DiscussionComment.ID == "" {
					return utils.NewToolResultError(fmt.Sprintf("replyToCommentNodeID %q does not resolve to a valid discussion comment", replyToCommentNodeID)), nil, nil
				}
				id := githubv4.ID(replyToCommentNodeID)
				input.ReplyToID = &id
			}

			var mutation struct {
				AddDiscussionComment struct {
					Comment struct {
						ID  githubv4.ID
						URL githubv4.String `graphql:"url"`
					}
				} `graphql:"addDiscussionComment(input: $input)"`
			}

			if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			comment := mutation.AddDiscussionComment.Comment
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%v", comment.ID),
				URL: string(comment.URL),
			}

			out, err := json.Marshal(minimalResponse)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal comment: %w", err)
			}

			return utils.NewToolResultText(string(out)), nil, nil
		})
}

func UpdateDiscussionComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDiscussions,
		mcp.Tool{
			Name:        "update_discussion_comment",
			Description: t("TOOL_UPDATE_DISCUSSION_COMMENT_DESCRIPTION", "Update a comment on a discussion"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_DISCUSSION_COMMENT_USER_TITLE", "Update discussion comment"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"commentNodeID": {
						Type:        "string",
						Description: "The Node ID of the discussion comment to update",
					},
					"body": {
						Type:        "string",
						Description: "The new contents of the comment",
					},
				},
				Required: []string{"commentNodeID", "body"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			commentNodeID, err := RequiredParam[string](args, "commentNodeID")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			body, err := RequiredParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			input := githubv4.UpdateDiscussionCommentInput{
				CommentID: githubv4.ID(commentNodeID),
				Body:      githubv4.String(body),
			}

			var mutation struct {
				UpdateDiscussionComment struct {
					Comment struct {
						ID  githubv4.ID
						URL githubv4.String `graphql:"url"`
					}
				} `graphql:"updateDiscussionComment(input: $input)"`
			}

			if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			comment := mutation.UpdateDiscussionComment.Comment
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%v", comment.ID),
				URL: string(comment.URL),
			}

			out, err := json.Marshal(minimalResponse)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal comment: %w", err)
			}

			return utils.NewToolResultText(string(out)), nil, nil
		})
}

func DeleteDiscussionComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDiscussions,
		mcp.Tool{
			Name:        "delete_discussion_comment",
			Description: t("TOOL_DELETE_DISCUSSION_COMMENT_DESCRIPTION", "Delete a comment on a discussion"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_DELETE_DISCUSSION_COMMENT_USER_TITLE", "Delete discussion comment"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"commentNodeID": {
						Type:        "string",
						Description: "The Node ID of the discussion comment to delete",
					},
				},
				Required: []string{"commentNodeID"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			commentNodeID, err := RequiredParam[string](args, "commentNodeID")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			input := githubv4.DeleteDiscussionCommentInput{
				ID: githubv4.ID(commentNodeID),
			}

			var mutation struct {
				DeleteDiscussionComment struct {
					Comment struct {
						ID  githubv4.ID
						URL githubv4.String `graphql:"url"`
					}
				} `graphql:"deleteDiscussionComment(input: $input)"`
			}

			if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			comment := mutation.DeleteDiscussionComment.Comment
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%v", comment.ID),
				URL: string(comment.URL),
			}

			out, err := json.Marshal(minimalResponse)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal comment: %w", err)
			}

			return utils.NewToolResultText(string(out)), nil, nil
		})
}

func SetDiscussionCommentAnswer(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDiscussions,
		mcp.Tool{
			Name:        "set_discussion_comment_answer",
			Description: t("TOOL_SET_DISCUSSION_COMMENT_ANSWER_DESCRIPTION", "Marks or unmarks a given discussion comment as the answer to the discussion."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SET_DISCUSSION_COMMENT_ANSWER_USER_TITLE", "Set discussion comment as answer"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"commentNodeID": {
						Type:        "string",
						Description: "The Node ID of the discussion comment to mark or unmark as the answer",
					},
					"isAnswer": {
						Type:        "boolean",
						Description: "Whether the comment is the answer to the discussion (true to mark, false to unmark)",
					},
				},
				Required: []string{"commentNodeID", "isAnswer"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			commentNodeID, err := RequiredParam[string](args, "commentNodeID")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if _, ok := args["isAnswer"]; !ok {
				return utils.NewToolResultError("missing required parameter: isAnswer"), nil, nil
			}
			isAnswer, err := OptionalParam[bool](args, "isAnswer")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			var discussionID githubv4.ID
			var discussionURL githubv4.String

			if isAnswer {
				input := githubv4.MarkDiscussionCommentAsAnswerInput{
					ID: githubv4.ID(commentNodeID),
				}
				var mutation struct {
					MarkDiscussionCommentAsAnswer struct {
						Discussion struct {
							ID  githubv4.ID
							URL githubv4.String `graphql:"url"`
						}
					} `graphql:"markDiscussionCommentAsAnswer(input: $input)"`
				}
				if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				discussionID = mutation.MarkDiscussionCommentAsAnswer.Discussion.ID
				discussionURL = mutation.MarkDiscussionCommentAsAnswer.Discussion.URL
			} else {
				input := githubv4.UnmarkDiscussionCommentAsAnswerInput{
					ID: githubv4.ID(commentNodeID),
				}
				var mutation struct {
					UnmarkDiscussionCommentAsAnswer struct {
						Discussion struct {
							ID  githubv4.ID
							URL githubv4.String `graphql:"url"`
						}
					} `graphql:"unmarkDiscussionCommentAsAnswer(input: $input)"`
				}
				if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				discussionID = mutation.UnmarkDiscussionCommentAsAnswer.Discussion.ID
				discussionURL = mutation.UnmarkDiscussionCommentAsAnswer.Discussion.URL
			}

			response := struct {
				DiscussionID  string `json:"discussionID"`
				DiscussionURL string `json:"discussionURL"`
			}{
				DiscussionID:  fmt.Sprintf("%v", discussionID),
				DiscussionURL: string(discussionURL),
			}

			out, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal discussion: %w", err)
			}

			return utils.NewToolResultText(string(out)), nil, nil
		},
	)
}

func ListDiscussionCategories(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataDiscussions,
		mcp.Tool{
			Name:        "list_discussion_categories",
			Description: t("TOOL_LIST_DISCUSSION_CATEGORIES_DESCRIPTION", "List discussion categories with their id and name, for a repository or organisation."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_DISCUSSION_CATEGORIES_USER_TITLE", "List discussion categories"),
				ReadOnlyHint: true,
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
						Description: "Repository name. If not provided, discussion categories will be queried at the organisation level.",
					},
				},
				Required: []string{"owner"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := OptionalParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			// when not provided, default to the .github repository
			// this will query discussion categories at the organisation level
			if repo == "" {
				repo = ".github"
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			var q struct {
				Repository struct {
					DiscussionCategories struct {
						Nodes []struct {
							ID   githubv4.ID
							Name githubv4.String
						}
						PageInfo struct {
							HasNextPage     githubv4.Boolean
							HasPreviousPage githubv4.Boolean
							StartCursor     githubv4.String
							EndCursor       githubv4.String
						}
						TotalCount int
					} `graphql:"discussionCategories(first: $first)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}
			vars := map[string]any{
				"owner": githubv4.String(owner),
				"repo":  githubv4.String(repo),
				"first": githubv4.Int(25),
			}
			if err := client.Query(ctx, &q, vars); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var categories []map[string]string
			for _, c := range q.Repository.DiscussionCategories.Nodes {
				categories = append(categories, map[string]string{
					"id":   fmt.Sprint(c.ID),
					"name": string(c.Name),
				})
			}

			// Create response with pagination info
			response := map[string]any{
				"categories": categories,
				"pageInfo": map[string]any{
					"hasNextPage":     q.Repository.DiscussionCategories.PageInfo.HasNextPage,
					"hasPreviousPage": q.Repository.DiscussionCategories.PageInfo.HasPreviousPage,
					"startCursor":     string(q.Repository.DiscussionCategories.PageInfo.StartCursor),
					"endCursor":       string(q.Repository.DiscussionCategories.PageInfo.EndCursor),
				},
				"totalCount": q.Repository.DiscussionCategories.TotalCount,
			}

			out, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal discussion categories: %w", err)
			}
			return utils.NewToolResultText(string(out)), nil, nil
		},
	)
}
