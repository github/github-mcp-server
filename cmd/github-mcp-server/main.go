package main

import (
	"context"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cli/go-gh/pkg/auth"
	"github.com/github/github-mcp-server/pkg/github"
	iolog "github.com/github/github-mcp-server/pkg/log"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var version = "version"
var commit = "commit"
var date = "date"

var rootCommandName = "github-mcp-server"
var defaultTokenSource = "env"

var (
	rootCmd = &cobra.Command{
		Use:     rootCommandName,
		Short:   "GitHub MCP Server",
		Long:    `A GitHub MCP server that handles various tools and resources.`,
		Version: fmt.Sprintf("Version: %s\nCommit: %s\nBuild Date: %s", version, commit, date),
	}

	stdioCmd = &cobra.Command{
		Use:   "stdio",
		Short: "Start stdio server",
		Long:  `Start a server that communicates via standard input/output streams using JSON-RPC messages.`,
		Run: func(_ *cobra.Command, _ []string) {
			logFile := viper.GetString("log-file")
			readOnly := viper.GetBool("read-only")
			exportTranslations := viper.GetBool("export-translations")
			logger, err := initLogger(logFile)
			if err != nil {
				stdlog.Fatal("Failed to initialize logger:", err)
			}

			enabledToolsets := viper.GetStringSlice("toolsets")

			logCommands := viper.GetBool("enable-command-logging")
			cfg := runConfig{
				readOnly:           readOnly,
				logger:             logger,
				logCommands:        logCommands,
				exportTranslations: exportTranslations,
				enabledToolsets:    enabledToolsets,
			}
			if err := runStdioServer(cfg); err != nil {
				stdlog.Fatal("failed to run stdio server:", err)
			}
		},
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.SetVersionTemplate("{{.Short}}\n{{.Version}}\n")

	// Add global flags that will be shared by all commands
	rootCmd.PersistentFlags().StringSlice("toolsets", github.DefaultTools, "An optional comma separated list of groups of tools to allow, defaults to enabling all")
	rootCmd.PersistentFlags().Bool("dynamic-toolsets", false, "Enable dynamic toolsets")
	rootCmd.PersistentFlags().Bool("read-only", false, "Restrict the server to read-only operations")
	rootCmd.PersistentFlags().String("log-file", "", "Path to log file")
	rootCmd.PersistentFlags().Bool("enable-command-logging", false, "When enabled, the server will log all command requests and responses to the log file")
	rootCmd.PersistentFlags().Bool("export-translations", false, "Save translations to a JSON file")
	rootCmd.PersistentFlags().String("gh-host", "", "Specify the GitHub hostname (for GitHub Enterprise etc.)")
	rootCmd.PersistentFlags().String("token-source", defaultTokenSource, "Authentication token source (e.g. env, gh)")

	// Bind flag to viper
	_ = viper.BindPFlag("toolsets", rootCmd.PersistentFlags().Lookup("toolsets"))
	_ = viper.BindPFlag("dynamic_toolsets", rootCmd.PersistentFlags().Lookup("dynamic-toolsets"))
	_ = viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("enable-command-logging", rootCmd.PersistentFlags().Lookup("enable-command-logging"))
	_ = viper.BindPFlag("export-translations", rootCmd.PersistentFlags().Lookup("export-translations"))
	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("gh-host"))
	_ = viper.BindPFlag("token-source", rootCmd.PersistentFlags().Lookup("token-source"))

	// Add subcommands
	rootCmd.AddCommand(stdioCmd)
}

func initConfig() {
	// Initialize Viper configuration
	viper.SetEnvPrefix("github")
	viper.AutomaticEnv()
}

func initLogger(outPath string) (*log.Logger, error) {
	if outPath == "" {
		return log.New(), nil
	}

	file, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := log.New()
	logger.SetLevel(log.DebugLevel)
	logger.SetOutput(file)

	return logger, nil
}

type runConfig struct {
	readOnly           bool
	logger             *log.Logger
	logCommands        bool
	exportTranslations bool
	enabledToolsets    []string
}

func runStdioServer(cfg runConfig) error {
	// Create app context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create GH client
	ghClient, err := newGitHubClient()
	if err != nil {
		cfg.logger.Fatalf("failed to create GitHub client: %v", err)
	}

	t, dumpTranslations := translations.TranslationHelper()

	beforeInit := func(_ context.Context, _ any, message *mcp.InitializeRequest) {
		ghClient.UserAgent = fmt.Sprintf("github-mcp-server/%s (%s/%s)", version, message.Params.ClientInfo.Name, message.Params.ClientInfo.Version)
	}

	getClient := func(_ context.Context) (*gogithub.Client, error) {
		return ghClient, nil // closing over client
	}

	hooks := &server.Hooks{
		OnBeforeInitialize: []server.OnBeforeInitializeFunc{beforeInit},
	}
	// Create server
	ghServer := github.NewServer(version, server.WithHooks(hooks))

	enabled := cfg.enabledToolsets
	dynamic := viper.GetBool("dynamic_toolsets")
	if dynamic {
		// filter "all" from the enabled toolsets
		enabled = make([]string, 0, len(cfg.enabledToolsets))
		for _, toolset := range cfg.enabledToolsets {
			if toolset != "all" {
				enabled = append(enabled, toolset)
			}
		}
	}

	// Create default toolsets
	toolsets, err := github.InitToolsets(enabled, cfg.readOnly, getClient, t)
	context := github.InitContextToolset(getClient, t)

	if err != nil {
		stdlog.Fatal("Failed to initialize toolsets:", err)
	}

	// Register resources with the server
	github.RegisterResources(ghServer, getClient, t)
	// Register the tools with the server
	toolsets.RegisterTools(ghServer)
	context.RegisterTools(ghServer)

	if dynamic {
		dynamic := github.InitDynamicToolset(ghServer, toolsets, t)
		dynamic.RegisterTools(ghServer)
	}

	stdioServer := server.NewStdioServer(ghServer)

	stdLogger := stdlog.New(cfg.logger.Writer(), "stdioserver", 0)
	stdioServer.SetErrorLogger(stdLogger)

	if cfg.exportTranslations {
		// Once server is initialized, all translations are loaded
		dumpTranslations()
	}

	// Start listening for messages
	errC := make(chan error, 1)
	go func() {
		in, out := io.Reader(os.Stdin), io.Writer(os.Stdout)

		if cfg.logCommands {
			loggedIO := iolog.NewIOLogger(in, out, cfg.logger)
			in, out = loggedIO, loggedIO
		}

		errC <- stdioServer.Listen(ctx, in, out)
	}()

	// Output github-mcp-server string
	_, _ = fmt.Fprintf(os.Stderr, "GitHub MCP Server running on stdio\n")

	// Wait for shutdown signal
	select {
	case <-ctx.Done():
		cfg.logger.Infof("shutting down server...")
	case err := <-errC:
		if err != nil {
			return fmt.Errorf("error running server: %w", err)
		}
	}

	return nil
}

func getToken(host string) (string, error) {
	tokenSource := viper.GetString("token-source")
	switch tokenSource {
	case "env":
		token := os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN")
		if token == "" {
			return "", fmt.Errorf("GITHUB_PERSONAL_ACCESS_TOKEN not set")
		}
		return token, nil
	case "gh":
		token, source := auth.TokenForHost(host)
		if source == "default" {
			return "", fmt.Errorf("no token found for host: %s", host)
		}
		return token, nil
	}
	return "", fmt.Errorf("unknown token source: %s", tokenSource)
}

func getHost() string {
	host := os.Getenv("GH_HOST")
	if host == "" {
		host = viper.GetString("gh-host")
	}
	return host
}

func newGitHubClient() (*gogithub.Client, error) {
	host := getHost()
	token, err := getToken(host)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	ghClient := gogithub.NewClient(nil).WithAuthToken(token)
	if host != "" {
		ghClient, err = ghClient.WithEnterpriseURLs(host, host)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub client with host: %w", err)
		}
	}
	ghClient.UserAgent = fmt.Sprintf("github-mcp-server/%s", version)
	return ghClient, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
