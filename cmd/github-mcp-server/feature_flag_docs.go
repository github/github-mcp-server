package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

// generateInsidersFeaturesDocs refreshes the auto-generated section of
// docs/insiders-features.md with the tools and schemas affected by each
// Insiders feature flag.
func generateInsidersFeaturesDocs(docsPath string) error {
	body := generateFlaggedToolsDoc(github.InsidersFeatureFlags, "_No Insiders-only tool changes._")
	return rewriteAutomatedSection(docsPath, "START AUTOMATED INSIDERS TOOLS", "END AUTOMATED INSIDERS TOOLS", body)
}

// generateFeatureFlagsDocs refreshes the auto-generated section of
// docs/feature-flags.md with the tools and schemas affected by each
// user-controllable feature flag.
func generateFeatureFlagsDocs(docsPath string) error {
	body := generateFlaggedToolsDoc(github.AllowedFeatureFlags, "_No user-controllable feature flags affect tool registration._")
	return rewriteAutomatedSection(docsPath, "START AUTOMATED FEATURE FLAG TOOLS", "END AUTOMATED FEATURE FLAG TOOLS", body)
}

// generateFlaggedToolsDoc renders, for each flag in the input set, the tools
// whose registration or definition differs from the default user experience.
// Each affected tool is printed with its full schema using the same writer
// used by the README so the output style stays consistent.
func generateFlaggedToolsDoc(flags []string, emptyMessage string) string {
	t, _ := translations.TranslationHelper()
	defaultTools := indexToolsByName(buildInventoryWithFlags(t, nil).ToolsForRegistration(context.Background()))

	var buf strings.Builder
	hasAny := false

	for _, flag := range flags {
		affected := flaggedToolDiff(t, flag, defaultTools)
		if len(affected) == 0 {
			continue
		}

		if hasAny {
			buf.WriteString("\n\n")
		}
		hasAny = true

		fmt.Fprintf(&buf, "### `%s`\n\n", flag)
		for i, tool := range affected {
			writeToolDoc(&buf, tool)
			if i < len(affected)-1 {
				buf.WriteString("\n\n")
			}
		}
	}

	if !hasAny {
		return emptyMessage
	}
	// Clarify scope semantics for the rendered tools: every listed required
	// scope is needed (AND), and a higher scope in the hierarchy also satisfies
	// a required scope.
	preamble := "> **OAuth scopes:** all listed required scopes are needed (AND). A higher scope in the hierarchy (e.g. `admin:org` for `read:org`, `repo` for `public_repo`) also satisfies a required scope.\n\n"
	// Leading/trailing newlines around the body produce blank lines between
	// our content and the surrounding marker comments, so the trailing comment
	// doesn't get absorbed into the final list item by markdown renderers.
	return "\n" + preamble + strings.TrimSuffix(buf.String(), "\n") + "\n"
}

// flaggedToolDiff returns the tools whose definition (input schema or meta)
// differs from the default-flagged inventory when only the given flag is on,
// plus tools that exist only in the flag-on inventory. Results are sorted by
// tool name.
func flaggedToolDiff(t translations.TranslationHelperFunc, flag string, defaultTools map[string]inventory.ServerTool) []inventory.ServerTool {
	flagTools := buildInventoryWithFlags(t, map[string]bool{flag: true}).ToolsForRegistration(context.Background())

	out := make([]inventory.ServerTool, 0)
	seen := make(map[string]struct{}, len(flagTools))

	for _, tool := range flagTools {
		if _, ok := seen[tool.Tool.Name]; ok {
			continue
		}
		seen[tool.Tool.Name] = struct{}{}

		baseline, hadBaseline := defaultTools[tool.Tool.Name]
		if hadBaseline && reflect.DeepEqual(tool.Tool.InputSchema, baseline.Tool.InputSchema) && reflect.DeepEqual(tool.Tool.Meta, baseline.Tool.Meta) {
			continue
		}
		out = append(out, tool)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Tool.Name < out[j].Tool.Name })
	return out
}

// buildInventoryWithFlags constructs an inventory whose feature checker treats
// the given flags as enabled and every other flag as disabled. Passing nil
// produces the default-flagged inventory.
func buildInventoryWithFlags(t translations.TranslationHelperFunc, enabled map[string]bool) *inventory.Inventory {
	checker := func(_ context.Context, flag string) (bool, error) {
		return enabled[flag], nil
	}
	inv, _ := github.NewInventory(t).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(checker).
		Build()
	return inv
}

// indexToolsByName returns a map keyed by tool name. When duplicates exist
// (e.g. flag-gated dual registrations), the first occurrence wins, mirroring
// AvailableTools' deterministic sort order.
func indexToolsByName(tools []inventory.ServerTool) map[string]inventory.ServerTool {
	out := make(map[string]inventory.ServerTool, len(tools))
	for _, tool := range tools {
		if _, ok := out[tool.Tool.Name]; ok {
			continue
		}
		out[tool.Tool.Name] = tool
	}
	return out
}

// rewriteAutomatedSection reads a markdown file, replaces the content between
// the named markers with body, and writes it back.
func rewriteAutomatedSection(path, startMarker, endMarker, body string) error {
	content, err := os.ReadFile(path) //#nosec G304
	if err != nil {
		return fmt.Errorf("failed to read docs file: %w", err)
	}
	updated, err := replaceSection(string(content), startMarker, endMarker, body)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(updated), 0600) //#nosec G306
}
