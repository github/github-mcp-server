package inventory

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

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

// ProtocolEraForVersion selects modern behavior for SDK-supported protocol
// versions from 2026-07-28 onward. Unknown versions remain legacy.
func ProtocolEraForVersion(version string) ProtocolEra {
	return protocolEraForSupportedVersion(version, mcp.SupportedProtocolVersions())
}

func protocolEraForSupportedVersion(version string, supported []string) ProtocolEra {
	if _, err := time.Parse(time.DateOnly, version); err != nil {
		return ProtocolEraLegacy
	}
	if version < ProtocolVersionMultiRoundTrip || !slices.Contains(supported, version) {
		return ProtocolEraLegacy
	}
	return ProtocolEraModern
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
	clonedSchema := CloneSchema(schema)
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
var headerInputSchemaCache sync.Map
var defaultObjectInputSchemaEntry inferredSchemaEntry

type resolvedInputSchemaEntry struct {
	once     sync.Once
	resolved *jsonschema.Resolved
	err      error
}

var resolvedInputSchemaCache sync.Map

func cachedInputSchema(value any) (*jsonschema.Schema, error) {
	schema, ok := value.(*jsonschema.Schema)
	if !ok {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		schema = new(jsonschema.Schema)
		if err := json.Unmarshal(encoded, schema); err != nil {
			return nil, err
		}
	}
	schema, err := CachedSchema(schema)
	if err != nil {
		return nil, err
	}
	if schema.Type != "object" {
		return nil, fmt.Errorf("input schema must have type \"object\"")
	}
	return schema, nil
}

func cachedResolvedInputSchema(value any) (*jsonschema.Resolved, error) {
	schema, err := cachedInputSchema(value)
	if err != nil {
		return nil, err
	}
	entryValue, _ := resolvedInputSchemaCache.LoadOrStore(schema, &resolvedInputSchemaEntry{})
	entry := entryValue.(*resolvedInputSchemaEntry)
	entry.once.Do(func() {
		entry.resolved, entry.err = schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	})
	return entry.resolved, entry.err
}

func cachedObjectInputSchema() (*jsonschema.Schema, error) {
	defaultObjectInputSchemaEntry.once.Do(func() {
		defaultObjectInputSchemaEntry.schema, defaultObjectInputSchemaEntry.err = CachedSchema(&jsonschema.Schema{Type: "object"})
	})
	return defaultObjectInputSchemaEntry.schema, defaultObjectInputSchemaEntry.err
}

// Keep header bindings visible to the SDK's HTTP transport without restoring
// required fields, defaults, or other body constraints ahead of tool guards.
func cachedHeaderInputSchema(value any) (*jsonschema.Schema, error) {
	schema, err := cachedInputSchema(value)
	if err != nil {
		return nil, err
	}
	entryValue, _ := headerInputSchemaCache.LoadOrStore(schema, &inferredSchemaEntry{})
	entry := entryValue.(*inferredSchemaEntry)
	entry.once.Do(func() {
		headerSchema := headerPropertySchema(schema)
		headerSchema.Type = "object"
		entry.schema, entry.err = CachedSchema(headerSchema)
	})
	return entry.schema, entry.err
}

func headerPropertySchema(schema *jsonschema.Schema) *jsonschema.Schema {
	result := &jsonschema.Schema{}
	if annotation, ok := schema.Extra["x-mcp-header"]; ok {
		// The SDK requires a primitive type when validating header annotations.
		result.Type = schema.Type
		result.Extra = map[string]any{"x-mcp-header": annotation}
	}
	for name, property := range schema.Properties {
		if property == nil {
			continue
		}
		child := headerPropertySchema(property)
		if len(child.Extra) == 0 && len(child.Properties) == 0 {
			continue
		}
		if result.Properties == nil {
			result.Properties = make(map[string]*jsonschema.Schema)
		}
		result.Properties[name] = child
	}
	return result
}

// CloneSchemaWithoutDefaults returns a deep schema copy with default keywords
// removed. The MCP SDK applies input-schema defaults before decoding arguments,
// so use this for runtime validation schemas when omitted arguments must stay
// omitted. The original advertised schema is unchanged.
func CloneSchemaWithoutDefaults(schema *jsonschema.Schema) *jsonschema.Schema {
	clonedSchema := CloneSchema(schema)
	removeSchemaDefaults(clonedSchema)
	return clonedSchema
}

// CloneSchema deep-copies schema nodes, numeric bounds, and mutable metadata.
func CloneSchema(schema *jsonschema.Schema) *jsonschema.Schema {
	clonedSchema := schema.CloneSchemas()
	cloneSchemaMetadata(clonedSchema)
	return clonedSchema
}

type schemaValueVisit struct {
	typeOf reflect.Type
	ptr    uintptr
}

func cloneSchemaMetadata(schema *jsonschema.Schema) {
	if schema == nil {
		return
	}
	schema.MultipleOf = cloneSchemaPointer(schema.MultipleOf)
	schema.Minimum = cloneSchemaPointer(schema.Minimum)
	schema.Maximum = cloneSchemaPointer(schema.Maximum)
	schema.ExclusiveMinimum = cloneSchemaPointer(schema.ExclusiveMinimum)
	schema.ExclusiveMaximum = cloneSchemaPointer(schema.ExclusiveMaximum)
	schema.MinLength = cloneSchemaPointer(schema.MinLength)
	schema.MaxLength = cloneSchemaPointer(schema.MaxLength)
	schema.MinItems = cloneSchemaPointer(schema.MinItems)
	schema.MaxItems = cloneSchemaPointer(schema.MaxItems)
	schema.MinContains = cloneSchemaPointer(schema.MinContains)
	schema.MaxContains = cloneSchemaPointer(schema.MaxContains)
	schema.MinProperties = cloneSchemaPointer(schema.MinProperties)
	schema.MaxProperties = cloneSchemaPointer(schema.MaxProperties)
	schema.Types = slices.Clone(schema.Types)
	schema.Enum = cloneSchemaValue(reflect.ValueOf(schema.Enum), make(map[schemaValueVisit]reflect.Value)).Interface().([]any)
	schema.Default = slices.Clone(schema.Default)
	if schema.Const != nil {
		value := any(nil)
		if *schema.Const != nil {
			value = cloneSchemaValue(reflect.ValueOf(*schema.Const), make(map[schemaValueVisit]reflect.Value)).Interface()
		}
		schema.Const = &value
	}
	schema.Examples = cloneSchemaValue(reflect.ValueOf(schema.Examples), make(map[schemaValueVisit]reflect.Value)).Interface().([]any)
	schema.Required = slices.Clone(schema.Required)
	if schema.Vocabulary != nil {
		schema.Vocabulary = maps.Clone(schema.Vocabulary)
	}
	if schema.DependencyStrings != nil {
		dependencies := make(map[string][]string, len(schema.DependencyStrings))
		for key, values := range schema.DependencyStrings {
			dependencies[key] = slices.Clone(values)
		}
		schema.DependencyStrings = dependencies
	}
	if schema.DependentRequired != nil {
		dependencies := make(map[string][]string, len(schema.DependentRequired))
		for key, values := range schema.DependentRequired {
			dependencies[key] = slices.Clone(values)
		}
		schema.DependentRequired = dependencies
	}
	schema.PropertyOrder = slices.Clone(schema.PropertyOrder)
	if schema.Extra != nil {
		schema.Extra = cloneSchemaValue(reflect.ValueOf(schema.Extra), make(map[schemaValueVisit]reflect.Value)).Interface().(map[string]any)
	}
	for _, child := range schemaChildren(schema) {
		cloneSchemaMetadata(child)
	}
}

func cloneSchemaPointer[T int | float64](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// cloneSchemaValue preserves the concrete metadata types while detaching
// nested mutable collections from schemas retained by callers.
func cloneSchemaValue(value reflect.Value, visited map[schemaValueVisit]reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := cloneSchemaValue(value.Elem(), visited)
		result := reflect.New(value.Type()).Elem()
		result.Set(cloned)
		return result
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		visit := schemaValueVisit{typeOf: value.Type(), ptr: value.Pointer()}
		if cloned, ok := visited[visit]; ok {
			return cloned
		}
		switch value.Kind() {
		case reflect.Pointer:
			result := reflect.New(value.Type().Elem())
			visited[visit] = result
			result.Elem().Set(cloneSchemaValue(value.Elem(), visited))
			return result
		case reflect.Map:
			result := reflect.MakeMapWithSize(value.Type(), value.Len())
			visited[visit] = result
			iter := value.MapRange()
			for iter.Next() {
				result.SetMapIndex(iter.Key(), cloneSchemaValue(iter.Value(), visited))
			}
			return result
		default:
			result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
			visited[visit] = result
			for i := range value.Len() {
				result.Index(i).Set(cloneSchemaValue(value.Index(i), visited))
			}
			return result
		}
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for i := range value.Len() {
			result.Index(i).Set(cloneSchemaValue(value.Index(i), visited))
		}
		return result
	case reflect.Struct:
		if !value.CanInterface() {
			return value
		}
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		for i := range value.NumField() {
			if value.Type().Field(i).IsExported() {
				result.Field(i).Set(cloneSchemaValue(value.Field(i), visited))
			}
		}
		return result
	default:
		return value
	}
}

func schemaChildren(schema *jsonschema.Schema) []*jsonschema.Schema {
	children := []*jsonschema.Schema{
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
	}
	children = append(children, schema.PrefixItems...)
	children = append(children, schema.ItemsArray...)
	children = append(children, schema.AllOf...)
	children = append(children, schema.AnyOf...)
	children = append(children, schema.OneOf...)
	for _, schemas := range []map[string]*jsonschema.Schema{
		schema.Defs,
		schema.Definitions,
		schema.Properties,
		schema.PatternProperties,
		schema.DependentSchemas,
		schema.DependencySchemas,
	} {
		for _, child := range schemas {
			children = append(children, child)
		}
	}
	return children
}

func removeSchemaDefaults(schema *jsonschema.Schema) {
	if schema == nil {
		return
	}
	schema.Default = nil
	for _, child := range schemaChildren(schema) {
		removeSchemaDefaults(child)
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
	clonedSchema := CloneSchema(schema)
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
	// Match SDK input inference without removing nullable output semantics.
	if inputSchema && goType.Kind() == reflect.Pointer {
		goType = goType.Elem()
	}
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
