package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

// generateInsidersFeaturesDocs 使用受各 Insiders feature flag 影响的工具和 schema，
// 刷新 docs/insiders-features.md 的自动生成部分。
func generateInsidersFeaturesDocs(docsPath string) error {
	body := generateFlaggedToolsDoc(github.InsidersFeatureFlags, "_没有 Insiders 专属工具变更。_")
	return rewriteAutomatedSection(docsPath, "START AUTOMATED INSIDERS TOOLS", "END AUTOMATED INSIDERS TOOLS", body)
}

// generateFeatureFlagsDocs 使用受各用户可控 feature flag 影响的工具和 schema，
// 刷新 docs/feature-flags.md 的自动生成部分。
func generateFeatureFlagsDocs(docsPath string) error {
	body := generateFlaggedToolsDoc(github.AllowedFeatureFlags, "_没有用户可控 feature flag 会影响工具注册。_")
	return rewriteAutomatedSection(docsPath, "START AUTOMATED FEATURE FLAG TOOLS", "END AUTOMATED FEATURE FLAG TOOLS", body)
}

// generateFlaggedToolsDoc 为输入集合中的每个 flag 渲染注册或定义不同于默认用户体验的工具。
// 每个受影响的工具均使用 README 所用的同一 writer 输出完整 schema，以保持输出格式一致。
func generateFlaggedToolsDoc(flags []string, emptyMessage string) string {
	t, _ := translations.TranslationHelper()
	defaultTools := indexToolsByName(buildInventoryWithFlags(t, nil).ToolsForRegistration(context.Background()))

	var buf strings.Builder
	hasAny := false

	for _, flag := range flags {
		affected := flaggedToolDiff(t, flag, defaultTools)
		if len(affected) == 0 {
			continue
		}

		if hasAny {
			buf.WriteString("\n\n")
		}
		hasAny = true

		fmt.Fprintf(&buf, "### `%s`\n\n", flag)
		for i, tool := range affected {
			writeToolDoc(&buf, tool)
			if i < len(affected)-1 {
				buf.WriteString("\n\n")
			}
		}
	}

	if !hasAny {
		return emptyMessage
	}
	// body 前后的换行会在内容与周围标记注释之间产生空行，
	// 避免 markdown 渲染器将结尾注释并入最后一个列表项。
	return "\n" + strings.TrimSuffix(buf.String(), "\n") + "\n"
}

// flaggedToolDiff 返回仅启用给定 flag 时，定义（input schema 或 meta）不同于默认清单的工具，
// 以及仅存在于启用 flag 清单中的工具。结果按工具名称排序。
func flaggedToolDiff(t translations.TranslationHelperFunc, flag string, defaultTools map[string]inventory.ServerTool) []inventory.ServerTool {
	flagTools := buildInventoryWithFlags(t, map[string]bool{flag: true}).ToolsForRegistration(context.Background())

	out := make([]inventory.ServerTool, 0)
	seen := make(map[string]struct{}, len(flagTools))

	for _, tool := range flagTools {
		if _, ok := seen[tool.Tool.Name]; ok {
			continue
		}
		seen[tool.Tool.Name] = struct{}{}

		baseline, hadBaseline := defaultTools[tool.Tool.Name]
		if hadBaseline && reflect.DeepEqual(tool.Tool.InputSchema, baseline.Tool.InputSchema) && reflect.DeepEqual(tool.Tool.Meta, baseline.Tool.Meta) {
			continue
		}
		out = append(out, tool)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Tool.Name < out[j].Tool.Name })
	return out
}

// buildInventoryWithFlags 构建一个清单，其 feature checker 将给定 flags 视为启用，
// 将所有其他 flag 视为禁用。传入 nil 会生成默认标记清单。
func buildInventoryWithFlags(t translations.TranslationHelperFunc, enabled map[string]bool) *inventory.Inventory {
	checker := func(_ context.Context, flag string) (bool, error) {
		return enabled[flag], nil
	}
	inv, _ := github.NewInventory(t).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker).
		Build()
	return inv
}

// indexToolsByName 返回以工具名称为键的 map。存在重复项时（如由 flag 控制的双重注册），
// 第一个出现的项胜出，以复现 AvailableTools 的确定性排序顺序。
func indexToolsByName(tools []inventory.ServerTool) map[string]inventory.ServerTool {
	out := make(map[string]inventory.ServerTool, len(tools))
	for _, tool := range tools {
		if _, ok := out[tool.Tool.Name]; ok {
			continue
		}
		out[tool.Tool.Name] = tool
	}
	return out
}

// rewriteAutomatedSection 读取 markdown 文件，以 body 替换指定标记之间的内容，再写回文件。
func rewriteAutomatedSection(path, startMarker, endMarker, body string) error {
	content, err := os.ReadFile(path) //#nosec G304
	if err != nil {
		return fmt.Errorf("读取 docs 文件失败: %w", err)
	}
	updated, err := replaceSection(string(content), startMarker, endMarker, body)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(updated), 0600) //#nosec G306
}
