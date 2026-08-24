package github

import (
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	listDiscussionsOutputQuery                  = "query($after:String$first:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussions(first: $first, after: $after){nodes{number,title,createdAt,updatedAt,closed,isAnswered,answerChosenAt,author{login},category{name},url},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}"
	listFilteredDiscussionsOutputQuery          = "query($after:String!$categoryId:ID!$first:Int!$orderByDirection:OrderDirection!$orderByField:DiscussionOrderField!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussions(first: $first, after: $after, categoryId: $categoryId, orderBy: { field: $orderByField, direction: $orderByDirection }){nodes{number,title,createdAt,updatedAt,closed,isAnswered,answerChosenAt,author{login},category{name},url},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}"
	getDiscussionOutputQuery                    = "query($discussionNumber:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussion(number: $discussionNumber){number,title,body,createdAt,closed,isAnswered,answerChosenAt,url,category{name}}}}"
	getDiscussionCommentsOutputQuery            = "query($after:String$discussionNumber:Int!$first:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussion(number: $discussionNumber){comments(first: $first, after: $after){nodes{id,body,isAnswer},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}}"
	getDiscussionCommentsWithRepliesOutputQuery = "query($after:String!$discussionNumber:Int!$first:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussion(number: $discussionNumber){comments(first: $first, after: $after){nodes{id,body,isAnswer,replies(first: 100){nodes{id,body,isAnswer},totalCount}},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}}"
	listDiscussionCategoriesOutputQuery         = "query($first:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussionCategories(first: $first){nodes{id,name},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}"
)

func TestListDiscussionsStructuredOutput(t *testing.T) {
	tool := ListDiscussions(translations.NullTranslationHelper)
	response := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussions": map[string]any{
				"nodes": []any{map[string]any{
					"number": 0, "title": "",
					"createdAt": "2026-08-24T12:00:00Z", "updatedAt": "2026-08-24T12:01:00Z",
					"closed": false, "isAnswered": false, "author": map[string]any{"login": ""},
					"url": "", "category": map[string]any{"name": ""},
				}},
				"pageInfo": map[string]any{
					"hasNextPage": false, "hasPreviousPage": false,
					"startCursor": "", "endCursor": "",
				},
				"totalCount": 1,
			},
		},
	})
	result := callRegisteredTool(t, tool, discussionOutputDeps(t, listDiscussionsOutputQuery,
		map[string]any{
			"owner": "owner", "repo": "repo", "first": float64(30), "after": (*string)(nil),
		}, response), map[string]any{"owner": "owner", "repo": "repo"})

	requireStructuredJSONAgreement(t, tool, result)
	assert.Equal(t, map[string]any{
		"discussions": []any{map[string]any{
			"number": float64(0), "title": "", "html_url": "",
			"created_at": "2026-08-24T12:00:00Z", "updated_at": "2026-08-24T12:01:00Z",
			"user": map[string]any{"login": ""}, "category": map[string]any{"name": ""},
		}},
		"pageInfo": map[string]any{
			"hasNextPage": false, "hasPreviousPage": false,
			"startCursor": "", "endCursor": "",
		},
		"totalCount": float64(1),
	}, result.StructuredContent)
}

func TestListDiscussionsStructuredEmptyFilteredPage(t *testing.T) {
	tool := ListDiscussions(translations.NullTranslationHelper)
	response := discussionConnectionResponse("discussions", []any{}, map[string]any{
		"hasNextPage": true, "hasPreviousPage": true,
		"startCursor": "start", "endCursor": "end",
	}, 0)
	result := callRegisteredTool(t, tool, discussionOutputDeps(t, listFilteredDiscussionsOutputQuery,
		map[string]any{
			"owner": "owner", "repo": "repo", "categoryId": "category",
			"orderByField": "UPDATED_AT", "orderByDirection": "DESC",
			"first": float64(1), "after": "cursor",
		}, response), map[string]any{
		"owner": "owner", "repo": "repo", "category": "category",
		"orderBy": "UPDATED_AT", "direction": "DESC", "perPage": 1, "after": "cursor",
	})

	requireStructuredJSONAgreement(t, tool, result)
	assert.Equal(t, map[string]any{
		"discussions": []any{},
		"pageInfo": map[string]any{
			"hasNextPage": true, "hasPreviousPage": true,
			"startCursor": "start", "endCursor": "end",
		},
		"totalCount": float64(0),
	}, result.StructuredContent)
}

func TestGetDiscussionStructuredOutputPreservesValues(t *testing.T) {
	tool := GetDiscussion(translations.NullTranslationHelper)
	response := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussion": map[string]any{
				"number": 0, "title": "", "body": "", "url": "",
				"createdAt": "2026-08-24T12:00:00Z", "closed": false, "isAnswered": false,
				"answerChosenAt": "2026-08-24T12:01:00Z", "category": map[string]any{"name": ""},
			},
		},
	})
	result := callRegisteredTool(t, tool, discussionOutputDeps(t, getDiscussionOutputQuery,
		discussionNumberVars(), response), discussionArgs())

	requireStructuredJSONAgreement(t, tool, result)
	assert.Equal(t, map[string]any{
		"number": float64(0), "title": "", "body": "", "url": "",
		"closed": false, "isAnswered": false, "createdAt": "2026-08-24T12:00:00Z",
		"answerChosenAt": "2026-08-24T12:01:00Z", "category": map[string]any{"name": ""},
	}, result.StructuredContent)
}

func TestGetDiscussionStructuredOutputOmitsAbsentOptionalFields(t *testing.T) {
	tool := GetDiscussion(translations.NullTranslationHelper)
	response := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussion": map[string]any{
				"number": 1, "title": "title", "body": "body", "url": "url",
				"createdAt": "2026-08-24T12:00:00Z", "closed": false, "isAnswered": false,
				"category": map[string]any{"name": "General"},
			},
		},
	})
	result := callRegisteredTool(t, tool, discussionOutputDeps(t, getDiscussionOutputQuery,
		discussionNumberVars(), response), discussionArgs())

	requireStructuredJSONAgreement(t, tool, result)
	assert.Equal(t, map[string]any{
		"number": float64(1), "title": "title", "body": "body", "url": "url",
		"closed": false, "isAnswered": false, "createdAt": "2026-08-24T12:00:00Z",
		"category": map[string]any{"name": "General"},
	}, result.StructuredContent)
}

func TestDiscussionCommentsStructuredReplySelection(t *testing.T) {
	t.Run("without replies", func(t *testing.T) {
		tool := GetDiscussionComments(translations.NullTranslationHelper)
		response := discussionCommentsResponse([]any{
			map[string]any{"id": "comment", "body": "", "isAnswer": false},
		}, map[string]any{
			"hasNextPage": false, "hasPreviousPage": false,
			"startCursor": "", "endCursor": "",
		}, 1)
		result := callRegisteredTool(t, tool, discussionOutputDeps(t, getDiscussionCommentsOutputQuery,
			discussionCommentVars(float64(30), (*string)(nil)), response), discussionArgs())

		requireStructuredJSONAgreement(t, tool, result)
		assert.Equal(t, map[string]any{
			"comments": []any{map[string]any{
				"id": "comment", "body": "", "isAnswer": false,
			}},
			"pageInfo": map[string]any{
				"hasNextPage": false, "hasPreviousPage": false,
				"startCursor": "", "endCursor": "",
			},
			"totalCount": float64(1),
		}, result.StructuredContent)
	})

	t.Run("with nested and empty replies", func(t *testing.T) {
		tool := GetDiscussionComments(translations.NullTranslationHelper)
		response := discussionCommentsResponse([]any{
			map[string]any{
				"id": "empty", "body": "", "isAnswer": false,
				"replies": map[string]any{"nodes": []any{}, "totalCount": 0},
			},
			map[string]any{
				"id": "nested", "body": "top", "isAnswer": true,
				"replies": map[string]any{
					"nodes": []any{map[string]any{
						"id": "reply", "body": "", "isAnswer": false,
					}},
					"totalCount": 1,
				},
			},
		}, map[string]any{
			"hasNextPage": true, "hasPreviousPage": false,
			"startCursor": "start", "endCursor": "end",
		}, 2)
		args := discussionArgs()
		args["includeReplies"] = true
		args["perPage"] = 1
		args["after"] = "cursor"
		result := callRegisteredTool(t, tool, discussionOutputDeps(t,
			getDiscussionCommentsWithRepliesOutputQuery,
			discussionCommentVars(float64(1), "cursor"), response), args)

		requireStructuredJSONAgreement(t, tool, result)
		assert.Equal(t, map[string]any{
			"comments": []any{
				map[string]any{
					"id": "empty", "body": "", "isAnswer": false,
					"replies": []any{}, "replyTotalCount": float64(0),
				},
				map[string]any{
					"id": "nested", "body": "top", "isAnswer": true,
					"replies": []any{map[string]any{
						"id": "reply", "body": "", "isAnswer": false,
					}},
					"replyTotalCount": float64(1),
				},
			},
			"pageInfo": map[string]any{
				"hasNextPage": true, "hasPreviousPage": false,
				"startCursor": "start", "endCursor": "end",
			},
			"totalCount": float64(2),
		}, result.StructuredContent)
	})
}

func TestDiscussionListOutputsValidateEmptyResults(t *testing.T) {
	t.Run("comments", func(t *testing.T) {
		tool := GetDiscussionComments(translations.NullTranslationHelper)
		response := discussionCommentsResponse([]any{}, emptyDiscussionPageInfo(), 0)
		result := callRegisteredTool(t, tool, discussionOutputDeps(t, getDiscussionCommentsOutputQuery,
			discussionCommentVars(float64(30), (*string)(nil)), response), discussionArgs())

		requireStructuredJSONAgreement(t, tool, result)
		output := result.StructuredContent.(map[string]any)
		assert.Equal(t, []any{}, output["comments"])
	})

	t.Run("categories", func(t *testing.T) {
		tool := ListDiscussionCategories(translations.NullTranslationHelper)
		response := discussionConnectionResponse("discussionCategories", []any{}, emptyDiscussionPageInfo(), 0)
		result := callRegisteredTool(t, tool, discussionOutputDeps(t, listDiscussionCategoriesOutputQuery,
			map[string]any{"owner": "owner", "repo": ".github", "first": float64(25)},
			response), map[string]any{"owner": "owner"})

		requireStructuredJSONAgreement(t, tool, result)
		output := result.StructuredContent.(map[string]any)
		assert.Equal(t, []any{}, output["categories"])
	})
}

func TestListDiscussionCategoriesStructuredOutput(t *testing.T) {
	tool := ListDiscussionCategories(translations.NullTranslationHelper)
	response := discussionConnectionResponse("discussionCategories", []any{
		map[string]any{"id": "", "name": ""},
	}, emptyDiscussionPageInfo(), 1)
	result := callRegisteredTool(t, tool, discussionOutputDeps(t, listDiscussionCategoriesOutputQuery,
		map[string]any{"owner": "owner", "repo": "repo", "first": float64(25)},
		response), map[string]any{"owner": "owner", "repo": "repo"})

	requireStructuredJSONAgreement(t, tool, result)
	assert.Equal(t, map[string]any{
		"categories": []any{map[string]any{"id": "", "name": ""}},
		"pageInfo":   emptyDiscussionPageInfo(),
		"totalCount": float64(1),
	}, result.StructuredContent)
}

func TestDiscussionOutputErrorsHaveNoStructuredContent(t *testing.T) {
	tests := []struct {
		name  string
		tool  inventory.ServerTool
		query string
		vars  map[string]any
		args  map[string]any
	}{
		{
			name: "list_discussions", tool: ListDiscussions(translations.NullTranslationHelper),
			query: listDiscussionsOutputQuery,
			vars: map[string]any{
				"owner": "owner", "repo": "repo", "first": float64(30), "after": (*string)(nil),
			},
			args: map[string]any{"owner": "owner", "repo": "repo"},
		},
		{
			name: "get_discussion", tool: GetDiscussion(translations.NullTranslationHelper),
			query: getDiscussionOutputQuery, vars: discussionNumberVars(), args: discussionArgs(),
		},
		{
			name: "get_discussion_comments", tool: GetDiscussionComments(translations.NullTranslationHelper),
			query: getDiscussionCommentsOutputQuery,
			vars:  discussionCommentVars(float64(30), (*string)(nil)), args: discussionArgs(),
		},
		{
			name: "list_discussion_categories", tool: ListDiscussionCategories(translations.NullTranslationHelper),
			query: listDiscussionCategoriesOutputQuery,
			vars:  map[string]any{"owner": "owner", "repo": "repo", "first": float64(25)},
			args:  map[string]any{"owner": "owner", "repo": "repo"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := callRegisteredTool(t, tc.tool, discussionOutputDeps(t, tc.query, tc.vars,
				githubv4mock.ErrorResponse("boom")), tc.args)

			require.True(t, result.IsError)
			assert.Nil(t, result.StructuredContent)
		})
	}
}

func discussionOutputDeps(
	t *testing.T,
	query string,
	vars map[string]any,
	response githubv4mock.GQLResponse,
) ToolDependencies {
	t.Helper()
	matcher := githubv4mock.NewQueryMatcher(query, vars, response)
	return BaseDeps{
		GQLClient: githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matcher)),
		Obsv:      stubExporters(),
	}
}

func discussionArgs() map[string]any {
	return map[string]any{"owner": "owner", "repo": "repo", "discussionNumber": 1}
}

func discussionNumberVars() map[string]any {
	return map[string]any{"owner": "owner", "repo": "repo", "discussionNumber": float64(1)}
}

func discussionCommentVars(first any, after any) map[string]any {
	return map[string]any{
		"owner": "owner", "repo": "repo", "discussionNumber": float64(1),
		"first": first, "after": after,
	}
}

func discussionConnectionResponse(
	field string,
	nodes []any,
	pageInfo map[string]any,
	totalCount int,
) githubv4mock.GQLResponse {
	return githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			field: map[string]any{
				"nodes": nodes, "pageInfo": pageInfo, "totalCount": totalCount,
			},
		},
	})
}

func discussionCommentsResponse(
	nodes []any,
	pageInfo map[string]any,
	totalCount int,
) githubv4mock.GQLResponse {
	return githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussion": map[string]any{
				"comments": map[string]any{
					"nodes": nodes, "pageInfo": pageInfo, "totalCount": totalCount,
				},
			},
		},
	})
}

func emptyDiscussionPageInfo() map[string]any {
	return map[string]any{
		"hasNextPage": false, "hasPreviousPage": false,
		"startCursor": "", "endCursor": "",
	}
}
