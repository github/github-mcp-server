package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/spf13/cobra"
)

var generateDocsCmd = &cobra.Command{
	Use:   "generate-docs",
	Short: "生成工具和工具集文档",
	Long:  `使用当前工具和工具集信息生成 README.md 与 docs/remote-server.md 中的自动化章节。`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return generateAllDocs()
	},
}

func init() {
	rootCmd.AddCommand(generateDocsCmd)
}

// noFeatureFlagsChecker 报告所有 feature flag 均为禁用状态。
// 它模拟生成文档所使用的默认用户体验。
func noFeatureFlagsChecker(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func generateAllDocs() error {
	for _, doc := range []struct {
		path string
		fn   func(string) error
	}{
		// 要编辑的文件，以及用于生成其文档的函数。
		{"README.md", generateReadmeDocs},
		{"docs/remote-server.md", generateRemoteServerDocs},
		{"docs/insiders-features.md", generateInsidersFeaturesDocs},
		{"docs/feature-flags.md", generateFeatureFlagsDocs},
		{"docs/tool-renaming.md", generateDeprecatedAliasesDocs},
	} {
		if err := doc.fn(doc.path); err != nil {
			return fmt.Errorf("为 %s 生成文档失败: %w", doc.path, err)
		}
		fmt.Printf("已使用自动化文档更新 %s\n", doc.path)
	}
	return nil
}

func generateReadmeDocs(readmePath string) error {
	// 创建翻译 helper。
	t, _ := translations.TranslationHelper()

	// README 记录默认用户体验：未设置特殊 flag 时启用的工具。安装一个报告所有 flag
	// 均禁用的 checker，会排除由 FeatureFlagEnable 控制的工具，并保留由
	// FeatureFlagDisable 控制的旧变体，从而避免 flag 控制的重复项出现两次。
	// Build() 只有在 WithTools 指定无效工具时才会失败，此处未使用该配置。
	r, _ := github.NewInventory(t).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(noFeatureFlagsChecker).
		Build()

	// 生成工具集文档。
	toolsetsDoc := generateToolsetsDoc(r)

	// 生成工具文档。
	toolsDoc := generateToolsDoc(r)

	// 读取当前 README.md。
	// #nosec G304 - readmePath is controlled by command line flag, not user input
	content, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("读取 README.md 失败: %w", err)
	}

	// 替换工具集章节。
	updatedContent, err := replaceSection(string(content), "START AUTOMATED TOOLSETS", "END AUTOMATED TOOLSETS", toolsetsDoc)
	if err != nil {
		return err
	}

	// 替换工具章节。
	updatedContent, err = replaceSection(updatedContent, "START AUTOMATED TOOLS", "END AUTOMATED TOOLS", toolsDoc)
	if err != nil {
		return err
	}

	// 写回文件。
	err = os.WriteFile(readmePath, []byte(updatedContent), 0600)
	if err != nil {
		return fmt.Errorf("写入 README.md 失败: %w", err)
	}

	return nil
}

func generateRemoteServerDocs(docsPath string) error {
	content, err := os.ReadFile(docsPath) //#nosec G304
	if err != nil {
		return fmt.Errorf("读取 docs 文件失败: %w", err)
	}

	toolsetsDoc := generateRemoteToolsetsDoc()

	// 替换标记之间的内容。
	updatedContent, err := replaceSection(string(content), "START AUTOMATED TOOLSETS", "END AUTOMATED TOOLSETS", toolsetsDoc)
	if err != nil {
		return err
	}

	// 同时生成仅远程可用的工具集章节。
	remoteOnlyDoc := generateRemoteOnlyToolsetsDoc()
	updatedContent, err = replaceSection(updatedContent, "START AUTOMATED REMOTE TOOLSETS", "END AUTOMATED REMOTE TOOLSETS", remoteOnlyDoc)
	if err != nil {
		return err
	}

	return os.WriteFile(docsPath, []byte(updatedContent), 0600) //#nosec G306
}

// octiconImg 返回适配 GitHub 浅色/深色主题的 Octicon img 标签。
// 使用包含 prefers-color-scheme 的 picture 元素自动切换主题。
// 图标引用仓库中 pkg/octicons/icons 目录下的文件。
// pathPrefix 可选，用于子目录中的文件（例如 docs/ 使用 "../"）。
func octiconImg(name string, pathPrefix ...string) string {
	if name == "" {
		return ""
	}
	prefix := ""
	if len(pathPrefix) > 0 {
		prefix = pathPrefix[0]
	}
	// 使用带 media query 的 picture 元素支持浅色/深色模式。
	// GitHub 能在 markdown 中正确渲染这些元素。
	lightIcon := fmt.Sprintf("%spkg/octicons/icons/%s-light.png", prefix, name)
	darkIcon := fmt.Sprintf("%spkg/octicons/icons/%s-dark.png", prefix, name)
	return fmt.Sprintf(`<picture><source media="(prefers-color-scheme: dark)" srcset="%s"><source media="(prefers-color-scheme: light)" srcset="%s"><img src="%s" width="20" height="20" alt="%s"></picture>`, darkIcon, lightIcon, lightIcon, name)
}

func generateToolsetsDoc(i *inventory.Inventory) string {
	var buf strings.Builder

	// 添加表头和分隔行（含图标列）。
	buf.WriteString("|     | 工具集                  | 描述                                                          |\n")
	buf.WriteString("| --- | ----------------------- | ------------------------------------------------------------- |\n")

	// 添加带自定义描述的 context 工具集行（强烈推荐）。
	// 获取 context 工具集的图标。
	contextIcon := octiconImg("person")
	fmt.Fprintf(&buf, "| %s | `context`               | **强烈推荐**：提供当前用户以及正在操作的 GitHub 上下文的工具 |\n", contextIcon)

	// AvailableToolsets() 返回拥有工具的工具集，并按 ID 排序。
	// 排除 context（上方已有自定义描述）。
	for _, ts := range i.AvailableToolsets("context") {
		icon := octiconImg(ts.Icon)
		fmt.Fprintf(&buf, "| %s | `%s` | %s |\n", icon, ts.ID, ts.Description)
	}

	return strings.TrimSuffix(buf.String(), "\n")
}

func generateToolsDoc(r *inventory.Inventory) string {
	tools := r.ToolsForRegistration(context.Background())
	if len(tools) == 0 {
		return ""
	}

	var buf strings.Builder
	var toolBuf strings.Builder
	var currentToolsetID inventory.ToolsetID
	var currentToolsetIcon string
	firstSection := true

	writeSection := func() {
		if toolBuf.Len() == 0 {
			return
		}
		if !firstSection {
			buf.WriteString("\n\n")
		}
		firstSection = false
		sectionName := formatToolsetName(string(currentToolsetID))
		icon := octiconImg(currentToolsetIcon)
		if icon != "" {
			icon += " "
		}
		fmt.Fprintf(&buf, "<details>\n\n<summary>%s%s</summary>\n\n%s\n\n</details>", icon, sectionName, strings.TrimSuffix(toolBuf.String(), "\n\n"))
		toolBuf.Reset()
	}

	for _, tool := range tools {
		// 工具集发生变化时，输出前一个章节。
		if tool.Toolset.ID != currentToolsetID {
			writeSection()
			currentToolsetID = tool.Toolset.ID
			currentToolsetIcon = tool.Toolset.Icon
		}
		writeToolDoc(&toolBuf, tool)
		toolBuf.WriteString("\n\n")
	}

	// 输出最后一个章节。
	writeSection()

	return buf.String()
}

func writeToolDoc(buf *strings.Builder, tool inventory.ServerTool) {
	// 工具名称（不带图标；章节标题已有工具集图标）。
	fmt.Fprintf(buf, "- **%s** - %s\n", tool.Tool.Name, tool.Tool.Annotations.Title)

	// 如果存在 OAuth scopes，则输出它们。
	if len(tool.RequiredScopes) > 0 {
		// Scope 过滤使用“任一满足”语义（见 scopes.HasRequiredScopes），
		// 因此列出多个 required scopes 时，应渲染为可选项，而不是暗示必须全部具备。
		scopeList := "`" + strings.Join(tool.RequiredScopes, "`, `") + "`"
		if len(tool.RequiredScopes) > 1 {
			fmt.Fprintf(buf, "  - **所需 OAuth Scopes（任一）**：%s\n", scopeList)
		} else {
			fmt.Fprintf(buf, "  - **所需 OAuth Scopes**：%s\n", scopeList)
		}

		// 仅当 accepted scopes 与 required scopes 不同时显示。
		if len(tool.AcceptedScopes) > 0 && !scopesEqual(tool.RequiredScopes, tool.AcceptedScopes) {
			fmt.Fprintf(buf, "  - **可接受 OAuth Scopes**：`%s`\n", strings.Join(tool.AcceptedScopes, "`, `"))
		}
	}

	// MCP App UI metadata 仅在 remote_mcp_ui_apps flag 应用于清单时渲染；
	// 对于无 flag 的 README，该部分会在渲染前由 inventory.ToolsForRegistration 移除。
	if ui, ok := tool.Tool.Meta["ui"].(map[string]any); ok {
		if uri, ok := ui["resourceUri"].(string); ok && uri != "" {
			fmt.Fprintf(buf, "  - **MCP App UI**：`%s`\n", uri)
		}
	}

	// 参数。
	if tool.Tool.InputSchema == nil {
		buf.WriteString("  - 无需参数")
		return
	}
	schema, ok := tool.Tool.InputSchema.(*jsonschema.Schema)
	if !ok || schema == nil {
		buf.WriteString("  - 无需参数")
		return
	}

	if len(schema.Properties) > 0 {
		// 获取参数名并排序，以保证输出确定。
		var paramNames []string
		for propName := range schema.Properties {
			paramNames = append(paramNames, propName)
		}
		sort.Strings(paramNames)

		for i, propName := range paramNames {
			prop := schema.Properties[propName]
			required := slices.Contains(schema.Required, propName)
			requiredStr := "可选"
			if required {
				requiredStr = "必需"
			}

			var typeStr string

			// 获取类型和描述。
			switch prop.Type {
			case "array":
				if prop.Items != nil {
					typeStr = prop.Items.Type + "[]"
				} else {
					typeStr = "array"
				}
			default:
				typeStr = prop.Type
			}

			// 缩进描述中的续行，以保持 markdown 格式。
			description := indentMultilineDescription(prop.Description, "    ")

			fmt.Fprintf(buf, "  - `%s`: %s (%s, %s)", propName, description, typeStr, requiredStr)
			if i < len(paramNames)-1 {
				buf.WriteString("\n")
			}
		}
	} else {
		buf.WriteString("  - 无需参数")
	}
}

// scopesEqual 检查两个 scope 切片是否包含相同元素（忽略顺序）。
func scopesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// 创建用于快速查找的 map。
	aMap := make(map[string]bool, len(a))
	for _, scope := range a {
		aMap[scope] = true
	}

	// 检查 b 中所有元素是否都在 a 中。
	for _, scope := range b {
		if !aMap[scope] {
			return false
		}
	}

	return true
}

// indentMultilineDescription 将指定缩进添加到首行之后的所有行。
// 这可确保多行描述保持正确的 markdown 列表格式。
func indentMultilineDescription(description, indent string) string {
	if !strings.Contains(description, "\n") {
		return description
	}
	var buf strings.Builder
	lines := strings.Split(description, "\n")
	buf.WriteString(lines[0])
	for i := 1; i < len(lines); i++ {
		buf.WriteString("\n")
		buf.WriteString(indent)
		buf.WriteString(lines[i])
	}
	return buf.String()
}

func replaceSection(content, startMarker, endMarker, newContent string) (string, error) {
	start := fmt.Sprintf("<!-- %s -->", startMarker)
	end := fmt.Sprintf("<!-- %s -->", endMarker)

	before, _, ok := strings.Cut(content, start)
	endIdx := strings.Index(content, end)
	if !ok || endIdx == -1 {
		return "", fmt.Errorf("未找到标记: %s / %s", start, end)
	}

	var buf strings.Builder
	buf.WriteString(before)
	buf.WriteString(start)
	buf.WriteString("\n")
	buf.WriteString(newContent)
	buf.WriteString("\n")
	buf.WriteString(content[endIdx:])
	return buf.String(), nil
}

func generateRemoteToolsetsDoc() string {
	var buf strings.Builder

	// 创建翻译 helper。
	t, _ := translations.TranslationHelper()

	// 构建无状态清单。
	// Build() 只有在 WithTools 指定无效工具时才会失败，此处未使用该配置。
	r, _ := github.NewInventory(t).Build()

	// 生成表头（图标合并到名称列）。
	buf.WriteString("| 名称 | 描述 | API URL | 一键安装（VS Code） | 只读链接 | 一键只读安装（VS Code） |\n")
	buf.WriteString("| ---- | ----------- | ------- | ------------------------- | -------------- | ----------------------------------- |\n")

	// 先添加 "default" 和 "all" 元工具集（特殊情况）。
	// 基础 URL 提供默认工具集，/x/all 会一次性启用所有工具集。
	metaIcon := octiconImg("apps", "../")
	fmt.Fprintf(&buf, "| %s<br>`default` | 默认工具集 | https://api.githubcopilot.com/mcp/ | [安装](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%%7B%%22type%%22%%3A%%20%%22http%%22%%2C%%22url%%22%%3A%%20%%22https%%3A%%2F%%2Fapi.githubcopilot.com%%2Fmcp%%2F%%22%%7D) | [只读](https://api.githubcopilot.com/mcp/readonly) | [安装只读](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%%7B%%22type%%22%%3A%%20%%22http%%22%%2C%%22url%%22%%3A%%20%%22https%%3A%%2F%%2Fapi.githubcopilot.com%%2Fmcp%%2Freadonly%%22%%7D) |\n", metaIcon)
	fmt.Fprintf(&buf, "| %s<br>`all` | 所有可用的 GitHub MCP 工具 | https://api.githubcopilot.com/mcp/x/all | [安装](https://insiders.vscode.dev/redirect/mcp/install?name=gh-all&config=%%7B%%22type%%22%%3A%%20%%22http%%22%%2C%%22url%%22%%3A%%20%%22https%%3A%%2F%%2Fapi.githubcopilot.com%%2Fmcp%%2Fx%%2Fall%%22%%7D) | [只读](https://api.githubcopilot.com/mcp/x/all/readonly) | [安装只读](https://insiders.vscode.dev/redirect/mcp/install?name=gh-all&config=%%7B%%22type%%22%%3A%%20%%22http%%22%%2C%%22url%%22%%3A%%20%%22https%%3A%%2F%%2Fapi.githubcopilot.com%%2Fmcp%%2Fx%%2Fall%%2Freadonly%%22%%7D) |\n", metaIcon)

	// AvailableToolsets() 返回拥有工具的工具集，并按 ID 排序。
	// 排除 context（单独处理）。
	for _, ts := range r.AvailableToolsets("context") {
		idStr := string(ts.ID)

		apiURL := fmt.Sprintf("https://api.githubcopilot.com/mcp/x/%s", idStr)
		readonlyURL := fmt.Sprintf("https://api.githubcopilot.com/mcp/x/%s/readonly", idStr)

		// 创建安装配置 JSON（URL 编码）。
		installConfig := url.QueryEscape(fmt.Sprintf(`{"type": "http","url": "%s"}`, apiURL))
		readonlyConfig := url.QueryEscape(fmt.Sprintf(`{"type": "http","url": "%s"}`, readonlyURL))

		// 修正 URL 编码，对空格使用 %20 而不是 +。
		installConfig = strings.ReplaceAll(installConfig, "+", "%20")
		readonlyConfig = strings.ReplaceAll(readonlyConfig, "+", "%20")

		installLink := fmt.Sprintf("[安装](https://insiders.vscode.dev/redirect/mcp/install?name=gh-%s&config=%s)", idStr, installConfig)
		readonlyInstallLink := fmt.Sprintf("[安装只读](https://insiders.vscode.dev/redirect/mcp/install?name=gh-%s&config=%s)", idStr, readonlyConfig)

		icon := octiconImg(ts.Icon, "../")
		fmt.Fprintf(&buf, "| %s<br>`%s` | %s | %s | %s | [只读](%s) | %s |\n",
			icon,
			idStr,
			ts.Description,
			apiURL,
			installLink,
			readonlyURL,
			readonlyInstallLink,
		)
	}

	return strings.TrimSuffix(buf.String(), "\n")
}

func generateRemoteOnlyToolsetsDoc() string {
	var buf strings.Builder

	// 生成表头（图标合并到名称列）。
	buf.WriteString("| 名称 | 描述 | API URL | 一键安装（VS Code） | 只读链接 | 一键只读安装（VS Code） |\n")
	buf.WriteString("| ---- | ----------- | ------- | ------------------------- | -------------- | ----------------------------------- |\n")

	// 使用 github package 中的 RemoteOnlyToolsets。
	for _, ts := range github.RemoteOnlyToolsets() {
		idStr := string(ts.ID)

		apiURL := fmt.Sprintf("https://api.githubcopilot.com/mcp/x/%s", idStr)
		readonlyURL := fmt.Sprintf("https://api.githubcopilot.com/mcp/x/%s/readonly", idStr)

		// 创建安装配置 JSON（URL 编码）。
		installConfig := url.QueryEscape(fmt.Sprintf(`{"type": "http","url": "%s"}`, apiURL))
		readonlyConfig := url.QueryEscape(fmt.Sprintf(`{"type": "http","url": "%s"}`, readonlyURL))

		// 修正 URL 编码，对空格使用 %20 而不是 +。
		installConfig = strings.ReplaceAll(installConfig, "+", "%20")
		readonlyConfig = strings.ReplaceAll(readonlyConfig, "+", "%20")

		installLink := fmt.Sprintf("[安装](https://insiders.vscode.dev/redirect/mcp/install?name=gh-%s&config=%s)", idStr, installConfig)
		readonlyInstallLink := fmt.Sprintf("[安装只读](https://insiders.vscode.dev/redirect/mcp/install?name=gh-%s&config=%s)", idStr, readonlyConfig)

		icon := octiconImg(ts.Icon, "../")
		fmt.Fprintf(&buf, "| %s<br>`%s` | %s | %s | %s | [只读](%s) | %s |\n",
			icon,
			idStr,
			ts.Description,
			apiURL,
			installLink,
			readonlyURL,
			readonlyInstallLink,
		)
	}

	return strings.TrimSuffix(buf.String(), "\n")
}

func generateDeprecatedAliasesDocs(docsPath string) error {
	// 读取当前文件。
	content, err := os.ReadFile(docsPath) //#nosec G304
	if err != nil {
		return fmt.Errorf("读取 docs 文件失败: %w", err)
	}

	// 生成表格。
	aliasesDoc := generateDeprecatedAliasesTable()

	// 替换标记之间的内容。
	updatedContent, err := replaceSection(string(content), "START AUTOMATED ALIASES", "END AUTOMATED ALIASES", aliasesDoc)
	if err != nil {
		return err
	}

	// 写回文件。
	err = os.WriteFile(docsPath, []byte(updatedContent), 0600)
	if err != nil {
		return fmt.Errorf("写入弃用别名文档失败: %w", err)
	}

	return nil
}

func generateDeprecatedAliasesTable() string {
	var buf strings.Builder

	// 添加表头。
	buf.WriteString("| 旧名称 | 新名称 |\n")
	buf.WriteString("|----------|----------|\n")

	aliases := github.DeprecatedToolAliases
	if len(aliases) == 0 {
		buf.WriteString("| *（当前无）* | |")
	} else {
		// 排序 key，确保输出确定。
		var oldNames []string
		for oldName := range aliases {
			oldNames = append(oldNames, oldName)
		}
		sort.Strings(oldNames)

		for i, oldName := range oldNames {
			newName := aliases[oldName]
			fmt.Fprintf(&buf, "| `%s` | `%s` |", oldName, newName)
			if i < len(oldNames)-1 {
				buf.WriteString("\n")
			}
		}
	}

	return buf.String()
}
