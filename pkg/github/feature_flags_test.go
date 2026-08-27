package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/utils"
)

// RemoteMCPEnthusiasticGreeting is a dummy test feature flag .
const RemoteMCPEnthusiasticGreeting inventory.FeatureFlag = "remote_mcp_enthusiastic_greeting"

func featureCheckerFor(enabledFlags ...inventory.FeatureFlag) inventory.FeatureFlagChecker {
	enabled := make(map[inventory.FeatureFlag]bool, len(enabledFlags))
	for _, flag := range enabledFlags {
		enabled[flag] = true
	}
	return func(_ context.Context, flagName inventory.FeatureFlag) (bool, error) {
		return enabled[flagName], nil
	}
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
		scopes.NoScopes(),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, _ map[string]any) (*mcp.CallToolResult, any, error) {

			// Check feature flag to determine greeting style
			greeting := "Hello, world!"
			if deps.IsFeatureEnabled(ctx, RemoteMCPEnthusiasticGreeting) {
				greeting += " Welcome to the future of MCP! 🎉"
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

			var enabledFlags []inventory.FeatureFlag
			if tt.featureFlagEnabled {
				enabledFlags = append(enabledFlags, RemoteMCPEnthusiasticGreeting)
			}

			// Create deps with the checker
			deps := NewBaseDeps(
				nil, nil, nil, nil,
				translations.NullTranslationHelper,
				FeatureFlags{},
				0,
				featureCheckerFor(enabledFlags...),
				stubExporters(),
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

func TestResolveFeatureFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		enabledFeatures []inventory.FeatureFlag
		insidersMode    bool
		expectedFlags   []inventory.FeatureFlag
		unexpectedFlags []inventory.FeatureFlag
	}{
		{
			name:            "no features, no insiders",
			enabledFeatures: nil,
			expectedFlags:   nil,
			unexpectedFlags: []inventory.FeatureFlag{MCPAppsFeatureFlag},
		},
		{
			name:            "explicit feature enabled",
			enabledFeatures: []inventory.FeatureFlag{MCPAppsFeatureFlag},
			expectedFlags:   []inventory.FeatureFlag{MCPAppsFeatureFlag},
		},
		{
			name:            "MCP Apps form deferral can be disabled directly",
			enabledFeatures: []inventory.FeatureFlag{MCPAppsDisableFormDeferralFeatureFlag},
			expectedFlags:   []inventory.FeatureFlag{MCPAppsDisableFormDeferralFeatureFlag},
		},
		{
			name:            "insiders mode enables insiders flags",
			enabledFeatures: nil,
			insidersMode:    true,
			expectedFlags:   InsidersFeatureFlags,
		},
		{
			name:            "insiders mode does not auto-enable ifc labels",
			enabledFeatures: nil,
			insidersMode:    true,
			unexpectedFlags: []inventory.FeatureFlag{FeatureFlagIFCLabels},
		},
		{
			name:            "insiders mode does not disable MCP Apps form deferral",
			enabledFeatures: nil,
			insidersMode:    true,
			unexpectedFlags: []inventory.FeatureFlag{MCPAppsDisableFormDeferralFeatureFlag},
		},
		{
			name:            "ifc_labels can be directly enabled",
			enabledFeatures: []inventory.FeatureFlag{FeatureFlagIFCLabels},
			expectedFlags:   []inventory.FeatureFlag{FeatureFlagIFCLabels},
		},
		{
			name:            "unknown flags are filtered out",
			enabledFeatures: []inventory.FeatureFlag{"unknown_flag", "another_unknown"},
			unexpectedFlags: []inventory.FeatureFlag{"unknown_flag", "another_unknown"},
		},
		{
			name:            "mix of known and unknown flags",
			enabledFeatures: []inventory.FeatureFlag{MCPAppsFeatureFlag, "unknown_flag"},
			expectedFlags:   []inventory.FeatureFlag{MCPAppsFeatureFlag},
			unexpectedFlags: []inventory.FeatureFlag{"unknown_flag"},
		},
		{
			name:            "user-only flags can be enabled but are not turned on by insiders",
			enabledFeatures: []inventory.FeatureFlag{FeatureFlagIssuesGranular},
			insidersMode:    false,
			expectedFlags:   []inventory.FeatureFlag{FeatureFlagIssuesGranular},
		},
		{
			name:            "thread resolution reason can be directly enabled",
			enabledFeatures: []inventory.FeatureFlag{FeatureFlagThreadResolutionReason},
			expectedFlags:   []inventory.FeatureFlag{FeatureFlagThreadResolutionReason},
		},
		{
			name:            "insiders does not enable user-only allowed flags",
			enabledFeatures: nil,
			insidersMode:    true,
			unexpectedFlags: []inventory.FeatureFlag{FeatureFlagIssuesGranular, FeatureFlagPullRequestsGranular},
		},
		{
			name:            "explicit plus insiders deduplicates",
			enabledFeatures: []inventory.FeatureFlag{MCPAppsFeatureFlag},
			insidersMode:    true,
			expectedFlags:   InsidersFeatureFlags,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ResolveFeatureFlags(tt.enabledFeatures, tt.insidersMode)
			for _, flag := range tt.expectedFlags {
				assert.True(t, result[flag], "expected flag %q to be enabled", flag)
			}
			for _, flag := range tt.unexpectedFlags {
				assert.False(t, result[flag], "expected flag %q to not be enabled", flag)
			}
		})
	}
}

func TestThreadResolutionReasonToolVariants(t *testing.T) {
	tests := []struct {
		name      string
		flags     []inventory.FeatureFlag
		host      utils.HostType
		toolName  string
		hasReason bool
	}{
		{
			name:     "consolidated flag off",
			toolName: "pull_request_review_write",
		},
		{
			name:      "consolidated flag on",
			flags:     []inventory.FeatureFlag{FeatureFlagThreadResolutionReason},
			toolName:  "pull_request_review_write",
			hasReason: true,
		},
		{
			name:     "granular flag off",
			flags:    []inventory.FeatureFlag{FeatureFlagPullRequestsGranular},
			toolName: "resolve_review_thread",
		},
		{
			name:      "granular flag on",
			flags:     []inventory.FeatureFlag{FeatureFlagPullRequestsGranular, FeatureFlagThreadResolutionReason},
			toolName:  "resolve_review_thread",
			hasReason: true,
		},
		{
			name:     "consolidated flag on GHES",
			flags:    []inventory.FeatureFlag{FeatureFlagThreadResolutionReason},
			host:     utils.HostTypeGHES,
			toolName: "pull_request_review_write",
		},
		{
			name:     "granular flag on GHES",
			flags:    []inventory.FeatureFlag{FeatureFlagPullRequestsGranular, FeatureFlagThreadResolutionReason},
			host:     utils.HostTypeGHES,
			toolName: "resolve_review_thread",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv, err := NewInventory(translations.NullTranslationHelper, WithHost(tt.host)).
				WithToolsets([]string{"all"}).
				WithFeatureChecker(featureCheckerFor(tt.flags...)).
				Build()
			require.NoError(t, err)

			var matches []inventory.ServerTool
			for _, tool := range inv.AvailableTools(context.Background()) {
				if tool.Tool.Name == tt.toolName {
					matches = append(matches, tool)
				}
			}
			require.Len(t, matches, 1)
			schema := matches[0].Tool.InputSchema.(*jsonschema.Schema)
			_, hasReason := schema.Properties["resolutionReason"]
			assert.Equal(t, tt.hasReason, hasReason)
		})
	}
}
