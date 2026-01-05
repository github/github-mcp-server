package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// testToolsetMetadata returns a ToolsetMetadata for testing
func testToolsetMetadata(id string) ToolsetMetadata {
	return ToolsetMetadata{
		ID:          ToolsetID(id),
		Description: "Test toolset: " + id,
	}
}

// testToolsetMetadataWithDefault returns a ToolsetMetadata with Default flag for testing
func testToolsetMetadataWithDefault(id string, isDefault bool) ToolsetMetadata {
	return ToolsetMetadata{
		ID:          ToolsetID(id),
		Description: "Test toolset: " + id,
		Default:     isDefault,
	}
}

// mockToolWithDefault creates a mock tool with a default toolset flag
func mockToolWithDefault(name string, toolsetID string, readOnly bool, isDefault bool) ServerTool {
	return NewServerToolFromHandler(
		mcp.Tool{
			Name: name,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint: readOnly,
			},
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		testToolsetMetadataWithDefault(toolsetID, isDefault),
		func(_ any) mcp.ToolHandler {
			return func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return nil, nil
			}
		},
	)
}

// mockTool creates a minimal ServerTool for testing
func mockTool(name string, toolsetID string, readOnly bool) ServerTool {
	return NewServerToolFromHandler(
		mcp.Tool{
			Name: name,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint: readOnly,
			},
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		testToolsetMetadata(toolsetID),
		func(_ any) mcp.ToolHandler {
			return func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return nil, nil
			}
		},
	)
}

func TestNewRegistryEmpty(t *testing.T) {
	reg := NewBuilder().Build()
	if len(reg.AvailableTools(context.Background())) != 0 {
		t.Fatalf("Expected tools to be empty")
	}
	if len(reg.AvailableResourceTemplates(context.Background())) != 0 {
		t.Fatalf("Expected resourceTemplates to be empty")
	}
	if len(reg.AvailablePrompts(context.Background())) != 0 {
		t.Fatalf("Expected prompts to be empty")
	}
}

func TestNewRegistryWithTools(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset1", false),
		mockTool("tool3", "toolset2", true),
	}

	reg := NewBuilder().SetTools(tools).Build()

	if len(reg.AllTools()) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(reg.AllTools()))
	}
}

func TestAvailableTools_NoFilters(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool_b", "toolset1", true),
		mockTool("tool_a", "toolset1", false),
		mockTool("tool_c", "toolset2", true),
	}

	reg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	available := reg.AvailableTools(context.Background())

	if len(available) != 3 {
		t.Fatalf("Expected 3 available tools, got %d", len(available))
	}

	// Verify deterministic sorting: by toolset ID, then tool name
	expectedOrder := []string{"tool_a", "tool_b", "tool_c"}
	for i, tool := range available {
		if tool.Tool.Name != expectedOrder[i] {
			t.Errorf("Tool at index %d: expected %s, got %s", i, expectedOrder[i], tool.Tool.Name)
		}
	}
}

func TestWithReadOnly(t *testing.T) {
	tools := []ServerTool{
		mockTool("read_tool", "toolset1", true),
		mockTool("write_tool", "toolset1", false),
	}

	// Build without read-only - should have both tools
	reg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	allTools := reg.AvailableTools(context.Background())
	if len(allTools) != 2 {
		t.Fatalf("Expected 2 tools without read-only, got %d", len(allTools))
	}

	// Build with read-only - should filter out write tools
	readOnlyReg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithReadOnly(true).Build()
	readOnlyTools := readOnlyReg.AvailableTools(context.Background())
	if len(readOnlyTools) != 1 {
		t.Fatalf("Expected 1 tool in read-only, got %d", len(readOnlyTools))
	}
	if readOnlyTools[0].Tool.Name != "read_tool" {
		t.Errorf("Expected read_tool, got %s", readOnlyTools[0].Tool.Name)
	}
}

func TestWithToolsets(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset2", true),
		mockTool("tool3", "toolset3", true),
	}

	// Build with all toolsets
	allReg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	allTools := allReg.AvailableTools(context.Background())
	if len(allTools) != 3 {
		t.Fatalf("Expected 3 tools without filter, got %d", len(allTools))
	}

	// Build with specific toolsets
	filteredReg := NewBuilder().SetTools(tools).WithToolsets([]string{"toolset1", "toolset3"}).Build()
	filteredTools := filteredReg.AvailableTools(context.Background())

	if len(filteredTools) != 2 {
		t.Fatalf("Expected 2 filtered tools, got %d", len(filteredTools))
	}

	// Verify correct tools are included
	toolNames := make(map[string]bool)
	for _, tool := range filteredTools {
		toolNames[tool.Tool.Name] = true
	}
	if !toolNames["tool1"] || !toolNames["tool3"] {
		t.Errorf("Expected tool1 and tool3, got %v", toolNames)
	}
}

func TestWithToolsetsTrimsWhitespace(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset2", true),
	}

	// Whitespace should be trimmed
	filteredReg := NewBuilder().SetTools(tools).WithToolsets([]string{" toolset1 ", "  toolset2  "}).Build()
	filteredTools := filteredReg.AvailableTools(context.Background())

	if len(filteredTools) != 2 {
		t.Fatalf("Expected 2 tools after whitespace trimming, got %d", len(filteredTools))
	}
}

func TestWithToolsetsDeduplicates(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
	}

	// Duplicates should be removed
	filteredReg := NewBuilder().SetTools(tools).WithToolsets([]string{"toolset1", "toolset1", " toolset1 "}).Build()
	filteredTools := filteredReg.AvailableTools(context.Background())

	if len(filteredTools) != 1 {
		t.Fatalf("Expected 1 tool after deduplication, got %d", len(filteredTools))
	}
}

func TestWithToolsetsIgnoresEmptyStrings(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
	}

	// Empty strings should be ignored
	filteredReg := NewBuilder().SetTools(tools).WithToolsets([]string{"", "toolset1", "  ", ""}).Build()
	filteredTools := filteredReg.AvailableTools(context.Background())

	if len(filteredTools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(filteredTools))
	}
}

func TestUnrecognizedToolsets(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset2", true),
	}

	tests := []struct {
		name                 string
		input                []string
		expectedUnrecognized []string
	}{
		{
			name:                 "all valid",
			input:                []string{"toolset1", "toolset2"},
			expectedUnrecognized: nil,
		},
		{
			name:                 "one invalid",
			input:                []string{"toolset1", "invalid_toolset"},
			expectedUnrecognized: []string{"invalid_toolset"},
		},
		{
			name:                 "multiple invalid",
			input:                []string{"typo1", "toolset1", "typo2"},
			expectedUnrecognized: []string{"typo1", "typo2"},
		},
		{
			name:                 "invalid with whitespace trimmed",
			input:                []string{" invalid_tool "},
			expectedUnrecognized: []string{"invalid_tool"},
		},
		{
			name:                 "empty input",
			input:                []string{},
			expectedUnrecognized: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := NewBuilder().SetTools(tools).WithToolsets(tt.input).Build()
			unrecognized := filtered.UnrecognizedToolsets()

			if len(unrecognized) != len(tt.expectedUnrecognized) {
				t.Fatalf("Expected %d unrecognized, got %d: %v",
					len(tt.expectedUnrecognized), len(unrecognized), unrecognized)
			}

			for i, expected := range tt.expectedUnrecognized {
				if unrecognized[i] != expected {
					t.Errorf("Expected unrecognized[%d] = %q, got %q", i, expected, unrecognized[i])
				}
			}
		})
	}
}

func TestWithTools(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset1", true),
		mockTool("tool3", "toolset2", true),
	}

	// WithTools adds additional tools that bypass toolset filtering
	// When combined with WithToolsets([]), only the additional tools should be available
	filteredReg := NewBuilder().SetTools(tools).WithToolsets([]string{}).WithTools([]string{"tool1", "tool3"}).Build()
	filteredTools := filteredReg.AvailableTools(context.Background())

	if len(filteredTools) != 2 {
		t.Fatalf("Expected 2 filtered tools, got %d", len(filteredTools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range filteredTools {
		toolNames[tool.Tool.Name] = true
	}
	if !toolNames["tool1"] || !toolNames["tool3"] {
		t.Errorf("Expected tool1 and tool3, got %v", toolNames)
	}
}

func TestChainedFilters(t *testing.T) {
	tools := []ServerTool{
		mockTool("read1", "toolset1", true),
		mockTool("write1", "toolset1", false),
		mockTool("read2", "toolset2", true),
		mockTool("write2", "toolset2", false),
	}

	// Chain read-only and toolset filter
	filtered := NewBuilder().SetTools(tools).WithReadOnly(true).WithToolsets([]string{"toolset1"}).Build()
	result := filtered.AvailableTools(context.Background())

	if len(result) != 1 {
		t.Fatalf("Expected 1 tool after chained filters, got %d", len(result))
	}
	if result[0].Tool.Name != "read1" {
		t.Errorf("Expected read1, got %s", result[0].Tool.Name)
	}
}

func TestToolsetIDs(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset_b", true),
		mockTool("tool2", "toolset_a", true),
		mockTool("tool3", "toolset_b", true), // duplicate toolset
	}

	reg := NewBuilder().SetTools(tools).Build()
	ids := reg.ToolsetIDs()

	if len(ids) != 2 {
		t.Fatalf("Expected 2 unique toolset IDs, got %d", len(ids))
	}

	// Should be sorted
	if ids[0] != "toolset_a" || ids[1] != "toolset_b" {
		t.Errorf("Expected sorted IDs [toolset_a, toolset_b], got %v", ids)
	}
}

func TestToolsetDescriptions(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset2", true),
	}

	reg := NewBuilder().SetTools(tools).Build()
	descriptions := reg.ToolsetDescriptions()

	if len(descriptions) != 2 {
		t.Fatalf("Expected 2 descriptions, got %d", len(descriptions))
	}

	if descriptions["toolset1"] != "Test toolset: toolset1" {
		t.Errorf("Wrong description for toolset1: %s", descriptions["toolset1"])
	}
}

func TestToolsForToolset(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset1", true),
		mockTool("tool3", "toolset2", true),
	}

	reg := NewBuilder().SetTools(tools).Build()
	toolset1Tools := reg.ToolsForToolset("toolset1")

	if len(toolset1Tools) != 2 {
		t.Fatalf("Expected 2 tools for toolset1, got %d", len(toolset1Tools))
	}
}

func TestWithDeprecatedAliases(t *testing.T) {
	tools := []ServerTool{
		mockTool("new_name", "toolset1", true),
	}

	reg := NewBuilder().SetTools(tools).WithDeprecatedAliases(map[string]string{
		"old_name":  "new_name",
		"get_issue": "issue_read",
	}).Build()

	// Test resolving aliases
	resolved, aliasesUsed := reg.ResolveToolAliases([]string{"old_name"})
	if len(resolved) != 1 || resolved[0] != "new_name" {
		t.Errorf("expected alias to resolve to 'new_name', got %v", resolved)
	}
	if len(aliasesUsed) != 1 || aliasesUsed["old_name"] != "new_name" {
		t.Errorf("expected alias mapping, got %v", aliasesUsed)
	}
}

func TestResolveToolAliases(t *testing.T) {
	tools := []ServerTool{
		mockTool("issue_read", "toolset1", true),
		mockTool("some_tool", "toolset1", true),
	}

	reg := NewBuilder().SetTools(tools).
		WithDeprecatedAliases(map[string]string{
			"get_issue": "issue_read",
		}).Build()

	// Test resolving a mix of aliases and canonical names
	input := []string{"get_issue", "some_tool"}
	resolved, aliasesUsed := reg.ResolveToolAliases(input)

	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved names, got %d", len(resolved))
	}
	if resolved[0] != "issue_read" {
		t.Errorf("expected 'issue_read', got '%s'", resolved[0])
	}
	if resolved[1] != "some_tool" {
		t.Errorf("expected 'some_tool' (unchanged), got '%s'", resolved[1])
	}

	if len(aliasesUsed) != 1 {
		t.Fatalf("expected 1 alias used, got %d", len(aliasesUsed))
	}
	if aliasesUsed["get_issue"] != "issue_read" {
		t.Errorf("expected aliasesUsed['get_issue'] = 'issue_read', got '%s'", aliasesUsed["get_issue"])
	}
}

func TestFindToolByName(t *testing.T) {
	tools := []ServerTool{
		mockTool("issue_read", "toolset1", true),
	}

	reg := NewBuilder().SetTools(tools).Build()

	// Find by name
	tool, toolsetID, err := reg.FindToolByName("issue_read")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tool.Tool.Name != "issue_read" {
		t.Errorf("expected tool name 'issue_read', got '%s'", tool.Tool.Name)
	}
	if toolsetID != "toolset1" {
		t.Errorf("expected toolset ID 'toolset1', got '%s'", toolsetID)
	}

	// Non-existent tool
	_, _, err = reg.FindToolByName("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent tool")
	}
}

func TestWithToolsAdditive(t *testing.T) {
	tools := []ServerTool{
		mockTool("issue_read", "toolset1", true),
		mockTool("issue_write", "toolset1", false),
		mockTool("repo_read", "toolset2", true),
	}

	// Test WithTools bypasses toolset filtering
	// Enable only toolset2, but add issue_read as additional tool
	filtered := NewBuilder().SetTools(tools).WithToolsets([]string{"toolset2"}).WithTools([]string{"issue_read"}).Build()

	available := filtered.AvailableTools(context.Background())
	if len(available) != 2 {
		t.Errorf("expected 2 tools (repo_read from toolset + issue_read additional), got %d", len(available))
	}

	// Verify both tools are present
	toolNames := make(map[string]bool)
	for _, tool := range available {
		toolNames[tool.Tool.Name] = true
	}
	if !toolNames["issue_read"] {
		t.Error("expected issue_read to be included as additional tool")
	}
	if !toolNames["repo_read"] {
		t.Error("expected repo_read to be included from toolset2")
	}

	// Test WithTools respects read-only mode
	readOnlyFiltered := NewBuilder().SetTools(tools).WithReadOnly(true).WithTools([]string{"issue_write"}).Build()
	available = readOnlyFiltered.AvailableTools(context.Background())

	// issue_write should be excluded because read-only applies to additional tools too
	for _, tool := range available {
		if tool.Tool.Name == "issue_write" {
			t.Error("expected issue_write to be excluded in read-only mode")
		}
	}

	// Test WithTools with non-existent tool (should not error, just won't match anything)
	nonexistent := NewBuilder().SetTools(tools).WithToolsets([]string{}).WithTools([]string{"nonexistent"}).Build()
	available = nonexistent.AvailableTools(context.Background())
	if len(available) != 0 {
		t.Errorf("expected 0 tools for non-existent additional tool, got %d", len(available))
	}
}

func TestWithToolsResolvesAliases(t *testing.T) {
	tools := []ServerTool{
		mockTool("issue_read", "toolset1", true),
	}

	// Using deprecated alias should resolve to canonical name
	filtered := NewBuilder().SetTools(tools).
		WithDeprecatedAliases(map[string]string{
			"get_issue": "issue_read",
		}).
		WithToolsets([]string{}).
		WithTools([]string{"get_issue"}).
		Build()
	available := filtered.AvailableTools(context.Background())

	if len(available) != 1 {
		t.Errorf("expected 1 tool, got %d", len(available))
	}
	if available[0].Tool.Name != "issue_read" {
		t.Errorf("expected issue_read, got %s", available[0].Tool.Name)
	}
}

func TestHasToolset(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
	}

	reg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()

	if !reg.HasToolset("toolset1") {
		t.Error("expected HasToolset to return true for existing toolset")
	}
	if reg.HasToolset("nonexistent") {
		t.Error("expected HasToolset to return false for non-existent toolset")
	}
}

func TestEnabledToolsetIDs(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset2", true),
	}

	// Without filter, all toolsets are enabled
	reg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	ids := reg.EnabledToolsetIDs()
	if len(ids) != 2 {
		t.Fatalf("Expected 2 enabled toolset IDs, got %d", len(ids))
	}

	// With filter
	filtered := NewBuilder().SetTools(tools).WithToolsets([]string{"toolset1"}).Build()
	filteredIDs := filtered.EnabledToolsetIDs()
	if len(filteredIDs) != 1 {
		t.Fatalf("Expected 1 enabled toolset ID, got %d", len(filteredIDs))
	}
	if filteredIDs[0] != "toolset1" {
		t.Errorf("Expected toolset1, got %s", filteredIDs[0])
	}
}

func TestAllTools(t *testing.T) {
	tools := []ServerTool{
		mockTool("read_tool", "toolset1", true),
		mockTool("write_tool", "toolset1", false),
	}

	// Even with read-only filter, AllTools returns everything
	readOnlyReg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithReadOnly(true).Build()

	allTools := readOnlyReg.AllTools()
	if len(allTools) != 2 {
		t.Fatalf("Expected 2 tools from AllTools, got %d", len(allTools))
	}

	// But AvailableTools respects the filter
	availableTools := readOnlyReg.AvailableTools(context.Background())
	if len(availableTools) != 1 {
		t.Fatalf("Expected 1 tool from AvailableTools, got %d", len(availableTools))
	}
}

func TestServerToolIsReadOnly(t *testing.T) {
	readTool := mockTool("read_tool", "toolset1", true)
	writeTool := mockTool("write_tool", "toolset1", false)

	if !readTool.IsReadOnly() {
		t.Error("Expected read tool to be read-only")
	}
	if writeTool.IsReadOnly() {
		t.Error("Expected write tool to not be read-only")
	}
}

// mockResource creates a minimal ServerResourceTemplate for testing
func mockResource(name string, toolsetID string, uriTemplate string) ServerResourceTemplate {
	return NewServerResourceTemplate(
		testToolsetMetadata(toolsetID),
		mcp.ResourceTemplate{
			Name:        name,
			URITemplate: uriTemplate,
		},
		func(_ any) mcp.ResourceHandler {
			return func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				return nil, nil
			}
		},
	)
}

// mockPrompt creates a minimal ServerPrompt for testing
func mockPrompt(name string, toolsetID string) ServerPrompt {
	return NewServerPrompt(
		testToolsetMetadata(toolsetID),
		mcp.Prompt{Name: name},
		func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return nil, nil
		},
	)
}

func TestForMCPRequest_Initialize(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "repos", true),
		mockTool("tool2", "issues", false),
	}
	resources := []ServerResourceTemplate{
		mockResource("res1", "repos", "repo://{owner}/{repo}"),
	}
	prompts := []ServerPrompt{
		mockPrompt("prompt1", "repos"),
	}

	reg := NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}).Build()
	filtered := reg.ForMCPRequest(MCPMethodInitialize, "")

	// Initialize should return empty - capabilities come from ServerOptions
	if len(filtered.AvailableTools(context.Background())) != 0 {
		t.Errorf("Expected 0 tools for initialize, got %d", len(filtered.AvailableTools(context.Background())))
	}
	if len(filtered.AvailableResourceTemplates(context.Background())) != 0 {
		t.Errorf("Expected 0 resources for initialize, got %d", len(filtered.AvailableResourceTemplates(context.Background())))
	}
	if len(filtered.AvailablePrompts(context.Background())) != 0 {
		t.Errorf("Expected 0 prompts for initialize, got %d", len(filtered.AvailablePrompts(context.Background())))
	}
}

func TestForMCPRequest_ToolsList(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "repos", true),
		mockTool("tool2", "issues", true),
	}
	resources := []ServerResourceTemplate{
		mockResource("res1", "repos", "repo://{owner}/{repo}"),
	}
	prompts := []ServerPrompt{
		mockPrompt("prompt1", "repos"),
	}

	reg := NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}).Build()
	filtered := reg.ForMCPRequest(MCPMethodToolsList, "")

	// tools/list should return all tools, no resources or prompts
	if len(filtered.AvailableTools(context.Background())) != 2 {
		t.Errorf("Expected 2 tools for tools/list, got %d", len(filtered.AvailableTools(context.Background())))
	}
	if len(filtered.AvailableResourceTemplates(context.Background())) != 0 {
		t.Errorf("Expected 0 resources for tools/list, got %d", len(filtered.AvailableResourceTemplates(context.Background())))
	}
	if len(filtered.AvailablePrompts(context.Background())) != 0 {
		t.Errorf("Expected 0 prompts for tools/list, got %d", len(filtered.AvailablePrompts(context.Background())))
	}
}

func TestForMCPRequest_ToolsCall(t *testing.T) {
	tools := []ServerTool{
		mockTool("get_me", "context", true),
		mockTool("create_issue", "issues", false),
		mockTool("list_repos", "repos", true),
	}

	reg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	filtered := reg.ForMCPRequest(MCPMethodToolsCall, "get_me")

	available := filtered.AvailableTools(context.Background())
	if len(available) != 1 {
		t.Fatalf("Expected 1 tool for tools/call with name, got %d", len(available))
	}
	if available[0].Tool.Name != "get_me" {
		t.Errorf("Expected tool name 'get_me', got %q", available[0].Tool.Name)
	}
}

func TestForMCPRequest_ToolsCall_NotFound(t *testing.T) {
	tools := []ServerTool{
		mockTool("get_me", "context", true),
	}

	reg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	filtered := reg.ForMCPRequest(MCPMethodToolsCall, "nonexistent")

	if len(filtered.AvailableTools(context.Background())) != 0 {
		t.Errorf("Expected 0 tools for nonexistent tool, got %d", len(filtered.AvailableTools(context.Background())))
	}
}

func TestForMCPRequest_ToolsCall_DeprecatedAlias(t *testing.T) {
	tools := []ServerTool{
		mockTool("get_me", "context", true),
		mockTool("list_commits", "repos", true),
	}

	reg := NewBuilder().SetTools(tools).
		WithToolsets([]string{"all"}).
		WithDeprecatedAliases(map[string]string{
			"old_get_me": "get_me",
		}).Build()

	// Request using the deprecated alias
	filtered := reg.ForMCPRequest(MCPMethodToolsCall, "old_get_me")

	available := filtered.AvailableTools(context.Background())
	if len(available) != 1 {
		t.Fatalf("Expected 1 tool when using deprecated alias, got %d", len(available))
	}
	if available[0].Tool.Name != "get_me" {
		t.Errorf("Expected canonical name 'get_me', got %q", available[0].Tool.Name)
	}
}

func TestForMCPRequest_ToolsCall_RespectsFilters(t *testing.T) {
	tools := []ServerTool{
		mockTool("create_issue", "issues", false), // write tool
	}

	// Apply read-only filter at build time, then ForMCPRequest
	reg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithReadOnly(true).Build()
	filtered := reg.ForMCPRequest(MCPMethodToolsCall, "create_issue")

	// The tool exists in the filtered group, but AvailableTools respects read-only
	available := filtered.AvailableTools(context.Background())
	if len(available) != 0 {
		t.Errorf("Expected 0 tools - write tool should be filtered by read-only, got %d", len(available))
	}
}

func TestForMCPRequest_ResourcesList(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "repos", true),
	}
	resources := []ServerResourceTemplate{
		mockResource("res1", "repos", "repo://{owner}/{repo}"),
		mockResource("res2", "repos", "branch://{owner}/{repo}/{branch}"),
	}
	prompts := []ServerPrompt{
		mockPrompt("prompt1", "repos"),
	}

	reg := NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}).Build()
	filtered := reg.ForMCPRequest(MCPMethodResourcesList, "")

	if len(filtered.AvailableTools(context.Background())) != 0 {
		t.Errorf("Expected 0 tools for resources/list, got %d", len(filtered.AvailableTools(context.Background())))
	}
	if len(filtered.AvailableResourceTemplates(context.Background())) != 2 {
		t.Errorf("Expected 2 resources for resources/list, got %d", len(filtered.AvailableResourceTemplates(context.Background())))
	}
	if len(filtered.AvailablePrompts(context.Background())) != 0 {
		t.Errorf("Expected 0 prompts for resources/list, got %d", len(filtered.AvailablePrompts(context.Background())))
	}
}

func TestForMCPRequest_ResourcesRead(t *testing.T) {
	resources := []ServerResourceTemplate{
		mockResource("res1", "repos", "repo://{owner}/{repo}"),
		mockResource("res2", "repos", "branch://{owner}/{repo}/{branch}"),
	}

	reg := NewBuilder().SetResources(resources).WithToolsets([]string{"all"}).Build()
	filtered := reg.ForMCPRequest(MCPMethodResourcesRead, "repo://{owner}/{repo}")

	available := filtered.AvailableResourceTemplates(context.Background())
	if len(available) != 1 {
		t.Fatalf("Expected 1 resource for resources/read, got %d", len(available))
	}
	if available[0].Template.URITemplate != "repo://{owner}/{repo}" {
		t.Errorf("Expected URI template 'repo://{owner}/{repo}', got %q", available[0].Template.URITemplate)
	}
}

func TestForMCPRequest_PromptsList(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "repos", true),
	}
	resources := []ServerResourceTemplate{
		mockResource("res1", "repos", "repo://{owner}/{repo}"),
	}
	prompts := []ServerPrompt{
		mockPrompt("prompt1", "repos"),
		mockPrompt("prompt2", "issues"),
	}

	reg := NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}).Build()
	filtered := reg.ForMCPRequest(MCPMethodPromptsList, "")

	if len(filtered.AvailableTools(context.Background())) != 0 {
		t.Errorf("Expected 0 tools for prompts/list, got %d", len(filtered.AvailableTools(context.Background())))
	}
	if len(filtered.AvailableResourceTemplates(context.Background())) != 0 {
		t.Errorf("Expected 0 resources for prompts/list, got %d", len(filtered.AvailableResourceTemplates(context.Background())))
	}
	if len(filtered.AvailablePrompts(context.Background())) != 2 {
		t.Errorf("Expected 2 prompts for prompts/list, got %d", len(filtered.AvailablePrompts(context.Background())))
	}
}

func TestForMCPRequest_PromptsGet(t *testing.T) {
	prompts := []ServerPrompt{
		mockPrompt("prompt1", "repos"),
		mockPrompt("prompt2", "issues"),
	}

	reg := NewBuilder().SetPrompts(prompts).WithToolsets([]string{"all"}).Build()
	filtered := reg.ForMCPRequest(MCPMethodPromptsGet, "prompt1")

	available := filtered.AvailablePrompts(context.Background())
	if len(available) != 1 {
		t.Fatalf("Expected 1 prompt for prompts/get, got %d", len(available))
	}
	if available[0].Prompt.Name != "prompt1" {
		t.Errorf("Expected prompt name 'prompt1', got %q", available[0].Prompt.Name)
	}
}

func TestForMCPRequest_UnknownMethod(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "repos", true),
	}
	resources := []ServerResourceTemplate{
		mockResource("res1", "repos", "repo://{owner}/{repo}"),
	}
	prompts := []ServerPrompt{
		mockPrompt("prompt1", "repos"),
	}

	reg := NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}).Build()
	filtered := reg.ForMCPRequest("unknown/method", "")

	// Unknown methods should return empty
	if len(filtered.AvailableTools(context.Background())) != 0 {
		t.Errorf("Expected 0 tools for unknown method, got %d", len(filtered.AvailableTools(context.Background())))
	}
	if len(filtered.AvailableResourceTemplates(context.Background())) != 0 {
		t.Errorf("Expected 0 resources for unknown method, got %d", len(filtered.AvailableResourceTemplates(context.Background())))
	}
	if len(filtered.AvailablePrompts(context.Background())) != 0 {
		t.Errorf("Expected 0 prompts for unknown method, got %d", len(filtered.AvailablePrompts(context.Background())))
	}
}

func TestForMCPRequest_DoesNotMutateOriginal(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "repos", true),
		mockTool("tool2", "issues", true),
	}
	resources := []ServerResourceTemplate{
		mockResource("res1", "repos", "repo://{owner}/{repo}"),
	}
	prompts := []ServerPrompt{
		mockPrompt("prompt1", "repos"),
	}

	original := NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}).Build()
	filtered := original.ForMCPRequest(MCPMethodToolsCall, "tool1")

	// Original should be unchanged
	if len(original.AvailableTools(context.Background())) != 2 {
		t.Errorf("Original was mutated! Expected 2 tools, got %d", len(original.AvailableTools(context.Background())))
	}
	if len(original.AvailableResourceTemplates(context.Background())) != 1 {
		t.Errorf("Original was mutated! Expected 1 resource, got %d", len(original.AvailableResourceTemplates(context.Background())))
	}
	if len(original.AvailablePrompts(context.Background())) != 1 {
		t.Errorf("Original was mutated! Expected 1 prompt, got %d", len(original.AvailablePrompts(context.Background())))
	}

	// Filtered should have only the requested tool
	if len(filtered.AvailableTools(context.Background())) != 1 {
		t.Errorf("Expected 1 tool in filtered, got %d", len(filtered.AvailableTools(context.Background())))
	}
	if len(filtered.AvailableResourceTemplates(context.Background())) != 0 {
		t.Errorf("Expected 0 resources in filtered, got %d", len(filtered.AvailableResourceTemplates(context.Background())))
	}
	if len(filtered.AvailablePrompts(context.Background())) != 0 {
		t.Errorf("Expected 0 prompts in filtered, got %d", len(filtered.AvailablePrompts(context.Background())))
	}
}

func TestForMCPRequest_ChainedWithOtherFilters(t *testing.T) {
	tools := []ServerTool{
		mockToolWithDefault("get_me", "context", true, true),        // default toolset
		mockToolWithDefault("create_issue", "issues", false, false), // not default
		mockToolWithDefault("list_repos", "repos", true, true),      // default toolset
		mockToolWithDefault("delete_repo", "repos", false, true),    // default but write
	}

	// Chain: default toolsets -> read-only -> specific method
	reg := NewBuilder().SetTools(tools).
		WithToolsets([]string{"default"}).
		WithReadOnly(true).
		Build()
	filtered := reg.ForMCPRequest(MCPMethodToolsList, "")

	available := filtered.AvailableTools(context.Background())

	// Should have: get_me (context, read), list_repos (repos, read)
	// Should NOT have: create_issue (issues not in default), delete_repo (write)
	if len(available) != 2 {
		t.Fatalf("Expected 2 tools after filter chain, got %d", len(available))
	}

	toolNames := make(map[string]bool)
	for _, tool := range available {
		toolNames[tool.Tool.Name] = true
	}

	if !toolNames["get_me"] {
		t.Error("Expected get_me to be available")
	}
	if !toolNames["list_repos"] {
		t.Error("Expected list_repos to be available")
	}
	if toolNames["create_issue"] {
		t.Error("create_issue should not be available (toolset not enabled)")
	}
	if toolNames["delete_repo"] {
		t.Error("delete_repo should not be available (write tool in read-only mode)")
	}
}

func TestForMCPRequest_ResourcesTemplatesList(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "repos", true),
	}
	resources := []ServerResourceTemplate{
		mockResource("res1", "repos", "repo://{owner}/{repo}"),
	}

	reg := NewBuilder().SetTools(tools).SetResources(resources).WithToolsets([]string{"all"}).Build()
	filtered := reg.ForMCPRequest(MCPMethodResourcesTemplatesList, "")

	// Same behavior as resources/list
	if len(filtered.AvailableTools(context.Background())) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(filtered.AvailableTools(context.Background())))
	}
	if len(filtered.AvailableResourceTemplates(context.Background())) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(filtered.AvailableResourceTemplates(context.Background())))
	}
}

func TestMCPMethodConstants(t *testing.T) {
	// Verify constants match expected MCP method names
	tests := []struct {
		constant string
		expected string
	}{
		{MCPMethodInitialize, "initialize"},
		{MCPMethodToolsList, "tools/list"},
		{MCPMethodToolsCall, "tools/call"},
		{MCPMethodResourcesList, "resources/list"},
		{MCPMethodResourcesRead, "resources/read"},
		{MCPMethodResourcesTemplatesList, "resources/templates/list"},
		{MCPMethodPromptsList, "prompts/list"},
		{MCPMethodPromptsGet, "prompts/get"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("Constant mismatch: got %q, expected %q", tt.constant, tt.expected)
		}
	}
}

// mockToolWithFlags creates a ServerTool with feature flags for testing
func mockToolWithFlags(name string, toolsetID string, readOnly bool, enableFlag, disableFlag string) ServerTool {
	tool := mockTool(name, toolsetID, readOnly)
	tool.FeatureFlagEnable = enableFlag
	tool.FeatureFlagDisable = disableFlag
	return tool
}

func TestFeatureFlagEnable(t *testing.T) {
	tools := []ServerTool{
		mockTool("always_available", "toolset1", true),
		mockToolWithFlags("needs_flag", "toolset1", true, "my_feature", ""),
	}

	// Without feature checker, tool with FeatureFlagEnable should be excluded
	reg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	available := reg.AvailableTools(context.Background())
	if len(available) != 1 {
		t.Fatalf("Expected 1 tool without feature checker, got %d", len(available))
	}
	if available[0].Tool.Name != "always_available" {
		t.Errorf("Expected always_available, got %s", available[0].Tool.Name)
	}

	// With feature checker returning false, tool should still be excluded
	checkerFalse := func(_ context.Context, _ string) (bool, error) { return false, nil }
	regFalse := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checkerFalse).Build()
	availableFalse := regFalse.AvailableTools(context.Background())
	if len(availableFalse) != 1 {
		t.Fatalf("Expected 1 tool with false checker, got %d", len(availableFalse))
	}

	// With feature checker returning true for "my_feature", tool should be included
	checkerTrue := func(_ context.Context, flag string) (bool, error) {
		return flag == "my_feature", nil
	}
	regTrue := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checkerTrue).Build()
	availableTrue := regTrue.AvailableTools(context.Background())
	if len(availableTrue) != 2 {
		t.Fatalf("Expected 2 tools with true checker, got %d", len(availableTrue))
	}
}

func TestFeatureFlagDisable(t *testing.T) {
	tools := []ServerTool{
		mockTool("always_available", "toolset1", true),
		mockToolWithFlags("disabled_by_flag", "toolset1", true, "", "kill_switch"),
	}

	// Without feature checker, tool with FeatureFlagDisable should be included (flag is false)
	reg := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	available := reg.AvailableTools(context.Background())
	if len(available) != 2 {
		t.Fatalf("Expected 2 tools without feature checker, got %d", len(available))
	}

	// With feature checker returning true for "kill_switch", tool should be excluded
	checkerTrue := func(_ context.Context, flag string) (bool, error) {
		return flag == "kill_switch", nil
	}
	regFiltered := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checkerTrue).Build()
	availableFiltered := regFiltered.AvailableTools(context.Background())
	if len(availableFiltered) != 1 {
		t.Fatalf("Expected 1 tool with kill_switch enabled, got %d", len(availableFiltered))
	}
	if availableFiltered[0].Tool.Name != "always_available" {
		t.Errorf("Expected always_available, got %s", availableFiltered[0].Tool.Name)
	}
}

func TestFeatureFlagBoth(t *testing.T) {
	// Tool that requires "new_feature" AND is disabled by "kill_switch"
	tools := []ServerTool{
		mockToolWithFlags("complex_tool", "toolset1", true, "new_feature", "kill_switch"),
	}

	// Enable flag not set -> excluded
	checker1 := func(_ context.Context, _ string) (bool, error) { return false, nil }
	reg1 := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checker1).Build()
	if len(reg1.AvailableTools(context.Background())) != 0 {
		t.Error("Tool should be excluded when enable flag is false")
	}

	// Enable flag set, disable flag not set -> included
	checker2 := func(_ context.Context, flag string) (bool, error) { return flag == "new_feature", nil }
	reg2 := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checker2).Build()
	if len(reg2.AvailableTools(context.Background())) != 1 {
		t.Error("Tool should be included when enable flag is true and disable flag is false")
	}

	// Enable flag set, disable flag also set -> excluded (disable wins)
	checker3 := func(_ context.Context, _ string) (bool, error) { return true, nil }
	reg3 := NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checker3).Build()
	if len(reg3.AvailableTools(context.Background())) != 0 {
		t.Error("Tool should be excluded when both flags are true (disable wins)")
	}
}

func TestFeatureFlagError(t *testing.T) {
	tools := []ServerTool{
		mockToolWithFlags("needs_flag", "toolset1", true, "my_feature", ""),
	}

	// Checker that returns error should treat as false (tool excluded)
	checkerError := func(_ context.Context, _ string) (bool, error) {
		return false, fmt.Errorf("simulated error")
	}
	reg := NewBuilder().SetTools(tools).WithFeatureChecker(checkerError).Build()
	available := reg.AvailableTools(context.Background())
	if len(available) != 0 {
		t.Errorf("Expected 0 tools when checker errors, got %d", len(available))
	}
}

func TestFeatureFlagResources(t *testing.T) {
	resources := []ServerResourceTemplate{
		mockResource("always_available", "toolset1", "uri1"),
		{
			Template:          mcp.ResourceTemplate{Name: "needs_flag", URITemplate: "uri2"},
			Toolset:           testToolsetMetadata("toolset1"),
			FeatureFlagEnable: "my_feature",
		},
	}

	// Without checker, resource with enable flag should be excluded
	reg := NewBuilder().SetResources(resources).WithToolsets([]string{"all"}).Build()
	available := reg.AvailableResourceTemplates(context.Background())
	if len(available) != 1 {
		t.Fatalf("Expected 1 resource without checker, got %d", len(available))
	}

	// With checker returning true, both should be included
	checker := func(_ context.Context, _ string) (bool, error) { return true, nil }
	regWithChecker := NewBuilder().SetResources(resources).WithToolsets([]string{"all"}).WithFeatureChecker(checker).Build()
	if len(regWithChecker.AvailableResourceTemplates(context.Background())) != 2 {
		t.Errorf("Expected 2 resources with checker, got %d", len(regWithChecker.AvailableResourceTemplates(context.Background())))
	}
}

func TestFeatureFlagPrompts(t *testing.T) {
	prompts := []ServerPrompt{
		mockPrompt("always_available", "toolset1"),
		{
			Prompt:            mcp.Prompt{Name: "needs_flag"},
			Toolset:           testToolsetMetadata("toolset1"),
			FeatureFlagEnable: "my_feature",
		},
	}

	// Without checker, prompt with enable flag should be excluded
	reg := NewBuilder().SetPrompts(prompts).WithToolsets([]string{"all"}).Build()
	available := reg.AvailablePrompts(context.Background())
	if len(available) != 1 {
		t.Fatalf("Expected 1 prompt without checker, got %d", len(available))
	}

	// With checker returning true, both should be included
	checker := func(_ context.Context, _ string) (bool, error) { return true, nil }
	regWithChecker := NewBuilder().SetPrompts(prompts).WithToolsets([]string{"all"}).WithFeatureChecker(checker).Build()
	if len(regWithChecker.AvailablePrompts(context.Background())) != 2 {
		t.Errorf("Expected 2 prompts with checker, got %d", len(regWithChecker.AvailablePrompts(context.Background())))
	}
}

func TestServerToolHasHandler(t *testing.T) {
	// Tool with handler
	toolWithHandler := mockTool("has_handler", "toolset1", true)
	if !toolWithHandler.HasHandler() {
		t.Error("Expected HasHandler() to return true for tool with handler")
	}

	// Tool without handler
	toolWithoutHandler := ServerTool{
		Tool:    mcp.Tool{Name: "no_handler"},
		Toolset: testToolsetMetadata("toolset1"),
	}
	if toolWithoutHandler.HasHandler() {
		t.Error("Expected HasHandler() to return false for tool without handler")
	}
}

func TestServerToolHandlerPanicOnNil(t *testing.T) {
	tool := ServerTool{
		Tool:    mcp.Tool{Name: "no_handler"},
		Toolset: testToolsetMetadata("toolset1"),
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected Handler() to panic when HandlerFunc is nil")
		}
	}()

	tool.Handler(nil)
}

// Tests for Enabled function on ServerTool
func TestServerToolEnabled(t *testing.T) {
	tests := []struct {
		name           string
		enabledFunc    func(ctx context.Context) (bool, error)
		expectedCount  int
		expectInResult bool
	}{
		{
			name:           "nil Enabled function - tool included",
			enabledFunc:    nil,
			expectedCount:  1,
			expectInResult: true,
		},
		{
			name: "Enabled returns true - tool included",
			enabledFunc: func(_ context.Context) (bool, error) {
				return true, nil
			},
			expectedCount:  1,
			expectInResult: true,
		},
		{
			name: "Enabled returns false - tool excluded",
			enabledFunc: func(_ context.Context) (bool, error) {
				return false, nil
			},
			expectedCount:  0,
			expectInResult: false,
		},
		{
			name: "Enabled returns error - tool excluded",
			enabledFunc: func(_ context.Context) (bool, error) {
				return false, fmt.Errorf("simulated error")
			},
			expectedCount:  0,
			expectInResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := mockTool("test_tool", "toolset1", true)
			tool.Enabled = tt.enabledFunc

			reg := NewBuilder().SetTools([]ServerTool{tool}).WithToolsets([]string{"all"}).Build()
			available := reg.AvailableTools(context.Background())

			if len(available) != tt.expectedCount {
				t.Errorf("Expected %d tools, got %d", tt.expectedCount, len(available))
			}

			found := false
			for _, t := range available {
				if t.Tool.Name == "test_tool" {
					found = true
					break
				}
			}
			if found != tt.expectInResult {
				t.Errorf("Expected tool in result: %v, got: %v", tt.expectInResult, found)
			}
		})
	}
}

func TestServerToolEnabledWithContext(t *testing.T) {
	type contextKey string
	const userKey contextKey = "user"

	// Tool that checks context for user
	tool := mockTool("context_aware_tool", "toolset1", true)
	tool.Enabled = func(ctx context.Context) (bool, error) {
		user := ctx.Value(userKey)
		return user != nil && user.(string) == "authorized", nil
	}

	reg := NewBuilder().SetTools([]ServerTool{tool}).WithToolsets([]string{"all"}).Build()

	// Without user in context - tool should be excluded
	available := reg.AvailableTools(context.Background())
	if len(available) != 0 {
		t.Errorf("Expected 0 tools without user, got %d", len(available))
	}

	// With authorized user - tool should be included
	ctxWithUser := context.WithValue(context.Background(), userKey, "authorized")
	availableWithUser := reg.AvailableTools(ctxWithUser)
	if len(availableWithUser) != 1 {
		t.Errorf("Expected 1 tool with authorized user, got %d", len(availableWithUser))
	}

	// With unauthorized user - tool should be excluded
	ctxWithBadUser := context.WithValue(context.Background(), userKey, "unauthorized")
	availableWithBadUser := reg.AvailableTools(ctxWithBadUser)
	if len(availableWithBadUser) != 0 {
		t.Errorf("Expected 0 tools with unauthorized user, got %d", len(availableWithBadUser))
	}
}

// Tests for WithFilter builder method
func TestBuilderWithFilter(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset1", true),
		mockTool("tool3", "toolset1", true),
	}

	// Filter that excludes tool2
	filter := func(_ context.Context, tool *ServerTool) (bool, error) {
		return tool.Tool.Name != "tool2", nil
	}

	reg := NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFilter(filter).
		Build()

	available := reg.AvailableTools(context.Background())
	if len(available) != 2 {
		t.Fatalf("Expected 2 tools after filter, got %d", len(available))
	}

	for _, tool := range available {
		if tool.Tool.Name == "tool2" {
			t.Error("tool2 should have been filtered out")
		}
	}
}

func TestBuilderWithMultipleFilters(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset1", true),
		mockTool("tool3", "toolset1", true),
		mockTool("tool4", "toolset1", true),
	}

	// First filter excludes tool2
	filter1 := func(_ context.Context, tool *ServerTool) (bool, error) {
		return tool.Tool.Name != "tool2", nil
	}

	// Second filter excludes tool3
	filter2 := func(_ context.Context, tool *ServerTool) (bool, error) {
		return tool.Tool.Name != "tool3", nil
	}

	reg := NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFilter(filter1).
		WithFilter(filter2).
		Build()

	available := reg.AvailableTools(context.Background())
	if len(available) != 2 {
		t.Fatalf("Expected 2 tools after multiple filters, got %d", len(available))
	}

	toolNames := make(map[string]bool)
	for _, tool := range available {
		toolNames[tool.Tool.Name] = true
	}

	if !toolNames["tool1"] || !toolNames["tool4"] {
		t.Error("Expected tool1 and tool4 to be available")
	}
	if toolNames["tool2"] || toolNames["tool3"] {
		t.Error("tool2 and tool3 should have been filtered out")
	}
}

func TestBuilderFilterError(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
	}

	// Filter that returns an error
	filter := func(_ context.Context, _ *ServerTool) (bool, error) {
		return false, fmt.Errorf("filter error")
	}

	reg := NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFilter(filter).
		Build()

	available := reg.AvailableTools(context.Background())
	if len(available) != 0 {
		t.Errorf("Expected 0 tools when filter returns error, got %d", len(available))
	}
}

func TestBuilderFilterWithContext(t *testing.T) {
	type contextKey string
	const scopeKey contextKey = "scope"

	tools := []ServerTool{
		mockTool("public_tool", "toolset1", true),
		mockTool("private_tool", "toolset1", true),
	}

	// Filter that checks context for scope
	filter := func(ctx context.Context, tool *ServerTool) (bool, error) {
		scope := ctx.Value(scopeKey)
		if scope == "public" && tool.Tool.Name == "private_tool" {
			return false, nil
		}
		return true, nil
	}

	reg := NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFilter(filter).
		Build()

	// With public scope - private_tool should be excluded
	ctxPublic := context.WithValue(context.Background(), scopeKey, "public")
	availablePublic := reg.AvailableTools(ctxPublic)
	if len(availablePublic) != 1 {
		t.Fatalf("Expected 1 tool with public scope, got %d", len(availablePublic))
	}
	if availablePublic[0].Tool.Name != "public_tool" {
		t.Error("Expected only public_tool to be available")
	}

	// With private scope - both tools should be available
	ctxPrivate := context.WithValue(context.Background(), scopeKey, "private")
	availablePrivate := reg.AvailableTools(ctxPrivate)
	if len(availablePrivate) != 2 {
		t.Errorf("Expected 2 tools with private scope, got %d", len(availablePrivate))
	}
}

// Tests for interaction between Enabled, feature flags, and filters
func TestEnabledAndFeatureFlagInteraction(t *testing.T) {
	// Tool with both Enabled function and feature flag
	tool := mockToolWithFlags("complex_tool", "toolset1", true, "my_feature", "")
	tool.Enabled = func(_ context.Context) (bool, error) {
		return true, nil
	}

	// Feature flag not enabled - tool should be excluded despite Enabled returning true
	reg1 := NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		Build()
	available1 := reg1.AvailableTools(context.Background())
	if len(available1) != 0 {
		t.Error("Tool should be excluded when feature flag is not enabled")
	}

	// Feature flag enabled - tool should be included
	checker := func(_ context.Context, flag string) (bool, error) {
		return flag == "my_feature", nil
	}
	reg2 := NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker).
		Build()
	available2 := reg2.AvailableTools(context.Background())
	if len(available2) != 1 {
		t.Error("Tool should be included when both Enabled and feature flag pass")
	}

	// Enabled returns false - tool should be excluded despite feature flag
	tool.Enabled = func(_ context.Context) (bool, error) {
		return false, nil
	}
	reg3 := NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker).
		Build()
	available3 := reg3.AvailableTools(context.Background())
	if len(available3) != 0 {
		t.Error("Tool should be excluded when Enabled returns false")
	}
}

func TestEnabledAndBuilderFilterInteraction(t *testing.T) {
	tool := mockTool("test_tool", "toolset1", true)
	tool.Enabled = func(_ context.Context) (bool, error) {
		return true, nil
	}

	// Filter that excludes the tool
	filter := func(_ context.Context, _ *ServerTool) (bool, error) {
		return false, nil
	}

	reg := NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFilter(filter).
		Build()

	available := reg.AvailableTools(context.Background())
	if len(available) != 0 {
		t.Error("Tool should be excluded when filter returns false, despite Enabled returning true")
	}
}

func TestAllFiltersInteraction(t *testing.T) {
	// Tool with Enabled, feature flag, and subject to builder filter
	tool := mockToolWithFlags("complex_tool", "toolset1", true, "my_feature", "")
	tool.Enabled = func(_ context.Context) (bool, error) {
		return true, nil
	}

	filter := func(_ context.Context, _ *ServerTool) (bool, error) {
		return true, nil
	}

	checker := func(_ context.Context, flag string) (bool, error) {
		return flag == "my_feature", nil
	}

	// All conditions pass - tool should be included
	reg := NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker).
		WithFilter(filter).
		Build()

	available := reg.AvailableTools(context.Background())
	if len(available) != 1 {
		t.Error("Tool should be included when all filters pass")
	}

	// Change filter to return false - tool should be excluded
	filterFalse := func(_ context.Context, _ *ServerTool) (bool, error) {
		return false, nil
	}

	reg2 := NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker).
		WithFilter(filterFalse).
		Build()

	available2 := reg2.AvailableTools(context.Background())
	if len(available2) != 0 {
		t.Error("Tool should be excluded when any filter fails")
	}
}

// Test FilteredTools method
func TestFilteredTools(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset1", true),
	}

	filter := func(_ context.Context, tool *ServerTool) (bool, error) {
		return tool.Tool.Name == "tool1", nil
	}

	reg := NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFilter(filter).
		Build()

	filtered, err := reg.FilteredTools(context.Background())
	if err != nil {
		t.Fatalf("FilteredTools returned error: %v", err)
	}

	if len(filtered) != 1 {
		t.Fatalf("Expected 1 filtered tool, got %d", len(filtered))
	}

	if filtered[0].Tool.Name != "tool1" {
		t.Errorf("Expected tool1, got %s", filtered[0].Tool.Name)
	}
}

func TestFilteredToolsMatchesAvailableTools(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset1", false),
		mockTool("tool3", "toolset2", true),
	}

	reg := NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"toolset1"}).
		WithReadOnly(true).
		Build()

	ctx := context.Background()
	filtered, err := reg.FilteredTools(ctx)
	if err != nil {
		t.Fatalf("FilteredTools returned error: %v", err)
	}

	available := reg.AvailableTools(ctx)

	// Both methods should return the same results
	if len(filtered) != len(available) {
		t.Errorf("FilteredTools and AvailableTools returned different counts: %d vs %d",
			len(filtered), len(available))
	}

	for i := range filtered {
		if filtered[i].Tool.Name != available[i].Tool.Name {
			t.Errorf("Tool at index %d differs: FilteredTools=%s, AvailableTools=%s",
				i, filtered[i].Tool.Name, available[i].Tool.Name)
		}
	}
}

func TestFilteringOrder(t *testing.T) {
	// Test that filters are applied in the correct order:
	// 1. Tool.Enabled
	// 2. Feature flags
	// 3. Read-only
	// 4. Builder filters
	// 5. Toolset/additional tools

	callOrder := []string{}

	tool := mockToolWithFlags("test_tool", "toolset1", false, "my_feature", "")
	tool.Enabled = func(_ context.Context) (bool, error) {
		callOrder = append(callOrder, "Enabled")
		return true, nil
	}

	filter := func(_ context.Context, _ *ServerTool) (bool, error) {
		callOrder = append(callOrder, "Filter")
		return true, nil
	}

	checker := func(_ context.Context, _ string) (bool, error) {
		callOrder = append(callOrder, "FeatureFlag")
		return true, nil
	}

	reg := NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithReadOnly(true). // This will exclude the tool (it's not read-only)
		WithFeatureChecker(checker).
		WithFilter(filter).
		Build()

	_ = reg.AvailableTools(context.Background())

	// Expected order: Enabled, FeatureFlag, ReadOnly (stops here because it's write tool)
	expectedOrder := []string{"Enabled", "FeatureFlag"}
	if len(callOrder) != len(expectedOrder) {
		t.Errorf("Expected %d checks, got %d: %v", len(expectedOrder), len(callOrder), callOrder)
	}

	for i, expected := range expectedOrder {
		if i >= len(callOrder) || callOrder[i] != expected {
			t.Errorf("At position %d: expected %s, got %v", i, expected, callOrder)
		}
	}
}

func TestForMCPRequest_ToolsCall_FeatureFlaggedVariants(t *testing.T) {
	// Simulate the get_job_logs scenario: two tools with the same name but different feature flags
	// - "get_job_logs" with FeatureFlagDisable (available when flag is OFF)
	// - "get_job_logs" with FeatureFlagEnable (available when flag is ON)
	tools := []ServerTool{
		mockToolWithFlags("get_job_logs", "actions", true, "", "consolidated_flag"), // disabled when flag is ON
		mockToolWithFlags("get_job_logs", "actions", true, "consolidated_flag", ""), // enabled when flag is ON
		mockTool("other_tool", "actions", true),
	}

	// Test 1: Flag is OFF - first tool variant should be available
	regFlagOff := NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		Build()
	filteredOff := regFlagOff.ForMCPRequest(MCPMethodToolsCall, "get_job_logs")
	availableOff := filteredOff.AvailableTools(context.Background())
	if len(availableOff) != 1 {
		t.Fatalf("Flag OFF: Expected 1 tool, got %d", len(availableOff))
	}
	if availableOff[0].FeatureFlagDisable != "consolidated_flag" {
		t.Errorf("Flag OFF: Expected tool with FeatureFlagDisable, got FeatureFlagEnable=%q, FeatureFlagDisable=%q",
			availableOff[0].FeatureFlagEnable, availableOff[0].FeatureFlagDisable)
	}

	// Test 2: Flag is ON - second tool variant should be available
	checker := func(_ context.Context, flag string) (bool, error) {
		return flag == "consolidated_flag", nil
	}
	regFlagOn := NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker).
		Build()
	filteredOn := regFlagOn.ForMCPRequest(MCPMethodToolsCall, "get_job_logs")
	availableOn := filteredOn.AvailableTools(context.Background())
	if len(availableOn) != 1 {
		t.Fatalf("Flag ON: Expected 1 tool, got %d", len(availableOn))
	}
	if availableOn[0].FeatureFlagEnable != "consolidated_flag" {
		t.Errorf("Flag ON: Expected tool with FeatureFlagEnable, got FeatureFlagEnable=%q, FeatureFlagDisable=%q",
			availableOn[0].FeatureFlagEnable, availableOn[0].FeatureFlagDisable)
	}
}
