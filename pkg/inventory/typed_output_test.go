package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
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

func TestTypedExplicitErrorOutputProtocolGate(t *testing.T) {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {Type: "string"},
		},
		Required: []string{"query"},
	}
	registration := &typedToolRegistration{
		name:           "pending",
		modernTool:     &mcp.Tool{Name: "pending", OutputSchema: schema},
		legacyTool:     &mcp.Tool{Name: "pending"},
		hasTypedOutput: true,
	}
	middleware := typedOutputMiddleware(&typedToolRegistrationSet{
		byName: map[string]*typedToolRegistration{"pending": registration},
	})

	for _, tc := range []struct {
		name              string
		protocol          string
		result            *mcp.CallToolResult
		wantStructured    any
		wantInputRequests bool
	}{
		{
			name:     "modern explicit awaiting status",
			protocol: ProtocolVersionMultiRoundTrip,
			result: &mcp.CallToolResult{
				IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "wait for input"}},
				StructuredContent: typedTestOutput{Query: "awaiting"},
			},
			wantStructured: typedTestOutput{Query: "awaiting"},
		},
		{
			name:     "legacy omits awaiting status",
			protocol: "2025-11-25",
			result: &mcp.CallToolResult{
				IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "wait for input"}},
				StructuredContent: typedTestOutput{Query: "awaiting"},
			},
		},
		{
			name:     "ordinary typed error",
			protocol: ProtocolVersionMultiRoundTrip,
			result: &mcp.CallToolResult{
				IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "request failed"}},
			},
		},
		{
			name:     "modern elicitation retains status and request",
			protocol: ProtocolVersionMultiRoundTrip,
			result: &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: "wait for input"}},
				StructuredContent: typedTestOutput{Query: "awaiting"},
				InputRequests:     mcp.InputRequestMap{"form": &mcp.ElicitParams{Mode: "form"}},
			},
			wantStructured:    typedTestOutput{Query: "awaiting"},
			wantInputRequests: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := middleware(func(context.Context, string, mcp.Request) (mcp.Result, error) {
				return tc.result, nil
			})
			result, err := handler(context.Background(), MCPMethodToolsCall, &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{
					Name: "pending",
					Meta: mcp.Meta{mcp.MetaKeyProtocolVersion: tc.protocol},
				},
			})
			require.NoError(t, err)
			callResult, ok := result.(*mcp.CallToolResult)
			require.True(t, ok)
			require.True(t, callResult.IsError || callResult.InputRequests != nil)
			assert.Equal(t, tc.wantStructured, callResult.StructuredContent)
			if tc.wantInputRequests {
				assert.Equal(t, tc.result.InputRequests, callResult.InputRequests)
			} else {
				assert.Nil(t, callResult.InputRequests)
			}
			assert.Equal(t, tc.result.Content, callResult.Content)
		})
	}
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
	assert.JSONEq(t, `{"query":"is:open"}`, result.Content[0].(*mcp.TextContent).Text)

	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "typed_tool",
		Arguments: map[string]any{"owner": "octo", "repo": "hello", "query": 42},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, 1, handlerCalls, "SDK validation must reject invalid arguments before calling the handler")
}

func TestTypedToolLegacySchemaRetainsHeaderRoutingMetadata(t *testing.T) {
	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "legacy_routing_tool"},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, input typedTestInput) (*mcp.CallToolResult, typedTestOutput, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "legacy text result"}},
			}, typedTestOutput{Query: input.Query}, nil
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "2025-11-25")

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	assert.Nil(t, list.Tools[0].OutputSchema)
	var inputSchema map[string]any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, list.Tools[0].InputSchema)), &inputSchema))
	properties := inputSchema["properties"].(map[string]any)
	assert.Equal(t, "owner", properties["owner"].(map[string]any)["x-mcp-header"])
	assert.Equal(t, "repo", properties["repo"].(map[string]any)["x-mcp-header"])

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "legacy_routing_tool",
		Arguments: map[string]any{"owner": "octo", "repo": "hello", "query": "is:open"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Nil(t, result.StructuredContent)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "legacy text result", result.Content[0].(*mcp.TextContent).Text)
}

func TestTypedToolInferredInputSchemaUsesStablePointer(t *testing.T) {
	type input struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	}
	first, err := CachedInputSchemaFor[input](nil)
	require.NoError(t, err)
	second, err := CachedInputSchemaFor[input](nil)
	require.NoError(t, err)
	assert.Same(t, first, second, "schema inference must be cached across tool/server instances")
	inferredSchema := first
	assert.Equal(t, "owner", inferredSchema.Properties["owner"].Extra["x-mcp-header"])
	assert.Equal(t, "repo", inferredSchema.Properties["repo"].Extra["x-mcp-header"])
}

func TestTypedSchemasAreCachedAcrossFreshRegistrations(t *testing.T) {
	type input struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	}
	type output struct {
		State string `json:"state"`
	}
	var firstInput, firstOutput *jsonschema.Schema
	for range 2 {
		tool := NewServerToolWithContextHandler(
			mcp.Tool{Name: "cached_typed_tool"},
			testToolsetMetadata("test"),
			func(context.Context, *mcp.CallToolRequest, input) (*mcp.CallToolResult, output, error) {
				return nil, output{}, nil
			},
		)
		registration := tool.typedRegistration(&tool.Tool)
		modernInput := registration.modernTool.InputSchema.(*jsonschema.Schema)
		legacyInput := registration.legacyTool.InputSchema.(*jsonschema.Schema)
		assert.Same(t, modernInput, legacyInput, "both eras must share the pre-annotated input schema")
		assert.Same(t, modernInput, registration.modernRuntimeTool.InputSchema)
		assert.Same(t, modernInput, registration.legacyRuntimeTool.InputSchema)
		assert.Nil(t, registration.legacyTool.OutputSchema)
		assert.Nil(t, registration.legacyRuntimeTool.OutputSchema)
		if firstInput == nil {
			firstInput = modernInput
			firstOutput = registration.modernTool.OutputSchema.(*jsonschema.Schema)
			continue
		}
		assert.Same(t, firstInput, modernInput)
		assert.Same(t, firstOutput, registration.modernTool.OutputSchema)
	}
}

func TestCachedExplicitSchemaWarmPathsPreserveOwnership(t *testing.T) {
	original := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {Type: "string", Description: "original owner"},
		},
	}
	cached, err := CachedSchema(original)
	require.NoError(t, err)
	require.NotSame(t, original, cached)
	original.Properties["owner"].Description = "changed owner"
	assert.Equal(t, "original owner", cached.Properties["owner"].Description)
	changed, err := CachedSchema(original)
	require.NoError(t, err)
	assert.NotSame(t, cached, changed, "caller-owned pointers must retain content-based lookup")

	tool := mcp.Tool{InputSchema: cached}
	AnnotateHeaderParams(&tool)
	annotated := tool.InputSchema.(*jsonschema.Schema)
	assert.NotSame(t, cached, annotated)
	assert.Empty(t, cached.Properties["owner"].Extra, "header preparation must not mutate the advertised schema")
	assert.Equal(t, "owner", annotated.Properties["owner"].Extra["x-mcp-header"])

	var cacheErr error
	allocations := testing.AllocsPerRun(10, func() {
		warm := mcp.Tool{InputSchema: cached}
		AnnotateHeaderParams(&warm)
		var prepared *jsonschema.Schema
		prepared, cacheErr = CachedSchema(warm.InputSchema.(*jsonschema.Schema))
		if prepared != annotated {
			panic("warm registration changed the cached schema pointer")
		}
	})
	require.NoError(t, cacheErr)
	assert.Zero(t, allocations, "cache-owned warm schemas must avoid canonical serialization and annotation cloning")
}

func TestProtocolEraForSupportedVersions(t *testing.T) {
	supported := []string{"2027-01-01", ProtocolVersionMultiRoundTrip, "2025-11-25", "2027-02-30", "garbage"}
	for _, tc := range []struct {
		name    string
		version string
		want    ProtocolEra
	}{
		{name: "exact threshold", version: ProtocolVersionMultiRoundTrip, want: ProtocolEraModern},
		{name: "supported later", version: "2027-01-01", want: ProtocolEraModern},
		{name: "older", version: "2025-11-25", want: ProtocolEraLegacy},
		{name: "unsupported future", version: "2099-01-01", want: ProtocolEraLegacy},
		{name: "invalid date even if listed", version: "2027-02-30", want: ProtocolEraLegacy},
		{name: "garbage even if listed", version: "garbage", want: ProtocolEraLegacy},
		{name: "absent", want: ProtocolEraLegacy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, protocolEraForSupportedVersion(tc.version, supported))
		})
	}
	assert.Equal(t, ProtocolEraLegacy, protocolEraForSupportedVersion(ProtocolVersionMultiRoundTrip, nil),
		"a valid modern date must still be supported")

	for _, version := range mcp.SupportedProtocolVersions() {
		want := ProtocolEraLegacy
		if version >= ProtocolVersionMultiRoundTrip {
			want = ProtocolEraModern
		}
		assert.Equal(t, want, ProtocolEraForVersion(version), "SDK-supported version %q", version)
	}
	for _, version := range []string{"", "garbage", "2027-02-30", "2099-01-01"} {
		assert.Equal(t, ProtocolEraLegacy, ProtocolEraForVersion(version), "unknown version %q", version)
	}
}

func TestTypedPointerInputUsesObjectSchemaInBothEras(t *testing.T) {
	type input struct {
		Query string `json:"query"`
	}
	valueSchema, err := CachedInputSchemaFor[input](nil)
	require.NoError(t, err)
	pointerSchema, err := CachedInputSchemaFor[*input](nil)
	require.NoError(t, err)
	assert.Same(t, valueSchema, pointerSchema, "SDK inference unwraps one input pointer before caching")
	assert.Equal(t, "object", pointerSchema.Type)
	assert.Empty(t, pointerSchema.Types)

	outputSchema, err := CachedSchemaFor[*input](nil)
	require.NoError(t, err)
	assert.Contains(t, outputSchema.Types, "null", "output pointers must retain nullability")

	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "pointer_input"},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, args *input) (*mcp.CallToolResult, typedTestOutput, error) {
			require.NotNil(t, args)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: args.Query}}},
				typedTestOutput{Query: args.Query}, nil
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	require.NotPanics(t, func() { tool.RegisterFunc(server, nil) })
	for _, version := range []string{"", "2025-11-25"} {
		session := connectTypedTestClient(t, server, version)
		list, err := session.ListTools(context.Background(), nil)
		require.NoError(t, err)
		require.Len(t, list.Tools, 1)
		assert.Contains(t, mustMarshalJSON(t, list.Tools[0].InputSchema), `"type":"object"`)
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: tool.Tool.Name, Arguments: map[string]any{"query": "pointer input decoded"},
		})
		require.NoError(t, err)
		require.False(t, result.IsError)
		if version == "" {
			assert.JSONEq(t, `{"query":"pointer input decoded"}`, mustMarshalJSON(t, result.StructuredContent))
		} else {
			assert.Nil(t, result.StructuredContent)
			require.Len(t, result.Content, 1)
			assert.Equal(t, "pointer input decoded", result.Content[0].(*mcp.TextContent).Text)
		}
	}
}

func TestCachedSchemaDeepCopiesMutableMetadata(t *testing.T) {
	constant := any(map[string]any{"nested": []string{"const"}})
	nullConstant := any(nil)
	original := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"state": {
				Type: "string",
				Enum: []any{map[string]any{"values": []string{"open"}}},
				Extra: map[string]any{
					"nested": map[string]any{"values": []string{"property"}},
				},
			},
		},
		Enum:     []any{map[string]any{"values": []string{"root"}}},
		Const:    &constant,
		Examples: []any{map[string]any{"values": []string{"example"}}},
		DependencyStrings: map[string][]string{
			"state": {"repo"},
		},
		PropertyOrder: []string{"state"},
		Extra: map[string]any{
			"nested": map[string]any{"values": []string{"extra"}},
		},
	}
	cached, err := CachedSchema(original)
	require.NoError(t, err)

	original.Enum[0].(map[string]any)["values"].([]string)[0] = "mutated-root"
	original.Properties["state"].Enum[0].(map[string]any)["values"].([]string)[0] = "mutated-property"
	original.Properties["state"].Extra["nested"].(map[string]any)["values"].([]string)[0] = "mutated-property-extra"
	(*original.Const).(map[string]any)["nested"].([]string)[0] = "mutated-const"
	original.Examples[0].(map[string]any)["values"].([]string)[0] = "mutated-example"
	original.DependencyStrings["state"][0] = "mutated-dependency"
	original.PropertyOrder[0] = "mutated-order"
	original.Extra["nested"].(map[string]any)["values"].([]string)[0] = "mutated-extra"

	assert.Equal(t, "root", cached.Enum[0].(map[string]any)["values"].([]string)[0])
	assert.Equal(t, "open", cached.Properties["state"].Enum[0].(map[string]any)["values"].([]string)[0])
	assert.Equal(t, "property", cached.Properties["state"].Extra["nested"].(map[string]any)["values"].([]string)[0])
	assert.Equal(t, "const", (*cached.Const).(map[string]any)["nested"].([]string)[0])
	assert.Equal(t, "example", cached.Examples[0].(map[string]any)["values"].([]string)[0])
	assert.Equal(t, []string{"repo"}, cached.DependencyStrings["state"])
	assert.Equal(t, []string{"state"}, cached.PropertyOrder)
	assert.Equal(t, "extra", cached.Extra["nested"].(map[string]any)["values"].([]string)[0])
	nullSchema, err := CachedSchema(&jsonschema.Schema{Const: &nullConstant})
	require.NoError(t, err)
	require.NotNil(t, nullSchema.Const)
	assert.Nil(t, *nullSchema.Const)

	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for range 100 {
			original.Properties["state"].Extra["nested"].(map[string]any)["values"].([]string)[0] = "concurrent mutation"
		}
	}()
	go func() {
		defer workers.Done()
		for range 100 {
			if _, err := json.Marshal(cached); err != nil {
				t.Errorf("marshal cached schema: %v", err)
				return
			}
		}
	}()
	workers.Wait()
}

func TestSchemaClonesDetachEveryNumericKeyword(t *testing.T) {
	for name, clone := range map[string]func(*jsonschema.Schema) *jsonschema.Schema{
		"direct": CloneSchema,
		"cache": func(schema *jsonschema.Schema) *jsonschema.Schema {
			cached, err := CachedSchema(schema)
			require.NoError(t, err)
			return cached
		},
		"inference options": func(schema *jsonschema.Schema) *jsonschema.Schema {
			options := cloneJSONSchemaForOptions(&jsonschema.ForOptions{
				TypeSchemas: map[reflect.Type]*jsonschema.Schema{reflect.TypeFor[int](): schema},
			})
			return options.TypeSchemas[reflect.TypeFor[int]()]
		},
		"without defaults": CloneSchemaWithoutDefaults,
	} {
		t.Run(name, func(t *testing.T) {
			bounds := &jsonschema.Schema{}
			fields := reflect.ValueOf(bounds).Elem()
			var numericFields []int
			for i := range fields.NumField() {
				field := fields.Field(i)
				if field.Type() != reflect.TypeFor[*int]() && field.Type() != reflect.TypeFor[*float64]() {
					continue
				}
				value := reflect.New(field.Type().Elem())
				if value.Elem().Kind() == reflect.Int {
					value.Elem().SetInt(11)
				} else {
					value.Elem().SetFloat(11)
				}
				field.Set(value)
				numericFields = append(numericFields, i)
			}
			require.NotEmpty(t, numericFields)
			original := &jsonschema.Schema{
				Type:       "object",
				Comment:    "numeric keyword isolation " + name,
				Properties: map[string]*jsonschema.Schema{"bounds": bounds},
			}
			cloned := clone(original)
			clonedFields := reflect.ValueOf(cloned.Properties["bounds"]).Elem()
			for _, i := range numericFields {
				keyword := fields.Type().Field(i).Name
				assert.NotEqual(t, fields.Field(i).Pointer(), clonedFields.Field(i).Pointer(), keyword)
				assert.Equal(t, fields.Field(i).Elem().Interface(), clonedFields.Field(i).Elem().Interface(), keyword)
			}

			// Mutating caller-owned bounds must not race with cached-schema reads.
			var workers sync.WaitGroup
			workers.Add(2)
			go func() {
				defer workers.Done()
				for range 100 {
					for _, i := range numericFields {
						value := fields.Field(i).Elem()
						if value.Kind() == reflect.Int {
							value.SetInt(22)
						} else {
							value.SetFloat(22)
						}
					}
				}
			}()
			go func() {
				defer workers.Done()
				for range 100 {
					if _, err := json.Marshal(cloned); err != nil {
						t.Errorf("marshal detached schema: %v", err)
						return
					}
				}
			}()
			workers.Wait()
			for _, i := range numericFields {
				value := clonedFields.Field(i).Elem()
				if value.Kind() == reflect.Int {
					assert.Equal(t, int64(11), value.Int())
				} else {
					assert.Equal(t, float64(11), value.Float())
				}
			}
			assert.Nil(t, cloned.Minimum, "absent bounds must remain absent")
		})
	}
}

func TestCloneSchemaWithoutDefaultsTraversesAllChildKinds(t *testing.T) {
	schemaType := reflect.TypeFor[jsonschema.Schema]()
	for i := range schemaType.NumField() {
		field := schemaType.Field(i)
		if field.Type != reflect.TypeFor[*jsonschema.Schema]() &&
			field.Type != reflect.TypeFor[[]*jsonschema.Schema]() &&
			field.Type != reflect.TypeFor[map[string]*jsonschema.Schema]() {
			continue
		}
		t.Run(field.Name, func(t *testing.T) {
			original := &jsonschema.Schema{Default: json.RawMessage(`1`)}
			// A single deep chain makes repeated per-level traversal exponential.
			const depth = 48
			for range depth {
				parent := &jsonschema.Schema{Default: json.RawMessage(`1`)}
				target := reflect.ValueOf(parent).Elem().Field(i)
				switch field.Type {
				case reflect.TypeFor[*jsonschema.Schema]():
					target.Set(reflect.ValueOf(original))
				case reflect.TypeFor[[]*jsonschema.Schema]():
					target.Set(reflect.ValueOf([]*jsonschema.Schema{original}))
				case reflect.TypeFor[map[string]*jsonschema.Schema]():
					target.Set(reflect.ValueOf(map[string]*jsonschema.Schema{"child": original}))
				}
				original = parent
			}
			cloned := CloneSchemaWithoutDefaults(original)
			for level := 0; level <= depth; level++ {
				require.NotSame(t, original, cloned)
				assert.Equal(t, json.RawMessage(`1`), original.Default)
				assert.Nil(t, cloned.Default)
				var originalChild, clonedChild *jsonschema.Schema
				for _, child := range schemaChildren(original) {
					if child != nil {
						originalChild = child
					}
				}
				for _, child := range schemaChildren(cloned) {
					if child != nil {
						clonedChild = child
					}
				}
				original, cloned = originalChild, clonedChild
			}
			assert.Nil(t, original)
			assert.Nil(t, cloned)
		})
	}
}

func TestSchemaInferenceOptionsDeepCopyOverrides(t *testing.T) {
	type input struct {
		State string `json:"state"`
	}
	original := &jsonschema.Schema{
		Type: "string",
		Enum: []any{map[string]any{"values": []string{"open"}}},
		Extra: map[string]any{
			"nested": map[string]any{"values": []string{"option"}},
		},
	}
	options := cloneJSONSchemaForOptions(&jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[input](): original,
		},
	})
	original.Enum[0].(map[string]any)["values"].([]string)[0] = "mutated"
	original.Extra["nested"].(map[string]any)["values"].([]string)[0] = "mutated"

	cloned := options.TypeSchemas[reflect.TypeFor[input]()]
	assert.Equal(t, "open", cloned.Enum[0].(map[string]any)["values"].([]string)[0])
	assert.Equal(t, "option", cloned.Extra["nested"].(map[string]any)["values"].([]string)[0])
}

func TestRawExplicitSchemasReuseAnnotatedPointersAcrossRegistrations(t *testing.T) {
	original := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {Type: "string"},
		},
	}
	tool := NewServerTool(
		mcp.Tool{Name: "cached_raw_tool", InputSchema: original},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		},
	)
	var first *jsonschema.Schema
	for range 2 {
		toolCopy := tool.Tool
		AnnotateHeaderParams(&toolCopy)
		registration := tool.typedRegistration(&toolCopy)
		schema := registration.modernRuntimeTool.InputSchema.(*jsonschema.Schema)
		if first == nil {
			first = schema
		} else {
			assert.Same(t, first, schema)
		}
	}
	assert.Empty(t, original.Properties["owner"].Extra)
	assert.Empty(t, tool.Tool.InputSchema.(*jsonschema.Schema).Properties["owner"].Extra,
		"constructor caching must not change the public definition's schema metadata")
}

func TestTypedAnyInputPreservesSDKDefaultObjectSchemaInBothEras(t *testing.T) {
	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_any_input"},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, input any) (*mcp.CallToolResult, typedTestOutput, error) {
			values, ok := input.(map[string]any)
			require.True(t, ok)
			return nil, typedTestOutput{Query: values["query"].(string)}, nil
		},
	)
	require.Nil(t, tool.Tool.InputSchema, "the caller-owned static definition remains untouched")
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	tool.RegisterFunc(server, nil)
	modern := connectTypedTestClient(t, server, "")
	legacy := connectTypedTestClient(t, server, "2025-11-25")

	for _, session := range []*mcp.ClientSession{modern, legacy} {
		list, err := session.ListTools(context.Background(), nil)
		require.NoError(t, err)
		require.Len(t, list.Tools, 1)
		assert.JSONEq(t, `{"type":"object"}`, mustMarshalJSON(t, list.Tools[0].InputSchema))
	}
	list, err := modern.ListTools(context.Background(), nil)
	require.NoError(t, err)
	assert.NotNil(t, list.Tools[0].OutputSchema)
	list, err = legacy.ListTools(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, list.Tools[0].OutputSchema)

	for _, tc := range []struct {
		name    string
		session *mcp.ClientSession
		modern  bool
	}{
		{name: "modern call", session: modern, modern: true},
		{name: "legacy call", session: legacy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "typed_any_input",
				Arguments: map[string]any{"query": "accepted"},
			})
			require.NoError(t, err)
			require.False(t, result.IsError)
			if tc.modern {
				assert.JSONEq(t, `{"query":"accepted"}`, mustMarshalJSON(t, result.StructuredContent))
			} else {
				assert.Nil(t, result.StructuredContent)
			}
		})
	}
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

func TestTypedSchemaOptionsKeepAdvertisedSchemaAndRuntimeValidationSeparate(t *testing.T) {
	type input struct {
		State string `json:"state"`
	}
	type output struct {
		State string `json:"state"`
	}
	publicSchema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"state": {Type: "string", Enum: []any{"open"}},
		},
		Required: []string{"state"},
	}
	validationSchema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"state": {Type: "string"},
		},
		Required: []string{"state"},
	}
	tool := NewServerToolWithContextHandlerAndSchemaOptions(
		mcp.Tool{Name: "validation_override_tool", InputSchema: publicSchema},
		testToolsetMetadata("test"),
		func(_ context.Context, _ *mcp.CallToolRequest, args input) (*mcp.CallToolResult, output, error) {
			return nil, output(args), nil
		},
		TypedSchemaOptions{ValidationInputSchema: validationSchema},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	assert.Contains(t, mustMarshalJSON(t, list.Tools[0].InputSchema), `"enum":["open"]`)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "validation_override_tool",
		Arguments: map[string]any{"state": "legacy-value"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.JSONEq(t, `{"state":"legacy-value"}`, mustMarshalJSON(t, result.StructuredContent))
}

func TestValidationInputSchemaIsCachedAndDoesNotInjectDefaults(t *testing.T) {
	type paginationInput struct {
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	}
	type paginationOutput struct {
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	}
	publicSchema := &jsonschema.Schema{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"type":"object",
		"properties":{
			"page":{"type":"integer","minimum":1,"default":1},
			"per_page":{"type":"integer","minimum":1,"default":30}
		}
	}`), publicSchema))
	validationSchema := CloneSchemaWithoutDefaults(publicSchema)
	minimum := float64(0)
	validationSchema.Properties["page"].Minimum = &minimum
	validationSchema.Properties["per_page"].Minimum = &minimum

	handlerCalls := 0
	newTool := func(name string) ServerTool {
		return NewServerToolWithContextHandlerAndSchemaOptions(
			mcp.Tool{Name: name, InputSchema: publicSchema},
			testToolsetMetadata("test"),
			func(_ context.Context, _ *mcp.CallToolRequest, args paginationInput) (*mcp.CallToolResult, paginationOutput, error) {
				handlerCalls++
				return nil, paginationOutput(args), nil
			},
			TypedSchemaOptions{ValidationInputSchema: validationSchema},
		)
	}
	tool := newTool("pagination_tool")
	registration := tool.typedRegistration(&tool.Tool)
	runtimeSchema := registration.modernRuntimeTool.InputSchema.(*jsonschema.Schema)
	otherTool := newTool("other_pagination_tool")
	otherRegistration := otherTool.typedRegistration(&otherTool.Tool)
	assert.Same(t, runtimeSchema, otherRegistration.modernRuntimeTool.InputSchema,
		"equal runtime schemas must reuse a process-cached immutable pointer")
	assert.Contains(t, mustMarshalJSON(t, registration.modernTool.InputSchema), `"minimum":1`)
	assert.Contains(t, mustMarshalJSON(t, registration.modernTool.InputSchema), `"default":30`)
	assert.Contains(t, mustMarshalJSON(t, runtimeSchema), `"minimum":0`)
	assert.NotContains(t, mustMarshalJSON(t, runtimeSchema), `"default"`)

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "pagination_tool",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.JSONEq(t, `{"page":0,"per_page":0}`, mustMarshalJSON(t, result.StructuredContent),
		"JSON Schema defaults are validation metadata and must not populate omitted query parameters")
	assert.Equal(t, 1, handlerCalls)

	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "pagination_tool",
		Arguments: map[string]any{"page": -1},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, 1, handlerCalls, "runtime validation must continue enforcing the relaxed schema's minimum")
}

func TestTypedSchemaEnumsAndPreflightRunBeforeSDKValidation(t *testing.T) {
	type input struct {
		State string `json:"state"`
	}
	type output struct {
		State string `json:"state"`
	}
	type preparedKey struct{}
	handlerCalls := 0
	preflightCalls := 0
	tool := NewServerToolWithContextHandlerAndSchemaOptions(
		mcp.Tool{Name: "preflight_schema_tool"},
		testToolsetMetadata("test"),
		func(ctx context.Context, _ *mcp.CallToolRequest, args input) (*mcp.CallToolResult, output, error) {
			handlerCalls++
			require.Equal(t, "prepared", ctx.Value(preparedKey{}))
			return nil, output(args), nil
		},
		TypedSchemaOptions{
			OutputEnums: []SchemaEnum{{Path: "state", Values: []string{"open", "closed"}}},
			Preflight: func(ctx context.Context, req *mcp.CallToolRequest) (context.Context, *mcp.CallToolResult, error) {
				preflightCalls++
				if bytes.Contains(req.Params.Arguments, []byte(`"state":42`)) {
					return ctx, &mcp.CallToolResult{
						Content: []mcp.Content{&mcp.TextContent{Text: "preflight rejected raw arguments"}},
						IsError: true,
					}, nil
				}
				return context.WithValue(ctx, preparedKey{}, "prepared"), nil, nil
			},
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)
	session := connectTypedTestClient(t, server, "")

	invalid, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "preflight_schema_tool",
		Arguments: map[string]any{"state": 42},
	})
	require.NoError(t, err)
	require.True(t, invalid.IsError)
	require.Len(t, invalid.Content, 1)
	assert.Equal(t, "preflight rejected raw arguments", invalid.Content[0].(*mcp.TextContent).Text)
	assert.Nil(t, invalid.StructuredContent)
	assert.Zero(t, handlerCalls)

	valid, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "preflight_schema_tool",
		Arguments: map[string]any{"state": "open"},
	})
	require.NoError(t, err)
	require.False(t, valid.IsError)
	assert.JSONEq(t, `{"state":"open"}`, mustMarshalJSON(t, valid.StructuredContent))
	assert.Equal(t, 2, preflightCalls)
	assert.Equal(t, 1, handlerCalls)

	_, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "preflight_schema_tool",
		Arguments: map[string]any{"state": "unknown"},
	})
	require.ErrorContains(t, err, "validating tool output")
	assert.Equal(t, 2, handlerCalls, "the SDK must reject output enum violations before returning structured success")
}

func TestTypedPreflightRunsAfterAvailabilityAndAuthorization(t *testing.T) {
	for _, blockedBy := range []string{"availability", "authorization"} {
		t.Run(blockedBy, func(t *testing.T) {
			preflightCalls := 0
			handlerCalls := 0
			tool := NewServerToolWithContextHandlerAndSchemaOptions(
				mcp.Tool{Name: "guarded_preflight"},
				testToolsetMetadata("test"),
				func(context.Context, *mcp.CallToolRequest, typedTestInput) (*mcp.CallToolResult, typedTestOutput, error) {
					handlerCalls++
					return nil, typedTestOutput{}, nil
				},
				TypedSchemaOptions{
					Preflight: func(ctx context.Context, _ *mcp.CallToolRequest) (context.Context, *mcp.CallToolResult, error) {
						preflightCalls++
						return ctx, &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "preflight ran"}}}, nil
					},
				},
			)
			var middleware []ToolHandlerMiddleware
			if blockedBy == "availability" {
				tool.MinimumProtocolVersion = "2027-01-01"
			} else {
				middleware = append(middleware, func(_ mcp.ToolHandler) mcp.ToolHandler {
					return func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
						return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "unauthorized"}}}, nil
					}
				})
			}
			server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
			tool.RegisterFunc(server, nil, middleware...)
			session := connectTypedTestClient(t, server, "")
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name})
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Zero(t, preflightCalls)
			assert.Zero(t, handlerCalls)
			assert.NotContains(t, mustMarshalJSON(t, result), "preflight ran")
			if blockedBy == "authorization" {
				assert.Equal(t, "unauthorized", result.Content[0].(*mcp.TextContent).Text)
			}
		})
	}
}

func TestTypedPreflightErrorsPreserveToolAndProtocolErrorSemantics(t *testing.T) {
	for _, protocolError := range []bool{false, true} {
		t.Run(fmt.Sprintf("protocol=%t", protocolError), func(t *testing.T) {
			tool := NewServerToolWithContextHandlerAndSchemaOptions(
				mcp.Tool{Name: "preflight_error"},
				testToolsetMetadata("test"),
				func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, typedTestOutput, error) {
					t.Fatal("preflight must short-circuit the handler")
					return nil, typedTestOutput{}, nil
				},
				TypedSchemaOptions{
					Preflight: func(ctx context.Context, _ *mcp.CallToolRequest) (context.Context, *mcp.CallToolResult, error) {
						if protocolError {
							return ctx, nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "protocol rejected"}
						}
						return ctx, nil, errors.New("preflight failed")
					},
				},
			)
			server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
			tool.RegisterFunc(server, nil)
			session := connectTypedTestClient(t, server, "")
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Tool.Name})
			if protocolError {
				require.ErrorContains(t, err, "protocol rejected")
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, result.Content[0].(*mcp.TextContent).Text, "preflight failed")
				assert.Nil(t, result.StructuredContent)
			}
		})
	}
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

func TestInvalidArgumentsPreservesToolInputErrorMessage(t *testing.T) {
	result := invalidArgumentsResult(fmt.Errorf(
		"normalize tool arguments: %w",
		&ToolInputError{Message: "missing required parameter: method"},
	))
	require.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "missing required parameter: method", result.Content[0].(*mcp.TextContent).Text)
}

func TestTypedInputNormalizerReusesScopeNormalizedArguments(t *testing.T) {
	normalizerCalls := 0
	tool := &mcp.Tool{Name: "normalized_scope_tool"}
	registration := &typedToolRegistration{
		name:              tool.Name,
		modernTool:        tool,
		legacyTool:        tool,
		modernRuntimeTool: tool,
		legacyRuntimeTool: tool,
		hasTypedOutput:    true,
		inputNormalizer: func(_ json.RawMessage) (json.RawMessage, error) {
			normalizerCalls++
			return json.RawMessage(`{"state":"CHANGED"}`), nil
		},
	}
	wrapped := typedOutputMiddleware(&typedToolRegistrationSet{byName: map[string]*typedToolRegistration{
		tool.Name: registration,
	}})(func(_ context.Context, _ string, request mcp.Request) (mcp.Result, error) {
		call := request.(*mcp.CallToolRequest)
		assert.JSONEq(t, `{"state":"OPEN"}`, string(call.Params.Arguments))
		return &mcp.CallToolResult{}, nil
	})
	ctx := ghcontext.WithMCPMethodInfo(context.Background(), &ghcontext.MCPMethodInfo{
		Method:              MCPMethodToolsCall,
		ItemName:            tool.Name,
		ArgumentsNormalized: true,
		NormalizedArguments: json.RawMessage(`{"state":"OPEN"}`),
	})
	result, err := wrapped(ctx, MCPMethodToolsCall, &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      tool.Name,
			Arguments: json.RawMessage(`{"state":"open"}`),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Zero(t, normalizerCalls, "scope and typed adapters must not run a non-idempotent normalizer twice")
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
	type enumOutput struct {
		Method string `json:"method"`
	}
	errorResult := NewServerToolWithContextHandler(
		mcp.Tool{
			Name: "typed_error_result_tool",
			OutputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {Type: "string", Enum: []any{"list_projects"}},
				},
				Required: []string{"method"},
			},
		},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, *enumOutput, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "project list failed"}},
				IsError: true,
			}, nil, nil
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
	errorResult.RegisterFunc(server, nil)
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
	require.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "typed handler failed", result.Content[0].(*mcp.TextContent).Text)

	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_error_result_tool"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Nil(t, result.StructuredContent)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "project list failed", result.Content[0].(*mcp.TextContent).Text)
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

	modern := connectTypedTestClient(t, server, "")
	legacy := connectTypedTestClient(t, server, "2025-11-25")
	assert.Equal(t, ProtocolVersionMultiRoundTrip, modern.InitializeResult().ProtocolVersion)
	assert.Equal(t, "2025-11-25", legacy.InitializeResult().ProtocolVersion)

	legacyList, err := legacy.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, legacyList.Tools, 1)
	assert.Nil(t, legacyList.Tools[0].OutputSchema)

	modernList, err := modern.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, modernList.Tools, 1)
	assert.NotNil(t, modernList.Tools[0].OutputSchema)
	assert.JSONEq(t, `{"type":["null","array"],"items":{"type":"string"}}`, mustMarshalJSON(t, modernList.Tools[0].OutputSchema))

	legacyResult, err := legacy.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_array_tool"})
	require.NoError(t, err)
	assert.Nil(t, legacyResult.StructuredContent)
	require.Len(t, legacyResult.Content, 1, "the SDK's array fallback must not duplicate the old text")
	assert.Equal(t, "legacy array text", legacyResult.Content[0].(*mcp.TextContent).Text)

	modernResult, err := modern.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_array_tool"})
	require.NoError(t, err)
	assert.JSONEq(t, `["one","two"]`, mustMarshalJSON(t, modernResult.StructuredContent))
	require.Len(t, modernResult.Content, 1)
	assert.Equal(t, `["one","two"]`, modernResult.Content[0].(*mcp.TextContent).Text)

	legacyList, err = legacy.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, legacyList.Tools, 1)
	assert.Nil(t, legacyList.Tools[0].OutputSchema, "modern discovery must not leak schema to an active legacy session")

	legacyResult, err = legacy.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_array_tool"})
	require.NoError(t, err)
	assert.Nil(t, legacyResult.StructuredContent)
	require.Len(t, legacyResult.Content, 1)
	assert.Equal(t, "legacy array text", legacyResult.Content[0].(*mcp.TextContent).Text)

	modernResult, err = modern.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_array_tool"})
	require.NoError(t, err)
	assert.JSONEq(t, `["one","two"]`, mustMarshalJSON(t, modernResult.StructuredContent))
	require.Len(t, modernResult.Content, 1)
	assert.Equal(t, `["one","two"]`, modernResult.Content[0].(*mcp.TextContent).Text)

	type concurrentCall struct {
		list   *mcp.ListToolsResult
		result *mcp.CallToolResult
		err    error
	}
	type clientCalls struct {
		modern bool
		calls  []concurrentCall
	}
	const iterations = 5
	start := make(chan struct{})
	calls := make(chan clientCalls, 2)
	var workers sync.WaitGroup
	runClient := func(session *mcp.ClientSession, isModern bool) {
		defer workers.Done()
		<-start
		run := clientCalls{modern: isModern, calls: make([]concurrentCall, 0, iterations)}
		for range iterations {
			list, listErr := session.ListTools(context.Background(), nil)
			if listErr != nil {
				run.calls = append(run.calls, concurrentCall{err: listErr})
				break
			}
			result, callErr := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_array_tool"})
			run.calls = append(run.calls, concurrentCall{list: list, result: result, err: callErr})
			if callErr != nil {
				break
			}
		}
		calls <- run
	}
	workers.Add(2)
	go runClient(modern, true)
	go runClient(legacy, false)
	close(start)
	go func() {
		workers.Wait()
		close(calls)
	}()
	for run := range calls {
		require.Len(t, run.calls, iterations)
		for _, call := range run.calls {
			require.NoError(t, call.err)
			require.Len(t, call.list.Tools, 1)
			if run.modern {
				assert.NotNil(t, call.list.Tools[0].OutputSchema)
				assert.JSONEq(t, `["one","two"]`, mustMarshalJSON(t, call.result.StructuredContent))
				require.Len(t, call.result.Content, 1)
				assert.Equal(t, `["one","two"]`, call.result.Content[0].(*mcp.TextContent).Text)
			} else {
				assert.Nil(t, call.list.Tools[0].OutputSchema)
				assert.Nil(t, call.result.StructuredContent)
				require.Len(t, call.result.Content, 1)
				assert.Equal(t, "legacy array text", call.result.Content[0].(*mcp.TextContent).Text)
			}
		}
	}

	require.Nil(t, tool.Tool.OutputSchema, "registration must not mutate the shared definition")
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

func TestTypedLegacyOutputGateRemovesFallbackWithEmptyHandlerContent(t *testing.T) {
	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_empty_content_tool"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, []string, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{}}, []string{"structured"}, nil
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)
	legacy := connectTypedTestClient(t, server, "2025-11-25")

	result, err := legacy.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_empty_content_tool"})
	require.NoError(t, err)
	assert.Nil(t, result.StructuredContent)
	assert.Empty(t, result.Content, "the SDK fallback is not part of the legacy handler result")
}

func TestTypedOutputMiddlewareShortCircuitPreservesErrorResult(t *testing.T) {
	type output struct {
		Required string `json:"required"`
	}
	handlerCalls := 0
	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_short_circuit_tool"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, output, error) {
			handlerCalls++
			return nil, output{Required: "unexpected"}, nil
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil, func(_ mcp.ToolHandler) mcp.ToolHandler {
		return func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "blocked"}},
				IsError: true,
			}, nil
		}
	})
	session := connectTypedTestClient(t, server, "")

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_short_circuit_tool"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "blocked", result.Content[0].(*mcp.TextContent).Text)
	assert.Nil(t, result.StructuredContent)
	assert.Zero(t, handlerCalls)
}

func TestTypedOutputGateUsesCurrentRequestVersionNotInitializeRequest(t *testing.T) {
	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_initialize_version_tool"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, []string, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "legacy initialize text"}},
			}, []string{"one"}, nil
		},
	)
	server := mcp.NewServer(
		&mcp.Implementation{Name: "test-server", Version: "v0.0.1"},
		&mcp.ServerOptions{SupportedProtocolVersions: []string{"2025-11-25"}},
	)
	tool.RegisterFunc(server, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	transport := protocolVersionTransport{
		Transport:       clientTransport,
		protocolVersion: ProtocolVersionMultiRoundTrip,
	}
	session, err := client.Connect(context.Background(), transport, &mcp.ClientSessionOptions{
		ProtocolVersion: "2025-11-25",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	require.Equal(t, "2025-11-25", session.InitializeResult().ProtocolVersion)

	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	assert.Nil(t, list.Tools[0].OutputSchema)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_initialize_version_tool"})
	require.NoError(t, err)
	assert.Nil(t, result.StructuredContent)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "legacy initialize text", result.Content[0].(*mcp.TextContent).Text)
}

func TestTypedNullableOutputPreservesProtocolSemantics(t *testing.T) {
	type output struct {
		Value *string `json:"value"`
	}
	tool := NewServerToolWithContextHandler(
		mcp.Tool{Name: "typed_nullable_tool"},
		testToolsetMetadata("test"),
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, output, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "legacy null text"}},
			}, output{}, nil
		},
	)
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	tool.RegisterFunc(server, nil)
	modern := connectTypedTestClient(t, server, "")
	legacy := connectTypedTestClient(t, server, "2025-11-25")

	modernResult, err := modern.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_nullable_tool"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"value":null}`, mustMarshalJSON(t, modernResult.StructuredContent))
	require.Len(t, modernResult.Content, 1)
	assert.JSONEq(t, `{"value":null}`, modernResult.Content[0].(*mcp.TextContent).Text)

	legacyResult, err := legacy.CallTool(context.Background(), &mcp.CallToolParams{Name: "typed_nullable_tool"})
	require.NoError(t, err)
	assert.Nil(t, legacyResult.StructuredContent)
	require.Len(t, legacyResult.Content, 1)
	assert.Equal(t, "legacy null text", legacyResult.Content[0].(*mcp.TextContent).Text)
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
	inputSchema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {Type: "string", Extra: map[string]any{"x-mcp-header": "owner"}},
		},
	}
	modernTool := &mcp.Tool{
		Name:         "typed_tool",
		InputSchema:  inputSchema,
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
	legacyTool := &mcp.Tool{Name: "typed_tool", InputSchema: inputSchema}
	registration := &typedToolRegistration{
		name:              "typed_tool",
		modernTool:        modernTool,
		legacyTool:        legacyTool,
		modernRuntimeTool: modernTool,
		legacyRuntimeTool: legacyTool,
		hasTypedOutput:    true,
	}
	wrapped := typedOutputMiddleware(&typedToolRegistrationSet{byName: map[string]*typedToolRegistration{
		"typed_tool": registration,
	}})(func(_ context.Context, method string, _ mcp.Request) (mcp.Result, error) {
		require.Equal(t, MCPMethodToolsList, method)
		return &mcp.ListToolsResult{Tools: []*mcp.Tool{modernTool}}, nil
	})

	for _, tc := range []struct {
		name            string
		protocolVersion string
		wantTool        *mcp.Tool
	}{
		{name: "stateless modern", protocolVersion: ProtocolVersionMultiRoundTrip, wantTool: modernTool},
		{name: "stateless legacy", protocolVersion: "2025-11-25", wantTool: legacyTool},
		{name: "unknown version defaults to legacy", protocolVersion: "2027-01-01", wantTool: legacyTool},
		{name: "absent version defaults to legacy", wantTool: legacyTool},
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
			require.Len(t, list.Tools, 1)
			assert.Same(t, tc.wantTool, list.Tools[0])
			assert.Contains(t, mustMarshalJSON(t, list.Tools[0].InputSchema), `"x-mcp-header":"owner"`,
				"legacy and unknown-version variants must retain routing metadata")
		})
	}

	inputRequired := &mcp.CallToolResult{
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

	typedRegistration := &typedToolRegistration{
		name:              "typed_tool",
		modernTool:        modernTool,
		legacyTool:        legacyTool,
		modernRuntimeTool: modernTool,
		legacyRuntimeTool: legacyTool,
		hasTypedOutput:    true,
	}
	typedMiddleware := typedOutputMiddleware(&typedToolRegistrationSet{byName: map[string]*typedToolRegistration{
		"typed_tool": typedRegistration,
	}})(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return inputRequired, nil
	})
	for _, tc := range []struct {
		name            string
		protocolVersion string
		wantStructured  bool
	}{
		{name: "modern", protocolVersion: ProtocolVersionMultiRoundTrip, wantStructured: true},
		{name: "legacy", protocolVersion: "2025-11-25"},
	} {
		t.Run("typed awaiting form "+tc.name, func(t *testing.T) {
			meta := mcp.Meta{}
			if tc.protocolVersion != "" {
				meta[mcp.MetaKeyProtocolVersion] = tc.protocolVersion
			}
			result, err := typedMiddleware(context.Background(), MCPMethodToolsCall, &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{Name: "typed_tool", Meta: meta},
			})
			require.NoError(t, err)
			callResult := result.(*mcp.CallToolResult)
			if tc.wantStructured {
				assert.Same(t, inputRequired, callResult)
				assert.Equal(t, map[string]any{"awaiting": true}, callResult.StructuredContent)
			} else {
				assert.NotSame(t, inputRequired, callResult)
				assert.Nil(t, callResult.StructuredContent, "legacy typed tools must not expose structured output")
			}
			assert.Equal(t, inputRequired.InputRequests, callResult.InputRequests)
		})
	}
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
	assert.JSONEq(t, `{"type":"object"}`, mustMarshalJSON(t, list.Tools[0].OutputSchema), "raw tool schemas must remain unchanged")

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
