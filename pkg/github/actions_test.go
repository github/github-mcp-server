package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RunWorkflow(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := RunWorkflow(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "run_workflow", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "workflow_file")
	assert.Contains(t, tool.InputSchema.Properties, "ref")
	assert.Contains(t, tool.InputSchema.Properties, "inputs")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "workflow_file", "ref"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow trigger",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"workflow_file": "main.yaml",
				"ref":           "main",
				"inputs": map[string]any{
					"input1": "value1",
					"input2": "value2",
				},
			},
			expectError: false,
		},
		{
			name:         "missing required parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"workflow_file": "main.yaml",
				// missing ref
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: ref",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := RunWorkflow(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			// Unmarshal and verify the result
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.Equal(t, true, response["success"])
			assert.Equal(t, "Workflow triggered successfully", response["message"])
		})
	}
}
