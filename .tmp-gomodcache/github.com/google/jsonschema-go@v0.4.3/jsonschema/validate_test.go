// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// The test for validation uses the official test suite, expressed as a set of JSON files.
// Each file is an array of group objects.

// A testGroup consists of a schema and some tests on it.
type testGroup struct {
	Description string
	Schema      *Schema
	Tests       []test
}

// A test consists of a JSON instance to be validated and the expected result.
type test struct {
	Description string
	Data        any
	Valid       bool
	ErrContains string
}

func TestValidateDraft2020_12(t *testing.T) {
	files, err := filepath.Glob(filepath.FromSlash("testdata/draft2020-12/*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no files")
	}
	testValidate(t, files, "")
}

func TestValidateDraft7(t *testing.T) {
	files, err := filepath.Glob(filepath.FromSlash("testdata/draft7/*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no files")
	}
	testValidate(t, files, "https://json-schema.org/draft-07/schema#")
}

func testValidate(t *testing.T, files []string, draft string) {
	for _, file := range files {
		base := filepath.Base(file)
		t.Run(base, func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var groups []testGroup
			if err := json.Unmarshal(data, &groups); err != nil {
				t.Fatal(err)
			}
			for _, g := range groups {
				t.Run(g.Description, func(t *testing.T) {
					if g.Schema.Schema == "" {
						g.Schema.Schema = draft
					}
					rs, err := g.Schema.Resolve(&ResolveOptions{Loader: loadRemote})
					if err != nil {
						t.Fatal(err)
					}
					for _, test := range g.Tests {
						t.Run(test.Description, func(t *testing.T) {
							err = rs.Validate(test.Data)
							if err != nil && test.Valid {
								t.Errorf("wanted success, but failed with: %v", err)
							}
							if err == nil && !test.Valid {
								t.Error("succeeded but wanted failure")
							}
							if err != nil && test.ErrContains != "" {
								if !strings.Contains(err.Error(), test.ErrContains) {
									t.Errorf("got error %q, want containing %q", err, test.ErrContains)
								}
							}
							if t.Failed() {
								t.Errorf("schema: %s", g.Schema.json())
								t.Fatalf("instance: %v (%[1]T)", test.Data)
							}
						})
					}
				})
			}
		})
	}
}

func TestValidateErrors(t *testing.T) {
	schema := &Schema{
		PrefixItems: []*Schema{{Contains: &Schema{Type: "integer"}}},
	}
	rs, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	err = rs.Validate([]any{[]any{"1"}})
	want := "prefixItems/0"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("error:\n%s\ndoes not contain %q", err, want)
	}
}

func TestValidateDefaults(t *testing.T) {
	s := &Schema{
		Properties: map[string]*Schema{
			"a": {Type: "integer", Default: mustMarshal(1)},
			"b": {Type: "string", Default: mustMarshal("s")},
		},
		Default: mustMarshal(map[string]any{"a": 1, "b": "two"}),
	}
	if _, err := s.Resolve(&ResolveOptions{ValidateDefaults: true}); err != nil {
		t.Fatal(err)
	}

	s = &Schema{
		Properties: map[string]*Schema{
			"a": {Type: "integer", Default: mustMarshal(3)},
			"b": {Type: "string", Default: mustMarshal("s")},
		},
		Default: mustMarshal(map[string]any{"a": 1, "b": 2}),
	}
	_, err := s.Resolve(&ResolveOptions{ValidateDefaults: true})
	want := `has type "integer", want "string"`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Resolve returned error %q, want %q", err, want)
	}
}

func TestApplyDefaults(t *testing.T) {
	schema := &Schema{
		Properties: map[string]*Schema{
			"A": {Default: mustMarshal(1)},
			"B": {Default: mustMarshal(2)},
			"C": {Default: mustMarshal(3)},
		},
		Required: []string{"C"},
	}
	rs, err := schema.Resolve(&ResolveOptions{ValidateDefaults: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		instancep any // pointer to instance value
		want      any // desired value (not a pointer)
	}{
		{
			&map[string]any{"B": 0},
			map[string]any{
				"A": float64(1), // filled from default
				"B": 0,          // untouched: it was already there
				// "C" not added: it is required (Validate will catch that)
			},
		},
	} {
		if err := rs.ApplyDefaults(tt.instancep); err != nil {
			t.Fatal(err)
		}
		got := reflect.ValueOf(tt.instancep).Elem().Interface() // dereference the pointer
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("\ngot  %#v\nwant %#v", got, tt.want)
		}
	}
}

func TestApplyNestedDefaults(t *testing.T) {
	base := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"A": {
				Type: "object",
				Properties: map[string]*Schema{
					"B": {Type: "string", Default: mustMarshal("foo")},
				},
			},
		},
	}
	// Variant where parent A has its own default object; recursion should still fill B.
	withParentDefault := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"A": {
				Type:    "object",
				Default: mustMarshal(map[string]any{"X": 1}),
				Properties: map[string]*Schema{
					"B": {Type: "string", Default: mustMarshal("foo")},
				},
			},
		},
	}

	for _, tc := range []struct {
		name      string
		schema    *Schema
		instancep any
		want      any
	}{
		{
			name:      "MapMissingParent",
			schema:    base,
			instancep: &map[string]any{},
			want:      map[string]any{"A": map[string]any{"B": "foo"}},
		},
		{
			name:      "MapEmptyParent",
			schema:    base,
			instancep: &map[string]any{"A": map[string]any{}},
			want:      map[string]any{"A": map[string]any{"B": "foo"}},
		},
		{
			name:      "MapParentHasDefaultObjectMissing",
			schema:    withParentDefault,
			instancep: &map[string]any{},
			want:      map[string]any{"A": map[string]any{"X": float64(1), "B": "foo"}},
		},
		{
			name:      "MapParentHasDefaultObjectPresent",
			schema:    withParentDefault,
			instancep: &map[string]any{"A": map[string]any{}},
			// Parent default is applied only when the property is missing, so
			// with the key present (even if empty), we apply only the nested defaults.
			want: map[string]any{"A": map[string]any{"B": "foo"}},
		},
		{
			name:      "MapValueMapMissingParentTyped",
			schema:    base,
			instancep: &map[string]map[string]any{},
			want:      map[string]map[string]any{"A": {"B": "foo"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs, err := tc.schema.Resolve(&ResolveOptions{ValidateDefaults: true})
			if err != nil {
				t.Fatal(err)
			}
			if err := rs.ApplyDefaults(tc.instancep); err != nil {
				t.Fatal(err)
			}
			got := reflect.ValueOf(tc.instancep).Elem().Interface()
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("nested defaults:\n got  %#v\n want %#v", got, tc.want)
			}
		})
	}
}

func TestStructInstance(t *testing.T) {
	instance := struct {
		I int
		B bool `json:"b"`
		P *int // either missing or nil
		u int  // unexported: not a property
	}{1, true, nil, 0}

	for _, tt := range []struct {
		s    Schema
		want bool
	}{
		{
			Schema{MinProperties: Ptr(4)},
			false,
		},
		{
			Schema{MinProperties: Ptr(3)},
			true, // P interpreted as present
		},
		{
			Schema{MaxProperties: Ptr(1)},
			false,
		},
		{
			Schema{MaxProperties: Ptr(2)},
			true, // P interpreted as absent
		},
		{
			Schema{Required: []string{"i"}}, // the name is "I"
			false,
		},
		{
			Schema{Required: []string{"B"}}, // the name is "b"
			false,
		},
		{
			Schema{PropertyNames: &Schema{MinLength: Ptr(2)}},
			false,
		},
		{
			Schema{Properties: map[string]*Schema{"b": {Type: "boolean"}}},
			true,
		},
		{
			Schema{Properties: map[string]*Schema{"b": {Type: "number"}}},
			false,
		},
		{
			Schema{Required: []string{"I"}},
			true,
		},
		{
			Schema{Required: []string{"I", "P"}},
			true, // P interpreted as present
		},
		{
			Schema{Required: []string{"I", "P"}, Properties: map[string]*Schema{"P": {Type: "number"}}},
			false, // P interpreted as present, but not a number
		},
		{
			Schema{Required: []string{"I"}, Properties: map[string]*Schema{"P": {Type: "number"}}},
			true, // P not required, so interpreted as absent
		},
		{
			Schema{Required: []string{"I"}, AdditionalProperties: falseSchema()},
			false,
		},
		{
			Schema{DependentRequired: map[string][]string{"b": {"u"}}},
			false,
		},
		{
			Schema{DependentSchemas: map[string]*Schema{"b": falseSchema()}},
			false,
		},
		{
			Schema{UnevaluatedProperties: falseSchema()},
			false,
		},
	} {
		res, err := tt.s.Resolve(nil)
		if err != nil {
			t.Fatal(err)
		}
		err = res.Validate(instance)
		// Validating a struct always fails.
		if err == nil {
			t.Error("struct validation succeeded")
		}
	}
}

func TestStructEmbedding(t *testing.T) {
	// For exported pointer embedding
	type Apple struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type Banana struct {
		*Apple        // Pointer embedded - should flatten.
		Extra  string `json:"extra"`
	}

	// For unexported pointer embedding
	type cranberry struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type Durian struct {
		*cranberry        // Pointer embedded - should flatten.
		Extra      string `json:"extra"`
	}

	// For exported value embedding
	type Elderberry struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type Fig struct {
		Elderberry        // Value embedded - should flatten.
		Extra      string `json:"extra"`
	}

	// For unexported value embedding
	type grape struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type Honeyberry struct {
		grape        // Value embedded - should flatten.
		Extra string `json:"extra"`
	}

	// For outer field shadowing a pointer embed
	type Inner struct {
		Conflict  string `json:"conflict_field"` // This string field should be ignored.
		InnerOnly string `json:"inner_only"`
	}
	type Outer struct {
		*Inner
		Conflict int `json:"conflict_field"` // This int field should take precedence.
	}

	testCases := []struct {
		name          string
		targetType    reflect.Type
		wantSchema    *Schema
		validInstance any
	}{
		{
			name:       "Slice",
			targetType: reflect.TypeOf([]Banana{}),
			wantSchema: &Schema{
				Types: []string{"null", "array"},
				Items: &Schema{
					Type: "object",
					Properties: map[string]*Schema{
						"id":    {Type: "string"},
						"name":  {Type: "string"},
						"extra": {Type: "string"},
					},
					Required:             []string{"id", "name", "extra"},
					AdditionalProperties: falseSchema(),
					PropertyOrder:        []string{"id", "name", "extra"},
				},
			},
			validInstance: []Banana{
				{Apple: &Apple{ID: "foo1", Name: "Test Foo 2"}, Extra: "additional data 1"},
				{Apple: &Apple{ID: "foo2", Name: "Test Foo 2"}, Extra: "additional data 2"},
			},
		},
		{
			name:       "Array",
			targetType: reflect.TypeOf([2]Banana{}),
			wantSchema: &Schema{
				Type:     "array",
				MinItems: Ptr(2),
				MaxItems: Ptr(2),
				Items: &Schema{
					Type: "object",
					Properties: map[string]*Schema{
						"id":    {Type: "string"},
						"name":  {Type: "string"},
						"extra": {Type: "string"},
					},
					Required:             []string{"id", "name", "extra"},
					AdditionalProperties: falseSchema(),
					PropertyOrder:        []string{"id", "name", "extra"},
				},
			},
			validInstance: [2]Banana{
				{Apple: &Apple{ID: "foo1", Name: "Test Foo 2"}, Extra: "additional data 1"},
				{Apple: &Apple{ID: "foo2", Name: "Test Foo 2"}, Extra: "additional data 2"},
			},
		},
		{
			name:       "UnExportedPointer",
			targetType: reflect.TypeOf([]Durian{}),
			wantSchema: &Schema{
				Types: []string{"null", "array"},
				Items: &Schema{
					Type: "object",
					Properties: map[string]*Schema{
						"id":    {Type: "string"},
						"name":  {Type: "string"},
						"extra": {Type: "string"},
					},
					Required:             []string{"id", "name", "extra"},
					AdditionalProperties: falseSchema(),
					PropertyOrder:        []string{"id", "name", "extra"},
				},
			},
			validInstance: []Durian{
				{cranberry: &cranberry{ID: "foo1", Name: "Test Foo 2"}, Extra: "additional data 1"},
				{cranberry: &cranberry{ID: "foo2", Name: "Test Foo 2"}, Extra: "additional data 2"},
			},
		},
		{
			name:       "ExportedValue",
			targetType: reflect.TypeOf([]Fig{}),
			wantSchema: &Schema{
				Types: []string{"null", "array"},
				Items: &Schema{
					Type: "object",
					Properties: map[string]*Schema{
						"id":    {Type: "string"},
						"name":  {Type: "string"},
						"extra": {Type: "string"},
					},
					Required:             []string{"id", "name", "extra"},
					AdditionalProperties: falseSchema(),
					PropertyOrder:        []string{"id", "name", "extra"},
				},
			},
			validInstance: []Fig{
				{Elderberry: Elderberry{ID: "foo1", Name: "Test Foo 2"}, Extra: "additional data 1"},
				{Elderberry: Elderberry{ID: "foo2", Name: "Test Foo 2"}, Extra: "additional data 2"},
			},
		},
		{
			name:       "UnExportedValue",
			targetType: reflect.TypeOf([]Honeyberry{}),
			wantSchema: &Schema{
				Types: []string{"null", "array"},
				Items: &Schema{
					Type: "object",
					Properties: map[string]*Schema{
						"id":    {Type: "string"},
						"name":  {Type: "string"},
						"extra": {Type: "string"},
					},
					Required:             []string{"id", "name", "extra"},
					AdditionalProperties: falseSchema(),
					PropertyOrder:        []string{"id", "name", "extra"},
				},
			},
			validInstance: []Honeyberry{
				{grape: grape{ID: "foo1", Name: "Test Foo 2"}, Extra: "additional data 1"},
				{grape: grape{ID: "foo2", Name: "Test Foo 2"}, Extra: "additional data 2"},
			},
		},
		{
			name:       "FieldShadowing",
			targetType: reflect.TypeOf(Outer{}),
			wantSchema: &Schema{
				Type: "object",
				Properties: map[string]*Schema{
					// The "integer" from the Outer struct takes precedence.
					"conflict_field": {Type: "integer"},
					// The non-conflicting field from the Inner struct is still present.
					"inner_only": {Type: "string"},
				},
				Required:             []string{"inner_only", "conflict_field"},
				AdditionalProperties: falseSchema(),
				PropertyOrder:        []string{"inner_only", "conflict_field"},
			},
			validInstance: Outer{Inner: &Inner{InnerOnly: "data"}, Conflict: 123},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			schema, err := ForType(tc.targetType, &ForOptions{})
			if err != nil {
				t.Fatalf("ForType() returned an unexpected error: %v", err)
			}

			if diff := cmp.Diff(tc.wantSchema, schema, cmpopts.IgnoreUnexported(Schema{})); diff != "" {
				t.Fatalf("Schema mismatch (-want +got):\n%s", diff)
			}
			resolved, err := schema.Resolve(nil)
			if err != nil {
				t.Fatalf("schema.Resolve() failed: %v", err)
			}
			// Validate a correct instance against the generated schema.
			// Struct validation always fails.
			if err := resolved.Validate(tc.validInstance); err == nil {
				t.Error("struct validation succeeded")
			}
		})
	}
}

func mustMarshal(x any) json.RawMessage {
	data, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(data)
}

// loadRemote loads a remote reference used in the test suite.
func loadRemote(uri *url.URL) (*Schema, error) {
	// Anything with localhost:1234 refers to the remotes directory in the test suite repo.
	if uri.Host == "localhost:1234" {
		return loadSchemaFromFile(filepath.FromSlash(filepath.Join("testdata/remotes", uri.Path)))
	}
	// One test needs the meta-schema files.
	const metaPrefix = "https://json-schema.org/draft/2020-12/"
	if after, ok := strings.CutPrefix(uri.String(), metaPrefix); ok {
		return loadSchemaFromFile(filepath.FromSlash("meta-schemas/draft2020-12/" + after + ".json"))
	}
	const metaPrefixDraft7 = "https://json-schema.org/draft-07/"
	if after, ok := strings.CutPrefix(uri.String(), metaPrefixDraft7); ok {
		return loadSchemaFromFile(filepath.FromSlash("meta-schemas/draft7/" + after + ".json"))
	}
	const metaPrefixDraft7s = "http://json-schema.org/draft-07/"
	if after, ok := strings.CutPrefix(uri.String(), metaPrefixDraft7s); ok {
		return loadSchemaFromFile(filepath.FromSlash("meta-schemas/draft7/" + after + ".json"))
	}
	return nil, fmt.Errorf("don't know how to load %s", uri)
}

func loadSchemaFromFile(filename string) (*Schema, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var s Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshaling JSON at %s: %w", filename, err)
	}
	return &s, nil
}
