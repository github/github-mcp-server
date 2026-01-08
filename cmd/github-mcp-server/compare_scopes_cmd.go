package main

import (
	"context"
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

var compareScopesCmd = &cobra.Command{
	Use:   "compare-scopes",
	Short: "Compare PAT scopes with required scopes for MCP tools",
	Long: `Compare the scopes granted to your Personal Access Token (PAT) with the scopes
required by the GitHub MCP server tools. This helps identify missing permissions
that would prevent certain tools from working.

The PAT is provided via the GITHUB_PERSONAL_ACCESS_TOKEN environment variable.
Use --gh-host to specify a GitHub Enterprise Server host.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return compareScopes()
	},
}

func init() {
	rootCmd.AddCommand(compareScopesCmd)
}

func compareScopes() error {
	// Get the token from environment
	token := viper.GetString("personal_access_token")
	if token == "" {
		return fmt.Errorf("GITHUB_PERSONAL_ACCESS_TOKEN environment variable is not set")
	}

	// Get the API host
	apiHost := viper.GetString("host")
	if apiHost == "" {
		apiHost = "https://api.github.com"
	} else if !strings.HasPrefix(apiHost, "http://") && !strings.HasPrefix(apiHost, "https://") {
		apiHost = "https://" + apiHost
	}

	// Fetch the PAT's scopes
	ctx := context.Background()
	fetcher := scopes.NewFetcher(scopes.FetcherOptions{
		APIHost: apiHost,
	})

	fmt.Fprintf(os.Stderr, "Fetching token scopes from %s...\n", apiHost)
	tokenScopes, err := fetcher.FetchTokenScopes(ctx, token)
	if err != nil {
		return fmt.Errorf("failed to fetch token scopes: %w", err)
	}

	// Get all required scopes from the inventory
	t, _ := translations.TranslationHelper()
	inventory := github.NewInventory(t).WithToolsets([]string{"all"}).Build()

	allTools := inventory.AllTools()

	// Collect unique required and accepted scopes
	requiredScopesSet := make(map[string]bool)
	acceptedScopesSet := make(map[string]bool)

	for _, tool := range allTools {
		for _, scope := range tool.RequiredScopes {
			requiredScopesSet[scope] = true
		}
		for _, scope := range tool.AcceptedScopes {
			acceptedScopesSet[scope] = true
		}
	}

	// Convert to sorted slices
	var requiredScopes []string
	for scope := range requiredScopesSet {
		requiredScopes = append(requiredScopes, scope)
	}
	sort.Strings(requiredScopes)

	var acceptedScopes []string
	for scope := range acceptedScopesSet {
		acceptedScopes = append(acceptedScopes, scope)
	}
	sort.Strings(acceptedScopes)

	// Sort token scopes
	sort.Strings(tokenScopes)

	// Print results
	fmt.Println("\n=== PAT Scope Comparison ===")
	fmt.Println()

	// Show token scopes
	fmt.Println("Token Scopes:")
	if len(tokenScopes) == 0 {
		fmt.Println("  (none - this may be a fine-grained PAT which doesn't expose scopes)")
	} else {
		for _, scope := range tokenScopes {
			fmt.Printf("  - %s\n", scope)
		}
	}

	// Show required scopes
	fmt.Println("\nRequired Scopes (by tools):")
	if len(requiredScopes) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, scope := range requiredScopes {
			fmt.Printf("  - %s\n", scope)
		}
	}

	// Calculate missing scopes - check each tool to see if token has required permissions
	tokenScopesSet := make(map[string]bool)
	for _, scope := range tokenScopes {
		tokenScopesSet[scope] = true
	}

	// Track which tools are missing scopes and collect unique missing scopes
	missingScopesSet := make(map[string]bool)
	toolsMissingScopes := make(map[string][]string) // scope -> list of affected tools

	for _, tool := range allTools {
		// Skip tools that don't require any scopes
		if len(tool.AcceptedScopes) == 0 {
			continue
		}

		// Use the existing HasRequiredScopes function which handles hierarchy correctly
		if !scopes.HasRequiredScopes(tokenScopes, tool.AcceptedScopes) {
			// This tool is not usable - track which required scopes are missing
			for _, reqScope := range tool.RequiredScopes {
				missingScopesSet[reqScope] = true
				toolsMissingScopes[reqScope] = append(toolsMissingScopes[reqScope], tool.Tool.Name)
			}
		}
	}

	// Convert to sorted slice
	var missingScopes []string
	for scope := range missingScopesSet {
		missingScopes = append(missingScopes, scope)
	}
	sort.Strings(missingScopes)

	// Find extra scopes (scopes in token but not required)
	var extraScopes []string
	for _, scope := range tokenScopes {
		if !requiredScopesSet[scope] && !acceptedScopesSet[scope] {
			extraScopes = append(extraScopes, scope)
		}
	}
	sort.Strings(extraScopes)

	// Print comparison summary
	fmt.Println("\n=== Comparison Summary ===")
	fmt.Println()

	if len(missingScopes) > 0 {
		fmt.Println("Missing Scopes (required by tools but not granted to token):")
		for _, scope := range missingScopes {
			fmt.Printf("  - %s\n", scope)
			// Show which tools require this scope
			if tools, ok := toolsMissingScopes[scope]; ok && len(tools) > 0 {
				// Limit to first 5 tools to avoid overwhelming output
				displayTools := tools
				if len(displayTools) > 5 {
					displayTools = tools[:5]
					fmt.Printf("    Tools affected: %s, ... and %d more\n", strings.Join(displayTools, ", "), len(tools)-5)
				} else {
					fmt.Printf("    Tools affected: %s\n", strings.Join(displayTools, ", "))
				}
			}
		}
		fmt.Println("\nWarning: Some tools may not be available due to missing scopes.")
		return fmt.Errorf("token is missing %d required scope(s)", len(missingScopes))
	}

	fmt.Println("✓ Token has all required scopes")

	if len(extraScopes) > 0 {
		fmt.Println("\nExtra Scopes (granted to token but not required by any tool):")
		for _, scope := range extraScopes {
			fmt.Printf("  - %s\n", scope)
		}
	}

	return nil
}
