package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createDynamicRequest creates an MCP request with the given arguments for dynamic tools.
func createDynamicRequest(args map[string]any) *mcp.CallToolRequest {
	argsJSON, _ := json.Marshal(args)
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: json.RawMessage(argsJSON),
		},
	}
}

func TestDynamicTools_ListAvailableToolsets(t *testing.T) {
	// Build a registry with no toolsets enabled (dynamic mode)
	reg := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{}).
		Build()

	// Create a mock server
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)

	// Create dynamic tool dependencies
	deps := DynamicToolDependencies{
		Server:    server,
		Inventory: reg,
		ToolDeps:  nil,
		T:         translations.NullTranslationHelper,
	}

	// Get the list_available_toolsets tool
	tool := ListAvailableToolsets()
	handler := tool.Handler(deps)

	// Call the handler
	result, err := handler(context.Background(), createDynamicRequest(map[string]any{}))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	// Parse the result
	var toolsets []map[string]string
	textContent := result.Content[0].(*mcp.TextContent)
	err = json.Unmarshal([]byte(textContent.Text), &toolsets)
	require.NoError(t, err)

	// Verify we got toolsets
	assert.NotEmpty(t, toolsets, "should have available toolsets")

	// Find the repos toolset and verify it's not enabled
	var reposToolset map[string]string
	for _, ts := range toolsets {
		if ts["name"] == "repos" {
			reposToolset = ts
			break
		}
	}
	require.NotNil(t, reposToolset, "repos toolset should exist")
	assert.Equal(t, "false", reposToolset["currently_enabled"], "repos should not be enabled initially")
}

func TestDynamicTools_GetToolsetTools(t *testing.T) {
	// Build a registry with no toolsets enabled (dynamic mode)
	reg := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{}).
		Build()

	// Create a mock server
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)

	// Create dynamic tool dependencies
	deps := DynamicToolDependencies{
		Server:    server,
		Inventory: reg,
		ToolDeps:  nil,
		T:         translations.NullTranslationHelper,
	}

	// Get the get_toolset_tools tool
	tool := GetToolsetsTools(reg)
	handler := tool.Handler(deps)

	// Call the handler for repos toolset
	result, err := handler(context.Background(), createDynamicRequest(map[string]any{
		"toolset": "repos",
	}))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	// Parse the result
	var tools []map[string]string
	textContent := result.Content[0].(*mcp.TextContent)
	err = json.Unmarshal([]byte(textContent.Text), &tools)
	require.NoError(t, err)

	// Verify we got tools for the repos toolset
	assert.NotEmpty(t, tools, "repos toolset should have tools")

	// Verify at least get_commit is there (a repos toolset tool)
	var foundGetCommit bool
	for _, tool := range tools {
		if tool["name"] == "get_commit" {
			foundGetCommit = true
			break
		}
	}
	assert.True(t, foundGetCommit, "get_commit should be in repos toolset")
}

func TestDynamicTools_EnableToolset(t *testing.T) {
	// Build a registry with no toolsets enabled (dynamic mode)
	reg := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{}).
		Build()

	// Create a mock server
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)

	// Create dynamic tool dependencies
	deps := DynamicToolDependencies{
		Server:    server,
		Inventory: reg,
		ToolDeps:  NewBaseDeps(nil, nil, nil, nil, translations.NullTranslationHelper, FeatureFlags{}, 0),
		T:         translations.NullTranslationHelper,
	}

	// Verify repos is not enabled initially
	assert.False(t, reg.IsToolsetEnabled(inventory.ToolsetID("repos")))

	// Get the enable_toolset tool
	tool := EnableToolset(reg)
	handler := tool.Handler(deps)

	// Enable the repos toolset
	result, err := handler(context.Background(), createDynamicRequest(map[string]any{
		"toolset": "repos",
	}))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	// Verify the toolset is now enabled
	assert.True(t, reg.IsToolsetEnabled(inventory.ToolsetID("repos")), "repos should be enabled after enable_toolset")

	// Verify the success message
	textContent := result.Content[0].(*mcp.TextContent)
	assert.Contains(t, textContent.Text, "enabled")

	// Try enabling again - should say already enabled
	result2, err := handler(context.Background(), createDynamicRequest(map[string]any{
		"toolset": "repos",
	}))
	require.NoError(t, err)
	textContent2 := result2.Content[0].(*mcp.TextContent)
	assert.Contains(t, textContent2.Text, "already enabled")
}

func TestDynamicTools_EnableToolset_InvalidToolset(t *testing.T) {
	// Build a registry with no toolsets enabled (dynamic mode)
	reg := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{}).
		Build()

	// Create a mock server
	server := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)

	// Create dynamic tool dependencies
	deps := DynamicToolDependencies{
		Server:    server,
		Inventory: reg,
		ToolDeps:  nil,
		T:         translations.NullTranslationHelper,
	}

	// Get the enable_toolset tool
	tool := EnableToolset(reg)
	handler := tool.Handler(deps)

	// Try to enable a non-existent toolset
	result, err := handler(context.Background(), createDynamicRequest(map[string]any{
		"toolset": "nonexistent",
	}))
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should be an error result
	textContent := result.Content[0].(*mcp.TextContent)
	assert.Contains(t, textContent.Text, "not found")
}

func TestDynamicTools_ToolsetsEnum(t *testing.T) {
	// Build a registry
	reg := NewInventory(translations.NullTranslationHelper).Build()

	// Get tools to verify they have proper enum values
	tools := DynamicTools(reg)

	// Find enable_toolset and get_toolset_tools
	for _, tool := range tools {
		if tool.Tool.Name == "enable_toolset" || tool.Tool.Name == "get_toolset_tools" {
			// Verify the toolset property has an enum
			schema := tool.Tool.InputSchema.(*jsonschema.Schema)
			toolsetProp := schema.Properties["toolset"]
			require.NotNil(t, toolsetProp, "toolset property should exist")
			assert.NotEmpty(t, toolsetProp.Enum, "toolset property should have enum values")

			// Verify repos is in the enum
			var foundRepos bool
			for _, v := range toolsetProp.Enum {
				if v == inventory.ToolsetID("repos") {
					foundRepos = true
					break
				}
			}
			assert.True(t, foundRepos, "repos should be in toolset enum for %s", tool.Tool.Name)
		}
	}
}
