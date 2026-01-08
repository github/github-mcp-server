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
	fmt.Println("\n=== PAT Scope Comparison ===\n")

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

	// Calculate missing and extra scopes
	tokenScopesSet := make(map[string]bool)
	for _, scope := range tokenScopes {
		tokenScopesSet[scope] = true
	}

	// Expand token scopes to include child scopes they grant
	grantedScopes := expandScopeSet(tokenScopes)

	// Find missing required scopes (considering hierarchy)
	var missingScopes []string
	for _, scope := range requiredScopes {
		// Check if this scope is granted directly or via hierarchy
		found := false
		for acceptedScope := range acceptedScopesSet {
			if strings.HasPrefix(acceptedScope, scope) || scope == acceptedScope {
				if grantedScopes[acceptedScope] {
					found = true
					break
				}
			}
		}
		
		// Also check if any token scope grants this required scope
		if !found {
			for tokenScope := range tokenScopesSet {
				// Check if this token scope is in the accepted scopes for this required scope
				if tokenScope == scope || grantedScopes[scope] {
					found = true
					break
				}
			}
		}
		
		if !found && !tokenScopesSet[scope] && !grantedScopes[scope] {
			// Check if any accepted scope that matches this required scope is granted
			hasAcceptedEquivalent := false
			for _, tool := range allTools {
				hasThisRequired := false
				for _, rs := range tool.RequiredScopes {
					if rs == scope {
						hasThisRequired = true
						break
					}
				}
				if hasThisRequired {
					// Check if token has any of the accepted scopes for this tool
					for _, as := range tool.AcceptedScopes {
						if tokenScopesSet[as] || grantedScopes[as] {
							hasAcceptedEquivalent = true
							break
						}
					}
				}
				if hasAcceptedEquivalent {
					break
				}
			}
			if !hasAcceptedEquivalent {
				missingScopes = append(missingScopes, scope)
			}
		}
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
	fmt.Println("\n=== Comparison Summary ===\n")

	if len(missingScopes) > 0 {
		fmt.Println("Missing Scopes (required by tools but not granted to token):")
		for _, scope := range missingScopes {
			fmt.Printf("  - %s\n", scope)
			// Show which tools require this scope
			var toolsNeedingScope []string
			for _, tool := range allTools {
				for _, rs := range tool.RequiredScopes {
					if rs == scope {
						toolsNeedingScope = append(toolsNeedingScope, tool.Tool.Name)
						break
					}
				}
			}
			if len(toolsNeedingScope) > 0 {
				fmt.Printf("    Tools affected: %s\n", strings.Join(toolsNeedingScope, ", "))
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

// expandScopeSet returns a set of all scopes granted by the given scopes,
// including child scopes from the hierarchy.
func expandScopeSet(scopeList []string) map[string]bool {
	expanded := make(map[string]bool, len(scopeList))
	for _, scope := range scopeList {
		expanded[scope] = true
		// Add child scopes granted by this scope
		if children, ok := scopes.ScopeHierarchy[scopes.Scope(scope)]; ok {
			for _, child := range children {
				expanded[string(child)] = true
			}
		}
	}
	return expanded
}
