package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolsetEnum(t *testing.T) {
	tsg := toolsets.NewToolsetGroup(false)

	// Add some toolsets
	tsg.AddToolset(toolsets.NewToolset("toolset1", "Description 1"))
	tsg.AddToolset(toolsets.NewToolset("toolset2", "Description 2"))
	tsg.AddToolset(toolsets.NewToolset("toolset3", "Description 3"))

	option := ToolsetEnum(tsg)

	// The option should be created
	assert.NotNil(t, option)
}

func TestEnableToolset(t *testing.T) {
	tsg := toolsets.NewToolsetGroup(false)

	// Add test toolsets
	toolset1 := toolsets.NewToolset("issues", "GitHub Issues toolset")
	toolset2 := toolsets.NewToolset("pullrequests", "GitHub Pull Requests toolset")
	tsg.AddToolset(toolset1)
	tsg.AddToolset(toolset2)

	mcpServer := server.NewMCPServer("test", "1.0.0")

	tool, handler := EnableToolset(mcpServer, tsg, translations.NullTranslationHelper)

	// Verify tool definition
	assert.Equal(t, "enable_toolset", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, tool.Annotations)
	assert.NotNil(t, tool.Annotations.ReadOnlyHint)
	assert.True(t, *tool.Annotations.ReadOnlyHint, "Enable toolset should be read-only")

	// Verify required parameters
	assert.Contains(t, tool.InputSchema.Required, "toolset")
	assert.Contains(t, tool.InputSchema.Properties, "toolset")

	// Test handler
	assert.NotNil(t, handler)

	tests := []struct {
		name        string
		toolsetName string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "enable existing toolset",
			toolsetName: "issues",
			expectError: false,
		},
		{
			name:        "enable already enabled toolset",
			toolsetName: "issues",
			expectError: false,
		},
		{
			name:        "enable another toolset",
			toolsetName: "pullrequests",
			expectError: false,
		},
		{
			name:        "enable non-existent toolset",
			toolsetName: "nonexistent",
			expectError: true,
			errorMsg:    "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "enable_toolset",
					Arguments: map[string]interface{}{
						"toolset": tt.toolsetName,
					},
				},
			}

			result, err := handler(context.Background(), request)
			require.NoError(t, err, "Handler should not return error")
			require.NotNil(t, result)

			if tt.expectError {
				assert.True(t, result.IsError, "Result should indicate error")
				if tt.errorMsg != "" {
					for _, content := range result.Content {
						if textContent, ok := content.(mcp.TextContent); ok {
							assert.Contains(t, textContent.Text, tt.errorMsg)
						}
					}
				}
			} else {
				assert.False(t, result.IsError, "Result should not indicate error")
			}
		})
	}
}

func TestListAvailableToolsets(t *testing.T) {
	tsg := toolsets.NewToolsetGroup(false)

	// Add test toolsets
	toolset1 := toolsets.NewToolset("issues", "GitHub Issues toolset")
	toolset2 := toolsets.NewToolset("pullrequests", "GitHub Pull Requests toolset")
	toolset3 := toolsets.NewToolset("actions", "GitHub Actions toolset")

	toolset1.Enabled = true
	toolset2.Enabled = false
	toolset3.Enabled = true

	tsg.AddToolset(toolset1)
	tsg.AddToolset(toolset2)
	tsg.AddToolset(toolset3)

	tool, handler := ListAvailableToolsets(tsg, translations.NullTranslationHelper)

	// Verify tool definition
	assert.Equal(t, "list_available_toolsets", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, tool.Annotations)
	assert.NotNil(t, tool.Annotations.ReadOnlyHint)
	assert.True(t, *tool.Annotations.ReadOnlyHint, "List toolsets should be read-only")

	// Test handler
	assert.NotNil(t, handler)

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "list_available_toolsets",
			Arguments: map[string]interface{}{},
		},
	}

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Parse the result
	require.NotEmpty(t, result.Content)

	var toolsetsData []map[string]string
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			err := json.Unmarshal([]byte(textContent.Text), &toolsetsData)
			require.NoError(t, err)
			break
		}
	}

	// Verify we got all toolsets
	assert.Len(t, toolsetsData, 3)

	// Verify each toolset has the required fields
	toolsetMap := make(map[string]map[string]string)
	for _, ts := range toolsetsData {
		assert.Contains(t, ts, "name")
		assert.Contains(t, ts, "description")
		assert.Contains(t, ts, "can_enable")
		assert.Contains(t, ts, "currently_enabled")
		toolsetMap[ts["name"]] = ts
	}

	// Verify specific toolset states
	assert.Equal(t, "true", toolsetMap["issues"]["currently_enabled"])
	assert.Equal(t, "false", toolsetMap["pullrequests"]["currently_enabled"])
	assert.Equal(t, "true", toolsetMap["actions"]["currently_enabled"])

	// All should be enableable
	for _, ts := range toolsetsData {
		assert.Equal(t, "true", ts["can_enable"])
	}
}

func TestGetToolsetsTools(t *testing.T) {
	tsg := toolsets.NewToolsetGroup(false)

	// Create a toolset with some tools
	toolset1 := toolsets.NewToolset("issues", "GitHub Issues toolset")

	// Create mock tools
	readTool := createMockTool("list_issues", true)
	writeTool := createMockTool("create_issue", false)

	toolset1.AddReadTools(readTool)
	toolset1.AddWriteTools(writeTool)

	tsg.AddToolset(toolset1)

	tool, handler := GetToolsetsTools(tsg, translations.NullTranslationHelper)

	// Verify tool definition
	assert.Equal(t, "get_toolset_tools", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, tool.Annotations)
	assert.NotNil(t, tool.Annotations.ReadOnlyHint)
	assert.True(t, *tool.Annotations.ReadOnlyHint, "Get toolset tools should be read-only")

	// Verify required parameters
	assert.Contains(t, tool.InputSchema.Required, "toolset")

	// Test handler
	assert.NotNil(t, handler)

	tests := []struct {
		name         string
		toolsetName  string
		expectError  bool
		expectedTools int
	}{
		{
			name:          "get tools from existing toolset",
			toolsetName:   "issues",
			expectError:   false,
			expectedTools: 2, // list_issues + create_issue
		},
		{
			name:        "get tools from non-existent toolset",
			toolsetName: "nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_toolset_tools",
					Arguments: map[string]interface{}{
						"toolset": tt.toolsetName,
					},
				},
			}

			result, err := handler(context.Background(), request)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.expectError {
				assert.True(t, result.IsError)
			} else {
				assert.False(t, result.IsError)

				// Parse the result
				var toolsData []map[string]string
				for _, content := range result.Content {
					if textContent, ok := content.(mcp.TextContent); ok {
						err := json.Unmarshal([]byte(textContent.Text), &toolsData)
						require.NoError(t, err)
						break
					}
				}

				assert.Len(t, toolsData, tt.expectedTools)

				// Verify each tool has required fields
				for _, toolData := range toolsData {
					assert.Contains(t, toolData, "name")
					assert.Contains(t, toolData, "description")
					assert.Contains(t, toolData, "can_enable")
					assert.Contains(t, toolData, "toolset")
					assert.Equal(t, tt.toolsetName, toolData["toolset"])
				}
			}
		})
	}
}

func TestGetToolsetsTools_EmptyToolset(t *testing.T) {
	tsg := toolsets.NewToolsetGroup(false)

	// Create a toolset with no tools
	emptyToolset := toolsets.NewToolset("empty", "Empty toolset")
	tsg.AddToolset(emptyToolset)

	_, handler := GetToolsetsTools(tsg, translations.NullTranslationHelper)

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_toolset_tools",
			Arguments: map[string]interface{}{
				"toolset": "empty",
			},
		},
	}

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Parse the result
	var toolsData []map[string]string
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			err := json.Unmarshal([]byte(textContent.Text), &toolsData)
			require.NoError(t, err)
			break
		}
	}

	// Should return empty array
	assert.Len(t, toolsData, 0)
}

func TestEnableToolset_Integration(t *testing.T) {
	tsg := toolsets.NewToolsetGroup(false)

	// Add toolsets
	toolset1 := toolsets.NewToolset("issues", "Issues toolset")
	toolset2 := toolsets.NewToolset("pullrequests", "PRs toolset")

	tsg.AddToolset(toolset1)
	tsg.AddToolset(toolset2)

	mcpServer := server.NewMCPServer("test", "1.0.0")

	_, enableHandler := EnableToolset(mcpServer, tsg, translations.NullTranslationHelper)
	_, listHandler := ListAvailableToolsets(tsg, translations.NullTranslationHelper)

	// Initially, toolsets should be disabled
	listResult, err := listHandler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "list_available_toolsets"},
	})
	require.NoError(t, err)
	require.NotNil(t, listResult)

	// Enable the first toolset
	enableResult, err := enableHandler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "enable_toolset",
			Arguments: map[string]interface{}{"toolset": "issues"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, enableResult)
	assert.False(t, enableResult.IsError)

	// Verify toolset is now enabled
	assert.True(t, toolset1.Enabled)
	assert.False(t, toolset2.Enabled)
}

func TestListAvailableToolsets_WithDescriptions(t *testing.T) {
	tsg := toolsets.NewToolsetGroup(false)

	// Add toolsets with specific descriptions
	tsg.AddToolset(toolsets.NewToolset("toolset1", "This is toolset 1"))
	tsg.AddToolset(toolsets.NewToolset("toolset2", "This is toolset 2"))

	_, handler := ListAvailableToolsets(tsg, translations.NullTranslationHelper)

	result, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "list_available_toolsets"},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Parse and verify descriptions
	var toolsetsData []map[string]string
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			err := json.Unmarshal([]byte(textContent.Text), &toolsetsData)
			require.NoError(t, err)
			break
		}
	}

	descriptionMap := make(map[string]string)
	for _, ts := range toolsetsData {
		descriptionMap[ts["name"]] = ts["description"]
	}

	assert.Equal(t, "This is toolset 1", descriptionMap["toolset1"])
	assert.Equal(t, "This is toolset 2", descriptionMap["toolset2"])
}

// Helper function to create a mock tool
func createMockTool(name string, readOnly bool) server.ServerTool {
	tool := mcp.NewTool(name, mcp.WithDescription("Test tool"))
	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint: &readOnly,
	}
	handler := server.ToolHandlerFunc(func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("test"), nil
	})
	return toolsets.NewServerTool(tool, handler)
}

func Test_InitDynamicToolset(t *testing.T) {
	tsg := toolsets.NewToolsetGroup(false)

	// Add some toolsets
	tsg.AddToolset(toolsets.NewToolset("issues", "Issues"))
	tsg.AddToolset(toolsets.NewToolset("pullrequests", "PRs"))

	mcpServer := server.NewMCPServer("test", "1.0.0")

	dynamic := InitDynamicToolset(mcpServer, tsg, translations.NullTranslationHelper)

	// Verify dynamic toolset was created
	assert.NotNil(t, dynamic)
	assert.Equal(t, "dynamic", dynamic.Name)
	assert.NotEmpty(t, dynamic.Description)
}

// stubGetClientFn is defined in server_test.go and reused here
