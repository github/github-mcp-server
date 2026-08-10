package inventory

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mirrorStructuredContent gives tools that declare an OutputSchema a
// structuredContent value, taken as the raw bytes of the serialized-JSON text
// block they already emit. Reusing those bytes rather than re-marshaling keeps
// the two fields byte-identical, so number formatting and key order cannot
// drift between them.
//
// Only tools declaring a schema are wrapped, leaving the rest unchanged.
//
// omitRedundantText additionally drops the now-duplicated text block; see
// dropRedundantTextContent.
func mirrorStructuredContent(next mcp.ToolHandler, omitRedundantText bool) mcp.ToolHandler {
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
		newEnough := req.ProtocolVersion() >= ProtocolVersionNonObjectOutputSchemas
		if !isJSONObjectBytes(raw) && !newEnough {
			return res, err
		}
		res.StructuredContent = json.RawMessage(raw)
		if omitRedundantText && newEnough {
			dropRedundantTextContent(res)
		}
		return res, err
	}
}

// dropRedundantTextContent removes the text block that structuredContent now
// duplicates byte-for-byte. The spec's "SHOULD also return the serialized JSON
// in a TextContent block" is a backwards-compatibility clause, so for a client
// that reads structuredContent the block is pure duplication.
//
// Callers must only reach here having mirrored from this exact text. Empty
// rather than unset: content is in CallToolResult's required set.
func dropRedundantTextContent(res *mcp.CallToolResult) {
	res.Content = []mcp.Content{}
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
