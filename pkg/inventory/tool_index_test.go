package inventory

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

// mockServerToolInToolset creates a mock tool with a specific toolset
func mockServerToolInToolset(name string, toolsetID ToolsetID, readOnly bool) ServerTool {
	var annotations *mcp.ToolAnnotations
	if readOnly {
		annotations = &mcp.ToolAnnotations{ReadOnlyHint: true}
	}
	return ServerTool{
		Tool: mcp.Tool{
			Name:        name,
			Description: "Test tool: " + name,
			Annotations: annotations,
		},
		Toolset: ToolsetMetadata{ID: toolsetID},
	}
}

// mockServerToolWithFeatureFlag creates a mock tool that requires a feature flag
func mockServerToolWithFeatureFlag(name string, toolsetID ToolsetID, enableFlag string, disableFlag string) ServerTool {
	return ServerTool{
		Tool: mcp.Tool{
			Name:        name,
			Description: "Test tool: " + name,
		},
		Toolset:            ToolsetMetadata{ID: toolsetID},
		FeatureFlagEnable:  enableFlag,
		FeatureFlagDisable: disableFlag,
	}
}

// mockServerToolWithDynamicCheck creates a mock tool with a custom Enabled function
func mockServerToolWithDynamicCheck(name string, toolsetID ToolsetID, enabledFn func(context.Context) (bool, error)) ServerTool {
	return ServerTool{
		Tool: mcp.Tool{
			Name:        name,
			Description: "Test tool: " + name,
		},
		Toolset: ToolsetMetadata{ID: toolsetID},
		Enabled: enabledFn,
	}
}

func TestBuildToolIndex(t *testing.T) {
	t.Parallel()

	// Create test tools in different toolsets
	testTools := []ServerTool{
		mockServerToolInToolset("get_me", "users", true),
		mockServerToolInToolset("list_issues", "issues", true),
		mockServerToolInToolset("create_issue", "issues", false),
		mockServerToolInToolset("list_pull_requests", "pull_requests", true),
		mockServerToolInToolset("create_pull_request", "pull_requests", false),
	}

	index := BuildToolIndex(testTools)

	assert.NotNil(t, index)
	assert.Equal(t, 5, index.ToolCount())
	assert.Equal(t, 3, len(index.ToolsetIDs())) // users, issues, pull_requests
}

func TestToolIndex_Query_AllToolsets(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("tool1", "set_a", true),
		mockServerToolInToolset("tool2", "set_a", true),
		mockServerToolInToolset("tool3", "set_b", true),
	}

	index := BuildToolIndex(testTools)

	// Query for all toolsets
	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"set_a", "set_b"},
		ReadOnly:        false,
	})

	// All 3 tools should be in the result
	assert.Equal(t, 3, result.Guaranteed.PopCount())
	assert.True(t, result.NeedsDynamicCheck.IsEmpty())
}

func TestToolIndex_Query_SingleToolset(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("tool1", "set_a", true),
		mockServerToolInToolset("tool2", "set_a", true),
		mockServerToolInToolset("tool3", "set_b", true),
	}

	index := BuildToolIndex(testTools)

	// Query for only set_a
	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"set_a"},
		ReadOnly:        false,
	})

	// Only 2 tools should be in the result
	assert.Equal(t, 2, result.Guaranteed.PopCount())
}

func TestToolIndex_Query_ReadOnlyMode(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("get_thing", "things", true),     // read-only
		mockServerToolInToolset("create_thing", "things", false), // write
		mockServerToolInToolset("delete_thing", "things", false), // write
		mockServerToolInToolset("list_things", "things", true),   // read-only
	}

	index := BuildToolIndex(testTools)

	// Query in read-only mode
	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"things"},
		ReadOnly:        true,
	})

	// Only read-only tools should be in the result
	assert.Equal(t, 2, result.Guaranteed.PopCount())

	// Materialize and verify
	ctx := context.Background()
	tools := index.Materialize(ctx, result)

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Tool.Name
	}
	assert.Contains(t, names, "get_thing")
	assert.Contains(t, names, "list_things")
	assert.NotContains(t, names, "create_thing")
	assert.NotContains(t, names, "delete_thing")
}

func TestToolIndex_Query_FeatureFlags(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("basic_tool", "tools", true),
		mockServerToolWithFeatureFlag("advanced_tool", "tools", "advanced_features", ""),
		mockServerToolWithFeatureFlag("experimental_tool", "tools", "experimental_features", ""),
	}

	index := BuildToolIndex(testTools)

	// Query with no features enabled
	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"tools"},
		EnabledFeatures: []string{},
		ReadOnly:        false,
	})

	// Only basic_tool should be guaranteed (advanced requires flag)
	assert.Equal(t, 1, result.Guaranteed.PopCount())

	// Materialize to verify
	ctx := context.Background()
	tools := index.Materialize(ctx, result)

	assert.Len(t, tools, 1)
	assert.Equal(t, "basic_tool", tools[0].Tool.Name)

	// Query with advanced_features enabled
	result = index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"tools"},
		EnabledFeatures: []string{"advanced_features"},
		ReadOnly:        false,
	})

	assert.Equal(t, 2, result.Guaranteed.PopCount())
}

func TestToolIndex_Query_FeatureFlagDisables(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("standard_tool", "tools", true),
		mockServerToolWithFeatureFlag("legacy_tool", "tools", "", "new_mode"),
	}

	index := BuildToolIndex(testTools)

	// Query with new_mode OFF - legacy tool should be available
	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"tools"},
		EnabledFeatures: []string{},
		ReadOnly:        false,
	})
	assert.Equal(t, 2, result.Guaranteed.PopCount())

	// Query with new_mode ON - legacy tool should be disabled
	result = index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"tools"},
		EnabledFeatures: []string{"new_mode"},
		ReadOnly:        false,
	})
	assert.Equal(t, 1, result.Guaranteed.PopCount())

	ctx := context.Background()
	tools := index.Materialize(ctx, result)

	assert.Len(t, tools, 1)
	assert.Equal(t, "standard_tool", tools[0].Tool.Name)
}

func TestToolIndex_Query_DynamicChecks(t *testing.T) {
	t.Parallel()

	// Tool with dynamic Enabled check
	dynamicTool := mockServerToolWithDynamicCheck("dynamic_tool", "tools", func(_ context.Context) (bool, error) {
		return true, nil
	})

	testTools := []ServerTool{
		mockServerToolInToolset("static_tool", "tools", true),
		dynamicTool,
	}

	index := BuildToolIndex(testTools)

	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"tools"},
		ReadOnly:        false,
	})

	// static_tool is guaranteed, dynamic_tool needs check
	assert.Equal(t, 1, result.Guaranteed.PopCount())
	assert.Equal(t, 1, result.NeedsDynamicCheck.PopCount())
	assert.Equal(t, 2, result.StaticFiltered.PopCount())
}

func TestToolIndex_Query_DynamicChecksFilteredByToolset(t *testing.T) {
	t.Parallel()

	// Key test: dynamic check tool should NOT appear in NeedsDynamicCheck
	// if it's already filtered out by toolset

	dynamicTool := mockServerToolWithDynamicCheck("dynamic_tool", "set_b", func(_ context.Context) (bool, error) {
		return true, nil
	})

	testTools := []ServerTool{
		mockServerToolInToolset("static_tool", "set_a", true),
		dynamicTool,
	}

	index := BuildToolIndex(testTools)

	// Query only for set_a - dynamic_tool is in set_b, so it's filtered out
	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"set_a"},
		ReadOnly:        false,
	})

	// static_tool is guaranteed
	assert.Equal(t, 1, result.Guaranteed.PopCount())
	// dynamic_tool should NOT be in NeedsDynamicCheck because it's already filtered
	assert.True(t, result.NeedsDynamicCheck.IsEmpty())
}

func TestToolIndex_Materialize_NoDynamicChecks(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("tool1", "all", true),
		mockServerToolInToolset("tool2", "all", true),
		mockServerToolInToolset("tool3", "all", true),
	}

	index := BuildToolIndex(testTools)

	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"all"},
		ReadOnly:        false,
	})

	ctx := context.Background()
	materializedTools := index.Materialize(ctx, result)

	assert.Len(t, materializedTools, 3)

	// Verify tool names
	names := make([]string, len(materializedTools))
	for i, tool := range materializedTools {
		names[i] = tool.Tool.Name
	}
	assert.Contains(t, names, "tool1")
	assert.Contains(t, names, "tool2")
	assert.Contains(t, names, "tool3")
}

func TestToolIndex_Materialize_WithDynamicChecks(t *testing.T) {
	t.Parallel()

	enabledDynamic := mockServerToolWithDynamicCheck("enabled_dynamic", "all", func(_ context.Context) (bool, error) {
		return true, nil
	})

	disabledDynamic := mockServerToolWithDynamicCheck("disabled_dynamic", "all", func(_ context.Context) (bool, error) {
		return false, nil
	})

	testTools := []ServerTool{
		mockServerToolInToolset("static_tool", "all", true),
		enabledDynamic,
		disabledDynamic,
	}

	index := BuildToolIndex(testTools)

	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"all"},
		ReadOnly:        false,
	})

	ctx := context.Background()
	materializedTools := index.Materialize(ctx, result)

	// Should have static_tool and enabled_dynamic (disabled_dynamic returns false)
	assert.Len(t, materializedTools, 2)

	names := make([]string, len(materializedTools))
	for i, tool := range materializedTools {
		names[i] = tool.Tool.Name
	}
	assert.Contains(t, names, "static_tool")
	assert.Contains(t, names, "enabled_dynamic")
	assert.NotContains(t, names, "disabled_dynamic")
}

func TestToolIndex_Materialize_DynamicCheckError(t *testing.T) {
	t.Parallel()

	errorTool := mockServerToolWithDynamicCheck("error_tool", "all", func(_ context.Context) (bool, error) {
		return false, context.DeadlineExceeded
	})

	testTools := []ServerTool{
		mockServerToolInToolset("static_tool", "all", true),
		errorTool,
	}

	index := BuildToolIndex(testTools)

	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"all"},
		ReadOnly:        false,
	})

	ctx := context.Background()
	materializedTools := index.Materialize(ctx, result)

	// Current implementation skips on error - verify the result
	// Only static_tool should be present
	// The implementation silently skips errors
	assert.Len(t, materializedTools, 1)
	assert.Equal(t, "static_tool", materializedTools[0].Tool.Name)
}

func TestToolIndex_Query_AllToolsetsFlag(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("tool1", "set_a", true),
		mockServerToolInToolset("tool2", "set_b", true),
		mockServerToolInToolset("tool3", "set_c", true),
	}

	index := BuildToolIndex(testTools)

	// Query with AllToolsets flag
	result := index.Query(QueryConfig{
		AllToolsets: true,
		ReadOnly:    false,
	})

	assert.Equal(t, 3, result.Guaranteed.PopCount())
}

func TestToolIndex_Query_AdditionalTools(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("tool1", "set_a", true),
		mockServerToolInToolset("tool2", "set_b", true),
		mockServerToolInToolset("special_tool", "set_c", true),
	}

	index := BuildToolIndex(testTools)

	// Query for set_a only, but include special_tool via AdditionalTools
	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"set_a"},
		AdditionalTools: []string{"special_tool"},
		ReadOnly:        false,
	})

	// Should have tool1 (from set_a) and special_tool (from additional)
	assert.Equal(t, 2, result.Guaranteed.PopCount())

	ctx := context.Background()
	tools := index.Materialize(ctx, result)

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Tool.Name
	}
	assert.Contains(t, names, "tool1")
	assert.Contains(t, names, "special_tool")
	assert.NotContains(t, names, "tool2")
}

func TestToolIndex_GetTool(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("alpha", "set", true),
		mockServerToolInToolset("beta", "set", true),
	}

	index := BuildToolIndex(testTools)

	// Get tool by position
	tool := index.GetTool(0)
	assert.NotNil(t, tool)

	// Out of bounds
	tool = index.GetTool(-1)
	assert.Nil(t, tool)
	tool = index.GetTool(100)
	assert.Nil(t, tool)
}

func TestToolIndex_GetToolByName(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("alpha", "set", true),
		mockServerToolInToolset("beta", "set", true),
	}

	index := BuildToolIndex(testTools)

	// Get tool by name
	tool, pos, ok := index.GetToolByName("alpha")
	assert.True(t, ok)
	assert.NotNil(t, tool)
	assert.Equal(t, "alpha", tool.Tool.Name)
	assert.GreaterOrEqual(t, pos, 0)

	// Non-existent
	tool, pos, ok = index.GetToolByName("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, tool)
	assert.Equal(t, -1, pos)
}

func TestToolIndex_ToolsetBitmap(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("tool1", "set_a", true),
		mockServerToolInToolset("tool2", "set_a", true),
		mockServerToolInToolset("tool3", "set_b", true),
	}

	index := BuildToolIndex(testTools)

	bmA := index.ToolsetBitmap("set_a")
	assert.Equal(t, 2, bmA.PopCount())

	bmB := index.ToolsetBitmap("set_b")
	assert.Equal(t, 1, bmB.PopCount())

	// Non-existent toolset returns empty bitmap
	bmX := index.ToolsetBitmap("nonexistent")
	assert.True(t, bmX.IsEmpty())
}

func TestToolIndex_UniqueFeatureFlags(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("basic_tool", "tools", true),
		mockServerToolWithFeatureFlag("advanced_tool", "tools", "feature_a", ""),
		mockServerToolWithFeatureFlag("experimental_tool", "tools", "feature_b", ""),
		mockServerToolWithFeatureFlag("legacy_tool", "tools", "", "feature_c"),
		mockServerToolWithFeatureFlag("complex_tool", "tools", "feature_a", "feature_d"), // reuses feature_a
	}

	index := BuildToolIndex(testTools)

	flags := index.UniqueFeatureFlags()

	// Should have 4 unique flags: feature_a, feature_b, feature_c, feature_d
	assert.Len(t, flags, 4)
	assert.Contains(t, flags, "feature_a")
	assert.Contains(t, flags, "feature_b")
	assert.Contains(t, flags, "feature_c")
	assert.Contains(t, flags, "feature_d")
}

func TestToolIndex_QueryWithFeatureChecker(t *testing.T) {
	t.Parallel()

	testTools := []ServerTool{
		mockServerToolInToolset("basic_tool", "tools", true),
		mockServerToolWithFeatureFlag("needs_feature_a", "tools", "feature_a", ""),
		mockServerToolWithFeatureFlag("needs_feature_b", "tools", "feature_b", ""),
		mockServerToolWithFeatureFlag("disabled_by_feature_c", "tools", "", "feature_c"),
	}

	index := BuildToolIndex(testTools)

	// Track which flags were checked
	checkedFlags := make(map[string]int)

	checker := func(_ context.Context, flag string) (bool, error) {
		checkedFlags[flag]++
		// Enable feature_a, disable feature_b, enable feature_c
		switch flag {
		case "feature_a":
			return true, nil
		case "feature_b":
			return false, nil
		case "feature_c":
			return true, nil // This will disable "disabled_by_feature_c"
		default:
			return false, nil
		}
	}

	ctx := context.Background()
	result := index.QueryWithFeatureChecker(ctx, QueryConfigWithChecker{
		EnabledToolsets: []ToolsetID{"tools"},
		ReadOnly:        false,
		FeatureChecker:  checker,
	})

	// Each flag should be checked exactly once
	assert.Equal(t, 1, checkedFlags["feature_a"], "feature_a should be checked once")
	assert.Equal(t, 1, checkedFlags["feature_b"], "feature_b should be checked once")
	assert.Equal(t, 1, checkedFlags["feature_c"], "feature_c should be checked once")

	// Materialize and verify results
	tools := index.Materialize(ctx, result)
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Tool.Name
	}

	// basic_tool: no flags, included
	// needs_feature_a: feature_a enabled, included
	// needs_feature_b: feature_b disabled, excluded
	// disabled_by_feature_c: feature_c enabled, excluded
	assert.Len(t, tools, 2)
	assert.Contains(t, names, "basic_tool")
	assert.Contains(t, names, "needs_feature_a")
	assert.NotContains(t, names, "needs_feature_b")
	assert.NotContains(t, names, "disabled_by_feature_c")
}

func TestToolIndex_QueryWithFeatureChecker_MinimizesChecks(t *testing.T) {
	t.Parallel()

	// Create 50 tools that all use the same 3 feature flags
	testTools := make([]ServerTool, 50)
	for i := 0; i < 50; i++ {
		flag := []string{"flag_a", "flag_b", "flag_c"}[i%3]
		testTools[i] = mockServerToolWithFeatureFlag(
			"tool_"+string(rune('a'+i%26)),
			"tools",
			flag, // All tools require one of 3 flags
			"",
		)
	}

	index := BuildToolIndex(testTools)

	checkCount := 0
	checker := func(_ context.Context, _ string) (bool, error) {
		checkCount++
		return true, nil
	}

	ctx := context.Background()
	_ = index.QueryWithFeatureChecker(ctx, QueryConfigWithChecker{
		EnabledToolsets: []ToolsetID{"tools"},
		FeatureChecker:  checker,
	})

	// Should only check 3 unique flags, not 50 tools
	assert.Equal(t, 3, checkCount, "Should check each unique flag exactly once")
}

func BenchmarkBuildToolIndex_130Tools(b *testing.B) {
	// Create realistic toolset distribution
	toolsets := []ToolsetID{"repos", "issues", "pull_requests", "users", "actions", "code_security", "projects", "notifications", "discussions", "experiments"}

	testTools := make([]ServerTool, 130)
	for i := 0; i < 130; i++ {
		toolset := toolsets[i%len(toolsets)]
		readOnly := i%3 != 0 // 2/3 are read-only
		testTools[i] = mockServerToolInToolset("tool_"+string(rune('a'+i%26))+string(rune('0'+i/26)), toolset, readOnly)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildToolIndex(testTools)
	}
}

func BenchmarkToolIndex_Query_SmallConfig(b *testing.B) {
	toolsets := []ToolsetID{"repos", "issues", "pull_requests", "users", "actions", "code_security", "projects", "notifications", "discussions", "experiments"}

	testTools := make([]ServerTool, 130)
	for i := 0; i < 130; i++ {
		toolset := toolsets[i%len(toolsets)]
		readOnly := i%3 != 0
		testTools[i] = mockServerToolInToolset("tool_"+string(rune('a'+i%26))+string(rune('0'+i/26)), toolset, readOnly)
	}

	index := BuildToolIndex(testTools)

	config := QueryConfig{
		EnabledToolsets: []ToolsetID{"repos", "issues"},
		ReadOnly:        false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = index.Query(config)
	}
}

func BenchmarkToolIndex_Query_AllToolsets(b *testing.B) {
	toolsets := []ToolsetID{"repos", "issues", "pull_requests", "users", "actions", "code_security", "projects", "notifications", "discussions", "experiments"}

	testTools := make([]ServerTool, 130)
	for i := 0; i < 130; i++ {
		toolset := toolsets[i%len(toolsets)]
		readOnly := i%3 != 0
		testTools[i] = mockServerToolInToolset("tool_"+string(rune('a'+i%26))+string(rune('0'+i/26)), toolset, readOnly)
	}

	index := BuildToolIndex(testTools)

	config := QueryConfig{
		AllToolsets: true,
		ReadOnly:    true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = index.Query(config)
	}
}

func BenchmarkToolIndex_Materialize_NoDynamic(b *testing.B) {
	testTools := make([]ServerTool, 50)
	for i := 0; i < 50; i++ {
		testTools[i] = mockServerToolInToolset("tool_"+string(rune('a'+i%26)), "all", true)
	}

	index := BuildToolIndex(testTools)

	result := index.Query(QueryConfig{
		EnabledToolsets: []ToolsetID{"all"},
		ReadOnly:        false,
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = index.Materialize(ctx, result)
	}
}

func BenchmarkToolIndex_QueryAndMaterialize_Realistic(b *testing.B) {
	// Simulate realistic scenario: ~130 tools, 10 toolsets, some with dynamic checks
	toolsets := []ToolsetID{"repos", "issues", "pull_requests", "users", "actions", "code_security", "projects", "notifications", "discussions", "experiments"}

	testTools := make([]ServerTool, 130)
	for i := 0; i < 130; i++ {
		toolset := toolsets[i%len(toolsets)]
		readOnly := i%3 != 0

		// 10% have dynamic checks
		if i%10 == 0 {
			testTools[i] = mockServerToolWithDynamicCheck(
				"tool_"+string(rune('a'+i%26))+string(rune('0'+i/26)),
				toolset,
				func(_ context.Context) (bool, error) { return true, nil },
			)
			if readOnly {
				testTools[i].Tool.Annotations = &mcp.ToolAnnotations{ReadOnlyHint: true}
			}
		} else {
			testTools[i] = mockServerToolInToolset("tool_"+string(rune('a'+i%26))+string(rune('0'+i/26)), toolset, readOnly)
		}
	}

	index := BuildToolIndex(testTools)

	config := QueryConfig{
		EnabledToolsets: []ToolsetID{"repos", "issues", "pull_requests", "actions"},
		ReadOnly:        false,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := index.Query(config)
		_ = index.Materialize(ctx, result)
	}
}
