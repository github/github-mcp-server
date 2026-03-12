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
)

// RemoteMCPEnthusiasticGreeting is a dummy test feature flag .
const RemoteMCPEnthusiasticGreeting = "remote_mcp_enthusiastic_greeting"

// FeatureChecker is an interface for checking if a feature flag is enabled.
type FeatureChecker interface {
	// IsFeatureEnabled checks if a feature flag is enabled.
	IsFeatureEnabled(ctx context.Context, flagName string) bool
}

// HelloWorld returns a simple greeting tool that demonstrates feature flag conditional behavior.
// This tool is for testing and demonstration purposes only.
func HelloWorldTool(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataContext, // Use existing "context" toolset
		mcp.Tool{
			Name:        "hello_world",
			Description: t("TOOL_HELLO_WORLD_DESCRIPTION", "A simple greeting tool that demonstrates feature flag conditional behavior"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_HELLO_WORLD_TITLE", "Hello World"),
				ReadOnlyHint: true,
			},
		},
		[]scopes.Scope{},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) {

			// Check feature flag to determine greeting style
			greeting := "Hello, world!"
			if deps.IsFeatureEnabled(ctx, RemoteMCPEnthusiasticGreeting) {
				greeting += " Welcome to the future of MCP! 🎉"
			}
			if deps.GetFlags(ctx).InsidersMode {
				greeting += " Experimental features are enabled! 🚀"
			}

			// Build response
			response := map[string]any{
				"greeting": greeting,
			}

			jsonBytes, err := json.Marshal(response)
			if err != nil {
				return utils.NewToolResultError("failed to marshal response"), nil, nil
			}

			return utils.NewToolResultText(string(jsonBytes)), nil, nil
		},
	)
}

func TestHelloWorld_ConditionalBehavior_Featureflag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		featureFlagEnabled bool
		inputName          string
		expectedGreeting   string
	}{
		{
			name:               "Feature flag disabled - default greeting",
			featureFlagEnabled: false,
			expectedGreeting:   "Hello, world!",
		},
		{
			name:               "Feature flag enabled - enthusiastic greeting",
			featureFlagEnabled: true,
			expectedGreeting:   "Hello, world! Welcome to the future of MCP! 🎉",
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
			tool := HelloWorldTool(translations.NullTranslationHelper)
			handler := tool.Handler(deps)

			// Call the handler with deps in context
			ctx := ContextWithDeps(context.Background(), deps)
			result, err := handler(ctx, &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Arguments: json.RawMessage(`{}`),
				},
			})
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
		})
	}
}

func TestHelloWorld_ConditionalBehavior_Config(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		insidersMode     bool
		expectedGreeting string
	}{
		{
			name:             "Experimental disabled - default greeting",
			insidersMode:     false,
			expectedGreeting: "Hello, world!",
		},
		{
			name:             "Experimental enabled - experimental greeting",
			insidersMode:     true,
			expectedGreeting: "Hello, world! Experimental features are enabled! 🚀",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create deps with the checker
			deps := NewBaseDeps(
				nil, nil, nil, nil,
				translations.NullTranslationHelper,
				FeatureFlags{InsidersMode: tt.insidersMode},
				0,
				nil,
			)

			// Get the tool and its handler
			tool := HelloWorldTool(translations.NullTranslationHelper)
			handler := tool.Handler(deps)

			// Call the handler with deps in context
			ctx := ContextWithDeps(context.Background(), deps)
			result, err := handler(ctx, &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Arguments: json.RawMessage(`{}`),
				},
			})
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
		})
	}
}
