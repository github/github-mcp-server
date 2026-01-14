package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
)

// RemoteMCPExperimental is a long-lived feature flag for experimental remote MCP features.
// This flag enables experimental behaviors in tools that are being tested for remote server deployment.
const RemoteMCPEnthusiasticGreeting = "remote_mcp_enthusiastic_greeting"

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
		[]scopes.Scope{},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			// Extract name parameter (optional)
			name := "World"
			if nameArg, ok := args["name"].(string); ok && nameArg != "" {
				name = nameArg
			}

			// Check feature flag to determine greeting style
			var greeting string
			if deps.IsFeatureEnabled(ctx, RemoteMCPEnthusiasticGreeting) {
				// Experimental: More enthusiastic greeting
				greeting = "🚀 Hello, " + name + "! Welcome to the EXPERIMENTAL future of MCP! 🎉"
			} else {
				// Default: Simple greeting
				greeting = "Hello, " + name + "!"
			}

			// Build response
			response := map[string]any{
				"greeting":          greeting,
				"experimental_mode": deps.IsFeatureEnabled(ctx, RemoteMCPEnthusiasticGreeting),
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

func TestHelloWorld_ToolDefinition(t *testing.T) {
	t.Parallel()

	// Create tool
	tool := HelloWorld(translations.NullTranslationHelper)

	// Verify tool definition
	assert.Equal(t, "hello_world", tool.Tool.Name)
	assert.NotEmpty(t, tool.Tool.Description)
	assert.True(t, tool.Tool.Annotations.ReadOnlyHint, "hello_world should be read-only")
	assert.NotNil(t, tool.Tool.InputSchema)
	assert.NotNil(t, tool.HandlerFunc, "Tool must have a handler")

	// Verify it's in the context toolset
	assert.Equal(t, "context", string(tool.Toolset.ID))

	// Verify no scopes required
	assert.Empty(t, tool.RequiredScopes)

	// Verify no feature flags set (tool itself isn't gated by flags)
	assert.Empty(t, tool.FeatureFlagEnable)
	assert.Empty(t, tool.FeatureFlagDisable)
}

func TestHelloWorld_ConditionalBehavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		featureFlagEnabled       bool
		inputName                string
		expectedGreeting         string
		expectedExperimentalMode bool
	}{
		{
			name:                     "Feature flag disabled - default greeting",
			featureFlagEnabled:       false,
			inputName:                "Alice",
			expectedGreeting:         "Hello, Alice!",
			expectedExperimentalMode: false,
		},
		{
			name:                     "Feature flag enabled - experimental greeting",
			featureFlagEnabled:       true,
			inputName:                "Alice",
			expectedGreeting:         "🚀 Hello, Alice! Welcome to the EXPERIMENTAL future of MCP! 🎉",
			expectedExperimentalMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create feature checker based on test case
			checker := func(_ context.Context, flagName string) (bool, error) {
				if flagName == RemoteMCPEnthusiasticGreeting {
					return tt.featureFlagEnabled, nil
				}
				return false, nil
			}

			// Create deps with the checker
			deps := NewBaseDeps(
				nil, nil, nil, nil,
				translations.NullTranslationHelper,
				FeatureFlags{},
				0,
				checker,
			)

			// Get the tool and its handler
			tool := HelloWorld(translations.NullTranslationHelper)
			handler := tool.Handler(deps)

			// Create request
			args := map[string]any{}
			if tt.inputName != "" {
				args["name"] = tt.inputName
			}
			argsJSON, err := json.Marshal(args)
			require.NoError(t, err)

			request := mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Arguments: json.RawMessage(argsJSON),
				},
			}

			// Call the handler with deps in context
			ctx := ContextWithDeps(context.Background(), deps)
			result, err := handler(ctx, &request)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.Content, 1)

			// Parse the response - should be TextContent
			textContent, ok := result.Content[0].(*mcp.TextContent)
			require.True(t, ok, "expected content to be TextContent")

			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)

			// Verify the greeting matches expected based on feature flag
			assert.Equal(t, tt.expectedGreeting, response["greeting"])
			assert.Equal(t, tt.expectedExperimentalMode, response["experimental_mode"])
		})
	}
}
