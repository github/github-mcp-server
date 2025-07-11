package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteGraphQLQuery(t *testing.T) {
	t.Parallel()

	// Verify tool definition
	mockClient := &http.Client{}
	tool, _ := ExecuteGraphQLQuery(stubGetClientFromHTTPFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "execute_graphql_query", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "query")
	assert.Contains(t, tool.InputSchema.Properties, "variables")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"query"})

	// Test basic functionality
	tests := []struct {
		name        string
		requestArgs map[string]any
	}{
		{
			name: "basic query structure",
			requestArgs: map[string]any{
				"query": `query { viewer { login } }`,
			},
		},
		{
			name: "query with variables",
			requestArgs: map[string]any{
				"query": `query($login: String!) { user(login: $login) { login } }`,
				"variables": map[string]any{
					"login": "testuser",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, handler := ExecuteGraphQLQuery(stubGetClientFromHTTPFn(mockClient), translations.NullTranslationHelper)

			request := createMCPRequest(tt.requestArgs)
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.NotNil(t, result)

			textContent := getTextResult(t, result)
			var response map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)

			// Verify that the response contains the expected fields
			assert.Equal(t, tt.requestArgs["query"], response["query"])
			if variables, ok := tt.requestArgs["variables"]; ok {
				assert.Equal(t, variables, response["variables"])
			}

			// The response should have either success=true or success=false
			_, hasSuccess := response["success"]
			assert.True(t, hasSuccess, "Response should have 'success' field")
		})
	}
}

func TestGraphQLToolsRequiredParams(t *testing.T) {
	t.Parallel()

	t.Run("ExecuteGraphQLQuery requires query parameter", func(t *testing.T) {
		mockClient := &http.Client{}
		_, handler := ExecuteGraphQLQuery(stubGetClientFromHTTPFn(mockClient), translations.NullTranslationHelper)

		request := createMCPRequest(map[string]any{})
		result, err := handler(context.Background(), request)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Should return an error result for missing required parameter
		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "query")
	})
}
