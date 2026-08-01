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

// RemoteMCPEnthusiasticGreeting is 一个dummy test 功能标志 .
const RemoteMCPEnthusiasticGreeting = "remote_mcp_enthusiastic_greeting"

func featureCheckerFor(enabledFlags ...string) func(context.Context, string) (bool, error) {
	enabled := make(map[string]bool, len(enabledFlags))
	for _, flag := range enabledFlags {
		enabled[flag] = true
	}
	return func(_ context.Context, flagName string) (bool, error) {
		return enabled[flagName], nil
	}
}

// HelloWorld 返回 一个simple greeting 工具 that demonstrates 功能标志 conditional behavior.
// 此工具 is f或testing 和demonstration purposes only.
func HelloWorldTool(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataContext, // Use existing "上下文" 工具集
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

			// Check 功能标志 to determine greeting style
			greeting := "Hello, world!"
			if deps.IsFeatureEnabled(ctx, RemoteMCPEnthusiasticGreeting) {
				greeting += " Welcome to the future of MCP! 🎉"
			}

			// Build 响应
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

			var enabledFlags []string
			if tt.featureFlagEnabled {
				enabledFlags = append(enabledFlags, RemoteMCPEnthusiasticGreeting)
			}

			// Create deps 使用检查er
			deps := NewBaseDeps(
				nil, nil, nil, nil,
				translations.NullTranslationHelper,
				FeatureFlags{},
				0,
				featureCheckerFor(enabledFlags...),
				stubExporters(),
			)

			// Get 工具 和its 处理器
			tool := HelloWorldTool(translations.NullTranslationHelper)
			handler := tool.Handler(deps)

			// C所有处理器 with deps in 上下文
			ctx := ContextWithDeps(context.Background(), deps)
			result, err := handler(ctx, &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Arguments: json.RawMessage(`{}`),
				},
			})
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.Content, 1)

			// Parse 响应 - 应当是 TextContent
			textContent, ok := result.Content[0].(*mcp.TextContent)
			require.True(t, ok, "expected content to be TextContent")

			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)

			// Verify greeting matches expected based on 功能标志
			assert.Equal(t, tt.expectedGreeting, response["greeting"])
		})
	}
}

func TestResolveFeatureFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		enabledFeatures []string
		insidersMode    bool
		expectedFlags   []string
		unexpectedFlags []string
	}{
		{
			name:            "no features, no insiders",
			enabledFeatures: nil,
			expectedFlags:   nil,
			unexpectedFlags: []string{MCPAppsFeatureFlag},
		},
		{
			name:            "explicit feature enabled",
			enabledFeatures: []string{MCPAppsFeatureFlag},
			expectedFlags:   []string{MCPAppsFeatureFlag},
		},
		{
			name:            "MCP Apps form deferral can be disabled directly",
			enabledFeatures: []string{MCPAppsDisableFormDeferralFeatureFlag},
			expectedFlags:   []string{MCPAppsDisableFormDeferralFeatureFlag},
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
			unexpectedFlags: []string{FeatureFlagIFCLabels},
		},
		{
			name:            "insiders mode does not disable MCP Apps form deferral",
			enabledFeatures: nil,
			insidersMode:    true,
			unexpectedFlags: []string{MCPAppsDisableFormDeferralFeatureFlag},
		},
		{
			name:            "ifc_labels can be directly enabled",
			enabledFeatures: []string{FeatureFlagIFCLabels},
			expectedFlags:   []string{FeatureFlagIFCLabels},
		},
		{
			name:            "unknown flags are filtered out",
			enabledFeatures: []string{"unknown_flag", "another_unknown"},
			unexpectedFlags: []string{"unknown_flag", "another_unknown"},
		},
		{
			name:            "mix of known and unknown flags",
			enabledFeatures: []string{MCPAppsFeatureFlag, "unknown_flag"},
			expectedFlags:   []string{MCPAppsFeatureFlag},
			unexpectedFlags: []string{"unknown_flag"},
		},
		{
			name:            "user-only flags can be enabled but are not turned on by insiders",
			enabledFeatures: []string{FeatureFlagIssuesGranular},
			insidersMode:    false,
			expectedFlags:   []string{FeatureFlagIssuesGranular},
		},
		{
			name:            "insiders does not enable user-only allowed flags",
			enabledFeatures: nil,
			insidersMode:    true,
			unexpectedFlags: []string{FeatureFlagIssuesGranular, FeatureFlagPullRequestsGranular},
		},
		{
			name:            "explicit plus insiders deduplicates",
			enabledFeatures: []string{MCPAppsFeatureFlag},
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
