// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/maphash"
	"iter"
	"math"
	"math/big"
	"reflect"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

// The values of the "$schema" keyword for the versions that we can validate.
const (
	draft7SchemaVersion      = "http://json-schema.org/draft-07/schema#"
	draft7SecSchemaVersion   = "https://json-schema.org/draft-07/schema#"
	draft202012SchemaVersion = "https://json-schema.org/draft/2020-12/schema"
)

// isValidSchemaVersion checks if the given schema version is supported
func isValidSchemaVersion(version string) bool {
	return version == "" || version == draft7SchemaVersion || version == draft7SecSchemaVersion || version == draft202012SchemaVersion
}

// Validate validates the instance, which must be a JSON value, against the schema.
// It returns nil if validation is successful or an error if it is not.
// If the schema type is "object", instance should be a map[string]any.
func (rs *Resolved) Validate(instance any) error {
	if s := rs.root.Schema; !isValidSchemaVersion(s) {
		return fmt.Errorf("cannot validate version %s, supported versions: draft-07 and draft 2020-12", s)
	}
	st := &state{rs: rs}
	return st.validate(reflect.ValueOf(instance), st.rs.root, nil)
}

// validateDefaults walks the schema tree. If it finds a default, it validates it
// against the schema containing it.
//
// TODO(jba): account for dynamic refs. This algorithm simple-mindedly
// treats each schema with a default as its own root.
func (rs *Resolved) validateDefaults() error {
	if s := rs.root.Schema; !isValidSchemaVersion(s) {
		return fmt.Errorf("cannot validate version %s, supported versions: draft-07 and draft 2020-12", s)
	}
	st := &state{rs: rs}
	for s := range rs.root.all() {
		// We checked for nil schemas in [Schema.Resolve].
		assert(s != nil, "nil schema")
		if s.DynamicRef != "" {
			return fmt.Errorf("jsonschema: %s: validateDefaults does not support dynamic refs", rs.schemaString(s))
		}
		if s.Default != nil {
			var d any
			if err := json.Unmarshal(s.Default, &d); err != nil {
				return fmt.Errorf("unmarshaling default value of schema %s: %w", rs.schemaString(s), err)
			}
			if err := st.validate(reflect.ValueOf(d), s, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// state is the state of single call to ResolvedSchema.Validate.
type state struct {
	rs *Resolved
	// stack holds the schemas from recursive calls to validate.
	// These are the "dynamic scopes" used to resolve dynamic references.
	// https://json-schema.org/draft/2020-12/json-schema-core#scopes
	stack []*Schema
}

// validate validates the reflected value of the instance.
func (st *state) validate(instance reflect.Value, schema *Schema, callerAnns *annotations) (err error) {
	defer wrapf(&err, "validating %s", st.rs.schemaString(schema))

	// Maintain a stack for dynamic schema resolution.
	st.stack = append(st.stack, schema) // push
	defer func() {
		st.stack = st.stack[:len(st.stack)-1] // pop
	}()

	// We checked for nil schemas in [Schema.Resolve].
	assert(schema != nil, "nil schema")

	// Step through interfaces and pointers.
	for instance.Kind() == reflect.Pointer || instance.Kind() == reflect.Interface {
		instance = instance.Elem()
	}

	schemaInfo := st.rs.resolvedInfos[schema]

	var anns annotations // all the annotations for this call and child calls
	// $ref: https://json-schema.org/draft/2020-12/json-schema-core#section-8.2.3.1
	if schema.Ref != "" {
		if err := st.validate(instance, schemaInfo.resolvedRef, &anns); err != nil {
			return err
		}
		// https://json-schema.org/draft-07/draft-handrews-json-schema-01#rfc.section.8.3
		// "All other properties in a "$ref" object MUST be ignored."
		if st.rs.draft == draft7 {
			return nil
		}
	}

	// type: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.1.1
	if schema.Type != "" || schema.Types != nil {
		gotType, ok := jsonType(instance)
		if !ok {
			return fmt.Errorf("type: %v of type %[1]T is not a valid JSON value", instance)
		}
		if schema.Type != "" {
			// "number" subsumes integers
			if !(gotType == schema.Type ||
				gotType == "integer" && schema.Type == "number") {
				return fmt.Errorf("type: %v has type %q, want %q", instance, gotType, schema.Type)
			}
		} else {
			if !(slices.Contains(schema.Types, gotType) || (gotType == "integer" && slices.Contains(schema.Types, "number"))) {
				return fmt.Errorf("type: %v has type %q, want one of %q",
					instance, gotType, strings.Join(schema.Types, ", "))
			}
		}
	}
	// enum: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.1.2
	if schema.Enum != nil {
		ok := false
		for _, e := range schema.Enum {
			if equalValue(reflect.ValueOf(e), instance) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("enum: %v does not equal any of: %v", instance, schema.Enum)
		}
	}

	// const: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.1.3
	if schema.Const != nil {
		if !equalValue(reflect.ValueOf(*schema.Const), instance) {
			return fmt.Errorf("const: %v does not equal %v", instance, *schema.Const)
		}
	}

	// numbers: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.2
	if schema.MultipleOf != nil || schema.Minimum != nil || schema.Maximum != nil || schema.ExclusiveMinimum != nil || schema.ExclusiveMaximum != nil {
		n, ok := jsonNumber(instance)
		if ok { // these keywords don't apply to non-numbers
			if schema.MultipleOf != nil {
				// TODO: validate MultipleOf as non-zero.
				// The test suite assumes floats.
				nf, _ := n.Float64() // don't care if it's exact or not
				if _, f := math.Modf(nf / *schema.MultipleOf); f != 0 {
					return fmt.Errorf("multipleOf: %s is not a multiple of %f", n, *schema.MultipleOf)
				}
			}

			m := new(big.Rat) // reuse for all of the following
			cmp := func(f float64) int { return n.Cmp(m.SetFloat64(f)) }

			if schema.Minimum != nil && cmp(*schema.Minimum) < 0 {
				return fmt.Errorf("minimum: %s is less than %f", n, *schema.Minimum)
			}
			if schema.Maximum != nil && cmp(*schema.Maximum) > 0 {
				return fmt.Errorf("maximum: %s is greater than %f", n, *schema.Maximum)
			}
			if schema.ExclusiveMinimum != nil && cmp(*schema.ExclusiveMinimum) <= 0 {
				return fmt.Errorf("exclusiveMinimum: %s is less than or equal to %f", n, *schema.ExclusiveMinimum)
			}
			if schema.ExclusiveMaximum != nil && cmp(*schema.ExclusiveMaximum) >= 0 {
				return fmt.Errorf("exclusiveMaximum: %s is greater than or equal to %f", n, *schema.ExclusiveMaximum)
			}
		}
	}

	// strings: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.3
	if instance.Kind() == reflect.String && (schema.MinLength != nil || schema.MaxLength != nil || schema.Pattern != "") {
		str := instance.String()
		n := utf8.RuneCountInString(str)
		if schema.MinLength != nil {
			if m := *schema.MinLength; n < m {
				return fmt.Errorf("minLength: %q contains %d Unicode code points, fewer than %d", str, n, m)
			}
		}
		if schema.MaxLength != nil {
			if m := *schema.MaxLength; n > m {
				return fmt.Errorf("maxLength: %q contains %d Unicode code points, more than %d", str, n, m)
			}
		}

		if schema.Pattern != "" && !schemaInfo.pattern.MatchString(str) {
			return fmt.Errorf("pattern: %q does not match regular expression %q", str, schema.Pattern)
		}
	}

	// $dynamicRef: https://json-schema.org/draft/2020-12/json-schema-core#section-8.2.3.2
	if schema.DynamicRef != "" {
		// The ref behaves lexically or dynamically, but not both.
		assert((schemaInfo.resolvedDynamicRef == nil) != (schemaInfo.dynamicRefAnchor == ""),
			"DynamicRef not resolved properly")
		if schemaInfo.resolvedDynamicRef != nil {
			// Same as $ref.
			if err := st.validate(instance, schemaInfo.resolvedDynamicRef, &anns); err != nil {
				return err
			}
		} else {
			// Dynamic behavior.
			// Look for the base of the outermost schema on the stack with this dynamic
			// anchor. (Yes, outermost: the one farthest from here. This the opposite
			// of how ordinary dynamic variables behave.)
			// Why the base of the schema being validated and not the schema itself?
			// Because the base is the scope for anchors. In fact it's possible to
			// refer to a schema that is not on the stack, but a child of some base
			// on the stack.
			// For an example, search for "detached" in testdata/draft2020-12/dynamicRef.json.
			var dynamicSchema *Schema
			for _, s := range st.stack {
				base := st.rs.resolvedInfos[s].base
				info, ok := st.rs.resolvedInfos[base].anchors[schemaInfo.dynamicRefAnchor]
				if ok && info.dynamic {
					dynamicSchema = info.schema
					break
				}
			}
			if dynamicSchema == nil {
				return fmt.Errorf("missing dynamic anchor %q", schemaInfo.dynamicRefAnchor)
			}
			if err := st.validate(instance, dynamicSchema, &anns); err != nil {
				return err
			}
		}
	}

	// logic
	// https://json-schema.org/draft/2020-12/json-schema-core#section-10.2
	// These must happen before arrays and objects because if they evaluate an item or property,
	// then the unevaluatedItems/Properties schemas don't apply to it.
	// See https://json-schema.org/draft/2020-12/json-schema-core#section-11.2, paragraph 4.
	//
	// If any of these fail, then validation fails, even if there is an unevaluatedXXX
	// keyword in the schema. The spec is unclear about this, but that is the intention.

	valid := func(s *Schema, anns *annotations) bool { return st.validate(instance, s, anns) == nil }

	if schema.AllOf != nil {
		for _, ss := range schema.AllOf {
			if err := st.validate(instance, ss, &anns); err != nil {
				return err
			}
		}
	}
	if schema.AnyOf != nil {
		// We must visit them all, to collect annotations.
		var errs []error
		for _, ss := range schema.AnyOf {
			if err := st.validate(instance, ss, &anns); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) == len(schema.AnyOf) {
			return fmt.Errorf("anyOf: did not validate against any of %v:\n%v",
				schema.AnyOf, errors.Join(errs...))
		}
	}
	if schema.OneOf != nil {
		// Exactly one.
		var okSchema *Schema
		for _, ss := range schema.OneOf {
			if valid(ss, &anns) {
				if okSchema != nil {
					return fmt.Errorf("oneOf: validated against both %v and %v", okSchema, ss)
				}
				okSchema = ss
			}
		}
		if okSchema == nil {
			return fmt.Errorf("oneOf: did not validate against any of %v", schema.OneOf)
		}
	}
	if schema.Not != nil {
		// Ignore annotations from "not".
		if valid(schema.Not, nil) {
			return fmt.Errorf("not: validated against %v", schema.Not)
		}
	}
	if schema.If != nil {
		var ss *Schema
		if valid(schema.If, &anns) {
			ss = schema.Then
		} else {
			ss = schema.Else
		}
		if ss != nil {
			if err := st.validate(instance, ss, &anns); err != nil {
				return err
			}
		}
	}

	// arrays
	if instance.Kind() == reflect.Array || instance.Kind() == reflect.Slice {
		// Handle both draft-07 and draft 2020-12
		// https://json-schema.org/draft/2020-12/json-schema-core#section-10.3.1
		// This validate call doesn't collect annotations for the items of the instance; they are separate
		// instances in their own right.
		// TODO(jba): if the test suite doesn't cover this case, add a test. For example, nested arrays.
		if st.rs.draft == draft7 {
			// For draft-07: additionalItems applies to remaining items after items array.
			// If items is a Schema or if items is not set, additionalItems should be ignored
			if schema.ItemsArray != nil {
				for i, ischema := range schema.ItemsArray {
					if i >= instance.Len() {
						break // shorter is OK
					}
					if err := st.validate(instance.Index(i), ischema, nil); err != nil {
						return err
					}
				}
				anns.noteEndIndex(min(len(schema.ItemsArray), instance.Len()))
				if schema.AdditionalItems != nil {
					for i := len(schema.ItemsArray); i < instance.Len(); i++ {
						if err := st.validate(instance.Index(i), schema.AdditionalItems, nil); err != nil {
							return err
						}
					}
					anns.allItems = true
				}
			} else if schema.Items != nil {
				for i := 0; i < instance.Len(); i++ {
					if err := st.validate(instance.Index(i), schema.Items, nil); err != nil {
						return err
					}
				}
				// Note that all the items in this array have been validated.
				anns.allItems = true
			}
		} else if st.rs.draft == draft2020 {
			// For draft 2020-12: items applies to remaining items after prefixItems
			for i, ischema := range schema.PrefixItems {
				if i >= instance.Len() {
					break // shorter is OK
				}
				if err := st.validate(instance.Index(i), ischema, nil); err != nil {
					return err
				}
			}
			anns.noteEndIndex(min(len(schema.PrefixItems), instance.Len()))
			if schema.Items != nil {
				for i := len(schema.PrefixItems); i < instance.Len(); i++ {
					if err := st.validate(instance.Index(i), schema.Items, nil); err != nil {
						return err
					}
				}
				// Note that all the items in this array have been validated.
				anns.allItems = true
			}
		}
		nContains := 0
		if schema.Contains != nil {
			for i := range instance.Len() {
				if err := st.validate(instance.Index(i), schema.Contains, nil); err == nil {
					nContains++
					anns.noteIndex(i)
				}
			}
			if nContains == 0 && (schema.MinContains == nil || *schema.MinContains > 0) {
				return fmt.Errorf("contains: %s does not have an item matching %s", instance, schema.Contains)
			}
		}

		// https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.4
		// TODO(jba): check that these next four keywords' values are integers.
		if schema.MinContains != nil && schema.Contains != nil {
			if m := *schema.MinContains; nContains < m {
				return fmt.Errorf("minContains: contains validated %d items, less than %d", nContains, m)
			}
		}
		if schema.MaxContains != nil && schema.Contains != nil {
			if m := *schema.MaxContains; nContains > m {
				return fmt.Errorf("maxContains: contains validated %d items, greater than %d", nContains, m)
			}
		}
		if schema.MinItems != nil {
			if m := *schema.MinItems; instance.Len() < m {
				return fmt.Errorf("minItems: array length %d is less than %d", instance.Len(), m)
			}
		}
		if schema.MaxItems != nil {
			if m := *schema.MaxItems; instance.Len() > m {
				return fmt.Errorf("maxItems: array length %d is greater than %d", instance.Len(), m)
			}
		}
		if schema.UniqueItems {
			if instance.Len() > 1 {
				// Hash each item and compare the hashes.
				// If two hashes differ, the items differ.
				// If two hashes are the same, compare the collisions for equality.
				// (The same logic as hash table lookup.)
				// TODO(jba): Use container/hash.Map when it becomes available (https://go.dev/issue/69559),
				hashes := map[uint64][]int{} // from hash to indices
				seed := maphash.MakeSeed()
				for i := range instance.Len() {
					item := instance.Index(i)
					var h maphash.Hash
					h.SetSeed(seed)
					hashValue(&h, item)
					hv := h.Sum64()
					if sames := hashes[hv]; len(sames) > 0 {
						for _, j := range sames {
							if equalValue(item, instance.Index(j)) {
								return fmt.Errorf("uniqueItems: array items %d and %d are equal", i, j)
							}
						}
					}
					hashes[hv] = append(hashes[hv], i)
				}
			}
		}

		// https://json-schema.org/draft/2020-12/json-schema-core#section-11.2
		if schema.UnevaluatedItems != nil && !anns.allItems {
			// Apply this subschema to all items in the array that haven't been successfully validated.
			// That includes validations by subschemas on the same instance, like allOf.
			for i := anns.endIndex; i < instance.Len(); i++ {
				if !anns.evaluatedIndexes[i] {
					if err := st.validate(instance.Index(i), schema.UnevaluatedItems, nil); err != nil {
						return err
					}
				}
			}
			anns.allItems = true
		}
	}

	// objects
	// https://json-schema.org/draft/2020-12/json-schema-core#section-10.3.2
	// Validating structs is problematic. See https://github.com/google/jsonschema-go/issues/23.
	if instance.Kind() == reflect.Struct {
		return errors.New("cannot validate against a struct; see https://github.com/google/jsonschema-go/issues/23 for details")
	}
	if instance.Kind() == reflect.Map {
		if kt := instance.Type().Key(); kt.Kind() != reflect.String {
			return fmt.Errorf("map key type %s is not a string", kt)
		}
		// Track the evaluated properties for just this schema, to support additionalProperties.
		// If we used anns here, then we'd be including properties evaluated in subschemas
		// from allOf, etc., which additionalProperties shouldn't observe.
		evalProps := map[string]bool{}
		for prop, subschema := range schema.Properties {
			val := property(instance, prop)
			if !val.IsValid() {
				// It's OK if the instance doesn't have the property.
				continue
			}
			// If the instance is a struct and an optional property has the zero
			// value, then we could interpret it as present or missing. Be generous:
			// assume it's missing, and thus always validates successfully.
			if instance.Kind() == reflect.Struct && val.IsZero() && !schemaInfo.isRequired[prop] {
				continue
			}
			if err := st.validate(val, subschema, nil); err != nil {
				return err
			}
			evalProps[prop] = true
		}
		if len(schema.PatternProperties) > 0 {
			for prop, val := range properties(instance) {
				// Check every matching pattern.
				for re, schema := range schemaInfo.patternProperties {
					if re.MatchString(prop) {
						if err := st.validate(val, schema, nil); err != nil {
							return err
						}
						evalProps[prop] = true
					}
				}
			}
		}
		if schema.AdditionalProperties != nil {
			// Special case for a better error message when additional properties is
			// 'falsy'
			//
			// If additionalProperties is {"not":{}} (which is how we
			// unmarshal "false"), we can produce a better error message that
			// summarizes all the extra properties. Otherwise, we fall back to the
			// default validation.
			//
			// Note: this is much faster than comparing with falseSchema using Equal.
			isFalsy := schema.AdditionalProperties.Not != nil && reflect.ValueOf(*schema.AdditionalProperties.Not).IsZero()
			if isFalsy {
				var disallowed []string
				for prop := range properties(instance) {
					if !evalProps[prop] {
						disallowed = append(disallowed, prop)
					}
				}
				if len(disallowed) > 0 {
					return fmt.Errorf("unexpected additional properties %q", disallowed)
				}
			} else {
				// Apply to all properties not handled above.
				for prop, val := range properties(instance) {
					if !evalProps[prop] {
						if err := st.validate(val, schema.AdditionalProperties, nil); err != nil {
							return err
						}
						evalProps[prop] = true
					}
				}
			}
		}
		anns.noteProperties(evalProps)
		if schema.PropertyNames != nil {
			// Note: properties unnecessarily fetches each value. We could define a propertyNames function
			// if performance ever matters.
			for prop := range properties(instance) {
				if err := st.validate(reflect.ValueOf(prop), schema.PropertyNames, nil); err != nil {
					return err
				}
			}
		}

		// https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-01#section-6.5
		var min, max int
		if schema.MinProperties != nil || schema.MaxProperties != nil {
			min, max = numPropertiesBounds(instance, schemaInfo.isRequired)
		}
		if schema.MinProperties != nil {
			if n, m := max, *schema.MinProperties; n < m {
				return fmt.Errorf("minProperties: object has %d properties, less than %d", n, m)
			}
		}
		if schema.MaxProperties != nil {
			if n, m := min, *schema.MaxProperties; n > m {
				return fmt.Errorf("maxProperties: object has %d properties, greater than %d", n, m)
			}
		}

		hasProperty := func(prop string) bool {
			return property(instance, prop).IsValid()
		}

		missingProperties := func(props []string) []string {
			var missing []string
			for _, p := range props {
				if !hasProperty(p) {
					missing = append(missing, p)
				}
			}
			return missing
		}

		if schema.Required != nil {
			if m := missingProperties(schema.Required); len(m) > 0 {
				return fmt.Errorf("required: missing properties: %q", m)
			}
		}

		if st.rs.draft == draft7 {
			if schema.DependencyStrings != nil {
				for dprop, dstrings := range schema.DependencyStrings {
					if hasProperty(dprop) {
						if m := missingProperties(dstrings); len(m) > 0 {
							return fmt.Errorf("dependentRequired[%q]: missing properties %q", dprop, m)
						}
					}
				}
			}
			if schema.DependencySchemas != nil {
				for dprop, dschema := range schema.DependencySchemas {
					if hasProperty(dprop) {
						err := st.validate(instance, dschema, &anns)
						if err != nil {
							return err
						}
					}
				}
			}
		} else if st.rs.draft == draft2020 {
			if schema.DependentRequired != nil {
				// "Validation succeeds if, for each name that appears in both the instance
				// and as a name within this keyword's value, every item in the corresponding
				// array is also the name of a property in the instance." §6.5.4
				for dprop, reqs := range schema.DependentRequired {
					if hasProperty(dprop) {
						if m := missingProperties(reqs); len(m) > 0 {
							return fmt.Errorf("dependentRequired[%q]: missing properties %q", dprop, m)
						}
					}
				}
			}

			// https://json-schema.org/draft/2020-12/json-schema-core#section-10.2.2.4
			if schema.DependentSchemas != nil {
				// This does not collect annotations, although it seems like it should.
				for dprop, ss := range schema.DependentSchemas {
					if hasProperty(dprop) {
						// TODO: include dependentSchemas[dprop] in the errors.
						err := st.validate(instance, ss, &anns)
						if err != nil {
							return err
						}
					}
				}
			}
		}

		if schema.UnevaluatedProperties != nil && !anns.allProperties {
			// This looks a lot like AdditionalProperties, but depends on in-place keywords like allOf
			// in addition to sibling keywords.
			for prop, val := range properties(instance) {
				if !anns.evaluatedProperties[prop] {
					if err := st.validate(val, schema.UnevaluatedProperties, nil); err != nil {
						return err
					}
				}
			}
			// The spec says the annotation should be the set of evaluated properties, but we can optimize
			// by setting a single boolean, since after this succeeds all properties will be validated.
			// See https://json-schema.slack.com/archives/CT7FF623C/p1745592564381459.
			anns.allProperties = true
		}
	}

	if callerAnns != nil {
		// Our caller wants to know what we've validated.
		callerAnns.merge(&anns)
	}
	return nil
}

// resolveDynamicRef returns the schema referred to by the argument schema's
// $dynamicRef value.
// It returns an error if the dynamic reference has no referent.
// If there is no $dynamicRef, resolveDynamicRef returns nil, nil.
// See https://json-schema.org/draft/2020-12/json-schema-core#section-8.2.3.2.
func (st *state) resolveDynamicRef(schema *Schema) (*Schema, error) {
	if schema.DynamicRef == "" {
		return nil, nil
	}
	info := st.rs.resolvedInfos[schema]
	// The ref behaves lexically or dynamically, but not both.
	assert((info.resolvedDynamicRef == nil) != (info.dynamicRefAnchor == ""),
		"DynamicRef not statically resolved properly")
	if r := info.resolvedDynamicRef; r != nil {
		// Same as $ref.
		return r, nil
	}
	// Dynamic behavior.
	// Look for the base of the outermost schema on the stack with this dynamic
	// anchor. (Yes, outermost: the one farthest from here. This the opposite
	// of how ordinary dynamic variables behave.)
	// Why the base of the schema being validated and not the schema itself?
	// Because the base is the scope for anchors. In fact it's possible to
	// refer to a schema that is not on the stack, but a child of some base
	// on the stack.
	// For an example, search for "detached" in testdata/draft2020-12/dynamicRef.json.
	for _, s := range st.stack {
		base := st.rs.resolvedInfos[s].base
		info, ok := st.rs.resolvedInfos[base].anchors[info.dynamicRefAnchor]
		if ok && info.dynamic {
			return info.schema, nil
		}
	}
	return nil, fmt.Errorf("missing dynamic anchor %q", info.dynamicRefAnchor)
}

// ApplyDefaults modifies an instance by applying the schema's defaults to it. If
// a schema or sub-schema has a default, then a corresponding missing instance value
// is set to the default.
//
// The JSON Schema specification does not describe how defaults should be interpreted.
// This method honors defaults only on properties, and only those that are not required.
// If the instance is a map and the property is missing, the property is added to
// the map with the default.
// ApplyDefaults does not support structs, because it cannot know whether a field
// is missing in the JSON, or was explicitly set to its zero value.
//
// ApplyDefaults can panic if a default cannot be assigned to a field.
//
// The argument must be a pointer to the instance.
// (In case we decide that top-level defaults are meaningful.)
//
// It is recommended to first call Resolve with a ValidateDefaults option of true,
// then call this method, and lastly call Validate.
func (rs *Resolved) ApplyDefaults(instancep any) error {
	// TODO(jba): consider what defaults on top-level or array instances might mean.
	// TODO(jba): follow $ref and $dynamicRef
	st := &state{rs: rs}
	return st.applyDefaults(reflect.ValueOf(instancep), rs.root)
}

// Recursive helper used by ApplyDefaults. Applies defaults on sub-schemas
// of object properties recursively.
func (st *state) applyDefaults(instancep reflect.Value, schema *Schema) (err error) {
	defer wrapf(&err, "applyDefaults: schema %s, instance %v", st.rs.schemaString(schema), instancep)

	schemaInfo := st.rs.resolvedInfos[schema]
	instance := instancep.Elem()
	if instance.Kind() == reflect.Interface && instance.IsValid() {
		// If we unmarshalled into 'any', the default object unmarshalling will be map[string]any.
		instance = instance.Elem()
	}
	if instance.Kind() == reflect.Map || instance.Kind() == reflect.Struct {
		if instance.Kind() == reflect.Map {
			if kt := instance.Type().Key(); kt.Kind() != reflect.String {
				return fmt.Errorf("map key type %s is not a string", kt)
			}
		}
		for prop, subschema := range schema.Properties {
			// Ignore defaults on required properties. (A required property shouldn't have a default.)
			if schemaInfo.isRequired[prop] {
				continue
			}
			val := property(instance, prop)
			switch instance.Kind() {
			case reflect.Map:
				// If there is a default for this property, and the map key is missing,
				// set the map value to the default.
				if subschema.Default != nil && !val.IsValid() {
					// Create an lvalue, since map values aren't addressable.
					lvalue := reflect.New(instance.Type().Elem())
					if err := json.Unmarshal(subschema.Default, lvalue.Interface()); err != nil {
						return err
					}
					// Recurse unconditionally; applyDefaults will only act on object-like values.
					if err := st.applyDefaults(lvalue, subschema); err != nil {
						return err
					}
					instance.SetMapIndex(reflect.ValueOf(prop), lvalue.Elem())
				} else if val.IsValid() {
					// Recurse into an existing sub-instance.
					// MapIndex returns a non-addressable value; copy into an addressable lvalue, recurse, then set back.
					lvalue := reflect.New(instance.Type().Elem())
					// Initialize the lvalue with current value.
					lvalue.Elem().Set(val)
					if err := st.applyDefaults(lvalue, subschema); err != nil {
						return err
					}
					instance.SetMapIndex(reflect.ValueOf(prop), lvalue.Elem())
				} else if schemaHasDefaultsInProperties(subschema) {
					// Property is missing, but descendants still have some defaults
					// Create an empty container and recurse to populate
					elemType := instance.Type().Elem()
					var child reflect.Value
					switch elemType.Kind() {
					case reflect.Interface:
						child = reflect.ValueOf(map[string]any{})
					case reflect.Map:
						child = reflect.MakeMap(elemType)
					case reflect.Struct:
						child = reflect.New(elemType).Elem()
					}
					if child.IsValid() {
						lvalue := reflect.New(elemType)
						lvalue.Elem().Set(child)
						if err := st.applyDefaults(lvalue, subschema); err != nil {
							return err
						}
						instance.SetMapIndex(reflect.ValueOf(prop), lvalue.Elem())
					}
				}
			case reflect.Struct:
				return errors.New("cannot apply defaults to a struct")
			default:
				panic(fmt.Sprintf("applyDefaults: property %s: bad value %s of kind %s",
					prop, instance, instance.Kind()))
			}
		}
	}
	return nil
}

// schemaHasDefaultsInProperties reports whether s or any descendant schema under
// its Properties contains a default. Only walks Properties to match ApplyDefaults semantics.
func schemaHasDefaultsInProperties(s *Schema) bool {
	if s == nil {
		return false
	}
	if s.Default != nil {
		return true
	}
	if s.Properties != nil {
		for _, ss := range s.Properties {
			if schemaHasDefaultsInProperties(ss) {
				return true
			}
		}
	}
	return false
}

// property returns the value of the property of v with the given name, or the invalid
// reflect.Value if there is none.
// If v is a map, the property is the value of the map whose key is name.
// If v is a struct, the property is the value of the field with the given name according
// to the encoding/json package (see [jsonName]).
// If v is anything else, property panics.
func property(v reflect.Value, name string) reflect.Value {
	switch v.Kind() {
	case reflect.Map:
		return v.MapIndex(reflect.ValueOf(name))
	case reflect.Struct:
		props := structPropertiesOf(v.Type())
		// Ignore nonexistent properties.
		if sf, ok := props[name]; ok {
			return v.FieldByIndex(sf.Index)
		}
		return reflect.Value{}
	default:
		panic(fmt.Sprintf("property(%q): bad value %s of kind %s", name, v, v.Kind()))
	}
}

// properties returns an iterator over the names and values of all properties
// in v, which must be a map or a struct.
// If a struct, zero-valued properties that are marked omitempty or omitzero
// are excluded.
func properties(v reflect.Value) iter.Seq2[string, reflect.Value] {
	return func(yield func(string, reflect.Value) bool) {
		switch v.Kind() {
		case reflect.Map:
			for k, e := range v.Seq2() {
				if !yield(k.String(), e) {
					return
				}
			}
		case reflect.Struct:
			for name, sf := range structPropertiesOf(v.Type()) {
				val := v.FieldByIndex(sf.Index)
				if val.IsZero() {
					info := fieldJSONInfo(sf)
					if info.settings["omitempty"] || info.settings["omitzero"] {
						continue
					}
				}
				if !yield(name, val) {
					return
				}
			}
		default:
			panic(fmt.Sprintf("bad value %s of kind %s", v, v.Kind()))
		}
	}
}

// numPropertiesBounds returns bounds on the number of v's properties.
// v must be a map or a struct.
// If v is a map, both bounds are the map's size.
// If v is a struct, the max is the number of struct properties.
// But since we don't know whether a zero value indicates a missing optional property
// or not, be generous and use the number of non-zero properties as the min.
func numPropertiesBounds(v reflect.Value, isRequired map[string]bool) (int, int) {
	switch v.Kind() {
	case reflect.Map:
		return v.Len(), v.Len()
	case reflect.Struct:
		sp := structPropertiesOf(v.Type())
		min := 0
		for prop, sf := range sp {
			if !v.FieldByIndex(sf.Index).IsZero() || isRequired[prop] {
				min++
			}
		}
		return min, len(sp)
	default:
		panic(fmt.Sprintf("properties: bad value: %s of kind %s", v, v.Kind()))
	}
}

// A propertyMap is a map from property name to struct field index.
type propertyMap = map[string]reflect.StructField

var structProperties sync.Map // from reflect.Type to propertyMap

// structPropertiesOf returns the JSON Schema properties for the struct type t.
// The caller must not mutate the result.
func structPropertiesOf(t reflect.Type) propertyMap {
	// Mutex not necessary: at worst we'll recompute the same value.
	if props, ok := structProperties.Load(t); ok {
		return props.(propertyMap)
	}
	props := map[string]reflect.StructField{}
	for _, sf := range reflect.VisibleFields(t) {
		if sf.Anonymous {
			continue
		}
		info := fieldJSONInfo(sf)
		if !info.omit {
			props[info.name] = sf
		}
	}
	structProperties.Store(t, props)
	return props
}
