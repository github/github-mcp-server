package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueWriteCreateWithParentAndLabelsUsesSingleMutation(t *testing.T) {
	serverTool := IssueWrite(translations.NullTranslationHelper)
	schema := serverTool.Tool.InputSchema
	issueWriteSchema := schema.(*jsonschema.Schema)
	assert.Contains(t, issueWriteSchema.Properties, "parent_issue_number")
	assert.Contains(t, issueWriteSchema.Properties, "parent_owner")
	assert.Contains(t, issueWriteSchema.Properties, "parent_repo")
	assert.NotContains(t, issueWriteSchema.Required, "parent_issue_number")
	assert.NotContains(t, issueWriteSchema.Required, "parent_owner")
	assert.NotContains(t, issueWriteSchema.Required, "parent_repo")

	labelIDs := []githubv4.ID{"LABEL_backlog"}
	parentID := githubv4.ID("ISSUE_parent")
	expectedInput := CreateIssueInput{
		RepositoryID:  githubv4.ID("REPO_1"),
		Title:         githubv4.String("Atomic child"),
		Body:          githubv4.NewString(githubv4.String("Created under its parent")),
		LabelIDs:      &labelIDs,
		ParentIssueID: &parentID,
	}
	createMatcher := githubv4mock.NewMutationMatcher(
		createIssueMutation{},
		expectedInput,
		nil,
		githubv4mock.DataResponse(map[string]any{
			"createIssue": map[string]any{
				"issue": map[string]any{
					"fullDatabaseId": "12345",
					"url":            "https://github.com/owner/repo/issues/2",
				},
			},
		}),
	)
	assert.Contains(t, createMatcher.Request, "$input:CreateIssueInput!")

	gqlHTTPClient, gqlCalls := countingGraphQLClient(
		createIssueParentMatcher(1, "parent-owner", "parent-repo", "REPO_1", "ISSUE_parent"),
		createIssueLabelMatcher("status:backlog", "LABEL_backlog"),
		createMatcher,
	)
	restHTTPClient := MockHTTPClientWithHandlers(nil)
	restCounter := &countingRoundTripper{next: restHTTPClient.Transport}
	restHTTPClient.Transport = restCounter

	deps := BaseDeps{
		Client:    mustNewGHClient(t, restHTTPClient),
		GQLClient: githubv4.NewClient(gqlHTTPClient),
	}
	handler := serverTool.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":              "create",
		"owner":               "owner",
		"repo":                "repo",
		"title":               "Atomic child",
		"body":                "Created under its parent",
		"labels":              []any{"status:backlog"},
		"parent_issue_number": float64(1),
		"parent_owner":        "parent-owner",
		"parent_repo":         "parent-repo",
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, getTextResult(t, result).Text)
	assert.Equal(t, 3, gqlCalls(), "metadata lookups and exactly one create mutation are expected")
	assert.Zero(t, restCounter.count.Load(), "parent creation must not use REST create or attachment requests")

	var response MinimalResponse
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, "12345", response.ID)
	assert.Equal(t, "https://github.com/owner/repo/issues/2", response.URL)
}

func TestIssueWriteCreateWithParentDoesNotFallbackAfterMutationFailure(t *testing.T) {
	gqlHTTPClient, gqlCalls := countingGraphQLClient(
		createIssueParentMatcher(7, "owner", "repo", "REPO_1", "ISSUE_parent"),
		githubv4mock.NewMutationMatcher(
			createIssueMutation{},
			CreateIssueInput{
				RepositoryID:  githubv4.ID("REPO_1"),
				Title:         githubv4.String("Atomic child"),
				ParentIssueID: githubv4mock.Ptr[githubv4.ID]("ISSUE_parent"),
			},
			nil,
			githubv4mock.ErrorResponse("parent cannot accept sub-issues"),
		),
	)
	restHTTPClient := MockHTTPClientWithHandlers(nil)
	restCounter := &countingRoundTripper{next: restHTTPClient.Transport}
	restHTTPClient.Transport = restCounter

	deps := BaseDeps{
		Client:    mustNewGHClient(t, restHTTPClient),
		GQLClient: githubv4.NewClient(gqlHTTPClient),
	}
	serverTool := IssueWrite(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":              "create",
		"owner":               "owner",
		"repo":                "repo",
		"title":               "Atomic child",
		"parent_issue_number": float64(7),
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, getTextResult(t, result).Text, "failed to create issue")
	assert.Equal(t, 2, gqlCalls(), "a failed create mutation must not trigger an attachment mutation")
	assert.Zero(t, restCounter.count.Load(), "a failed create mutation must not fall back to REST create or attachment requests")
}

func TestGranularCreateIssueWithParentUsesAtomicMutation(t *testing.T) {
	serverTool := GranularCreateIssue(translations.NullTranslationHelper)
	schema := serverTool.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "parent_issue_number")
	assert.Contains(t, schema.Properties, "parent_owner")
	assert.Contains(t, schema.Properties, "parent_repo")

	parentID := githubv4.ID("ISSUE_parent")
	gqlHTTPClient := githubv4mock.NewMockedHTTPClient(
		createIssueParentMatcher(3, "owner", "repo", "REPO_1", "ISSUE_parent"),
		githubv4mock.NewMutationMatcher(
			createIssueMutation{},
			CreateIssueInput{
				RepositoryID:  githubv4.ID("REPO_1"),
				Title:         githubv4.String("Granular child"),
				ParentIssueID: &parentID,
			},
			nil,
			githubv4mock.DataResponse(map[string]any{
				"createIssue": map[string]any{
					"issue": map[string]any{
						"fullDatabaseId": "23456",
						"url":            "https://github.com/owner/repo/issues/4",
					},
				},
			}),
		),
	)
	restHTTPClient := MockHTTPClientWithHandlers(nil)

	deps := BaseDeps{
		Client:    mustNewGHClient(t, restHTTPClient),
		GQLClient: githubv4.NewClient(gqlHTTPClient),
	}
	handler := serverTool.Handler(deps)
	request := createMCPRequest(map[string]any{
		"owner":               "owner",
		"repo":                "repo",
		"title":               "Granular child",
		"parent_issue_number": float64(3),
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateIssueParentRepositoryRequiresParentIssueNumber(t *testing.T) {
	tests := []struct {
		name    string
		handler func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args    map[string]any
		want    string
	}{
		{
			name: "issue_write",
			handler: func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				serverTool := IssueWrite(translations.NullTranslationHelper)
				return serverTool.Handler(BaseDeps{})(ctx, request)
			},
			args: map[string]any{
				"method":       "create",
				"owner":        "owner",
				"repo":         "repo",
				"title":        "Child",
				"parent_owner": "parent-owner",
			},
			want: "can only be used when parent_issue_number is provided",
		},
		{
			name: "create_issue",
			handler: func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				serverTool := GranularCreateIssue(translations.NullTranslationHelper)
				return serverTool.Handler(BaseDeps{})(ctx, request)
			},
			args: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"title":       "Child",
				"parent_repo": "parent-repo",
			},
			want: "can only be used when parent_issue_number is provided",
		},
		{
			name: "issue fields",
			handler: func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				serverTool := IssueWrite(translations.NullTranslationHelper)
				return serverTool.Handler(BaseDeps{})(ctx, request)
			},
			args: map[string]any{
				"method":              "create",
				"owner":               "owner",
				"repo":                "repo",
				"title":               "Child",
				"parent_issue_number": float64(1),
				"issue_fields": []any{
					map[string]any{"field_name": "Priority", "field_option_name": "High"},
				},
			},
			want: "issue_fields cannot be used with parent_issue_number",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := createMCPRequest(test.args)
			result, err := test.handler(ContextWithDeps(context.Background(), BaseDeps{}), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getTextResult(t, result).Text, test.want)
		})
	}
}

func createIssueParentMatcher(parentIssueNumber int, parentOwner, parentRepo string, repositoryID, parentIssueID githubv4.ID) githubv4mock.Matcher {
	return githubv4mock.NewQueryMatcher(
		createIssueParentMetadataQuery{},
		map[string]any{
			"owner":             githubv4.String("owner"),
			"repo":              githubv4.String("repo"),
			"parentOwner":       githubv4.String(parentOwner),
			"parentRepo":        githubv4.String(parentRepo),
			"parentIssueNumber": githubv4.Int(parentIssueNumber), // #nosec G115 - test issue numbers are small
		},
		githubv4mock.DataResponse(map[string]any{
			"childRepository": map[string]any{
				"id": repositoryID,
			},
			"parentRepository": map[string]any{
				"issue": map[string]any{
					"id": parentIssueID,
				},
			},
		}),
	)
}

func createIssueLabelMatcher(name string, id githubv4.ID) githubv4mock.Matcher {
	return githubv4mock.NewQueryMatcher(
		struct {
			Repository struct {
				Label struct {
					ID   githubv4.ID
					Name githubv4.String
				} `graphql:"label(name: $name)"`
			} `graphql:"repository(owner: $owner, name: $repo)"`
		}{},
		map[string]any{
			"owner": githubv4.String("owner"),
			"repo":  githubv4.String("repo"),
			"name":  githubv4.String(name),
		},
		githubv4mock.DataResponse(map[string]any{
			"repository": map[string]any{
				"label": map[string]any{
					"id":   id,
					"name": name,
				},
			},
		}),
	)
}
