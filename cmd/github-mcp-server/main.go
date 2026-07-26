package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/github/github-mcp-server/internal/buildinfo"
	"github.com/github/github-mcp-server/internal/ghmcp"
	"github.com/github/github-mcp-server/internal/githubapp"
	"github.com/github/github-mcp-server/internal/oauth"
	"github.com/github/github-mcp-server/pkg/github"
	ghhttp "github.com/github/github-mcp-server/pkg/http"
	ghoauth "github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/utils"
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
			token := viper.GetString("personal_access_token")
			appID := viper.GetString("app-id")
			appInstallationID := viper.GetString("app-installation-id")
			appPrivateKeyPath := viper.GetString("app-private-key-path")
			appPrivateKeyInline := viper.GetString("app-private-key")
			appAuthRequested := appID != "" || appInstallationID != "" || appPrivateKeyPath != "" || appPrivateKeyInline != ""

			oauthClientID := viper.GetString("oauth-client-id")
			oauthClientSecret := viper.GetString("oauth-client-secret")
			// Fall back to the build-time baked-in client (official releases) when none is
			// configured explicitly. The baked-in app is registered on github.com, so it is
			// only applied to the default host; GHES/ghe.com users must bring their own
			// --oauth-client-id. Recognizing the host via NormalizeHost means an explicit
			// GITHUB_HOST=github.com (or api.github.com) still counts as the default and keeps
			// zero-config login working. The secret tracks the id, so an explicitly provided
			// id with no secret never picks up the baked-in secret.
			if oauthClientID == "" && !appAuthRequested && oauth.NormalizeHost(viper.GetString("host")) == "https://github.com" {
				oauthClientID = buildinfo.OAuthClientID
				oauthClientSecret = buildinfo.OAuthClientSecret
			}
			if token == "" && !appAuthRequested && oauthClientID == "" {
				return errors.New("authentication required: set GITHUB_PERSONAL_ACCESS_TOKEN, configure GitHub App auth, or pass --oauth-client-id to log in via OAuth")
			}
			if appAuthRequested && token != "" {
				return errors.New("GitHub App authentication and GITHUB_PERSONAL_ACCESS_TOKEN are mutually exclusive: set only one")
			}
			if appAuthRequested && oauthClientID != "" {
				return errors.New("GitHub App authentication and OAuth login (--oauth-client-id) are mutually exclusive: set only one")
			}

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

			// Parse excluded tools (similar to tools)
			var excludeTools []string
			if viper.IsSet("exclude_tools") {
				if err := viper.UnmarshalKey("exclude_tools", &excludeTools); err != nil {
					return fmt.Errorf("failed to unmarshal exclude-tools: %w", err)
				}
			}

			// Parse enabled features (similar to toolsets)
			var enabledFeatures []string
			if viper.IsSet("features") {
				if err := viper.UnmarshalKey("features", &enabledFeatures); err != nil {
					return fmt.Errorf("failed to unmarshal features: %w", err)
				}
			}

			ttl := viper.GetDuration("repo-access-cache-ttl")
			stdioServerConfig := ghmcp.StdioServerConfig{
				Version:              version,
				Host:                 viper.GetString("host"),
				Token:                token,
				EnabledToolsets:      enabledToolsets,
				EnabledTools:         enabledTools,
				EnabledFeatures:      enabledFeatures,
				ReadOnly:             viper.GetBool("read-only"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
				ContentWindowSize:    viper.GetInt("content-window-size"),
				LockdownMode:         viper.GetBool("lockdown-mode"),
				InsidersMode:         viper.GetBool("insiders"),
				ExcludeTools:         excludeTools,
				RepoAccessCacheTTL:   &ttl,
			}

			// When no static token is provided, log in via OAuth using the given
			// client. The requested scopes default to the full supported set
			// (which filters out no tools); an explicit, narrower --oauth-scopes
			// both narrows the grant and hides tools needing other scopes.
			if token == "" && !appAuthRequested {
				scopes := ghoauth.SupportedScopes
				if viper.IsSet("oauth-scopes") {
					if err := viper.UnmarshalKey("oauth-scopes", &scopes); err != nil {
						return fmt.Errorf("failed to unmarshal oauth-scopes: %w", err)
					}
				}
				oauthConfig := oauth.NewGitHubConfig(
					oauthClientID,
					oauthClientSecret,
					scopes,
					viper.GetString("host"),
					viper.GetInt("oauth-callback-port"),
				)
				stdioServerConfig.OAuthManager = oauth.NewManager(oauthConfig, nil)
				stdioServerConfig.OAuthScopes = scopes
			}

			if appAuthRequested {
				tokenProvider, err := newGitHubAppTokenProvider(appID, appInstallationID, appPrivateKeyPath, appPrivateKeyInline, viper.GetString("host"))
				if err != nil {
					return err
				}
				stdioServerConfig.TokenProvider = tokenProvider
			}

			return ghmcp.RunStdioServer(stdioServerConfig)
		},
	}

	httpCmd = &cobra.Command{
		Use:   "http",
		Short: "Start HTTP server",
		Long:  `Start an HTTP server that listens for MCP requests over HTTP.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// Parse toolsets (same approach as stdio — see comment there)
			var enabledToolsets []string
			if viper.IsSet("toolsets") {
				if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
					return fmt.Errorf("failed to unmarshal toolsets: %w", err)
				}
			}

			var enabledTools []string
			if viper.IsSet("tools") {
				if err := viper.UnmarshalKey("tools", &enabledTools); err != nil {
					return fmt.Errorf("failed to unmarshal tools: %w", err)
				}
			}

			var excludeTools []string
			if viper.IsSet("exclude_tools") {
				if err := viper.UnmarshalKey("exclude_tools", &excludeTools); err != nil {
					return fmt.Errorf("failed to unmarshal exclude-tools: %w", err)
				}
			}

			var enabledFeatures []string
			if viper.IsSet("features") {
				if err := viper.UnmarshalKey("features", &enabledFeatures); err != nil {
					return fmt.Errorf("failed to unmarshal features: %w", err)
				}
			}

			ttl := viper.GetDuration("repo-access-cache-ttl")
			httpConfig := ghhttp.ServerConfig{
				Version:              version,
				Host:                 viper.GetString("host"),
				Port:                 viper.GetInt("port"),
				ListenHost:           viper.GetString("listen-host"),
				BaseURL:              viper.GetString("base-url"),
				ResourcePath:         viper.GetString("base-path"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
				ContentWindowSize:    viper.GetInt("content-window-size"),
				LockdownMode:         viper.GetBool("lockdown-mode"),
				RepoAccessCacheTTL:   &ttl,
				ScopeChallenge:       viper.GetBool("scope-challenge"),
				ReadOnly:             viper.GetBool("read-only"),
				EnabledToolsets:      enabledToolsets,
				EnabledTools:         enabledTools,
				ExcludeTools:         excludeTools,
				EnabledFeatures:      enabledFeatures,
				InsidersMode:         viper.GetBool("insiders"),
				TrustProxyHeaders:    viper.GetBool("trust-proxy-headers"),
			}

			return ghhttp.RunHTTPServer(httpConfig)
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
	rootCmd.PersistentFlags().StringSlice("exclude-tools", nil, "Comma-separated list of tool names to disable regardless of other settings")
	rootCmd.PersistentFlags().StringSlice("features", nil, "Comma-separated list of feature flags to enable")
	rootCmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")
	rootCmd.PersistentFlags().String("log-file", "", "Path to log file")
	rootCmd.PersistentFlags().Bool("enable-command-logging", false, "When enabled, the server will log all command requests and responses to the log file")
	rootCmd.PersistentFlags().Bool("export-translations", false, "Save translations to a JSON file")
	rootCmd.PersistentFlags().String("gh-host", "", "Specify the GitHub hostname (for GitHub Enterprise etc.)")
	rootCmd.PersistentFlags().Int("content-window-size", 5000, "Specify the content window size")
	rootCmd.PersistentFlags().Bool("lockdown-mode", false, "Enable lockdown mode")
	rootCmd.PersistentFlags().Bool("insiders", false, "Enable insiders features")
	rootCmd.PersistentFlags().Duration("repo-access-cache-ttl", 5*time.Minute, "Override the repo access cache TTL (e.g. 1m, 0s to disable)")

	// stdio-specific OAuth flags. Provide --oauth-client-id (instead of a token)
	// to log in via the browser-based OAuth flow on first use. Works for both
	// OAuth Apps and GitHub Apps.
	stdioCmd.Flags().String("oauth-client-id", "", "OAuth App or GitHub App client ID, enabling interactive OAuth login when no token is set")
	stdioCmd.Flags().String("oauth-client-secret", "", "OAuth client secret, if the app requires one (it is a public, non-confidential credential for distributed clients)")
	stdioCmd.Flags().StringSlice("oauth-scopes", nil, "Comma-separated OAuth scopes to request; also filters tools to those scopes. Defaults to the full supported set")
	stdioCmd.Flags().Int("oauth-callback-port", 0, "Fixed local port for the OAuth callback server. Defaults to a random port; set a fixed port when mapping it through Docker")

	// The private key has no flag because passing it in argv would expose it.
	stdioCmd.Flags().String("app-id", "", "GitHub App ID or client ID, enabling non-interactive server-to-server authentication")
	stdioCmd.Flags().String("app-installation-id", "", "GitHub App installation ID to mint installation access tokens for")
	stdioCmd.Flags().String("app-private-key-path", "", "Path to the GitHub App private key (PEM). Preferred over GITHUB_APP_PRIVATE_KEY: keeps the key off the command line and out of the environment")

	// HTTP-specific flags
	httpCmd.Flags().Int("port", 8082, "HTTP server port")
	httpCmd.Flags().String("listen-host", "", "Host the HTTP server binds to (e.g. 127.0.0.1). Empty binds to all interfaces.")
	httpCmd.Flags().String("base-url", "", "Base URL where this server is publicly accessible (for OAuth resource metadata)")
	httpCmd.Flags().String("base-path", "", "Externally visible base path for the HTTP server (for OAuth resource metadata)")
	httpCmd.Flags().Bool("scope-challenge", false, "Enable OAuth scope challenge responses")
	httpCmd.Flags().Bool("trust-proxy-headers", false, "Honor X-Forwarded-Host and X-Forwarded-Proto when constructing OAuth resource metadata URLs. Only enable when the server is deployed behind a trusted proxy that sets these headers. Ignored when --base-url is set.")

	// Bind flag to viper
	_ = viper.BindPFlag("toolsets", rootCmd.PersistentFlags().Lookup("toolsets"))
	_ = viper.BindPFlag("tools", rootCmd.PersistentFlags().Lookup("tools"))
	_ = viper.BindPFlag("exclude_tools", rootCmd.PersistentFlags().Lookup("exclude-tools"))
	_ = viper.BindPFlag("features", rootCmd.PersistentFlags().Lookup("features"))
	_ = viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("enable-command-logging", rootCmd.PersistentFlags().Lookup("enable-command-logging"))
	_ = viper.BindPFlag("export-translations", rootCmd.PersistentFlags().Lookup("export-translations"))
	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("gh-host"))
	_ = viper.BindPFlag("content-window-size", rootCmd.PersistentFlags().Lookup("content-window-size"))
	_ = viper.BindPFlag("lockdown-mode", rootCmd.PersistentFlags().Lookup("lockdown-mode"))
	_ = viper.BindPFlag("insiders", rootCmd.PersistentFlags().Lookup("insiders"))
	_ = viper.BindPFlag("repo-access-cache-ttl", rootCmd.PersistentFlags().Lookup("repo-access-cache-ttl"))
	_ = viper.BindPFlag("oauth-client-id", stdioCmd.Flags().Lookup("oauth-client-id"))
	_ = viper.BindPFlag("oauth-client-secret", stdioCmd.Flags().Lookup("oauth-client-secret"))
	_ = viper.BindPFlag("oauth-scopes", stdioCmd.Flags().Lookup("oauth-scopes"))
	_ = viper.BindPFlag("oauth-callback-port", stdioCmd.Flags().Lookup("oauth-callback-port"))
	_ = viper.BindPFlag("app-id", stdioCmd.Flags().Lookup("app-id"))
	_ = viper.BindPFlag("app-installation-id", stdioCmd.Flags().Lookup("app-installation-id"))
	_ = viper.BindPFlag("app-private-key-path", stdioCmd.Flags().Lookup("app-private-key-path"))
	_ = viper.BindPFlag("port", httpCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("listen-host", httpCmd.Flags().Lookup("listen-host"))
	_ = viper.BindPFlag("base-url", httpCmd.Flags().Lookup("base-url"))
	_ = viper.BindPFlag("base-path", httpCmd.Flags().Lookup("base-path"))
	_ = viper.BindPFlag("scope-challenge", httpCmd.Flags().Lookup("scope-challenge"))
	_ = viper.BindPFlag("trust-proxy-headers", httpCmd.Flags().Lookup("trust-proxy-headers"))
	// Add subcommands
	rootCmd.AddCommand(stdioCmd)
	rootCmd.AddCommand(httpCmd)
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

func newGitHubAppTokenProvider(appID, installationID, keyPath, keyInline, host string) (func() string, error) {
	keyBytes, err := loadAppPrivateKey(keyPath, keyInline)
	if err != nil {
		return nil, err
	}

	apiHost, err := utils.NewAPIHost(host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host for GitHub App authentication: %w", err)
	}
	restURL, err := apiHost.BaseRESTURL(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve REST URL for GitHub App authentication: %w", err)
	}

	provider, err := githubapp.NewProvider(githubapp.Config{
		AppID:          appID,
		InstallationID: installationID,
		PrivateKeyPEM:  keyBytes,
		BaseRESTURL:    restURL.String(),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to configure GitHub App authentication: %w", err)
	}
	return provider.AccessToken, nil
}

func loadAppPrivateKey(path, inline string) ([]byte, error) {
	switch {
	case path != "":
		data, err := os.ReadFile(path) //#nosec G304 -- operator-supplied path to their own key
		if err != nil {
			return nil, fmt.Errorf("reading GitHub App private key file: %w", err)
		}
		return data, nil
	case inline != "":
		return []byte(strings.ReplaceAll(inline, `\n`, "\n")), nil
	default:
		return nil, errors.New("GitHub App authentication requires a private key: set GITHUB_APP_PRIVATE_KEY_PATH (preferred) or GITHUB_APP_PRIVATE_KEY")
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
