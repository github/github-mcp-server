package ghmcp

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/stretchr/testify/require"
)

// TestCreateFeatureChecker covers createFeatureChecker: a flag that is both
// allowed and enabled resolves true, anything else resolves false (with no
// error), and insiders mode turns on the full insiders set.
func TestCreateFeatureChecker(t *testing.T) {
	ctx := context.Background()

	t.Run("enabled allowed flag is on", func(t *testing.T) {
		check := createFeatureChecker([]string{github.FeatureFlagCSVOutput}, false)
		on, err := check(ctx, github.FeatureFlagCSVOutput)
		require.NoError(t, err)
		require.True(t, on)
	})

	t.Run("allowed-but-not-enabled flag is off", func(t *testing.T) {
		check := createFeatureChecker([]string{github.FeatureFlagCSVOutput}, false)
		on, err := check(ctx, github.FeatureFlagFileBlame)
		require.NoError(t, err)
		require.False(t, on)
	})

	t.Run("unknown flag is off", func(t *testing.T) {
		check := createFeatureChecker(nil, false)
		on, err := check(ctx, "definitely_not_a_real_flag")
		require.NoError(t, err)
		require.False(t, on)
	})

	t.Run("non-allowlisted flag is ignored even if passed", func(t *testing.T) {
		check := createFeatureChecker([]string{"not_in_allowlist"}, false)
		on, err := check(ctx, "not_in_allowlist")
		require.NoError(t, err)
		require.False(t, on)
	})

	t.Run("insiders mode enables the insiders set", func(t *testing.T) {
		check := createFeatureChecker(nil, true)
		for _, flag := range github.InsidersFeatureFlags {
			on, err := check(ctx, flag)
			require.NoError(t, err)
			require.True(t, on, "insiders flag %q should be enabled", flag)
		}
	})
}
