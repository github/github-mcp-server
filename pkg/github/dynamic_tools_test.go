package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_EnableToolset(t *testing.T) {
	// Create a mock toolset group
	tsg := toolsets.NewToolsetGroup(false)
	mockToolset := toolsets.NewToolset("mock-toolset", "Mock toolset for testing")
	tsg.AddToolset(mockToolset)

	// Create mock server
	s := NewServer("test", &mcp.ServerOptions{})

	// Verify tool definition
	tool, _ := EnableToolset(s, tsg, translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Validate tool schema
	assert.Equal(t, "enable_toolset", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "enable_toolset tool should be read-only")

	tests := []struct {
		name           string
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successfully enable toolset",
			requestArgs: map[string]interface{}{
				"toolset": "mock-toolset",
			},
			expectError: false,
		},
		{
			name: "toolset not found",
			requestArgs: map[string]interface{}{
				"toolset": "non-existent",
			},
			expectError:    true,
			expectedErrMsg: "Toolset non-existent not found",
		},
		{
			name:        "missing toolset parameter",
			requestArgs: map[string]interface{}{
				// No toolset parameter
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: toolset",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset toolset state
			mockToolset.Enabled = false

			// Create new instances for each test
			s := NewServer("test", &mcp.ServerOptions{})
			tsg := toolsets.NewToolsetGroup(false)
			mockToolset := toolsets.NewToolset("mock-toolset", "Mock toolset for testing")
			tsg.AddToolset(mockToolset)

			_, handler := EnableToolset(s, tsg, translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, _, err := handler(context.Background(), &request, tc.requestArgs)

			// Verify results
			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			assert.Contains(t, textContent.Text, "enabled")
		})
	}
}

func Test_ListAvailableToolsets(t *testing.T) {
	// Create a mock toolset group
	tsg := toolsets.NewToolsetGroup(false)
	mockToolset1 := toolsets.NewToolset("toolset1", "First toolset")
	mockToolset2 := toolsets.NewToolset("toolset2", "Second toolset")
	tsg.AddToolset(mockToolset1)
	tsg.AddToolset(mockToolset2)

	// Verify tool definition
	tool, _ := ListAvailableToolsets(tsg, translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Validate tool schema
	assert.Equal(t, "list_available_toolsets", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "list_available_toolsets tool should be read-only")

	t.Run("successfully list toolsets", func(t *testing.T) {
		_, handler := ListAvailableToolsets(tsg, translations.NullTranslationHelper)

		// Create call request with empty args
		request := createMCPRequest(map[string]interface{}{})

		// Call handler
		result, _, err := handler(context.Background(), &request, map[string]interface{}{})

		// Verify results
		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)

		// Parse JSON response
		var toolsets []map[string]string
		err = json.Unmarshal([]byte(textContent.Text), &toolsets)
		require.NoError(t, err)

		// Verify we have the expected toolsets
		assert.Len(t, toolsets, 2)
		foundToolset1 := false
		foundToolset2 := false
		for _, ts := range toolsets {
			if ts["name"] == "toolset1" {
				foundToolset1 = true
				assert.Equal(t, "First toolset", ts["description"])
				assert.Equal(t, "true", ts["can_enable"])
			}
			if ts["name"] == "toolset2" {
				foundToolset2 = true
				assert.Equal(t, "Second toolset", ts["description"])
				assert.Equal(t, "true", ts["can_enable"])
			}
		}
		assert.True(t, foundToolset1, "Expected to find toolset1")
		assert.True(t, foundToolset2, "Expected to find toolset2")
	})
}

func Test_GetToolsetsTools(t *testing.T) {
	// Create a mock toolset group
	tsg := toolsets.NewToolsetGroup(false)
	mockToolset := toolsets.NewToolset("mock-toolset", "Mock toolset for testing")

	// Add a mock tool to the toolset
	mockTool, mockHandler := GetDependabotAlert(stubGetClientFn(nil), translations.NullTranslationHelper)
	mockToolset.AddReadTools(toolsets.NewServerTool(mockTool, mockHandler))
	tsg.AddToolset(mockToolset)

	// Verify tool definition
	tool, _ := GetToolsetsTools(tsg, translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Validate tool schema
	assert.Equal(t, "get_toolset_tools", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "get_toolset_tools tool should be read-only")

	tests := []struct {
		name           string
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successfully get toolset tools",
			requestArgs: map[string]interface{}{
				"toolset": "mock-toolset",
			},
			expectError: false,
		},
		{
			name: "toolset not found",
			requestArgs: map[string]interface{}{
				"toolset": "non-existent",
			},
			expectError:    true,
			expectedErrMsg: "Toolset non-existent not found",
		},
		{
			name:        "missing toolset parameter",
			requestArgs: map[string]interface{}{
				// No toolset parameter
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: toolset",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, handler := GetToolsetsTools(tsg, translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, _, err := handler(context.Background(), &request, tc.requestArgs)

			// Verify results
			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			// Parse JSON response
			var tools []map[string]string
			err = json.Unmarshal([]byte(textContent.Text), &tools)
			require.NoError(t, err)

			// Verify we have the expected tool
			assert.Len(t, tools, 1)
			assert.Equal(t, "get_dependabot_alert", tools[0]["name"])
			assert.Equal(t, "mock-toolset", tools[0]["toolset"])
		})
	}
}
