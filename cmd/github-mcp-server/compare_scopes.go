package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ScopeComparison represents the result of comparing token scopes with required scopes.
type ScopeComparison struct {
	TokenScopes    []string `json:"token_scopes"`
	RequiredScopes []string `json:"required_scopes"`
	MissingScopes  []string `json:"missing_scopes"`
	ExtraScopes    []string `json:"extra_scopes"`
	HasAllRequired bool     `json:"has_all_required"`
}

// CompareOutput is the full output structure for the compare-scopes command.
type CompareOutput struct {
	Comparison      ScopeComparison     `json:"comparison"`
	EnabledToolsets []string            `json:"enabled_toolsets"`
	ReadOnly        bool                `json:"read_only"`
	Tools           []ToolScopeInfo     `json:"tools,omitempty"`
	ScopesByTool    map[string][]string `json:"scopes_by_tool,omitempty"`
}

var compareScopesCmd = &cobra.Command{
	Use:   "compare-scopes",
	Short: "Compare PAT token scopes with required scopes",
	Long: `Compare the OAuth scopes granted to a PAT token with the scopes required by enabled tools.

This command fetches the scopes from your GitHub Personal Access Token and compares
them with the scopes required by the enabled tools. It reports any missing or extra
scopes to help you understand if your token has the necessary permissions.

The token is read from the GITHUB_PERSONAL_ACCESS_TOKEN environment variable or
can be provided via the --token flag.

The output format can be controlled with the --output flag:
  - text (default): Human-readable text output with colored indicators
  - json: JSON output for programmatic use

Examples:
  # Compare using token from environment variable
  export GITHUB_PERSONAL_ACCESS_TOKEN=ghp_...
  github-mcp-server compare-scopes

  # Compare for specific toolsets
  github-mcp-server compare-scopes --toolsets=repos,issues,pull_requests

  # Compare with token from flag
  github-mcp-server compare-scopes --token=ghp_...

  # Output as JSON
  github-mcp-server compare-scopes --output=json`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runCompareScopes()
	},
}

func init() {
	compareScopesCmd.Flags().StringP("output", "o", "text", "Output format: text or json")
	compareScopesCmd.Flags().String("token", "", "GitHub Personal Access Token (overrides GITHUB_PERSONAL_ACCESS_TOKEN env var)")
	_ = viper.BindPFlag("compare-scopes-output", compareScopesCmd.Flags().Lookup("output"))
	_ = viper.BindPFlag("compare-scopes-token", compareScopesCmd.Flags().Lookup("token"))

	rootCmd.AddCommand(compareScopesCmd)
}

func runCompareScopes() error {
	// Get token from flag or environment variable
	token := viper.GetString("compare-scopes-token")
	if token == "" {
		token = viper.GetString("personal_access_token")
	}
	if token == "" {
		return fmt.Errorf("GitHub Personal Access Token not provided. Set GITHUB_PERSONAL_ACCESS_TOKEN or use --token flag")
	}

	// Get toolsets configuration (same logic as list-scopes)
	var enabledToolsets []string
	if viper.IsSet("toolsets") {
		if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
			return fmt.Errorf("failed to unmarshal toolsets: %w", err)
		}
	}

	// Get specific tools
	var enabledTools []string
	if viper.IsSet("tools") {
		if err := viper.UnmarshalKey("tools", &enabledTools); err != nil {
			return fmt.Errorf("failed to unmarshal tools: %w", err)
		}
	}

	readOnly := viper.GetBool("read-only")
	outputFormat := viper.GetString("compare-scopes-output")

	// Get API host for GitHub Enterprise support
	apiHost := viper.GetString("host")
	if apiHost != "" {
		// Ensure it starts with https://
		if !strings.HasPrefix(apiHost, "http://") && !strings.HasPrefix(apiHost, "https://") {
			apiHost = "https://" + apiHost
		}
		// GitHub Enterprise uses /api/v3 endpoint
		if !strings.Contains(apiHost, "api.github.com") {
			apiHost = strings.TrimSuffix(apiHost, "/") + "/api/v3"
		}
	}

	// Fetch token scopes from GitHub API
	ctx := context.Background()
	var tokenScopes []string
	var err error

	if apiHost == "" {
		tokenScopes, err = scopes.FetchTokenScopes(ctx, token)
	} else {
		tokenScopes, err = scopes.FetchTokenScopesWithHost(ctx, token, apiHost)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch token scopes: %w", err)
	}

	// Build inventory to get required scopes
	t, _ := translations.TranslationHelper()
	inventoryBuilder := github.NewInventory(t).
		WithReadOnly(readOnly)

	if enabledToolsets != nil {
		inventoryBuilder = inventoryBuilder.WithToolsets(enabledToolsets)
	}
	if len(enabledTools) > 0 {
		inventoryBuilder = inventoryBuilder.WithTools(enabledTools)
	}

	inv := inventoryBuilder.Build()

	// Collect tool scopes
	scopesOutput := collectToolScopes(inv, readOnly)

	// Compare scopes
	comparison := compareScopes(tokenScopes, scopesOutput.UniqueScopes)

	// Create output structure
	output := CompareOutput{
		Comparison:      comparison,
		EnabledToolsets: scopesOutput.EnabledToolsets,
		ReadOnly:        readOnly,
		Tools:           scopesOutput.Tools,
		ScopesByTool:    scopesOutput.ScopesByTool,
	}

	// Output based on format
	switch outputFormat {
	case "json":
		return outputCompareJSON(output)
	default:
		return outputCompareText(output)
	}
}

func compareScopes(tokenScopes, requiredScopes []string) ScopeComparison {
	// Create sets for efficient lookup
	tokenSet := make(map[string]bool)
	for _, scope := range tokenScopes {
		tokenSet[scope] = true
	}

	requiredSet := make(map[string]bool)
	for _, scope := range requiredScopes {
		requiredSet[scope] = true
	}

	// Find missing scopes (required but not in token)
	var missingScopes []string
	for _, scope := range requiredScopes {
		// Use scope hierarchy to check if token has equivalent parent scope
		if !scopes.HasRequiredScopes(tokenScopes, []string{scope}) {
			missingScopes = append(missingScopes, scope)
		}
	}

	// Find extra scopes (in token but not covering any required scope)
	// A token scope is "extra" only if:
	// 1. It's not directly in the required set, AND
	// 2. None of the required scopes would be satisfied by this token scope, AND
	// 3. This token scope is not a child of any required scope
	var extraScopes []string
	for _, tokenScope := range tokenScopes {
		if requiredSet[tokenScope] {
			// Directly required, not extra
			continue
		}

		// Check if this token scope covers any required scope
		coversAnyRequired := false
		for _, reqScope := range requiredScopes {
			// Check if tokenScope would satisfy reqScope
			if scopes.HasRequiredScopes([]string{tokenScope}, []string{reqScope}) {
				coversAnyRequired = true
				break
			}
		}

		// Check if this token scope is covered by any required scope (i.e., it's a subset)
		isCoveredByRequired := false
		for _, reqScope := range requiredScopes {
			// Check if reqScope would satisfy tokenScope
			if scopes.HasRequiredScopes([]string{reqScope}, []string{tokenScope}) {
				isCoveredByRequired = true
				break
			}
		}

		// Only mark as extra if it doesn't cover any required scope AND isn't covered by any required scope
		if !coversAnyRequired && !isCoveredByRequired {
			extraScopes = append(extraScopes, tokenScope)
		}
	}

	sort.Strings(missingScopes)
	sort.Strings(extraScopes)

	return ScopeComparison{
		TokenScopes:    tokenScopes,
		RequiredScopes: requiredScopes,
		MissingScopes:  missingScopes,
		ExtraScopes:    extraScopes,
		HasAllRequired: len(missingScopes) == 0,
	}
}

func outputCompareJSON(output CompareOutput) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputCompareText(output CompareOutput) error {
	fmt.Println("PAT Scope Comparison")
	fmt.Println("====================")
	fmt.Println()

	comparison := output.Comparison

	// Token scopes section
	fmt.Println("Token Scopes:")
	if len(comparison.TokenScopes) == 0 {
		fmt.Println("  (no scopes - might be a fine-grained PAT)")
	} else {
		for _, scope := range comparison.TokenScopes {
			fmt.Printf("  • %s\n", scope)
		}
	}
	fmt.Println()

	// Required scopes section
	fmt.Println("Required Scopes:")
	if len(comparison.RequiredScopes) == 0 {
		fmt.Println("  (no scopes required)")
	} else {
		for _, scope := range comparison.RequiredScopes {
			fmt.Printf("  • %s\n", formatScopeDisplay(scope))
		}
	}
	fmt.Println()

	// Comparison result
	fmt.Println("Comparison Result:")
	if comparison.HasAllRequired {
		fmt.Println("  ✓ Token has all required scopes!")
	} else {
		fmt.Println("  ✗ Token is missing required scopes")
	}
	fmt.Println()

	// Missing scopes
	if len(comparison.MissingScopes) > 0 {
		fmt.Println("Missing Scopes (need to add):")
		for _, scope := range comparison.MissingScopes {
			fmt.Printf("  ✗ %s\n", formatScopeDisplay(scope))
		}
		fmt.Println()
	}

	// Extra scopes
	if len(comparison.ExtraScopes) > 0 {
		fmt.Println("Extra Scopes (not required but granted):")
		for _, scope := range comparison.ExtraScopes {
			fmt.Printf("  • %s\n", scope)
		}
		fmt.Println()
	}

	// Configuration info
	fmt.Printf("Configuration: %d toolset(s), read-only=%v\n", len(output.EnabledToolsets), output.ReadOnly)
	fmt.Printf("Toolsets: %s\n", strings.Join(output.EnabledToolsets, ", "))

	return nil
}
