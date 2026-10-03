package inventory

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProtocolEra selects the wire-compatible registration variant.
type ProtocolEra uint8

const (
	// ProtocolEraDynamic selects a variant at request time.
	ProtocolEraDynamic ProtocolEra = iota
	// ProtocolEraLegacy registers without an output schema or generated output.
	ProtocolEraLegacy
	// ProtocolEraModern registers the 2026-07-28 output-schema variant.
	ProtocolEraModern
)

// ProtocolEraForVersion selects modern behavior only for the exact supported
// version. Unknown and future versions remain on the legacy contract.
func ProtocolEraForVersion(version string) ProtocolEra {
	if version == ProtocolVersionMultiRoundTrip {
		return ProtocolEraModern
	}
	return ProtocolEraLegacy
}

// SchemaEnum adds an enum to a property of an inferred schema. Path is a
// dot-separated JSON Schema path; use "items" to descend into array items.
type SchemaEnum struct {
	Path   string
	Values []string
}

// EnumSchema returns a string schema constrained to values.
func EnumSchema(values ...string) *jsonschema.Schema {
	enum := make([]any, len(values))
	for i, value := range values {
		enum[i] = value
	}
	return &jsonschema.Schema{Type: "string", Enum: enum}
}

// WithEnum clones schema and adds a string enum at the requested schema path.
// Array item schemas are addressed with the "items" path segment.
func WithEnum(schema *jsonschema.Schema, path string, values ...string) (*jsonschema.Schema, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	segments := strings.Split(path, ".")
	if path == "" {
		return nil, fmt.Errorf("schema path is empty")
	}
	clonedSchema := schema.CloneSchemas()
	current := clonedSchema
	for _, segment := range segments {
		switch segment {
		case "items":
			if current.Items == nil {
				return nil, fmt.Errorf("schema path %q has no items schema", path)
			}
			current = current.Items
		default:
			if current.Properties == nil || current.Properties[segment] == nil {
				return nil, fmt.Errorf("schema path %q has no property %q", path, segment)
			}
			current = current.Properties[segment]
		}
	}
	current.Type = "string"
	current.Types = nil
	current.Enum = make([]any, len(values))
	for i, value := range values {
		current.Enum[i] = value
	}
	return clonedSchema, nil
}

type inferredSchemaKey struct {
	goType      reflect.Type
	options     string
	enums       string
	inputSchema bool
}

type inferredSchemaEntry struct {
	once   sync.Once
	schema *jsonschema.Schema
	err    error
}

var inferredSchemaCache sync.Map
var explicitSchemaCache sync.Map
var ownedSchemaPointers sync.Map
var annotatedSchemaCache sync.Map

// CloneSchemaWithoutDefaults returns a deep schema copy with default keywords
// removed. The MCP SDK applies input-schema defaults before decoding arguments,
// so use this for runtime validation schemas when omitted arguments must stay
// omitted. The original advertised schema is unchanged.
func CloneSchemaWithoutDefaults(schema *jsonschema.Schema) *jsonschema.Schema {
	clonedSchema := schema.CloneSchemas()
	removeSchemaDefaults(clonedSchema)
	return clonedSchema
}

func removeSchemaDefaults(schema *jsonschema.Schema) {
	if schema == nil {
		return
	}
	schema.Default = nil
	for _, children := range []map[string]*jsonschema.Schema{
		schema.Defs,
		schema.Definitions,
		schema.Properties,
		schema.PatternProperties,
		schema.DependentSchemas,
		schema.DependencySchemas,
	} {
		for _, child := range children {
			removeSchemaDefaults(child)
		}
	}
	for _, child := range []*jsonschema.Schema{
		schema.Items,
		schema.AdditionalItems,
		schema.Contains,
		schema.UnevaluatedItems,
		schema.AdditionalProperties,
		schema.PropertyNames,
		schema.UnevaluatedProperties,
		schema.Not,
		schema.If,
		schema.Then,
		schema.Else,
		schema.ContentSchema,
	} {
		removeSchemaDefaults(child)
	}
	for _, children := range [][]*jsonschema.Schema{
		schema.PrefixItems,
		schema.ItemsArray,
		schema.AllOf,
		schema.AnyOf,
		schema.OneOf,
	} {
		for _, child := range children {
			removeSchemaDefaults(child)
		}
	}
}

// CachedSchema clones an explicit schema once per process and returns the
// immutable shared pointer. It is useful for schemas prepared by tool
// definitions that are reconstructed for request-scoped servers.
func CachedSchema(schema *jsonschema.Schema) (*jsonschema.Schema, error) {
	if schema == nil {
		return nil, nil
	}
	if _, owned := ownedSchemaPointers.Load(schema); owned {
		return schema, nil
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal explicit schema: %w", err)
	}
	key := string(encoded)
	if cached, ok := explicitSchemaCache.Load(key); ok {
		return cached.(*jsonschema.Schema), nil
	}
	clonedSchema := schema.CloneSchemas()
	cached, _ := explicitSchemaCache.LoadOrStore(key, clonedSchema)
	result := cached.(*jsonschema.Schema)
	ownedSchemaPointers.Store(result, struct{}{})
	return result, nil
}

// CachedSchemaFor infers a schema once per process for the Go type and
// immutable inference options. The returned schema is shared and must not be
// mutated. Enum overrides are applied while constructing the cached value.
func CachedSchemaFor[T any](options *jsonschema.ForOptions, enums ...SchemaEnum) (*jsonschema.Schema, error) {
	return cachedSchemaFor(reflect.TypeFor[T](), options, enums, false)
}

// CachedInputSchemaFor is like CachedSchemaFor and adds the standard owner/repo
// routing annotations before caching the immutable schema.
func CachedInputSchemaFor[T any](options *jsonschema.ForOptions, enums ...SchemaEnum) (*jsonschema.Schema, error) {
	return cachedSchemaFor(reflect.TypeFor[T](), options, enums, true)
}

func cachedSchemaFor(goType reflect.Type, options *jsonschema.ForOptions, enums []SchemaEnum, inputSchema bool) (*jsonschema.Schema, error) {
	optionsKey, err := schemaOptionsKey(options)
	if err != nil {
		return nil, err
	}
	enumKey, err := json.Marshal(enums)
	if err != nil {
		return nil, fmt.Errorf("marshal schema enum overrides: %w", err)
	}
	key := inferredSchemaKey{
		goType:      goType,
		options:     optionsKey,
		enums:       string(enumKey),
		inputSchema: inputSchema,
	}
	entryValue, _ := inferredSchemaCache.LoadOrStore(key, &inferredSchemaEntry{})
	entry := entryValue.(*inferredSchemaEntry)
	entry.once.Do(func() {
		entry.schema, entry.err = jsonschema.ForType(goType, options)
		if entry.err != nil {
			return
		}
		for _, override := range enums {
			entry.schema, entry.err = WithEnum(entry.schema, override.Path, override.Values...)
			if entry.err != nil {
				return
			}
		}
		if inputSchema {
			tool := mcp.Tool{InputSchema: entry.schema}
			AnnotateHeaderParams(&tool)
			entry.schema = tool.InputSchema.(*jsonschema.Schema)
		}
	})
	return entry.schema, entry.err
}

func schemaOptionsKey(options *jsonschema.ForOptions) (string, error) {
	if options == nil {
		return "", nil
	}
	type schemaEntry struct {
		Type   string          `json:"type"`
		Schema json.RawMessage `json:"schema"`
	}
	types := make([]reflect.Type, 0, len(options.TypeSchemas))
	for goType := range options.TypeSchemas {
		types = append(types, goType)
	}
	sort.Slice(types, func(i, j int) bool {
		return schemaTypeKey(types[i]) < schemaTypeKey(types[j])
	})
	entries := make([]schemaEntry, 0, len(types))
	for _, goType := range types {
		encoded, err := json.Marshal(options.TypeSchemas[goType])
		if err != nil {
			return "", fmt.Errorf("marshal schema override for %s: %w", goType, err)
		}
		entries = append(entries, schemaEntry{
			Type:   schemaTypeKey(goType),
			Schema: encoded,
		})
	}
	encoded, err := json.Marshal(struct {
		IgnoreInvalidTypes bool          `json:"ignoreInvalidTypes"`
		TypeSchemas        []schemaEntry `json:"typeSchemas"`
	}{
		IgnoreInvalidTypes: options.IgnoreInvalidTypes,
		TypeSchemas:        entries,
	})
	if err != nil {
		return "", fmt.Errorf("marshal schema inference options: %w", err)
	}
	return string(encoded), nil
}

func schemaTypeKey(goType reflect.Type) string {
	if goType.Name() != "" {
		return goType.PkgPath() + "." + goType.Name()
	}
	return goType.String()
}
