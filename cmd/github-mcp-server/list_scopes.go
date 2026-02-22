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

// formatScopeDisplay formats a scope string for display, handling empty scopes.
func formatScopeDisplay(scope string) string {
	if scope == "" {
		return "(no scope required for public read access)"
	}
	return scope
}

func runListScopes() error {
	// Get toolsets configuration (same logic as stdio command)
	var enabledToolsets []string
	if viper.IsSet("toolsets") {
		if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
			return fmt.Errorf("failed to unmarshal toolsets: %w", err)
		}
	}
	// else: enabledToolsets stays nil, meaning "use defaults"

	// Get specific tools (similar to toolsets)
	var enabledTools []string
	if viper.IsSet("tools") {
		if err := viper.UnmarshalKey("tools", &enabledTools); err != nil {
			return fmt.Errorf("failed to unmarshal tools: %w", err)
		}
	}

	readOnly := viper.GetBool("read-only")
	outputFormat := viper.GetString("list-scopes-output")

	// Create translation helper
	t, _ := translations.TranslationHelper()

	// Build inventory using the same logic as the stdio server
	inventoryBuilder := github.NewInventory(t).
		WithReadOnly(readOnly)

	// Configure toolsets (same as stdio)
	if enabledToolsets != nil {
		inventoryBuilder = inventoryBuilder.WithToolsets(enabledToolsets)
	}

	// Configure specific tools
	if len(enabledTools) > 0 {
		inventoryBuilder = inventoryBuilder.WithTools(enabledTools)
	}

	inv, err := inventoryBuilder.Build()
	if err != nil {
		return fmt.Errorf("failed to build inventory: %w", err)
	}

	// Collect all tools and their scopes
	output := collectToolScopes(inv, readOnly)

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

func collectToolScopes(inv *inventory.Inventory, readOnly bool) ScopesOutput {
	var tools []ToolScopeInfo
	scopeSet := make(map[string]bool)
	scopesByTool := make(map[string][]string)
	toolsByScope := make(map[string][]string)

	// Get all available tools from the inventory
	// Use context.Background() for feature flag evaluation
	availableTools := inv.AvailableTools(context.Background())

	for _, serverTool := range availableTools {
		tool := serverTool.Tool

		// Get scope information directly from ServerTool
		requiredScopes := serverTool.RequiredScopes
		acceptedScopes := serverTool.AcceptedScopes

		// Determine if tool is read-only
		isReadOnly := serverTool.IsReadOnly()

		toolInfo := ToolScopeInfo{
			Name:           tool.Name,
			Toolset:        string(serverTool.Toolset.ID),
			ReadOnly:       isReadOnly,
			RequiredScopes: requiredScopes,
			AcceptedScopes: acceptedScopes,
		}
		tools = append(tools, toolInfo)

		// Track unique scopes
		for _, s := range requiredScopes {
			scopeSet[s] = true
			toolsByScope[s] = append(toolsByScope[s], tool.Name)
		}

		// Track scopes by tool
		scopesByTool[tool.Name] = requiredScopes
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

	// Get enabled toolsets as string slice
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

	// Summary
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

	// Legend
	fmt.Println("\nLegend: 👁 = read-only, 📝 = read-write")

	return nil
}
