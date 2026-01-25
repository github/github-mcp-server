package main

import (
	"fmt"
	"sort"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listToolsPocCmd = &cobra.Command{
	Use:   "list-tools",
	Short: "List available MCP tools grouped by toolset",
	Long:  `Display all registered MCP tools, grouped by toolset.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("TODO: Implement list-tools functionality")
		fmt.Println("This is a proof of concept for the list-tools command.")
		fmt.Println("Would display all available MCP tools grouped by toolset.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listToolsPocCmd)
}

// TODO: Add filtering by toolset (e.g., --toolset=repos)
// TODO: Support output formats (e.g., --format=json, yaml, table)
// TODO: Respect active toolsets, dynamic toolsets, and read-only flags
// TODO: Add unit tests once design is confirmed useful

func listTools() error {
	// Get configuration from viper
	var enabledToolsets []string
	if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
		return fmt.Errorf("failed to unmarshal toolsets: %w", err)
	}

	readOnly := viper.GetBool("read-only")

	// Create translation helper
	t, _ := translations.TranslationHelper()

	// Create toolset group with mock clients
	tsg := github.DefaultToolsetGroup(readOnly, mockGetClient, mockGetGQLClient, mockGetRawClient, t)

	// Enable specified toolsets
	if err := tsg.EnableToolsets(enabledToolsets); err != nil {
		return fmt.Errorf("failed to enable toolsets: %w", err)
	}

	// Get sorted toolset names
	var toolsetNames []string
	for name := range tsg.Toolsets {
		toolsetNames = append(toolsetNames, name)
	}
	sort.Strings(toolsetNames)

	for _, toolsetName := range toolsetNames {
		toolset := tsg.Toolsets[toolsetName]

		// Skip if toolset is not enabled
		if !toolset.Enabled {
			continue
		}

		fmt.Printf("\nToolset: %s\n", toolsetName)
		fmt.Printf("Description: %s\n", toolset.Description)
		fmt.Println()

		tools := toolset.GetActiveTools()
		if len(tools) == 0 {
			fmt.Println("  No tools available")
			continue
		}

		// Sort tools by name
		sort.Slice(tools, func(i, j int) bool {
			return tools[i].Tool.Name < tools[j].Tool.Name
		})

		for _, serverTool := range tools {
			tool := serverTool.Tool
			fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
		}
		fmt.Println()
	}

	return nil
}
