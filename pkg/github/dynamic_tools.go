package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func ToolsetEnum(toolsetGroup *toolsets.ToolsetGroup) []any {
	toolsetNames := make([]any, 0, len(toolsetGroup.Toolsets))
	for name := range toolsetGroup.Toolsets {
		toolsetNames = append(toolsetNames, name)
	}
	return toolsetNames
}

func EnableToolset(s *mcp.Server, toolsetGroup *toolsets.ToolsetGroup, t translations.TranslationHelperFunc) toolsets.ServerTool {
	return toolsets.NewServerToolLegacy(mcp.Tool{
		Name:        "enable_toolset",
		Description: t("TOOL_ENABLE_TOOLSET_DESCRIPTION", "Enable one of the sets of tools the GitHub MCP server provides, use get_toolset_tools and list_available_toolsets first to see what this will enable"),
		Annotations: &mcp.ToolAnnotations{
			Title: t("TOOL_ENABLE_TOOLSET_USER_TITLE", "Enable a toolset"),
			// Not modifying GitHub data so no need to show a warning
			ReadOnlyHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"toolset": {
					Type:        "string",
					Description: "The name of the toolset to enable",
					Enum:        ToolsetEnum(toolsetGroup),
				},
			},
			Required: []string{"toolset"},
		},
	},
		mcp.ToolHandlerFor[map[string]any, any](func(_ context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			// We need to convert the toolsets back to a map for JSON serialization
			toolsetName, err := RequiredParam[string](args, "toolset")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			toolset := toolsetGroup.Toolsets[toolsetName]
			if toolset == nil {
				return utils.NewToolResultError(fmt.Sprintf("Toolset %s not found", toolsetName)), nil, nil
			}
			if toolset.Enabled {
				return utils.NewToolResultText(fmt.Sprintf("Toolset %s is already enabled", toolsetName)), nil, nil
			}

			toolset.Enabled = true

			// caution: this currently affects the global tools and notifies all clients:
			//
			// Send notification to all initialized sessions
			// s.sendNotificationToAllClients("notifications/tools/list_changed", nil)
			toolset.RegisterTools(s)

			return utils.NewToolResultText(fmt.Sprintf("Toolset %s enabled", toolsetName)), nil, nil
		}))
}

func ListAvailableToolsets(toolsetGroup *toolsets.ToolsetGroup, t translations.TranslationHelperFunc) toolsets.ServerTool {
	return toolsets.NewServerToolLegacy(mcp.Tool{
		Name:        "list_available_toolsets",
		Description: t("TOOL_LIST_AVAILABLE_TOOLSETS_DESCRIPTION", "List all available toolsets this GitHub MCP server can offer, providing the enabled status of each. Use this when a task could be achieved with a GitHub tool and the currently available tools aren't enough. Call get_toolset_tools with these toolset names to discover specific tools you can call"),
		Annotations: &mcp.ToolAnnotations{
			Title:        t("TOOL_LIST_AVAILABLE_TOOLSETS_USER_TITLE", "List available toolsets"),
			ReadOnlyHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{},
		},
	},
		mcp.ToolHandlerFor[map[string]any, any](func(_ context.Context, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) {
			// We need to convert the toolsetGroup back to a map for JSON serialization

			payload := []map[string]string{}

			for name, ts := range toolsetGroup.Toolsets {
				{
					t := map[string]string{
						"name":              name,
						"description":       ts.Description,
						"can_enable":        "true",
						"currently_enabled": fmt.Sprintf("%t", ts.Enabled),
					}
					payload = append(payload, t)
				}
			}

			r, err := json.Marshal(payload)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal features: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		}))
}

func GetToolsetsTools(toolsetGroup *toolsets.ToolsetGroup, t translations.TranslationHelperFunc) toolsets.ServerTool {
	return toolsets.NewServerToolLegacy(mcp.Tool{
		Name:        "get_toolset_tools",
		Description: t("TOOL_GET_TOOLSET_TOOLS_DESCRIPTION", "Lists all the capabilities that are enabled with the specified toolset, use this to get clarity on whether enabling a toolset would help you to complete a task"),
		Annotations: &mcp.ToolAnnotations{
			Title:        t("TOOL_GET_TOOLSET_TOOLS_USER_TITLE", "List all tools in a toolset"),
			ReadOnlyHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"toolset": {
					Type:        "string",
					Description: "The name of the toolset you want to get the tools for",
					Enum:        ToolsetEnum(toolsetGroup),
				},
			},
			Required: []string{"toolset"},
		},
	},
		mcp.ToolHandlerFor[map[string]any, any](func(_ context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			// We need to convert the toolsetGroup back to a map for JSON serialization
			toolsetName, err := RequiredParam[string](args, "toolset")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			toolset := toolsetGroup.Toolsets[toolsetName]
			if toolset == nil {
				return utils.NewToolResultError(fmt.Sprintf("Toolset %s not found", toolsetName)), nil, nil
			}
			payload := []map[string]string{}

			for _, st := range toolset.GetAvailableTools() {
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
		}))
}
