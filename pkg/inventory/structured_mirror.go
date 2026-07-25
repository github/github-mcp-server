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
//
// When omitRedundantText is set, the now-duplicated text block is dropped for
// clients new enough to read structuredContent, halving the response instead of
// doubling it. See dropRedundantTextContent.
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
// duplicates byte-for-byte.
//
// The spec's "a tool that returns structured content SHOULD also return the
// serialized JSON in a TextContent block" is a backwards-compatibility clause.
// The Go SDK says as much where it synthesises that block on its typed path:
// the fallback exists "so that pre-SEP-2106 clients can recover the structured
// payload from unstructured content" (mcp/server.go). A client that negotiated
// 2026-07-28 is not such a client, so for those the block is pure duplication —
// this server otherwise sends the same JSON twice, which works against
// csv_output, minimal_output and the fields param, all of which exist to make
// responses smaller.
//
// Callers must only reach here when structuredContent was mirrored from this
// exact text, so nothing is lost: the bytes survive, they just travel once.
//
// content stays present as an empty array rather than being unset — the draft
// schema still lists it in CallToolResult's required set, and the SDK
// normalises an empty slice to `[]` rather than `null`.
//
// This is opt-in (see RegisterToolOptions.OmitRedundantTextContent) rather than
// automatic, because negotiating 2026-07-28 does not prove a client actually
// reads structuredContent — there is no capability that says so, and a client
// that ignored it would see an empty result.
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
