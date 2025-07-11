package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGraphQLToolsIntegration tests that GraphQL tools can be created and called
func TestGraphQLToolsIntegration(t *testing.T) {
	t.Parallel()

	// Create mock clients
	mockHTTPClient := &http.Client{}
	getClient := stubGetClientFromHTTPFn(mockHTTPClient)

	// Test that we can create execute tool without errors
	t.Run("create_tools", func(t *testing.T) {
		executeTool, executeHandler := ExecuteGraphQLQuery(getClient, translations.NullTranslationHelper)

		// Verify tool definitions
		assert.Equal(t, "execute_graphql_query", executeTool.Name)
		assert.NotNil(t, executeHandler)

		// Verify tool schemas have required fields
		assert.Contains(t, executeTool.InputSchema.Properties, "query")
		assert.Contains(t, executeTool.InputSchema.Properties, "variables")

		// Verify required parameters
		assert.Contains(t, executeTool.InputSchema.Required, "query")
	})

	// Test basic invocation of execution tool
	t.Run("invoke_execute_tool", func(t *testing.T) {
		_, handler := ExecuteGraphQLQuery(getClient, translations.NullTranslationHelper)

		request := createMCPRequest(map[string]any{
			"query": `query { viewer { login } }`,
		})

		result, err := handler(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent := getTextResult(t, result)
		var response map[string]interface{}
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		// Should have basic response structure
		assert.Contains(t, response, "query")
		assert.Contains(t, response, "variables")
		assert.Contains(t, response, "success")
	})

	// Test error handling for missing required parameters
	t.Run("error_handling", func(t *testing.T) {
		_, executeHandler := ExecuteGraphQLQuery(getClient, translations.NullTranslationHelper)

		emptyRequest := createMCPRequest(map[string]any{})

		// Execute tool should handle missing query parameter
		executeResult, err := executeHandler(context.Background(), emptyRequest)
		require.NoError(t, err)
		textContent := getTextResult(t, executeResult)
		assert.Contains(t, textContent.Text, "query")
	})
}
