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

func TestResolveToolAliases(t *testing.T) {
	tsg := NewToolsetGroup(false)
	tsg.AddDeprecatedToolAliases(map[string]string{
		"get_issue": "issue_read",
		"create_pr": "pull_request_create",
	})

	// Test resolving a mix of aliases and canonical names
	input := []string{"get_issue", "some_tool", "create_pr"}
	resolved, aliasesUsed := tsg.ResolveToolAliases(input)

	// Verify resolved names
	if len(resolved) != 3 {
		t.Fatalf("expected 3 resolved names, got %d", len(resolved))
	}
	if resolved[0] != "issue_read" {
		t.Errorf("expected 'issue_read', got '%s'", resolved[0])
	}
	if resolved[1] != "some_tool" {
		t.Errorf("expected 'some_tool' (unchanged), got '%s'", resolved[1])
	}
	if resolved[2] != "pull_request_create" {
		t.Errorf("expected 'pull_request_create', got '%s'", resolved[2])
	}

	// Verify aliasesUsed map
	if len(aliasesUsed) != 2 {
		t.Fatalf("expected 2 aliases used, got %d", len(aliasesUsed))
	}
	if aliasesUsed["get_issue"] != "issue_read" {
		t.Errorf("expected aliasesUsed['get_issue'] = 'issue_read', got '%s'", aliasesUsed["get_issue"])
	}
	if aliasesUsed["create_pr"] != "pull_request_create" {
		t.Errorf("expected aliasesUsed['create_pr'] = 'pull_request_create', got '%s'", aliasesUsed["create_pr"])
	}
}

func TestFindToolByName(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Create a toolset with a tool
	toolset := NewToolset("test-toolset", "Test toolset")
	toolset.readTools = append(toolset.readTools, mockTool("issue_read", true))
	tsg.AddToolset(toolset)

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

	// FindToolByName does NOT resolve aliases - it expects canonical names
	_, _, err = tsg.FindToolByName("get_issue")
	if err == nil {
		t.Error("expected error when using alias directly with FindToolByName")
	}
}

func TestRegisterSpecificTools(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Create a toolset with both read and write tools
	toolset := NewToolset("test-toolset", "Test toolset")
	toolset.readTools = append(toolset.readTools, mockTool("issue_read", true))
	toolset.writeTools = append(toolset.writeTools, mockTool("issue_write", false))
	tsg.AddToolset(toolset)

	// Test registering with canonical names
	err := tsg.RegisterSpecificTools(nil, []string{"issue_read"}, false)
	if err != nil {
		t.Errorf("expected no error registering tool, got %v", err)
	}

	// Test registering write tool in read-only mode (should skip but not error)
	err = tsg.RegisterSpecificTools(nil, []string{"issue_write"}, true)
	if err != nil {
		t.Errorf("expected no error when skipping write tool in read-only mode, got %v", err)
	}

	// Test registering non-existent tool (should error)
	err = tsg.RegisterSpecificTools(nil, []string{"nonexistent"}, false)
	if err == nil {
		t.Error("expected error for non-existent tool")
	}
}

// mockToolWithMeta creates a ServerTool with metadata for testing NewToolsetGroupFromTools
func mockToolWithMeta(name string, toolsetName string, readOnly bool) ServerTool {
	return ServerTool{
		Tool: mcp.Tool{
			Name: name,
			Meta: mcp.Meta{"toolset": toolsetName},
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint: readOnly,
			},
		},
		RegisterFunc: func(_ *mcp.Server) {},
	}
}

// mockToolWithScopes creates a ServerTool with metadata including required scopes
func mockToolWithScopes(name string, toolsetName string, readOnly bool, requiredScopes []string) ServerTool {
	meta := mcp.Meta{"toolset": toolsetName}
	if requiredScopes != nil {
		meta["requiredOAuthScopes"] = requiredScopes
	}
	return ServerTool{
		Tool: mcp.Tool{
			Name: name,
			Meta: meta,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint: readOnly,
			},
		},
		RegisterFunc: func(_ *mcp.Server) {},
	}
}

func TestNewToolsetGroupFromTools(t *testing.T) {
	toolsetMetadatas := []ToolsetMetadata{
		{ID: "repos", Description: "Repository tools"},
		{ID: "issues", Description: "Issue tools"},
	}

	// Create tools with meta containing toolset info
	tools := []ServerTool{
		mockToolWithMeta("get_repo", "repos", true),
		mockToolWithMeta("create_repo", "repos", false),
		mockToolWithMeta("get_issue", "issues", true),
		mockToolWithMeta("create_issue", "issues", false),
		mockToolWithMeta("list_issues", "issues", true),
	}

	tsg := NewToolsetGroupFromTools(false, toolsetMetadatas, tools...)

	// Verify toolsets were created
	if len(tsg.Toolsets) != 2 {
		t.Fatalf("expected 2 toolsets, got %d", len(tsg.Toolsets))
	}

	// Verify repos toolset
	reposToolset, exists := tsg.Toolsets["repos"]
	if !exists {
		t.Fatal("expected 'repos' toolset to exist")
	}
	if reposToolset.Description != "Repository tools" {
		t.Errorf("expected repos description 'Repository tools', got '%s'", reposToolset.Description)
	}
	if len(reposToolset.readTools) != 1 {
		t.Errorf("expected 1 read tool in repos, got %d", len(reposToolset.readTools))
	}
	if len(reposToolset.writeTools) != 1 {
		t.Errorf("expected 1 write tool in repos, got %d", len(reposToolset.writeTools))
	}

	// Verify issues toolset
	issuesToolset, exists := tsg.Toolsets["issues"]
	if !exists {
		t.Fatal("expected 'issues' toolset to exist")
	}
	if len(issuesToolset.readTools) != 2 {
		t.Errorf("expected 2 read tools in issues, got %d", len(issuesToolset.readTools))
	}
	if len(issuesToolset.writeTools) != 1 {
		t.Errorf("expected 1 write tool in issues, got %d", len(issuesToolset.writeTools))
	}
}

func TestNewToolsetGroupFromToolsReadOnly(t *testing.T) {
	toolsetMetadatas := []ToolsetMetadata{
		{ID: "repos", Description: "Repository tools"},
	}

	tools := []ServerTool{
		mockToolWithMeta("get_repo", "repos", true),
		mockToolWithMeta("create_repo", "repos", false),
	}

	// Create with readOnly=true
	tsg := NewToolsetGroupFromTools(true, toolsetMetadatas, tools...)

	reposToolset := tsg.Toolsets["repos"]
	if !reposToolset.readOnly {
		t.Error("expected toolset to be in read-only mode")
	}

	// GetActiveTools should only return read tools
	activeTools := reposToolset.GetActiveTools()
	if len(activeTools) != 0 {
		// Toolset is not enabled yet
		t.Errorf("expected 0 active tools (not enabled), got %d", len(activeTools))
	}

	reposToolset.Enabled = true
	activeTools = reposToolset.GetActiveTools()
	if len(activeTools) != 1 {
		t.Errorf("expected 1 active tool in read-only mode, got %d", len(activeTools))
	}
	if activeTools[0].Tool.Name != "get_repo" {
		t.Errorf("expected only read tool 'get_repo', got '%s'", activeTools[0].Tool.Name)
	}
}

func TestNewToolsetGroupFromToolsPanicsOnMissingToolset(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when tool has no toolset in Meta")
		}
	}()

	// Tool without toolset in meta
	tools := []ServerTool{
		{
			Tool: mcp.Tool{
				Name: "bad_tool",
				Meta: nil, // No meta
				Annotations: &mcp.ToolAnnotations{
					ReadOnlyHint: true,
				},
			},
			RegisterFunc: func(_ *mcp.Server) {},
		},
	}

	NewToolsetGroupFromTools(false, nil, tools...)
}

func TestIsReadOnlyTool(t *testing.T) {
	tests := []struct {
		name     string
		tool     ServerTool
		expected bool
	}{
		{
			name: "read-only tool",
			tool: ServerTool{
				Tool: mcp.Tool{
					Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
				},
			},
			expected: true,
		},
		{
			name: "write tool",
			tool: ServerTool{
				Tool: mcp.Tool{
					Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false},
				},
			},
			expected: false,
		},
		{
			name: "no annotations (assume write)",
			tool: ServerTool{
				Tool: mcp.Tool{
					Annotations: nil,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReadOnlyTool(tt.tool)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetToolsetFromMeta(t *testing.T) {
	tests := []struct {
		name     string
		meta     mcp.Meta
		expected string
	}{
		{
			name:     "valid toolset",
			meta:     mcp.Meta{"toolset": "repos"},
			expected: "repos",
		},
		{
			name:     "nil meta",
			meta:     nil,
			expected: "",
		},
		{
			name:     "missing toolset key",
			meta:     mcp.Meta{"other": "value"},
			expected: "",
		},
		{
			name:     "wrong type for toolset",
			meta:     mcp.Meta{"toolset": 123},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getToolsetFromMeta(tt.meta)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestToolsetRegistry_NewToolsetGroup(t *testing.T) {
	toolsetMetadatas := []ToolsetMetadata{
		{ID: "repos", Description: "Repository tools"},
		{ID: "issues", Description: "Issue tools"},
	}

	tools := []ServerTool{
		mockToolWithMeta("get_repo", "repos", true),
		mockToolWithMeta("create_repo", "repos", false),
		mockToolWithMeta("get_issue", "issues", true),
		mockToolWithMeta("create_issue", "issues", false),
	}

	registry := NewToolsetRegistry(toolsetMetadatas, tools)

	t.Run("basic creation", func(t *testing.T) {
		tsg := registry.NewToolsetGroup(ToolsetGroupConfig{})

		if len(tsg.Toolsets) != 2 {
			t.Fatalf("expected 2 toolsets, got %d", len(tsg.Toolsets))
		}

		// Verify toolsets are not enabled by default
		if tsg.IsEnabled("repos") {
			t.Error("expected repos to be disabled by default")
		}
	})

	t.Run("with active toolsets", func(t *testing.T) {
		tsg := registry.NewToolsetGroup(ToolsetGroupConfig{
			ActiveToolsets: []string{"repos"},
		})

		if !tsg.IsEnabled("repos") {
			t.Error("expected repos to be enabled")
		}
		if tsg.IsEnabled("issues") {
			t.Error("expected issues to be disabled")
		}
	})

	t.Run("with read-only mode", func(t *testing.T) {
		tsg := registry.NewToolsetGroup(ToolsetGroupConfig{
			ReadOnly:       true,
			ActiveToolsets: []string{"repos"},
		})

		reposToolset := tsg.Toolsets["repos"]
		if !reposToolset.readOnly {
			t.Error("expected toolset to be in read-only mode")
		}

		activeTools := reposToolset.GetActiveTools()
		if len(activeTools) != 1 {
			t.Errorf("expected 1 active tool in read-only mode, got %d", len(activeTools))
		}
	})
}

func TestToolsetRegistry_NewToolsetGroup_WithScopes(t *testing.T) {
	toolsetMetadatas := []ToolsetMetadata{
		{ID: "repos", Description: "Repository tools"},
		{ID: "issues", Description: "Issue tools"},
	}

	tools := []ServerTool{
		mockToolWithScopes("get_repo", "repos", true, nil),                  // No scope required
		mockToolWithScopes("create_repo", "repos", false, []string{"repo"}), // Requires repo scope
		mockToolWithScopes("get_issue", "issues", true, []string{"repo"}),   // Requires repo scope
		mockToolWithScopes("public_issue", "issues", true, []string{}),      // Empty scopes (no requirement)
	}

	registry := NewToolsetRegistry(toolsetMetadatas, tools)

	t.Run("nil scopes allows all tools", func(t *testing.T) {
		tsg := registry.NewToolsetGroup(ToolsetGroupConfig{
			AvailableScopes: nil,
		})

		reposToolset := tsg.Toolsets["repos"]
		if len(reposToolset.readTools)+len(reposToolset.writeTools) != 2 {
			t.Errorf("expected 2 tools in repos, got %d", len(reposToolset.readTools)+len(reposToolset.writeTools))
		}
	})

	t.Run("empty scopes filters tools requiring scopes", func(t *testing.T) {
		tsg := registry.NewToolsetGroup(ToolsetGroupConfig{
			AvailableScopes: []string{}, // No scopes available
		})

		// repos should only have get_repo (no scope required)
		reposToolset, exists := tsg.Toolsets["repos"]
		if !exists {
			t.Fatal("expected repos toolset to exist")
		}
		if len(reposToolset.readTools) != 1 {
			t.Errorf("expected 1 read tool in repos (no scope required), got %d", len(reposToolset.readTools))
		}
		if len(reposToolset.writeTools) != 0 {
			t.Errorf("expected 0 write tools in repos (scope required), got %d", len(reposToolset.writeTools))
		}

		// issues should only have public_issue (empty scopes)
		issuesToolset, exists := tsg.Toolsets["issues"]
		if !exists {
			t.Fatal("expected issues toolset to exist")
		}
		if len(issuesToolset.readTools) != 1 {
			t.Errorf("expected 1 read tool in issues, got %d", len(issuesToolset.readTools))
		}
	})

	t.Run("with repo scope allows repo-scoped tools", func(t *testing.T) {
		tsg := registry.NewToolsetGroup(ToolsetGroupConfig{
			AvailableScopes: []string{"repo"},
		})

		reposToolset := tsg.Toolsets["repos"]
		if len(reposToolset.readTools) != 1 {
			t.Errorf("expected 1 read tool, got %d", len(reposToolset.readTools))
		}
		if len(reposToolset.writeTools) != 1 {
			t.Errorf("expected 1 write tool, got %d", len(reposToolset.writeTools))
		}

		issuesToolset := tsg.Toolsets["issues"]
		if len(issuesToolset.readTools) != 2 {
			t.Errorf("expected 2 read tools in issues, got %d", len(issuesToolset.readTools))
		}
	})
}

func TestToolScopeSatisfied(t *testing.T) {
	tests := []struct {
		name            string
		tool            ServerTool
		availableScopes []string
		expected        bool
	}{
		{
			name:            "no meta",
			tool:            ServerTool{Tool: mcp.Tool{Meta: nil}},
			availableScopes: []string{},
			expected:        true,
		},
		{
			name:            "no scope requirement",
			tool:            mockToolWithScopes("test", "repos", true, nil),
			availableScopes: []string{},
			expected:        true,
		},
		{
			name:            "empty scope requirement",
			tool:            mockToolWithScopes("test", "repos", true, []string{}),
			availableScopes: []string{},
			expected:        true,
		},
		{
			name:            "scope required and available",
			tool:            mockToolWithScopes("test", "repos", true, []string{"repo"}),
			availableScopes: []string{"repo"},
			expected:        true,
		},
		{
			name:            "scope required but not available",
			tool:            mockToolWithScopes("test", "repos", true, []string{"repo"}),
			availableScopes: []string{"gist"},
			expected:        false,
		},
		{
			name:            "multiple scopes required all available",
			tool:            mockToolWithScopes("test", "repos", true, []string{"repo", "gist"}),
			availableScopes: []string{"repo", "gist", "user"},
			expected:        true,
		},
		{
			name:            "multiple scopes required one missing",
			tool:            mockToolWithScopes("test", "repos", true, []string{"repo", "admin:org"}),
			availableScopes: []string{"repo"},
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toolScopeSatisfied(tt.tool, tt.availableScopes)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
