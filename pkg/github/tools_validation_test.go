package github

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubTranslation is 一个simple translation 函数 f或testing
func stubTranslation(_, fallback string) string {
	return fallback
}

// TestAllToolsHaveRequiredMeta数据 验证所有 工具 have mandatory 元数据:
// - Toolset 必须是 set (non-空 ID)
// - ReadOnlyHint annotation 必须是 explicitly set (不nil)
func TestAllToolsHaveRequiredMetadata(t *testing.T) {
	tools := AllTools(stubTranslation)

	require.NotEmpty(t, tools, "AllTools should return at least one tool")

	for _, tool := range tools {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			// Toolset ID 必须是 set
			assert.NotEmpty(t, tool.Toolset.ID,
				"Tool %q must have a Toolset.ID", tool.Tool.Name)

			// Toolset description 应当是 set f或documentation
			assert.NotEmpty(t, tool.Toolset.Description,
				"Tool %q should have a Toolset.Description", tool.Tool.Name)

			// Annotations must exist 和have ReadOnlyHint explicitly set
			require.NotNil(t, tool.Tool.Annotations,
				"Tool %q must have Annotations set (for ReadOnlyHint)", tool.Tool.Name)

			// We can't distinguish between "不set" 和"set to 假" f或一个bool,
			// 但having Annotations non-nil confirms developer thought about it.
			// ReadOnlyHint 值 itself is 验证d by ensuring Annotations exist.
		})
	}
}

// TestAllResourcesHaveRequiredMeta数据 验证所有 资源 have mandatory 元数据
func TestAllResourcesHaveRequiredMetadata(t *testing.T) {
	// Resources 现在 stateless - no 客户端 函数s needed
	resources := AllResources(stubTranslation)

	require.NotEmpty(t, resources, "AllResources should return at least one resource")

	for _, res := range resources {
		t.Run(res.Template.Name, func(t *testing.T) {
			// Toolset ID 必须是 set
			assert.NotEmpty(t, res.Toolset.ID,
				"Resource %q must have a Toolset.ID", res.Template.Name)

			// HandlerFunc 必须是 set
			assert.True(t, res.HasHandler(),
				"Resource %q must have a HandlerFunc", res.Template.Name)
		})
	}
}

// TestAllPromptsHaveRequiredMeta数据 验证所有 提示 have mandatory 元数据
func TestAllPromptsHaveRequiredMetadata(t *testing.T) {
	prompts := AllPrompts(stubTranslation)

	require.NotEmpty(t, prompts, "AllPrompts should return at least one prompt")

	for _, prompt := range prompts {
		t.Run(prompt.Prompt.Name, func(t *testing.T) {
			// Toolset ID 必须是 set
			assert.NotEmpty(t, prompt.Toolset.ID,
				"Prompt %q must have a Toolset.ID", prompt.Prompt.Name)

			// Handler 必须是 set
			assert.NotNil(t, prompt.Handler,
				"Prompt %q must have a Handler", prompt.Prompt.Name)
		})
	}
}

// TestToolReadOnlyHintConsistency 验证s that 读取-仅工具 are correctly annotated
func TestToolReadOnlyHintConsistency(t *testing.T) {
	tools := AllTools(stubTranslation)

	for _, tool := range tools {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			require.NotNil(t, tool.Tool.Annotations,
				"Tool %q must have Annotations", tool.Tool.Name)

			// Verify IsReadOnly() method matches annotation
			assert.Equal(t, tool.Tool.Annotations.ReadOnlyHint, tool.IsReadOnly(),
				"Tool %q: IsReadOnly() should match Annotations.ReadOnlyHint", tool.Tool.Name)
		})
	}
}

// TestNoDuplicateToolNames 确保所有 工具 have unique names
func TestNoDuplicateToolNames(t *testing.T) {
	tools := AllTools(stubTranslation)
	seen := make(map[string]bool)
	featureFlagged := make(map[string]bool)

	// 获取_label is intentionally in both 议题 和labels 工具集s f或conformance
	// with original behavi或where it was registered in both
	allowedDuplicates := map[string]bool{
		"get_label": true,
	}

	// First pass: identify 工具 that have 功能标志 (mutually exclusive at runtime)
	for _, tool := range tools {
		if tool.FeatureFlagEnable != "" || len(tool.FeatureFlagDisable) > 0 {
			featureFlagged[tool.Tool.Name] = true
		}
	}

	for _, tool := range tools {
		name := tool.Tool.Name
		// Allow duplicates f或explicitly allowed 工具 和feature-flagged 工具
		if !allowedDuplicates[name] && !featureFlagged[name] {
			assert.False(t, seen[name],
				"Duplicate tool name found: %q", name)
		}
		seen[name] = true
	}
}

// TestNoDuplicateResourceNames 确保所有 资源 have unique names
func TestNoDuplicateResourceNames(t *testing.T) {
	resources := AllResources(stubTranslation)
	seen := make(map[string]bool)

	for _, res := range resources {
		name := res.Template.Name
		assert.False(t, seen[name],
			"Duplicate resource name found: %q", name)
		seen[name] = true
	}
}

// TestNoDuplicatePromptNames 确保所有 提示 have unique names
func TestNoDuplicatePromptNames(t *testing.T) {
	prompts := AllPrompts(stubTranslation)
	seen := make(map[string]bool)

	for _, prompt := range prompts {
		name := prompt.Prompt.Name
		assert.False(t, seen[name],
			"Duplicate prompt name found: %q", name)
		seen[name] = true
	}
}

// TestAllToolsHaveHandlerFunc 确保所有 工具 have 一个处理器 函数
func TestAllToolsHaveHandlerFunc(t *testing.T) {
	tools := AllTools(stubTranslation)

	for _, tool := range tools {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			assert.NotNil(t, tool.HandlerFunc,
				"Tool %q must have a HandlerFunc", tool.Tool.Name)
			assert.True(t, tool.HasHandler(),
				"Tool %q HasHandler() should return true", tool.Tool.Name)
		})
	}
}

// TestToolsetMeta数据Consistency ensures 工具 在相同 工具集 have consistent descriptions
func TestToolsetMetadataConsistency(t *testing.T) {
	tools := AllTools(stubTranslation)
	toolsetDescriptions := make(map[inventory.ToolsetID]string)

	for _, tool := range tools {
		id := tool.Toolset.ID
		desc := tool.Toolset.Description

		if existing, ok := toolsetDescriptions[id]; ok {
			assert.Equal(t, existing, desc,
				"Toolset %q has inconsistent descriptions across tools", id)
		} else {
			toolsetDescriptions[id] = desc
		}
	}
}

func TestGitHubPackageDoesNotReadInsidersMode(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, 0)
		require.NoError(t, err, "failed to parse %s", file)

		ast.Inspect(node, func(n ast.Node) bool {
			selector, ok := n.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "InsidersMode" {
				return true
			}

			position := fset.Position(selector.Sel.Pos())
			t.Errorf("%s reads InsidersMode directly; gate behavior on concrete feature flags instead", position)
			return true
		})
	}
}
