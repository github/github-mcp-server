package http

import (
	"context"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateHTTPFeatureChecker_Whitelist(t *testing.T) {
	checker := createHTTPFeatureChecker()

	tests := []struct {
		name           string
		flagName       string
		headerFeatures []string
		wantEnabled    bool
	}{
		{
			name:           "whitelisted issues_granular flag accepted from header",
			flagName:       github.FeatureFlagIssuesGranular,
			headerFeatures: []string{github.FeatureFlagIssuesGranular},
			wantEnabled:    true,
		},
		{
			name:           "whitelisted pull_requests_granular flag accepted from header",
			flagName:       github.FeatureFlagPullRequestsGranular,
			headerFeatures: []string{github.FeatureFlagPullRequestsGranular},
			wantEnabled:    true,
		},
		{
			name:           "unknown flag in header is ignored",
			flagName:       "unknown_flag",
			headerFeatures: []string{"unknown_flag"},
			wantEnabled:    false,
		},
		{
			name:           "whitelisted flag not in header returns false",
			flagName:       github.FeatureFlagIssuesGranular,
			headerFeatures: nil,
			wantEnabled:    false,
		},
		{
			name:           "whitelisted flag with different flag in header returns false",
			flagName:       github.FeatureFlagIssuesGranular,
			headerFeatures: []string{github.FeatureFlagPullRequestsGranular},
			wantEnabled:    false,
		},
		{
			name:           "multiple whitelisted flags in header",
			flagName:       github.FeatureFlagIssuesGranular,
			headerFeatures: []string{github.FeatureFlagIssuesGranular, github.FeatureFlagPullRequestsGranular},
			wantEnabled:    true,
		},
		{
			name:           "empty header features",
			flagName:       github.FeatureFlagIssuesGranular,
			headerFeatures: []string{},
			wantEnabled:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if len(tt.headerFeatures) > 0 {
				ctx = ghcontext.WithHeaderFeatures(ctx, tt.headerFeatures)
			}

			enabled, err := checker(ctx, tt.flagName)
			require.NoError(t, err)
			assert.Equal(t, tt.wantEnabled, enabled)
		})
	}
}

func TestKnownFeatureFlagsMatchesHeaderAllowed(t *testing.T) {
	// Ensure knownFeatureFlags stays in sync with HeaderAllowedFeatureFlags
	allowed := github.HeaderAllowedFeatureFlags()
	assert.Equal(t, allowed, knownFeatureFlags,
		"knownFeatureFlags should match github.HeaderAllowedFeatureFlags()")
	assert.NotEmpty(t, knownFeatureFlags, "knownFeatureFlags should not be empty")
}
