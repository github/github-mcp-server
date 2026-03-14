package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetRepository_ToolDefinition(t *testing.T) {
	t.Parallel()

	toolDef := GetRepository(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_repository", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)

	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "url")
	// owner and repo are not in Required when url can substitute
	assert.Empty(t, inputSchema.Required)
}

func Test_GetRepository(t *testing.T) {
	t.Parallel()

	toolDef := GetRepository(translations.NullTranslationHelper)

	createdAt := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	pushedAt := time.Date(2024, 6, 15, 8, 30, 0, 0, time.UTC)

	mockRepo := &github.Repository{
		ID:              github.Ptr(int64(12345)),
		Name:            github.Ptr("react"),
		FullName:        github.Ptr("facebook/react"),
		Description:     github.Ptr("The library for web and native user interfaces."),
		Visibility:      github.Ptr("public"),
		DefaultBranch:   github.Ptr("main"),
		Fork:            github.Ptr(false),
		Archived:        github.Ptr(false),
		IsTemplate:      github.Ptr(false),
		StargazersCount: github.Ptr(200000),
		ForksCount:      github.Ptr(40000),
		OpenIssuesCount: github.Ptr(1000),
		Topics:          []string{"javascript", "react"},
		CloneURL:        github.Ptr("https://github.com/facebook/react.git"),
		SSHURL:          github.Ptr("git@github.com:facebook/react.git"),
		HTMLURL:         github.Ptr("https://github.com/facebook/react"),
		CreatedAt:       &github.Timestamp{Time: createdAt},
		UpdatedAt:       &github.Timestamp{Time: updatedAt},
		PushedAt:        &github.Timestamp{Time: pushedAt},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
		validate       func(t *testing.T, result RepositoryDetails)
	}{
		{
			name: "successful fetch by owner and repo",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposByOwnerByRepo: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(mockRepo)
				},
			}),
			requestArgs: map[string]any{
				"owner": "facebook",
				"repo":  "react",
			},
			validate: func(t *testing.T, result RepositoryDetails) {
				t.Helper()
				assert.Equal(t, int64(12345), result.ID)
				assert.Equal(t, "react", result.Name)
				assert.Equal(t, "facebook/react", result.FullName)
				assert.Equal(t, "public", result.Visibility)
				assert.Equal(t, "main", result.DefaultBranch)
				assert.False(t, result.IsFork)
				assert.False(t, result.IsArchived)
				assert.False(t, result.IsTemplate)
				assert.Equal(t, 200000, result.Stars)
				assert.Equal(t, 40000, result.Forks)
				assert.Equal(t, 1000, result.OpenIssues)
				assert.Equal(t, []string{"javascript", "react"}, result.Topics)
				assert.Equal(t, "https://github.com/facebook/react.git", result.CloneURL)
				assert.Equal(t, "git@github.com:facebook/react.git", result.SSHURL)
				assert.Equal(t, "https://github.com/facebook/react", result.HTMLURL)
				assert.NotEmpty(t, result.CreatedAt)
				assert.NotEmpty(t, result.UpdatedAt)
				assert.NotEmpty(t, result.PushedAt)
			},
		},
		{
			name: "successful fetch via URL param",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposByOwnerByRepo: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(mockRepo)
				},
			}),
			requestArgs: map[string]any{
				"url": "https://github.com/facebook/react",
			},
			validate: func(t *testing.T, result RepositoryDetails) {
				t.Helper()
				assert.Equal(t, "react", result.Name)
				assert.Equal(t, "facebook/react", result.FullName)
			},
		},
		{
			name: "explicit owner takes precedence over URL",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposByOwnerByRepo: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(mockRepo)
				},
			}),
			requestArgs: map[string]any{
				"url":   "https://github.com/facebook/react",
				"owner": "override-owner",
				"repo":  "react",
			},
			validate: func(t *testing.T, result RepositoryDetails) {
				t.Helper()
				// API call succeeded with overridden owner
				assert.Equal(t, "react", result.Name)
			},
		},
		{
			name:         "missing owner and no URL returns error",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"repo": "react",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing repo and no URL returns error",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"owner": "facebook",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: repo",
		},
		{
			name:         "invalid URL returns error",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"url": "not-a-url",
			},
			expectError:    true,
			expectedErrMsg: "invalid GitHub URL",
		},
		{
			name: "API 404 returns tool error",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposByOwnerByRepo: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
				},
			}),
			requestArgs: map[string]any{
				"owner": "facebook",
				"repo":  "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to get repository",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{Client: client}

			handler := toolDef.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expectError {
				assert.True(t, result.IsError, "expected tool error result")
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError, "unexpected tool error: %v", result)
			textContent := getTextResult(t, result)

			var details RepositoryDetails
			require.NoError(t, json.Unmarshal([]byte(textContent.Text), &details))
			tc.validate(t, details)
		})
	}
}
