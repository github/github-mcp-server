// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"maps"
	"math"
	"reflect"
	"slices"
)

// A Schema is a JSON schema object.
// It supports both draft-07 and the 2020-12 draft specifications:
//   - Draft-07: https://json-schema.org/draft-07/draft-handrews-json-schema-01
//     and https://json-schema.org/draft-07/draft-handrews-json-schema-validation-01
//   - Draft 2020-12: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-01
//     and https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01
//
// A Schema value may have non-zero values for more than one field:
// all relevant non-zero fields are used for validation.
// There is one exception to provide more Go type-safety: the Type and Types fields
// are mutually exclusive.
//
// Since this struct is a Go representation of a JSON value, it inherits JSON's
// distinction between nil and empty. Nil slices and maps are considered absent,
// but empty ones are present and affect validation. For example,
//
//	Schema{Enum: nil}
//
// is equivalent to an empty schema, so it validates every instance. But
//
//	Schema{Enum: []any{}}
//
// requires equality to some slice element, so it vacuously rejects every instance.
type Schema struct {
	// core
	ID          string             `json:"$id,omitempty"`
	Schema      string             `json:"$schema,omitempty"`
	Ref         string             `json:"$ref,omitempty"`
	Comment     string             `json:"$comment,omitempty"`
	Defs        map[string]*Schema `json:"$defs,omitempty"`
	Definitions map[string]*Schema `json:"definitions,omitempty"`

	// split draft 7 Dependencies into DependencySchemas and DependencyStrings
	DependencySchemas map[string]*Schema  `json:"-"`
	DependencyStrings map[string][]string `json:"-"`

	Anchor        string          `json:"$anchor,omitempty"`
	DynamicAnchor string          `json:"$dynamicAnchor,omitempty"`
	DynamicRef    string          `json:"$dynamicRef,omitempty"`
	Vocabulary    map[string]bool `json:"$vocabulary,omitempty"`

	// metadata
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Default     json.RawMessage `json:"default,omitempty"`
	Deprecated  bool            `json:"deprecated,omitempty"`
	ReadOnly    bool            `json:"readOnly,omitempty"`
	WriteOnly   bool            `json:"writeOnly,omitempty"`
	Examples    []any           `json:"examples,omitempty"`

	// validation
	// Use Type for a single type, or Types for multiple types; never both.
	Type  string   `json:"-"`
	Types []string `json:"-"`
	Enum  []any    `json:"enum,omitempty"`
	// Const is *any because a JSON null (Go nil) is a valid value.
	Const            *any     `json:"const,omitempty"`
	MultipleOf       *float64 `json:"multipleOf,omitempty"`
	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	MinLength        *int     `json:"minLength,omitempty"`
	MaxLength        *int     `json:"maxLength,omitempty"`
	Pattern          string   `json:"pattern,omitempty"`

	// arrays
	PrefixItems      []*Schema `json:"prefixItems,omitempty"`
	Items            *Schema   `json:"-"`
	ItemsArray       []*Schema `json:"-"`
	MinItems         *int      `json:"minItems,omitempty"`
	MaxItems         *int      `json:"maxItems,omitempty"`
	AdditionalItems  *Schema   `json:"additionalItems,omitempty"`
	UniqueItems      bool      `json:"uniqueItems,omitempty"`
	Contains         *Schema   `json:"contains,omitempty"`
	MinContains      *int      `json:"minContains,omitempty"` // *int, not int: default is 1, not 0
	MaxContains      *int      `json:"maxContains,omitempty"`
	UnevaluatedItems *Schema   `json:"unevaluatedItems,omitempty"`

	// objects
	MinProperties         *int                `json:"minProperties,omitempty"`
	MaxProperties         *int                `json:"maxProperties,omitempty"`
	Required              []string            `json:"required,omitempty"`
	DependentRequired     map[string][]string `json:"dependentRequired,omitempty"`
	Properties            map[string]*Schema  `json:"properties,omitempty"`
	PatternProperties     map[string]*Schema  `json:"patternProperties,omitempty"`
	AdditionalProperties  *Schema             `json:"additionalProperties,omitempty"`
	PropertyNames         *Schema             `json:"propertyNames,omitempty"`
	UnevaluatedProperties *Schema             `json:"unevaluatedProperties,omitempty"`

	// logic
	AllOf []*Schema `json:"allOf,omitempty"`
	AnyOf []*Schema `json:"anyOf,omitempty"`
	OneOf []*Schema `json:"oneOf,omitempty"`
	Not   *Schema   `json:"not,omitempty"`

	// conditional
	If               *Schema            `json:"if,omitempty"`
	Then             *Schema            `json:"then,omitempty"`
	Else             *Schema            `json:"else,omitempty"`
	DependentSchemas map[string]*Schema `json:"dependentSchemas,omitempty"`

	// other
	// https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-00#rfc.section.8
	ContentEncoding  string  `json:"contentEncoding,omitempty"`
	ContentMediaType string  `json:"contentMediaType,omitempty"`
	ContentSchema    *Schema `json:"contentSchema,omitempty"`

	// https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-00#rfc.section.7
	Format string `json:"format,omitempty"`

	// Extra allows for additional keywords beyond those specified.
	Extra map[string]any `json:"-"`

	// PropertyOrder records the ordering of properties for JSON rendering.
	//
	// During [For], PropertyOrder is set to the field order,
	// if the type used for inference is a struct.
	//
	// If PropertyOrder is set, it controls the relative ordering of properties in [Schema.MarshalJSON].
	// The rendered JSON first lists any properties that appear in the PropertyOrder slice in the order
	// they appear, followed by all other properties that do not appear in the PropertyOrder slice in an
	// undefined but deterministic order.
	PropertyOrder []string `json:"-"`
}

// falseSchema returns a new Schema tree that fails to validate any value.
func falseSchema() *Schema {
	return &Schema{Not: &Schema{}}
}

// anchorInfo records the subschema to which an anchor refers, and whether
// the anchor keyword is $anchor or $dynamicAnchor.
type anchorInfo struct {
	schema  *Schema
	dynamic bool
}

// String returns a short description of the schema.
func (s *Schema) String() string {
	if s.ID != "" {
		return s.ID
	}
	if a := cmp.Or(s.Anchor, s.DynamicAnchor); a != "" {
		return fmt.Sprintf("anchor %s", a)
	}
	return "<anonymous schema>"
}

// CloneSchemas returns a copy of s.
// The copy is shallow except for sub-schemas, which are themelves copied with CloneSchemas.
// This allows both s and s.CloneSchemas() to appear as sub-schemas of the same parent.
func (s *Schema) CloneSchemas() *Schema {
	if s == nil {
		return nil
	}
	s2 := *s
	v := reflect.ValueOf(&s2)
	for _, info := range schemaFieldInfos {
		fv := v.Elem().FieldByIndex(info.sf.Index)
		switch info.sf.Type {
		case schemaType:
			sscss := fv.Interface().(*Schema)
			fv.Set(reflect.ValueOf(sscss.CloneSchemas()))

		case schemaSliceType:
			slice := fv.Interface().([]*Schema)
			slice = slices.Clone(slice)
			for i, ss := range slice {
				slice[i] = ss.CloneSchemas()
			}
			fv.Set(reflect.ValueOf(slice))

		case schemaMapType:
			m := fv.Interface().(map[string]*Schema)
			m = maps.Clone(m)
			for k, ss := range m {
				m[k] = ss.CloneSchemas()
			}
			fv.Set(reflect.ValueOf(m))

		}
	}
	return &s2
}

func (s *Schema) basicChecks() error {
	if s.Type != "" && s.Types != nil {
		return errors.New("both Type and Types are set; at most one should be")
	}
	if s.Defs != nil && s.Definitions != nil {
		return errors.New("both Defs and Definitions are set; at most one should be")
	}
	if s.Items != nil && s.ItemsArray != nil {
		return errors.New("both Items and ItemsArray are set; at most one should be")
	}
	propertyOrderSeen := make(map[string]bool)
	for _, val := range s.PropertyOrder {
		if _, ok := propertyOrderSeen[val]; ok {
			// Duplicate found
			return fmt.Errorf("property order slice cannot contain duplicate entries, found duplicate %q", val)
		}
		propertyOrderSeen[val] = true
	}

	for key := range s.DependencySchemas {
		// Check if the key exists in the dependency strings map
		if _, exists := s.DependencyStrings[key]; exists {
			return fmt.Errorf("dependency key %q cannot be defined as both a schema and a string array", key)
		}
	}
	return nil
}

type schemaWithoutMethods Schema // doesn't implement json.{Unm,M}arshaler

func (s Schema) MarshalJSON() ([]byte, error) {
	// NOTE: Use a value receiver here to avoid the encoding/json bugs
	// described in golang/go#22967, golang/go#33993, and golang/go#55890.
	// With a pointer receiver, MarshalJSON is only called for Schema in
	// some cases (for example when the field value is addressable, or not
	// stored as a map value), which leads to inconsistent JSON encoding.
	// A value receiver makes Schema itself implement json.Marshaler and
	// ensures that encoding/json always calls this method.
	if err := s.basicChecks(); err != nil {
		return nil, err
	}
	// Marshal either Type or Types as "type".
	var typ any
	switch {
	case s.Type != "":
		typ = s.Type
	case s.Types != nil:
		typ = s.Types
	}

	var items any
	switch {
	case s.Items != nil:
		items = s.Items
	case s.ItemsArray != nil:
		items = s.ItemsArray
	}

	var dep map[string]any
	size := len(s.DependencySchemas) + len(s.DependencyStrings)
	if size > 0 {
		dep = make(map[string]any, size)
		for k, v := range s.DependencySchemas {
			dep[k] = v
		}
		for k, v := range s.DependencyStrings {
			dep[k] = v
		}
	}

	ms := struct {
		Type         any            `json:"type,omitempty"`
		Properties   json.Marshaler `json:"properties,omitempty"`
		Dependencies map[string]any `json:"dependencies,omitempty"`
		Items        any            `json:"items,omitempty"`
		*schemaWithoutMethods
	}{
		Type:                 typ,
		Dependencies:         dep,
		Items:                items,
		schemaWithoutMethods: (*schemaWithoutMethods)(&s),
	}
	// Marshal properties, even if the empty map (but not nil).
	if s.Properties != nil {
		ms.Properties = orderedProperties{
			props: s.Properties,
			order: s.PropertyOrder,
		}
	}

	bs, err := marshalStructWithMap(&ms, "Extra")
	if err != nil {
		return nil, err
	}
	// Marshal {} as true and {"not": {}} as false.
	// It is wasteful to do this here instead of earlier, but much easier.
	switch {
	case bytes.Equal(bs, []byte(`{}`)):
		bs = []byte("true")
	case bytes.Equal(bs, []byte(`{"not":true}`)):
		bs = []byte("false")
	}
	return bs, nil
}

// orderedProperties is a helper to marshal the properties map in a specific order.
type orderedProperties struct {
	props map[string]*Schema
	order []string
}

func (op orderedProperties) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	first := true
	processed := make(map[string]bool, len(op.props))

	// Helper closure to write "key": value
	writeEntry := func(key string, val *Schema) error {
		if !first {
			buf.WriteByte(',')
		}
		first = false

		// Marshal the Key
		keyBytes, err := json.Marshal(key)
		if err != nil {
			return err
		}
		buf.Write(keyBytes)

		buf.WriteByte(':')

		// Marshal the Value
		valBytes, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(valBytes)
		return nil
	}

	// Write keys explicitly listed in PropertyOrder
	for _, name := range op.order {
		if prop, ok := op.props[name]; ok {
			if err := writeEntry(name, prop); err != nil {
				return nil, err
			}
			processed[name] = true
		}
	}

	// Write any remaining keys
	var remaining []string
	for name := range op.props {
		if !processed[name] {
			remaining = append(remaining, name)
		}
	}

	// Sort the slice alphabetically
	slices.Sort(remaining)

	for _, name := range remaining {
		if err := writeEntry(name, op.props[name]); err != nil {
			return nil, err
		}
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (s *Schema) UnmarshalJSON(data []byte) error {
	// A JSON boolean is a valid schema.
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		if b {
			// true is the empty schema, which validates everything.
			*s = Schema{}
		} else {
			// false is the schema that validates nothing.
			*s = *falseSchema()
		}
		return nil
	}

	ms := struct {
		Type          json.RawMessage            `json:"type,omitempty"`
		Dependencies  map[string]json.RawMessage `json:"dependencies,omitempty"`
		Items         json.RawMessage            `json:"items,omitempty"`
		Const         json.RawMessage            `json:"const,omitempty"`
		MinLength     *integer                   `json:"minLength,omitempty"`
		MaxLength     *integer                   `json:"maxLength,omitempty"`
		MinItems      *integer                   `json:"minItems,omitempty"`
		MaxItems      *integer                   `json:"maxItems,omitempty"`
		MinProperties *integer                   `json:"minProperties,omitempty"`
		MaxProperties *integer                   `json:"maxProperties,omitempty"`
		MinContains   *integer                   `json:"minContains,omitempty"`
		MaxContains   *integer                   `json:"maxContains,omitempty"`

		*schemaWithoutMethods
	}{
		schemaWithoutMethods: (*schemaWithoutMethods)(s),
	}
	if err := unmarshalStructWithMap(data, &ms, "Extra"); err != nil {
		return err
	}
	// Unmarshal "type" as either Type or Types.
	var err error
	if len(ms.Type) > 0 {
		switch ms.Type[0] {
		case '"':
			err = json.Unmarshal(ms.Type, &s.Type)
		case '[':
			err = json.Unmarshal(ms.Type, &s.Types)
		default:
			err = fmt.Errorf(`invalid value for "type": %q`, ms.Type)
		}
	}
	if err != nil {
		return err
	}

	// Unmarshal "items" as either Items or ItemsArray.
	if len(ms.Items) > 0 {
		switch ms.Items[0] {
		case '[':
			var schemas []*Schema
			err = json.Unmarshal(ms.Items, &schemas)
			s.ItemsArray = schemas
		default:
			var schema Schema
			err = json.Unmarshal(ms.Items, &schema)
			s.Items = &schema
		}
	}
	if err != nil {
		return err
	}

	// Unmarshal "Dependencies" values as either string arrays or schemas
	// and assign them to specific map DependencySchemas or DependencyStrings.
	for k, v := range ms.Dependencies {
		if len(v) > 0 {
			switch v[0] {
			case '[':
				var dstrings []string
				err = json.Unmarshal(v, &dstrings)
				if s.DependencyStrings == nil {
					s.DependencyStrings = make(map[string][]string)
				}
				s.DependencyStrings[k] = dstrings
			default:
				var dschema Schema
				err = json.Unmarshal(v, &dschema)
				if s.DependencySchemas == nil {
					s.DependencySchemas = make(map[string]*Schema)
				}
				s.DependencySchemas[k] = &dschema
			}
		}
		if err != nil {
			return err
		}
	}

	unmarshalAnyPtr := func(p **any, raw json.RawMessage) error {
		if len(raw) == 0 {
			return nil
		}
		if bytes.Equal(raw, []byte("null")) {
			*p = new(any)
			return nil
		}
		return json.Unmarshal(raw, p)
	}

	// Setting Const to a pointer to null will marshal properly, but won't
	// unmarshal: the *any is set to nil, not a pointer to nil.
	if err := unmarshalAnyPtr(&s.Const, ms.Const); err != nil {
		return err
	}

	set := func(dst **int, src *integer) {
		if src != nil {
			*dst = Ptr(int(*src))
		}
	}

	set(&s.MinLength, ms.MinLength)
	set(&s.MaxLength, ms.MaxLength)
	set(&s.MinItems, ms.MinItems)
	set(&s.MaxItems, ms.MaxItems)
	set(&s.MinProperties, ms.MinProperties)
	set(&s.MaxProperties, ms.MaxProperties)
	set(&s.MinContains, ms.MinContains)
	set(&s.MaxContains, ms.MaxContains)

	return nil
}

type integer int32 // for the integer-valued fields of Schema

func (ip *integer) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		// nothing to do
		return nil
	}
	// If there is a decimal point, src is a floating-point number.
	var i int64
	if bytes.ContainsRune(data, '.') {
		var f float64
		if err := json.Unmarshal(data, &f); err != nil {
			return errors.New("not a number")
		}
		i = int64(f)
		if float64(i) != f {
			return errors.New("not an integer value")
		}
	} else {
		if err := json.Unmarshal(data, &i); err != nil {
			return errors.New("cannot be unmarshaled into an int")
		}
	}
	// Ensure behavior is the same on both 32-bit and 64-bit systems.
	if i < math.MinInt32 || i > math.MaxInt32 {
		return errors.New("integer is out of range")
	}
	*ip = integer(i)
	return nil
}

// Ptr returns a pointer to a new variable whose value is x.
func Ptr[T any](x T) *T { return &x }

// every applies f preorder to every schema under s including s.
// The second argument to f is the path to the schema appended to the argument path.
// It stops when f returns false.
func (s *Schema) every(f func(*Schema) bool) bool {
	return f(s) && s.everyChild(func(s *Schema) bool { return s.every(f) })
}

// everyChild reports whether f is true for every immediate child schema of s.
func (s *Schema) everyChild(f func(*Schema) bool) bool {
	v := reflect.ValueOf(s)
	for _, info := range schemaFieldInfos {
		fv := v.Elem().FieldByIndex(info.sf.Index)
		switch info.sf.Type {
		case schemaType:
			// A field that contains an individual schema. A nil is valid: it just means the field isn't present.
			c := fv.Interface().(*Schema)
			if c != nil && !f(c) {
				return false
			}

		case schemaSliceType:
			slice := fv.Interface().([]*Schema)
			for _, c := range slice {
				if !f(c) {
					return false
				}
			}

		case schemaMapType:
			// Sort keys for determinism.
			m := fv.Interface().(map[string]*Schema)
			for _, k := range slices.Sorted(maps.Keys(m)) {
				if !f(m[k]) {
					return false
				}
			}
		}
	}

	return true
}

// all wraps every in an iterator.
func (s *Schema) all() iter.Seq[*Schema] {
	return func(yield func(*Schema) bool) { s.every(yield) }
}

// children wraps everyChild in an iterator.
func (s *Schema) children() iter.Seq[*Schema] {
	return func(yield func(*Schema) bool) { s.everyChild(yield) }
}

var (
	schemaType      = reflect.TypeFor[*Schema]()
	schemaSliceType = reflect.TypeFor[[]*Schema]()
	schemaMapType   = reflect.TypeFor[map[string]*Schema]()
)

type structFieldInfo struct {
	sf       reflect.StructField
	jsonName string
}

var (
	// the visible fields of Schema that have a JSON name, sorted by that name
	schemaFieldInfos []structFieldInfo
	// map from JSON name to field
	schemaFieldMap = map[string]reflect.StructField{}
)

func init() {
	t := reflect.VisibleFields(reflect.TypeFor[Schema]())
	for _, sf := range t {
		info := fieldJSONInfo(sf)
		if !info.omit {
			schemaFieldInfos = append(schemaFieldInfos, structFieldInfo{sf, info.name})
		} else {
			// jsoninfo.name is used to build the info paths. The items and dependencies are ommited,
			// since the original fields are separated to handle the union types supported in json and
			// these fields have custom marshalling and unmarshalling logic.
			// we still need these fields in schemaFieldInfos for creating schema trees and calculating paths and refs.
			// so we manually create them and assign the jsonName to the original field json name.
			switch sf.Name {
			case "Items", "ItemsArray":
				schemaFieldInfos = append(schemaFieldInfos, structFieldInfo{sf, "items"})
			case "DependencySchemas", "DependencyStrings":
				schemaFieldInfos = append(schemaFieldInfos, structFieldInfo{sf, "dependencies"})
			}
		}
	}
	// The value of "dependencies" this sort of schemaFieldInfos.
	// This sort is unstable and is comparing the json.names of DependencyStrings and DependencySchemas which are both "dependencies".
	// Since the sort is unstable it cannot be guarantied that "dependencies" has the DependencySchemas value.
	slices.SortFunc(schemaFieldInfos, func(i1, i2 structFieldInfo) int {
		return cmp.Compare(i1.jsonName, i2.jsonName)
	})
	for _, info := range schemaFieldInfos {
		schemaFieldMap[info.jsonName] = info.sf
	}
}
