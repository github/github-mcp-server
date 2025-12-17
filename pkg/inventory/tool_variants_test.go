package inventory

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func makeTool(name string) ServerTool {
	return ServerTool{
		Tool: mcp.Tool{
			Name:        name,
			Description: "Tool " + name,
		},
	}
}

func makeOverride(name, desc string) ServerTool {
	return ServerTool{
		Tool: mcp.Tool{
			Name:        name,
			Description: desc,
		},
	}
}

func TestToolOverrides_Apply(t *testing.T) {
	t.Parallel()

	overrides := ToolOverrides{
		"create_issue": {
			ToolName:  "create_issue",
			Condition: func(_ context.Context) (bool, error) { return true, nil },
			Override:  makeOverride("create_issue", "Enterprise variant"),
		},
	}

	ctx := context.Background()

	// Tool with override
	result := overrides.Apply(ctx, "create_issue")
	assert.NotNil(t, result)
	assert.Equal(t, "Enterprise variant", result.Tool.Description)

	// Tool without override
	result = overrides.Apply(ctx, "list_repos")
	assert.Nil(t, result)
}

func TestToolOverrides_Apply_ConditionFalse(t *testing.T) {
	t.Parallel()

	overrides := ToolOverrides{
		"create_issue": {
			ToolName:  "create_issue",
			Condition: func(_ context.Context) (bool, error) { return false, nil },
			Override:  makeOverride("create_issue", "Enterprise variant"),
		},
	}

	ctx := context.Background()

	// Condition doesn't match - no override
	result := overrides.Apply(ctx, "create_issue")
	assert.Nil(t, result)
}

func TestToolOverrides_Apply_NilCondition(t *testing.T) {
	t.Parallel()

	overrides := ToolOverrides{
		"create_issue": {
			ToolName: "create_issue",
			// nil Condition - always applies
			Override: makeOverride("create_issue", "Always applied"),
		},
	}

	ctx := context.Background()

	result := overrides.Apply(ctx, "create_issue")
	assert.NotNil(t, result)
	assert.Equal(t, "Always applied", result.Tool.Description)
}

func TestToolOverrides_ApplyToTools(t *testing.T) {
	t.Parallel()

	tools := []*ServerTool{
		ptr(makeTool("create_issue")),
		ptr(makeTool("list_repos")),
		ptr(makeTool("get_me")),
	}

	overrides := ToolOverrides{
		"create_issue": {
			ToolName:  "create_issue",
			Condition: func(_ context.Context) (bool, error) { return true, nil },
			Override:  makeOverride("create_issue", "Enterprise create_issue"),
		},
	}

	ctx := context.Background()
	result := overrides.ApplyToTools(ctx, tools)

	assert.Len(t, result, 3)
	assert.Equal(t, "Enterprise create_issue", result[0].Tool.Description)
	assert.Equal(t, "Tool list_repos", result[1].Tool.Description)
	assert.Equal(t, "Tool get_me", result[2].Tool.Description)
}

func TestToolOverrides_ApplyToTools_Empty(t *testing.T) {
	t.Parallel()

	tools := []*ServerTool{
		ptr(makeTool("create_issue")),
	}

	overrides := ToolOverrides{}

	ctx := context.Background()
	result := overrides.ApplyToTools(ctx, tools)

	// Empty overrides returns original slice
	assert.Equal(t, tools, result)
}

func ptr(t ServerTool) *ServerTool {
	return &t
}

func BenchmarkToolOverrides_ApplyToTools(b *testing.B) {
	// 130 tools, 2 overrides (realistic)
	tools := make([]*ServerTool, 130)
	for i := range tools {
		tools[i] = ptr(makeTool("tool_" + string(rune('a'+i%26))))
	}

	overrides := ToolOverrides{
		"tool_a": {
			ToolName:  "tool_a",
			Condition: func(_ context.Context) (bool, error) { return true, nil },
			Override:  makeOverride("tool_a", "Override A"),
		},
		"tool_b": {
			ToolName:  "tool_b",
			Condition: func(_ context.Context) (bool, error) { return true, nil },
			Override:  makeOverride("tool_b", "Override B"),
		},
	}

	ctx := context.Background()

	b.ReportAllocs() // Only count allocs in the hot loop
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = overrides.ApplyToTools(ctx, tools)
	}
}

func BenchmarkToolOverrides_ApplyToTools_NoMatch(b *testing.B) {
	// 130 tools, overrides don't match any - should be zero alloc
	tools := make([]*ServerTool, 130)
	for i := range tools {
		tools[i] = ptr(makeTool("tool_" + string(rune('a'+i%26))))
	}

	// Override exists but for a tool not in list
	overrides := ToolOverrides{
		"nonexistent_tool": {
			ToolName: "nonexistent_tool",
			Override: makeOverride("nonexistent_tool", "Override"),
		},
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = overrides.ApplyToTools(ctx, tools)
	}
}
