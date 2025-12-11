package toolsets

import (
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mockTool creates a minimal ServerTool for testing
func mockTool(name string, readOnly bool) ServerTool {
	return ServerTool{
		Tool: mcp.Tool{
			Name: name,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint: readOnly,
			},
		},
		RegisterFunc: func(_ *mcp.Server) {},
	}
}

func TestNewToolsetGroupIsEmptyWithoutEverythingOn(t *testing.T) {
	tsg := NewToolsetGroup(false)
	if len(tsg.Toolsets) != 0 {
		t.Fatalf("Expected Toolsets map to be empty, got %d items", len(tsg.Toolsets))
	}
	if tsg.everythingOn {
		t.Fatal("Expected everythingOn to be initialized as false")
	}
}

func TestAddToolset(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Test adding a toolset
	toolset := NewToolset("test-toolset", "A test toolset")
	toolset.Enabled = true
	tsg.AddToolset(toolset)

	// Verify toolset was added correctly
	if len(tsg.Toolsets) != 1 {
		t.Errorf("Expected 1 toolset, got %d", len(tsg.Toolsets))
	}

	toolset, exists := tsg.Toolsets["test-toolset"]
	if !exists {
		t.Fatal("Feature was not added to the map")
	}

	if toolset.Name != "test-toolset" {
		t.Errorf("Expected toolset name to be 'test-toolset', got '%s'", toolset.Name)
	}

	if toolset.Description != "A test toolset" {
		t.Errorf("Expected toolset description to be 'A test toolset', got '%s'", toolset.Description)
	}

	if !toolset.Enabled {
		t.Error("Expected toolset to be enabled")
	}

	// Test adding another toolset
	anotherToolset := NewToolset("another-toolset", "Another test toolset")
	tsg.AddToolset(anotherToolset)

	if len(tsg.Toolsets) != 2 {
		t.Errorf("Expected 2 toolsets, got %d", len(tsg.Toolsets))
	}

	// Test overriding existing toolset
	updatedToolset := NewToolset("test-toolset", "Updated description")
	tsg.AddToolset(updatedToolset)

	toolset = tsg.Toolsets["test-toolset"]
	if toolset.Description != "Updated description" {
		t.Errorf("Expected toolset description to be updated to 'Updated description', got '%s'", toolset.Description)
	}

	if toolset.Enabled {
		t.Error("Expected toolset to be disabled after update")
	}
}

func TestIsEnabled(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Test with non-existent toolset
	if tsg.IsEnabled("non-existent") {
		t.Error("Expected IsEnabled to return false for non-existent toolset")
	}

	// Test with disabled toolset
	disabledToolset := NewToolset("disabled-toolset", "A disabled toolset")
	tsg.AddToolset(disabledToolset)
	if tsg.IsEnabled("disabled-toolset") {
		t.Error("Expected IsEnabled to return false for disabled toolset")
	}

	// Test with enabled toolset
	enabledToolset := NewToolset("enabled-toolset", "An enabled toolset")
	enabledToolset.Enabled = true
	tsg.AddToolset(enabledToolset)
	if !tsg.IsEnabled("enabled-toolset") {
		t.Error("Expected IsEnabled to return true for enabled toolset")
	}
}

func TestEnableFeature(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Test enabling non-existent toolset
	err := tsg.EnableToolset("non-existent")
	if err == nil {
		t.Error("Expected error when enabling non-existent toolset")
	}

	// Test enabling toolset
	testToolset := NewToolset("test-toolset", "A test toolset")
	tsg.AddToolset(testToolset)

	if tsg.IsEnabled("test-toolset") {
		t.Error("Expected toolset to be disabled initially")
	}

	err = tsg.EnableToolset("test-toolset")
	if err != nil {
		t.Errorf("Expected no error when enabling toolset, got: %v", err)
	}

	if !tsg.IsEnabled("test-toolset") {
		t.Error("Expected toolset to be enabled after EnableFeature call")
	}

	// Test enabling already enabled toolset
	err = tsg.EnableToolset("test-toolset")
	if err != nil {
		t.Errorf("Expected no error when enabling already enabled toolset, got: %v", err)
	}
}

func TestEnableToolsets(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Prepare toolsets
	toolset1 := NewToolset("toolset1", "Feature 1")
	toolset2 := NewToolset("toolset2", "Feature 2")
	tsg.AddToolset(toolset1)
	tsg.AddToolset(toolset2)

	// Test enabling multiple toolsets
	err := tsg.EnableToolsets([]string{"toolset1", "toolset2"}, &EnableToolsetsOptions{})
	if err != nil {
		t.Errorf("Expected no error when enabling toolsets, got: %v", err)
	}

	if !tsg.IsEnabled("toolset1") {
		t.Error("Expected toolset1 to be enabled")
	}

	if !tsg.IsEnabled("toolset2") {
		t.Error("Expected toolset2 to be enabled")
	}

	// Test with non-existent toolset in the list
	err = tsg.EnableToolsets([]string{"toolset1", "non-existent"}, nil)
	if err != nil {
		t.Errorf("Expected no error when ignoring unknown toolsets, got: %v", err)
	}

	err = tsg.EnableToolsets([]string{"toolset1", "non-existent"}, &EnableToolsetsOptions{
		ErrorOnUnknown: false,
	})
	if err != nil {
		t.Errorf("Expected no error when ignoring unknown toolsets, got: %v", err)
	}

	err = tsg.EnableToolsets([]string{"toolset1", "non-existent"}, &EnableToolsetsOptions{ErrorOnUnknown: true})
	if err == nil {
		t.Error("Expected error when enabling list with non-existent toolset")
	}
	if !errors.Is(err, NewToolsetDoesNotExistError("non-existent")) {
		t.Errorf("Expected ToolsetDoesNotExistError when enabling non-existent toolset, got: %v", err)
	}

	// Test with empty list
	err = tsg.EnableToolsets([]string{}, &EnableToolsetsOptions{})
	if err != nil {
		t.Errorf("Expected no error with empty toolset list, got: %v", err)
	}

	// Test enabling everything through EnableToolsets
	tsg = NewToolsetGroup(false)
	err = tsg.EnableToolsets([]string{"all"}, &EnableToolsetsOptions{})
	if err != nil {
		t.Errorf("Expected no error when enabling 'all', got: %v", err)
	}

	if !tsg.everythingOn {
		t.Error("Expected everythingOn to be true after enabling 'all' via EnableToolsets")
	}
}

func TestEnableEverything(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Add a disabled toolset
	testToolset := NewToolset("test-toolset", "A test toolset")
	tsg.AddToolset(testToolset)

	// Verify it's disabled
	if tsg.IsEnabled("test-toolset") {
		t.Error("Expected toolset to be disabled initially")
	}

	// Enable "all"
	err := tsg.EnableToolsets([]string{"all"}, &EnableToolsetsOptions{})
	if err != nil {
		t.Errorf("Expected no error when enabling 'all', got: %v", err)
	}

	// Verify everythingOn was set
	if !tsg.everythingOn {
		t.Error("Expected everythingOn to be true after enabling 'all'")
	}

	// Verify the previously disabled toolset is now enabled
	if !tsg.IsEnabled("test-toolset") {
		t.Error("Expected toolset to be enabled when everythingOn is true")
	}

	// Verify a non-existent toolset is also enabled
	if !tsg.IsEnabled("non-existent") {
		t.Error("Expected non-existent toolset to be enabled when everythingOn is true")
	}
}

func TestIsEnabledWithEverythingOn(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Enable "all"
	err := tsg.EnableToolsets([]string{"all"}, &EnableToolsetsOptions{})
	if err != nil {
		t.Errorf("Expected no error when enabling 'all', got: %v", err)
	}

	// Test that any toolset name returns true with IsEnabled
	if !tsg.IsEnabled("some-toolset") {
		t.Error("Expected IsEnabled to return true for any toolset when everythingOn is true")
	}

	if !tsg.IsEnabled("another-toolset") {
		t.Error("Expected IsEnabled to return true for any toolset when everythingOn is true")
	}
}

func TestToolsetGroup_GetToolset(t *testing.T) {
	tsg := NewToolsetGroup(false)
	toolset := NewToolset("my-toolset", "desc")
	tsg.AddToolset(toolset)

	// Should find the toolset
	got, err := tsg.GetToolset("my-toolset")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != toolset {
		t.Errorf("expected to get the same toolset instance")
	}

	// Should not find a non-existent toolset
	_, err = tsg.GetToolset("does-not-exist")
	if err == nil {
		t.Error("expected error for missing toolset, got nil")
	}
	if !errors.Is(err, NewToolsetDoesNotExistError("does-not-exist")) {
		t.Errorf("expected error to be ToolsetDoesNotExistError, got %v", err)
	}
}

func TestAddDeprecatedToolAliases(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Test adding aliases
	tsg.AddDeprecatedToolAliases(map[string]string{
		"old_name":  "new_name",
		"get_issue": "issue_read",
		"create_pr": "pull_request_create",
	})

	if len(tsg.deprecatedAliases) != 3 {
		t.Errorf("expected 3 aliases, got %d", len(tsg.deprecatedAliases))
	}
	if tsg.deprecatedAliases["old_name"] != "new_name" {
		t.Errorf("expected alias 'old_name' -> 'new_name', got '%s'", tsg.deprecatedAliases["old_name"])
	}
	if tsg.deprecatedAliases["get_issue"] != "issue_read" {
		t.Errorf("expected alias 'get_issue' -> 'issue_read'")
	}
	if tsg.deprecatedAliases["create_pr"] != "pull_request_create" {
		t.Errorf("expected alias 'create_pr' -> 'pull_request_create'")
	}
}

func TestIsDeprecatedToolAlias(t *testing.T) {
	tsg := NewToolsetGroup(false)
	tsg.AddDeprecatedToolAliases(map[string]string{"old_tool": "new_tool"})

	// Test with a deprecated alias
	canonical, isAlias := tsg.IsDeprecatedToolAlias("old_tool")
	if !isAlias {
		t.Error("expected 'old_tool' to be recognized as an alias")
	}
	if canonical != "new_tool" {
		t.Errorf("expected canonical name 'new_tool', got '%s'", canonical)
	}

	// Test with a non-alias
	canonical, isAlias = tsg.IsDeprecatedToolAlias("some_other_tool")
	if isAlias {
		t.Error("expected 'some_other_tool' to not be an alias")
	}
	if canonical != "" {
		t.Errorf("expected empty canonical name, got '%s'", canonical)
	}
}

func TestFindToolByName_WithAlias(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Create a toolset with a tool
	toolset := NewToolset("test-toolset", "Test toolset")
	toolset.readTools = append(toolset.readTools, mockTool("issue_read", true))
	tsg.AddToolset(toolset)

	// Add an alias
	tsg.AddDeprecatedToolAliases(map[string]string{"get_issue": "issue_read"})

	// Find by canonical name
	tool, toolsetName, err := tsg.FindToolByName("issue_read")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tool.Tool.Name != "issue_read" {
		t.Errorf("expected tool name 'issue_read', got '%s'", tool.Tool.Name)
	}
	if toolsetName != "test-toolset" {
		t.Errorf("expected toolset name 'test-toolset', got '%s'", toolsetName)
	}

	// Find by deprecated alias (should resolve to canonical)
	tool, toolsetName, err = tsg.FindToolByName("get_issue")
	if err != nil {
		t.Fatalf("expected no error when using alias, got %v", err)
	}
	if tool.Tool.Name != "issue_read" {
		t.Errorf("expected tool name 'issue_read' when using alias, got '%s'", tool.Tool.Name)
	}
	if toolsetName != "test-toolset" {
		t.Errorf("expected toolset name 'test-toolset', got '%s'", toolsetName)
	}
}

func TestFindToolByName_NotFound(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Create a toolset with a tool
	toolset := NewToolset("test-toolset", "Test toolset")
	toolset.readTools = append(toolset.readTools, mockTool("some_tool", true))
	tsg.AddToolset(toolset)

	// Try to find a non-existent tool
	_, _, err := tsg.FindToolByName("nonexistent_tool")
	if err == nil {
		t.Error("expected error for non-existent tool")
	}

	var toolErr *ToolDoesNotExistError
	if !errors.As(err, &toolErr) {
		t.Errorf("expected ToolDoesNotExistError, got %T", err)
	}
}

func TestRegisterSpecificTools_WithAliases(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Create a toolset with both read and write tools
	toolset := NewToolset("test-toolset", "Test toolset")
	toolset.readTools = append(toolset.readTools, mockTool("issue_read", true))
	toolset.writeTools = append(toolset.writeTools, mockTool("issue_write", false))
	tsg.AddToolset(toolset)

	// Add aliases
	tsg.AddDeprecatedToolAliases(map[string]string{
		"get_issue":    "issue_read",
		"create_issue": "issue_write",
	})

	// Test registering with aliases (should work)
	err := tsg.RegisterSpecificTools(nil, []string{"get_issue"}, false)
	if err != nil {
		t.Errorf("expected no error registering aliased tool, got %v", err)
	}

	// Test registering write tool alias in read-only mode (should skip but not error)
	err = tsg.RegisterSpecificTools(nil, []string{"create_issue"}, true)
	if err != nil {
		t.Errorf("expected no error when skipping write tool in read-only mode, got %v", err)
	}

	// Test registering non-existent tool (should error)
	err = tsg.RegisterSpecificTools(nil, []string{"nonexistent"}, false)
	if err == nil {
		t.Error("expected error for non-existent tool")
	}
}
