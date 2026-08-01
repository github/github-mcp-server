// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestMarshalJSONConsistency(t *testing.T) {
	// Test that MarshalJSON with value receiver ensures consistent JSON encoding
	// regardless of how Schema is stored (fixes golang/go#22967, golang/go#33993, golang/go#55890)

	// Create a test schema
	testSchema := Schema{
		Type:      "object",
		MinLength: Ptr(10),
		Properties: map[string]*Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}

	// Expected JSON output
	expectedJSON, err := json.Marshal(testSchema)
	if err != nil {
		t.Fatalf("Failed to marshal expected schema: %v", err)
	}

	if !strings.Contains(string(expectedJSON), "object") {
		t.Fatalf("Expected JSON does not contain 'object': %s", string(expectedJSON))
	}

	t.Run("DirectValue", func(t *testing.T) {
		// Test direct value marshaling
		got, err := json.Marshal(testSchema)
		if err != nil {
			t.Fatalf("Failed to marshal direct value: %v", err)
		}
		if string(got) != string(expectedJSON) {
			t.Errorf("Direct value marshaling mismatch\ngot:  %s\nwant: %s", got, expectedJSON)
		}
	})

	t.Run("Pointer", func(t *testing.T) {
		// Test pointer marshaling
		schemaPtr := &testSchema
		got, err := json.Marshal(schemaPtr)
		if err != nil {
			t.Fatalf("Failed to marshal pointer: %v", err)
		}
		if string(got) != string(expectedJSON) {
			t.Errorf("Pointer marshaling mismatch\ngot:  %s\nwant: %s", got, expectedJSON)
		}
	})

	t.Run("MapValue", func(t *testing.T) {
		// Test marshaling when stored as map value (non-addressable)
		// This is a key case that fails with pointer receiver
		schemaMap := map[string]Schema{
			"test": testSchema,
		}
		got, err := json.Marshal(schemaMap["test"])
		if err != nil {
			t.Fatalf("Failed to marshal map value: %v", err)
		}
		if string(got) != string(expectedJSON) {
			t.Errorf("Map value marshaling mismatch\ngot:  %s\nwant: %s", got, expectedJSON)
		}
	})

	t.Run("MapWithSchemaValues", func(t *testing.T) {
		// Test marshaling a map containing Schema values
		schemaMap := map[string]Schema{
			"schema1": testSchema,
			"schema2": {Type: "string"},
		}
		got, err := json.Marshal(schemaMap)
		if err != nil {
			t.Fatalf("Failed to marshal map with Schema values: %v", err)
		}

		// Verify the map marshals correctly
		var result map[string]json.RawMessage
		if err := json.Unmarshal(got, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		// Check that schema1 matches expected
		if string(result["schema1"]) != string(expectedJSON) {
			t.Errorf("Map schema1 marshaling mismatch\ngot:  %s\nwant: %s", result["schema1"], expectedJSON)
		}
	})

	t.Run("SliceElement", func(t *testing.T) {
		// Test marshaling when stored in a slice
		schemas := []Schema{testSchema}
		gotSlice, err := json.Marshal(schemas)
		if err != nil {
			t.Fatalf("Failed to marshal slice: %v", err)
		}

		var unmarshaledSlice []json.RawMessage
		if err := json.Unmarshal(gotSlice, &unmarshaledSlice); err != nil {
			t.Fatalf("Failed to unmarshal slice: %v", err)
		}

		if len(unmarshaledSlice) != 1 || string(unmarshaledSlice[0]) != string(expectedJSON) {
			t.Errorf("Slice element marshaling mismatch\ngot:  %s\nwant: %s",
				unmarshaledSlice[0], expectedJSON)
		}
	})

	t.Run("StructField", func(t *testing.T) {
		// Test marshaling when embedded in another struct
		type Container struct {
			Schema Schema `json:"schema"`
			Name   string `json:"name"`
		}

		container := Container{
			Schema: testSchema,
			Name:   "test",
		}

		got, err := json.Marshal(container)
		if err != nil {
			t.Fatalf("Failed to marshal struct with Schema field: %v", err)
		}

		var result map[string]json.RawMessage
		if err := json.Unmarshal(got, &result); err != nil {
			t.Fatalf("Failed to unmarshal struct result: %v", err)
		}

		if string(result["schema"]) != string(expectedJSON) {
			t.Errorf("Struct field marshaling mismatch\ngot:  %s\nwant: %s",
				result["schema"], expectedJSON)
		}
	})

	t.Run("InterfaceValue", func(t *testing.T) {
		// Test marshaling when stored as interface{}
		var iface any = testSchema
		got, err := json.Marshal(iface)
		if err != nil {
			t.Fatalf("Failed to marshal interface value: %v", err)
		}
		if string(got) != string(expectedJSON) {
			t.Errorf("Interface value marshaling mismatch\ngot:  %s\nwant: %s", got, expectedJSON)
		}
	})

	t.Run("EmptyPropertiesMap", func(t *testing.T) {
		// Test that an empty map in Properties marshals as "{}".
		s := &Schema{Type: "object", Properties: map[string]*Schema{}}
		got, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("Failed to marshal interface value: %v", err)
		}
		want := `{"type":"object","properties":{}}`
		if string(got) != want {
			t.Errorf("\ngot  %s\nwant %s", got, want)
		}
	})
}

func TestGoRoundTrip(t *testing.T) {
	// Verify that Go representations round-trip.
	for _, s := range []*Schema{
		{Type: "null"},
		{Types: []string{"null", "number"}},
		{Type: "string", MinLength: Ptr(20)},
		{Minimum: Ptr(20.0)},
		{Items: &Schema{Type: "integer"}},
		{Const: Ptr(any(0))},
		{Const: Ptr(any(nil))},
		{Const: Ptr(any([]int{}))},
		{Const: Ptr(any(map[string]any{}))},
		{Default: mustMarshal(1)},
		{Default: mustMarshal(nil)},
		{Extra: map[string]any{"test": "value"}},
	} {
		data, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var got *Schema
		mustUnmarshal(t, data, &got)
		if !Equal(got, s) {
			t.Errorf("got %s, want %s", got.json(), s.json())
			if got.Const != nil && s.Const != nil {
				t.Logf("Consts: got %#v (%[1]T), want %#v (%[2]T)", *got.Const, *s.Const)
			}
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	// Verify that JSON texts for schemas marshal into equivalent forms.
	// We don't expect everything to round-trip perfectly. For example, "true" and "false"
	// will turn into their object equivalents.
	// But most things should.
	// Some of these cases test Schema.{UnM,M}arshalJSON.
	// Most of others follow from the behavior of encoding/json, but they are still
	// valuable as regression tests of this package's behavior.
	for _, tt := range []struct {
		in, want string
	}{
		{`true`, `true`},
		{`false`, `false`},
		{`{"type":"", "enum":null}`, `true`}, // empty fields are omitted
		{`{"minimum":1}`, `{"minimum":1}`},
		{`{"minimum":1.0}`, `{"minimum":1}`},     // floating-point integers lose their fractional part
		{`{"minLength":1.0}`, `{"minLength":1}`}, // some floats are unmarshaled into ints, but you can't tell
		{
			// map keys are sorted
			`{"$vocabulary":{"b":true, "a":false}}`,
			`{"$vocabulary":{"a":false,"b":true}}`,
		},
		{`{"unk":0}`, `{"unk":0}`}, // unknown fields are not dropped
		{
			// known and unknown fields are not dropped
			// note that the order will be by the declaration order in the anonymous struct inside MarshalJSON
			`{"comment":"test","type":"example","unk":0}`,
			`{"type":"example","comment":"test","unk":0}`,
		},
		{`{"extra":0}`, `{"extra":0}`}, // extra is not a special keyword and should not be dropped
		{`{"Extra":0}`, `{"Extra":0}`}, // Extra is not a special keyword and should not be dropped
	} {
		var s Schema
		mustUnmarshal(t, []byte(tt.in), &s)
		data, err := json.Marshal(&s)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(data); got != tt.want {
			t.Errorf("%s:\ngot  %s\nwant %s", tt.in, got, tt.want)
		}
	}
}

func TestUnmarshalErrors(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want string // error must match this regexp
	}{
		{`1`, "cannot unmarshal number"},
		{`{"type":1}`, `invalid value for "type"`},
		{`{"minLength":1.5}`, `not an integer value`},
		{`{"maxLength":1.5}`, `not an integer value`},
		{`{"minItems":1.5}`, `not an integer value`},
		{`{"maxItems":1.5}`, `not an integer value`},
		{`{"minProperties":1.5}`, `not an integer value`},
		{`{"maxProperties":1.5}`, `not an integer value`},
		{`{"minContains":1.5}`, `not an integer value`},
		{`{"maxContains":1.5}`, `not an integer value`},
		{fmt.Sprintf(`{"maxContains":%d}`, int64(math.MaxInt32+1)), `out of range`},
		{`{"minLength":9e99}`, `cannot be unmarshaled`},
		{`{"minLength":"1.5"}`, `not a number`},
	} {
		var s Schema
		err := json.Unmarshal([]byte(tt.in), &s)
		if err == nil {
			t.Fatalf("%s: no error but expected one", tt.in)
		}
		if !regexp.MustCompile(tt.want).MatchString(err.Error()) {
			t.Errorf("%s: error %q does not match %q", tt.in, err, tt.want)
		}

	}
}

func TestMarshalOrder(t *testing.T) {
	for _, tt := range []struct {
		order      []string
		want       string
		wantErr    bool
		errMessage string
	}{
		{
			[]string{"A", "B", "C", "D"},
			`{"type":"object","properties":{"A":{"type":"integer"},"B":{"type":"integer"},"C":{"type":"integer"},"D":{"type":"integer"},"E":{"type":"integer"}}}`,
			false,
			"",
		},
		{
			[]string{"A", "C", "B", "D"},
			`{"type":"object","properties":{"A":{"type":"integer"},"C":{"type":"integer"},"B":{"type":"integer"},"D":{"type":"integer"},"E":{"type":"integer"}}}`,
			false,
			"",
		},
		{
			[]string{"D", "C", "B", "A"},
			`{"type":"object","properties":{"D":{"type":"integer"},"C":{"type":"integer"},"B":{"type":"integer"},"A":{"type":"integer"},"E":{"type":"integer"}}}`,
			false,
			"",
		},
		{
			[]string{"A", "B", "C"},
			`{"type":"object","properties":{"A":{"type":"integer"},"B":{"type":"integer"},"C":{"type":"integer"},"D":{"type":"integer"},"E":{"type":"integer"}}}`,
			false,
			"",
		},
		{
			[]string{"A", "E", "C"},
			`{"type":"object","properties":{"A":{"type":"integer"},"E":{"type":"integer"},"C":{"type":"integer"},"B":{"type":"integer"},"D":{"type":"integer"}}}`,
			false,
			"",
		},
		{
			[]string{"A", "B", "C", "D", "D"},
			"",
			true,
			"json: error calling MarshalJSON for type *jsonschema.Schema: property order slice cannot contain duplicate entries, found duplicate \"D\"",
		},
	} {
		s := &Schema{
			Type: "object",
			Properties: map[string]*Schema{
				"A": {Type: "integer"},
				"B": {Type: "integer"},
				"C": {Type: "integer"},
				"D": {Type: "integer"},
				"E": {Type: "integer"},
			},
		}
		s.PropertyOrder = tt.order
		gotBytes, err := json.Marshal(s)
		if err != nil {
			if !tt.wantErr {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.errMessage, err.Error()); diff != "" {
				t.Fatalf("error message mismatch (-want +got):\n%s", diff)
			}
			continue
		}
		if diff := cmp.Diff(tt.want, string(gotBytes), cmpopts.IgnoreUnexported(Schema{})); diff != "" {
			t.Fatalf("ForType mismatch (-want +got):\n%s", diff)
		}
	}
}

func mustUnmarshal(t *testing.T, data []byte, ptr any) {
	t.Helper()
	if err := json.Unmarshal(data, ptr); err != nil {
		t.Fatal(err)
	}
}

// json returns the schema in json format.
func (s *Schema) json() string {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Sprintf("<jsonschema.Schema:%v>", err)
	}
	return string(data)
}

// json returns the schema in json format, indented.
func (s *Schema) jsonIndent() string {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Sprintf("<jsonschema.Schema:%v>", err)
	}
	return string(data)
}

func TestCloneSchemas(t *testing.T) {
	ss1 := &Schema{Type: "string"}
	ss2 := &Schema{Type: "integer"}
	ss3 := &Schema{Type: "boolean"}
	ss4 := &Schema{Type: "number"}
	ss5 := &Schema{Contains: ss4}

	s1 := Schema{
		Contains:    ss1,
		PrefixItems: []*Schema{ss2, ss3},
		Properties:  map[string]*Schema{"a": ss5},
	}
	s2 := s1.CloneSchemas()

	// The clones should appear identical.
	if g, w := s1.json(), s2.json(); g != w {
		t.Errorf("\ngot  %s\nwant %s", g, w)
	}
	// None of the schemas should overlap.
	schemas1 := map[*Schema]bool{ss1: true, ss2: true, ss3: true, ss4: true, ss5: true}
	for ss := range s2.all() {
		if schemas1[ss] {
			t.Errorf("uncloned schema %s", ss.json())
		}
	}
	// s1's original schemas should be intact.
	if s1.Contains != ss1 || s1.PrefixItems[0] != ss2 || s1.PrefixItems[1] != ss3 || ss5.Contains != ss4 || s1.Properties["a"] != ss5 {
		t.Errorf("s1 modified")
	}
}

func TestCloneMarshalDraft2020_12(t *testing.T) {
	files, err := filepath.Glob(filepath.FromSlash("testdata/draft2020-12/*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no files")
	}
	testCloneMarshalValidate(t, files, "")
}

func TestCloneMarshalDraft7(t *testing.T) {
	files, err := filepath.Glob(filepath.FromSlash("testdata/draft7/*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no files")
	}
	testCloneMarshalValidate(t, files, "https://json-schema.org/draft-07/schema#")
}

func testCloneMarshalValidate(t *testing.T, files []string, draft string) {
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
					original := g.Schema.json()
					clone := g.Schema.CloneSchemas()
					if diff := cmp.Diff(original, clone.json()); diff != "" {
						t.Fatalf("Schema mismatch (-want +got):\n%s", diff)
					}
				})
			}
		})
	}
}
