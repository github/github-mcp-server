package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_UIGet(t *testing.T) {
	// Verify tool definition
	serverTool := UIGet(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "ui_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner"})
	assert.True(t, tool.Annotations.ReadOnlyHint, "ui_get should be read-only")
	assert.True(t, serverTool.InsidersOnly, "ui_get should be insiders only")

	// Setup mock data
	mockAssignees := []*github.User{
		{Login: github.Ptr("user1"), AvatarURL: github.Ptr("https://avatars.githubusercontent.com/u/1")},
		{Login: github.Ptr("user2"), AvatarURL: github.Ptr("https://avatars.githubusercontent.com/u/2")},
	}

	mockBranches := []*github.Branch{
		{Name: github.Ptr("main"), Protected: github.Ptr(true)},
		{Name: github.Ptr("feature"), Protected: github.Ptr(false)},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
		validateResult func(t *testing.T, response map[string]any)
	}{
		{
			name: "successful assignees fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/owner/repo/assignees": mockResponse(t, http.StatusOK, mockAssignees),
			}),
			requestArgs: map[string]any{
				"method": "assignees",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, response map[string]any) {
				assert.Contains(t, response, "assignees")
				assert.Contains(t, response, "totalCount")
			},
		},
		{
			name: "successful branches fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/owner/repo/branches": mockResponse(t, http.StatusOK, mockBranches),
			}),
			requestArgs: map[string]any{
				"method": "branches",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
			validateResult: func(t *testing.T, response map[string]any) {
				assert.Contains(t, response, "branches")
				assert.Contains(t, response, "totalCount")
			},
		},
		{
			name:         "missing method parameter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: method",
		},
		{
			name:         "missing owner parameter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method": "assignees",
				"repo":   "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing repo parameter for assignees",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method": "assignees",
				"owner":  "owner",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: repo",
		},
		{
			name:         "unknown method",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method": "unknown",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError:    true,
			expectedErrMsg: "unknown method: unknown",
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
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
					return
				}
				require.NotNil(t, result)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)

			if tc.validateResult != nil {
				tc.validateResult(t, response)
			}
		})
	}
}
