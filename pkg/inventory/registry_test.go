package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// mustBuild is a test helper that calls Build() and fails the test if an error occurs.
// Use this for tests where Build() is not expected to fail.
func mustBuild(t *testing.T, b *Builder) *Inventory {
	t.Helper()
	inv, err := b.Build()
	require.NoError(t, err)
	return inv
}

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
	reg := mustBuild(t, NewBuilder())
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

	reg := mustBuild(t, NewBuilder().SetTools(tools))

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

	reg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))
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
	reg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))
	allTools := reg.AvailableTools(context.Background())
	if len(allTools) != 2 {
		t.Fatalf("Expected 2 tools without read-only, got %d", len(allTools))
	}

	// Build with read-only - should filter out write tools
	readOnlyReg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithReadOnly(true))
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
	allReg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))
	allTools := allReg.AvailableTools(context.Background())
	if len(allTools) != 3 {
		t.Fatalf("Expected 3 tools without filter, got %d", len(allTools))
	}

	// Build with specific toolsets
	filteredReg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"toolset1", "toolset3"}))
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
	filteredReg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{" toolset1 ", "  toolset2  "}))
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
	filteredReg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"toolset1", "toolset1", " toolset1 "}))
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
	filteredReg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"", "toolset1", "  ", ""}))
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
			filtered := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets(tt.input))
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

func TestBuildErrorsOnUnrecognizedTools(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset2", true),
	}

	deprecatedAliases := map[string]string{
		"old_tool": "tool1",
	}

	tests := []struct {
		name          string
		withTools     []string
		expectError   bool
		errorContains string
	}{
		{
			name:        "all valid",
			withTools:   []string{"tool1", "tool2"},
			expectError: false,
		},
		{
			name:          "one invalid",
			withTools:     []string{"tool1", "blabla"},
			expectError:   true,
			errorContains: "blabla",
		},
		{
			name:          "multiple invalid",
			withTools:     []string{"invalid1", "tool1", "invalid2"},
			expectError:   true,
			errorContains: "invalid1",
		},
		{
			name:        "deprecated alias is valid",
			withTools:   []string{"old_tool"},
			expectError: false,
		},
		{
			name:        "mixed valid and deprecated alias",
			withTools:   []string{"old_tool", "tool2"},
			expectError: false,
		},
		{
			name:        "empty input",
			withTools:   []string{},
			expectError: false,
		},
		{
			name:        "whitespace trimmed from valid tool",
			withTools:   []string{" tool1 ", "  tool2  "},
			expectError: false,
		},
		{
			name:          "whitespace trimmed from invalid tool",
			withTools:     []string{" invalid_tool "},
			expectError:   true,
			errorContains: "invalid_tool",
		},
		{
			name:        "duplicate tools deduplicated",
			withTools:   []string{"tool1", "tool1"},
			expectError: false,
		},
		{
			name:          "duplicate invalid tools deduplicated",
			withTools:     []string{"blabla", "blabla"},
			expectError:   true,
			errorContains: "blabla",
		},
		{
			name:        "mixed whitespace and duplicates",
			withTools:   []string{" tool1 ", "tool1", "  tool1  "},
			expectError: false,
		},
		{
			name:        "empty strings ignored",
			withTools:   []string{"", "tool1", "  ", ""},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv, err := NewBuilder().
				SetTools(tools).
				WithDeprecatedAliases(deprecatedAliases).
				WithToolsets([]string{"all"}).
				WithTools(tt.withTools).
				Build()

			if tt.expectError {
				require.Error(t, err, "Expected error for unrecognized tools")
				require.Contains(t, err.Error(), tt.errorContains)
				require.Nil(t, inv)
			} else {
				require.NoError(t, err)
				require.NotNil(t, inv)
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
	filteredReg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{}).WithTools([]string{"tool1", "tool3"}))
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
	filtered := mustBuild(t, NewBuilder().SetTools(tools).WithReadOnly(true).WithToolsets([]string{"toolset1"}))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools))
	toolset1Tools := reg.ToolsForToolset("toolset1")

	if len(toolset1Tools) != 2 {
		t.Fatalf("Expected 2 tools for toolset1, got %d", len(toolset1Tools))
	}
}

func TestWithDeprecatedAliases(t *testing.T) {
	tools := []ServerTool{
		mockTool("new_name", "toolset1", true),
	}

	reg := mustBuild(t, NewBuilder().SetTools(tools).WithDeprecatedAliases(map[string]string{
		"old_name":  "new_name",
		"get_issue": "issue_read",
	}))

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

	reg := mustBuild(t, NewBuilder().SetTools(tools).
		WithDeprecatedAliases(map[string]string{
			"get_issue": "issue_read",
		}))

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

	reg := mustBuild(t, NewBuilder().SetTools(tools))

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
	filtered := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"toolset2"}).WithTools([]string{"issue_read"}))

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
	readOnlyFiltered := mustBuild(t, NewBuilder().SetTools(tools).WithReadOnly(true).WithTools([]string{"issue_write"}))
	available = readOnlyFiltered.AvailableTools(context.Background())

	// issue_write should be excluded because read-only applies to additional tools too
	for _, tool := range available {
		if tool.Tool.Name == "issue_write" {
			t.Error("expected issue_write to be excluded in read-only mode")
		}
	}

	// Test WithTools with non-existent tool (should error during Build)
	_, err := NewBuilder().SetTools(tools).WithToolsets([]string{}).WithTools([]string{"nonexistent"}).Build()
	require.Error(t, err, "expected error for non-existent tool")
	require.Contains(t, err.Error(), "nonexistent")
}

func TestWithToolsResolvesAliases(t *testing.T) {
	tools := []ServerTool{
		mockTool("issue_read", "toolset1", true),
	}

	// Using deprecated alias should resolve to canonical name
	filtered := mustBuild(t, NewBuilder().SetTools(tools).
		WithDeprecatedAliases(map[string]string{
			"get_issue": "issue_read",
		}).
		WithToolsets([]string{}).
		WithTools([]string{"get_issue"}))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))

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
	reg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))
	ids := reg.EnabledToolsetIDs()
	if len(ids) != 2 {
		t.Fatalf("Expected 2 enabled toolset IDs, got %d", len(ids))
	}

	// With filter
	filtered := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"toolset1"}))
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
	readOnlyReg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithReadOnly(true))

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

	reg := mustBuild(t, NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools).
		WithToolsets([]string{"all"}).
		WithDeprecatedAliases(map[string]string{
			"old_get_me": "get_me",
		}))

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
	reg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithReadOnly(true))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}))
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

	reg := mustBuild(t, NewBuilder().SetResources(resources).WithToolsets([]string{"all"}))
	// Pass a concrete URI - all resources remain registered, SDK handles matching
	filtered := reg.ForMCPRequest(MCPMethodResourcesRead, "repo://owner/repo")

	// All resources should be available - SDK handles URI template matching internally
	available := filtered.AvailableResourceTemplates(context.Background())
	if len(available) != 2 {
		t.Fatalf("Expected 2 resources for resources/read (SDK handles matching), got %d", len(available))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}))
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

	reg := mustBuild(t, NewBuilder().SetPrompts(prompts).WithToolsets([]string{"all"}))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}))
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

	original := mustBuild(t, NewBuilder().SetTools(tools).SetResources(resources).SetPrompts(prompts).WithToolsets([]string{"all"}))
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
	reg := mustBuild(t, NewBuilder().SetTools(tools).
		WithToolsets([]string{"default"}).
		WithReadOnly(true))
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

	reg := mustBuild(t, NewBuilder().SetTools(tools).SetResources(resources).WithToolsets([]string{"all"}))
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
	reg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))
	available := reg.AvailableTools(context.Background())
	if len(available) != 1 {
		t.Fatalf("Expected 1 tool without feature checker, got %d", len(available))
	}
	if available[0].Tool.Name != "always_available" {
		t.Errorf("Expected always_available, got %s", available[0].Tool.Name)
	}

	// With feature checker returning false, tool should still be excluded
	checkerFalse := func(_ context.Context, _ string) (bool, error) { return false, nil }
	regFalse := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checkerFalse))
	availableFalse := regFalse.AvailableTools(context.Background())
	if len(availableFalse) != 1 {
		t.Fatalf("Expected 1 tool with false checker, got %d", len(availableFalse))
	}

	// With feature checker returning true for "my_feature", tool should be included
	checkerTrue := func(_ context.Context, flag string) (bool, error) {
		return flag == "my_feature", nil
	}
	regTrue := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checkerTrue))
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
	reg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))
	available := reg.AvailableTools(context.Background())
	if len(available) != 2 {
		t.Fatalf("Expected 2 tools without feature checker, got %d", len(available))
	}

	// With feature checker returning true for "kill_switch", tool should be excluded
	checkerTrue := func(_ context.Context, flag string) (bool, error) {
		return flag == "kill_switch", nil
	}
	regFiltered := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checkerTrue))
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
	reg1 := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checker1))
	if len(reg1.AvailableTools(context.Background())) != 0 {
		t.Error("Tool should be excluded when enable flag is false")
	}

	// Enable flag set, disable flag not set -> included
	checker2 := func(_ context.Context, flag string) (bool, error) { return flag == "new_feature", nil }
	reg2 := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checker2))
	if len(reg2.AvailableTools(context.Background())) != 1 {
		t.Error("Tool should be included when enable flag is true and disable flag is false")
	}

	// Enable flag set, disable flag also set -> excluded (disable wins)
	checker3 := func(_ context.Context, _ string) (bool, error) { return true, nil }
	reg3 := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithFeatureChecker(checker3))
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
	reg := mustBuild(t, NewBuilder().SetTools(tools).WithFeatureChecker(checkerError))
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
	reg := mustBuild(t, NewBuilder().SetResources(resources).WithToolsets([]string{"all"}))
	available := reg.AvailableResourceTemplates(context.Background())
	if len(available) != 1 {
		t.Fatalf("Expected 1 resource without checker, got %d", len(available))
	}

	// With checker returning true, both should be included
	checker := func(_ context.Context, _ string) (bool, error) { return true, nil }
	regWithChecker := mustBuild(t, NewBuilder().SetResources(resources).WithToolsets([]string{"all"}).WithFeatureChecker(checker))
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
	reg := mustBuild(t, NewBuilder().SetPrompts(prompts).WithToolsets([]string{"all"}))
	available := reg.AvailablePrompts(context.Background())
	if len(available) != 1 {
		t.Fatalf("Expected 1 prompt without checker, got %d", len(available))
	}

	// With checker returning true, both should be included
	checker := func(_ context.Context, _ string) (bool, error) { return true, nil }
	regWithChecker := mustBuild(t, NewBuilder().SetPrompts(prompts).WithToolsets([]string{"all"}).WithFeatureChecker(checker))
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

			reg := mustBuild(t, NewBuilder().SetTools([]ServerTool{tool}).WithToolsets([]string{"all"}))
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

	reg := mustBuild(t, NewBuilder().SetTools([]ServerTool{tool}).WithToolsets([]string{"all"}))

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

	reg := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFilter(filter))

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

	reg := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFilter(filter1).
		WithFilter(filter2))

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

	reg := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFilter(filter))

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

	reg := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFilter(filter))

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
	reg1 := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}))
	available1 := reg1.AvailableTools(context.Background())
	if len(available1) != 0 {
		t.Error("Tool should be excluded when feature flag is not enabled")
	}

	// Feature flag enabled - tool should be included
	checker := func(_ context.Context, flag string) (bool, error) {
		return flag == "my_feature", nil
	}
	reg2 := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker))
	available2 := reg2.AvailableTools(context.Background())
	if len(available2) != 1 {
		t.Error("Tool should be included when both Enabled and feature flag pass")
	}

	// Enabled returns false - tool should be excluded despite feature flag
	tool.Enabled = func(_ context.Context) (bool, error) {
		return false, nil
	}
	reg3 := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker))
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

	reg := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFilter(filter))

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
	reg := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker).
		WithFilter(filter))

	available := reg.AvailableTools(context.Background())
	if len(available) != 1 {
		t.Error("Tool should be included when all filters pass")
	}

	// Change filter to return false - tool should be excluded
	filterFalse := func(_ context.Context, _ *ServerTool) (bool, error) {
		return false, nil
	}

	reg2 := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker).
		WithFilter(filterFalse))

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

	reg := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFilter(filter))

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

	reg := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"toolset1"}).
		WithReadOnly(true))

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

	reg := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithReadOnly(true). // This will exclude the tool (it's not read-only)
		WithFeatureChecker(checker).
		WithFilter(filter))

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
	regFlagOff := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}))
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
	regFlagOn := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker))
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

// TestWithTools_DeprecatedAliasAndFeatureFlag tests that deprecated aliases work correctly
// when the old tool is controlled by a feature flag. This covers the scenario where:
// - Old tool "old_tool" has FeatureFlagDisable="my_flag" (available when flag is OFF)
// - New tool "new_tool" has FeatureFlagEnable="my_flag" (available when flag is ON)
// - Deprecated alias maps "old_tool" -> "new_tool"
// - User specifies --tools=old_tool
// Expected behavior:
// - Flag OFF: old_tool should be available (not the new_tool via alias)
// - Flag ON: new_tool should be available (via alias resolution)
func TestWithTools_DeprecatedAliasAndFeatureFlag(t *testing.T) {
	oldTool := mockToolWithFlags("old_tool", "actions", true, "", "my_flag")
	newTool := mockToolWithFlags("new_tool", "actions", true, "my_flag", "")
	tools := []ServerTool{oldTool, newTool}

	deprecatedAliases := map[string]string{
		"old_tool": "new_tool",
	}

	// Test 1: Flag OFF - old_tool should be available via direct name match
	// (not via alias resolution to new_tool, since old_tool still exists)
	regFlagOff := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithDeprecatedAliases(deprecatedAliases).
		WithToolsets([]string{}).        // No toolsets enabled
		WithTools([]string{"old_tool"})) // Explicitly request old tool
	availableOff := regFlagOff.AvailableTools(context.Background())
	if len(availableOff) != 1 {
		t.Fatalf("Flag OFF: Expected 1 tool, got %d", len(availableOff))
	}
	if availableOff[0].Tool.Name != "old_tool" {
		t.Errorf("Flag OFF: Expected old_tool, got %s", availableOff[0].Tool.Name)
	}

	// Test 2: Flag ON - new_tool should be available via alias resolution
	checker := func(_ context.Context, flag string) (bool, error) {
		return flag == "my_flag", nil
	}
	regFlagOn := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithDeprecatedAliases(deprecatedAliases).
		WithToolsets([]string{}).        // No toolsets enabled
		WithTools([]string{"old_tool"}). // Request old tool name
		WithFeatureChecker(checker))
	availableOn := regFlagOn.AvailableTools(context.Background())
	if len(availableOn) != 1 {
		t.Fatalf("Flag ON: Expected 1 tool, got %d", len(availableOn))
	}
	if availableOn[0].Tool.Name != "new_tool" {
		t.Errorf("Flag ON: Expected new_tool (via alias), got %s", availableOn[0].Tool.Name)
	}
}

// mockToolWithMeta creates a ServerTool with Meta for testing insiders mode
func mockToolWithMeta(name string, toolsetID string, meta map[string]any) ServerTool {
	return NewServerToolFromHandler(
		mcp.Tool{
			Name: name,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint: true,
			},
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
			Meta:        meta,
		},
		testToolsetMetadata(toolsetID),
		func(_ any) mcp.ToolHandler {
			return func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return nil, nil
			}
		},
	)
}

func TestWithInsidersMode_DisabledStripsUIMetadata(t *testing.T) {
	toolWithUI := mockToolWithMeta("tool_with_ui", "toolset1", map[string]any{
		"ui":          map[string]any{"html": "<div>hello</div>"},
		"description": "kept",
	})

	// Default: insiders mode is disabled - UI meta should be stripped
	reg := mustBuild(t, NewBuilder().SetTools([]ServerTool{toolWithUI}).WithToolsets([]string{"all"}))
	available := reg.AvailableTools(context.Background())

	require.Len(t, available, 1)
	// UI metadata should be stripped
	if available[0].Tool.Meta["ui"] != nil {
		t.Errorf("Expected 'ui' meta to be stripped, but it was present")
	}
	// Other metadata should be preserved
	if available[0].Tool.Meta["description"] != "kept" {
		t.Errorf("Expected 'description' meta to be preserved, got %v", available[0].Tool.Meta["description"])
	}
}

func TestWithInsidersMode_EnabledPreservesUIMetadata(t *testing.T) {
	uiData := map[string]any{"html": "<div>hello</div>"}
	toolWithUI := mockToolWithMeta("tool_with_ui", "toolset1", map[string]any{
		"ui":          uiData,
		"description": "kept",
	})

	// Insiders mode enabled - UI meta should be preserved
	reg := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{toolWithUI}).
		WithToolsets([]string{"all"}).
		WithInsidersMode(true))
	available := reg.AvailableTools(context.Background())

	require.Len(t, available, 1)
	// UI metadata should be preserved
	if available[0].Tool.Meta["ui"] == nil {
		t.Errorf("Expected 'ui' meta to be preserved in insiders mode")
	}
	// Other metadata should also be preserved
	if available[0].Tool.Meta["description"] != "kept" {
		t.Errorf("Expected 'description' meta to be preserved, got %v", available[0].Tool.Meta["description"])
	}
}

func TestWithInsidersMode_EnabledPreservesInsidersOnlyTools(t *testing.T) {
	normalTool := mockTool("normal", "toolset1", true)
	insidersTool := mockTool("insiders_only", "toolset1", true)
	insidersTool.InsidersOnly = true

	// With insiders mode enabled, both tools should be available
	reg := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{normalTool, insidersTool}).
		WithToolsets([]string{"all"}).
		WithInsidersMode(true))
	available := reg.AvailableTools(context.Background())

	require.Len(t, available, 2)
	names := []string{available[0].Tool.Name, available[1].Tool.Name}
	require.Contains(t, names, "normal")
	require.Contains(t, names, "insiders_only")
}

func TestWithInsidersMode_DisabledRemovesInsidersOnlyTools(t *testing.T) {
	normalTool := mockTool("normal", "toolset1", true)
	insidersTool := mockTool("insiders_only", "toolset1", true)
	insidersTool.InsidersOnly = true

	// With insiders mode disabled, insiders-only tool should be removed
	reg := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{normalTool, insidersTool}).
		WithToolsets([]string{"all"}).
		WithInsidersMode(false))
	available := reg.AvailableTools(context.Background())

	require.Len(t, available, 1)
	require.Equal(t, "normal", available[0].Tool.Name)
}

func TestWithInsidersMode_ToolsWithoutUIMetaUnaffected(t *testing.T) {
	toolNoUI := mockToolWithMeta("tool_no_ui", "toolset1", map[string]any{
		"description": "kept",
		"version":     "1.0",
	})
	toolNilMeta := mockTool("tool_nil_meta", "toolset1", true)

	// Test with insiders disabled
	reg := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{toolNoUI, toolNilMeta}).
		WithToolsets([]string{"all"}))
	available := reg.AvailableTools(context.Background())

	require.Len(t, available, 2)

	// Find toolNoUI
	var foundNoUI, foundNilMeta *ServerTool
	for i := range available {
		switch available[i].Tool.Name {
		case "tool_no_ui":
			foundNoUI = &available[i]
		case "tool_nil_meta":
			foundNilMeta = &available[i]
		}
	}

	require.NotNil(t, foundNoUI)
	require.NotNil(t, foundNilMeta)

	// toolNoUI should have its metadata preserved
	if foundNoUI.Tool.Meta["description"] != "kept" || foundNoUI.Tool.Meta["version"] != "1.0" {
		t.Errorf("Expected toolNoUI meta to be unchanged, got %v", foundNoUI.Tool.Meta)
	}

	// toolNilMeta should still have nil meta
	if foundNilMeta.Tool.Meta != nil {
		t.Errorf("Expected toolNilMeta to have nil meta, got %v", foundNilMeta.Tool.Meta)
	}
}

func TestWithInsidersMode_UIOnlyMetaBecomesNil(t *testing.T) {
	// Tool with ONLY ui metadata - should become nil after stripping
	toolUIOnly := mockToolWithMeta("tool_ui_only", "toolset1", map[string]any{
		"ui": map[string]any{"html": "<div>hello</div>"},
	})

	reg := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{toolUIOnly}).
		WithToolsets([]string{"all"}))
	available := reg.AvailableTools(context.Background())

	require.Len(t, available, 1)
	// Meta should be nil since ui was the only key
	if available[0].Tool.Meta != nil {
		t.Errorf("Expected Meta to be nil after stripping only key, got %v", available[0].Tool.Meta)
	}
}

func TestStripInsidersMetaFromTool(t *testing.T) {
	tests := []struct {
		name         string
		meta         map[string]any
		expectChange bool
		expectedMeta map[string]any // nil means Meta should be nil
	}{
		{
			name:         "nil meta - no change",
			meta:         nil,
			expectChange: false,
		},
		{
			name:         "no insiders keys - no change",
			meta:         map[string]any{"description": "test", "version": "1.0"},
			expectChange: false,
		},
		{
			name:         "ui key only - becomes nil",
			meta:         map[string]any{"ui": "data"},
			expectChange: true,
			expectedMeta: nil,
		},
		{
			name:         "ui key with other keys - ui stripped",
			meta:         map[string]any{"ui": "data", "description": "kept"},
			expectChange: true,
			expectedMeta: map[string]any{"description": "kept"},
		},
		{
			name:         "ui is nil value - no change (nil value means key not present)",
			meta:         map[string]any{"ui": nil, "description": "kept"},
			expectChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := mockToolWithMeta("test", "toolset1", tt.meta)
			result := stripInsidersMetaFromTool(tool)

			if tt.expectChange {
				require.NotNil(t, result, "expected change but got nil")
				if tt.expectedMeta == nil {
					require.Nil(t, result.Tool.Meta, "expected Meta to be nil")
				} else {
					// Compare values by key since types may differ (map[string]any vs mcp.Meta)
					for k, v := range tt.expectedMeta {
						require.Equal(t, v, result.Tool.Meta[k], "key %s should match", k)
					}
					require.Len(t, result.Tool.Meta, len(tt.expectedMeta))
				}
			} else {
				require.Nil(t, result, "expected no change but got result")
			}
		})
	}
}

func TestStripInsidersFeatures(t *testing.T) {
	tools := []ServerTool{
		mockToolWithMeta("tool1", "toolset1", map[string]any{"ui": "data"}),
		mockToolWithMeta("tool2", "toolset1", map[string]any{"description": "kept"}),
		mockTool("tool3", "toolset1", true), // nil meta
	}

	result := stripInsidersFeatures(tools)

	require.Len(t, result, 3)

	// tool1: ui should be stripped, meta becomes nil
	require.Nil(t, result[0].Tool.Meta, "tool1 meta should be nil after stripping ui")

	// tool2: unchanged (compare by key since types differ)
	require.Equal(t, "kept", result[1].Tool.Meta["description"])
	require.Len(t, result[1].Tool.Meta, 1)

	// tool3: unchanged (nil)
	require.Nil(t, result[2].Tool.Meta)
}

func TestStripInsidersFeatures_RemovesInsidersOnlyTools(t *testing.T) {
	// Create tools: one normal, one insiders-only, one normal
	normalTool1 := mockTool("normal1", "toolset1", true)
	insidersTool := mockTool("insiders_only", "toolset1", true)
	insidersTool.InsidersOnly = true
	normalTool2 := mockTool("normal2", "toolset1", true)

	tools := []ServerTool{normalTool1, insidersTool, normalTool2}

	result := stripInsidersFeatures(tools)

	// Should only have 2 tools (insiders-only tool filtered out)
	require.Len(t, result, 2)
	require.Equal(t, "normal1", result[0].Tool.Name)
	require.Equal(t, "normal2", result[1].Tool.Name)
}

func TestInsidersOnlyMetaKeys_FutureAdditions(t *testing.T) {
	// This test verifies the mechanism works for multiple keys
	// If we add new experimental keys to insidersOnlyMetaKeys, they should be stripped

	// Save original and restore after test
	originalKeys := insidersOnlyMetaKeys
	defer func() { insidersOnlyMetaKeys = originalKeys }()

	// Add a hypothetical future experimental key
	insidersOnlyMetaKeys = []string{"ui", "experimental_feature", "beta"}

	tool := mockToolWithMeta("test", "toolset1", map[string]any{
		"ui":                   "ui data",
		"experimental_feature": "exp data",
		"beta":                 "beta data",
		"description":          "kept",
	})

	result := stripInsidersMetaFromTool(tool)

	require.NotNil(t, result)
	require.NotNil(t, result.Tool.Meta)
	require.Nil(t, result.Tool.Meta["ui"], "ui should be stripped")
	require.Nil(t, result.Tool.Meta["experimental_feature"], "experimental_feature should be stripped")
	require.Nil(t, result.Tool.Meta["beta"], "beta should be stripped")
	require.Equal(t, "kept", result.Tool.Meta["description"], "description should be preserved")
}

func TestWithInsidersMode_DoesNotMutateOriginalTools(t *testing.T) {
	originalMeta := map[string]any{"ui": "data", "description": "kept"}
	tool := mockToolWithMeta("test", "toolset1", originalMeta)
	tools := []ServerTool{tool}

	// Build with insiders disabled - should strip ui
	_ = mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))

	// Original tool should be unchanged
	require.Equal(t, "data", tools[0].Tool.Meta["ui"], "original tool should not be mutated")
	require.Equal(t, "kept", tools[0].Tool.Meta["description"], "original tool should not be mutated")
}

func TestWithDisallowedTools(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset1", true),
		mockTool("tool3", "toolset2", true),
	}

	tests := []struct {
		name            string
		disallowed      []string
		toolsets        []string
		expectedNames   []string
		unexpectedNames []string
	}{
		{
			name:            "single tool disallowed",
			disallowed:      []string{"tool2"},
			toolsets:        []string{"all"},
			expectedNames:   []string{"tool1", "tool3"},
			unexpectedNames: []string{"tool2"},
		},
		{
			name:            "multiple tools disallowed",
			disallowed:      []string{"tool1", "tool3"},
			toolsets:        []string{"all"},
			expectedNames:   []string{"tool2"},
			unexpectedNames: []string{"tool1", "tool3"},
		},
		{
			name:            "empty disallowed list is a no-op",
			disallowed:      []string{},
			toolsets:        []string{"all"},
			expectedNames:   []string{"tool1", "tool2", "tool3"},
			unexpectedNames: nil,
		},
		{
			name:            "nil disallowed list is a no-op",
			disallowed:      nil,
			toolsets:        []string{"all"},
			expectedNames:   []string{"tool1", "tool2", "tool3"},
			unexpectedNames: nil,
		},
		{
			name:            "disallowing non-existent tool is a no-op",
			disallowed:      []string{"nonexistent"},
			toolsets:        []string{"all"},
			expectedNames:   []string{"tool1", "tool2", "tool3"},
			unexpectedNames: nil,
		},
		{
			name:            "disallow all tools",
			disallowed:      []string{"tool1", "tool2", "tool3"},
			toolsets:        []string{"all"},
			expectedNames:   nil,
			unexpectedNames: []string{"tool1", "tool2", "tool3"},
		},
		{
			name:            "whitespace is trimmed",
			disallowed:      []string{" tool2 ", "  tool3  "},
			toolsets:        []string{"all"},
			expectedNames:   []string{"tool1"},
			unexpectedNames: []string{"tool2", "tool3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := mustBuild(t, NewBuilder().
				SetTools(tools).
				WithToolsets(tt.toolsets).
				WithDisallowedTools(tt.disallowed))

			available := reg.AvailableTools(context.Background())
			names := make(map[string]bool)
			for _, tool := range available {
				names[tool.Tool.Name] = true
			}

			for _, expected := range tt.expectedNames {
				require.True(t, names[expected], "tool %q should be available", expected)
			}
			for _, unexpected := range tt.unexpectedNames {
				require.False(t, names[unexpected], "tool %q should be disallowed", unexpected)
			}
		})
	}
}

func TestWithDisallowedTools_OverridesAdditionalTools(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", true),
		mockTool("tool2", "toolset1", true),
		mockTool("tool3", "toolset2", true),
	}

	// tool3 is explicitly enabled via WithTools, but also disallowed
	// disallowed should win because builder filters run before additional tools check
	reg := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"toolset1"}).
		WithTools([]string{"tool3"}).
		WithDisallowedTools([]string{"tool3"}))

	available := reg.AvailableTools(context.Background())
	names := make(map[string]bool)
	for _, tool := range available {
		names[tool.Tool.Name] = true
	}

	require.True(t, names["tool1"], "tool1 should be available")
	require.True(t, names["tool2"], "tool2 should be available")
	require.False(t, names["tool3"], "tool3 should be disallowed even though explicitly added via WithTools")
}

func TestWithDisallowedTools_CombinesWithReadOnly(t *testing.T) {
	tools := []ServerTool{
		mockTool("read_tool", "toolset1", true),
		mockTool("write_tool", "toolset1", false),
		mockTool("another_read", "toolset1", true),
	}

	// read-only excludes write_tool, disallowed excludes read_tool
	reg := mustBuild(t, NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		WithReadOnly(true).
		WithDisallowedTools([]string{"read_tool"}))

	available := reg.AvailableTools(context.Background())
	require.Len(t, available, 1)
	require.Equal(t, "another_read", available[0].Tool.Name)
}

func TestCreateDisallowedToolsFilter(t *testing.T) {
	filter := CreateDisallowedToolsFilter([]string{"blocked_tool"})

	blockedTool := mockTool("blocked_tool", "toolset1", true)
	allowedTool := mockTool("allowed_tool", "toolset1", true)

	allowed, err := filter(context.Background(), &blockedTool)
	require.NoError(t, err)
	require.False(t, allowed, "blocked_tool should be excluded")

	allowed, err = filter(context.Background(), &allowedTool)
	require.NoError(t, err)
	require.True(t, allowed, "allowed_tool should be included")
}
