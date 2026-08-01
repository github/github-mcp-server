// Copyright (C) 2016 Kohei YOSHIDA. All rights reserved.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of The BSD 3-Clause License
// that can be found in the LICENSE file.

package uritemplate

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func ExampleTemplate_Match() {
	tmpl := MustNew("https://example.com/dictionary/{term:1}/{term}")
	match := tmpl.Match("https://example.com/dictionary/c/cat")
	if match == nil {
		fmt.Println("not matched")
		return
	}

	fmt.Printf("term:1 is %q\n", match.Get("term:1").String())
	fmt.Printf("term is %q\n", match.Get("term").String())

	// Output:
	// term:1 is "c"
	// term is "cat"
}

func TestTemplate_Match(t *testing.T) {
	for i, c := range testTemplateCases {
		if c.failMatch {
			continue
		}

		tmpl, err := New(c.raw)
		if err != nil {
			t.Errorf("unexpected error on %q: %#v", c.raw, err)
			continue
		}

		match := tmpl.Match(c.expected)
		if match == nil {
			t.Errorf("%d: failed to match %q against %q", i, c.raw, c.expected)
			t.Log(tmpl.prog.String())
			continue
		}

		for name, actual := range match {
			var expected Value
			if semi := strings.Index(name, ":"); semi >= 0 {
				maxlen, _ := strconv.Atoi(name[semi+1:])
				name = name[:semi]
				expected = testExpressionExpandVarMap[name]

				if expected.T != ValueTypeString {
					t.Errorf("%d: failed to match %q against %q", i, c.raw, c.expected)
					t.Errorf("%d: expected %#v, but got %#v", i, expected, actual)
					continue
				}
				if v := expected.V[0]; len(v) > maxlen {
					expected.V = []string{v[:maxlen]}
				}
			} else {
				expected = testExpressionExpandVarMap[name]
			}

			if actual.T != expected.T {
				t.Errorf("%d: failed to match %q against %q", i, c.raw, c.expected)
				t.Errorf("%d: expected %#v, but got %#v", i, expected, actual)
			} else if le, la := len(expected.V), len(actual.V); le == la {
				for i := range actual.V {
					if actual.V[i] != expected.V[i] {
						t.Errorf("%d: failed to match %q against %q", i, c.raw, c.expected)
						t.Errorf("%d: expected %#v, but got %#v", i, expected, actual)
						break
					}
				}
			} else if !(le == 0 && la == 1 && actual.V[0] == "") { // not undef
				t.Errorf("%d: failed to match %q against %q", i, c.raw, c.expected)
				t.Errorf("%d: expected %#v, but got %#v", i, expected, actual)
			}
		}
	}
}

func TestTemplate_NotMatch(t *testing.T) {
	tmpl := MustNew("https://example.com/foo{?bar}")
	match := tmpl.Match("https://example.com/foobaz")
	if match != nil {
		t.Errorf("must not match")
	}
}
