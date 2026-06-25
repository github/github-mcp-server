package binding

import (
	"fmt"

	"github.com/github/github-mcp-server/pkg/inventory"
)

// ApplyTools transforms the full tool universe into the scoped surface for the
// bound context. Tools admitted by the context's manifest are bound (schema
// pruned, description rewritten, handler wrapped); every other tool is dropped.
//
// The result is a []inventory.ServerTool that can be handed to
// inventory.Builder.SetTools exactly like the unscoped universe, so all
// downstream filtering (read-only, feature flags, toolsets, PAT scopes) runs
// unchanged on top of the scoped set.
func ApplyTools(universe []inventory.ServerTool, ctx Context) ([]inventory.ServerTool, error) {
	m, ok := ManifestFor(ctx.Kind)
	if !ok {
		return nil, fmt.Errorf("no manifest for scope kind %q", ctx.Kind)
	}

	out := make([]inventory.ServerTool, 0, len(m.Admit))
	for _, st := range universe {
		tb, admitted := m.Admit[st.Tool.Name]
		if !admitted {
			continue
		}
		bound, err := bindTool(st, tb, ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, bound)
	}
	return out, nil
}

// ApplyResources scopes resource templates. Resource templates carry their own
// owner/repo context and cannot yet be bound safely, so v1 drops them entirely
// in every scoped mode.
func ApplyResources(_ []inventory.ServerResourceTemplate, _ Context) []inventory.ServerResourceTemplate {
	return nil
}

// ApplyPrompts scopes prompts. Prompts may take owner/repo arguments and are
// dropped in v1 scoped modes for the same reason as resource templates.
func ApplyPrompts(_ []inventory.ServerPrompt, _ Context) []inventory.ServerPrompt {
	return nil
}
