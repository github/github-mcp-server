package inventory

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProtocolVersionNonObjectOutputSchemas is the first MCP protocol revision
// permitting a tool's outputSchema to have a root other than
// `{"type":"object"}`, and structuredContent to be any JSON value rather than
// only an object. Before it, the normative schema restricted outputSchema to
// "type: \"object\" at the root level" (SEP-2106 lifted both).
const ProtocolVersionNonObjectOutputSchemas = "2026-07-28"

// HasObjectRootOutputSchema reports whether schema's root declares
// `"type": "object"`, which needs no gating at any version — even when it uses
// anyOf internally. Anything else (a bare anyOf, an array, a $ref) is gated,
// as is a schema that fails to marshal: failing closed keeps an unparseable
// schema from breaking a client's whole tools/list.
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
// Copies on write: the SDK's listTools hands back the *same* *mcp.Tool
// pointers it holds in its registry, so mutating one here would strip the
// schema from the server's stored definition for every later session.
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
			// Reads the per-request _meta for >= 2026-07-28 clients (which no
			// longer send initialize at all, per SEP-2575), falling back to
			// the session's InitializeParams. Empty means undeterminable, so
			// it gates.
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
