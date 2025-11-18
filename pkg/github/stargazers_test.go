package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListStarredRepositories(t *testing.T) {
	t.Parallel()

	tool, _ := ListStarredRepositories(nil, translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Verify some basic very important properties
	assert.Equal(t, "list_starred_repositories", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint, "list_starred_repositories tool should be read-only")

	// Setup mock starred repositories
	mockRepo := &github.Repository{
		ID:              github.Ptr(int64(123)),
		Name:            github.Ptr("test-repo"),
		FullName:        github.Ptr("testuser/test-repo"),
		Description:     github.Ptr("A test repository"),
		HTMLURL:         github.Ptr("https://github.com/testuser/test-repo"),
		Language:        github.Ptr("Go"),
		StargazersCount: github.Ptr(42),
		ForksCount:      github.Ptr(5),
		OpenIssuesCount: github.Ptr(3),
		Private:         github.Ptr(false),
		Fork:            github.Ptr(false),
		Archived:        github.Ptr(false),
		DefaultBranch:   github.Ptr("main"),
		UpdatedAt:       &github.Timestamp{Time: time.Now()},
	}

	mockStarredRepos := []*github.StarredRepository{
		{
			Repository: mockRepo,
		},
	}

	tests := []struct {
		name               string
		stubbedGetClientFn GetClientFn
		requestArgs        map[string]any
		expectToolError    bool
		expectedRepos      []*github.StarredRepository
		expectedToolErrMsg string
	}{
		{
			name: "successful list for authenticated user",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatch(
						mock.GetUserStarred,
						mockStarredRepos,
					),
				),
			),
			requestArgs:     map[string]any{},
			expectToolError: false,
			expectedRepos:   mockStarredRepos,
		},
		{
			name: "successful list with username",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatch(
						mock.GetUsersStarredByUsername,
						mockStarredRepos,
					),
				),
			),
			requestArgs: map[string]any{
				"username": "testuser",
			},
			expectToolError: false,
			expectedRepos:   mockStarredRepos,
		},
		{
			name: "successful list with pagination",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatch(
						mock.GetUserStarred,
						mockStarredRepos,
					),
				),
			),
			requestArgs: map[string]any{
				"page":    float64(2),
				"perPage": float64(50),
			},
			expectToolError: false,
			expectedRepos:   mockStarredRepos,
		},
		{
			name: "successful list with sort and direction",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatch(
						mock.GetUserStarred,
						mockStarredRepos,
					),
				),
			),
			requestArgs: map[string]any{
				"sort":      "updated",
				"direction": "desc",
			},
			expectToolError: false,
			expectedRepos:   mockStarredRepos,
		},
		{
			name:               "getting client fails",
			stubbedGetClientFn: stubGetClientFnErr("expected test error"),
			requestArgs:        map[string]any{},
			expectToolError:    true,
			expectedToolErrMsg: "failed to get GitHub client: expected test error",
		},
		{
			name: "list starred repos fails",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.GetUserStarred,
						badRequestHandler("expected test failure"),
					),
				),
			),
			requestArgs:        map[string]any{},
			expectToolError:    true,
			expectedToolErrMsg: "expected test failure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, handler := ListStarredRepositories(tc.stubbedGetClientFn, translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, _, err := handler(context.Background(), &request, tc.requestArgs)
			textContent := getTextResult(t, result)

			if tc.expectToolError {
				assert.Error(t, err)
				assert.True(t, result.IsError, "expected tool call result to be an error")
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			// Unmarshal and verify the result
			var returnedRepos []MinimalRepository
			err = json.Unmarshal([]byte(textContent.Text), &returnedRepos)
			require.NoError(t, err)

			// Verify repository details
			assert.Equal(t, len(tc.expectedRepos), len(returnedRepos))
			if len(returnedRepos) > 0 {
				assert.Equal(t, *tc.expectedRepos[0].Repository.ID, returnedRepos[0].ID)
				assert.Equal(t, *tc.expectedRepos[0].Repository.Name, returnedRepos[0].Name)
				assert.Equal(t, *tc.expectedRepos[0].Repository.FullName, returnedRepos[0].FullName)
				assert.Equal(t, *tc.expectedRepos[0].Repository.Description, returnedRepos[0].Description)
				assert.Equal(t, *tc.expectedRepos[0].Repository.HTMLURL, returnedRepos[0].HTMLURL)
			}
		})
	}
}

func Test_StarRepository(t *testing.T) {
	t.Parallel()

	tool, _ := StarRepository(nil, translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Verify some basic very important properties
	assert.Equal(t, "star_repository", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint, "star_repository tool should not be read-only")

	tests := []struct {
		name               string
		stubbedGetClientFn GetClientFn
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful star repository",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.PutUserStarredByOwnerByRepo,
						mockResponse(t, http.StatusNoContent, ""),
					),
				),
			),
			requestArgs: map[string]any{
				"owner": "testuser",
				"repo":  "test-repo",
			},
			expectToolError: false,
		},
		{
			name: "missing owner parameter",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(),
			),
			requestArgs: map[string]any{
				"repo": "test-repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "missing required parameter: owner",
		},
		{
			name: "missing repo parameter",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(),
			),
			requestArgs: map[string]any{
				"owner": "testuser",
			},
			expectToolError:    true,
			expectedToolErrMsg: "missing required parameter: repo",
		},
		{
			name:               "getting client fails",
			stubbedGetClientFn: stubGetClientFnErr("expected test error"),
			requestArgs: map[string]any{
				"owner": "testuser",
				"repo":  "test-repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "failed to get GitHub client: expected test error",
		},
		{
			name: "star repository fails",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.PutUserStarredByOwnerByRepo,
						badRequestHandler("expected test failure"),
					),
				),
			),
			requestArgs: map[string]any{
				"owner": "testuser",
				"repo":  "test-repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "expected test failure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, handler := StarRepository(tc.stubbedGetClientFn, translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, _, err := handler(context.Background(), &request, tc.requestArgs)
			textContent := getTextResult(t, result)

			if tc.expectToolError {
				if tc.expectedToolErrMsg != "" {
					assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				}
				assert.True(t, result.IsError, "expected tool call result to be an error")
				return
			}

			// Verify success message
			assert.NoError(t, err)
			assert.False(t, result.IsError)
			assert.Contains(t, textContent.Text, "Successfully starred repository")
		})
	}
}

func Test_UnstarRepository(t *testing.T) {
	t.Parallel()

	tool, _ := UnstarRepository(nil, translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Verify some basic very important properties
	assert.Equal(t, "unstar_repository", tool.Name)
	assert.False(t, tool.Annotations.ReadOnlyHint, "unstar_repository tool should not be read-only")

	tests := []struct {
		name               string
		stubbedGetClientFn GetClientFn
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful unstar repository",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.DeleteUserStarredByOwnerByRepo,
						mockResponse(t, http.StatusNoContent, ""),
					),
				),
			),
			requestArgs: map[string]any{
				"owner": "testuser",
				"repo":  "test-repo",
			},
			expectToolError: false,
		},
		{
			name: "missing owner parameter",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(),
			),
			requestArgs: map[string]any{
				"repo": "test-repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "missing required parameter: owner",
		},
		{
			name: "missing repo parameter",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(),
			),
			requestArgs: map[string]any{
				"owner": "testuser",
			},
			expectToolError:    true,
			expectedToolErrMsg: "missing required parameter: repo",
		},
		{
			name:               "getting client fails",
			stubbedGetClientFn: stubGetClientFnErr("expected test error"),
			requestArgs: map[string]any{
				"owner": "testuser",
				"repo":  "test-repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "failed to get GitHub client: expected test error",
		},
		{
			name: "unstar repository fails",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.DeleteUserStarredByOwnerByRepo,
						badRequestHandler("expected test failure"),
					),
				),
			),
			requestArgs: map[string]any{
				"owner": "testuser",
				"repo":  "test-repo",
			},
			expectToolError:    true,
			expectedToolErrMsg: "expected test failure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, handler := UnstarRepository(tc.stubbedGetClientFn, translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, _, err := handler(context.Background(), &request, tc.requestArgs)
			textContent := getTextResult(t, result)

			if tc.expectToolError {
				if tc.expectedToolErrMsg != "" {
					assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				}
				assert.True(t, result.IsError, "expected tool call result to be an error")
				return
			}

			// Verify success message
			assert.NoError(t, err)
			assert.False(t, result.IsError)
			assert.Contains(t, textContent.Text, "Successfully unstarred repository")
		})
	}
}
