// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cast_test

import (
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/spf13/cast"
)

func runSliceTests[T cast.Basic | any](t *testing.T, testCases []testCase, to func(i any) []T, toErr func(i any) ([]T, error)) {
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
					if v == nil {
						return
					}

					c.Assert(v, qt.DeepEquals, testCase.expected)
				})

				// t.Run("To", func(t *testing.T) {
				// 	return

				// 	t.Parallel()

				// 	c := qt.New(t)

				// 	v := cast.To[T](testCase.input)
				// 	c.Assert(v, qt.DeepEquals, testCase.expected)
				// })

				t.Run("ToTypeE", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v, err := toErr(testCase.input)
					if testCase.expectError {
						c.Assert(err, qt.IsNotNil)
					} else {
						c.Assert(err, qt.IsNil)
						c.Assert(v, qt.DeepEquals, testCase.expected)
					}
				})

				// t.Run("ToE", func(t *testing.T) {
				// 	return

				// 	t.Parallel()

				// 	c := qt.New(t)

				// 	v, err := cast.ToE[T](testCase.input)
				// 	if testCase.expectError {
				// 		c.Assert(err, qt.IsNotNil)
				// 	} else {
				// 		c.Assert(err, qt.IsNil)
				// 		c.Assert(v, qt.DeepEquals, testCase.expected)
				// 	}
				// })
			})

			t.Run("Pointer", func(t *testing.T) {
				t.Run("ToType", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v := to(&testCase.input)
					if v == nil {
						return
					}

					c.Assert(v, qt.DeepEquals, testCase.expected)
				})

				// t.Run("To", func(t *testing.T) {
				// 	return

				// 	t.Parallel()

				// 	c := qt.New(t)

				// 	v := cast.To[T](&testCase.input)
				// 	c.Assert(v, qt.DeepEquals, testCase.expected)
				// })

				t.Run("ToTypeE", func(t *testing.T) {
					t.Parallel()

					c := qt.New(t)

					v, err := toErr(&testCase.input)
					if testCase.expectError {
						c.Assert(err, qt.IsNotNil)
					} else {
						c.Assert(err, qt.IsNil)
						c.Assert(v, qt.DeepEquals, testCase.expected)
					}
				})

				// t.Run("ToE", func(t *testing.T) {
				// 	return

				// 	t.Parallel()

				// 	c := qt.New(t)

				// 	v, err := cast.ToE[T](&testCase.input)
				// 	if testCase.expectError {
				// 		c.Assert(err, qt.IsNotNil)
				// 	} else {
				// 		c.Assert(err, qt.IsNil)
				// 		c.Assert(v, qt.DeepEquals, testCase.expected)
				// 	}
				// })
			})
		})
	}
}

func TestBoolSlice(t *testing.T) {
	testCases := []testCase{
		{[]bool{true, false, true}, []bool{true, false, true}, false},
		{[]any{true, false, true}, []bool{true, false, true}, false},
		{[]int{1, 0, 1}, []bool{true, false, true}, false},
		{[]string{"true", "false", "true"}, []bool{true, false, true}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{[]string{"foo", "bar"}, nil, true},
	}

	runSliceTests(t, testCases, cast.ToBoolSlice, cast.ToBoolSliceE)
}

func TestIntSlice(t *testing.T) {
	testCases := []testCase{
		{[]int{1, 3}, []int{1, 3}, false},
		{[]any{1.2, 3.2}, []int{1, 3}, false},
		{[]string{"2", "3"}, []int{2, 3}, false},
		{[2]string{"2", "3"}, []int{2, 3}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{[]string{"foo", "bar"}, nil, true},
	}

	runSliceTests(t, testCases, cast.ToIntSlice, cast.ToIntSliceE)
}

func TestInt64Slice(t *testing.T) {
	testCases := []testCase{
		{[]int{1, 3}, []int64{1, 3}, false},
		{[]any{1.2, 3.2}, []int64{1, 3}, false},
		{[]string{"2", "3"}, []int64{2, 3}, false},
		{[2]string{"2", "3"}, []int64{2, 3}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{[]string{"foo", "bar"}, nil, true},
	}

	runSliceTests(t, testCases, cast.ToInt64Slice, cast.ToInt64SliceE)
}

func TestFloat64Slice(t *testing.T) {
	testCases := []testCase{
		{[]int{1, 3}, []float64{1, 3}, false},
		{[]float64{1.2, 3.2}, []float64{1.2, 3.2}, false},
		{[]any{1.2, 3.2}, []float64{1.2, 3.2}, false},
		{[]string{"2", "3"}, []float64{2, 3}, false},
		{[]string{"1.2", "3.2"}, []float64{1.2, 3.2}, false},
		{[2]string{"2", "3"}, []float64{2, 3}, false},
		{[2]string{"1.2", "3.2"}, []float64{1.2, 3.2}, false},
		{[]int32{1, 3}, []float64{1.0, 3.0}, false},
		{[]int64{1, 3}, []float64{1.0, 3.0}, false},
		{[]bool{true, false}, []float64{1.0, 0.0}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{[]string{"foo", "bar"}, nil, true},
	}

	runSliceTests(t, testCases, cast.ToFloat64Slice, cast.ToFloat64SliceE)
}

func TestUintSlice(t *testing.T) {
	testCases := []testCase{
		{[]uint{1, 3}, []uint{1, 3}, false},
		{[]any{1, 3}, []uint{1, 3}, false},
		{[]string{"2", "3"}, []uint{2, 3}, false},
		{[]int{1, 3}, []uint{1, 3}, false},
		{[]int32{1, 3}, []uint{1, 3}, false},
		{[]int64{1, 3}, []uint{1, 3}, false},
		{[]float32{1.0, 3.0}, []uint{1, 3}, false},
		{[]float64{1.0, 3.0}, []uint{1, 3}, false},
		{[]bool{true, false}, []uint{1, 0}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{[]string{"foo", "bar"}, nil, true},
	}

	runSliceTests(t, testCases, cast.ToUintSlice, cast.ToUintSliceE)
}

func TestSlice(t *testing.T) {
	testCases := []testCase{
		{[]any{1, 3}, []any{1, 3}, false},
		{[]map[string]any{{"k1": 1}, {"k2": 2}}, []any{map[string]any{"k1": 1}, map[string]any{"k2": 2}}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
	}

	runSliceTests(t, testCases, cast.ToSlice, cast.ToSliceE)
}

func TestStringSlice(t *testing.T) {
	testCases := []testCase{
		{[]int{1, 2}, []string{"1", "2"}, false},
		{[]int8{int8(1), int8(2)}, []string{"1", "2"}, false},
		{[]int32{int32(1), int32(2)}, []string{"1", "2"}, false},
		{[]int64{int64(1), int64(2)}, []string{"1", "2"}, false},
		{[]uint{uint(1), uint(2)}, []string{"1", "2"}, false},
		{[]uint8{uint8(1), uint8(2)}, []string{"1", "2"}, false},
		{[]uint32{uint32(1), uint32(2)}, []string{"1", "2"}, false},
		{[]uint64{uint64(1), uint64(2)}, []string{"1", "2"}, false},
		{[]float32{float32(1.01), float32(2.01)}, []string{"1.01", "2.01"}, false},
		{[]float64{float64(1.01), float64(2.01)}, []string{"1.01", "2.01"}, false},
		{[]string{"a", "b"}, []string{"a", "b"}, false},
		{[]any{1, 3}, []string{"1", "3"}, false},
		{any(1), []string{"1"}, false},
		{[]error{errors.New("a"), errors.New("b")}, []string{"a", "b"}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
	}

	runSliceTests(t, testCases, cast.ToStringSlice, cast.ToStringSliceE)
}

func TestDurationSlice(t *testing.T) {
	testCases := []testCase{
		{[]string{"1s", "1m"}, []time.Duration{time.Second, time.Minute}, false},
		{[]int{1, 2}, []time.Duration{1, 2}, false},
		{[]any{1, 3}, []time.Duration{1, 3}, false},
		{[]time.Duration{1, 3}, []time.Duration{1, 3}, false},

		// errors
		{nil, nil, true},
		{testing.T{}, nil, true},
		{[]string{"invalid"}, nil, true},
	}

	runSliceTests(t, testCases, cast.ToDurationSlice, cast.ToDurationSliceE)
}
