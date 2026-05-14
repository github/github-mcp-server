package http

import (
	"context"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateHTTPFeatureChecker(t *testing.T) {
	checker := createHTTPFeatureChecker()

	tests := []struct {
		name           string
		flagName       string
		headerFeatures []string
		insidersMode   bool
		wantEnabled    bool
	}{
		{
			name:           "allowed issues_granular flag accepted from header",
			flagName:       github.FeatureFlagIssuesGranular,
			headerFeatures: []string{github.FeatureFlagIssuesGranular},
			wantEnabled:    true,
		},
		{
			name:           "allowed pull_requests_granular flag accepted from header",
			flagName:       github.FeatureFlagPullRequestsGranular,
			headerFeatures: []string{github.FeatureFlagPullRequestsGranular},
			wantEnabled:    true,
		},
		{
			name:           "MCP Apps flag accepted from header",
			flagName:       github.MCPAppsFeatureFlag,
			headerFeatures: []string{github.MCPAppsFeatureFlag},
			wantEnabled:    true,
		},
		{
			name:           "unknown flag in header is ignored",
			flagName:       "unknown_flag",
			headerFeatures: []string{"unknown_flag"},
			wantEnabled:    false,
		},
		{
			name:           "allowed flag not in header returns false",
			flagName:       github.FeatureFlagIssuesGranular,
			headerFeatures: nil,
			wantEnabled:    false,
		},
		{
			name:           "allowed flag with different flag in header returns false",
			flagName:       github.FeatureFlagIssuesGranular,
			headerFeatures: []string{github.FeatureFlagPullRequestsGranular},
			wantEnabled:    false,
		},
		{
			name:           "multiple allowed flags in header",
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
		{
			name:         "insiders mode enables MCP Apps without header",
			flagName:     github.MCPAppsFeatureFlag,
			insidersMode: true,
			wantEnabled:  true,
		},
		{
			name:         "insiders mode does not enable granular flags",
			flagName:     github.FeatureFlagIssuesGranular,
			insidersMode: true,
			wantEnabled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if len(tt.headerFeatures) > 0 {
				ctx = ghcontext.WithHeaderFeatures(ctx, tt.headerFeatures)
			}
			if tt.insidersMode {
				ctx = ghcontext.WithInsidersMode(ctx, true)
			}

			enabled, err := checker(ctx, tt.flagName)
			require.NoError(t, err)
			assert.Equal(t, tt.wantEnabled, enabled)
		})
	}
}

func TestHeaderAllowedFeatureFlagsMatchesAllowed(t *testing.T) {
	// Ensure HeaderAllowedFeatureFlags delegates to AllowedFeatureFlags
	allowed := github.HeaderAllowedFeatureFlags()
	assert.Equal(t, github.AllowedFeatureFlags, allowed,
		"HeaderAllowedFeatureFlags() should match AllowedFeatureFlags")
	assert.NotEmpty(t, allowed, "AllowedFeatureFlags should not be empty")
}
