package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
)

// TestAllToolsRoutingParamsGetHeaders enforces that every tool exposing a
// routing-relevant param (owner/repo, per inventory.HeaderParams) has it
// projected to an Mcp-Param-* header. This guards the per-request header
// optimization used by the remote proxy: a future tool must not silently ship
// without its owner/repo header, so it can never fall back to body re-parsing.
func TestAllToolsRoutingParamsGetHeaders(t *testing.T) {
	inv, err := NewInventory(stubTranslator).WithToolsets([]string{"all"}).Build()
	require.NoError(t, err)

	tools := inv.AvailableTools(context.Background())
	require.NotEmpty(t, tools)

	checked := 0
	for _, st := range tools {
		tool := st.Tool
		inventory.AnnotateHeaderParams(&tool)
		schema, ok := tool.InputSchema.(*jsonschema.Schema)
		if !ok || schema == nil {
			continue
		}
		for prop, header := range inventory.HeaderParams {
			ps, ok := schema.Properties[prop]
			if !ok || ps == nil {
				continue
			}
			require.NotNilf(t, ps.Extra, "tool %q param %q missing x-mcp-header annotation", tool.Name, prop)
			require.Equalf(t, header, ps.Extra["x-mcp-header"],
				"tool %q param %q must project to Mcp-Param-%s", tool.Name, prop, header)
			checked++
		}
	}
	require.Positive(t, checked, "expected at least one owner/repo param across all toolsets")
}
