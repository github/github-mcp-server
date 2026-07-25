package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/pkg/inventory"
)

type outputSchemaTestPayload struct {
	Message string `json:"message"`
}

func TestMustOutputSchemaInfersObject(t *testing.T) {
	schema := MustOutputSchema[outputSchemaTestPayload]()
	require.NotNil(t, schema)
	assert.Equal(t, "object", schema.Type)
	assert.Contains(t, schema.Properties, "message")
}

// The 2025-11-25 schema typed outputSchema as a closed object shape; 2026-07-28
// (SEP-2106) allows any valid 2020-12 schema. Non-object roots must therefore
// be inferable rather than a panic — this is the constraint being lifted.
func TestMustOutputSchemaAllowsNonObjectRoots(t *testing.T) {
	assert.NotPanics(t, func() {
		schema := MustOutputSchema[[]outputSchemaTestPayload]()
		// Inference emits the nullable form {"type":["null","array"]} because a
		// Go slice can be nil, so the single-valued Type field is empty and the
		// union lands in Types.
		assert.Empty(t, schema.Type)
		assert.ElementsMatch(t, []string{"null", "array"}, schema.Types)
	}, "a bare array output schema is legal from 2026-07-28")

	assert.NotPanics(t, func() {
		schema := MustOutputSchema[string]()
		assert.Equal(t, "string", schema.Type)
	}, "a bare string output schema is legal from 2026-07-28")
}

// Both of the above roots must be gated for pre-2026-07-28 clients: a type
// array is not the {"type":"object"} shape those revisions required.
func TestInferredNonObjectRootsAreGated(t *testing.T) {
	assert.False(t, inventory.HasObjectRootOutputSchema(MustOutputSchema[[]outputSchemaTestPayload]()))
	assert.False(t, inventory.HasObjectRootOutputSchema(MustOutputSchema[string]()))
	assert.True(t, inventory.HasObjectRootOutputSchema(MustOutputSchema[outputSchemaTestPayload]()))
}

func TestAnyOfSchemaNeverEmitsOneOf(t *testing.T) {
	schema := AnyOfSchema(
		MustOutputSchema[outputSchemaTestPayload](),
		MustOutputSchema[[]outputSchemaTestPayload](),
	)
	require.Len(t, schema.AnyOf, 2)
	assert.Nil(t, schema.OneOf, "oneOf requires exactly one branch to match and is wrong for these unions")

	raw, err := json.Marshal(schema)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"anyOf"`)
	assert.NotContains(t, string(raw), `"oneOf"`)
}

// Guards the reason AnyOfSchema exists: an empty array satisfies every array
// branch, so oneOf ("exactly one") fails where anyOf ("at least one") succeeds.
func TestAnyOfAcceptsAmbiguousPayloadsButStillRejectsGarbage(t *testing.T) {
	schema := MustRawOutputSchema(`{"anyOf":[
		{"type":"array","items":{"type":"object","properties":{"id":{"type":"integer"}}}},
		{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"}}}}
	]}`)
	var parsed = mustResolveSchema(t, schema)

	assert.NoError(t, parsed.Validate([]any{}), "empty array matches both branches; anyOf must accept it")
	assert.Error(t, parsed.Validate(map[string]any{"junk": 1}), "anyOf must still reject values matching no branch")
}

func TestMustRawOutputSchemaRejectsBadInput(t *testing.T) {
	assert.Panics(t, func() { MustRawOutputSchema(`{"type":`) }, "malformed JSON must fail the build")
	assert.Panics(t, func() {
		MustRawOutputSchema(`{"$ref":"#/$defs/missing"}`)
	}, "a dangling $ref must fail the build, not the request")
	assert.NotPanics(t, func() {
		MustRawOutputSchema(`{"$ref":"#/$defs/ok","$defs":{"ok":{"type":"object"}}}`)
	})
}

func depsWithOutputSchemas(enabled bool) BaseDeps {
	return BaseDeps{featureChecker: func(_ context.Context, flag string) (bool, error) {
		return enabled && flag == FeatureFlagOutputSchemas, nil
	}}
}

func callToolReqAtVersion(version string) *mcp.CallToolRequest {
	params := &mcp.CallToolParamsRaw{}
	if version != "" {
		params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: version}
	}
	return &mcp.CallToolRequest{Params: params}
}

func TestStructuredTextResultGating(t *testing.T) {
	objectValue := outputSchemaTestPayload{Message: "structured"}
	arrayValue := []outputSchemaTestPayload{{Message: "a"}}

	tests := []struct {
		name           string
		flagEnabled    bool
		version        string
		structured     any
		wantStructured bool
	}{
		{"flag off means no structured content", false, "2026-07-28", objectValue, false},
		{"object ships on 2025-11-25", true, "2025-11-25", objectValue, true},
		{"object ships on 2026-07-28", true, "2026-07-28", objectValue, true},
		{"array is withheld from 2025-11-25", true, "2025-11-25", arrayValue, false},
		{"array ships on 2026-07-28", true, "2026-07-28", arrayValue, true},
		{"array is withheld when version is unknown", true, "", arrayValue, false},
		{"nil structured is always omitted", true, "2026-07-28", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			textValue := map[string]string{"message": "text"}
			result, err := structuredTextResult(
				context.Background(), depsWithOutputSchemas(tt.flagEnabled),
				callToolReqAtVersion(tt.version), textValue, tt.structured,
			)
			require.NoError(t, err)
			require.NotNil(t, result)

			// The text block is populated unconditionally and is byte-identical
			// to the pre-output-schema behaviour.
			textContent := getTextResult(t, result)
			var gotText map[string]string
			require.NoError(t, json.Unmarshal([]byte(textContent.Text), &gotText))
			assert.Equal(t, textValue, gotText)

			if tt.wantStructured {
				assert.Equal(t, tt.structured, result.StructuredContent)
			} else {
				assert.Nil(t, result.StructuredContent)
			}
		})
	}
}

func TestStructuredTextResultMarshalError(t *testing.T) {
	_, err := structuredTextResult(context.Background(), BaseDeps{}, nil,
		map[string]any{"invalid": func() {}}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal response")
}

func mustResolveSchema(t *testing.T, raw json.RawMessage) *jsonschema.Resolved {
	t.Helper()
	var s jsonschema.Schema
	require.NoError(t, json.Unmarshal(raw, &s))
	resolved, err := s.Resolve(nil)
	require.NoError(t, err)
	return resolved
}
