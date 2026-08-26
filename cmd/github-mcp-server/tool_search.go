package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/tooldiscovery"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type toolSearchConfig struct {
	enabledToolsets []string
	enabledTools    []string
	excludedTools   []string
	enabledFeatures []string
	host            string
	readOnly        bool
	insiders        bool
}

var toolSearchCmd = &cobra.Command{
	Use:   "tool-search <query>",
	Short: "Search enabled tools",
	Long:  "Search enabled tools by name, description, and input parameter names.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		maxResults, err := cmd.Flags().GetInt("max-results")
		if err != nil {
			return fmt.Errorf("failed to read max-results: %w", err)
		}
		if maxResults < 1 {
			return fmt.Errorf("max-results must be greater than zero")
		}

		return runToolSearch(cmd.Context(), cmd.OutOrStdout(), args[0], maxResults)
	},
}

func init() {
	toolSearchCmd.Flags().Int("max-results", tooldiscovery.DefaultMaxSearchResults, "Maximum number of matching tools to return")
	rootCmd.AddCommand(toolSearchCmd)
}

func runToolSearch(ctx context.Context, output io.Writer, query string, maxResults int) error {
	cfg, err := loadToolSearchConfig()
	if err != nil {
		return err
	}

	translator, _ := translations.TranslationHelper()
	results, err := searchConfiguredTools(ctx, query, maxResults, cfg, translator)
	if err != nil {
		return err
	}

	return writeToolSearchResults(output, results)
}

func loadToolSearchConfig() (toolSearchConfig, error) {
	enabledToolsets, err := toolSearchStringSlice("toolsets")
	if err != nil {
		return toolSearchConfig{}, err
	}
	enabledTools, err := toolSearchStringSlice("tools")
	if err != nil {
		return toolSearchConfig{}, err
	}
	excludedTools, err := toolSearchStringSlice("exclude_tools")
	if err != nil {
		return toolSearchConfig{}, err
	}
	enabledFeatures, err := toolSearchStringSlice("features")
	if err != nil {
		return toolSearchConfig{}, err
	}

	return toolSearchConfig{
		enabledToolsets: enabledToolsets,
		enabledTools:    enabledTools,
		excludedTools:   excludedTools,
		enabledFeatures: enabledFeatures,
		host:            viper.GetString("host"),
		readOnly:        viper.GetBool("read-only"),
		insiders:        viper.GetBool("insiders"),
	}, nil
}

func toolSearchStringSlice(key string) ([]string, error) {
	if !viper.IsSet(key) {
		return nil, nil
	}

	var values []string
	if err := viper.UnmarshalKey(key, &values); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", strings.ReplaceAll(key, "_", "-"), err)
	}
	return values, nil
}

func searchConfiguredTools(
	ctx context.Context,
	query string,
	maxResults int,
	cfg toolSearchConfig,
	translator translations.TranslationHelperFunc,
) ([]tooldiscovery.SearchResult, error) {
	hostType, err := utils.ParseHostType(cfg.host)
	if err != nil {
		return nil, fmt.Errorf("failed to classify API host: %w", err)
	}

	enabledFeatures := github.ResolveFeatureFlags(cfg.enabledFeatures, cfg.insiders)
	inventoryBuilder := github.NewInventory(translator, github.WithHost(hostType)).
		WithDeprecatedAliases(github.DeprecatedToolAliases).
		WithReadOnly(cfg.readOnly).
		WithToolsets(github.ResolvedEnabledToolsets(cfg.enabledToolsets, cfg.enabledTools)).
		WithTools(github.CleanTools(cfg.enabledTools)).
		WithExcludeTools(cfg.excludedTools).
		WithFeatureChecker(func(_ context.Context, flagName string) (bool, error) {
			return enabledFeatures[flagName], nil
		})

	inv, err := inventoryBuilder.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build inventory: %w", err)
	}

	serverTools := inv.AvailableTools(ctx)
	tools := make([]mcp.Tool, len(serverTools))
	for i, serverTool := range serverTools {
		tools[i] = serverTool.Tool
	}

	results, err := tooldiscovery.SearchTools(tools, query, tooldiscovery.SearchOptions{MaxResults: maxResults})
	if err != nil {
		return nil, fmt.Errorf("failed to search tools: %w", err)
	}
	return results, nil
}

func writeToolSearchResults(output io.Writer, results []tooldiscovery.SearchResult) error {
	if len(results) == 0 {
		_, err := fmt.Fprintln(output, "No matching tools found.")
		return err
	}

	noun := "tools"
	if len(results) == 1 {
		noun = "tool"
	}
	if _, err := fmt.Fprintf(output, "Found %d matching %s:\n", len(results), noun); err != nil {
		return err
	}

	toolName := color.New(color.FgCyan, color.Bold)
	for _, result := range results {
		if _, err := fmt.Fprintln(output); err != nil {
			return err
		}
		if _, err := toolName.Fprintln(output, result.Tool.Name); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, result.Tool.Description); err != nil {
			return err
		}
	}

	return nil
}
