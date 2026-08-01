// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file contains functions that infer a schema from a Go type.

package jsonschema

import (
	"encoding"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"math/big"
	"os"
	"reflect"
	"regexp"
	"slices"
	"time"
)

const debugEnv = "JSONSCHEMAGODEBUG"

// ForOptions are options for the [For] and [ForType] functions.
type ForOptions struct {
	// If IgnoreInvalidTypes is true, fields that can't be represented as a JSON
	// Schema are ignored instead of causing an error.
	// This allows callers to adjust the resulting schema using custom knowledge.
	// For example, an interface type where all the possible implementations are
	// known can be described with "oneof".
	IgnoreInvalidTypes bool

	// TypeSchemas maps types to their schemas.
	// If [For] encounters a type that is a key in this map, the
	// corresponding value is used as the resulting schema (after cloning to
	// ensure uniqueness).
	// Types in this map override the default translations, as described
	// in [For]'s documentation.
	// PropertyOrder defined in these schemas will not be used in [For] or [ForType].
	TypeSchemas map[reflect.Type]*Schema
}

// For constructs a JSON schema object for the given type argument.
// If non-nil, the provided options configure certain aspects of this contruction,
// described below.

// It translates Go types into compatible JSON schema types, as follows.
// These defaults can be overridden by [ForOptions.TypeSchemas].
//
//   - Strings have schema type "string".
//   - Bools have schema type "boolean".
//   - Signed and unsigned integer types have schema type "integer".
//   - Floating point types have schema type "number".
//   - Slices and arrays have schema type "array", and a corresponding schema
//     for items.
//   - Maps with string key have schema type "object", and corresponding
//     schema for additionalProperties.
//   - Structs have schema type "object", and disallow additionalProperties.
//     Their properties are derived from exported struct fields, using the
//     struct field JSON name. Fields that are marked "omitempty" or "omitzero" are
//     considered optional; all other fields become required properties.
//     For structs, the PropertyOrder will be set to the field order.
//   - Some types in the standard library that implement json.Marshaler
//     translate to schemas that match the values to which they marshal.
//     For example, [time.Time] translates to the schema for strings.
//
// For will return an error if there is a cycle in the types.
//
// By default, For returns an error if t contains (possibly recursively) any of the
// following Go types, as they are incompatible with the JSON schema spec.
// If [ForOptions.IgnoreInvalidTypes] is true, then these types are ignored instead.
//   - maps with key other than 'string'
//   - function types
//   - channel types
//   - complex numbers
//   - unsafe pointers
//
// This function recognizes struct field tags named "jsonschema".
// A jsonschema tag on a field is used as the description for the corresponding property.
// For future compatibility, descriptions must not start with "WORD=", where WORD is a
// sequence of non-whitespace characters.
func For[T any](opts *ForOptions) (*Schema, error) {
	if opts == nil {
		opts = &ForOptions{}
	}
	schemas := maps.Clone(initialSchemaMap)
	// Add types from the options. They override the default ones.
	maps.Copy(schemas, opts.TypeSchemas)
	s, err := forType(reflect.TypeFor[T](), map[reflect.Type]bool{}, opts.IgnoreInvalidTypes, schemas)
	if err != nil {
		var z T
		return nil, fmt.Errorf("For[%T](): %w", z, err)
	}
	return s, nil
}

// ForType is like [For], but takes a [reflect.Type]
func ForType(t reflect.Type, opts *ForOptions) (*Schema, error) {
	if opts == nil {
		opts = &ForOptions{}
	}
	schemas := maps.Clone(initialSchemaMap)
	// Add types from the options. They override the default ones.
	maps.Copy(schemas, opts.TypeSchemas)
	s, err := forType(t, map[reflect.Type]bool{}, opts.IgnoreInvalidTypes, schemas)
	if err != nil {
		return nil, fmt.Errorf("ForType(%s): %w", t, err)
	}
	return s, nil
}

// Helper to create a *float64 pointer from a value
func f64Ptr(f float64) *float64 {
	return &f
}

func forType(t reflect.Type, seen map[reflect.Type]bool, ignore bool, schemas map[reflect.Type]*Schema) (*Schema, error) {
	// Follow pointers: the schema for *T is almost the same as for T, except that
	// an explicit JSON "null" is allowed for the pointer.
	allowNull := false
	for t.Kind() == reflect.Pointer {
		allowNull = true
		t = t.Elem()
	}

	// Check for cycles
	// User defined types have a name, so we can skip those that are natively defined
	if t.Name() != "" {
		if seen[t] {
			return nil, fmt.Errorf("cycle detected for type %v", t)
		}
		seen[t] = true
		defer delete(seen, t)
	}

	if s := schemas[t]; s != nil {
		cloned := s.CloneSchemas()
		if os.Getenv(debugEnv) != "typeschemasnull=1" && allowNull {
			if cloned.Type != "" {
				cloned.Types = []string{"null", cloned.Type}
				cloned.Type = ""
			} else if !slices.Contains(cloned.Types, "null") {
				cloned.Types = append([]string{"null"}, cloned.Types...)
			}
		}
		return cloned, nil
	}

	var (
		s   = new(Schema)
		err error
	)

	switch t.Kind() {
	case reflect.Bool:
		s.Type = "boolean"

	case reflect.Int, reflect.Int64:
		s.Type = "integer"

	case reflect.Uint, reflect.Uint64, reflect.Uintptr:
		s.Type = "integer"
		s.Minimum = f64Ptr(0)

	case reflect.Int8:
		s.Type = "integer"
		s.Minimum = f64Ptr(math.MinInt8)
		s.Maximum = f64Ptr(math.MaxInt8)

	case reflect.Uint8:
		s.Type = "integer"
		s.Minimum = f64Ptr(0)
		s.Maximum = f64Ptr(math.MaxUint8)

	case reflect.Int16:
		s.Type = "integer"
		s.Minimum = f64Ptr(math.MinInt16)
		s.Maximum = f64Ptr(math.MaxInt16)

	case reflect.Uint16:
		s.Type = "integer"
		s.Minimum = f64Ptr(0)
		s.Maximum = f64Ptr(math.MaxUint16)

	case reflect.Int32:
		s.Type = "integer"
		s.Minimum = f64Ptr(math.MinInt32)
		s.Maximum = f64Ptr(math.MaxInt32)

	case reflect.Uint32:
		s.Type = "integer"
		s.Minimum = f64Ptr(0)
		s.Maximum = f64Ptr(math.MaxUint32)

	case reflect.Float32, reflect.Float64:
		s.Type = "number"

	case reflect.Interface:
		// Unrestricted

	case reflect.Map:
		if t.Key().Kind() != reflect.String && !t.Key().Implements(reflect.TypeFor[encoding.TextMarshaler]()) {
			if ignore {
				return nil, nil // ignore
			}
			return nil, fmt.Errorf("unsupported map key type %v", t.Key().Kind())
		}
		s.Type = "object"
		s.AdditionalProperties, err = forType(t.Elem(), seen, ignore, schemas)
		if err != nil {
			return nil, fmt.Errorf("computing map value schema: %v", err)
		}
		if ignore && s.AdditionalProperties == nil {
			// Ignore if the element type is invalid.
			return nil, nil
		}

	case reflect.Slice, reflect.Array:
		if os.Getenv(debugEnv) != "typeschemasnull=1" && t.Kind() == reflect.Slice {
			s.Types = []string{"null", "array"}
		} else {
			s.Type = "array"
		}
		itemsSchema, err := forType(t.Elem(), seen, ignore, schemas)
		if err != nil {
			return nil, fmt.Errorf("computing element schema: %v", err)
		}
		if itemsSchema == nil {
			return nil, nil
		}
		s.Items = itemsSchema
		if ignore && s.Items == nil {
			// Ignore if the element type is invalid.
			return nil, nil
		}
		if t.Kind() == reflect.Array {
			s.MinItems = Ptr(t.Len())
			s.MaxItems = Ptr(t.Len())
		}

	case reflect.String:
		s.Type = "string"

	case reflect.Struct:
		s.Type = "object"
		// no additional properties are allowed
		s.AdditionalProperties = falseSchema()

		// If skipPath is non-nil, it is path to an anonymous field whose
		// schema has been replaced by a known schema.
		var skipPath []int
		for _, field := range reflect.VisibleFields(t) {
			if s.Properties == nil {
				s.Properties = make(map[string]*Schema)
			}
			if field.Anonymous {
				override := schemas[field.Type]
				if override != nil {
					// Type must be object, and only properties can be set.
					if override.Type != "object" {
						return nil, fmt.Errorf(`custom schema for embedded struct must have type "object", got %q`,
							override.Type)
					}
					// Check that all keywords relevant for objects are absent, except properties.
					ov := reflect.ValueOf(override).Elem()
					for _, sfi := range schemaFieldInfos {
						if sfi.sf.Name == "Type" || sfi.sf.Name == "Properties" {
							continue
						}
						fv := ov.FieldByIndex(sfi.sf.Index)
						if !fv.IsZero() {
							return nil, fmt.Errorf(`overrides for embedded fields can have only "Type" and "Properties"; this has %q`, sfi.sf.Name)
						}
					}

					skipPath = field.Index
					keys := make([]string, 0, len(override.Properties))
					for k := range override.Properties {
						keys = append(keys, k)
					}
					slices.Sort(keys)
					for _, name := range keys {
						if _, ok := s.Properties[name]; !ok {
							s.Properties[name] = override.Properties[name].CloneSchemas()
							s.PropertyOrder = append(s.PropertyOrder, name)
						}
					}
				}
				continue
			}

			// Check to see if this field has been promoted from a replaced anonymous
			// type.
			if skipPath != nil {
				skip := false
				if len(field.Index) >= len(skipPath) {
					skip = true
					for i, index := range skipPath {
						if field.Index[i] != index {
							// If we're no longer in a subfield.
							skip = false
							break
						}
					}
				}
				if skip {
					continue
				} else {
					// Anonymous fields are followed immediately by their promoted fields.
					// Once we encounter a field that *isn't* promoted, we can stop
					// checking.
					skipPath = nil
				}
			}

			info := fieldJSONInfo(field)
			if info.omit {
				continue
			}
			fs, err := forType(field.Type, seen, ignore, schemas)
			if err != nil {
				return nil, err
			}
			if ignore && fs == nil {
				// Skip fields of invalid type.
				continue
			}
			if tag, ok := field.Tag.Lookup("jsonschema"); ok {
				if tag == "" {
					return nil, fmt.Errorf("empty jsonschema tag on struct field %s.%s", t, field.Name)
				}
				if disallowedPrefixRegexp.MatchString(tag) {
					return nil, fmt.Errorf("tag must not begin with 'WORD=': %q", tag)
				}
				fs.Description = tag
			}
			s.Properties[info.name] = fs

			s.PropertyOrder = append(s.PropertyOrder, info.name)

			if !info.settings["omitempty"] && !info.settings["omitzero"] {
				s.Required = append(s.Required, info.name)
			}
		}

		// Remove PropertyOrder duplicates, keeping the last occurrence
		if len(s.PropertyOrder) > 1 {
			seen := make(map[string]bool)
			// Create a slice to hold the cleaned order (capacity = current length)
			cleaned := make([]string, 0, len(s.PropertyOrder))

			// Iterate backwards
			for i := len(s.PropertyOrder) - 1; i >= 0; i-- {
				name := s.PropertyOrder[i]
				if !seen[name] {
					cleaned = append(cleaned, name)
					seen[name] = true
				}
			}

			// Since we collected them backwards, we need to reverse the result
			// to restore the correct order.
			slices.Reverse(cleaned)
			s.PropertyOrder = cleaned
		}

	default:
		if ignore {
			// Ignore.
			return nil, nil
		}
		return nil, fmt.Errorf("type %v is unsupported by jsonschema", t)
	}
	if allowNull && s.Type != "" {
		s.Types = []string{"null", s.Type}
		s.Type = ""
	}
	return s, nil
}

// initialSchemaMap holds types from the standard library that have MarshalJSON methods.
var initialSchemaMap = make(map[reflect.Type]*Schema)

func init() {
	ss := &Schema{Type: "string"}
	initialSchemaMap[reflect.TypeFor[time.Time]()] = ss
	initialSchemaMap[reflect.TypeFor[slog.Level]()] = ss
	if os.Getenv(debugEnv) == "typeschemasnull=1" {
		initialSchemaMap[reflect.TypeFor[big.Int]()] = &Schema{Types: []string{"null", "string"}}
	} else {
		initialSchemaMap[reflect.TypeFor[big.Int]()] = ss
	}
	initialSchemaMap[reflect.TypeFor[big.Rat]()] = ss
	initialSchemaMap[reflect.TypeFor[big.Float]()] = ss
}

// Disallow jsonschema tag values beginning "WORD=", for future expansion.
var disallowedPrefixRegexp = regexp.MustCompile("^[^ \t\n]*=")
