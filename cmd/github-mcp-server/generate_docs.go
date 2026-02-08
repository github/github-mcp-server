package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
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
	Short: "Generate documentation for tools and toolsets",
	Long:  `Generate the automated sections of README.md and docs/remote-server.md with current tool and toolset information.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return generateAllDocs()
	},
}

func init() {
	rootCmd.AddCommand(generateDocsCmd)
}

func generateAllDocs() error {
	for _, doc := range []struct {
		path string
		fn   func(string) error
	}{
		// File to edit, function to generate its docs
		{"README.md", generateReadmeDocs},
		{"docs/remote-server.md", generateRemoteServerDocs},
		{"docs/tool-renaming.md", generateDeprecatedAliasesDocs},
	} {
		if err := doc.fn(doc.path); err != nil {
			return fmt.Errorf("failed to generate docs for %s: %w", doc.path, err)
		}
		fmt.Printf("Successfully updated %s with automated documentation\n", doc.path)
	}
	return nil
}

func generateReadmeDocs(readmePath string) error {
	// Create translation helper
	t, _ := translations.TranslationHelper()

	// (not available to regular users) while including tools with FeatureFlagDisable.
	// Build() can only fail if WithTools specifies invalid tools - not used here
	r, _ := github.NewInventory(t).WithToolsets([]string{"all"}).Build()

	// Generate toolsets documentation
	toolsetsDoc := generateToolsetsDoc(r)

	// Generate tools documentation
	toolsDoc := generateToolsDoc(r)

	// Read the current README.md
	// #nosec G304 - readmePath is controlled by command line flag, not user input
	content, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("failed to read README.md: %w", err)
	}

	// Replace toolsets section
	updatedContent, err := replaceSection(string(content), "START AUTOMATED TOOLSETS", "END AUTOMATED TOOLSETS", toolsetsDoc)
	if err != nil {
		return err
	}

	// Replace tools section
	updatedContent, err = replaceSection(updatedContent, "START AUTOMATED TOOLS", "END AUTOMATED TOOLS", toolsDoc)
	if err != nil {
		return err
	}

	// Write back to file
	err = os.WriteFile(readmePath, []byte(updatedContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write README.md: %w", err)
	}

	return nil
}

func generateRemoteServerDocs(docsPath string) error {
	content, err := os.ReadFile(docsPath) //#nosec G304
	if err != nil {
		return fmt.Errorf("failed to read docs file: %w", err)
	}

	toolsetsDoc := generateRemoteToolsetsDoc()

	// Replace content between markers
	updatedContent, err := replaceSection(string(content), "START AUTOMATED TOOLSETS", "END AUTOMATED TOOLSETS", toolsetsDoc)
	if err != nil {
		return err
	}

	// Also generate remote-only toolsets section
	remoteOnlyDoc := generateRemoteOnlyToolsetsDoc()
	updatedContent, err = replaceSection(updatedContent, "START AUTOMATED REMOTE TOOLSETS", "END AUTOMATED REMOTE TOOLSETS", remoteOnlyDoc)
	if err != nil {
		return err
	}

	return os.WriteFile(docsPath, []byte(updatedContent), 0600) //#nosec G306
}

// octiconImg returns an img tag for an Octicon that works with GitHub's light/dark theme.
// Uses picture element with prefers-color-scheme for automatic theme switching.
// References icons from the repo's pkg/octicons/icons directory.
// Optional pathPrefix for files in subdirectories (e.g., "../" for docs/).
func octiconImg(name string, pathPrefix ...string) string {
	if name == "" {
		return ""
	}
	prefix := ""
	if len(pathPrefix) > 0 {
		prefix = pathPrefix[0]
	}
	// Use picture element with media queries for light/dark mode support
	// GitHub renders these correctly in markdown
	lightIcon := fmt.Sprintf("%spkg/octicons/icons/%s-light.png", prefix, name)
	darkIcon := fmt.Sprintf("%spkg/octicons/icons/%s-dark.png", prefix, name)
	return fmt.Sprintf(`<picture><source media="(prefers-color-scheme: dark)" srcset="%s"><source media="(prefers-color-scheme: light)" srcset="%s"><img src="%s" width="20" height="20" alt="%s"></picture>`, darkIcon, lightIcon, lightIcon, name)
}

func generateToolsetsDoc(i *inventory.Inventory) string {
	var buf strings.Builder

	// Add table header and separator (with icon column)
	buf.WriteString("|     | Toolset                 | Description                                                   |\n")
	buf.WriteString("| --- | ----------------------- | ------------------------------------------------------------- |\n")

	// Add the context toolset row with custom description (strongly recommended)
	// Get context toolset for its icon
	contextIcon := octiconImg("person")
	fmt.Fprintf(&buf, "| %s | `context`               | **Strongly recommended**: Tools that provide context about the current user and GitHub context you are operating in |\n", contextIcon)

	// AvailableToolsets() returns toolsets that have tools, sorted by ID
	// Exclude context (custom description above) and dynamic (internal only)
	for _, ts := range i.AvailableToolsets("context", "dynamic") {
		icon := octiconImg(ts.Icon)
		fmt.Fprintf(&buf, "| %s | `%s` | %s |\n", icon, ts.ID, ts.Description)
	}

	return strings.TrimSuffix(buf.String(), "\n")
}

func generateToolsDoc(r *inventory.Inventory) string {
	tools := r.AvailableTools(context.Background())
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
		// When toolset changes, emit the previous section
		if tool.Toolset.ID != currentToolsetID {
			writeSection()
			currentToolsetID = tool.Toolset.ID
			currentToolsetIcon = tool.Toolset.Icon
		}
		writeToolDoc(&toolBuf, tool)
		toolBuf.WriteString("\n\n")
	}

	// Emit the last section
	writeSection()

	return buf.String()
}

func writeToolDoc(buf *strings.Builder, tool inventory.ServerTool) {
	// Tool name (no icon - section header already has the toolset icon)
	fmt.Fprintf(buf, "- **%s** - %s\n", tool.Tool.Name, tool.Tool.Annotations.Title)

	// OAuth scopes if present
	if len(tool.RequiredScopes) > 0 {
		fmt.Fprintf(buf, "  - **Required OAuth Scopes**: `%s`\n", strings.Join(tool.RequiredScopes, "`, `"))

		// Only show accepted scopes if they differ from required scopes
		if len(tool.AcceptedScopes) > 0 && !scopesEqual(tool.RequiredScopes, tool.AcceptedScopes) {
			fmt.Fprintf(buf, "  - **Accepted OAuth Scopes**: `%s`\n", strings.Join(tool.AcceptedScopes, "`, `"))
		}
	}

	// Parameters
	if tool.Tool.InputSchema == nil {
		buf.WriteString("  - No parameters required")
		return
	}
	schema, ok := tool.Tool.InputSchema.(*jsonschema.Schema)
	if !ok || schema == nil {
		buf.WriteString("  - No parameters required")
		return
	}

	if len(schema.Properties) > 0 {
		// Get parameter names and sort them for deterministic order
		var paramNames []string
		for propName := range schema.Properties {
			paramNames = append(paramNames, propName)
		}
		sort.Strings(paramNames)

		for i, propName := range paramNames {
			prop := schema.Properties[propName]
			required := contains(schema.Required, propName)
			requiredStr := "optional"
			if required {
				requiredStr = "required"
			}

			var typeStr string

			// Get the type and description
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

			// Indent any continuation lines in the description to maintain markdown formatting
			description := indentMultilineDescription(prop.Description, "    ")

			fmt.Fprintf(buf, "  - `%s`: %s (%s, %s)", propName, description, typeStr, requiredStr)
			if i < len(paramNames)-1 {
				buf.WriteString("\n")
			}
		}
	} else {
		buf.WriteString("  - No parameters required")
	}
}

// scopesEqual checks if two scope slices contain the same elements (order-independent)
func scopesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create a map for quick lookup
	aMap := make(map[string]bool, len(a))
	for _, scope := range a {
		aMap[scope] = true
	}

	// Check if all elements in b are in a
	for _, scope := range b {
		if !aMap[scope] {
			return false
		}
	}

	return true
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// indentMultilineDescription adds the specified indent to all lines after the first line.
// This ensures that multi-line descriptions maintain proper markdown list formatting.
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

	startIdx := strings.Index(content, start)
	endIdx := strings.Index(content, end)
	if startIdx == -1 || endIdx == -1 {
		return "", fmt.Errorf("markers not found: %s / %s", start, end)
	}

	var buf strings.Builder
	buf.WriteString(content[:startIdx])
	buf.WriteString(start)
	buf.WriteString("\n")
	buf.WriteString(newContent)
	buf.WriteString("\n")
	buf.WriteString(content[endIdx:])
	return buf.String(), nil
}

func generateRemoteToolsetsDoc() string {
	var buf strings.Builder

	// Create translation helper
	t, _ := translations.TranslationHelper()

	// Build inventory - stateless
	// Build() can only fail if WithTools specifies invalid tools - not used here
	r, _ := github.NewInventory(t).Build()

	// Generate table header (icon is combined with Name column)
	buf.WriteString("| Name | Description | API URL | 1-Click Install (VS Code) | Read-only Link | 1-Click Read-only Install (VS Code) |\n")
	buf.WriteString("| ---- | ----------- | ------- | ------------------------- | -------------- | ----------------------------------- |\n")

	// Add "all" toolset first (special case)
	allIcon := octiconImg("apps", "../")
	fmt.Fprintf(&buf, "| %s<br>`all` | All available GitHub MCP tools | https://api.githubcopilot.com/mcp/ | [Install](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%%7B%%22type%%22%%3A%%20%%22http%%22%%2C%%22url%%22%%3A%%20%%22https%%3A%%2F%%2Fapi.githubcopilot.com%%2Fmcp%%2F%%22%%7D) | [read-only](https://api.githubcopilot.com/mcp/readonly) | [Install read-only](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%%7B%%22type%%22%%3A%%20%%22http%%22%%2C%%22url%%22%%3A%%20%%22https%%3A%%2F%%2Fapi.githubcopilot.com%%2Fmcp%%2Freadonly%%22%%7D) |\n", allIcon)

	// AvailableToolsets() returns toolsets that have tools, sorted by ID
	// Exclude context (handled separately) and dynamic (internal only)
	for _, ts := range r.AvailableToolsets("context", "dynamic") {
		idStr := string(ts.ID)

		apiURL := fmt.Sprintf("https://api.githubcopilot.com/mcp/x/%s", idStr)
		readonlyURL := fmt.Sprintf("https://api.githubcopilot.com/mcp/x/%s/readonly", idStr)

		// Create install config JSON (URL encoded)
		installConfig := url.QueryEscape(fmt.Sprintf(`{"type": "http","url": "%s"}`, apiURL))
		readonlyConfig := url.QueryEscape(fmt.Sprintf(`{"type": "http","url": "%s"}`, readonlyURL))

		// Fix URL encoding to use %20 instead of + for spaces
		installConfig = strings.ReplaceAll(installConfig, "+", "%20")
		readonlyConfig = strings.ReplaceAll(readonlyConfig, "+", "%20")

		installLink := fmt.Sprintf("[Install](https://insiders.vscode.dev/redirect/mcp/install?name=gh-%s&config=%s)", idStr, installConfig)
		readonlyInstallLink := fmt.Sprintf("[Install read-only](https://insiders.vscode.dev/redirect/mcp/install?name=gh-%s&config=%s)", idStr, readonlyConfig)

		icon := octiconImg(ts.Icon, "../")
		fmt.Fprintf(&buf, "| %s<br>`%s` | %s | %s | %s | [read-only](%s) | %s |\n",
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

	// Generate table header (icon is combined with Name column)
	buf.WriteString("| Name | Description | API URL | 1-Click Install (VS Code) | Read-only Link | 1-Click Read-only Install (VS Code) |\n")
	buf.WriteString("| ---- | ----------- | ------- | ------------------------- | -------------- | ----------------------------------- |\n")

	// Use RemoteOnlyToolsets from github package
	for _, ts := range github.RemoteOnlyToolsets() {
		idStr := string(ts.ID)

		apiURL := fmt.Sprintf("https://api.githubcopilot.com/mcp/x/%s", idStr)
		readonlyURL := fmt.Sprintf("https://api.githubcopilot.com/mcp/x/%s/readonly", idStr)

		// Create install config JSON (URL encoded)
		installConfig := url.QueryEscape(fmt.Sprintf(`{"type": "http","url": "%s"}`, apiURL))
		readonlyConfig := url.QueryEscape(fmt.Sprintf(`{"type": "http","url": "%s"}`, readonlyURL))

		// Fix URL encoding to use %20 instead of + for spaces
		installConfig = strings.ReplaceAll(installConfig, "+", "%20")
		readonlyConfig = strings.ReplaceAll(readonlyConfig, "+", "%20")

		installLink := fmt.Sprintf("[Install](https://insiders.vscode.dev/redirect/mcp/install?name=gh-%s&config=%s)", idStr, installConfig)
		readonlyInstallLink := fmt.Sprintf("[Install read-only](https://insiders.vscode.dev/redirect/mcp/install?name=gh-%s&config=%s)", idStr, readonlyConfig)

		icon := octiconImg(ts.Icon, "../")
		fmt.Fprintf(&buf, "| %s<br>`%s` | %s | %s | %s | [read-only](%s) | %s |\n",
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
	// Read the current file
	content, err := os.ReadFile(docsPath) //#nosec G304
	if err != nil {
		return fmt.Errorf("failed to read docs file: %w", err)
	}

	// Generate the table
	aliasesDoc := generateDeprecatedAliasesTable()

	// Replace content between markers
	updatedContent, err := replaceSection(string(content), "START AUTOMATED ALIASES", "END AUTOMATED ALIASES", aliasesDoc)
	if err != nil {
		return err
	}

	// Write back to file
	err = os.WriteFile(docsPath, []byte(updatedContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write deprecated aliases docs: %w", err)
	}

	return nil
}

func generateDeprecatedAliasesTable() string {
	var buf strings.Builder

	// Add table header
	buf.WriteString("| Old Name | New Name |\n")
	buf.WriteString("|----------|----------|\n")

	aliases := github.DeprecatedToolAliases
	if len(aliases) == 0 {
		buf.WriteString("| *(none currently)* | |")
	} else {
		// Sort keys for deterministic output
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
