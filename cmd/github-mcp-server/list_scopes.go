package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ToolScopeInfo 包含单个工具的 scope 信息。
type ToolScopeInfo struct {
	Name           string   `json:"name"`
	Toolset        string   `json:"toolset"`
	ReadOnly       bool     `json:"read_only"`
	RequiredScopes []string `json:"required_scopes"`
	AcceptedScopes []string `json:"accepted_scopes,omitempty"`
}

// ScopesOutput 是 list-scopes 命令的完整输出结构。
type ScopesOutput struct {
	Tools           []ToolScopeInfo     `json:"tools"`
	UniqueScopes    []string            `json:"unique_scopes"`
	ScopesByTool    map[string][]string `json:"scopes_by_tool"`
	ToolsByScope    map[string][]string `json:"tools_by_scope"`
	EnabledToolsets []string            `json:"enabled_toolsets"`
	ReadOnly        bool                `json:"read_only"`
}

var listScopesCmd = &cobra.Command{
	Use:   "list-scopes",
	Short: "List required OAuth scopes for enabled tools",
	Long: `List the required OAuth scopes for all enabled tools.

This command creates an inventory based on the same flags as the stdio command
and outputs the required OAuth scopes for each enabled tool. This is useful for
determining what scopes a token needs to use specific tools.

The output format can be controlled with the --output flag:
  - text (default): Human-readable text output
  - json: JSON output for programmatic use
  - summary: Just the unique scopes needed

Examples:
  # List scopes for default toolsets
  github-mcp-server list-scopes

  # List scopes for specific toolsets
  github-mcp-server list-scopes --toolsets=repos,issues,pull_requests

  # List scopes for all toolsets
  github-mcp-server list-scopes --toolsets=all

  # Output as JSON
  github-mcp-server list-scopes --output=json

  # Just show unique scopes needed
  github-mcp-server list-scopes --output=summary`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runListScopes()
	},
}

func init() {
	listScopesCmd.Flags().StringP("output", "o", "text", "Output format: text, json, or summary")
	_ = viper.BindPFlag("list-scopes-output", listScopesCmd.Flags().Lookup("output"))

	rootCmd.AddCommand(listScopesCmd)
}

// formatScopeDisplay 格式化供显示的 scope 字符串，并处理空 scope。
func formatScopeDisplay(scope string) string {
	if scope == "" {
		return "(no scope required for public read access)"
	}
	return scope
}

func runListScopes() error {
	// 获取工具集配置（与 stdio 命令使用相同逻辑）。
	var enabledToolsets []string
	if viper.IsSet("toolsets") {
		if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
			return fmt.Errorf("failed to unmarshal toolsets: %w", err)
		}
	}
	// 否则 enabledToolsets 保持为 nil，表示“使用默认值”。

	// 获取指定工具（与工具集类似）。
	var enabledTools []string
	if viper.IsSet("tools") {
		if err := viper.UnmarshalKey("tools", &enabledTools); err != nil {
			return fmt.Errorf("failed to unmarshal tools: %w", err)
		}
	}

	readOnly := viper.GetBool("read-only")
	outputFormat := viper.GetString("list-scopes-output")

	// 创建翻译辅助程序。
	t, _ := translations.TranslationHelper()

	// 使用与 stdio 服务器相同的逻辑构建清单。
	inventoryBuilder := github.NewInventory(t).
		WithReadOnly(readOnly)

	// 配置工具集（与 stdio 相同）。
	if enabledToolsets != nil {
		inventoryBuilder = inventoryBuilder.WithToolsets(enabledToolsets)
	}

	// 配置指定工具。
	if len(enabledTools) > 0 {
		inventoryBuilder = inventoryBuilder.WithTools(enabledTools)
	}

	inv, err := inventoryBuilder.Build()
	if err != nil {
		return fmt.Errorf("failed to build inventory: %w", err)
	}

	// 收集所有工具及其 scope。
	output := collectToolScopes(inv, readOnly)

	// 根据格式输出。
	switch outputFormat {
	case "json":
		return outputJSON(output)
	case "summary":
		return outputSummary(output)
	default:
		return outputText(output)
	}
}

func collectToolScopes(inv *inventory.Inventory, readOnly bool) ScopesOutput {
	var tools []ToolScopeInfo
	scopeSet := make(map[string]bool)
	scopesByTool := make(map[string][]string)
	toolsByScope := make(map[string][]string)

	// 从清单获取所有可用工具。
	// 使用 context.Background() 评估 feature flag。
	availableTools := inv.AvailableTools(context.Background())

	for _, serverTool := range availableTools {
		tool := serverTool.Tool

		// 直接从 ServerTool 获取 scope 信息。
		requiredScopes := serverTool.RequiredScopes
		acceptedScopes := serverTool.AcceptedScopes

		// 确定工具是否只读。
		isReadOnly := serverTool.IsReadOnly()

		toolInfo := ToolScopeInfo{
			Name:           tool.Name,
			Toolset:        string(serverTool.Toolset.ID),
			ReadOnly:       isReadOnly,
			RequiredScopes: requiredScopes,
			AcceptedScopes: acceptedScopes,
		}
		tools = append(tools, toolInfo)

		// 跟踪唯一的 scope。
		for _, s := range requiredScopes {
			scopeSet[s] = true
			toolsByScope[s] = append(toolsByScope[s], tool.Name)
		}

		// 按工具跟踪 scope。
		scopesByTool[tool.Name] = requiredScopes
	}

	// 按名称排序工具。
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	// 获取排序后的唯一 scope 切片。
	var uniqueScopes []string
	for s := range scopeSet {
		uniqueScopes = append(uniqueScopes, s)
	}
	sort.Strings(uniqueScopes)

	// 对每个 scope 内的工具排序。
	for scope := range toolsByScope {
		sort.Strings(toolsByScope[scope])
	}

	// 将已启用工具集获取为字符串切片。
	toolsetIDs := inv.ToolsetIDs()
	toolsetIDStrs := make([]string, len(toolsetIDs))
	for i, id := range toolsetIDs {
		toolsetIDStrs[i] = string(id)
	}

	return ScopesOutput{
		Tools:           tools,
		UniqueScopes:    uniqueScopes,
		ScopesByTool:    scopesByTool,
		ToolsByScope:    toolsByScope,
		EnabledToolsets: toolsetIDStrs,
		ReadOnly:        readOnly,
	}
}

func outputJSON(output ScopesOutput) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputSummary(output ScopesOutput) error {
	if len(output.UniqueScopes) == 0 {
		fmt.Println("No OAuth scopes required for enabled tools.")
		return nil
	}

	fmt.Println("Required OAuth scopes for enabled tools:")
	fmt.Println()
	for _, scope := range output.UniqueScopes {
		fmt.Printf("  %s\n", formatScopeDisplay(scope))
	}
	fmt.Printf("\nTotal: %d unique scope(s)\n", len(output.UniqueScopes))
	return nil
}

func outputText(output ScopesOutput) error {
	fmt.Printf("OAuth Scopes for Enabled Tools\n")
	fmt.Printf("==============================\n\n")

	fmt.Printf("Enabled Toolsets: %s\n", strings.Join(output.EnabledToolsets, ", "))
	fmt.Printf("Read-Only Mode: %v\n\n", output.ReadOnly)

	// 按工具集对工具分组。
	toolsByToolset := make(map[string][]ToolScopeInfo)
	for _, tool := range output.Tools {
		toolsByToolset[tool.Toolset] = append(toolsByToolset[tool.Toolset], tool)
	}

	// 获取排序后的工具集名称。
	var toolsetNames []string
	for name := range toolsByToolset {
		toolsetNames = append(toolsetNames, name)
	}
	sort.Strings(toolsetNames)

	for _, toolsetName := range toolsetNames {
		tools := toolsByToolset[toolsetName]
		fmt.Printf("## %s\n\n", formatToolsetName(toolsetName))

		for _, tool := range tools {
			rwIndicator := "📝"
			if tool.ReadOnly {
				rwIndicator = "👁"
			}

			scopeStr := "(no scope required)"
			if len(tool.RequiredScopes) > 0 {
				scopeStr = strings.Join(tool.RequiredScopes, ", ")
			}

			fmt.Printf("  %s %s: %s\n", rwIndicator, tool.Name, scopeStr)
		}
		fmt.Println()
	}

	// 摘要。
	fmt.Println("## Summary")
	fmt.Println()
	if len(output.UniqueScopes) == 0 {
		fmt.Println("No OAuth scopes required for enabled tools.")
	} else {
		fmt.Println("Unique scopes required:")
		for _, scope := range output.UniqueScopes {
			fmt.Printf("  • %s\n", formatScopeDisplay(scope))
		}
	}
	fmt.Printf("\nTotal: %d tools, %d unique scopes\n", len(output.Tools), len(output.UniqueScopes))

	// 图例。
	fmt.Println("\nLegend: 👁 = read-only, 📝 = read-write")

	return nil
}
