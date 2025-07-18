package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/go-github/v73/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"
)

type DiscussionFragment struct {
	Number    githubv4.Int
	Title     githubv4.String
	CreatedAt githubv4.DateTime
	UpdatedAt githubv4.DateTime
	Author    struct {
		Login githubv4.String
	}
	Category struct {
		Name githubv4.String
	} `graphql:"category"`
	URL githubv4.String `graphql:"url"`
}

type BasicNoOrder struct {
	Repository struct {
		Discussions struct {
			Nodes []DiscussionFragment
		} `graphql:"discussions(first: 100)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type BasicWithOrder struct {
	Repository struct {
		Discussions struct {
			Nodes []DiscussionFragment
		} `graphql:"discussions(first: 100, orderBy: { field: $orderByField, direction: $orderByDirection })"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type WithCategoryAndOrder struct {
	Repository struct {
		Discussions struct {
			Nodes []DiscussionFragment
		} `graphql:"discussions(first: 100, categoryId: $categoryId, orderBy: { field: $orderByField, direction: $orderByDirection })"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type WithCategoryNoOrder struct {
	Repository struct {
		Discussions struct {
			Nodes []DiscussionFragment
		} `graphql:"discussions(first: 100, categoryId: $categoryId)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

func fragmentToDiscussion(fragment DiscussionFragment) *github.Discussion {
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

func ListDiscussions(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_discussions",
			mcp.WithDescription(t("TOOL_LIST_DISCUSSIONS_DESCRIPTION", "List discussions for a repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_DISCUSSIONS_USER_TITLE", "List discussions"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("category",
				mcp.Description("Optional filter by discussion category ID. If provided, only discussions with this category are listed."),
			),
			mcp.WithString("orderBy",
				mcp.Description("Order discussions by field. If provided, the 'direction' also needs to be provided."),
				mcp.Enum("CREATED_AT", "UPDATED_AT"),
			),
			mcp.WithString("direction",
				mcp.Description("Order direction."),
				mcp.Enum("ASC", "DESC"),
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

			category, err := OptionalParam[string](request, "category")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			orderBy, err := OptionalParam[string](request, "orderBy")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			direction, err := OptionalParam[string](request, "direction")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil
			}

			var categoryID *githubv4.ID
			if category != "" {
				id := githubv4.ID(category)
				categoryID = &id
			}

			vars := map[string]interface{}{
				"owner": githubv4.String(owner),
				"repo":  githubv4.String(repo),
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

			var discussions []*github.Discussion
			discussionQuery := getQueryType(useOrdering, categoryID)

			if err := client.Query(ctx, discussionQuery, vars); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// we need to check what user inputs we received at runtime, and use the
			// most appropriate query based on that
			switch queryType := discussionQuery.(type) {
			case *WithCategoryAndOrder:
				for _, node := range queryType.Repository.Discussions.Nodes {
					discussions = append(discussions, fragmentToDiscussion(node))
				}

			case *WithCategoryNoOrder:
				for _, node := range queryType.Repository.Discussions.Nodes {
					discussions = append(discussions, fragmentToDiscussion(node))
				}

			case *BasicWithOrder:
				for _, node := range queryType.Repository.Discussions.Nodes {
					discussions = append(discussions, fragmentToDiscussion(node))
				}

			case *BasicNoOrder:
				for _, node := range queryType.Repository.Discussions.Nodes {
					discussions = append(discussions, fragmentToDiscussion(node))
				}
			}

			out, err := json.Marshal(discussions)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal discussions: %w", err)
			}
			return mcp.NewToolResultText(string(out)), nil
		}
}

func GetDiscussion(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_discussion",
			mcp.WithDescription(t("TOOL_GET_DISCUSSION_DESCRIPTION", "Get a specific discussion by ID")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_DISCUSSION_USER_TITLE", "Get discussion"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("discussionNumber",
				mcp.Required(),
				mcp.Description("Discussion Number"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Decode params
			var params struct {
				Owner            string
				Repo             string
				DiscussionNumber int32
			}
			if err := mapstructure.Decode(request.Params.Arguments, &params); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil
			}

			var q struct {
				Repository struct {
					Discussion struct {
						Number    githubv4.Int
						Body      githubv4.String
						CreatedAt githubv4.DateTime
						URL       githubv4.String `graphql:"url"`
						Category  struct {
							Name githubv4.String
						} `graphql:"category"`
					} `graphql:"discussion(number: $discussionNumber)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}
			vars := map[string]interface{}{
				"owner":            githubv4.String(params.Owner),
				"repo":             githubv4.String(params.Repo),
				"discussionNumber": githubv4.Int(params.DiscussionNumber),
			}
			if err := client.Query(ctx, &q, vars); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			d := q.Repository.Discussion
			discussion := &github.Discussion{
				Number:    github.Ptr(int(d.Number)),
				Body:      github.Ptr(string(d.Body)),
				HTMLURL:   github.Ptr(string(d.URL)),
				CreatedAt: &github.Timestamp{Time: d.CreatedAt.Time},
				DiscussionCategory: &github.DiscussionCategory{
					Name: github.Ptr(string(d.Category.Name)),
				},
			}
			out, err := json.Marshal(discussion)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal discussion: %w", err)
			}

			return mcp.NewToolResultText(string(out)), nil
		}
}

func GetDiscussionComments(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_discussion_comments",
			mcp.WithDescription(t("TOOL_GET_DISCUSSION_COMMENTS_DESCRIPTION", "Get comments from a discussion")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_DISCUSSION_COMMENTS_USER_TITLE", "Get discussion comments"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner")),
			mcp.WithString("repo", mcp.Required(), mcp.Description("Repository name")),
			mcp.WithNumber("discussionNumber", mcp.Required(), mcp.Description("Discussion Number")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Decode params
			var params struct {
				Owner            string
				Repo             string
				DiscussionNumber int32
			}
			if err := mapstructure.Decode(request.Params.Arguments, &params); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil
			}

			var q struct {
				Repository struct {
					Discussion struct {
						Comments struct {
							Nodes []struct {
								Body githubv4.String
							}
						} `graphql:"comments(first:100)"`
					} `graphql:"discussion(number: $discussionNumber)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}
			vars := map[string]interface{}{
				"owner":            githubv4.String(params.Owner),
				"repo":             githubv4.String(params.Repo),
				"discussionNumber": githubv4.Int(params.DiscussionNumber),
			}
			if err := client.Query(ctx, &q, vars); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			var comments []*github.IssueComment
			for _, c := range q.Repository.Discussion.Comments.Nodes {
				comments = append(comments, &github.IssueComment{Body: github.Ptr(string(c.Body))})
			}

			out, err := json.Marshal(comments)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal comments: %w", err)
			}

			return mcp.NewToolResultText(string(out)), nil
		}
}

func ListDiscussionCategories(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_discussion_categories",
			mcp.WithDescription(t("TOOL_LIST_DISCUSSION_CATEGORIES_DESCRIPTION", "List discussion categories with their id and name, for a repository")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_DISCUSSION_CATEGORIES_USER_TITLE", "List discussion categories"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("first",
				mcp.Description("Number of categories to return per page (min 1, max 100)"),
				mcp.Min(1),
				mcp.Max(100),
			),
			mcp.WithNumber("last",
				mcp.Description("Number of categories to return from the end (min 1, max 100)"),
				mcp.Min(1),
				mcp.Max(100),
			),
			mcp.WithString("after",
				mcp.Description("Cursor for pagination, use the 'after' field from the previous response"),
			),
			mcp.WithString("before",
				mcp.Description("Cursor for pagination, use the 'before' field from the previous response"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Decode params
			var params struct {
				Owner  string
				Repo   string
				First  int32
				Last   int32
				After  string
				Before string
			}
			if err := mapstructure.Decode(request.Params.Arguments, &params); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Validate pagination parameters
			if params.First != 0 && params.Last != 0 {
				return mcp.NewToolResultError("only one of 'first' or 'last' may be specified"), nil
			}
			if params.After != "" && params.Before != "" {
				return mcp.NewToolResultError("only one of 'after' or 'before' may be specified"), nil
			}
			if params.After != "" && params.Last != 0 {
				return mcp.NewToolResultError("'after' cannot be used with 'last'. Did you mean to use 'before' instead?"), nil
			}
			if params.Before != "" && params.First != 0 {
				return mcp.NewToolResultError("'before' cannot be used with 'first'. Did you mean to use 'after' instead?"), nil
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil
			}
			var q struct {
				Repository struct {
					DiscussionCategories struct {
						Nodes []struct {
							ID   githubv4.ID
							Name githubv4.String
						}
					} `graphql:"discussionCategories(first: 100)"`
				} `graphql:"repository(owner: $owner, name: $repo)"`
			}
			vars := map[string]interface{}{
				"owner": githubv4.String(params.Owner),
				"repo":  githubv4.String(params.Repo),
			}
			if err := client.Query(ctx, &q, vars); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			var categories []map[string]string
			for _, c := range q.Repository.DiscussionCategories.Nodes {
				categories = append(categories, map[string]string{
					"id":   fmt.Sprint(c.ID),
					"name": string(c.Name),
				})
			}
			out, err := json.Marshal(categories)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal discussion categories: %w", err)
			}
			return mcp.NewToolResultText(string(out)), nil
		}
}
