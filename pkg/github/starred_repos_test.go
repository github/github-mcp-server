package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v73/github"
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
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"username"})

	// Setup mock starred repositories for success case
	mockStarredRepos := []*github.StarredRepository{
		{
			Repository: &github.Repository{
				ID:       github.Ptr(int64(1)),
				Name:     github.Ptr("awesome-repo"),
				FullName: github.Ptr("owner/awesome-repo"),
				Owner: &github.User{
					Login: github.Ptr("owner"),
				},
				Description:     github.Ptr("An awesome repository"),
				HTMLURL:         github.Ptr("https://github.com/owner/awesome-repo"),
				StargazersCount: github.Ptr(100),
				Language:        github.Ptr("Go"),
				Fork:            github.Ptr(false),
				Private:         github.Ptr(false),
			},
			StarredAt: &github.Timestamp{Time: time.Now().Add(-24 * time.Hour)},
		},
		{
			Repository: &github.Repository{
				ID:       github.Ptr(int64(2)),
				Name:     github.Ptr("cool-project"),
				FullName: github.Ptr("another/cool-project"),
				Owner: &github.User{
					Login: github.Ptr("another"),
				},
				Description:     github.Ptr("A cool project"),
				HTMLURL:         github.Ptr("https://github.com/another/cool-project"),
				StargazersCount: github.Ptr(250),
				Language:        github.Ptr("JavaScript"),
				Fork:            github.Ptr(true),
				Private:         github.Ptr(false),
			},
			StarredAt: &github.Timestamp{Time: time.Now().Add(-48 * time.Hour)},
		},
	}

	tests := []struct {
		name                 string
		mockedClient         *http.Client
		requestArgs          map[string]interface{}
		expectError          bool
		expectedStarredRepos []*github.StarredRepository
		expectedErrMsg       string
	}{
		{
			name: "successful starred repositories retrieval with minimal parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetUsersStarredByUsername,
					mockStarredRepos,
				),
			),
			requestArgs: map[string]interface{}{
				"username": "testuser",
			},
			expectError:          false,
			expectedStarredRepos: mockStarredRepos,
		},
		{
			name: "successful starred repositories retrieval with all parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUsersStarredByUsername,
					expectQueryParams(t, map[string]string{
						"sort":      "created",
						"direction": "desc",
						"page":      "2",
						"per_page":  "50",
					}).andThen(
						mockResponse(t, http.StatusOK, mockStarredRepos),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"username":  "testuser",
				"sort":      "created",
				"direction": "desc",
				"page":      float64(2),
				"perPage":   float64(50),
			},
			expectError:          false,
			expectedStarredRepos: mockStarredRepos,
		},
		{
			name: "successful starred repositories retrieval with sort updated",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUsersStarredByUsername,
					expectQueryParams(t, map[string]string{
						"sort":      "updated",
						"direction": "asc",
						"page":      "1",
						"per_page":  "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockStarredRepos),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"username":  "testuser",
				"sort":      "updated",
				"direction": "asc",
			},
			expectError:          false,
			expectedStarredRepos: mockStarredRepos,
		},
		{
			name: "successful starred repositories retrieval with default perPage",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUsersStarredByUsername,
					expectQueryParams(t, map[string]string{
						"page":     "1",
						"per_page": "30", // Default perPage should be 30
					}).andThen(
						mockResponse(t, http.StatusOK, mockStarredRepos),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"username": "testuser",
				"page":     float64(1),
			},
			expectError:          false,
			expectedStarredRepos: mockStarredRepos,
		},
		{
			name: "empty starred repositories list",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetUsersStarredByUsername,
					[]*github.StarredRepository{},
				),
			),
			requestArgs: map[string]interface{}{
				"username": "userwithnorepos",
			},
			expectError:          false,
			expectedStarredRepos: []*github.StarredRepository{},
		},
		{
			name: "user not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUsersStarredByUsername,
					mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
				),
			),
			requestArgs: map[string]interface{}{
				"username": "nonexistentuser",
			},
			expectError:    true,
			expectedErrMsg: "failed to list starred repositories for user",
		},
		{
			name: "rate limit exceeded",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUsersStarredByUsername,
					mockResponse(t, http.StatusForbidden, `{"message": "API rate limit exceeded"}`),
				),
			),
			requestArgs: map[string]interface{}{
				"username": "testuser",
			},
			expectError:    true,
			expectedErrMsg: "failed to list starred repositories for user",
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
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			if tc.expectedErrMsg != "" {
				require.NotNil(t, result)
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedStarredRepos []*github.StarredRepository
			err = json.Unmarshal([]byte(textContent.Text), &returnedStarredRepos)
			require.NoError(t, err)

			assert.Len(t, returnedStarredRepos, len(tc.expectedStarredRepos))
			for i, starredRepo := range returnedStarredRepos {
				if i < len(tc.expectedStarredRepos) {
					assert.Equal(t, *tc.expectedStarredRepos[i].Repository.ID, *starredRepo.Repository.ID)
					assert.Equal(t, *tc.expectedStarredRepos[i].Repository.Name, *starredRepo.Repository.Name)
					assert.Equal(t, *tc.expectedStarredRepos[i].Repository.FullName, *starredRepo.Repository.FullName)
					assert.Equal(t, *tc.expectedStarredRepos[i].Repository.Owner.Login, *starredRepo.Repository.Owner.Login)
					assert.Equal(t, *tc.expectedStarredRepos[i].Repository.HTMLURL, *starredRepo.Repository.HTMLURL)
					assert.NotNil(t, starredRepo.StarredAt)

					if tc.expectedStarredRepos[i].Repository.Description != nil {
						assert.Equal(t, *tc.expectedStarredRepos[i].Repository.Description, *starredRepo.Repository.Description)
					}
					if tc.expectedStarredRepos[i].Repository.Language != nil {
						assert.Equal(t, *tc.expectedStarredRepos[i].Repository.Language, *starredRepo.Repository.Language)
					}
				}
			}
		})
	}
}
