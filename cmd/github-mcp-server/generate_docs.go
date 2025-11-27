package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/shurcooL/githubv4"
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

// mockGetClient returns a mock GitHub client for documentation generation
func mockGetClient(_ context.Context) (*gogithub.Client, error) {
	return gogithub.NewClient(nil), nil
}

// mockGetGQLClient returns a mock GraphQL client for documentation generation
func mockGetGQLClient(_ context.Context) (*githubv4.Client, error) {
	return githubv4.NewClient(nil), nil
}

// mockGetRawClient returns a mock raw client for documentation generation
func mockGetRawClient(_ context.Context) (*raw.Client, error) {
	return nil, nil
}

func generateAllDocs() error {
	if err := generateReadmeDocs("README.md"); err != nil {
		return fmt.Errorf("failed to generate README docs: %w", err)
	}

	if err := generateRemoteServerDocs("docs/remote-server.md"); err != nil {
		return fmt.Errorf("failed to generate remote-server docs: %w", err)
	}

	return nil
}

func generateReadmeDocs(readmePath string) error {
	// Create translation helper
	t, _ := translations.TranslationHelper()

	// Create toolset group with mock clients
	repoAccessCache := lockdown.GetInstance(nil)
	tsg := github.DefaultToolsetGroup(false, mockGetClient, mockGetGQLClient, mockGetRawClient, t, 5000, github.FeatureFlags{}, repoAccessCache)

	// Generate toolsets documentation
	toolsetsDoc := generateToolsetsDoc(tsg)

	// Generate tools documentation
	toolsDoc := generateToolsDoc(tsg)

	// Read the current README.md
	// #nosec G304 - readmePath is controlled by command line flag, not user input
	content, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("failed to read README.md: %w", err)
	}

	// Replace toolsets section
	updatedContent := replaceSection(string(content), "START AUTOMATED TOOLSETS", "END AUTOMATED TOOLSETS", toolsetsDoc)

	// Replace tools section
	updatedContent = replaceSection(updatedContent, "START AUTOMATED TOOLS", "END AUTOMATED TOOLS", toolsDoc)

	// Write back to file
	err = os.WriteFile(readmePath, []byte(updatedContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write README.md: %w", err)
	}

	fmt.Println("Successfully updated README.md with automated documentation")
	return nil
}

func generateRemoteServerDocs(docsPath string) error {
	content, err := os.ReadFile(docsPath) //#nosec G304
	if err != nil {
		return fmt.Errorf("failed to read docs file: %w", err)
	}

	toolsetsDoc := generateRemoteToolsetsDoc()

	// Replace content between markers
	startMarker := "<!-- START AUTOMATED TOOLSETS -->"
	endMarker := "<!-- END AUTOMATED TOOLSETS -->"

	contentStr := string(content)
	startIndex := strings.Index(contentStr, startMarker)
	endIndex := strings.Index(contentStr, endMarker)

	if startIndex == -1 || endIndex == -1 {
		return fmt.Errorf("automation markers not found in %s", docsPath)
	}

	newContent := contentStr[:startIndex] + startMarker + "\n" + toolsetsDoc + "\n" + endMarker + contentStr[endIndex+len(endMarker):]

	return os.WriteFile(docsPath, []byte(newContent), 0600) //#nosec G306
}

func generateToolsetsDoc(tsg *toolsets.ToolsetGroup) string {
	var lines []string

	// Add table header and separator
	lines = append(lines, "| Toolset                 | Description                                                   |")
	lines = append(lines, "| ----------------------- | ------------------------------------------------------------- |")

	// Add the context toolset row (handled separately in README)
	lines = append(lines, "| `context`               | **Strongly recommended**: Tools that provide context about the current user and GitHub context you are operating in |")

	// Get all toolsets except context (which is handled separately above)
	var toolsetNames []string
	for name := range tsg.Toolsets {
		if name != "context" && name != "dynamic" { // Skip context and dynamic toolsets as they're handled separately
			toolsetNames = append(toolsetNames, name)
		}
	}

	// Sort toolset names for consistent output
	sort.Strings(toolsetNames)

	for _, name := range toolsetNames {
		toolset := tsg.Toolsets[name]
		lines = append(lines, fmt.Sprintf("| `%s` | %s |", name, toolset.Description))
	}

	return strings.Join(lines, "\n")
}

func generateToolsDoc(tsg *toolsets.ToolsetGroup) string {
	var sections []string

	// Get all toolset names and sort them alphabetically for deterministic order
	var toolsetNames []string
	for name := range tsg.Toolsets {
		if name != "dynamic" { // Skip dynamic toolset as it's handled separately
			toolsetNames = append(toolsetNames, name)
		}
	}
	sort.Strings(toolsetNames)

	for _, toolsetName := range toolsetNames {
		toolset := tsg.Toolsets[toolsetName]

		tools := toolset.GetAvailableTools()
		if len(tools) == 0 {
			continue
		}

		// Sort tools by name for deterministic order
		sort.Slice(tools, func(i, j int) bool {
			return tools[i].Tool.Name < tools[j].Tool.Name
		})

		// Generate section header - capitalize first letter and replace underscores
		sectionName := formatToolsetName(toolsetName)

		var toolDocs []string
		for _, serverTool := range tools {
			toolDoc := generateToolDoc(serverTool.Tool)
			toolDocs = append(toolDocs, toolDoc)
		}

		if len(toolDocs) > 0 {
			section := fmt.Sprintf("<details>\n\n<summary>%s</summary>\n\n%s\n\n</details>",
				sectionName, strings.Join(toolDocs, "\n\n"))
			sections = append(sections, section)
		}
	}

	return strings.Join(sections, "\n\n")
}

func formatToolsetName(name string) string {
	switch name {
	case "pull_requests":
		return "Pull Requests"
	case "repos":
		return "Repositories"
	case "code_security":
		return "Code Security"
	case "secret_protection":
		return "Secret Protection"
	case "orgs":
		return "Organizations"
	default:
		// Fallback: capitalize first letter and replace underscores with spaces
		parts := strings.Split(name, "_")
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(string(part[0])) + part[1:]
			}
		}
		return strings.Join(parts, " ")
	}
}

func generateToolDoc(tool mcp.Tool) string {
	var lines []string

	// Tool name only (using annotation name instead of verbose description)
	lines = append(lines, fmt.Sprintf("- **%s** - %s", tool.Name, tool.Annotations.Title))

	// Parameters
	schema := tool.InputSchema
	if len(schema.Properties) > 0 {
		// Get parameter names and sort them for deterministic order
		var paramNames []string
		for propName := range schema.Properties {
			paramNames = append(paramNames, propName)
		}
		sort.Strings(paramNames)

		for _, propName := range paramNames {
			prop := schema.Properties[propName]
			required := contains(schema.Required, propName)
			requiredStr := "optional"
			if required {
				requiredStr = "required"
			}

			// Get the type and description
			typeStr := "unknown"
			description := ""

			if propMap, ok := prop.(map[string]interface{}); ok {
				if typeVal, ok := propMap["type"].(string); ok {
					if typeVal == "array" {
						if items, ok := propMap["items"].(map[string]interface{}); ok {
							if itemType, ok := items["type"].(string); ok {
								typeStr = itemType + "[]"
							}
						} else {
							typeStr = "array"
						}
					} else {
						typeStr = typeVal
					}
				}

				if desc, ok := propMap["description"].(string); ok {
					description = desc
				}
			}

			paramLine := fmt.Sprintf("  - `%s`: %s (%s, %s)", propName, description, typeStr, requiredStr)
			lines = append(lines, paramLine)
		}
	} else {
		lines = append(lines, "  - No parameters required")
	}

	return strings.Join(lines, "\n")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func replaceSection(content, startMarker, endMarker, newContent string) string {
	startPattern := fmt.Sprintf(`<!-- %s -->`, regexp.QuoteMeta(startMarker))
	endPattern := fmt.Sprintf(`<!-- %s -->`, regexp.QuoteMeta(endMarker))

	re := regexp.MustCompile(fmt.Sprintf(`(?s)%s.*?%s`, startPattern, endPattern))

	replacement := fmt.Sprintf("<!-- %s -->\n%s\n<!-- %s -->", startMarker, newContent, endMarker)

	return re.ReplaceAllString(content, replacement)
}

func generateRemoteToolsetsDoc() string {
	var buf strings.Builder

	// Create translation helper
	t, _ := translations.TranslationHelper()

	// Create toolset group with mock clients
	repoAccessCache := lockdown.GetInstance(nil)
	tsg := github.DefaultToolsetGroup(false, mockGetClient, mockGetGQLClient, mockGetRawClient, t, 5000, github.FeatureFlags{}, repoAccessCache)

	// Generate table header
	buf.WriteString("| Name           | Description                                      | API URL                                               | 1-Click Install (VS Code)                                                                                                                                                                                                 | Read-only Link                                                                                                 | 1-Click Read-only Install (VS Code)                                                                                                                                                                                                 |\n")
	buf.WriteString("|----------------|--------------------------------------------------|-------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|\n")

	// Get all toolsets
	toolsetNames := make([]string, 0, len(tsg.Toolsets))
	for name := range tsg.Toolsets {
		if name != "context" && name != "dynamic" { // Skip context and dynamic toolsets as they're handled separately
			toolsetNames = append(toolsetNames, name)
		}
	}
	sort.Strings(toolsetNames)

	// Add "all" toolset first (special case)
	buf.WriteString("| all            | All available GitHub MCP tools                    | https://api.githubcopilot.com/mcp/                    | [Install](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D)                                      | [read-only](https://api.githubcopilot.com/mcp/readonly)                                                      | [Install read-only](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2Freadonly%22%7D) |\n")

	// Add individual toolsets
	for _, name := range toolsetNames {
		toolset := tsg.Toolsets[name]

		formattedName := formatToolsetName(name)
		description := toolset.Description
		apiURL := fmt.Sprintf("https://api.githubcopilot.com/mcp/x/%s", name)
		readonlyURL := fmt.Sprintf("https://api.githubcopilot.com/mcp/x/%s/readonly", name)

		// Create install config JSON (URL encoded)
		installConfig := url.QueryEscape(fmt.Sprintf(`{"type": "http","url": "%s"}`, apiURL))
		readonlyConfig := url.QueryEscape(fmt.Sprintf(`{"type": "http","url": "%s"}`, readonlyURL))

		// Fix URL encoding to use %20 instead of + for spaces
		installConfig = strings.ReplaceAll(installConfig, "+", "%20")
		readonlyConfig = strings.ReplaceAll(readonlyConfig, "+", "%20")

		installLink := fmt.Sprintf("[Install](https://insiders.vscode.dev/redirect/mcp/install?name=gh-%s&config=%s)", name, installConfig)
		readonlyInstallLink := fmt.Sprintf("[Install read-only](https://insiders.vscode.dev/redirect/mcp/install?name=gh-%s&config=%s)", name, readonlyConfig)

		buf.WriteString(fmt.Sprintf("| %-14s | %-48s | %-53s | %-218s | %-110s | %-288s |\n",
			formattedName,
			description,
			apiURL,
			installLink,
			fmt.Sprintf("[read-only](%s)", readonlyURL),
			readonlyInstallLink,
		))
	}

	return buf.String()
}
