package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
)

func Test_GetUser(t *testing.T) {
	// Verify tool definition once
	serverTool := GetUser(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "get_user", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "username")
	assert.ElementsMatch(t, schema.Required, []string{"username"})

	mockUser := &github.User{
		Login:             github.Ptr("google?"),
		ID:                github.Ptr(int64(1234)),
		HTMLURL:           github.Ptr("https://github.com/non-existent-john-doe"),
		AvatarURL:         github.Ptr("https://github.com/avatar-url/avatar.png"),
		Name:              github.Ptr("John Doe"),
		Company:           github.Ptr("Gophers"),
		Blog:              github.Ptr("https://blog.golang.org"),
		Location:          github.Ptr("Europe/Berlin"),
		Email:             github.Ptr("non-existent-john-doe@gmail.com"),
		Hireable:          github.Ptr(false),
		Bio:               github.Ptr("Just a test user"),
		TwitterUsername:   github.Ptr("non_existent_john_doe"),
		PublicRepos:       github.Ptr(42),
		PublicGists:       github.Ptr(11),
		Followers:         github.Ptr(10),
		Following:         github.Ptr(50),
		CreatedAt:         &github.Timestamp{Time: time.Now().Add(-365 * 24 * time.Hour)},
		UpdatedAt:         &github.Timestamp{Time: time.Now()},
		PrivateGists:      github.Ptr(11),
		TotalPrivateRepos: github.Ptr(int64(5)),
		OwnedPrivateRepos: github.Ptr(int64(3)),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedUser   *github.User
		expectedErrMsg string
	}{
		{
			name: "successful user retrieval by username",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetUserByUsername: mockResponse(t, http.StatusOK, mockUser),
			}),
			requestArgs: map[string]any{
				"username": "non-existent-john-doe",
			},
			expectError:  false,
			expectedUser: mockUser,
		},
		{
			name:         "user not found",
			mockedClient: MockHTTPClientWithHandler(mockResponse(t, http.StatusNotFound, `{"message":"user not found"}`)),
			requestArgs: map[string]any{
				"username": "other-non-existent-john-doe",
			},
			expectError:    true,
			expectedErrMsg: "failed to get user",
		},
		{
			name:         "error getting user",
			mockedClient: MockHTTPClientWithHandler(badRequestHandler("some other error")),
			requestArgs: map[string]any{
				"username": "non-existent-john-doe",
			},
			expectError:    true,
			expectedErrMsg: "failed to get user",
		},
		{
			name:           "missing username parameter",
			mockedClient:   MockHTTPClientWithHandler(badRequestHandler("missing username parameter")),
			requestArgs:    map[string]any{},
			expectError:    true,
			expectedErrMsg: "missing required parameter",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Parse and verify the result
			var returnedUser MinimalUser
			err = json.Unmarshal([]byte(textContent.Text), &returnedUser)
			require.NoError(t, err)

			assert.Equal(t, *tc.expectedUser.Login, returnedUser.Login)
			assert.Equal(t, *tc.expectedUser.ID, returnedUser.ID)
			assert.Equal(t, *tc.expectedUser.HTMLURL, returnedUser.ProfileURL)
			assert.Equal(t, *tc.expectedUser.AvatarURL, returnedUser.AvatarURL)
			// Details
			assert.Equal(t, *tc.expectedUser.Name, returnedUser.Details.Name)
			assert.Equal(t, *tc.expectedUser.Company, returnedUser.Details.Company)
			assert.Equal(t, *tc.expectedUser.Blog, returnedUser.Details.Blog)
			assert.Equal(t, *tc.expectedUser.Location, returnedUser.Details.Location)
			assert.Equal(t, *tc.expectedUser.Email, returnedUser.Details.Email)
			assert.Equal(t, *tc.expectedUser.Hireable, returnedUser.Details.Hireable)
			assert.Equal(t, *tc.expectedUser.Bio, returnedUser.Details.Bio)
			assert.Equal(t, *tc.expectedUser.TwitterUsername, returnedUser.Details.TwitterUsername)
			assert.Equal(t, *tc.expectedUser.PublicRepos, returnedUser.Details.PublicRepos)
			assert.Equal(t, *tc.expectedUser.PublicGists, returnedUser.Details.PublicGists)
			assert.Equal(t, *tc.expectedUser.Followers, returnedUser.Details.Followers)
			assert.Equal(t, *tc.expectedUser.Following, returnedUser.Details.Following)
			assert.Equal(t, *tc.expectedUser.PrivateGists, returnedUser.Details.PrivateGists)
			assert.Equal(t, *tc.expectedUser.TotalPrivateRepos, returnedUser.Details.TotalPrivateRepos)
			assert.Equal(t, *tc.expectedUser.OwnedPrivateRepos, returnedUser.Details.OwnedPrivateRepos)
		})
	}
}
