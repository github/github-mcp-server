package http

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHTTPServerRejectsInvalidStaticTools(t *testing.T) {
	tests := []struct {
		name         string
		enabledTools []string
	}{
		{
			name:         "mixed valid and invalid tools",
			enabledTools: []string{"get_file_contents", "nonexistent_tool"},
		},
		{
			name:         "all invalid tools",
			enabledTools: []string{"nonexistent_tool", "another_nonexistent_tool"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RunHTTPServer(ServerConfig{
				Version:      "test",
				Host:         "https://github.com",
				EnabledTools: tt.enabledTools,
			})

			require.ErrorIs(t, err, inventory.ErrUnknownTools)
			assert.ErrorContains(t, err, "failed to build inventory")
		})
	}
}

func TestNewOAuthConfig(t *testing.T) {
	tests := []struct {
		name                string
		authorizationServer string
	}{
		{
			name: "unset preserves host-derived authorization server",
		},
		{
			name:                "explicit override is propagated",
			authorizationServer: "https://oauth-proxy.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newOAuthConfig(ServerConfig{
				BaseURL:             "https://mcp.example.com",
				ResourcePath:        "/mcp",
				TrustProxyHeaders:   true,
				AuthorizationServer: tt.authorizationServer,
			})

			assert.Equal(t, &oauth.Config{
				BaseURL:             "https://mcp.example.com",
				ResourcePath:        "/mcp",
				TrustProxyHeaders:   true,
				AuthorizationServer: tt.authorizationServer,
			}, cfg)
		})
	}
}

func TestInitGlobalToolScopeMapUsesHost(t *testing.T) {
	tests := []struct {
		name     string
		hostType utils.HostType
		want     string
	}{
		{
			name:     "dotcom uses semantic search",
			hostType: utils.HostTypeDotcom,
			want:     "Search issues using natural-language semantic matching. Best for conceptual or paraphrased queries (e.g. \"login fails after password reset\"). Already scoped to is:issue.",
		},
		{
			name:     "GHES uses lexical search",
			hostType: utils.HostTypeGHES,
			want:     "Search for issues in GitHub repositories using issues search syntax already scoped to is:issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translations := make(map[string]string)
			translator := func(key, defaultValue string) string {
				if value, ok := translations[key]; ok {
					return value
				}
				translations[key] = defaultValue
				return defaultValue
			}

			require.NoError(t, initGlobalToolScopeMap(translator, tt.hostType))

			tool := github.SearchIssues(translator, github.WithHost(tt.hostType))
			assert.Equal(t, tt.want, tool.Tool.Description)
		})
	}
}

func TestCreateHTTPFeatureChecker(t *testing.T) {
	tests := []struct {
		name           string
		staticFeatures []string
		staticInsiders bool
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
			name:           "MCP Apps form deferral opt-out accepted from header",
			flagName:       github.MCPAppsDisableFormDeferralFeatureFlag,
			headerFeatures: []string{github.MCPAppsDisableFormDeferralFeatureFlag},
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
			name:         "insiders mode does not disable MCP Apps form deferral",
			flagName:     github.MCPAppsDisableFormDeferralFeatureFlag,
			insidersMode: true,
			wantEnabled:  false,
		},
		{
			name:           "static feature is enabled without header",
			staticFeatures: []string{github.FeatureFlagCSVOutput},
			flagName:       github.FeatureFlagCSVOutput,
			wantEnabled:    true,
		},
		{
			name:           "static features combine with header features",
			staticFeatures: []string{github.FeatureFlagCSVOutput},
			flagName:       github.FeatureFlagIssuesGranular,
			headerFeatures: []string{github.FeatureFlagIssuesGranular},
			wantEnabled:    true,
		},
		{
			name:           "static insiders enables insiders flags without route context",
			staticInsiders: true,
			flagName:       github.FeatureFlagCSVOutput,
			wantEnabled:    true,
		},
		{
			name:         "insiders mode does not auto-enable ifc labels",
			flagName:     github.FeatureFlagIFCLabels,
			insidersMode: true,
			wantEnabled:  false,
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
			checker := createHTTPFeatureChecker(tt.staticFeatures, tt.staticInsiders)
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

func TestResolveListenAddress(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{
			name: "empty host falls back to :port",
			host: "",
			port: 8082,
			want: ":8082",
		},
		{
			name: "ipv4 host is joined with port",
			host: "127.0.0.1",
			port: 9090,
			want: "127.0.0.1:9090",
		},
		{
			name: "ipv6 host is bracketed and joined with port",
			host: "::1",
			port: 9090,
			want: "[::1]:9090",
		},
		{
			name: "hostname is joined with port",
			host: "localhost",
			port: 8082,
			want: "localhost:8082",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveListenAddress(tt.host, tt.port)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfigureRequestState(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("missing key disables delete repository", func(t *testing.T) {
		cfg := &ServerConfig{}
		sealer, err := configureRequestState(cfg, logger)
		require.NoError(t, err)
		assert.Nil(t, sealer)
		assert.True(t, cfg.disableDeleteRepository)
	})

	t.Run("valid key configures sealer", func(t *testing.T) {
		cfg := &ServerConfig{
			MRTRStateKey: base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")),
		}
		sealer, err := configureRequestState(cfg, logger)
		require.NoError(t, err)
		require.NotNil(t, sealer)
		assert.False(t, cfg.disableDeleteRepository)

		token, err := sealer.Seal(context.Background(), []byte("state"))
		require.NoError(t, err)
		opened, err := sealer.Open(token)
		require.NoError(t, err)
		assert.Equal(t, []byte("state"), opened)
	})

	t.Run("malformed key fails", func(t *testing.T) {
		cfg := &ServerConfig{MRTRStateKey: "invalid"}
		_, err := configureRequestState(cfg, logger)
		require.ErrorContains(t, err, "invalid "+MRTRStateKeyEnv)
	})
}

func TestHeaderAllowedFeatureFlagsMatchesAllowed(t *testing.T) {
	// Ensure HeaderAllowedFeatureFlags delegates to AllowedFeatureFlags
	allowed := github.HeaderAllowedFeatureFlags()
	assert.Equal(t, github.AllowedFeatureFlags, allowed,
		"HeaderAllowedFeatureFlags() should match AllowedFeatureFlags")
	assert.NotEmpty(t, allowed, "AllowedFeatureFlags should not be empty")
}
