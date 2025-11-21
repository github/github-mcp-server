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

func Test_EnableToolset_Success(t *testing.T) {
	// Create a toolset group with some toolsets
	toolsetGroup := toolsets.NewToolsetGroup(false)

	toolset1 := toolsets.NewToolset("test_toolset", "Test toolset description")
	toolset1.Enabled = false
	toolsetGroup.AddToolset(toolset1)

	// Create a mock server
	mockServer := &server.MCPServer{}

	tool, handler := EnableToolset(mockServer, toolsetGroup, translations.NullTranslationHelper)

	// Verify tool definition
	assert.Equal(t, "enable_toolset", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, tool.Annotations)
	assert.True(t, *tool.Annotations.ReadOnlyHint)

	// Test enabling a toolset
	request := createMCPRequest(map[string]any{
		"toolset": "test_toolset",
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "enabled")

	// Verify toolset was actually enabled
	assert.True(t, toolset1.Enabled)
}

func Test_EnableToolset_AlreadyEnabled(t *testing.T) {
	toolsetGroup := toolsets.NewToolsetGroup(false)

	toolset1 := toolsets.NewToolset("already_enabled", "Already enabled toolset")
	toolset1.Enabled = true
	toolsetGroup.AddToolset(toolset1)

	mockServer := &server.MCPServer{}

	_, handler := EnableToolset(mockServer, toolsetGroup, translations.NullTranslationHelper)

	request := createMCPRequest(map[string]any{
		"toolset": "already_enabled",
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "already enabled")
}

func Test_EnableToolset_ToolsetNotFound(t *testing.T) {
	toolsetGroup := toolsets.NewToolsetGroup(false)
	mockServer := &server.MCPServer{}

	_, handler := EnableToolset(mockServer, toolsetGroup, translations.NullTranslationHelper)

	request := createMCPRequest(map[string]any{
		"toolset": "nonexistent_toolset",
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "not found")
}

func Test_EnableToolset_MissingParameter(t *testing.T) {
	toolsetGroup := toolsets.NewToolsetGroup(false)
	mockServer := &server.MCPServer{}

	_, handler := EnableToolset(mockServer, toolsetGroup, translations.NullTranslationHelper)

	request := createMCPRequest(map[string]any{})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "missing required parameter")
}

func Test_ListAvailableToolsets(t *testing.T) {
	toolsetGroup := toolsets.NewToolsetGroup(false)

	toolset1 := toolsets.NewToolset("toolset1", "Description 1")
	toolset1.Enabled = true
	toolsetGroup.AddToolset(toolset1)

	toolset2 := toolsets.NewToolset("toolset2", "Description 2")
	toolset2.Enabled = false
	toolsetGroup.AddToolset(toolset2)

	tool, handler := ListAvailableToolsets(toolsetGroup, translations.NullTranslationHelper)

	// Verify tool definition
	assert.Equal(t, "list_available_toolsets", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, tool.Annotations)
	assert.True(t, *tool.Annotations.ReadOnlyHint)

	// Call handler
	request := createMCPRequest(map[string]any{})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent := getTextResult(t, result)

	// Parse JSON response
	var toolsets []map[string]string
	err = json.Unmarshal([]byte(textContent.Text), &toolsets)
	require.NoError(t, err)

	// Should have 2 toolsets
	assert.Len(t, toolsets, 2)

	// Find each toolset
	var ts1, ts2 map[string]string
	for _, ts := range toolsets {
		if ts["name"] == "toolset1" {
			ts1 = ts
		} else if ts["name"] == "toolset2" {
			ts2 = ts
		}
	}

	require.NotNil(t, ts1)
	require.NotNil(t, ts2)

	// Verify toolset1
	assert.Equal(t, "toolset1", ts1["name"])
	assert.Equal(t, "Description 1", ts1["description"])
	assert.Equal(t, "true", ts1["can_enable"])
	assert.Equal(t, "true", ts1["currently_enabled"])

	// Verify toolset2
	assert.Equal(t, "toolset2", ts2["name"])
	assert.Equal(t, "Description 2", ts2["description"])
	assert.Equal(t, "true", ts2["can_enable"])
	assert.Equal(t, "false", ts2["currently_enabled"])
}

func Test_ListAvailableToolsets_EmptyGroup(t *testing.T) {
	toolsetGroup := toolsets.NewToolsetGroup(false)

	_, handler := ListAvailableToolsets(toolsetGroup, translations.NullTranslationHelper)

	request := createMCPRequest(map[string]any{})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent := getTextResult(t, result)

	var toolsetsResult []map[string]string
	err = json.Unmarshal([]byte(textContent.Text), &toolsetsResult)
	require.NoError(t, err)

	assert.Empty(t, toolsetsResult)
}

func Test_GetToolsetsTools(t *testing.T) {
	toolsetGroup := toolsets.NewToolsetGroup(false)

	toolset1 := toolsets.NewToolset("test_toolset", "Test toolset")

	// Add some tools to the toolset
	testTool1 := toolsets.NewServerTool(
		mcp.NewTool("tool1",
			mcp.WithDescription("Tool 1 description"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: ToBoolPtr(true)}),
		),
		nil,
	)
	testTool2 := toolsets.NewServerTool(
		mcp.NewTool("tool2",
			mcp.WithDescription("Tool 2 description"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{ReadOnlyHint: ToBoolPtr(true)}),
		),
		nil,
	)

	toolset1.AddReadTools(testTool1, testTool2)
	toolsetGroup.AddToolset(toolset1)

	tool, handler := GetToolsetsTools(toolsetGroup, translations.NullTranslationHelper)

	// Verify tool definition
	assert.Equal(t, "get_toolset_tools", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, tool.Annotations)
	assert.True(t, *tool.Annotations.ReadOnlyHint)

	// Call handler
	request := createMCPRequest(map[string]any{
		"toolset": "test_toolset",
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent := getTextResult(t, result)

	// Parse JSON response
	var tools []map[string]string
	err = json.Unmarshal([]byte(textContent.Text), &tools)
	require.NoError(t, err)

	// Should have 2 tools
	assert.Len(t, tools, 2)

	// Verify tools
	var tool1Found, tool2Found bool
	for _, tool := range tools {
		if tool["name"] == "tool1" {
			tool1Found = true
			assert.Equal(t, "Tool 1 description", tool["description"])
			assert.Equal(t, "true", tool["can_enable"])
			assert.Equal(t, "test_toolset", tool["toolset"])
		} else if tool["name"] == "tool2" {
			tool2Found = true
			assert.Equal(t, "Tool 2 description", tool["description"])
			assert.Equal(t, "true", tool["can_enable"])
			assert.Equal(t, "test_toolset", tool["toolset"])
		}
	}

	assert.True(t, tool1Found, "tool1 should be in response")
	assert.True(t, tool2Found, "tool2 should be in response")
}

func Test_GetToolsetsTools_ToolsetNotFound(t *testing.T) {
	toolsetGroup := toolsets.NewToolsetGroup(false)

	_, handler := GetToolsetsTools(toolsetGroup, translations.NullTranslationHelper)

	request := createMCPRequest(map[string]any{
		"toolset": "nonexistent",
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "not found")
}

func Test_GetToolsetsTools_MissingParameter(t *testing.T) {
	toolsetGroup := toolsets.NewToolsetGroup(false)

	_, handler := GetToolsetsTools(toolsetGroup, translations.NullTranslationHelper)

	request := createMCPRequest(map[string]any{})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "missing required parameter")
}

func Test_GetToolsetsTools_EmptyToolset(t *testing.T) {
	toolsetGroup := toolsets.NewToolsetGroup(false)

	toolset1 := toolsets.NewToolset("empty_toolset", "Empty toolset")
	toolsetGroup.AddToolset(toolset1)

	_, handler := GetToolsetsTools(toolsetGroup, translations.NullTranslationHelper)

	request := createMCPRequest(map[string]any{
		"toolset": "empty_toolset",
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent := getTextResult(t, result)

	var tools []map[string]string
	err = json.Unmarshal([]byte(textContent.Text), &tools)
	require.NoError(t, err)

	assert.Empty(t, tools)
}

func Test_ToolsetEnum(t *testing.T) {
	toolsetGroup := toolsets.NewToolsetGroup(false)

	toolset1 := toolsets.NewToolset("toolset1", "Description 1")
	toolset2 := toolsets.NewToolset("toolset2", "Description 2")
	toolset3 := toolsets.NewToolset("toolset3", "Description 3")

	toolsetGroup.AddToolset(toolset1)
	toolsetGroup.AddToolset(toolset2)
	toolsetGroup.AddToolset(toolset3)

	enumOption := ToolsetEnum(toolsetGroup)
	require.NotNil(t, enumOption)

	// ToolsetEnum returns a PropertyOption function that we can verify exists
	// We can't easily test its internal behavior without creating a full tool,
	// but we can verify it doesn't panic and returns a non-nil function
	assert.NotNil(t, enumOption)
}
