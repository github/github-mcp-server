package inventory

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProtocolVersionNonObjectOutputSchemas is the first MCP protocol revision that
// permits a tool's outputSchema to have a root other than `{"type":"object"}`,
// and correspondingly permits structuredContent to be any JSON value rather
// than only a JSON object (SEP-2106).
//
// Under 2025-11-25 the normative schema typed outputSchema as a closed shape:
//
//	outputSchema?: { $schema?: string; type: "object";
//	                 properties?: { [key: string]: object }; required?: string[]; }
//
// with the doc comment "Currently restricted to type: \"object\" at the root
// level." Both the restriction and the closed shape were removed in
// 2026-07-28, which types it as `{ $schema?: string; [key: string]: unknown }`
// and describes it as "any valid JSON Schema 2020-12".
const ProtocolVersionNonObjectOutputSchemas = "2026-07-28"

// HasObjectRootOutputSchema reports whether schema marshals to a JSON Schema
// whose root declares `"type": "object"`. Such a schema is legal under every
// protocol revision that supports outputSchema at all, so it needs no gating —
// even when it uses composition keywords like anyOf internally.
//
// A schema that fails to marshal, or that declares any other root (a bare
// anyOf, `"type": "array"`, a $ref) is reported as non-object-root and is
// therefore gated. Failing closed is deliberate: an unmarshalable schema
// should not be advertised to a client that may reject the whole tools/list.
func HasObjectRootOutputSchema(schema any) bool {
	if schema == nil {
		return false
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return false
	}
	var root struct {
		Type any `json:"type"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return false
	}
	// 2020-12 permits `"type"` to be an array of types. Only a bare "object"
	// satisfies the 2025-11-25 shape, so anything else is gated.
	t, ok := root.Type.(string)
	return ok && t == "object"
}

// OutputSchemaVersionGate returns receiving middleware that removes
// non-object-root output schemas from tools/list responses sent to clients
// speaking a protocol revision older than 2026-07-28.
//
// Object-root schemas pass through untouched at every version.
//
// The middleware copies on write. The SDK's listTools appends the *same*
// *mcp.Tool pointers it holds in its registry (mcp/server.go:936-939), so
// mutating a tool in place here would strip the schema from the server's
// stored definition and leak that to every later session on the same server.
// Only tools that actually need gating are copied.
func OutputSchemaVersionGate() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			res, err := next(ctx, method, req)
			if err != nil || method != "tools/list" {
				return res, err
			}
			listRes, ok := res.(*mcp.ListToolsResult)
			if !ok || listRes == nil {
				return res, err
			}
			listReq, ok := req.(*mcp.ListToolsRequest)
			if !ok {
				return res, err
			}
			// ProtocolVersion reads the per-request _meta for >= 2026-07-28
			// clients (which no longer send initialize at all, per SEP-2575)
			// and falls back to the session's InitializeParams for older ones.
			// An empty version means we could not determine it; gate in that
			// case, since only a client we know is new can be trusted with a
			// non-object root.
			if listReq.ProtocolVersion() >= ProtocolVersionNonObjectOutputSchemas {
				return res, err
			}
			listRes.Tools = stripNonObjectRootOutputSchemas(listRes.Tools)
			return listRes, err
		}
	}
}

// stripNonObjectRootOutputSchemas returns tools with non-object-root output
// schemas cleared, copying only the entries it changes and leaving the
// caller's slice untouched.
func stripNonObjectRootOutputSchemas(tools []*mcp.Tool) []*mcp.Tool {
	var out []*mcp.Tool
	for i, t := range tools {
		if t == nil || t.OutputSchema == nil || HasObjectRootOutputSchema(t.OutputSchema) {
			continue
		}
		if out == nil {
			out = make([]*mcp.Tool, len(tools))
			copy(out, tools)
		}
		toolCopy := *t
		toolCopy.OutputSchema = nil
		out[i] = &toolCopy
	}
	if out == nil {
		return tools
	}
	return out
}
