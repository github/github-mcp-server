package github

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetMe(t *testing.T) {
	t.Parallel()

	tool, _ := GetMe(nil, translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Verify some basic very important properties
	assert.Equal(t, "get_me", tool.Name)
	assert.True(t, *tool.Annotations.ReadOnlyHint, "get_me tool should be read-only")

	// Setup mock user response
	mockUser := &github.User{
		Login:     github.Ptr("testuser"),
		Name:      github.Ptr("Test User"),
		Email:     github.Ptr("test@example.com"),
		Bio:       github.Ptr("GitHub user for testing"),
		Company:   github.Ptr("Test Company"),
		Location:  github.Ptr("Test Location"),
		HTMLURL:   github.Ptr("https://github.com/testuser"),
		CreatedAt: &github.Timestamp{Time: time.Now().Add(-365 * 24 * time.Hour)},
		Type:      github.Ptr("User"),
		Plan: &github.Plan{
			Name: github.Ptr("pro"),
		},
	}

	tests := []struct {
		name               string
		stubbedGetClientFn GetClientFn
		requestArgs        map[string]any
		expectToolError    bool
		expectedUser       *github.User
		expectedToolErrMsg string
	}{
		{
			name: "successful get user",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatch(
						mock.GetUser,
						mockUser,
					),
				),
			),
			requestArgs:     map[string]any{},
			expectToolError: false,
			expectedUser:    mockUser,
		},
		{
			name: "successful get user with reason",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatch(
						mock.GetUser,
						mockUser,
					),
				),
			),
			requestArgs: map[string]any{
				"reason": "Testing API",
			},
			expectToolError: false,
			expectedUser:    mockUser,
		},
		{
			name:               "getting client fails",
			stubbedGetClientFn: stubGetClientFnErr("expected test error"),
			requestArgs:        map[string]any{},
			expectToolError:    true,
			expectedToolErrMsg: "failed to get GitHub client: expected test error",
		},
		{
			name: "get user fails",
			stubbedGetClientFn: stubGetClientFromHTTPFn(
				mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.GetUser,
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
			_, handler := GetMe(tc.stubbedGetClientFn, translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)
			require.NoError(t, err)
			textContent := getTextResult(t, result)

			if tc.expectToolError {
				assert.True(t, result.IsError, "expected tool call result to be an error")
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			// Unmarshal and verify the result
			var returnedUser github.User
			err = json.Unmarshal([]byte(textContent.Text), &returnedUser)
			require.NoError(t, err)

			// Verify user details
			assert.Equal(t, *tc.expectedUser.Login, *returnedUser.Login)
			assert.Equal(t, *tc.expectedUser.Name, *returnedUser.Name)
			assert.Equal(t, *tc.expectedUser.Email, *returnedUser.Email)
			assert.Equal(t, *tc.expectedUser.Bio, *returnedUser.Bio)
			assert.Equal(t, *tc.expectedUser.HTMLURL, *returnedUser.HTMLURL)
			assert.Equal(t, *tc.expectedUser.Type, *returnedUser.Type)
		})
	}
}
