package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DynamicToolDependencies contains dependencies for dynamic toolset management tools.
// It includes the managed Inventory, the server for registration, and the deps
// that will be passed to tools when they are dynamically enabled.
type DynamicToolDependencies struct {
	// Server is the MCP server to register tools with
	Server *mcp.Server
	// Inventory contains all available tools, resources and prompts that can be enabled dynamically
	Inventory *inventory.Inventory
	// ToolDeps are the dependencies passed to tools when they are registered
	ToolDeps any
	// T is the translation helper function
	T translations.TranslationHelperFunc
}

// NewDynamicTool creates a ServerTool with fully-typed DynamicToolDependencies.
// Dynamic tools use a different dependency structure (DynamicToolDependencies) than regular
// tools (ToolDependencies), so they intentionally use the closure pattern.
func NewDynamicTool(toolset inventory.ToolsetMetadata, tool mcp.Tool, handler func(deps DynamicToolDependencies) mcp.ToolHandlerFor[map[string]any, any]) inventory.ServerTool {
	//nolint:staticcheck // SA1019: Dynamic tools use a different deps structure, closure pattern is intentional
	return inventory.NewServerTool(tool, toolset, func(d any) mcp.ToolHandlerFor[map[string]any, any] {
		return handler(d.(DynamicToolDependencies))
	})
}

// toolsetIDsEnum returns the list of toolset IDs as an enum for JSON Schema.
func toolsetIDsEnum(r *inventory.Inventory) []any {
	toolsetIDs := r.ToolsetIDs()
	result := make([]any, len(toolsetIDs))
	for i, id := range toolsetIDs {
		result[i] = id
	}
	return result
}

// DynamicTools returns the tools for dynamic toolset management.
// These tools allow runtime discovery and enablement of inventory.
// The r parameter provides the available toolset IDs for JSON Schema enums.
func DynamicTools(r *inventory.Inventory) []inventory.ServerTool {
	return []inventory.ServerTool{
		ListAvailableToolsets(),
		GetToolsetsTools(r),
		EnableToolset(r),
	}
}

// EnableToolset creates a tool that enables a toolset at runtime.
func EnableToolset(r *inventory.Inventory) inventory.ServerTool {
	return NewDynamicTool(
		ToolsetMetadataDynamic,
		mcp.Tool{
			Name:        "enable_toolset",
			Description: "Enable one of the sets of tools the GitHub MCP server provides, use get_toolset_tools and list_available_toolsets first to see what this will enable",
			Annotations: &mcp.ToolAnnotations{
				Title:        "Enable a toolset",
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"toolset": {
						Type:        "string",
						Description: "The name of the toolset to enable",
						Enum:        toolsetIDsEnum(r),
					},
				},
				Required: []string{"toolset"},
			},
		},
		func(deps DynamicToolDependencies) mcp.ToolHandlerFor[map[string]any, any] {
			return func(_ context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
				toolsetName, err := RequiredParam[string](args, "toolset")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				toolsetID := inventory.ToolsetID(toolsetName)

				if !deps.Inventory.HasToolset(toolsetID) {
					return utils.NewToolResultError(fmt.Sprintf("Toolset %s not found", toolsetName)), nil, nil
				}

				if deps.Inventory.IsToolsetEnabled(toolsetID) {
					return utils.NewToolResultText(fmt.Sprintf("Toolset %s is already enabled", toolsetName)), nil, nil
				}

				// Mark the toolset as enabled so IsToolsetEnabled returns true
				deps.Inventory.EnableToolset(toolsetID)

				// Get tools for this toolset and register them with the managed deps
				toolsForToolset := deps.Inventory.ToolsForToolset(toolsetID)
				for _, st := range toolsForToolset {
					st.RegisterFunc(deps.Server, deps.ToolDeps)
				}

				return utils.NewToolResultText(fmt.Sprintf("Toolset %s enabled with %d tools", toolsetName, len(toolsForToolset))), nil, nil
			}
		},
	)
}

// ListAvailableToolsets creates a tool that lists all available inventory.
func ListAvailableToolsets() inventory.ServerTool {
	return NewDynamicTool(
		ToolsetMetadataDynamic,
		mcp.Tool{
			Name:        "list_available_toolsets",
			Description: "List all available toolsets this GitHub MCP server can offer, providing the enabled status of each. Use this when a task could be achieved with a GitHub tool and the currently available tools aren't enough. Call get_toolset_tools with these toolset names to discover specific tools you can call",
			Annotations: &mcp.ToolAnnotations{
				Title:        "List available toolsets",
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: map[string]*jsonschema.Schema{},
			},
		},
		func(deps DynamicToolDependencies) mcp.ToolHandlerFor[map[string]any, any] {
			return func(_ context.Context, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) {
				toolsetIDs := deps.Inventory.ToolsetIDs()
				descriptions := deps.Inventory.ToolsetDescriptions()

				payload := make([]map[string]string, 0, len(toolsetIDs))
				for _, id := range toolsetIDs {
					t := map[string]string{
						"name":              string(id),
						"description":       descriptions[id],
						"can_enable":        "true",
						"currently_enabled": fmt.Sprintf("%t", deps.Inventory.IsToolsetEnabled(id)),
					}
					payload = append(payload, t)
				}

				r, err := json.Marshal(payload)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to marshal features: %w", err)
				}

				return utils.NewToolResultText(string(r)), nil, nil
			}
		},
	)
}

// GetToolsetsTools creates a tool that lists all tools in a specific toolset.
func GetToolsetsTools(r *inventory.Inventory) inventory.ServerTool {
	return NewDynamicTool(
		ToolsetMetadataDynamic,
		mcp.Tool{
			Name:        "get_toolset_tools",
			Description: "Lists all the capabilities that are enabled with the specified toolset, use this to get clarity on whether enabling a toolset would help you to complete a task",
			Annotations: &mcp.ToolAnnotations{
				Title:        "List all tools in a toolset",
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"toolset": {
						Type:        "string",
						Description: "The name of the toolset you want to get the tools for",
						Enum:        toolsetIDsEnum(r),
					},
				},
				Required: []string{"toolset"},
			},
		},
		func(deps DynamicToolDependencies) mcp.ToolHandlerFor[map[string]any, any] {
			return func(_ context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
				toolsetName, err := RequiredParam[string](args, "toolset")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				toolsetID := inventory.ToolsetID(toolsetName)

				if !deps.Inventory.HasToolset(toolsetID) {
					return utils.NewToolResultError(fmt.Sprintf("Toolset %s not found", toolsetName)), nil, nil
				}

				// Get all tools for this toolset (ignoring current filters for discovery)
				toolsInToolset := deps.Inventory.ToolsForToolset(toolsetID)
				payload := make([]map[string]string, 0, len(toolsInToolset))

				for _, st := range toolsInToolset {
					tool := map[string]string{
						"name":        st.Tool.Name,
						"description": st.Tool.Description,
						"can_enable":  "true",
						"toolset":     toolsetName,
					}
					payload = append(payload, tool)
				}

				r, err := json.Marshal(payload)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to marshal features: %w", err)
				}

				return utils.NewToolResultText(string(r)), nil, nil
			}
		},
	)
}
