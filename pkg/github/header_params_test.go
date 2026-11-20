package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
)

// TestAllToolsRoutingParamsGetHeaders enforces that every tool exposing a
// routing-relevant param (owner/repo, per inventory.HeaderParams) has it
// projected to an Mcp-Param-* header. This guards the per-request header
// optimization used by the remote proxy: a future tool must not silently ship
// without its owner/repo header, so it can never fall back to body re-parsing.
//
// App-only tools are the documented exception: their only callers are MCP App
// views, which cannot set request headers, so requiring projection on them
// would reject every view lookup (see issue #3834). They must instead stay
// unannotated, and this test enforces that they are.
func TestAllToolsRoutingParamsGetHeaders(t *testing.T) {
	inv, err := NewInventory(stubTranslator).WithToolsets([]string{"all"}).Build()
	require.NoError(t, err)

	tools := inv.AvailableTools(context.Background())
	require.NotEmpty(t, tools)

	checked := 0
	appOnlyChecked := 0
	for _, st := range tools {
		tool := st.Tool
		appOnly := inventory.IsAppOnlyTool(&tool)
		inventory.AnnotateHeaderParams(&tool)
		schema, ok := tool.InputSchema.(*jsonschema.Schema)
		if !ok || schema == nil {
			continue
		}
		if pathSchema := schema.Properties["path"]; pathSchema != nil {
			require.NotContainsf(t, pathSchema.Extra, "x-mcp-header",
				"tool %q path must remain in MCP arguments", tool.Name)
		}
		for prop, header := range inventory.HeaderParams {
			ps, ok := schema.Properties[prop]
			if !ok || ps == nil {
				continue
			}
			if appOnly {
				require.NotContainsf(t, ps.Extra, "x-mcp-header",
					"app-only tool %q param %q must not require an Mcp-Param-%s header",
					tool.Name, prop, header)
				appOnlyChecked++
				continue
			}
			require.NotNilf(t, ps.Extra, "tool %q param %q missing x-mcp-header annotation", tool.Name, prop)
			require.Equalf(t, header, ps.Extra["x-mcp-header"],
				"tool %q param %q must project to Mcp-Param-%s", tool.Name, prop, header)
			checked++
		}
	}
	require.Positive(t, checked, "expected at least one owner/repo param across all toolsets")
	require.Positive(t, appOnlyChecked, "expected at least one app-only owner/repo param across all toolsets")
}
