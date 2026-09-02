package github

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubTranslation is a simple translation function for testing
func stubTranslation(_, fallback string) string {
	return fallback
}

// TestAllToolsHaveRequiredMetadata validates that all tools have mandatory metadata:
// - Toolset must be set (non-empty ID)
// - ReadOnlyHint annotation must be explicitly set (not nil)
func TestAllToolsHaveRequiredMetadata(t *testing.T) {
	tools := AllTools(stubTranslation)

	require.NotEmpty(t, tools, "AllTools should return at least one tool")

	for _, tool := range tools {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			// Toolset ID must be set
			assert.NotEmpty(t, tool.Toolset.ID,
				"Tool %q must have a Toolset.ID", tool.Tool.Name)

			// Toolset description should be set for documentation
			assert.NotEmpty(t, tool.Toolset.Description,
				"Tool %q should have a Toolset.Description", tool.Tool.Name)

			// Annotations must exist and have ReadOnlyHint explicitly set
			require.NotNil(t, tool.Tool.Annotations,
				"Tool %q must have Annotations set (for ReadOnlyHint)", tool.Tool.Name)

			// We can't distinguish between "not set" and "set to false" for a bool,
			// but having Annotations non-nil confirms the developer thought about it.
			// The ReadOnlyHint value itself is validated by ensuring Annotations exist.
		})
	}
}

// TestAllToolInputSchemasAvoidTopLevelCombinators keeps the complete OSS tool
// inventory portable across provider JSON Schema subsets. Some providers reject
// an entire tools/list payload when any input schema has a top-level combinator,
// so cross-field constraints belong in handlers or below ordinary properties.
func TestAllToolInputSchemasAvoidTopLevelCombinators(t *testing.T) {
	tools := AllTools(stubTranslation)
	require.NotEmpty(t, tools, "AllTools should return at least one tool")

	for _, serverTool := range tools {
		tool := serverTool.Tool
		t.Run(tool.Name, func(t *testing.T) {
			data, err := json.Marshal(tool.InputSchema)
			require.NoError(t, err, "Tool %q InputSchema must marshal", tool.Name)

			var schema map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data, &schema), "Tool %q InputSchema must be a JSON object", tool.Name)
			assert.NotContains(t, schema, "anyOf", "Tool %q InputSchema must not use top-level anyOf", tool.Name)
			assert.NotContains(t, schema, "oneOf", "Tool %q InputSchema must not use top-level oneOf", tool.Name)
			assert.NotContains(t, schema, "allOf", "Tool %q InputSchema must not use top-level allOf", tool.Name)
		})
	}
}

// TestAllToolInputSchemasUseCanonicalPaginationNames keeps pagination properties
// spelled one way across the whole inventory. The pagination helpers read page,
// perPage, after and before, so a schema that advertises a case or underscore
// variant of one of those names promises a knob the handler never turns: whatever
// the client sends is dropped and the default is used instead. actions_list
// advertised per_page for months that way.
func TestAllToolInputSchemasUseCanonicalPaginationNames(t *testing.T) {
	canonical := map[string]string{
		"page":    "page",
		"perpage": "perPage",
		"after":   "after",
		"before":  "before",
	}

	tools := AllTools(stubTranslation)
	require.NotEmpty(t, tools, "AllTools should return at least one tool")

	for _, serverTool := range tools {
		tool := serverTool.Tool
		t.Run(tool.Name, func(t *testing.T) {
			data, err := json.Marshal(tool.InputSchema)
			require.NoError(t, err, "Tool %q InputSchema must marshal", tool.Name)

			var schema struct {
				Properties map[string]json.RawMessage `json:"properties"`
			}
			require.NoError(t, json.Unmarshal(data, &schema), "Tool %q InputSchema must be a JSON object", tool.Name)

			for name := range schema.Properties {
				want, ok := canonical[strings.ToLower(strings.ReplaceAll(name, "_", ""))]
				if !ok {
					continue
				}
				assert.Equal(t, want, name,
					"Tool %q advertises pagination property %q; the canonical spelling is %q", tool.Name, name, want)
			}
		})
	}
}

// TestAllResourcesHaveRequiredMetadata validates that all resources have mandatory metadata
func TestAllResourcesHaveRequiredMetadata(t *testing.T) {
	// Resources are now stateless - no client functions needed
	resources := AllResources(stubTranslation)

	require.NotEmpty(t, resources, "AllResources should return at least one resource")

	for _, res := range resources {
		t.Run(res.Template.Name, func(t *testing.T) {
			// Toolset ID must be set
			assert.NotEmpty(t, res.Toolset.ID,
				"Resource %q must have a Toolset.ID", res.Template.Name)

			// HandlerFunc must be set
			assert.True(t, res.HasHandler(),
				"Resource %q must have a HandlerFunc", res.Template.Name)
		})
	}
}

// TestAllPromptsHaveRequiredMetadata validates that all prompts have mandatory metadata
func TestAllPromptsHaveRequiredMetadata(t *testing.T) {
	prompts := AllPrompts(stubTranslation)

	require.NotEmpty(t, prompts, "AllPrompts should return at least one prompt")

	for _, prompt := range prompts {
		t.Run(prompt.Prompt.Name, func(t *testing.T) {
			// Toolset ID must be set
			assert.NotEmpty(t, prompt.Toolset.ID,
				"Prompt %q must have a Toolset.ID", prompt.Prompt.Name)

			// Handler must be set
			assert.NotNil(t, prompt.Handler,
				"Prompt %q must have a Handler", prompt.Prompt.Name)
		})
	}
}

// TestToolReadOnlyHintConsistency validates that read-only tools are correctly annotated
func TestToolReadOnlyHintConsistency(t *testing.T) {
	tools := AllTools(stubTranslation)

	for _, tool := range tools {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			require.NotNil(t, tool.Tool.Annotations,
				"Tool %q must have Annotations", tool.Tool.Name)

			// Verify IsReadOnly() method matches the annotation
			assert.Equal(t, tool.Tool.Annotations.ReadOnlyHint, tool.IsReadOnly(),
				"Tool %q: IsReadOnly() should match Annotations.ReadOnlyHint", tool.Tool.Name)
		})
	}
}

// TestNoDuplicateToolNames ensures duplicate names cannot be enabled together.
func TestNoDuplicateToolNames(t *testing.T) {
	tools := AllTools(stubTranslation)
	toolsByName := make(map[string][]inventory.ServerTool)
	for _, tool := range tools {
		toolsByName[tool.Tool.Name] = append(toolsByName[tool.Tool.Name], tool)
	}

	// get_label is intentionally in both issues and labels toolsets for conformance
	// with original behavior where it was registered in both
	for name, variants := range toolsByName {
		if name == "get_label" || len(variants) < 2 {
			continue
		}
		assert.False(t, featureDeclarationsOverlap(variants), "tool variants for %q can be enabled together", name)
	}
}

func featureDeclarationsOverlap(variants []inventory.ServerTool) bool {
	positions := make(map[inventory.FeatureFlag]uint)
	for _, variant := range variants {
		for _, feature := range variant.FeatureRule.Features() {
			if _, ok := positions[feature]; !ok {
				positions[feature] = uint(len(positions))
			}
		}
	}
	if len(positions) > 16 {
		return true
	}

	for assignment := range 1 << len(positions) {
		enabled := 0
		featureAsBool := func(feature inventory.FeatureFlag) bool {
			return assignment&(1<<positions[feature]) != 0
		}
		for _, variant := range variants {
			if variant.FeatureRule.IsZero() || variant.FeatureRule.Enabled(featureAsBool) {
				enabled++
			}
		}
		if enabled > 1 {
			return true
		}
	}
	return false
}

func TestFeatureRulesOverlap(t *testing.T) {
	flag := inventory.FeatureFlag("flag")
	enabled := inventory.ServerTool{FeatureRule: inventory.NewFeatureRule([]inventory.FeatureFlag{flag}, func(featureAsBool inventory.FeatureResolver) bool {
		return featureAsBool(flag)
	})}
	disabled := inventory.ServerTool{FeatureRule: inventory.NewFeatureRule([]inventory.FeatureFlag{flag}, func(featureAsBool inventory.FeatureResolver) bool {
		return !featureAsBool(flag)
	})}
	otherFlag := inventory.FeatureFlag("other")
	otherEnabled := inventory.ServerTool{FeatureRule: inventory.NewFeatureRule([]inventory.FeatureFlag{otherFlag}, func(featureAsBool inventory.FeatureResolver) bool {
		return featureAsBool(otherFlag)
	})}
	ungated := inventory.ServerTool{}

	assert.True(t, featureDeclarationsOverlap([]inventory.ServerTool{enabled, enabled}))
	assert.True(t, featureDeclarationsOverlap([]inventory.ServerTool{enabled, otherEnabled}))
	assert.True(t, featureDeclarationsOverlap([]inventory.ServerTool{ungated, enabled}))
	assert.False(t, featureDeclarationsOverlap([]inventory.ServerTool{enabled, disabled}))
}

func TestMCPAppsFeatureFlagMatchesInventory(t *testing.T) {
	inv, err := NewInventory(stubTranslation).Build()
	require.NoError(t, err)
	assert.Contains(t, inv.RequiredFeatures(), inventory.FeatureFlag(MCPAppsFeatureFlag))
}

// TestNoDuplicateResourceNames ensures all resources have unique names
func TestNoDuplicateResourceNames(t *testing.T) {
	resources := AllResources(stubTranslation)
	seen := make(map[string]bool)

	for _, res := range resources {
		name := res.Template.Name
		assert.False(t, seen[name],
			"Duplicate resource name found: %q", name)
		seen[name] = true
	}
}

// TestNoDuplicatePromptNames ensures all prompts have unique names
func TestNoDuplicatePromptNames(t *testing.T) {
	prompts := AllPrompts(stubTranslation)
	seen := make(map[string]bool)

	for _, prompt := range prompts {
		name := prompt.Prompt.Name
		assert.False(t, seen[name],
			"Duplicate prompt name found: %q", name)
		seen[name] = true
	}
}

// TestAllToolsHaveHandlerFunc ensures all tools have a handler function
func TestAllToolsHaveHandlerFunc(t *testing.T) {
	tools := AllTools(stubTranslation)

	for _, tool := range tools {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			assert.NotNil(t, tool.HandlerFunc,
				"Tool %q must have a HandlerFunc", tool.Tool.Name)
			assert.True(t, tool.HasHandler(),
				"Tool %q HasHandler() should return true", tool.Tool.Name)
		})
	}
}

// TestToolsetMetadataConsistency ensures tools in the same toolset have consistent descriptions
func TestToolsetMetadataConsistency(t *testing.T) {
	tools := AllTools(stubTranslation)
	toolsetDescriptions := make(map[inventory.ToolsetID]string)

	for _, tool := range tools {
		id := tool.Toolset.ID
		desc := tool.Toolset.Description

		if existing, ok := toolsetDescriptions[id]; ok {
			assert.Equal(t, existing, desc,
				"Toolset %q has inconsistent descriptions across tools", id)
		} else {
			toolsetDescriptions[id] = desc
		}
	}
}

func TestGitHubPackageDoesNotReadInsidersMode(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, file, nil, 0)
		require.NoError(t, err, "failed to parse %s", file)

		ast.Inspect(node, func(n ast.Node) bool {
			selector, ok := n.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "InsidersMode" {
				return true
			}

			position := fset.Position(selector.Sel.Pos())
			t.Errorf("%s reads InsidersMode directly; gate behavior on concrete feature flags instead", position)
			return true
		})
	}
}
