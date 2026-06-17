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
	"github.com/github/github-mcp-server/pkg/permissions"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ToolPermissionInfo contains fine-grained permission information for a single tool.
type ToolPermissionInfo struct {
	Name        string `json:"name"`
	Toolset     string `json:"toolset"`
	ReadOnly    bool   `json:"read_only"`
	Requirement string `json:"requirement,omitempty"`
}

// PermissionsOutput is the full output structure for the list-permissions command.
type PermissionsOutput struct {
	Tools             []ToolPermissionInfo `json:"tools"`
	UniquePermissions []string             `json:"unique_permissions"`
	EnabledToolsets   []string             `json:"enabled_toolsets"`
	ReadOnly          bool                 `json:"read_only"`
}

var listPermissionsCmd = &cobra.Command{
	Use:   "list-permissions",
	Short: "List required fine-grained permissions for enabled tools",
	Long: `List the required fine-grained permissions for all enabled tools.

This command creates an inventory based on the same flags as the stdio command
and outputs the declared fine-grained permission requirement for each enabled
tool. Tools with no declared requirement are ungated (always shown) and are
omitted from the per-tool listing.

The output format can be controlled with the --output flag:
  - text (default): Human-readable text output
  - json: JSON output for programmatic use
  - summary: Just the unique permissions referenced

Examples:
  # List permissions for default toolsets
  github-mcp-server list-permissions

  # List permissions for specific toolsets
  github-mcp-server list-permissions --toolsets=repos,issues,pull_requests

  # List permissions for all toolsets
  github-mcp-server list-permissions --toolsets=all

  # Output as JSON
  github-mcp-server list-permissions --output=json

  # Just show unique permissions referenced
  github-mcp-server list-permissions --output=summary`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runListPermissions()
	},
}

func init() {
	listPermissionsCmd.Flags().StringP("output", "o", "text", "Output format: text, json, or summary")
	_ = viper.BindPFlag("list-permissions-output", listPermissionsCmd.Flags().Lookup("output"))

	rootCmd.AddCommand(listPermissionsCmd)
}

func runListPermissions() error {
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
	outputFormat := viper.GetString("list-permissions-output")

	// Create translation helper
	t, _ := translations.TranslationHelper()

	// Build inventory using the same logic as the stdio server
	inventoryBuilder := github.NewInventory(t).
		WithReadOnly(readOnly)

	if enabledToolsets != nil {
		inventoryBuilder = inventoryBuilder.WithToolsets(enabledToolsets)
	}

	if len(enabledTools) > 0 {
		inventoryBuilder = inventoryBuilder.WithTools(enabledTools)
	}

	inv, err := inventoryBuilder.Build()
	if err != nil {
		return fmt.Errorf("failed to build inventory: %w", err)
	}

	output := collectToolPermissions(inv, readOnly)

	switch outputFormat {
	case "json":
		return outputPermissionsJSON(output)
	case "summary":
		return outputPermissionsSummary(output)
	default:
		return outputPermissionsText(output)
	}
}

func collectToolPermissions(inv *inventory.Inventory, readOnly bool) PermissionsOutput {
	var tools []ToolPermissionInfo
	permSet := make(map[permissions.Permission]bool)

	// Get all available tools from the inventory; use context.Background()
	// for feature flag evaluation.
	availableTools := inv.AvailableTools(context.Background())

	for _, serverTool := range availableTools {
		req := serverTool.RequiredPermissions
		if req.IsZero() {
			continue
		}

		tools = append(tools, ToolPermissionInfo{
			Name:        serverTool.Tool.Name,
			Toolset:     string(serverTool.Toolset.ID),
			ReadOnly:    serverTool.IsReadOnly(),
			Requirement: req.String(),
		})

		for _, p := range req.Permissions() {
			permSet[p] = true
		}
	}

	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	uniquePermissions := make([]string, 0, len(permSet))
	for p := range permSet {
		uniquePermissions = append(uniquePermissions, string(p))
	}
	sort.Strings(uniquePermissions)

	toolsetIDs := inv.ToolsetIDs()
	toolsetIDStrs := make([]string, len(toolsetIDs))
	for i, id := range toolsetIDs {
		toolsetIDStrs[i] = string(id)
	}

	return PermissionsOutput{
		Tools:             tools,
		UniquePermissions: uniquePermissions,
		EnabledToolsets:   toolsetIDStrs,
		ReadOnly:          readOnly,
	}
}

func outputPermissionsJSON(output PermissionsOutput) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputPermissionsSummary(output PermissionsOutput) error {
	if len(output.UniquePermissions) == 0 {
		fmt.Println("No fine-grained permissions declared for enabled tools.")
		return nil
	}

	fmt.Println("Fine-grained permissions referenced by enabled tools:")
	fmt.Println()
	for _, p := range output.UniquePermissions {
		fmt.Printf("  %s\n", p)
	}
	fmt.Printf("\nTotal: %d unique permission(s)\n", len(output.UniquePermissions))
	return nil
}

func outputPermissionsText(output PermissionsOutput) error {
	fmt.Printf("Fine-Grained Permissions for Enabled Tools\n")
	fmt.Printf("==========================================\n\n")

	fmt.Printf("Enabled Toolsets: %s\n", strings.Join(output.EnabledToolsets, ", "))
	fmt.Printf("Read-Only Mode: %v\n\n", output.ReadOnly)

	if len(output.Tools) == 0 {
		fmt.Println("No tools declare a fine-grained permission requirement (all are ungated).")
		return nil
	}

	// Group tools by toolset
	toolsByToolset := make(map[string][]ToolPermissionInfo)
	for _, tool := range output.Tools {
		toolsByToolset[tool.Toolset] = append(toolsByToolset[tool.Toolset], tool)
	}

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
			fmt.Printf("  %s %s: %s\n", rwIndicator, tool.Name, tool.Requirement)
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("## Summary")
	fmt.Println()
	fmt.Println("Unique permissions referenced:")
	for _, p := range output.UniquePermissions {
		fmt.Printf("  • %s\n", p)
	}
	fmt.Printf("\nTotal: %d gated tools, %d unique permissions\n", len(output.Tools), len(output.UniquePermissions))

	fmt.Println("\nLegend: 👁 = read-only, 📝 = read-write")
	fmt.Println("Note: tools without a declared requirement are ungated and always shown.")

	return nil
}
