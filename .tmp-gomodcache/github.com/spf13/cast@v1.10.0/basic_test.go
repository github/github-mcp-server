// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cast_test

import (
	"encoding/json"
	"html/template"
	"testing"
	"time"

	"github.com/spf13/cast"
)

func TestBool(t *testing.T) {
	var ptr *bool

	testCases := []testCase{
		{0, false, false},
		{int(0), false, false},
		{int8(0), false, false},
		{int16(0), false, false},
		{int32(0), false, false},
		{int64(0), false, false},
		{uint(0), false, false},
		{uint8(0), false, false},
		{uint16(0), false, false},
		{uint32(0), false, false},
		{uint64(0), false, false},
		{float32(0), false, false},
		{float32(0.0), false, false},
		{float64(0), false, false},
		{float64(0.0), false, false},

		{time.Duration(0), false, false},
		{json.Number("0"), false, false},

		{nil, false, false},
		{ptr, false, false},
		{"false", false, false},
		{"FALSE", false, false},
		{"False", false, false},
		{"f", false, false},
		{"F", false, false},
		{false, false, false},

		{"true", true, false},
		{"TRUE", true, false},
		{"True", true, false},
		{"t", true, false},
		{"T", true, false},

		{1, true, false},
		{int(1), true, false},
		{int8(1), true, false},
		{int16(1), true, false},
		{int32(1), true, false},
		{int64(1), true, false},
		{uint(1), true, false},
		{uint8(1), true, false},
		{uint16(1), true, false},
		{uint32(1), true, false},
		{uint64(1), true, false},
		{float32(1), true, false},
		{float32(1.0), true, false},
		{float64(1), true, false},
		{float64(1.0), true, false},

		{time.Duration(1), true, false},
		{json.Number("1"), true, false},
		{json.Number("1.0"), true, false},

		{true, true, false},
		{-1, true, false},
		{int(-1), true, false},
		{int8(-1), true, false},
		{int16(-1), true, false},
		{int32(-1), true, false},
		{int64(-1), true, false},

		// Alias
		{MyBool(true), true, false},
		{MyBool(false), false, false},

		// Failure cases
		{"test", false, true},
		{testing.T{}, false, true},
	}

	runTests(t, testCases, cast.ToBool, cast.ToBoolE)
}

func BenchmarkToBool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if !cast.ToBool(true) {
			b.Fatal("ToBool returned false")
		}
	}
}

func TestString(t *testing.T) {
	type Key struct {
		k string
	}
	key := &Key{"foo"}

	var ptr *string

	testCases := []testCase{
		{int(8), "8", false},
		{int8(8), "8", false},
		{int16(8), "8", false},
		{int32(8), "8", false},
		{int64(8), "8", false},
		{uint(8), "8", false},
		{uint8(8), "8", false},
		{uint16(8), "8", false},
		{uint32(8), "8", false},
		{uint64(8), "8", false},
		{float32(8.31), "8.31", false},
		{float64(8.31), "8.31", false},
		{json.Number("8"), "8", false},
		{true, "true", false},
		{false, "false", false},
		{nil, "", false},
		{ptr, "", false},
		{[]byte("one time"), "one time", false},
		{"one more time", "one more time", false},
		{template.HTML("one time"), "one time", false},
		{template.URL("http://somehost.foo"), "http://somehost.foo", false},
		{template.JS("(1+2)"), "(1+2)", false},
		{template.CSS("a"), "a", false},
		{template.HTMLAttr("a"), "a", false},

		// Alias
		{MyString("foo"), "foo", false},

		// Stringer and error
		{foo{val: "bar"}, "bar", false},
		{fu{val: "bar"}, "bar", false},

		// Failure cases
		{testing.T{}, "", true},
		{key, "", true},
	}

	runTests(t, testCases, cast.ToString, cast.ToStringE)
}

type foo struct {
	val string
}

func (x foo) String() string {
	return x.val
}

type fu struct {
	val string
}

func (x fu) Error() string {
	return x.val
}
