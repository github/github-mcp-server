package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListStarredRepositories(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ListStarredRepositories(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_starred_repositories", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "username")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "direction")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.Empty(t, tool.InputSchema.Required) // All parameters are optional

	// Setup mock starred repositories
	starredAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now().Add(-2 * time.Hour)
	mockStarredRepos := []*github.StarredRepository{
		{
			StarredAt: &github.Timestamp{Time: starredAt},
			Repository: &github.Repository{
				ID:              github.Ptr(int64(12345)),
				Name:            github.Ptr("awesome-repo"),
				FullName:        github.Ptr("owner/awesome-repo"),
				Description:     github.Ptr("An awesome repository"),
				HTMLURL:         github.Ptr("https://github.com/owner/awesome-repo"),
				Language:        github.Ptr("Go"),
				StargazersCount: github.Ptr(100),
				ForksCount:      github.Ptr(25),
				OpenIssuesCount: github.Ptr(5),
				UpdatedAt:       &github.Timestamp{Time: updatedAt},
				Private:         github.Ptr(false),
				Fork:            github.Ptr(false),
				Archived:        github.Ptr(false),
				DefaultBranch:   github.Ptr("main"),
			},
		},
		{
			StarredAt: &github.Timestamp{Time: starredAt.Add(-12 * time.Hour)},
			Repository: &github.Repository{
				ID:              github.Ptr(int64(67890)),
				Name:            github.Ptr("cool-project"),
				FullName:        github.Ptr("user/cool-project"),
				Description:     github.Ptr("A very cool project"),
				HTMLURL:         github.Ptr("https://github.com/user/cool-project"),
				Language:        github.Ptr("Python"),
				StargazersCount: github.Ptr(500),
				ForksCount:      github.Ptr(75),
				OpenIssuesCount: github.Ptr(10),
				UpdatedAt:       &github.Timestamp{Time: updatedAt.Add(-1 * time.Hour)},
				Private:         github.Ptr(false),
				Fork:            github.Ptr(true),
				Archived:        github.Ptr(false),
				DefaultBranch:   github.Ptr("master"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
		expectedCount  int
	}{
		{
			name: "successful list for authenticated user",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserStarred,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(mockStarredRepos))
					}),
				),
			),
			requestArgs:   map[string]interface{}{},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "successful list for specific user",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUsersStarredByUsername,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(mockStarredRepos))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"username": "testuser",
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "list fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserStarred,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs:    map[string]interface{}{},
			expectError:    true,
			expectedErrMsg: "failed to list starred repositories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListStarredRepositories(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NotNil(t, result)
				textResult, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "Expected text content")
				assert.Contains(t, textResult.Text, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Parse the result and get the text content
				textContent := getTextResult(t, result)

				// Unmarshal and verify the result
				var returnedRepos []MinimalRepository
				err = json.Unmarshal([]byte(textContent.Text), &returnedRepos)
				require.NoError(t, err)

				assert.Len(t, returnedRepos, tc.expectedCount)
				if tc.expectedCount > 0 {
					assert.Equal(t, "awesome-repo", returnedRepos[0].Name)
					assert.Equal(t, "owner/awesome-repo", returnedRepos[0].FullName)
				}
			}
		})
	}
}

func Test_StarRepository(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := StarRepository(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "star_repository", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful star",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutUserStarredByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "testowner",
				"repo":  "testrepo",
			},
			expectError: false,
		},
		{
			name: "star fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutUserStarredByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "testowner",
				"repo":  "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to star repository",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := StarRepository(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NotNil(t, result)
				textResult, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "Expected text content")
				assert.Contains(t, textResult.Text, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Parse the result and get the text content
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, "Successfully starred repository")
			}
		})
	}
}

func Test_UnstarRepository(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := UnstarRepository(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "unstar_repository", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful unstar",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.DeleteUserStarredByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "testowner",
				"repo":  "testrepo",
			},
			expectError: false,
		},
		{
			name: "unstar fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.DeleteUserStarredByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "testowner",
				"repo":  "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to unstar repository",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := UnstarRepository(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NotNil(t, result)
				textResult, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "Expected text content")
				assert.Contains(t, textResult.Text, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Parse the result and get the text content
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, "Successfully unstarred repository")
			}
		})
	}
}
