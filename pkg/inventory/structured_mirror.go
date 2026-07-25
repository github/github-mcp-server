package inventory

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mirrorStructuredContent wraps a tool handler so that a tool which declares an
// OutputSchema also returns structuredContent, without the handler having to
// produce it separately.
//
// This exploits the relationship the spec already defines between the two
// fields: "For backwards compatibility, a tool that returns structured content
// SHOULD also return the serialized JSON in a TextContent block." Every tool in
// this package already emits exactly that — one text block holding
// json.Marshal of the result — so the structured value is recoverable from it
// exactly, with no re-marshaling and therefore no risk of changing number
// formatting or key order.
//
// The mirrored value is the raw bytes of the text block, so content and
// structuredContent are byte-identical by construction and cannot drift.
//
// It applies only to tools that declared an OutputSchema. Tools without one are
// untouched, so the wire format of the other ~120 tools is unchanged.
func mirrorStructuredContent(next mcp.ToolHandler) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res, err := next(ctx, req)
		if err != nil || res == nil {
			return res, err
		}
		// Never attach structured content to an error result: outputSchema
		// describes the success shape, and a client validating an error
		// payload against it would fail.
		if res.IsError || res.StructuredContent != nil {
			return res, err
		}
		raw, ok := singleJSONTextBlock(res)
		if !ok {
			return res, err
		}
		// Under 2025-11-25 and earlier, structuredContent is typed as a JSON
		// object. Only send a non-object value to a client that can represent
		// it. The text block is unaffected either way, so nothing is lost.
		if !isJSONObjectBytes(raw) && req.ProtocolVersion() < ProtocolVersionNonObjectOutputSchemas {
			return res, err
		}
		res.StructuredContent = json.RawMessage(raw)
		return res, err
	}
}

// singleJSONTextBlock returns the bytes of the result's sole text content when
// that content is valid JSON. Results that are not a single JSON text block —
// raw diffs, logs, file contents, embedded resources, multi-part results — are
// left alone, since there is no structured value to mirror.
func singleJSONTextBlock(res *mcp.CallToolResult) ([]byte, bool) {
	if len(res.Content) != 1 {
		return nil, false
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, false
	}
	raw := []byte(text.Text)
	if !json.Valid(raw) {
		return nil, false
	}
	return raw, true
}

func isJSONObjectBytes(raw []byte) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return b == '{'
		}
	}
	return false
}
