package github_test

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelloWorld_ToolDefinition(t *testing.T) {
	t.Parallel()

	// Create tool
	tool := github.HelloWorld(translations.NullTranslationHelper)

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

func TestHelloWorld_IsFeatureEnabledIntegration(t *testing.T) {
	t.Parallel()

	// This test verifies that the feature flag checking mechanism works
	// by testing deps.IsFeatureEnabled directly

	// Test 1: With feature flag disabled
	checkerDisabled := func(ctx context.Context, flagName string) (bool, error) {
		return false, nil
	}
	depsDisabled := github.NewBaseDeps(
		nil, nil, nil, nil,
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,
		checkerDisabled,
	)

	result := depsDisabled.IsFeatureEnabled(context.Background(), github.RemoteMCPExperimental)
	assert.False(t, result, "Feature flag should be disabled")

	// Test 2: With feature flag enabled
	checkerEnabled := func(ctx context.Context, flagName string) (bool, error) {
		return flagName == github.RemoteMCPExperimental, nil
	}
	depsEnabled := github.NewBaseDeps(
		nil, nil, nil, nil,
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,
		checkerEnabled,
	)

	result = depsEnabled.IsFeatureEnabled(context.Background(), github.RemoteMCPExperimental)
	assert.True(t, result, "Feature flag should be enabled")

	result = depsEnabled.IsFeatureEnabled(context.Background(), "other_flag")
	assert.False(t, result, "Other flag should be disabled")
}

func TestHelloWorld_FeatureFlagConstant(t *testing.T) {
	t.Parallel()

	// Verify the constant exists and has the expected value
	assert.Equal(t, "remote_mcp_experimental", github.RemoteMCPExperimental)
	require.NotEmpty(t, github.RemoteMCPExperimental, "Feature flag constant should not be empty")
}
