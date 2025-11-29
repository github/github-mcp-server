package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CreateRelease(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := CreateRelease(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "create_release", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "tag_name")
	assert.Contains(t, tool.InputSchema.Properties, "target_commitish")
	assert.Contains(t, tool.InputSchema.Properties, "name")
	assert.Contains(t, tool.InputSchema.Properties, "body")
	assert.Contains(t, tool.InputSchema.Properties, "draft")
	assert.Contains(t, tool.InputSchema.Properties, "prerelease")
	assert.Contains(t, tool.InputSchema.Properties, "generate_release_notes")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "tag_name"})

	// Setup mock release response
	mockRelease := &github.RepositoryRelease{
		ID:         github.Ptr(int64(1)),
		TagName:    github.Ptr("v1.0.0"),
		Name:       github.Ptr("Release v1.0.0"),
		Body:       github.Ptr("This is the release description"),
		Draft:      github.Ptr(false),
		Prerelease: github.Ptr(false),
		HTMLURL:    github.Ptr("https://github.com/owner/repo/releases/tag/v1.0.0"),
		CreatedAt:  &github.Timestamp{Time: time.Now()},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult *github.RepositoryRelease
		expectedErrMsg string
	}{
		{
			name: "successful release creation with all parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposReleasesByOwnerByRepo,
					expectRequestBody(t, map[string]interface{}{
						"tag_name":               "v1.0.0",
						"target_commitish":       "main",
						"name":                   "Release v1.0.0",
						"body":                   "This is the release description",
						"draft":                  false,
						"prerelease":             false,
						"generate_release_notes": true,
					}).andThen(
						mockResponse(t, http.StatusCreated, mockRelease),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":                  "owner",
				"repo":                   "repo",
				"tag_name":               "v1.0.0",
				"target_commitish":       "main",
				"name":                   "Release v1.0.0",
				"body":                   "This is the release description",
				"draft":                  false,
				"prerelease":             false,
				"generate_release_notes": true,
			},
			expectError:    false,
			expectedResult: mockRelease,
		},
		{
			name: "successful release creation with minimal parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposReleasesByOwnerByRepo,
					expectRequestBody(t, map[string]interface{}{
						"tag_name":               "v1.0.0",
						"target_commitish":       "",
						"name":                   "",
						"body":                   "",
						"draft":                  false,
						"prerelease":             false,
						"generate_release_notes": false,
					}).andThen(
						mockResponse(t, http.StatusCreated, mockRelease),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":    "owner",
				"repo":     "repo",
				"tag_name": "v1.0.0",
			},
			expectError:    false,
			expectedResult: mockRelease,
		},
		{
			name: "release creation fails with conflict",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposReleasesByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Validation Failed", "errors": [{"code": "already_exists"}]}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":    "owner",
				"repo":     "repo",
				"tag_name": "v1.0.0",
			},
			expectError:    true,
			expectedErrMsg: "failed to create release",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := CreateRelease(stubGetClientFn(client), translations.NullTranslationHelper)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)
			var returnedRelease github.RepositoryRelease
			err = json.Unmarshal([]byte(textContent.Text), &returnedRelease)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedResult.TagName, *returnedRelease.TagName)
		})
	}
}