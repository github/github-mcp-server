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
// Unlike input schemas, an output schema root need not be `{"type":"object"}`,
// so T may be a slice or a scalar. Non-object roots are stripped per-request
// for pre-2026-07-28 clients by inventory.OutputSchemaVersionGate.
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
// Always anyOf, never oneOf: oneOf requires EXACTLY ONE branch to match, and
// these unions have structurally identical branches, empty arrays that satisfy
// every array branch, and overlapping optional fields. anyOf still rejects
// values matching no branch, which is the useful half of the validation.
// TestAnyOfAcceptsAmbiguousPayloadsButStillRejectsGarbage demonstrates both.
func AnyOfSchema(branches ...*jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{AnyOf: branches}
}

// MustRawOutputSchema wraps a hand-authored JSON Schema document as raw bytes,
// so it reaches the client exactly as written rather than through
// jsonschema-go's struct coverage and field ordering.
//
// Resolution is checked, not just parsing, so a dangling $ref fails the build
// rather than a request.
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

// outputSchemasEnabled is the rollout gate; canSendStructuredContent is the
// separate protocol-legality gate.
func outputSchemasEnabled(ctx context.Context, deps ToolDependencies) bool {
	return deps.IsFeatureEnabled(ctx, FeatureFlagOutputSchemas)
}

// isJSONObject reports whether the marshaled form of v is a JSON object.
func isJSONObject(marshaled []byte) bool {
	return bytes.HasPrefix(bytes.TrimLeft(marshaled, " \t\r\n"), []byte("{"))
}

// canSendStructuredContent reports whether a structuredContent value of this
// shape may legally be sent to this client. An object is always safe; anything
// else needs 2026-07-28, which widened the field from an object to any JSON
// value. A nil req (tests) sends objects only.
func canSendStructuredContent(req *mcp.CallToolRequest, marshaled []byte) bool {
	if isJSONObject(marshaled) {
		return true
	}
	if req == nil {
		return false
	}
	return req.ProtocolVersion() >= inventory.ProtocolVersionNonObjectOutputSchemas
}

// structuredTextResult returns a result whose text content is the serialized
// textValue — byte-identical to the pre-output-schema behaviour, and populated
// regardless of the gates — plus structured as structuredContent when both the
// feature flag and the client's protocol version allow it.
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
