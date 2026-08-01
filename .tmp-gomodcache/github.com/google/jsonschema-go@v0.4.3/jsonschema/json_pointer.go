// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file implements JSON Pointers.
// A JSON Pointer is a path that refers to one JSON value within another.
// If the path is empty, it refers to the root value.
// Otherwise, it is a sequence of slash-prefixed strings, like "/points/1/x",
// selecting successive properties (for JSON objects) or items (for JSON arrays).
// For example, when applied to this JSON value:
//    {
// 	     "points": [
// 	 	    {"x": 1, "y": 2},
// 	 	    {"x": 3, "y": 4}
// 		 ]
//    }
//
// the JSON Pointer "/points/1/x" refers to the number 3.
// See the spec at https://datatracker.ietf.org/doc/html/rfc6901.

package jsonschema

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	jsonPointerEscaper   = strings.NewReplacer("~", "~0", "/", "~1")
	jsonPointerUnescaper = strings.NewReplacer("~0", "~", "~1", "/")
)

func escapeJSONPointerSegment(s string) string {
	return jsonPointerEscaper.Replace(s)
}

func unescapeJSONPointerSegment(s string) string {
	return jsonPointerUnescaper.Replace(s)
}

// parseJSONPointer splits a JSON Pointer into a sequence of segments. It doesn't
// convert strings to numbers, because that depends on the traversal: a segment
// is treated as a number when applied to an array, but a string when applied to
// an object. See section 4 of the spec.
func parseJSONPointer(ptr string) (segments []string, err error) {
	if ptr == "" {
		return nil, nil
	}
	if ptr[0] != '/' {
		return nil, fmt.Errorf("JSON Pointer %q does not begin with '/'", ptr)
	}
	// Unlike file paths, consecutive slashes are not coalesced.
	// Split is nicer than Cut here, because it gets a final "/" right.
	segments = strings.Split(ptr[1:], "/")
	if strings.Contains(ptr, "~") {
		// Undo the simple escaping rules that allow one to include a slash in a segment.
		for i := range segments {
			segments[i] = unescapeJSONPointerSegment(segments[i])
		}
	}
	return segments, nil
}

// dereferenceJSONPointer returns the Schema that sptr points to within s,
// or an error if none.
// This implementation suffices for JSON Schema: pointers are applied only to Schemas,
// and refer only to Schemas.
func dereferenceJSONPointer(s *Schema, sptr string) (_ *Schema, err error) {
	defer wrapf(&err, "JSON Pointer %q", sptr)

	segments, err := parseJSONPointer(sptr)
	if err != nil {
		return nil, err
	}
	v := reflect.ValueOf(s)
	for _, seg := range segments {
		switch v.Kind() {
		case reflect.Pointer:
			v = v.Elem()
			if !v.IsValid() {
				return nil, errors.New("navigated to nil reference")
			}
			fallthrough // if valid, can only be a pointer to a Schema

		case reflect.Struct:
			// The segment must refer to a field in a Schema.
			if v.Type() != reflect.TypeFor[Schema]() {
				return nil, fmt.Errorf("navigated to non-Schema %s", v.Type())
			}
			v = lookupSchemaField(v, seg)
			if !v.IsValid() {
				return nil, fmt.Errorf("no schema field %q", seg)
			}
		case reflect.Slice, reflect.Array:
			// The segment must be an integer without leading zeroes that refers to an item in the
			// slice or array.
			if seg == "-" {
				return nil, errors.New("the JSON Pointer array segment '-' is not supported")
			}
			if len(seg) > 1 && seg[0] == '0' {
				return nil, fmt.Errorf("segment %q has leading zeroes", seg)
			}
			n, err := strconv.Atoi(seg)
			if err != nil {
				return nil, fmt.Errorf("invalid int: %q", seg)
			}
			if n < 0 || n >= v.Len() {
				return nil, fmt.Errorf("index %d is out of bounds for array of length %d", n, v.Len())
			}
			v = v.Index(n)
			// Cannot be invalid.
		case reflect.Map:
			// The segment must be a key in the map.
			v = v.MapIndex(reflect.ValueOf(seg))
			if !v.IsValid() {
				return nil, fmt.Errorf("no key %q in map", seg)
			}
		default:
			return nil, fmt.Errorf("value %s (%s) is not a schema, slice or map", v, v.Type())
		}
	}
	if s, ok := v.Interface().(*Schema); ok {
		return s, nil
	}
	return nil, fmt.Errorf("does not refer to a schema, but to a %s", v.Type())
}

// lookupSchemaField returns the value of the field with the given name in v,
// or the zero value if there is no such field or it is not of type Schema or *Schema.
func lookupSchemaField(v reflect.Value, name string) reflect.Value {
	if name == "type" {
		// The "type" keyword may refer to Type or Types.
		// At most one will be non-zero.
		if t := v.FieldByName("Type"); !t.IsZero() {
			return t
		}
		return v.FieldByName("Types")
	}
	if name == "items" {
		// The "items" keyword refers to the "union type" that is either a schema or a schema array.
		// Implemented using the Items representing the schema and ItemsArray for the schema array.
		if items := v.FieldByName("Items"); items.IsValid() && !items.IsNil() {
			return items
		}
		return v.FieldByName("ItemsArray")
	}
	if name == "dependencies" {
		// The "dependencies" keyword refers to both DependencyStrings and DependencySchemas maps.
		// The value on schemaFieldMap is not garanteed to be DependencySchemas which we want
		// for pointer dereference. So we use FieldByName to get the DependencySchemas map.
		return v.FieldByName("DependencySchemas")
	}
	if sf, ok := schemaFieldMap[name]; ok {
		return v.FieldByIndex(sf.Index)
	}
	return reflect.Value{}
}
