package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/github/github-mcp-server/internal/buildinfo"
	"github.com/github/github-mcp-server/internal/ghmcp"
	"github.com/github/github-mcp-server/internal/oauth"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// These variables are set by the build process using ldflags.
var version = "version"
var commit = "commit"
var date = "date"

var (
	rootCmd = &cobra.Command{
		Use:     "server",
		Short:   "GitHub MCP Server",
		Long:    `A GitHub MCP server that handles various tools and resources.`,
		Version: fmt.Sprintf("Version: %s\nCommit: %s\nBuild Date: %s", version, commit, date),
	}

	stdioCmd = &cobra.Command{
		Use:   "stdio",
		Short: "Start stdio server",
		Long:  `Start a server that communicates via standard input/output streams using JSON-RPC messages.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// If you're wondering why we're not using viper.GetStringSlice("toolsets"),
			// it's because viper doesn't handle comma-separated values correctly for env
			// vars when using GetStringSlice.
			// https://github.com/spf13/viper/issues/380
			//
			// Additionally, viper.UnmarshalKey returns an empty slice even when the flag
			// is not set, but we need nil to indicate "use defaults". So we check IsSet first.
			var enabledToolsets []string
			if viper.IsSet("toolsets") {
				if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
					return fmt.Errorf("failed to unmarshal toolsets: %w", err)
				}
			}
			// else: enabledToolsets stays nil, meaning "use defaults"

			// Parse tools (similar to toolsets)
			var enabledTools []string
			if viper.IsSet("tools") {
				if err := viper.UnmarshalKey("tools", &enabledTools); err != nil {
					return fmt.Errorf("failed to unmarshal tools: %w", err)
				}
			}

			// Parse enabled features (similar to toolsets)
			var enabledFeatures []string
			if viper.IsSet("features") {
				if err := viper.UnmarshalKey("features", &enabledFeatures); err != nil {
					return fmt.Errorf("failed to unmarshal features: %w", err)
				}
			}

			token := viper.GetString("personal_access_token")
			var oauthMgr *oauth.Manager
			var oauthScopes []string
			var prebuiltInventory *inventory.Inventory

			// If no token provided, setup OAuth manager
			// Priority: 1. Explicit OAuth config, 2. Build-time credentials, 3. None
			if token == "" {
				oauthClientID, oauthClientSecret := resolveOAuthCredentials()
				if oauthClientID != "" {
					// Get translation helper for inventory building
					t, _ := translations.TranslationHelper()

					// Compute OAuth scopes and get inventory (avoids double building)
					scopesResult := getOAuthScopes(enabledToolsets, enabledTools, enabledFeatures, t)
					oauthScopes = scopesResult.scopes
					prebuiltInventory = scopesResult.inventory

					// Create OAuth manager for lazy authentication
					oauthCfg := oauth.GetGitHubOAuthConfig(
						oauthClientID,
						oauthClientSecret,
						oauthScopes,
						viper.GetString("host"),
						viper.GetInt("oauth_callback_port"),
					)
					oauthMgr = oauth.NewManager(oauthCfg)
					fmt.Fprintf(os.Stderr, "OAuth configured - will prompt for authentication when needed\n")
				} else {
					fmt.Fprintf(os.Stderr, "Warning: No authentication configured\n")
					fmt.Fprintf(os.Stderr, "  - Set GITHUB_PERSONAL_ACCESS_TOKEN, or\n")
					fmt.Fprintf(os.Stderr, "  - Configure OAuth with --oauth-client-id\n")
					fmt.Fprintf(os.Stderr, "Tools will prompt for authentication when called\n")
				}
			}

			ttl := viper.GetDuration("repo-access-cache-ttl")
			stdioServerConfig := ghmcp.StdioServerConfig{
				Version:              version,
				Host:                 viper.GetString("host"),
				Token:                token,
				OAuthManager:         oauthMgr,
				OAuthScopes:          oauthScopes,
				PrebuiltInventory:    prebuiltInventory,
				EnabledToolsets:      enabledToolsets,
				EnabledTools:         enabledTools,
				EnabledFeatures:      enabledFeatures,
				DynamicToolsets:      viper.GetBool("dynamic_toolsets"),
				ReadOnly:             viper.GetBool("read-only"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
				ContentWindowSize:    viper.GetInt("content-window-size"),
				LockdownMode:         viper.GetBool("lockdown-mode"),
				InsiderMode:          viper.GetBool("insider-mode"),
				RepoAccessCacheTTL:   &ttl,
			}
			return ghmcp.RunStdioServer(stdioServerConfig)
		},
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.SetGlobalNormalizationFunc(wordSepNormalizeFunc)

	rootCmd.SetVersionTemplate("{{.Short}}\n{{.Version}}\n")

	// Add global flags that will be shared by all commands
	rootCmd.PersistentFlags().StringSlice("toolsets", nil, github.GenerateToolsetsHelp())
	rootCmd.PersistentFlags().StringSlice("tools", nil, "Comma-separated list of specific tools to enable")
	rootCmd.PersistentFlags().StringSlice("features", nil, "Comma-separated list of feature flags to enable")
	rootCmd.PersistentFlags().Bool("dynamic-toolsets", false, "Enable dynamic toolsets")
	rootCmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")
	rootCmd.PersistentFlags().String("log-file", "", "Path to log file")
	rootCmd.PersistentFlags().Bool("enable-command-logging", false, "When enabled, the server will log all command requests and responses to the log file")
	rootCmd.PersistentFlags().Bool("export-translations", false, "Save translations to a JSON file")
	rootCmd.PersistentFlags().String("gh-host", "", "Specify the GitHub hostname (for GitHub Enterprise etc.)")
	rootCmd.PersistentFlags().Int("content-window-size", 5000, "Specify the content window size")
	rootCmd.PersistentFlags().Bool("lockdown-mode", false, "Enable lockdown mode")
	rootCmd.PersistentFlags().Bool("insider-mode", false, "Enable insider features")
	rootCmd.PersistentFlags().Duration("repo-access-cache-ttl", 5*time.Minute, "Override the repo access cache TTL (e.g. 1m, 0s to disable)")

	// OAuth flags (stdio mode only)
	rootCmd.PersistentFlags().String("oauth-client-id", "", "GitHub OAuth app client ID (enables interactive OAuth flow if token not set)")
	rootCmd.PersistentFlags().String("oauth-client-secret", "", "GitHub OAuth app client secret (recommended)")
	rootCmd.PersistentFlags().StringSlice("oauth-scopes", nil, "OAuth scopes to request (comma-separated)")
	rootCmd.PersistentFlags().Int("oauth-callback-port", 0, "Fixed port for OAuth callback (0 for random, required for Docker with -p flag)")

	// Bind flag to viper
	_ = viper.BindPFlag("toolsets", rootCmd.PersistentFlags().Lookup("toolsets"))
	_ = viper.BindPFlag("tools", rootCmd.PersistentFlags().Lookup("tools"))
	_ = viper.BindPFlag("features", rootCmd.PersistentFlags().Lookup("features"))
	_ = viper.BindPFlag("dynamic_toolsets", rootCmd.PersistentFlags().Lookup("dynamic-toolsets"))
	_ = viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("enable-command-logging", rootCmd.PersistentFlags().Lookup("enable-command-logging"))
	_ = viper.BindPFlag("export-translations", rootCmd.PersistentFlags().Lookup("export-translations"))
	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("gh-host"))
	_ = viper.BindPFlag("content-window-size", rootCmd.PersistentFlags().Lookup("content-window-size"))
	_ = viper.BindPFlag("lockdown-mode", rootCmd.PersistentFlags().Lookup("lockdown-mode"))
	_ = viper.BindPFlag("insider-mode", rootCmd.PersistentFlags().Lookup("insider-mode"))
	_ = viper.BindPFlag("repo-access-cache-ttl", rootCmd.PersistentFlags().Lookup("repo-access-cache-ttl"))
	_ = viper.BindPFlag("oauth_client_id", rootCmd.PersistentFlags().Lookup("oauth-client-id"))
	_ = viper.BindPFlag("oauth_client_secret", rootCmd.PersistentFlags().Lookup("oauth-client-secret"))
	_ = viper.BindPFlag("oauth_scopes", rootCmd.PersistentFlags().Lookup("oauth-scopes"))
	_ = viper.BindPFlag("oauth_callback_port", rootCmd.PersistentFlags().Lookup("oauth-callback-port"))

	// Add subcommands
	rootCmd.AddCommand(stdioCmd)
}

func initConfig() {
	// Initialize Viper configuration
	viper.SetEnvPrefix("github")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func wordSepNormalizeFunc(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	from := []string{"_"}
	to := "-"
	for _, sep := range from {
		name = strings.ReplaceAll(name, sep, to)
	}
	return pflag.NormalizedName(name)
}

// oauthScopesResult holds the result of OAuth scope computation
type oauthScopesResult struct {
	scopes    []string
	inventory *inventory.Inventory // reused inventory to avoid double building
}

// getOAuthScopes returns the OAuth scopes to request based on enabled tools
// Also returns the built inventory to avoid building it twice
// Uses custom scopes if explicitly provided, otherwise computes required scopes
// from the tools that will be enabled based on user configuration
func getOAuthScopes(enabledToolsets, enabledTools, enabledFeatures []string, t translations.TranslationHelperFunc) oauthScopesResult {
	// Allow explicit override via --oauth-scopes flag
	var scopeList []string
	if viper.IsSet("oauth_scopes") {
		if err := viper.UnmarshalKey("oauth_scopes", &scopeList); err == nil && len(scopeList) > 0 {
			// When scopes are explicit, don't build inventory (will be built in server)
			return oauthScopesResult{scopes: scopeList}
		}
	}

	// Build inventory with the same configuration that will be used at runtime
	// This allows us to determine which tools will actually be available
	// and avoids building the inventory twice
	inventoryBuilder := github.NewStandardBuilder(github.InventoryConfig{
		Translator:      t,
		ReadOnly:        viper.GetBool("read-only"),
		Toolsets:        enabledToolsets,
		Tools:           enabledTools,
		EnabledFeatures: enabledFeatures,
	})

	inv, err := inventoryBuilder.Build()
	if err != nil {
		// Inventory build only fails if invalid tool names are passed via --tools
		// In that case, return empty scopes - the error will surface when server starts
		return oauthScopesResult{scopes: nil}
	}

	// Collect all required scopes from available tools
	// This is the canonical source of OAuth scopes for the enabled tools
	requiredScopes := collectRequiredScopes(inv)
	return oauthScopesResult{scopes: requiredScopes, inventory: inv}
}

// collectRequiredScopes collects all unique required scopes from available tools
// Returns a sorted, deduplicated list of OAuth scopes needed for the enabled tools
func collectRequiredScopes(inv *inventory.Inventory) []string {
	scopeSet := make(map[string]bool)

	// Get available tools (respects filters like read-only, toolsets, etc.)
	for _, tool := range inv.AvailableTools(context.Background()) {
		for _, scope := range tool.RequiredScopes {
			if scope != "" {
				scopeSet[scope] = true
			}
		}
	}

	// Convert to sorted slice for deterministic output
	scopes := make([]string, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	return scopes
}

// resolveOAuthCredentials returns OAuth client credentials using the following priority:
// 1. Explicit configuration via flags/environment (--oauth-client-id, GITHUB_OAUTH_CLIENT_ID)
// 2. Build-time baked credentials (for official releases)
//
// This allows developers to override with their own OAuth app while providing
// a seamless "just works" experience for end users of official builds.
func resolveOAuthCredentials() (clientID, clientSecret string) {
	// Priority 1: Explicit user configuration
	clientID = viper.GetString("oauth_client_id")
	if clientID != "" {
		return clientID, viper.GetString("oauth_client_secret")
	}

	// Priority 2: Build-time baked credentials
	if buildinfo.HasOAuthCredentials() {
		return buildinfo.OAuthClientID, buildinfo.OAuthClientSecret
	}

	// No OAuth credentials available
	return "", ""
}
