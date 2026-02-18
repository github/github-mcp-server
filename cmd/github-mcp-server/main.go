package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/github/github-mcp-server/internal/ghmcp"
	"github.com/github/github-mcp-server/pkg/github"
	ghhttp "github.com/github/github-mcp-server/pkg/http"
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
			if token == "" {
				return errors.New("GITHUB_PERSONAL_ACCESS_TOKEN not set")
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

			// Parse disallowed tools (similar to tools)
			var disallowedTools []string
			if viper.IsSet("disallowed_tools") {
				if err := viper.UnmarshalKey("disallowed_tools", &disallowedTools); err != nil {
					return fmt.Errorf("failed to unmarshal disallowed-tools: %w", err)
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
				DynamicToolsets:      viper.GetBool("dynamic_toolsets"),
				ReadOnly:             viper.GetBool("read-only"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
				ContentWindowSize:    viper.GetInt("content-window-size"),
				LockdownMode:         viper.GetBool("lockdown-mode"),
				InsidersMode:         viper.GetBool("insiders"),
				DisallowedTools:      disallowedTools,
				RepoAccessCacheTTL:   &ttl,
			}
			return ghmcp.RunStdioServer(stdioServerConfig)
		},
	}

	httpCmd = &cobra.Command{
		Use:   "http",
		Short: "Start HTTP server",
		Long:  `Start an HTTP server that listens for MCP requests over HTTP.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			ttl := viper.GetDuration("repo-access-cache-ttl")
			httpConfig := ghhttp.ServerConfig{
				Version:              version,
				Host:                 viper.GetString("host"),
				Port:                 viper.GetInt("port"),
				BaseURL:              viper.GetString("base-url"),
				ResourcePath:         viper.GetString("base-path"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
				ContentWindowSize:    viper.GetInt("content-window-size"),
				LockdownMode:         viper.GetBool("lockdown-mode"),
				RepoAccessCacheTTL:   &ttl,
				ScopeChallenge:       viper.GetBool("scope-challenge"),
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
	rootCmd.PersistentFlags().StringSlice("disallowed-tools", nil, "Comma-separated list of tool names to disable regardless of other settings")
	rootCmd.PersistentFlags().StringSlice("features", nil, "Comma-separated list of feature flags to enable")
	rootCmd.PersistentFlags().Bool("dynamic-toolsets", false, "Enable dynamic toolsets")
	rootCmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")
	rootCmd.PersistentFlags().String("log-file", "", "Path to log file")
	rootCmd.PersistentFlags().Bool("enable-command-logging", false, "When enabled, the server will log all command requests and responses to the log file")
	rootCmd.PersistentFlags().Bool("export-translations", false, "Save translations to a JSON file")
	rootCmd.PersistentFlags().String("gh-host", "", "Specify the GitHub hostname (for GitHub Enterprise etc.)")
	rootCmd.PersistentFlags().Int("content-window-size", 5000, "Specify the content window size")
	rootCmd.PersistentFlags().Bool("lockdown-mode", false, "Enable lockdown mode")
	rootCmd.PersistentFlags().Bool("insiders", false, "Enable insiders features")
	rootCmd.PersistentFlags().Duration("repo-access-cache-ttl", 5*time.Minute, "Override the repo access cache TTL (e.g. 1m, 0s to disable)")

	// HTTP-specific flags
	httpCmd.Flags().Int("port", 8082, "HTTP server port")
	httpCmd.Flags().String("base-url", "", "Base URL where this server is publicly accessible (for OAuth resource metadata)")
	httpCmd.Flags().String("base-path", "", "Externally visible base path for the HTTP server (for OAuth resource metadata)")
	httpCmd.Flags().Bool("scope-challenge", false, "Enable OAuth scope challenge responses")

	// Bind flag to viper
	_ = viper.BindPFlag("toolsets", rootCmd.PersistentFlags().Lookup("toolsets"))
	_ = viper.BindPFlag("tools", rootCmd.PersistentFlags().Lookup("tools"))
	_ = viper.BindPFlag("disallowed_tools", rootCmd.PersistentFlags().Lookup("disallowed-tools"))
	_ = viper.BindPFlag("features", rootCmd.PersistentFlags().Lookup("features"))
	_ = viper.BindPFlag("dynamic_toolsets", rootCmd.PersistentFlags().Lookup("dynamic-toolsets"))
	_ = viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("enable-command-logging", rootCmd.PersistentFlags().Lookup("enable-command-logging"))
	_ = viper.BindPFlag("export-translations", rootCmd.PersistentFlags().Lookup("export-translations"))
	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("gh-host"))
	_ = viper.BindPFlag("content-window-size", rootCmd.PersistentFlags().Lookup("content-window-size"))
	_ = viper.BindPFlag("lockdown-mode", rootCmd.PersistentFlags().Lookup("lockdown-mode"))
	_ = viper.BindPFlag("insiders", rootCmd.PersistentFlags().Lookup("insiders"))
	_ = viper.BindPFlag("repo-access-cache-ttl", rootCmd.PersistentFlags().Lookup("repo-access-cache-ttl"))
	_ = viper.BindPFlag("port", httpCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("base-url", httpCmd.Flags().Lookup("base-url"))
	_ = viper.BindPFlag("base-path", httpCmd.Flags().Lookup("base-path"))
	_ = viper.BindPFlag("scope-challenge", httpCmd.Flags().Lookup("scope-challenge"))
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

func wordSepNormalizeFunc(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	from := []string{"_"}
	to := "-"
	for _, sep := range from {
		name = strings.ReplaceAll(name, sep, to)
	}
	return pflag.NormalizedName(name)
}
