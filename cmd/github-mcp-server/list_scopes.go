package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v79/github"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ToolScopeInfo contains scope information for a single tool.
type ToolScopeInfo struct {
	Name           string   `json:"name"`
	Toolset        string   `json:"toolset"`
	ReadOnly       bool     `json:"read_only"`
	RequiredScopes []string `json:"required_scopes"`
	AcceptedScopes []string `json:"accepted_scopes,omitempty"`
}

// ScopesOutput is the full output structure for the list-scopes command.
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

This command creates a toolset group based on the same flags as the stdio command
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

// mockScopesGetClient returns a mock GitHub client for scope listing.
func mockScopesGetClient(_ context.Context) (*gogithub.Client, error) {
	return gogithub.NewClient(nil), nil
}

// mockScopesGetGQLClient returns a mock GraphQL client for scope listing.
func mockScopesGetGQLClient(_ context.Context) (*githubv4.Client, error) {
	return githubv4.NewClient(nil), nil
}

// mockScopesGetRawClient returns a mock raw client for scope listing.
func mockScopesGetRawClient(_ context.Context) (*raw.Client, error) {
	return nil, nil
}

func runListScopes() error {
	// Get toolsets configuration (same logic as stdio command)
	var enabledToolsets []string
	if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
		return fmt.Errorf("failed to unmarshal toolsets: %w", err)
	}

	// No passed toolsets configuration means we enable the default toolset
	if len(enabledToolsets) == 0 {
		enabledToolsets = []string{github.ToolsetMetadataDefault.ID}
	}

	readOnly := viper.GetBool("read-only")
	outputFormat := viper.GetString("list-scopes-output")

	// Create translation helper
	t, _ := translations.TranslationHelper()

	// Create toolset group with mock clients (no actual API calls needed)
	repoAccessCache := lockdown.GetInstance(nil)
	tsg := github.DefaultToolsetGroup(readOnly, mockScopesGetClient, mockScopesGetGQLClient, mockScopesGetRawClient, t, 5000, github.FeatureFlags{}, repoAccessCache)

	// Process enabled toolsets (same logic as server.go)
	// If "all" is present, override all other toolsets
	if github.ContainsToolset(enabledToolsets, github.ToolsetMetadataAll.ID) {
		enabledToolsets = []string{github.ToolsetMetadataAll.ID}
	}
	// If "default" is present, expand to real toolset IDs
	if github.ContainsToolset(enabledToolsets, github.ToolsetMetadataDefault.ID) {
		enabledToolsets = github.AddDefaultToolset(enabledToolsets)
	}

	// Enable the requested toolsets
	err := tsg.EnableToolsets(enabledToolsets, nil)
	if err != nil {
		return fmt.Errorf("failed to enable toolsets: %w", err)
	}

	// Collect all tools and their scopes
	output := collectToolScopes(tsg, enabledToolsets, readOnly)

	// Output based on format
	switch outputFormat {
	case "json":
		return outputJSON(output)
	case "summary":
		return outputSummary(output)
	default:
		return outputText(output)
	}
}

func collectToolScopes(tsg *toolsets.ToolsetGroup, enabledToolsets []string, readOnly bool) ScopesOutput {
	var tools []ToolScopeInfo
	scopeSet := make(map[string]bool)
	scopesByTool := make(map[string][]string)
	toolsByScope := make(map[string][]string)

	// Get all toolset names and sort them for consistent output
	var toolsetNames []string
	for name := range tsg.Toolsets {
		if name != "dynamic" { // Skip dynamic toolset
			toolsetNames = append(toolsetNames, name)
		}
	}
	sort.Strings(toolsetNames)

	for _, toolsetName := range toolsetNames {
		toolset := tsg.Toolsets[toolsetName]
		if !toolset.Enabled {
			continue
		}

		// Get active tools (respects read-only setting)
		activeTools := toolset.GetActiveTools()

		for _, serverTool := range activeTools {
			tool := serverTool.Tool

			// Extract scopes from tool metadata
			requiredScopes := scopes.GetScopesFromMeta(tool.Meta)
			requiredScopeStrs := scopes.ScopeStrings(requiredScopes)

			// Calculate accepted scopes (scopes that also satisfy the requirement due to hierarchy)
			acceptedScopeStrs := []string{}
			for _, reqScope := range requiredScopes {
				accepted := scopes.GetAcceptedScopes(reqScope)
				for _, accScope := range accepted {
					if accScope != reqScope { // Don't duplicate the required scope
						accStr := accScope.String()
						// Avoid duplicates
						found := false
						for _, existing := range acceptedScopeStrs {
							if existing == accStr {
								found = true
								break
							}
						}
						if !found {
							acceptedScopeStrs = append(acceptedScopeStrs, accStr)
						}
					}
				}
			}
			sort.Strings(acceptedScopeStrs)

			// Determine if tool is read-only
			isReadOnly := tool.Annotations != nil && tool.Annotations.ReadOnlyHint

			toolInfo := ToolScopeInfo{
				Name:           tool.Name,
				Toolset:        toolsetName,
				ReadOnly:       isReadOnly,
				RequiredScopes: requiredScopeStrs,
				AcceptedScopes: acceptedScopeStrs,
			}
			tools = append(tools, toolInfo)

			// Track unique scopes
			for _, s := range requiredScopeStrs {
				scopeSet[s] = true
				toolsByScope[s] = append(toolsByScope[s], tool.Name)
			}

			// Track scopes by tool
			scopesByTool[tool.Name] = requiredScopeStrs
		}
	}

	// Sort tools by name
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	// Get unique scopes as sorted slice
	var uniqueScopes []string
	for s := range scopeSet {
		uniqueScopes = append(uniqueScopes, s)
	}
	sort.Strings(uniqueScopes)

	// Sort tools within each scope
	for scope := range toolsByScope {
		sort.Strings(toolsByScope[scope])
	}

	return ScopesOutput{
		Tools:           tools,
		UniqueScopes:    uniqueScopes,
		ScopesByTool:    scopesByTool,
		ToolsByScope:    toolsByScope,
		EnabledToolsets: enabledToolsets,
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
		if scope == "" {
			fmt.Println("  (no scope required for public read access)")
		} else {
			fmt.Printf("  %s\n", scope)
		}
	}
	fmt.Printf("\nTotal: %d unique scope(s)\n", len(output.UniqueScopes))
	return nil
}

func outputText(output ScopesOutput) error {
	fmt.Printf("OAuth Scopes for Enabled Tools\n")
	fmt.Printf("==============================\n\n")

	fmt.Printf("Enabled Toolsets: %s\n", strings.Join(output.EnabledToolsets, ", "))
	fmt.Printf("Read-Only Mode: %v\n\n", output.ReadOnly)

	// Group tools by toolset
	toolsByToolset := make(map[string][]ToolScopeInfo)
	for _, tool := range output.Tools {
		toolsByToolset[tool.Toolset] = append(toolsByToolset[tool.Toolset], tool)
	}

	// Get sorted toolset names
	var toolsetNames []string
	for name := range toolsByToolset {
		toolsetNames = append(toolsetNames, name)
	}
	sort.Strings(toolsetNames)

	for _, toolsetName := range toolsetNames {
		tools := toolsByToolset[toolsetName]
		fmt.Printf("## %s\n\n", formatToolsetNameForOutput(toolsetName))

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

	// Summary
	fmt.Println("## Summary")
	fmt.Println()
	if len(output.UniqueScopes) == 0 {
		fmt.Println("No OAuth scopes required for enabled tools.")
	} else {
		fmt.Println("Unique scopes required:")
		for _, scope := range output.UniqueScopes {
			if scope == "" {
				fmt.Println("  • (no scope - public read access)")
			} else {
				fmt.Printf("  • %s\n", scope)
			}
		}
	}
	fmt.Printf("\nTotal: %d tools, %d unique scopes\n", len(output.Tools), len(output.UniqueScopes))

	// Legend
	fmt.Println("\nLegend: 👁 = read-only, 📝 = read-write")

	return nil
}

func formatToolsetNameForOutput(name string) string {
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
		// Capitalize first letter and replace underscores with spaces
		parts := strings.Split(name, "_")
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(string(part[0])) + part[1:]
			}
		}
		return strings.Join(parts, " ")
	}
}
