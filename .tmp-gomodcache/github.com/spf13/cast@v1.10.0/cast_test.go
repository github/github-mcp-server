// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cast_test

import (
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/spf13/cast"
)

type testCase struct {
	input       any
	expected    any
	expectError bool
}

func runTests[T cast.Basic](t *testing.T, testCases []testCase, to func(i any) T, toErr func(i any) (T, error)) {
	var typ T
	_, isTime := any(typ).(time.Time)

	res := func(i any) any {
		return i
	}

	if isTime {
		res = func(i any) any {
			return i.(time.Time).UTC()
		}
	}

	for _, testCase := range testCases {
		// TODO: remove after minimum Go version is >=1.22
		testCase := testCase

		t.Run("", func(t *testing.T) {
			t.Parallel()

			t.Run("Value", func(t *testing.T) {
				t.Run("ToType", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v := to(testCase.input)
					c.Assert(res(v), qt.Equals, testCase.expected)
				})

				t.Run("To", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v := cast.To[T](testCase.input)
					c.Assert(res(v), qt.Equals, testCase.expected)
				})

				t.Run("ToTypeE", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v, err := toErr(testCase.input)
					if testCase.expectError {
						c.Assert(err, qt.IsNotNil)
					} else {
						c.Assert(err, qt.IsNil)
						c.Assert(res(v), qt.Equals, testCase.expected)
					}
				})

				t.Run("ToE", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v, err := cast.ToE[T](testCase.input)
					if testCase.expectError {
						c.Assert(err, qt.IsNotNil)
					} else {
						c.Assert(err, qt.IsNil)
						c.Assert(res(v), qt.Equals, testCase.expected)
					}
				})
			})

			t.Run("Pointer", func(t *testing.T) {
				t.Run("ToType", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v := to(&testCase.input)
					c.Assert(res(v), qt.Equals, testCase.expected)
				})

				t.Run("To", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v := cast.To[T](&testCase.input)
					c.Assert(res(v), qt.Equals, testCase.expected)
				})

				t.Run("ToTypeE", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v, err := toErr(&testCase.input)
					if testCase.expectError {
						c.Assert(err, qt.IsNotNil)
					} else {
						c.Assert(err, qt.IsNil)
						c.Assert(res(v), qt.Equals, testCase.expected)
					}
				})

				t.Run("ToE", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v, err := cast.ToE[T](&testCase.input)
					if testCase.expectError {
						c.Assert(err, qt.IsNotNil)
					} else {
						c.Assert(err, qt.IsNil)
						c.Assert(res(v), qt.Equals, testCase.expected)
					}
				})
			})
		})
	}
}

func Example() {
	// Cast a value to another type
	{
		v, err := cast.ToIntE("1234")
		if err != nil {
			panic(err)
		}
		fmt.Printf("%T(%v)\n", v, v)
	}

	// Alternatively, you can use the generic [ToE] function for [Basic] types
	{
		v, err := cast.ToE[int]("4321")
		if err != nil {
			panic(err)
		}
		fmt.Printf("%T(%v)\n", v, v)
	}

	// You can suppress errors by using the non-error versions
	{
		v := cast.ToInt("9876")
		fmt.Printf("%T(%v)\n", v, v)
	}

	// Similarly, there is a generic [To] function for [Basic] types
	{
		v := cast.To[int]("6789")
		fmt.Printf("%T(%v)\n", v, v)
	}

	// Finally, you can use [Must] to panic if there is an error.
	{
		v := cast.Must[int](cast.ToE[int]("5555"))
		fmt.Printf("%T(%v)\n", v, v)
	}

	// Output:
	// int(1234)
	// int(4321)
	// int(9876)
	// int(6789)
	// int(5555)
}

func BenchmarkCast(b *testing.B) {
	b.Run("Bool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cast.ToBool("true")
		}
	})

	b.Run("String", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cast.ToString(123456789)
		}
	})

	b.Run("Number", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cast.ToNumber[int]("123456789")
		}
	})

	b.Run("Int64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cast.ToInt64("123456789")
		}
	})

	b.Run("BoolSlice", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cast.ToBoolSlice([]string{"true", "false", "TRUE", "false"})
		}
	})

	b.Run("StringSlice", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cast.ToStringSlice([]int{123456789, 123456789, 123456789, 123456789})
		}
	})
}

// Alias types for alias testing
type MyString string
type MyBool bool
type MyInt int
type MyInt8 int8
type MyInt16 int16
type MyInt32 int32
type MyInt64 int64
type MyUint uint
type MyUint8 uint8
type MyUint16 uint16
type MyUint32 uint32
type MyUint64 uint64
type MyFloat32 float32
type MyFloat64 float64
