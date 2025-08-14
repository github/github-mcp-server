package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListProjects(t *testing.T) {
	mockClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Organization struct {
					Projects struct {
						Nodes []struct{ ID githubv4.ID }
					} `graphql:"projectsV2(first: 100)"`
				} `graphql:"organization(login: $login)"`
			}{},
			map[string]any{"login": githubv4.String("acme")},
			githubv4mock.DataResponse(map[string]any{"organization": map[string]any{"projectsV2": map[string]any{"nodes": []any{}}}}),
		),
	)
	tool, handler := ListProjects(stubGetGQLClientFn(githubv4.NewClient(mockClient)), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	res, err := handler(context.Background(), createMCPRequest(map[string]any{"owner": "acme"}))
	require.NoError(t, err)
	assert.NotNil(t, res)
}

func Test_AddIssueToProject(t *testing.T) {
	mockClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewMutationMatcher(
			struct {
				AddProjectV2ItemById struct{ Item struct{ ID githubv4.ID } } `graphql:"addProjectV2ItemById(input: $input)"`
			}{},
			githubv4.AddProjectV2ItemByIdInput{ProjectID: "proj", ContentID: "issue"},
			nil,
			githubv4mock.DataResponse(map[string]any{"addProjectV2ItemById": map[string]any{"item": map[string]any{"id": "1"}}}),
		),
	)
	tool, handler := AddIssueToProject(stubGetGQLClientFn(githubv4.NewClient(mockClient)), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	res, err := handler(context.Background(), createMCPRequest(map[string]any{"project_id": "proj", "issue_id": "issue"}))
	require.NoError(t, err)
	assert.NotNil(t, res)
}

func Test_CreateProjectIssue(t *testing.T) {
	mockClient := githubv4mock.NewMockedHTTPClient(
		githubv4mock.NewQueryMatcher(
			struct {
				Repository struct{ ID githubv4.ID } `graphql:"repository(owner: $owner, name: $name)"`
			}{},
			map[string]any{"owner": githubv4.String("acme"), "name": githubv4.String("demo")},
			githubv4mock.DataResponse(map[string]any{"repository": map[string]any{"id": "123"}}),
		),
		githubv4mock.NewMutationMatcher(
			struct {
				CreateIssue struct{ Issue struct{ ID githubv4.ID } } `graphql:"createIssue(input: $input)"`
			}{},
			githubv4.CreateIssueInput{RepositoryID: "123", Title: githubv4.String("hello")},
			nil,
			githubv4mock.DataResponse(map[string]any{"createIssue": map[string]any{"issue": map[string]any{"id": "456"}}}),
		),
	)
	tool, handler := CreateProjectIssue(stubGetGQLClientFn(githubv4.NewClient(mockClient)), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	res, err := handler(context.Background(), createMCPRequest(map[string]any{"owner": "acme", "repo": "demo", "title": "hello"}))
	require.NoError(t, err)
	assert.NotNil(t, res)
}

func Test_ProjectToolSchemas(t *testing.T) {
	client := githubv4.NewClient(nil)
	tools := []mcp.Tool{}
	t1, _ := ListProjects(stubGetGQLClientFn(client), translations.NullTranslationHelper)
	tools = append(tools, t1)
	t2, _ := GetProjectFields(stubGetGQLClientFn(client), translations.NullTranslationHelper)
	tools = append(tools, t2)
	t3, _ := GetProjectItems(stubGetGQLClientFn(client), translations.NullTranslationHelper)
	tools = append(tools, t3)
	t4, _ := CreateProjectIssue(stubGetGQLClientFn(client), translations.NullTranslationHelper)
	tools = append(tools, t4)
	t5, _ := AddIssueToProject(stubGetGQLClientFn(client), translations.NullTranslationHelper)
	tools = append(tools, t5)
	t6, _ := UpdateProjectItemField(stubGetGQLClientFn(client), translations.NullTranslationHelper)
	tools = append(tools, t6)
	t7, _ := CreateDraftIssue(stubGetGQLClientFn(client), translations.NullTranslationHelper)
	tools = append(tools, t7)
	t8, _ := DeleteProjectItem(stubGetGQLClientFn(client), translations.NullTranslationHelper)
	tools = append(tools, t8)
	for _, tool := range tools {
		require.NoError(t, toolsnaps.Test(tool.Name, tool))
	}
}
