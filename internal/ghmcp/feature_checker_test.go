package ghmcp

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateFeatureChecker(t *testing.T) {
	t.Parallel()

	// Only flags in AllowedFeatureFlags are honored; an enabled valid flag
	// resolves true, while other valid flags and unknown flags resolve false.
	checker := createFeatureChecker([]string{github.FeatureFlagCSVOutput, "bogus_unknown_flag"}, false)

	present, err := checker(context.Background(), github.FeatureFlagCSVOutput)
	require.NoError(t, err)
	assert.True(t, present)

	absent, err := checker(context.Background(), github.FeatureFlagIFCLabels)
	require.NoError(t, err)
	assert.False(t, absent)

	// An unknown flag is filtered out by ResolveFeatureFlags and stays disabled.
	unknown, err := checker(context.Background(), "bogus_unknown_flag")
	require.NoError(t, err)
	assert.False(t, unknown)

	// With no enabled features and insiders mode off, nothing is enabled.
	empty := createFeatureChecker(nil, false)
	got, err := empty(context.Background(), github.FeatureFlagCSVOutput)
	require.NoError(t, err)
	assert.False(t, got)
}
