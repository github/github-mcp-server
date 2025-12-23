package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	discussionsGeneral = []map[string]any{
		{"number": 1, "title": "Discussion 1 title", "createdAt": "2023-01-01T00:00:00Z", "updatedAt": "2023-01-01T00:00:00Z", "closed": false, "isAnswered": false, "author": map[string]any{"login": "user1"}, "url": "https://github.com/owner/repo/discussions/1", "category": map[string]any{"name": "General"}},
		{"number": 3, "title": "Discussion 3 title", "createdAt": "2023-03-01T00:00:00Z", "updatedAt": "2023-02-01T00:00:00Z", "closed": false, "isAnswered": false, "author": map[string]any{"login": "user1"}, "url": "https://github.com/owner/repo/discussions/3", "category": map[string]any{"name": "General"}},
	}
	discussionsAll = []map[string]any{
		{
			"number":     1,
			"title":      "Discussion 1 title",
			"createdAt":  "2023-01-01T00:00:00Z",
			"updatedAt":  "2023-01-01T00:00:00Z",
			"closed":     false,
			"isAnswered": false,
			"author":     map[string]any{"login": "user1"},
			"url":        "https://github.com/owner/repo/discussions/1",
			"category":   map[string]any{"name": "General"},
		},
		{
			"number":     2,
			"title":      "Discussion 2 title",
			"createdAt":  "2023-02-01T00:00:00Z",
			"updatedAt":  "2023-02-01T00:00:00Z",
			"closed":     false,
			"isAnswered": false,
			"author":     map[string]any{"login": "user2"},
			"url":        "https://github.com/owner/repo/discussions/2",
			"category":   map[string]any{"name": "Questions"},
		},
		{
			"number":     3,
			"title":      "Discussion 3 title",
			"createdAt":  "2023-03-01T00:00:00Z",
			"updatedAt":  "2023-03-01T00:00:00Z",
			"closed":     false,
			"isAnswered": false,
			"author":     map[string]any{"login": "user3"},
			"url":        "https://github.com/owner/repo/discussions/3",
			"category":   map[string]any{"name": "General"},
		},
	}

	discussionsOrgLevel = []map[string]any{
		{
			"number":     1,
			"title":      "Org Discussion 1 - Community Guidelines",
			"createdAt":  "2023-01-15T00:00:00Z",
			"updatedAt":  "2023-01-15T00:00:00Z",
			"closed":     false,
			"isAnswered": false,
			"author":     map[string]any{"login": "org-admin"},
			"url":        "https://github.com/owner/.github/discussions/1",
			"category":   map[string]any{"name": "Announcements"},
		},
		{
			"number":     2,
			"title":      "Org Discussion 2 - Roadmap 2023",
			"createdAt":  "2023-02-20T00:00:00Z",
			"updatedAt":  "2023-02-20T00:00:00Z",
			"closed":     false,
			"isAnswered": false,
			"author":     map[string]any{"login": "org-admin"},
			"url":        "https://github.com/owner/.github/discussions/2",
			"category":   map[string]any{"name": "General"},
		},
		{
			"number":     3,
			"title":      "Org Discussion 3 - Roadmap 2024",
			"createdAt":  "2023-02-20T00:00:00Z",
			"updatedAt":  "2023-02-20T00:00:00Z",
			"closed":     false,
			"isAnswered": false,
			"author":     map[string]any{"login": "org-admin"},
			"url":        "https://github.com/owner/.github/discussions/3",
			"category":   map[string]any{"name": "General"},
		},
		{
			"number":     4,
			"title":      "Org Discussion 4 - Roadmap 2025",
			"createdAt":  "2023-02-20T00:00:00Z",
			"updatedAt":  "2023-02-20T00:00:00Z",
			"closed":     false,
			"isAnswered": false,
			"author":     map[string]any{"login": "org-admin"},
			"url":        "https://github.com/owner/.github/discussions/4",
			"category":   map[string]any{"name": "General"},
		},
	}

	// Ordered mock responses
	discussionsOrderedCreatedAsc = []map[string]any{
		discussionsAll[0], // Discussion 1 (created 2023-01-01)
		discussionsAll[1], // Discussion 2 (created 2023-02-01)
		discussionsAll[2], // Discussion 3 (created 2023-03-01)
	}

	discussionsOrderedUpdatedDesc = []map[string]any{
		discussionsAll[2], // Discussion 3 (updated 2023-03-01)
		discussionsAll[1], // Discussion 2 (updated 2023-02-01)
		discussionsAll[0], // Discussion 1 (updated 2023-01-01)
	}

	// only 'General' category discussions ordered by created date descending
	discussionsGeneralOrderedDesc = []map[string]any{
		discussionsGeneral[1], // Discussion 3 (created 2023-03-01)
		discussionsGeneral[0], // Discussion 1 (created 2023-01-01)
	}

	mockResponseListAll = githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussions": map[string]any{
				"nodes": discussionsAll,
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 3,
			},
		},
	})
	mockResponseListGeneral = githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussions": map[string]any{
				"nodes": discussionsGeneral,
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 2,
			},
		},
	})
	mockResponseOrderedCreatedAsc = githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussions": map[string]any{
				"nodes": discussionsOrderedCreatedAsc,
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 3,
			},
		},
	})
	mockResponseOrderedUpdatedDesc = githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussions": map[string]any{
				"nodes": discussionsOrderedUpdatedDesc,
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 3,
			},
		},
	})
	mockResponseGeneralOrderedDesc = githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussions": map[string]any{
				"nodes": discussionsGeneralOrderedDesc,
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 2,
			},
		},
	})

	mockResponseOrgLevel = githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussions": map[string]any{
				"nodes": discussionsOrgLevel,
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 4,
			},
		},
	})

	mockErrorRepoNotFound = githubv4mock.ErrorResponse("repository not found")
)

func Test_ListDiscussions(t *testing.T) {
	toolDef := ListDiscussions(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_discussions", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "orderBy")
	assert.Contains(t, schema.Properties, "direction")
	assert.ElementsMatch(t, schema.Required, []string{"owner"})

	// Variables matching what GraphQL receives after JSON marshaling/unmarshaling
	varsListAll := map[string]interface{}{
		"owner": "owner",
		"repo":  "repo",
		"first": float64(30),
		"after": (*string)(nil),
	}

	varsRepoNotFound := map[string]interface{}{
		"owner": "owner",
		"repo":  "nonexistent-repo",
		"first": float64(30),
		"after": (*string)(nil),
	}

	varsDiscussionsFiltered := map[string]interface{}{
		"owner":      "owner",
		"repo":       "repo",
		"categoryId": "DIC_kwDOABC123",
		"first":      float64(30),
		"after":      (*string)(nil),
	}

	varsOrderByCreatedAsc := map[string]interface{}{
		"owner":            "owner",
		"repo":             "repo",
		"orderByField":     "CREATED_AT",
		"orderByDirection": "ASC",
		"first":            float64(30),
		"after":            (*string)(nil),
	}

	varsOrderByUpdatedDesc := map[string]interface{}{
		"owner":            "owner",
		"repo":             "repo",
		"orderByField":     "UPDATED_AT",
		"orderByDirection": "DESC",
		"first":            float64(30),
		"after":            (*string)(nil),
	}

	varsCategoryWithOrder := map[string]interface{}{
		"owner":            "owner",
		"repo":             "repo",
		"categoryId":       "DIC_kwDOABC123",
		"orderByField":     "CREATED_AT",
		"orderByDirection": "DESC",
		"first":            float64(30),
		"after":            (*string)(nil),
	}

	varsOrgLevel := map[string]interface{}{
		"owner": "owner",
		"repo":  ".github", // This is what gets set when repo is not provided
		"first": float64(30),
		"after": (*string)(nil),
	}

	tests := []struct {
		name          string
		reqParams     map[string]interface{}
		expectError   bool
		errContains   string
		expectedCount int
		verifyOrder   func(t *testing.T, discussions []*github.Discussion)
	}{
		{
			name: "list all discussions without category filter",
			reqParams: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:   false,
			expectedCount: 3, // All discussions
		},
		{
			name: "filter by category ID",
			reqParams: map[string]interface{}{
				"owner":    "owner",
				"repo":     "repo",
				"category": "DIC_kwDOABC123",
			},
			expectError:   false,
			expectedCount: 2, // Only General discussions (matching the category ID)
		},
		{
			name: "order by created at ascending",
			reqParams: map[string]interface{}{
				"owner":     "owner",
				"repo":      "repo",
				"orderBy":   "CREATED_AT",
				"direction": "ASC",
			},
			expectError:   false,
			expectedCount: 3,
			verifyOrder: func(t *testing.T, discussions []*github.Discussion) {
				// Verify discussions are ordered by created date ascending
				require.Len(t, discussions, 3)
				assert.Equal(t, 1, *discussions[0].Number, "First should be discussion 1 (created 2023-01-01)")
				assert.Equal(t, 2, *discussions[1].Number, "Second should be discussion 2 (created 2023-02-01)")
				assert.Equal(t, 3, *discussions[2].Number, "Third should be discussion 3 (created 2023-03-01)")
			},
		},
		{
			name: "order by updated at descending",
			reqParams: map[string]interface{}{
				"owner":     "owner",
				"repo":      "repo",
				"orderBy":   "UPDATED_AT",
				"direction": "DESC",
			},
			expectError:   false,
			expectedCount: 3,
			verifyOrder: func(t *testing.T, discussions []*github.Discussion) {
				// Verify discussions are ordered by updated date descending
				require.Len(t, discussions, 3)
				assert.Equal(t, 3, *discussions[0].Number, "First should be discussion 3 (updated 2023-03-01)")
				assert.Equal(t, 2, *discussions[1].Number, "Second should be discussion 2 (updated 2023-02-01)")
				assert.Equal(t, 1, *discussions[2].Number, "Third should be discussion 1 (updated 2023-01-01)")
			},
		},
		{
			name: "filter by category with order",
			reqParams: map[string]interface{}{
				"owner":     "owner",
				"repo":      "repo",
				"category":  "DIC_kwDOABC123",
				"orderBy":   "CREATED_AT",
				"direction": "DESC",
			},
			expectError:   false,
			expectedCount: 2,
			verifyOrder: func(t *testing.T, discussions []*github.Discussion) {
				// Verify only General discussions, ordered by created date descending
				require.Len(t, discussions, 2)
				assert.Equal(t, 3, *discussions[0].Number, "First should be discussion 3 (created 2023-03-01)")
				assert.Equal(t, 1, *discussions[1].Number, "Second should be discussion 1 (created 2023-01-01)")
			},
		},
		{
			name: "order by without direction (should not use ordering)",
			reqParams: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"orderBy": "CREATED_AT",
			},
			expectError:   false,
			expectedCount: 3,
		},
		{
			name: "direction without order by (should not use ordering)",
			reqParams: map[string]interface{}{
				"owner":     "owner",
				"repo":      "repo",
				"direction": "DESC",
			},
			expectError:   false,
			expectedCount: 3,
		},
		{
			name: "repository not found error",
			reqParams: map[string]interface{}{
				"owner": "owner",
				"repo":  "nonexistent-repo",
			},
			expectError: true,
			errContains: "repository not found",
		},
		{
			name: "list org-level discussions (no repo provided)",
			reqParams: map[string]interface{}{
				"owner": "owner",
				// repo is not provided, it will default to ".github"
			},
			expectError:   false,
			expectedCount: 4,
		},
	}

	// Define the actual query strings that match the implementation
	qBasicNoOrder := "query($after:String$first:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussions(first: $first, after: $after){nodes{number,title,createdAt,updatedAt,closed,isAnswered,answerChosenAt,author{login},category{name},url},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}"
	qWithCategoryNoOrder := "query($after:String$categoryId:ID!$first:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussions(first: $first, after: $after, categoryId: $categoryId){nodes{number,title,createdAt,updatedAt,closed,isAnswered,answerChosenAt,author{login},category{name},url},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}"
	qBasicWithOrder := "query($after:String$first:Int!$orderByDirection:OrderDirection!$orderByField:DiscussionOrderField!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussions(first: $first, after: $after, orderBy: { field: $orderByField, direction: $orderByDirection }){nodes{number,title,createdAt,updatedAt,closed,isAnswered,answerChosenAt,author{login},category{name},url},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}"
	qWithCategoryAndOrder := "query($after:String$categoryId:ID!$first:Int!$orderByDirection:OrderDirection!$orderByField:DiscussionOrderField!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussions(first: $first, after: $after, categoryId: $categoryId, orderBy: { field: $orderByField, direction: $orderByDirection }){nodes{number,title,createdAt,updatedAt,closed,isAnswered,answerChosenAt,author{login},category{name},url},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var httpClient *http.Client

			switch tc.name {
			case "list all discussions without category filter":
				matcher := githubv4mock.NewQueryMatcher(qBasicNoOrder, varsListAll, mockResponseListAll)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "filter by category ID":
				matcher := githubv4mock.NewQueryMatcher(qWithCategoryNoOrder, varsDiscussionsFiltered, mockResponseListGeneral)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "order by created at ascending":
				matcher := githubv4mock.NewQueryMatcher(qBasicWithOrder, varsOrderByCreatedAsc, mockResponseOrderedCreatedAsc)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "order by updated at descending":
				matcher := githubv4mock.NewQueryMatcher(qBasicWithOrder, varsOrderByUpdatedDesc, mockResponseOrderedUpdatedDesc)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "filter by category with order":
				matcher := githubv4mock.NewQueryMatcher(qWithCategoryAndOrder, varsCategoryWithOrder, mockResponseGeneralOrderedDesc)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "order by without direction (should not use ordering)":
				matcher := githubv4mock.NewQueryMatcher(qBasicNoOrder, varsListAll, mockResponseListAll)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "direction without order by (should not use ordering)":
				matcher := githubv4mock.NewQueryMatcher(qBasicNoOrder, varsListAll, mockResponseListAll)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "repository not found error":
				matcher := githubv4mock.NewQueryMatcher(qBasicNoOrder, varsRepoNotFound, mockErrorRepoNotFound)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "list org-level discussions (no repo provided)":
				matcher := githubv4mock.NewQueryMatcher(qBasicNoOrder, varsOrgLevel, mockResponseOrgLevel)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			}

			gqlClient := githubv4.NewClient(httpClient)
			deps := BaseDeps{GQLClient: gqlClient}
			handler := toolDef.Handler(deps)

			req := createMCPRequest(tc.reqParams)
			res, err := handler(ContextWithDeps(context.Background(), deps), &req)
			text := getTextResult(t, res).Text

			if tc.expectError {
				require.True(t, res.IsError)
				assert.Contains(t, text, tc.errContains)
				return
			}
			require.NoError(t, err)

			// Parse the structured response with pagination info
			var response struct {
				Discussions []*github.Discussion `json:"discussions"`
				PageInfo    struct {
					HasNextPage     bool   `json:"hasNextPage"`
					HasPreviousPage bool   `json:"hasPreviousPage"`
					StartCursor     string `json:"startCursor"`
					EndCursor       string `json:"endCursor"`
				} `json:"pageInfo"`
				TotalCount int `json:"totalCount"`
			}
			err = json.Unmarshal([]byte(text), &response)
			require.NoError(t, err)

			assert.Len(t, response.Discussions, tc.expectedCount, "Expected %d discussions, got %d", tc.expectedCount, len(response.Discussions))

			// Verify order if verifyOrder function is provided
			if tc.verifyOrder != nil {
				tc.verifyOrder(t, response.Discussions)
			}

			// Verify that all returned discussions have a category if filtered
			if _, hasCategory := tc.reqParams["category"]; hasCategory {
				for _, discussion := range response.Discussions {
					require.NotNil(t, discussion.DiscussionCategory, "Discussion should have category")
					assert.NotEmpty(t, *discussion.DiscussionCategory.Name, "Discussion should have category name")
				}
			}
		})
	}
}

func Test_GetDiscussion(t *testing.T) {
	// Verify tool definition and schema
	toolDef := GetDiscussion(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_discussion", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "discussionNumber")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "discussionNumber"})

	// Use exact string query that matches implementation output
	qGetDiscussion := "query($discussionNumber:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussion(number: $discussionNumber){number,title,body,createdAt,closed,isAnswered,answerChosenAt,url,category{name}}}}"

	vars := map[string]interface{}{
		"owner":            "owner",
		"repo":             "repo",
		"discussionNumber": float64(1),
	}
	tests := []struct {
		name        string
		response    githubv4mock.GQLResponse
		expectError bool
		expected    map[string]interface{}
		errContains string
	}{
		{
			name: "successful retrieval",
			response: githubv4mock.DataResponse(map[string]any{
				"repository": map[string]any{"discussion": map[string]any{
					"number":     1,
					"title":      "Test Discussion Title",
					"body":       "This is a test discussion",
					"url":        "https://github.com/owner/repo/discussions/1",
					"createdAt":  "2025-04-25T12:00:00Z",
					"closed":     false,
					"isAnswered": false,
					"category":   map[string]any{"name": "General"},
				}},
			}),
			expectError: false,
			expected: map[string]interface{}{
				"number":     float64(1),
				"title":      "Test Discussion Title",
				"body":       "This is a test discussion",
				"url":        "https://github.com/owner/repo/discussions/1",
				"closed":     false,
				"isAnswered": false,
			},
		},
		{
			name:        "discussion not found",
			response:    githubv4mock.ErrorResponse("discussion not found"),
			expectError: true,
			errContains: "discussion not found",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matcher := githubv4mock.NewQueryMatcher(qGetDiscussion, vars, tc.response)
			httpClient := githubv4mock.NewMockedHTTPClient(matcher)
			gqlClient := githubv4.NewClient(httpClient)
			deps := BaseDeps{GQLClient: gqlClient}
			handler := toolDef.Handler(deps)

			reqParams := map[string]interface{}{"owner": "owner", "repo": "repo", "discussionNumber": int32(1)}
			req := createMCPRequest(reqParams)
			res, err := handler(ContextWithDeps(context.Background(), deps), &req)
			text := getTextResult(t, res).Text

			if tc.expectError {
				require.True(t, res.IsError)
				assert.Contains(t, text, tc.errContains)
				return
			}

			require.NoError(t, err)
			var out map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(text), &out))
			assert.Equal(t, tc.expected["number"], out["number"])
			assert.Equal(t, tc.expected["title"], out["title"])
			assert.Equal(t, tc.expected["body"], out["body"])
			assert.Equal(t, tc.expected["url"], out["url"])
			assert.Equal(t, tc.expected["closed"], out["closed"])
			assert.Equal(t, tc.expected["isAnswered"], out["isAnswered"])
			// Check category is present
			category, ok := out["category"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "General", category["name"])
		})
	}
}

func Test_GetDiscussionComments(t *testing.T) {
	// Verify tool definition and schema
	toolDef := GetDiscussionComments(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_discussion_comments", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "discussionNumber")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "discussionNumber"})

	// Use exact string query that matches implementation output
	qGetComments := "query($after:String$discussionNumber:Int!$first:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussion(number: $discussionNumber){comments(first: $first, after: $after){nodes{body},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}}"

	// Variables matching what GraphQL receives after JSON marshaling/unmarshaling
	vars := map[string]interface{}{
		"owner":            "owner",
		"repo":             "repo",
		"discussionNumber": float64(1),
		"first":            float64(30),
		"after":            (*string)(nil),
	}

	mockResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussion": map[string]any{
				"comments": map[string]any{
					"nodes": []map[string]any{
						{"body": "This is the first comment"},
						{"body": "This is the second comment"},
					},
					"pageInfo": map[string]any{
						"hasNextPage":     false,
						"hasPreviousPage": false,
						"startCursor":     "",
						"endCursor":       "",
					},
					"totalCount": 2,
				},
			},
		},
	})
	matcher := githubv4mock.NewQueryMatcher(qGetComments, vars, mockResponse)
	httpClient := githubv4mock.NewMockedHTTPClient(matcher)
	gqlClient := githubv4.NewClient(httpClient)
	deps := BaseDeps{GQLClient: gqlClient}
	handler := toolDef.Handler(deps)

	reqParams := map[string]interface{}{
		"owner":            "owner",
		"repo":             "repo",
		"discussionNumber": int32(1),
	}
	request := createMCPRequest(reqParams)

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)

	textContent := getTextResult(t, result)

	// (Lines removed)

	var response struct {
		Comments []*github.IssueComment `json:"comments"`
		PageInfo struct {
			HasNextPage     bool   `json:"hasNextPage"`
			HasPreviousPage bool   `json:"hasPreviousPage"`
			StartCursor     string `json:"startCursor"`
			EndCursor       string `json:"endCursor"`
		} `json:"pageInfo"`
		TotalCount int `json:"totalCount"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)
	assert.Len(t, response.Comments, 2)
	expectedBodies := []string{"This is the first comment", "This is the second comment"}
	for i, comment := range response.Comments {
		assert.Equal(t, expectedBodies[i], *comment.Body)
	}
}

func Test_ListDiscussionCategories(t *testing.T) {
	toolDef := ListDiscussionCategories(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_discussion_categories", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.Description, "or organisation")
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.ElementsMatch(t, schema.Required, []string{"owner"})

	// Use exact string query that matches implementation output
	qListCategories := "query($first:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){discussionCategories(first: $first){nodes{id,name},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount}}}"

	// Variables for repository-level categories
	varsRepo := map[string]interface{}{
		"owner": "owner",
		"repo":  "repo",
		"first": float64(25),
	}

	// Variables for organization-level categories (using .github repo)
	varsOrg := map[string]interface{}{
		"owner": "owner",
		"repo":  ".github",
		"first": float64(25),
	}

	mockRespRepo := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussionCategories": map[string]any{
				"nodes": []map[string]any{
					{"id": "123", "name": "CategoryOne"},
					{"id": "456", "name": "CategoryTwo"},
				},
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 2,
			},
		},
	})

	mockRespOrg := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"discussionCategories": map[string]any{
				"nodes": []map[string]any{
					{"id": "789", "name": "Announcements"},
					{"id": "101", "name": "General"},
					{"id": "112", "name": "Ideas"},
				},
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 3,
			},
		},
	})

	tests := []struct {
		name               string
		reqParams          map[string]interface{}
		vars               map[string]interface{}
		mockResponse       githubv4mock.GQLResponse
		expectError        bool
		expectedCount      int
		expectedCategories []map[string]string
	}{
		{
			name: "list repository-level discussion categories",
			reqParams: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			vars:          varsRepo,
			mockResponse:  mockRespRepo,
			expectError:   false,
			expectedCount: 2,
			expectedCategories: []map[string]string{
				{"id": "123", "name": "CategoryOne"},
				{"id": "456", "name": "CategoryTwo"},
			},
		},
		{
			name: "list org-level discussion categories (no repo provided)",
			reqParams: map[string]interface{}{
				"owner": "owner",
				// repo is not provided, it will default to ".github"
			},
			vars:          varsOrg,
			mockResponse:  mockRespOrg,
			expectError:   false,
			expectedCount: 3,
			expectedCategories: []map[string]string{
				{"id": "789", "name": "Announcements"},
				{"id": "101", "name": "General"},
				{"id": "112", "name": "Ideas"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matcher := githubv4mock.NewQueryMatcher(qListCategories, tc.vars, tc.mockResponse)
			httpClient := githubv4mock.NewMockedHTTPClient(matcher)
			gqlClient := githubv4.NewClient(httpClient)

			deps := BaseDeps{GQLClient: gqlClient}
			handler := toolDef.Handler(deps)

			req := createMCPRequest(tc.reqParams)
			res, err := handler(ContextWithDeps(context.Background(), deps), &req)
			text := getTextResult(t, res).Text

			if tc.expectError {
				require.True(t, res.IsError)
				return
			}
			require.NoError(t, err)

			var response struct {
				Categories []map[string]string `json:"categories"`
				PageInfo   struct {
					HasNextPage     bool   `json:"hasNextPage"`
					HasPreviousPage bool   `json:"hasPreviousPage"`
					StartCursor     string `json:"startCursor"`
					EndCursor       string `json:"endCursor"`
				} `json:"pageInfo"`
				TotalCount int `json:"totalCount"`
			}
			require.NoError(t, json.Unmarshal([]byte(text), &response))
			assert.Equal(t, tc.expectedCategories, response.Categories)
		})
	}
}
