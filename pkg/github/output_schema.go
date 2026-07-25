package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/utils"
)

// MustOutputSchema infers an output schema for T, panicking during package
// initialization if inference fails.
//
// Unlike input schemas, an output schema root need not be `{"type":"object"}`:
// from protocol version 2026-07-28 (SEP-2106) it may be any valid JSON Schema
// 2020-12, including a bare array or a bare anyOf. Schemas whose root is not
// an object are stripped per-request for older clients — see
// inventory.OutputSchemaVersionGate — so inferring one here is safe.
func MustOutputSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		var zero T
		panic(fmt.Sprintf("failed to infer output schema for %T: %v", zero, err))
	}
	return schema
}

// AnyOfSchema builds a union output schema over the given branches.
//
// It deliberately emits `anyOf` and never `oneOf`. `oneOf` requires that
// EXACTLY ONE branch match, which is wrong for essentially every tool in this
// package whose output shape varies by method:
//
//   - Branches are frequently structurally identical. All four of
//     actions_run_trigger's non-run_workflow methods return the same
//     {message, run_id, status, status_code} map, so a oneOf over them matches
//     four branches and therefore always fails.
//   - Empty collections are ambiguous. issue_read method=get_comments on an
//     issue with no comments returns [], which vacuously satisfies every array
//     branch.
//   - Optional fields overlap. issue_read method=get on a sub-issue populates
//     `parent`, which also satisfies the get_parent branch.
//
// anyOf ("at least one") accepts all three while still rejecting values that
// match no branch, which is the useful half of the validation.
func AnyOfSchema(branches ...*jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{AnyOf: branches}
}

// MustRawOutputSchema wraps a hand-authored JSON Schema document, panicking
// during package initialization if it does not parse or does not resolve.
//
// Hand-authored schemas are kept as json.RawMessage rather than
// *jsonschema.Schema so they round-trip byte-for-byte to the client and
// produce deterministic toolsnaps output, independent of jsonschema-go's
// struct field coverage and marshaling order.
//
// Resolution is checked here (not just parsing) so a dangling $ref fails the
// build rather than the request.
func MustRawOutputSchema(raw string) json.RawMessage {
	var parsed jsonschema.Schema
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		panic(fmt.Sprintf("invalid output schema JSON: %v\nschema: %s", err, raw))
	}
	if _, err := parsed.Resolve(nil); err != nil {
		panic(fmt.Sprintf("output schema does not resolve: %v\nschema: %s", err, raw))
	}
	return json.RawMessage(raw)
}

// outputSchemasEnabled reports whether the output_schemas feature is on for
// this request. This is the rollout gate only; see canSendStructuredContent
// for the protocol-legality gate.
func outputSchemasEnabled(ctx context.Context, deps ToolDependencies) bool {
	return deps.IsFeatureEnabled(ctx, FeatureFlagOutputSchemas)
}

// isJSONObject reports whether the marshaled form of v is a JSON object.
func isJSONObject(marshaled []byte) bool {
	return bytes.HasPrefix(bytes.TrimLeft(marshaled, " \t\r\n"), []byte("{"))
}

// canSendStructuredContent reports whether a structuredContent value of the
// given marshaled shape may legally be sent to this client.
//
// Under 2025-11-25 and earlier, structuredContent is typed
// `{ [key: string]: unknown }` — a JSON object. 2026-07-28 widened it to
// `unknown`, explicitly "any JSON value (object, array, string, number,
// boolean, or null)". So an object is always safe; anything else requires the
// newer protocol. req may be nil in tests, in which case only objects are sent.
func canSendStructuredContent(req *mcp.CallToolRequest, marshaled []byte) bool {
	if isJSONObject(marshaled) {
		return true
	}
	if req == nil {
		return false
	}
	return req.ProtocolVersion() >= inventory.ProtocolVersionNonObjectOutputSchemas
}

// structuredTextResult builds a tool result whose text content is the
// serialized textValue — byte-identical to what the tool returned before
// output schemas existed — and which additionally carries structured as
// structuredContent when both gates allow it.
//
// The text block is always populated regardless of the gates. The spec calls
// for this independently: "For backwards compatibility, a tool that returns
// structured content SHOULD also return the serialized JSON in a TextContent
// block."
func structuredTextResult(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, textValue, structured any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(textValue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	result := utils.NewToolResultText(string(data))
	if structured == nil || !outputSchemasEnabled(ctx, deps) {
		return result, nil
	}

	structuredJSON, err := json.Marshal(structured)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal structured content: %w", err)
	}
	if !canSendStructuredContent(req, structuredJSON) {
		return result, nil
	}
	result.StructuredContent = structured
	return result, nil
}
