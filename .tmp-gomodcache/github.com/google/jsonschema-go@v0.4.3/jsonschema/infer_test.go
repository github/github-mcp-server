// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jsonschema_test

import (
	"encoding/json"
	"log/slog"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/jsonschema-go/jsonschema"
)

type custom int

func (c custom) MarshalText() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(c), 10)), nil
}

func forType[T any](ignore bool) *jsonschema.Schema {
	var s *jsonschema.Schema
	var err error

	opts := &jsonschema.ForOptions{
		IgnoreInvalidTypes: ignore,
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[custom](): {Type: "custom"},
		},
	}
	s, err = jsonschema.For[T](opts)
	if err != nil {
		panic(err)
	}
	return s
}

func TestFor(t *testing.T) {
	type schema = jsonschema.Schema

	type S struct {
		B int `jsonschema:"bdesc"`
	}

	type test struct {
		name string
		got  *jsonschema.Schema
		want *jsonschema.Schema
	}

	f64Ptr := jsonschema.Ptr[float64]

	tests := func(ignore bool) []test {
		return []test{
			{"string", forType[string](ignore), &schema{Type: "string"}},
			{
				"int8",
				forType[int8](ignore),
				&schema{Type: "integer", Minimum: f64Ptr(math.MinInt8), Maximum: f64Ptr(math.MaxInt8)},
			},
			{
				"uint8",
				forType[uint8](ignore),
				&schema{Type: "integer", Minimum: f64Ptr(0), Maximum: f64Ptr(math.MaxUint8)},
			},
			{
				"int16",
				forType[int16](ignore),
				&schema{Type: "integer", Minimum: f64Ptr(math.MinInt16), Maximum: f64Ptr(math.MaxInt16)},
			},
			{
				"uint16",
				forType[uint16](ignore),
				&schema{Type: "integer", Minimum: f64Ptr(0), Maximum: f64Ptr(math.MaxUint16)},
			},
			{
				"int32",
				forType[int32](ignore),
				&schema{Type: "integer", Minimum: f64Ptr(math.MinInt32), Maximum: f64Ptr(math.MaxInt32)},
			},
			{
				"uint32",
				forType[uint32](ignore),
				&schema{Type: "integer", Minimum: f64Ptr(0), Maximum: f64Ptr(math.MaxUint32)},
			},
			{"int64", forType[int64](ignore), &schema{Type: "integer"}},
			{"uint64", forType[uint64](ignore), &schema{Type: "integer", Minimum: f64Ptr(0)}},
			{"int", forType[int](ignore), &schema{Type: "integer"}},
			{"uint", forType[uint](ignore), &schema{Type: "integer", Minimum: f64Ptr(0)}},
			{"uintptr", forType[uintptr](ignore), &schema{Type: "integer", Minimum: f64Ptr(0)}},
			{"float64", forType[float64](ignore), &schema{Type: "number"}},
			{"bool", forType[bool](ignore), &schema{Type: "boolean"}},
			{"time", forType[time.Time](ignore), &schema{Type: "string"}},
			{"level", forType[slog.Level](ignore), &schema{Type: "string"}},
			{"bigint", forType[big.Int](ignore), &schema{Type: "string"}},
			{"bigint", forType[*big.Int](ignore), &schema{Types: []string{"null", "string"}}},
			{"int64slice", forType[[]int64](ignore), &schema{Types: []string{"null", "array"}, Items: &schema{Type: "integer"}}},
			{"int64array", forType[[2]int64](ignore), &schema{Type: "array", Items: &schema{Type: "integer"}, MinItems: jsonschema.Ptr(2), MaxItems: jsonschema.Ptr(2)}},
			{"custom", forType[custom](ignore), &schema{Type: "custom"}},
			{"intmap", forType[map[string]int](ignore), &schema{
				Type:                 "object",
				AdditionalProperties: &schema{Type: "integer"},
			}},
			{"int8map", forType[map[string]int8](ignore), &schema{
				Type:                 "object",
				AdditionalProperties: &schema{Type: "integer", Minimum: f64Ptr(math.MinInt8), Maximum: f64Ptr(math.MaxInt8)},
			}},
			{"anymap", forType[map[string]any](ignore), &schema{
				Type:                 "object",
				AdditionalProperties: &schema{},
			}},
			{"custommap", forType[map[custom]string](ignore), &schema{
				Type:                 "object",
				AdditionalProperties: &schema{Type: "string"},
			}},
			{
				"struct",
				forType[struct {
					F           int `json:"f" jsonschema:"fdesc"`
					G           []float64
					P           *bool `jsonschema:"pdesc"`
					PT          *time.Time
					Skip        string `json:"-"`
					NoSkip      string `json:",omitempty"`
					unexported  float64
					unexported2 int `json:"No"`
				}](ignore),
				&schema{
					Type: "object",
					Properties: map[string]*schema{
						"f":      {Type: "integer", Description: "fdesc"},
						"G":      {Types: []string{"null", "array"}, Items: &schema{Type: "number"}},
						"P":      {Types: []string{"null", "boolean"}, Description: "pdesc"},
						"PT":     {Types: []string{"null", "string"}},
						"NoSkip": {Type: "string"},
					},
					Required:             []string{"f", "G", "P", "PT"},
					AdditionalProperties: falseSchema(),
					PropertyOrder:        []string{"f", "G", "P", "PT", "NoSkip"},
				},
			},
			{
				"no sharing",
				forType[struct{ X, Y int }](ignore),
				&schema{
					Type: "object",
					Properties: map[string]*schema{
						"X": {Type: "integer"},
						"Y": {Type: "integer"},
					},
					Required:             []string{"X", "Y"},
					AdditionalProperties: falseSchema(),
					PropertyOrder:        []string{"X", "Y"},
				},
			},
			{
				"nested and embedded",
				forType[struct {
					A S
					S
				}](ignore),
				&schema{
					Type: "object",
					Properties: map[string]*schema{
						"A": {
							Type: "object",
							Properties: map[string]*schema{
								"B": {Type: "integer", Description: "bdesc"},
							},
							Required:             []string{"B"},
							AdditionalProperties: falseSchema(),
							PropertyOrder:        []string{"B"},
						},
						"B": {
							Type:        "integer",
							Description: "bdesc",
						},
					},
					Required:             []string{"A", "B"},
					AdditionalProperties: falseSchema(),
					PropertyOrder:        []string{"A", "B"},
				},
			},
		}
	}
	run := func(t *testing.T, tt test) {
		if diff := cmp.Diff(tt.want, tt.got, cmpopts.IgnoreUnexported(jsonschema.Schema{})); diff != "" {
			t.Fatalf("For mismatch (-want +got):\n%s", diff)
		}
		// These schemas should all resolve.
		if _, err := tt.got.Resolve(nil); err != nil {
			t.Fatalf("Resolving: %v", err)
		}
	}

	t.Run("strict", func(t *testing.T) {
		for _, test := range tests(false) {
			t.Run(test.name, func(t *testing.T) { run(t, test) })
		}
	})

	laxTests := append(tests(true), test{
		"ignore",
		forType[struct {
			A int
			B map[int]int
			C func()
		}](true),
		&schema{
			Type: "object",
			Properties: map[string]*schema{
				"A": {Type: "integer"},
			},
			Required:             []string{"A"},
			AdditionalProperties: falseSchema(),
			PropertyOrder:        []string{"A"},
		},
	})
	t.Run("lax", func(t *testing.T) {
		for _, test := range laxTests {
			t.Run(test.name, func(t *testing.T) { run(t, test) })
		}
	})
}

func TestForType(t *testing.T) {
	// This tests embedded structs with a custom schema in addition to ForType.
	type schema = jsonschema.Schema

	type E struct {
		G float64 // promoted into S
		B int     // hidden by S.B
	}

	type M1 int
	type M2 int

	type S struct {
		I  int
		F  func()
		C  custom
		P  *custom
		PP **custom
		E
		B   bool
		M1  M1
		PM1 *M1
		M2  M2
		PM2 *M2
	}

	opts := &jsonschema.ForOptions{
		IgnoreInvalidTypes: true,
		TypeSchemas: map[reflect.Type]*schema{
			reflect.TypeFor[custom](): {Type: "custom"},
			reflect.TypeFor[E](): {
				Type: "object",
				Properties: map[string]*schema{
					"G": {Type: "integer"},
					"B": {Type: "integer"},
				},
			},
			reflect.TypeFor[M1](): {Types: []string{"custom1", "custom2"}},
			reflect.TypeFor[M2](): {Types: []string{"null", "custom3", "custom4"}},
		},
	}
	got, err := jsonschema.ForType(reflect.TypeOf(S{}), opts)
	if err != nil {
		t.Fatal(err)
	}
	want := &schema{
		Type: "object",
		Properties: map[string]*schema{
			"I":   {Type: "integer"},
			"C":   {Type: "custom"},
			"P":   {Types: []string{"null", "custom"}},
			"PP":  {Types: []string{"null", "custom"}},
			"G":   {Type: "integer"},
			"B":   {Type: "boolean"},
			"M1":  {Types: []string{"custom1", "custom2"}},
			"PM1": {Types: []string{"null", "custom1", "custom2"}},
			"M2":  {Types: []string{"null", "custom3", "custom4"}},
			"PM2": {Types: []string{"null", "custom3", "custom4"}},
		},
		Required:             []string{"I", "C", "P", "PP", "B", "M1", "PM1", "M2", "PM2"},
		AdditionalProperties: falseSchema(),
		PropertyOrder:        []string{"I", "C", "P", "PP", "G", "B", "M1", "PM1", "M2", "PM2"},
	}
	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(schema{})); diff != "" {
		t.Fatalf("ForType mismatch (-want +got):\n%s", diff)
	}

	gotBytes, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	wantStr := `{"type":"object","properties":{"I":{"type":"integer"},"C":{"type":"custom"},"P":{"type":["null","custom"]},"PP":{"type":["null","custom"]},"G":{"type":"integer"}` +
		`,"B":{"type":"boolean"},"M1":{"type":["custom1","custom2"]},"PM1":{"type":["null","custom1","custom2"]},"M2":{"type":["null","custom3","custom4"]},` +
		`"PM2":{"type":["null","custom3","custom4"]}},"required":["I","C","P","PP","B","M1","PM1","M2","PM2"],"additionalProperties":false}`
	if diff := cmp.Diff(wantStr, string(gotBytes), cmpopts.IgnoreUnexported(schema{})); diff != "" {
		t.Fatalf("ForType mismatch (-want +got):\n%s", diff)
	}
}

func TestForTypeWithDifferentOrder(t *testing.T) {
	// This tests embedded structs with a custom schema in addition to ForType.
	type schema = jsonschema.Schema

	type E struct {
		G float64 // promoted into S
		B int     // hidden by S.B
	}

	type S struct {
		I int
		F func()
		C custom
		B bool
		E
	}

	opts := &jsonschema.ForOptions{
		IgnoreInvalidTypes: true,
		TypeSchemas: map[reflect.Type]*schema{
			reflect.TypeFor[custom](): {Type: "custom"},
			reflect.TypeFor[E](): {
				Type: "object",
				Properties: map[string]*schema{
					"G": {Type: "integer"},
					"B": {Type: "integer"},
				},
			},
		},
	}
	got, err := jsonschema.ForType(reflect.TypeOf(S{}), opts)
	if err != nil {
		t.Fatal(err)
	}
	want := &schema{
		Type: "object",
		Properties: map[string]*schema{
			"I": {Type: "integer"},
			"C": {Type: "custom"},
			"G": {Type: "integer"},
			"B": {Type: "boolean"},
		},
		Required:             []string{"I", "C", "B"},
		AdditionalProperties: falseSchema(),
		PropertyOrder:        []string{"I", "C", "B", "G"},
	}
	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(schema{})); diff != "" {
		t.Fatalf("ForType mismatch (-want +got):\n%s", diff)
	}

	gotBytes, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	wantStr := `{"type":"object","properties":{"I":{"type":"integer"},"C":{"type":"custom"},"B":{"type":"boolean"},"G":{"type":"integer"}},"required":["I","C","B"],"additionalProperties":false}`
	if diff := cmp.Diff(wantStr, string(gotBytes), cmpopts.IgnoreUnexported(schema{})); diff != "" {
		t.Fatalf("ForType mismatch (-want +got):\n%s", diff)
	}
}

func TestForTypeWithEmbeddedStruct(t *testing.T) {
	// This tests embedded structs with a custom schema in addition to ForType.
	type schema = jsonschema.Schema

	type E struct {
		G float64 // promoted into S
		B int     // promoted into S
		I int     // promoted into S
	}

	type S struct {
		F func()
		C custom
		E
	}

	type S1 struct {
		F func()
		C custom
		E
		M int
	}

	type test struct {
		name        string
		convertType reflect.Type
		opts        *jsonschema.ForOptions
		want        *jsonschema.Schema
		wantStr     string
	}

	tests := []test{
		{
			name:        "Embedded without override",
			convertType: reflect.TypeOf(S{}),
			opts: &jsonschema.ForOptions{
				IgnoreInvalidTypes: true,
				TypeSchemas: map[reflect.Type]*schema{
					reflect.TypeFor[custom](): {Type: "custom"},
				},
			},
			want: &schema{
				Type: "object",
				Properties: map[string]*schema{
					"C": {Type: "custom"},
					"G": {Type: "number"},
					"B": {Type: "integer"},
					"I": {Type: "integer"},
				},
				Required:             []string{"C", "G", "B", "I"},
				AdditionalProperties: falseSchema(),
				PropertyOrder:        []string{"C", "G", "B", "I"},
			},
			wantStr: `{"type":"object","properties":{"C":{"type":"custom"},"G":{"type":"number"},"B":{"type":"integer"},"I":{"type":"integer"}},"required":["C","G","B","I"],"additionalProperties":false}`,
		},
		{
			name:        "Embedded with overwrite",
			convertType: reflect.TypeOf(S{}),
			opts: &jsonschema.ForOptions{
				IgnoreInvalidTypes: true,
				TypeSchemas: map[reflect.Type]*schema{
					reflect.TypeFor[custom](): {Type: "custom"},
					reflect.TypeFor[E](): {
						Type: "object",
						Properties: map[string]*schema{
							"G": {Type: "integer"},
							"B": {Type: "integer"},
							"I": {Type: "integer"},
						},
					},
				},
			},
			want: &schema{
				Type: "object",
				Properties: map[string]*schema{
					"C": {Type: "custom"},
					"G": {Type: "integer"},
					"B": {Type: "integer"},
					"I": {Type: "integer"},
				},
				Required:             []string{"C"},
				AdditionalProperties: falseSchema(),
				PropertyOrder:        []string{"C", "B", "G", "I"},
			},
			wantStr: `{"type":"object","properties":{"C":{"type":"custom"},"B":{"type":"integer"},"G":{"type":"integer"},"I":{"type":"integer"}},"required":["C"],"additionalProperties":false}`,
		},
		{
			name:        "Embedded in the middle without overwrite",
			convertType: reflect.TypeOf(S1{}),
			opts: &jsonschema.ForOptions{
				IgnoreInvalidTypes: true,
				TypeSchemas: map[reflect.Type]*schema{
					reflect.TypeFor[custom](): {Type: "custom"},
				},
			},
			want: &schema{
				Type: "object",
				Properties: map[string]*schema{
					"C": {Type: "custom"},
					"G": {Type: "number"},
					"B": {Type: "integer"},
					"I": {Type: "integer"},
					"M": {Type: "integer"},
				},
				Required:             []string{"C", "G", "B", "I", "M"},
				AdditionalProperties: falseSchema(),
				PropertyOrder:        []string{"C", "G", "B", "I", "M"},
			},
			wantStr: `{"type":"object","properties":{"C":{"type":"custom"},"G":{"type":"number"},"B":{"type":"integer"},"I":{"type":"integer"},"M":{"type":"integer"}},"required":["C","G","B","I","M"],"additionalProperties":false}`,
		},
		{
			name:        "Embedded in the middle with overwrite",
			convertType: reflect.TypeOf(S1{}),
			opts: &jsonschema.ForOptions{
				IgnoreInvalidTypes: true,
				TypeSchemas: map[reflect.Type]*schema{
					reflect.TypeFor[custom](): {Type: "custom"},
					reflect.TypeFor[E](): {
						Type: "object",
						Properties: map[string]*schema{
							"G": {Type: "integer"},
							"B": {Type: "integer"},
							"I": {Type: "integer"},
						},
					},
				},
			},
			want: &schema{
				Type: "object",
				Properties: map[string]*schema{
					"C": {Type: "custom"},
					"G": {Type: "integer"},
					"B": {Type: "integer"},
					"I": {Type: "integer"},
					"M": {Type: "integer"},
				},
				Required:             []string{"C", "M"},
				AdditionalProperties: falseSchema(),
				PropertyOrder:        []string{"C", "B", "G", "I", "M"},
			},
			wantStr: `{"type":"object","properties":{"C":{"type":"custom"},"B":{"type":"integer"},"G":{"type":"integer"},"I":{"type":"integer"},"M":{"type":"integer"}},"required":["C","M"],"additionalProperties":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonschema.ForType(tt.convertType, tt.opts)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreUnexported(schema{})); diff != "" {
				t.Fatalf("ForType mismatch (-want +got):\n%s", diff)
			}
			gotBytes, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.wantStr, string(gotBytes), cmpopts.IgnoreUnexported(schema{})); diff != "" {
				t.Fatalf("ForType mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCustomEmbeddedError(t *testing.T) {
	// Disallow anything but "type" and "properties".
	type schema = jsonschema.Schema

	type (
		E struct{ G int }
		S struct{ E }
	)

	for _, tt := range []struct {
		name     string
		override *schema
	}{
		{
			"missing type",
			&schema{},
		},
		{
			"wrong type",
			&schema{Type: "number"},
		},
		{
			"extra string field",
			&schema{
				Type:  "object",
				Title: "t",
			},
		},
		{
			"extra pointer field",
			&schema{
				Type:          "object",
				MinProperties: jsonschema.Ptr(1),
			},
		},
		{
			"extra array field",
			&schema{
				Type:     "object",
				Required: []string{"G"},
			},
		},
		{
			"extra schema field",
			&schema{
				Type:                 "object",
				AdditionalProperties: falseSchema(),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts := &jsonschema.ForOptions{
				TypeSchemas: map[reflect.Type]*schema{
					reflect.TypeFor[E](): tt.override,
				},
			}
			if _, err := jsonschema.ForType(reflect.TypeOf(S{}), opts); err == nil {
				t.Error("got nil, want error")
			}
		})
	}
}

func forErr[T any]() error {
	_, err := jsonschema.For[T](nil)
	return err
}

func TestForErrors(t *testing.T) {
	type (
		s1 struct {
			Empty int `jsonschema:""`
		}
		s2 struct {
			Bad int `jsonschema:"$foo=1,bar"`
		}
	)

	for _, tt := range []struct {
		got  error
		want string
	}{
		{forErr[map[int]int](), "unsupported map key type"},
		{forErr[s1](), "empty jsonschema tag"},
		{forErr[s2](), "must not begin with"},
		{forErr[func()](), "unsupported"},
	} {
		if tt.got == nil {
			t.Errorf("got nil, want error containing %q", tt.want)
		} else if !strings.Contains(tt.got.Error(), tt.want) {
			t.Errorf("got %q\nwant it to contain %q", tt.got, tt.want)
		}
	}
}

func TestForWithMutation(t *testing.T) {
	// This test ensures that the cached schema is not mutated when the caller
	// mutates the returned schema.
	type S struct {
		A int
	}
	type T struct {
		A int `json:"A"`
		B map[string]int
		C []S
		D [3]S
		E *bool
	}
	s, err := jsonschema.For[T](nil)
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	s.Required[0] = "mutated"
	s.Properties["A"].Type = "mutated"
	s.Properties["C"].Items.Type = "mutated"
	s.Properties["D"].MaxItems = jsonschema.Ptr(10)
	s.Properties["D"].MinItems = jsonschema.Ptr(10)
	s.Properties["E"].Types[0] = "mutated"

	s2, err := jsonschema.For[T](nil)
	if err != nil {
		t.Fatalf("For: %v", err)
	}
	if s2.Properties["A"].Type == "mutated" {
		t.Fatalf("ForWithMutation: expected A.Type to not be mutated")
	}
	if s2.Properties["B"].AdditionalProperties.Type == "mutated" {
		t.Fatalf("ForWithMutation: expected B.AdditionalProperties.Type to not be mutated")
	}
	if s2.Properties["C"].Items.Type == "mutated" {
		t.Fatalf("ForWithMutation: expected C.Items.Type to not be mutated")
	}
	if *s2.Properties["D"].MaxItems == 10 {
		t.Fatalf("ForWithMutation: expected D.MaxItems to not be mutated")
	}
	if *s2.Properties["D"].MinItems == 10 {
		t.Fatalf("ForWithMutation: expected D.MinItems to not be mutated")
	}
	if s2.Properties["E"].Types[0] == "mutated" {
		t.Fatalf("ForWithMutation: expected E.Types[0] to not be mutated")
	}
	if s2.Required[0] == "mutated" {
		t.Fatalf("ForWithMutation: expected Required[0] to not be mutated")
	}
}

type x struct {
	Y y
}
type y struct {
	X []x
}

func TestForWithCycle(t *testing.T) {
	type a []*a
	type b1 struct{ b *b1 } // unexported field should be skipped
	type b2 struct{ B *b2 }
	type c1 struct{ c map[string]*c1 } // unexported field should be skipped
	type c2 struct{ C map[string]*c2 }

	tests := []struct {
		name      string
		shouldErr bool
		fn        func() error
	}{
		{"slice alias (a)", true, func() error { _, err := jsonschema.For[a](nil); return err }},
		{"unexported self cycle (b1)", false, func() error { _, err := jsonschema.For[b1](nil); return err }},
		{"exported self cycle (b2)", true, func() error { _, err := jsonschema.For[b2](nil); return err }},
		{"unexported map self cycle (c1)", false, func() error { _, err := jsonschema.For[c1](nil); return err }},
		{"exported map self cycle (c2)", true, func() error { _, err := jsonschema.For[c2](nil); return err }},
		{"cross-cycle x -> y -> x", true, func() error { _, err := jsonschema.For[x](nil); return err }},
		{"cross-cycle y -> x -> y", true, func() error { _, err := jsonschema.For[y](nil); return err }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.fn()
			if test.shouldErr && err == nil {
				t.Errorf("expected cycle error, got nil")
			}
			if !test.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func falseSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Not: &jsonschema.Schema{}}
}

func TestDupSchema(t *testing.T) {
	// Verify that we don't repeat schema contents, even if we clone the actual schemas.
	type args struct {
		S string   `jsonschema:"str"`
		A []string `jsonschema:"arr"`
	}

	s := forType[args](false)
	if g, w := s.Properties["S"].Description, "str"; g != w {
		t.Errorf("S: got %q, want %q", g, w)
	}
	if g, w := s.Properties["A"].Description, "arr"; g != w {
		t.Errorf("A: got %q, want %q", g, w)
	}
	if g, w := s.Properties["A"].Items.Description, ""; g != w {
		t.Errorf("A.items: got %q, want %q", g, w)
	}
}
