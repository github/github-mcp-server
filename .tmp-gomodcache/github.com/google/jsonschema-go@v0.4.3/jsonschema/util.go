// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/maphash"
	"math"
	"math/big"
	"reflect"
	"slices"
	"strings"
	"sync"
)

// Equal reports whether two Go values representing JSON values are equal according
// to the JSON Schema spec.
// The values must not contain cycles.
// See https://json-schema.org/draft/2020-12/json-schema-core#section-4.2.2.
// It behaves like reflect.DeepEqual, except that numbers are compared according
// to mathematical equality.
func Equal(x, y any) bool {
	return equalValue(reflect.ValueOf(x), reflect.ValueOf(y))
}

func equalValue(x, y reflect.Value) bool {
	// Copied from src/reflect/deepequal.go, omitting the visited check (because JSON
	// values are trees).
	if !x.IsValid() || !y.IsValid() {
		return x.IsValid() == y.IsValid()
	}

	// Treat numbers specially.
	rx, ok1 := jsonNumber(x)
	ry, ok2 := jsonNumber(y)
	if ok1 && ok2 {
		return rx.Cmp(ry) == 0
	}
	if x.Kind() != y.Kind() {
		return false
	}
	switch x.Kind() {
	case reflect.Array:
		if x.Len() != y.Len() {
			return false
		}
		for i := range x.Len() {
			if !equalValue(x.Index(i), y.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if x.IsNil() != y.IsNil() {
			return false
		}
		if x.Len() != y.Len() {
			return false
		}
		if x.UnsafePointer() == y.UnsafePointer() {
			return true
		}
		// Special case for []byte, which is common.
		if x.Type().Elem().Kind() == reflect.Uint8 && x.Type() == y.Type() {
			return bytes.Equal(x.Bytes(), y.Bytes())
		}
		for i := range x.Len() {
			if !equalValue(x.Index(i), y.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Interface:
		if x.IsNil() || y.IsNil() {
			return x.IsNil() == y.IsNil()
		}
		return equalValue(x.Elem(), y.Elem())
	case reflect.Pointer:
		if x.UnsafePointer() == y.UnsafePointer() {
			return true
		}
		return equalValue(x.Elem(), y.Elem())
	case reflect.Struct:
		t := x.Type()
		if t != y.Type() {
			return false
		}
		for i := range t.NumField() {
			sf := t.Field(i)
			if !sf.IsExported() {
				continue
			}
			if !equalValue(x.FieldByIndex(sf.Index), y.FieldByIndex(sf.Index)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if x.IsNil() != y.IsNil() {
			return false
		}
		if x.Len() != y.Len() {
			return false
		}
		if x.UnsafePointer() == y.UnsafePointer() {
			return true
		}
		iter := x.MapRange()
		for iter.Next() {
			vx := iter.Value()
			vy := y.MapIndex(iter.Key())
			if !vy.IsValid() || !equalValue(vx, vy) {
				return false
			}
		}
		return true
	case reflect.Func:
		if x.Type() != y.Type() {
			return false
		}
		if x.IsNil() && y.IsNil() {
			return true
		}
		panic("cannot compare functions")
	case reflect.String:
		return x.String() == y.String()
	case reflect.Bool:
		return x.Bool() == y.Bool()
	// Ints, uints and floats handled in jsonNumber, at top of function.
	default:
		panic(fmt.Sprintf("unsupported kind: %s", x.Kind()))
	}
}

// hashValue adds v to the data hashed by h. v must not have cycles.
// hashValue panics if the value contains functions or channels, or maps whose
// key type is not string.
// It ignores unexported fields of structs.
// Calls to hashValue with the equal values (in the sense
// of [Equal]) result in the same sequence of values written to the hash.
func hashValue(h *maphash.Hash, v reflect.Value) {
	// TODO: replace writes of basic types with WriteComparable in 1.24.

	writeUint := func(u uint64) {
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], u)
		h.Write(buf[:])
	}

	var write func(reflect.Value)
	write = func(v reflect.Value) {
		if r, ok := jsonNumber(v); ok {
			// We want 1.0 and 1 to hash the same.
			// big.Rats are always normalized, so they will be.
			// We could do this more efficiently by handling the int and float cases
			// separately, but that's premature.
			writeUint(uint64(r.Sign() + 1))
			h.Write(r.Num().Bytes())
			h.Write(r.Denom().Bytes())
			return
		}
		switch v.Kind() {
		case reflect.Invalid:
			h.WriteByte(0)
		case reflect.String:
			h.WriteString(v.String())
		case reflect.Bool:
			if v.Bool() {
				h.WriteByte(1)
			} else {
				h.WriteByte(0)
			}
		case reflect.Complex64, reflect.Complex128:
			c := v.Complex()
			writeUint(math.Float64bits(real(c)))
			writeUint(math.Float64bits(imag(c)))
		case reflect.Array, reflect.Slice:
			// Although we could treat []byte more efficiently,
			// JSON values are unlikely to contain them.
			writeUint(uint64(v.Len()))
			for i := range v.Len() {
				write(v.Index(i))
			}
		case reflect.Interface, reflect.Pointer:
			write(v.Elem())
		case reflect.Struct:
			t := v.Type()
			for i := range t.NumField() {
				if sf := t.Field(i); sf.IsExported() {
					write(v.FieldByIndex(sf.Index))
				}
			}
		case reflect.Map:
			if v.Type().Key().Kind() != reflect.String {
				panic("map with non-string key")
			}
			// Sort the keys so the hash is deterministic.
			keys := v.MapKeys()
			// Write the length. That distinguishes between, say, two consecutive
			// maps with disjoint keys from one map that has the items of both.
			writeUint(uint64(len(keys)))
			slices.SortFunc(keys, func(x, y reflect.Value) int { return cmp.Compare(x.String(), y.String()) })
			for _, k := range keys {
				write(k)
				write(v.MapIndex(k))
			}
		// Ints, uints and floats handled in jsonNumber, at top of function.
		default:
			panic(fmt.Sprintf("unsupported kind: %s", v.Kind()))
		}
	}

	write(v)
}

// jsonNumber converts a numeric value or a json.Number to a [big.Rat].
// If v is not a number, it returns nil, false.
func jsonNumber(v reflect.Value) (*big.Rat, bool) {
	r := new(big.Rat)
	switch {
	case !v.IsValid():
		return nil, false
	case v.CanInt():
		r.SetInt64(v.Int())
	case v.CanUint():
		r.SetUint64(v.Uint())
	case v.CanFloat():
		r.SetFloat64(v.Float())
	default:
		jn, ok := v.Interface().(json.Number)
		if !ok {
			return nil, false
		}
		if _, ok := r.SetString(jn.String()); !ok {
			// This can fail in rare cases; for example, "1e9999999".
			// That is a valid JSON number, since the spec puts no limit on the size
			// of the exponent.
			return nil, false
		}
	}
	return r, true
}

// jsonType returns a string describing the type of the JSON value,
// as described in the JSON Schema specification:
// https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.1.1.
// It returns "", false if the value is not valid JSON.
func jsonType(v reflect.Value) (string, bool) {
	if !v.IsValid() {
		// Not v.IsNil(): a nil []any is still a JSON array.
		return "null", true
	}
	if v.CanInt() || v.CanUint() {
		return "integer", true
	}
	if v.CanFloat() {
		if _, f := math.Modf(v.Float()); f == 0 {
			return "integer", true
		}
		return "number", true
	}
	switch v.Kind() {
	case reflect.Bool:
		return "boolean", true
	case reflect.String:
		return "string", true
	case reflect.Slice, reflect.Array:
		return "array", true
	case reflect.Map, reflect.Struct:
		return "object", true
	default:
		return "", false
	}
}

func assert(cond bool, msg string) {
	if !cond {
		panic("assertion failed: " + msg)
	}
}

// marshalStructWithMap marshals its first argument to JSON, treating the field named
// mapField as an embedded map. The first argument must be a pointer to
// a struct. The underlying type of mapField must be a map[string]any, and it must have
// a "-" json tag, meaning it will not be marshaled.
//
// For example, given this struct:
//
//	type S struct {
//	   A int
//	   Extra map[string] any `json:"-"`
//	}
//
// and this value:
//
//	s := S{A: 1, Extra: map[string]any{"B": 2}}
//
// the call marshalJSONWithMap(s, "Extra") would return
//
//	{"A": 1, "B": 2}
//
// It is an error if the map contains the same key as another struct field's
// JSON name.
//
// marshalStructWithMap calls json.Marshal on a value of type T, so T must not
// have a MarshalJSON method that calls this function, on pain of infinite regress.
//
// Note that there is a similar function in mcp/util.go, but they are not the same.
// Here the function requires `-` json tag, does not clear the mapField map,
// and handles embedded struct due to the implementation of jsonNames in this package.
//
// TODO: avoid this restriction on T by forcing it to marshal in a default way.
// See https://go.dev/play/p/EgXKJHxEx_R.
func marshalStructWithMap[T any](s *T, mapField string) ([]byte, error) {
	// Marshal the struct and the map separately, and concatenate the bytes.
	// This strategy is dramatically less complicated than
	// constructing a synthetic struct or map with the combined keys.
	if s == nil {
		return []byte("null"), nil
	}
	s2 := *s
	vMapField := reflect.ValueOf(&s2).Elem().FieldByName(mapField)
	mapVal := vMapField.Interface().(map[string]any)

	// Check for duplicates.
	names := jsonNames(reflect.TypeFor[T]())
	for key := range mapVal {
		if names[key] {
			return nil, fmt.Errorf("map key %q duplicates struct field", key)
		}
	}

	structBytes, err := json.Marshal(s2)
	if err != nil {
		return nil, fmt.Errorf("marshalStructWithMap(%+v): %w", s, err)
	}
	if len(mapVal) == 0 {
		return structBytes, nil
	}
	mapBytes, err := json.Marshal(mapVal)
	if err != nil {
		return nil, err
	}
	if len(structBytes) == 2 { // must be "{}"
		return mapBytes, nil
	}
	// "{X}" + "{Y}" => "{X,Y}"
	res := append(structBytes[:len(structBytes)-1], ',')
	res = append(res, mapBytes[1:]...)
	return res, nil
}

// unmarshalStructWithMap is the inverse of marshalStructWithMap.
// T has the same restrictions as in that function.
//
// Note that there is a similar function in mcp/util.go, but they are not the same.
// Here jsonNames also returns fields from embedded structs, hence this function
// handles embedded structs as well.
func unmarshalStructWithMap[T any](data []byte, v *T, mapField string) error {
	// Unmarshal into the struct, ignoring unknown fields.
	if err := json.Unmarshal(data, v); err != nil {
		return err
	}
	// Unmarshal into the map.
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	// Delete from the map the fields of the struct.
	for n := range jsonNames(reflect.TypeFor[T]()) {
		delete(m, n)
	}
	if len(m) != 0 {
		reflect.ValueOf(v).Elem().FieldByName(mapField).Set(reflect.ValueOf(m))
	}
	return nil
}

var jsonNamesMap sync.Map // from reflect.Type to map[string]bool

// jsonNames returns the set of JSON object keys that t will marshal into,
// including fields from embedded structs in t.
// t must be a struct type.
//
// Note that there is a similar function in mcp/util.go, but they are not the same
// Here the function recurses over embedded structs and includes fields from them.
func jsonNames(t reflect.Type) map[string]bool {
	// Lock not necessary: at worst we'll duplicate work.
	if val, ok := jsonNamesMap.Load(t); ok {
		return val.(map[string]bool)
	}
	m := map[string]bool{}
	for i := range t.NumField() {
		field := t.Field(i)
		// handle embedded structs
		if field.Anonymous {
			fieldType := field.Type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}
			for n := range jsonNames(fieldType) {
				m[n] = true
			}
			continue
		}
		info := fieldJSONInfo(field)
		if !info.omit {
			m[info.name] = true
		}
	}
	jsonNamesMap.Store(t, m)
	return m
}

type jsonInfo struct {
	omit     bool            // unexported or first tag element is "-"
	name     string          // Go field name or first tag element. Empty if omit is true.
	settings map[string]bool // "omitempty", "omitzero", etc.
}

// fieldJSONInfo reports information about how encoding/json
// handles the given struct field.
// If the field is unexported, jsonInfo.omit is true and no other jsonInfo field
// is populated.
// If the field is exported and has no tag, then name is the field's name and all
// other fields are false.
// Otherwise, the information is obtained from the tag.
func fieldJSONInfo(f reflect.StructField) jsonInfo {
	if !f.IsExported() {
		return jsonInfo{omit: true}
	}
	info := jsonInfo{name: f.Name}
	if tag, ok := f.Tag.Lookup("json"); ok {
		name, rest, found := strings.Cut(tag, ",")
		// "-" means omit, but "-," means the name is "-"
		if name == "-" && !found {
			return jsonInfo{omit: true}
		}
		if name != "" {
			info.name = name
		}
		if len(rest) > 0 {
			info.settings = map[string]bool{}
			for _, s := range strings.Split(rest, ",") {
				info.settings[s] = true
			}
		}
	}
	return info
}

// wrapf wraps *errp with the given formatted message if *errp is not nil.
func wrapf(errp *error, format string, args ...any) {
	if *errp != nil {
		*errp = fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), *errp)
	}
}
