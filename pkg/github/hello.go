package github

import (
	"context"
	"encoding/json"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HelloWorld returns a simple greeting tool that demonstrates feature flag conditional behavior.
// This tool is for testing and demonstration purposes only.
func HelloWorld(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataContext, // Use existing "context" toolset
		mcp.Tool{
			Name:        "hello_world",
			Description: t("TOOL_HELLO_WORLD_DESCRIPTION", "A simple greeting tool that demonstrates feature flag conditional behavior"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_HELLO_WORLD_TITLE", "Hello World"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {
						Type:        "string",
						Description: "Name to greet (optional, defaults to 'World')",
					},
				},
			},
		},
		[]scopes.Scope{}, // No GitHub scopes required - purely demonstrative
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			// Extract name parameter (optional)
			name := "World"
			if nameArg, ok := args["name"].(string); ok && nameArg != "" {
				name = nameArg
			}

			// Check feature flag to determine greeting style
			var greeting string
			if deps.IsFeatureEnabled(ctx, RemoteMCPExperimental) {
				// Experimental: More enthusiastic greeting
				greeting = "🚀 Hello, " + name + "! Welcome to the EXPERIMENTAL future of MCP! 🎉"
			} else {
				// Default: Simple greeting
				greeting = "Hello, " + name + "!"
			}

			// Build response
			response := map[string]any{
				"greeting":          greeting,
				"experimental_mode": deps.IsFeatureEnabled(ctx, RemoteMCPExperimental),
				"timestamp":         "2026-01-12", // Static for demonstration
			}

			jsonBytes, err := json.Marshal(response)
			if err != nil {
				return utils.NewToolResultError("failed to marshal response"), nil, nil
			}

			return utils.NewToolResultText(string(jsonBytes)), nil, nil
		},
	)
}
