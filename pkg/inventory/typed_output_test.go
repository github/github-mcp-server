package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type typedTestInput struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Query string `json:"query" jsonschema:"The search query"`
}

type typedTestOutput struct {
	Query string `json:"query"`
}

func TestTypedToolRegistrationInfersSchemasAndValidates(t *testing.T) {
	handlerCalls := 0
	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_tool"},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, input typedTestInput) (*mcp.CallToolResult, typedTestOutput, error) {
			handlerCalls++
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "old text result"}},
			}, typedTestOutput{Query: input.Query}, nil
		},
	)
	require.Nil(t, tool.Tool.InputSchema)
	require.Nil(t, tool.Tool.OutputSchema)

	inv, err := NewBuilder().SetTools([]ServerTool{tool}).WithToolsets([]string{"all"}).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	inv.RegisterTools(context.Background(), server, nil)
	session := connectTypedTestClient(t, server, "")

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	assert.NotNil(t, list.Tools[0].InputSchema)
	assert.NotNil(t, list.Tools[0].OutputSchema)
	assert.NotContains(t, list.Tools[0].Meta, typedOutputMetaKey)
	var inferredSchema map[string]any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, list.Tools[0].InputSchema)), &inferredSchema))
	properties := inferredSchema["properties"].(map[string]any)
	assert.Equal(t, "owner", properties["owner"].(map[string]any)["x-mcp-header"])
	assert.Equal(t, "repo", properties["repo"].(map[string]any)["x-mcp-header"])

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "typed_tool",
		Arguments: map[string]any{"owner": "octo", "repo": "hello", "query": "is:open"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Equal(t, 1, handlerCalls)
	require.NotNil(t, result.StructuredContent)
	assert.JSONEq(t, `{"query":"is:open"}`, mustMarshalJSON(t, result.StructuredContent))
	require.Len(t, result.Content, 1)
	assert.Equal(t, "old text result", result.Content[0].(*mcp.TextContent).Text)

	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "typed_tool",
		Arguments: map[string]any{"owner": "octo", "repo": "hello", "query": 42},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, 1, handlerCalls, "SDK validation must reject invalid arguments before calling the handler")
}

func TestTypedToolInferredInputSchemaUsesStablePointer(t *testing.T) {
	type input struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	}
	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "cached_schema_tool"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, input) (*mcp.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		},
	)
	require.NotNil(t, tool.inferredInputSchema)
	require.Nil(t, tool.inferredInputSchema.schema)

	cache := mcp.NewSchemaCache()
	var inferredSchema *jsonschema.Schema
	for range 2 {
		server := mcp.NewServer(
			&mcp.Implementation{Name: "test-server", Version: "v0.0.1"},
			&mcp.ServerOptions{SchemaCache: cache},
		)
		tool.RegisterFunc(server, nil)
		require.NotNil(t, tool.inferredInputSchema.schema)
		if inferredSchema == nil {
			inferredSchema = tool.inferredInputSchema.schema
		} else {
			assert.Same(t, inferredSchema, tool.inferredInputSchema.schema, "re-registration must reuse the inferred schema pointer")
		}
	}
	assert.Equal(t, "owner", inferredSchema.Properties["owner"].Extra["x-mcp-header"])
	assert.Equal(t, "repo", inferredSchema.Properties["repo"].Extra["x-mcp-header"])
}

func TestTypedToolInputSchemasPreserveObjectOptionality(t *testing.T) {
	type optionalPointerInput struct {
		Owner  string  `json:"owner"`
		Filter *string `json:"filter,omitempty"`
		Limit  int     `json:"limit,omitempty"`
	}

	const emptyInputSchema = `{"type":"object"}`
	const optionalPointerInputSchema = `{
		"type":"object",
		"properties":{
			"owner":{"type":"string"},
			"filter":{"type":"string"},
			"limit":{"type":"integer"}
		},
		"required":["owner"]
	}`

	legacyEmptyTool := NewServerTool(
		mcp.Tool{Name: "legacy_empty_input", InputSchema: json.RawMessage(emptyInputSchema)},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return nil, nil
		},
	)
	typedEmptyTool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_empty_input"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		},
	)
	legacyOptionalTool := NewServerTool(
		mcp.Tool{Name: "legacy_optional_input", InputSchema: json.RawMessage(optionalPointerInputSchema)},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return nil, nil
		},
	)
	typedOptionalTool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_optional_input"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, optionalPointerInput) (*mcp.CallToolResult, struct{}, error) {
			return nil, struct{}{}, nil
		},
	)

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	legacyEmptyTool.RegisterFunc(server, nil)
	typedEmptyTool.RegisterFunc(server, nil)
	legacyOptionalTool.RegisterFunc(server, nil)
	typedOptionalTool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	schemasByName := make(map[string]any, len(list.Tools))
	for _, tool := range list.Tools {
		schemasByName[tool.Name] = tool.InputSchema
	}

	for _, tc := range []struct {
		legacyName     string
		typedName      string
		wantSchema     string
		wantProperties []string
		wantRequired   []string
	}{
		{legacyName: "legacy_empty_input", typedName: "typed_empty_input", wantSchema: emptyInputSchema},
		{
			legacyName:     "legacy_optional_input",
			typedName:      "typed_optional_input",
			wantSchema:     optionalPointerInputSchema,
			wantProperties: []string{"filter", "limit", "owner"},
			wantRequired:   []string{"owner"},
		},
	} {
		t.Run(tc.typedName, func(t *testing.T) {
			legacySchema, ok := schemasByName[tc.legacyName]
			require.True(t, ok)
			typedSchema, ok := schemasByName[tc.typedName]
			require.True(t, ok)

			legacyJSON := mustMarshalJSON(t, legacySchema)
			typedJSON := mustMarshalJSON(t, typedSchema)

			legacyShape := readInputSchemaShape(t, legacyJSON)
			typedShape := readInputSchemaShape(t, typedJSON)
			expectedShape := readInputSchemaShape(t, tc.wantSchema)
			assert.Equal(t, expectedShape, legacyShape, "fixture must represent the pre-migration input contract")
			assert.Equal(t, legacyShape, typedShape, "typed registration must preserve the pre-migration input contract")

			assert.Equal(t, []string{"object"}, typedShape.rootTypes, "typed input roots must remain objects")
			assert.False(t, typedShape.hasAnyOf, "input roots must not become nullable or optional unions")
			assert.Equal(t, tc.wantProperties, typedShape.properties, "typed registration must preserve the input properties")
			assert.ElementsMatch(t, tc.wantRequired, typedShape.required, "typed registration must preserve required properties")
		})
	}
}

type inputSchemaShape struct {
	rootTypes  []string
	properties []string
	required   []string
	hasAnyOf   bool
}

func readInputSchemaShape(t *testing.T, schemaJSON string) inputSchemaShape {
	t.Helper()
	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), &root))
	var schemaValue any
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), &schemaValue))

	shape := inputSchemaShape{}
	rawType, ok := root["type"]
	require.True(t, ok, "schema must declare a root type")
	if len(rawType) > 0 && rawType[0] == '[' {
		require.NoError(t, json.Unmarshal(rawType, &shape.rootTypes))
	} else {
		var rootType string
		require.NoError(t, json.Unmarshal(rawType, &rootType))
		shape.rootTypes = []string{rootType}
	}
	shape.hasAnyOf = schemaContainsAnyOf(schemaValue)

	var properties map[string]json.RawMessage
	if rawProperties, ok := root["properties"]; ok {
		require.NoError(t, json.Unmarshal(rawProperties, &properties))
		for name := range properties {
			shape.properties = append(shape.properties, name)
		}
	}
	slices.Sort(shape.properties)

	if rawRequired, ok := root["required"]; ok {
		require.NoError(t, json.Unmarshal(rawRequired, &shape.required))
	}
	return shape
}

func schemaContainsAnyOf(value any) bool {
	switch value := value.(type) {
	case map[string]any:
		if _, ok := value["anyOf"]; ok {
			return true
		}
		for _, child := range value {
			if schemaContainsAnyOf(child) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(value, schemaContainsAnyOf)
	}
	return false
}

func TestTypedToolRegistrationAppliesExplicitSchemas(t *testing.T) {
	type input struct {
		Mode string `json:"mode,omitempty"`
	}
	type output struct {
		Mode  string `json:"mode,omitempty"`
		Count int    `json:"count"`
	}

	tool := NewServerToolWithContextHandler(
		mcp.Tool{
			Name: "explicit_schema_tool",
			InputSchema: json.RawMessage(`{
				"type":"object",
				"properties":{"mode":{"type":"string","enum":["wide","narrow"],"default":"wide"}},
				"required":["mode"]
			}`),
			OutputSchema: json.RawMessage(`{
				"type":"object",
				"properties":{
					"mode":{"type":"string","enum":["wide","narrow"],"default":"wide"},
					"count":{"type":"integer","minimum":2}
				},
				"required":["mode","count"]
			}`),
		},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, args input) (*mcp.CallToolResult, output, error) {
			count := 3
			if args.Mode == "narrow" {
				count = 1
			}
			return nil, output{Count: count, Mode: args.Mode}, nil
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	assert.Contains(t, mustMarshalJSON(t, list.Tools[0].InputSchema), `"default":"wide"`)
	assert.Contains(t, mustMarshalJSON(t, list.Tools[0].OutputSchema), `"minimum":2`)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "explicit_schema_tool",
		Arguments: map[string]any{"mode": "wide"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.JSONEq(t, `{"mode":"wide","count":3}`, mustMarshalJSON(t, result.StructuredContent))

	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "explicit_schema_tool",
		Arguments: map[string]any{"mode": "invalid"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)

	_, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "explicit_schema_tool",
		Arguments: map[string]any{"mode": "narrow"},
	})
	require.ErrorContains(t, err, "validating tool output")
}

func TestTypedInputNormalizerPreservesStrictSchemaAndLegacyValues(t *testing.T) {
	type listIssuesInput struct {
		Owner       string `json:"owner"`
		Repo        string `json:"repo"`
		State       string `json:"state"`
		IssueNumber int64  `json:"issue_number"`
	}
	type listIssuesOutput struct {
		State       string `json:"state"`
		IssueNumber int64  `json:"issue_number"`
	}

	handlerCalls := 0
	var got listIssuesInput
	tool := NewServerToolWithContextHandler(
		mcp.Tool{
			Name: "list_issues",
			InputSchema: json.RawMessage(`{
				"type":"object",
				"properties":{
					"owner":{"type":"string","minLength":1},
					"repo":{"type":"string","minLength":1},
					"state":{"type":"string","enum":["OPEN","CLOSED"]},
					"issue_number":{"type":"integer"}
				},
				"required":["owner","repo"]
			}`),
		},
		testToolsetMetadata("issues"),
		func(_ context.Context, _ *mcp.CallToolRequest, input listIssuesInput) (*mcp.CallToolResult, listIssuesOutput, error) {
			handlerCalls++
			got = input
			return nil, listIssuesOutput{State: input.State, IssueNumber: input.IssueNumber}, nil
		},
		normalizeListIssuesWireInput,
	)
	directResult, err := tool.Handler(nil)(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "list_issues",
			Arguments: json.RawMessage(`{"owner":"octo","repo":"hello","state":"open","issue_number":"42"}`),
		},
	})
	require.NoError(t, err)
	assert.Nil(t, directResult)
	assert.Equal(t, "OPEN", got.State)
	assert.Equal(t, int64(42), got.IssueNumber)
	handlerCalls = 0

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	var inputSchema map[string]any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, list.Tools[0].InputSchema)), &inputSchema))
	properties := inputSchema["properties"].(map[string]any)
	assert.Equal(t, "integer", properties["issue_number"].(map[string]any)["type"])
	assert.Equal(t, []any{"OPEN", "CLOSED"}, properties["state"].(map[string]any)["enum"])

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "list_issues",
		Arguments: map[string]any{
			"owner":        "octo",
			"repo":         "hello",
			"state":        "open",
			"issue_number": "42",
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Equal(t, 1, handlerCalls)
	assert.Equal(t, "OPEN", got.State)
	assert.Equal(t, int64(42), got.IssueNumber)
	assert.JSONEq(t, `{"state":"OPEN","issue_number":42}`, mustMarshalJSON(t, result.StructuredContent))

	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_issues",
		Arguments: map[string]any{"owner": "octo", "repo": "hello", "state": "unsupported"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError, "normalization must not bypass SDK enum validation")
	assert.Equal(t, 1, handlerCalls)

	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_issues",
		Arguments: map[string]any{"owner": "octo", "repo": "hello", "issue_number": "not-a-number"},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, 1, handlerCalls)
}

func normalizeListIssuesWireInput(arguments json.RawMessage) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &fields); err != nil {
		return nil, err
	}
	if rawState, ok := fields["state"]; ok {
		var state string
		if err := json.Unmarshal(rawState, &state); err != nil {
			return nil, err
		}
		normalizedState, err := json.Marshal(strings.ToUpper(state))
		if err != nil {
			return nil, err
		}
		fields["state"] = normalizedState
	}
	if rawID, ok := fields["issue_number"]; ok && len(rawID) > 0 && rawID[0] == '"' {
		var id string
		if err := json.Unmarshal(rawID, &id); err != nil {
			return nil, err
		}
		parsedID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, err
		}
		fields["issue_number"] = json.RawMessage(strconv.FormatInt(parsedID, 10))
	}
	return json.Marshal(fields)
}

func TestTypedInputNormalizerUsesLastRegisteredDefinition(t *testing.T) {
	type input struct {
		State string `json:"state"`
	}
	type output struct {
		State string `json:"state"`
	}

	first := NewServerToolWithContextHandler(
		mcp.Tool{Name: "duplicate_tool"},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, args input) (*mcp.CallToolResult, output, error) {
			return nil, output{State: "first:" + args.State}, nil
		},
		func(_ json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"state":"OPEN"}`), nil
		},
	)
	last := NewServerToolWithContextHandler(
		mcp.Tool{
			Name: "duplicate_tool",
			InputSchema: json.RawMessage(`{
				"type":"object",
				"properties":{"state":{"type":"string","enum":["open"]}},
				"required":["state"]
			}`),
		},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, args input) (*mcp.CallToolResult, output, error) {
			return nil, output{State: "last:" + args.State}, nil
		},
	)

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	first.RegisterFunc(server, nil)
	last.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "duplicate_tool",
		Arguments: map[string]any{"state": "open"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.JSONEq(t, `{"state":"last:open"}`, mustMarshalJSON(t, result.StructuredContent))
}

func TestTypedToolMiddlewareShortCircuitAndHandlerErrors(t *testing.T) {
	handlerCalls := 0
	shortCircuit := NewServerToolWithContextHandler(
		mcp.Tool{Name: "short_circuit_tool"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, typedTestOutput, error) {
			handlerCalls++
			return nil, typedTestOutput{Query: "should not run"}, nil
		},
	)
	handlerError := NewServerToolWithContextHandler(
		mcp.Tool{Name: "handler_error_tool"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, typedTestOutput, error) {
			return nil, typedTestOutput{}, errors.New("typed handler failed")
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	shortCircuit.RegisterFunc(server, nil, func(mcp.ToolHandler) mcp.ToolHandler {
		return func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "authorization required"}},
				IsError: true,
			}, nil
		}
	})
	handlerError.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "short_circuit_tool"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Nil(t, result.StructuredContent)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "authorization required", result.Content[0].(*mcp.TextContent).Text)
	assert.Zero(t, handlerCalls, "middleware short-circuit must not invoke the typed handler")

	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "handler_error_tool"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Nil(t, result.StructuredContent)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].(*mcp.TextContent).Text, "typed handler failed")
}

func TestTypedPointerOutputAndUntypedGenericCompatibility(t *testing.T) {
	type pointerOutput struct {
		Value *string `json:"value,omitempty"`
	}
	type pointerInput struct {
		Include bool `json:"include"`
	}
	value := "present"
	pointerTool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "pointer_tool"},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, input pointerInput) (*mcp.CallToolResult, *pointerOutput, error) {
			output := &pointerOutput{}
			if input.Include {
				output.Value = &value
			}
			return nil, output, nil
		},
	)
	untypedCalls := 0
	untypedTool := NewServerToolWithContextHandler[map[string]any, any](
		mcp.Tool{
			Name:        "untyped_generic_tool",
			InputSchema: &jsonschema.Schema{Type: "object"},
		},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
			untypedCalls++
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: input["extra"].(string)}},
			}, nil, nil
		},
	)
	require.Nil(t, untypedTool.registerTyped, "any output must retain the raw handler path")

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	pointerTool.RegisterFunc(server, nil)
	untypedTool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	toolsByName := make(map[string]*mcp.Tool, len(list.Tools))
	for _, tool := range list.Tools {
		toolsByName[tool.Name] = tool
	}
	assert.NotNil(t, toolsByName["pointer_tool"].OutputSchema)
	assert.Nil(t, toolsByName["untyped_generic_tool"].OutputSchema)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "pointer_tool",
		Arguments: map[string]any{"include": true},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"value":"present"}`, mustMarshalJSON(t, result.StructuredContent))

	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "pointer_tool",
		Arguments: map[string]any{"include": false},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, mustMarshalJSON(t, result.StructuredContent), "omitempty pointer fields must remain omitted")

	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "untyped_generic_tool",
		Arguments: map[string]any{"extra": "accepted"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "accepted", result.Content[0].(*mcp.TextContent).Text)
	assert.Equal(t, 1, untypedCalls)
}

func TestTypedToolOutputProtocolGatePreservesTextAndSharedTool(t *testing.T) {
	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_array_tool"},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, []string, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "legacy array text"}},
			}, []string{"one", "two"}, nil
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)

	legacy := connectTypedTestClient(t, server, "2025-11-25")
	legacyList, err := legacy.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, legacyList.Tools, 1)
	assert.Nil(t, legacyList.Tools[0].OutputSchema)
	assert.NotContains(t, legacyList.Tools[0].Meta, typedOutputMetaKey)

	legacyResult, err := legacy.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_array_tool"})
	require.NoError(t, err)
	assert.Nil(t, legacyResult.StructuredContent)
	require.Len(t, legacyResult.Content, 1, "the SDK's array fallback must not duplicate the old text")
	assert.Equal(t, "legacy array text", legacyResult.Content[0].(*mcp.TextContent).Text)

	modernServer := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(modernServer, nil)
	modern := connectTypedTestClient(t, modernServer, "")
	assert.Equal(t, ProtocolVersionMultiRoundTrip, modern.InitializeResult().ProtocolVersion)
	modernList, err := modern.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, modernList.Tools, 1)
	assert.NotNil(t, modernList.Tools[0].OutputSchema)
	assert.NotContains(t, modernList.Tools[0].Meta, typedOutputMetaKey)

	modernResult, err := modern.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_array_tool"})
	require.NoError(t, err)
	assert.JSONEq(t, `["one","two"]`, mustMarshalJSON(t, modernResult.StructuredContent))
	require.Len(t, modernResult.Content, 1)
	assert.Equal(t, "legacy array text", modernResult.Content[0].(*mcp.TextContent).Text)

	require.Nil(t, tool.Tool.OutputSchema, "registration must not mutate the shared definition")
	assert.NotContains(t, tool.Tool.Meta, typedOutputMetaKey)
}

func TestTypedScalarOutputProtocolGatePreservesHandlerText(t *testing.T) {
	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_scalar_tool"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, string, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "custom legacy text"}},
			}, "structured scalar", nil
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)
	legacy := connectTypedTestClient(t, server, "2025-11-25")

	list, err := legacy.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	assert.Nil(t, list.Tools[0].OutputSchema)

	result, err := legacy.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_scalar_tool"})
	require.NoError(t, err)
	assert.Nil(t, result.StructuredContent)
	require.Len(t, result.Content, 1, "SDK scalar fallback must not replace or duplicate handler text")
	assert.Equal(t, "custom legacy text", result.Content[0].(*mcp.TextContent).Text)
}

func TestTypedToolSupportsExplicitRootUnionOutputSchema(t *testing.T) {
	tool := NewServerToolWithContextHandler(
		mcp.Tool{
			Name: "union_output_tool",
			OutputSchema: json.RawMessage(`{
				"oneOf":[
					{"type":"string"},
					{"type":"array","items":{"type":"string"}}
				]
			}`),
		},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, json.RawMessage, error) {
			return nil, json.RawMessage(`["one","two"]`), nil
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	assert.Contains(t, mustMarshalJSON(t, list.Tools[0].OutputSchema), `"oneOf"`)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "union_output_tool"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.JSONEq(t, `["one","two"]`, mustMarshalJSON(t, result.StructuredContent))
}

func TestTypedOutputProtocolGateUsesStatelessRequestVersion(t *testing.T) {
	tool := &mcp.Tool{
		Name:         "typed_tool",
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		Meta:         mcp.Meta{typedOutputMetaKey: true},
	}
	next := func(_ context.Context, method string, _ mcp.Request) (mcp.Result, error) {
		switch method {
		case MCPMethodToolsList:
			return &mcp.ListToolsResult{Tools: []*mcp.Tool{tool}}, nil
		case MCPMethodToolsCall:
			return &mcp.CallToolResult{
				Meta:              mcp.Meta{typedOutputMetaKey: typedOutputMetadata{hasOutput: true}},
				Content:           []mcp.Content{&mcp.TextContent{Text: "text fallback"}},
				StructuredContent: map[string]any{"ok": true},
			}, nil
		default:
			t.Fatalf("unexpected method %q", method)
			return nil, nil
		}
	}
	wrapped := typedOutputMiddleware(nil)(next)

	for _, tc := range []struct {
		name            string
		protocolVersion string
		wantSchema      bool
		wantStructured  bool
	}{
		{name: "stateless modern", protocolVersion: ProtocolVersionMultiRoundTrip, wantSchema: true, wantStructured: true},
		{name: "stateless legacy", protocolVersion: "2025-11-25"},
		{name: "unknown version defaults to legacy behavior"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			meta := mcp.Meta{}
			if tc.protocolVersion != "" {
				meta[mcp.MetaKeyProtocolVersion] = tc.protocolVersion
			}
			listResult, err := wrapped(context.Background(), MCPMethodToolsList, &mcp.ListToolsRequest{
				Params: &mcp.ListToolsParams{Meta: meta},
			})
			require.NoError(t, err)
			list := listResult.(*mcp.ListToolsResult)
			if tc.wantSchema {
				assert.NotNil(t, list.Tools[0].OutputSchema)
			} else {
				assert.Nil(t, list.Tools[0].OutputSchema)
			}
			assert.NotContains(t, list.Tools[0].Meta, typedOutputMetaKey)

			callResult, err := wrapped(context.Background(), MCPMethodToolsCall, &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Name: "typed_tool",
					Meta: meta,
				},
			})
			require.NoError(t, err)
			call := callResult.(*mcp.CallToolResult)
			if tc.wantStructured {
				assert.NotNil(t, call.StructuredContent)
			} else {
				assert.Nil(t, call.StructuredContent)
			}
			assert.NotContains(t, call.Meta, typedOutputMetaKey)
			require.Len(t, call.Content, 1)
			assert.Equal(t, "text fallback", call.Content[0].(*mcp.TextContent).Text)
		})
	}

	inputRequired := &mcp.CallToolResult{
		Meta:              mcp.Meta{},
		StructuredContent: map[string]any{"awaiting": true},
		InputRequests:     mcp.InputRequestMap{"input": &mcp.ElicitParams{Mode: "form"}},
	}
	rawMiddleware := typedOutputMiddleware(nil)(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return inputRequired, nil
	})
	result, err := rawMiddleware(context.Background(), MCPMethodToolsCall, &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: "raw_tool"},
	})
	require.NoError(t, err)
	assert.Same(t, inputRequired, result, "non-typed multi-round-trip structured content must pass through")
}

func TestTypedOutputGateLeavesRawToolResultsAlone(t *testing.T) {
	typedOverride := NewServerToolWithContextHandler(
		mcp.Tool{Name: "raw_tool"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, map[string]any, error) {
			return nil, map[string]any{"typed": true}, nil
		},
	)
	rawTool := NewServerTool(
		mcp.Tool{
			Name:         "raw_tool",
			InputSchema:  &jsonschema.Schema{Type: "object"},
			OutputSchema: json.RawMessage(`{"type":"object"}`),
		},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				StructuredContent: map[string]any{"state": "awaiting"},
			}, nil
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	typedOverride.RegisterFunc(server, nil)
	rawTool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "2025-11-25")

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	assert.Nil(t, list.Tools[0].OutputSchema, "output schemas are hidden from legacy clients")

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "raw_tool"})
	require.NoError(t, err)
	require.NotNil(t, result.StructuredContent, "raw structured content without typed output must remain untouched")
}

func connectTypedTestClient(t *testing.T, server *mcp.Server, protocolVersion string) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if protocolVersion != "" {
		server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
				if method == "server/discover" {
					return nil, errors.New("legacy client uses initialize")
				}
				return next(ctx, method, request)
			}
		})
	}
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, &mcp.ClientOptions{})
	var transport mcp.Transport = clientTransport
	if protocolVersion != "" {
		transport = protocolVersionTransport{Transport: clientTransport, protocolVersion: protocolVersion}
	}
	clientSession, err := client.Connect(context.Background(), transport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSession.Close() })
	if protocolVersion != "" {
		require.Equal(t, protocolVersion, clientSession.InitializeResult().ProtocolVersion)
	}
	return clientSession
}

func mustMarshalJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}

type protocolVersionTransport struct {
	mcp.Transport
	protocolVersion string
}

func (t protocolVersionTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	connection, err := t.Transport.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return protocolVersionConnection{Connection: connection, protocolVersion: t.protocolVersion}, nil
}

type protocolVersionConnection struct {
	mcp.Connection
	protocolVersion string
}

func (c protocolVersionConnection) Write(ctx context.Context, message jsonrpc.Message) error {
	request, ok := message.(*jsonrpc.Request)
	if !ok || request.Method != "initialize" {
		return c.Connection.Write(ctx, message)
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return err
	}
	version, err := json.Marshal(c.protocolVersion)
	if err != nil {
		return err
	}
	params["protocolVersion"] = version
	encoded, err := json.Marshal(params)
	if err != nil {
		return err
	}
	requestCopy := *request
	requestCopy.Params = encoded
	return c.Connection.Write(ctx, &requestCopy)
}
