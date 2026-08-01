// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"strings"
	"testing"
)

func TestDereferenceJSONPointer(t *testing.T) {
	s := &Schema{
		AllOf: []*Schema{{}, {}},
		Defs: map[string]*Schema{
			"":  {Properties: map[string]*Schema{"": {}}},
			"A": {},
			"B": {
				Defs: map[string]*Schema{
					"X": {},
					"Y": {},
				},
			},
			"/~": {},
			"~1": {},
		},
	}

	for _, tt := range []struct {
		ptr  string
		want any
	}{
		{"", s},
		{"/$defs/A", s.Defs["A"]},
		{"/$defs/B", s.Defs["B"]},
		{"/$defs/B/$defs/X", s.Defs["B"].Defs["X"]},
		{"/$defs//properties/", s.Defs[""].Properties[""]},
		{"/allOf/1", s.AllOf[1]},
		{"/$defs/~1~0", s.Defs["/~"]},
		{"/$defs/~01", s.Defs["~1"]},
	} {
		got, err := dereferenceJSONPointer(s, tt.ptr)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("%s:\ngot  %+v\nwant %+v", tt.ptr, got, tt.want)
		}
	}
}

func TestDereferenceJSONPointerDraft7(t *testing.T) {
	s := &Schema{
		AllOf: []*Schema{{}, {}},
		Definitions: map[string]*Schema{
			"":  {Properties: map[string]*Schema{"": {}}},
			"A": {},
			"B": {
				Definitions: map[string]*Schema{
					"X": {},
					"Y": {},
				},
			},
			"/~": {},
			"~1": {},
		},
		ItemsArray: []*Schema{{}, {Items: &Schema{}}},
		DependencySchemas: map[string]*Schema{
			"A": {},
			"B": {
				Items: &Schema{},
			},
		},
		DependencyStrings: map[string][]string{
			"C": {"1", "2"},
		},
	}

	for _, tt := range []struct {
		ptr  string
		want any
	}{
		{"", s},
		{"/definitions/A", s.Definitions["A"]},
		{"/definitions/B", s.Definitions["B"]},
		{"/definitions/B/definitions/X", s.Definitions["B"].Definitions["X"]},
		{"/definitions//properties/", s.Definitions[""].Properties[""]},
		{"/allOf/1", s.AllOf[1]},
		{"/definitions/~1~0", s.Definitions["/~"]},
		{"/definitions/~01", s.Definitions["~1"]},
		{"/items/0", s.ItemsArray[0]},
		{"/items/1/items", s.ItemsArray[1].Items},
		{"/dependencies/A", s.DependencySchemas["A"]},
		{"/dependencies/B/items", s.DependencySchemas["B"].Items},
	} {
		got, err := dereferenceJSONPointer(s, tt.ptr)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("%s:\ngot  %+v\nwant %+v", tt.ptr, got, tt.want)
		}
	}
}

func TestDereferenceJSONPointerErrors(t *testing.T) {
	s := &Schema{
		Type:     "t",
		Items:    &Schema{},
		Required: []string{"a"},
		DependencySchemas: map[string]*Schema{
			"B": {},
		},
		DependencyStrings: map[string][]string{
			"A": {"1", "2"},
		},
	}
	for _, tt := range []struct {
		ptr  string
		want string // error must contain this string
	}{
		{"x", "does not begin"}, // parse error: no initial '/'
		{"/minItems", "does not refer to a schema"},
		{"/minItems/x", "navigated to nil"},
		{"/required/-", "not supported"},
		{"/required/01", "leading zeroes"},
		{"/required/x", "invalid int"},
		{"/required/1", "out of bounds"},
		{"/properties/x", "no key"},
		{"/dependencies/A", "no key \"A\" in map"},
	} {
		_, err := dereferenceJSONPointer(s, tt.ptr)
		if err == nil {
			t.Errorf("%q: succeeded, want failure", tt.ptr)
		} else if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: error is %q, which does not contain %q", tt.ptr, err, tt.want)
		}
	}
}
