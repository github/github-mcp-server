package github

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	stdfs "io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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

// TestNoDuplicateToolNames ensures all tools have unique names
func TestNoDuplicateToolNames(t *testing.T) {
	tools := AllTools(stubTranslation)
	seen := make(map[string]bool)
	featureFlagged := make(map[string]bool)

	// get_label is intentionally in both issues and labels toolsets for conformance
	// with original behavior where it was registered in both
	allowedDuplicates := map[string]bool{
		"get_label": true,
	}

	// First pass: identify tools that have feature flags (mutually exclusive at runtime)
	for _, tool := range tools {
		if tool.FeatureFlagEnable != "" || tool.FeatureFlagDisable != "" {
			featureFlagged[tool.Tool.Name] = true
		}
	}

	for _, tool := range tools {
		name := tool.Tool.Name
		// Allow duplicates for explicitly allowed tools and feature-flagged tools
		if !allowedDuplicates[name] && !featureFlagged[name] {
			assert.False(t, seen[name],
				"Duplicate tool name found: %q", name)
		}
		seen[name] = true
	}
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

// TestToolDefinitionsRequireExplicitReadOnlyHint ensures every registered tool is
// defined with an explicit Annotations.ReadOnlyHint value in source.
func TestToolDefinitionsRequireExplicitReadOnlyHint(t *testing.T) {
	tools := AllTools(stubTranslation)
	require.NotEmpty(t, tools, "AllTools should return at least one tool")

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "failed to determine current test file path")
	pkgDir := filepath.Dir(currentFile)

	fset := token.NewFileSet()
	checkedCount := 0
	var failures []string

	walkErr := filepath.Walk(pkgDir, func(path string, info stdfs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}

		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := lit.Type.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || pkgIdent.Name != "mcp" || sel.Sel.Name != "Tool" {
				return true
			}

			toolName, _ := toolNameFromToolLiteral(lit)
			checkedCount++
			if err := validateReadOnlyHintInToolLiteral(lit); err != nil {
				pos := fset.Position(lit.Pos())
				if toolName == "" {
					toolName = "<non-literal>"
				}
				failures = append(failures, pos.String()+" tool="+toolName+": "+err.Error())
			}
			return true
		})

		return nil
	})
	require.NoError(t, walkErr)
	require.Greater(t, checkedCount, 0, "expected to find at least one mcp.Tool literal")

	if len(failures) > 0 {
		sort.Strings(failures)
		t.Fatalf("tools with missing explicit ReadOnlyHint:\n%s", strings.Join(failures, "\n"))
	}
}

func toolNameFromToolLiteral(lit *ast.CompositeLit) (string, bool) {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Name" {
			continue
		}
		value, ok := kv.Value.(*ast.BasicLit)
		if !ok || value.Kind != token.STRING {
			return "", false
		}
		name, err := strconv.Unquote(value.Value)
		if err != nil {
			return "", false
		}
		return name, true
	}
	return "", false
}

func validateReadOnlyHintInToolLiteral(lit *ast.CompositeLit) error {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Annotations" {
			continue
		}

		annValue := kv.Value
		if unary, ok := annValue.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			annValue = unary.X
		}

		annLit, ok := annValue.(*ast.CompositeLit)
		if !ok {
			return errors.New("annotations field is not a literal")
		}

		for _, annElt := range annLit.Elts {
			annKV, ok := annElt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			annKey, ok := annKV.Key.(*ast.Ident)
			if ok && annKey.Name == "ReadOnlyHint" {
				return nil
			}
		}
		return errors.New("missing Annotations.ReadOnlyHint")
	}
	return errors.New("missing Annotations field")
}
