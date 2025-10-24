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

func Test_ReleaseRead(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ReleaseRead(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "release_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "method")
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "tag")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"method", "owner", "repo"})

	// Setup mock releases
	createdAt := time.Now().Add(-30 * 24 * time.Hour)
	publishedAt := time.Now().Add(-29 * 24 * time.Hour)

	mockReleases := []*github.RepositoryRelease{
		{
			ID:          github.Ptr(int64(1)),
			TagName:     github.Ptr("v1.0.0"),
			Name:        github.Ptr("Version 1.0.0"),
			Body:        github.Ptr("First stable release"),
			Draft:       github.Ptr(false),
			Prerelease:  github.Ptr(false),
			CreatedAt:   &github.Timestamp{Time: createdAt},
			PublishedAt: &github.Timestamp{Time: publishedAt},
			HTMLURL:     github.Ptr("https://github.com/owner/repo/releases/tag/v1.0.0"),
		},
		{
			ID:          github.Ptr(int64(2)),
			TagName:     github.Ptr("v0.9.0"),
			Name:        github.Ptr("Version 0.9.0 Beta"),
			Body:        github.Ptr("Beta release"),
			Draft:       github.Ptr(false),
			Prerelease:  github.Ptr(true),
			CreatedAt:   &github.Timestamp{Time: createdAt.Add(-7 * 24 * time.Hour)},
			PublishedAt: &github.Timestamp{Time: publishedAt.Add(-7 * 24 * time.Hour)},
			HTMLURL:     github.Ptr("https://github.com/owner/repo/releases/tag/v0.9.0"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
		validateResult func(t *testing.T, result *mcp.CallToolResult)
	}{
		{
			name: "list releases successfully",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposReleasesByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(mockReleases))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"method": "list",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, result *mcp.CallToolResult) {
				textContent := getTextResult(t, result)
				var releases []*github.RepositoryRelease
				err := json.Unmarshal([]byte(textContent.Text), &releases)
				require.NoError(t, err)
				assert.Len(t, releases, 2)
				assert.Equal(t, "v1.0.0", *releases[0].TagName)
				assert.Equal(t, "v0.9.0", *releases[1].TagName)
			},
		},
		{
			name: "list releases fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposReleasesByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"method": "list",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list releases",
		},
		{
			name: "get latest release successfully",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposReleasesLatestByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(mockReleases[0]))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"method": "get_latest",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, result *mcp.CallToolResult) {
				textContent := getTextResult(t, result)
				var release github.RepositoryRelease
				err := json.Unmarshal([]byte(textContent.Text), &release)
				require.NoError(t, err)
				assert.Equal(t, "v1.0.0", *release.TagName)
				assert.Equal(t, "Version 1.0.0", *release.Name)
			},
		},
		{
			name: "get latest release fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposReleasesLatestByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"method": "get_latest",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to get latest release",
		},
		{
			name: "get release by tag successfully",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposReleasesTagsByOwnerByRepoByTag,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(mockReleases[1]))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"method": "get_by_tag",
				"owner":  "owner",
				"repo":   "repo",
				"tag":    "v0.9.0",
			},
			expectError: false,
			validateResult: func(t *testing.T, result *mcp.CallToolResult) {
				textContent := getTextResult(t, result)
				var release github.RepositoryRelease
				err := json.Unmarshal([]byte(textContent.Text), &release)
				require.NoError(t, err)
				assert.Equal(t, "v0.9.0", *release.TagName)
				assert.Equal(t, "Version 0.9.0 Beta", *release.Name)
				assert.True(t, *release.Prerelease)
			},
		},
		{
			name: "get release by tag fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposReleasesTagsByOwnerByRepoByTag,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"method": "get_by_tag",
				"owner":  "owner",
				"repo":   "repo",
				"tag":    "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to get release by tag",
		},
		{
			name: "get release by tag missing tag parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"method": "get_by_tag",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError:    true,
			expectedErrMsg: "tag",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ReleaseRead(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			if tc.validateResult != nil {
				tc.validateResult(t, result)
			}
		})
	}
}
