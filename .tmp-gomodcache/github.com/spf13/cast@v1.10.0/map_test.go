// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cast_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/spf13/cast"
)

func runMapTests[K comparable, V cast.Basic | any](t *testing.T, testCases []testCase, to func(i any) map[K]V, toErr func(i any) (map[K]V, error)) {
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

			// t.Run("Pointer", func(t *testing.T) {
			// 	t.Run("ToType", func(t *testing.T) {
			// 		t.Parallel()

			// 		c := qt.New(t)

			// 		v := to(&testCase.input)
			// 		if v == nil {
			// 			return
			// 		}

			// 		c.Assert(v, qt.DeepEquals, testCase.expected)
			// 	})

			// 	// t.Run("To", func(t *testing.T) {
			// 	// 	return

			// 	// 	t.Parallel()

			// 	// 	c := qt.New(t)

			// 	// 	v := cast.To[T](&testCase.input)
			// 	// 	c.Assert(v, qt.DeepEquals, testCase.expected)
			// 	// })

			// 	t.Run("ToTypeE", func(t *testing.T) {
			// 		t.Parallel()

			// 		c := qt.New(t)

			// 		v, err := toErr(&testCase.input)
			// 		if testCase.expectError {
			// 			c.Assert(err, qt.IsNotNil)
			// 		} else {
			// 			c.Assert(err, qt.IsNil)
			// 			c.Assert(v, qt.DeepEquals, testCase.expected)
			// 		}
			// 	})

			// 	// t.Run("ToE", func(t *testing.T) {
			// 	// 	return

			// 	// 	t.Parallel()

			// 	// 	c := qt.New(t)

			// 	// 	v, err := cast.ToE[T](&testCase.input)
			// 	// 	if testCase.expectError {
			// 	// 		c.Assert(err, qt.IsNotNil)
			// 	// 	} else {
			// 	// 		c.Assert(err, qt.IsNil)
			// 	// 		c.Assert(v, qt.DeepEquals, testCase.expected)
			// 	// 	}
			// 	// })
			// })
		})
	}
}

func TestStringMapStringSlice(t *testing.T) {
	// ToStringMapString inputs/outputs
	var stringMapString = map[string]string{"key 1": "value 1", "key 2": "value 2", "key 3": "value 3"}
	var stringMapInterface = map[string]any{"key 1": "value 1", "key 2": "value 2", "key 3": "value 3"}
	var interfaceMapString = map[any]string{"key 1": "value 1", "key 2": "value 2", "key 3": "value 3"}
	var interfaceMapInterface = map[any]any{"key 1": "value 1", "key 2": "value 2", "key 3": "value 3"}

	// ToStringMapStringSlice inputs/outputs
	var stringMapStringSlice = map[string][]string{"key 1": {"value 1", "value 2", "value 3"}, "key 2": {"value 1", "value 2", "value 3"}, "key 3": {"value 1", "value 2", "value 3"}}
	var stringMapInterfaceSlice = map[string][]any{"key 1": {"value 1", "value 2", "value 3"}, "key 2": {"value 1", "value 2", "value 3"}, "key 3": {"value 1", "value 2", "value 3"}}
	var stringMapInterfaceInterfaceSlice = map[string]any{"key 1": []any{"value 1", "value 2", "value 3"}, "key 2": []any{"value 1", "value 2", "value 3"}, "key 3": []any{"value 1", "value 2", "value 3"}}
	var stringMapStringSingleSliceFieldsResult = map[string][]string{"key 1": {"value", "1"}, "key 2": {"value", "2"}, "key 3": {"value", "3"}}
	var interfaceMapStringSlice = map[any][]string{"key 1": {"value 1", "value 2", "value 3"}, "key 2": {"value 1", "value 2", "value 3"}, "key 3": {"value 1", "value 2", "value 3"}}
	var interfaceMapInterfaceSlice = map[any][]any{"key 1": {"value 1", "value 2", "value 3"}, "key 2": {"value 1", "value 2", "value 3"}, "key 3": {"value 1", "value 2", "value 3"}}

	var stringMapStringSliceMultiple = map[string][]string{"key 1": {"value 1", "value 2", "value 3"}, "key 2": {"value 1", "value 2", "value 3"}, "key 3": {"value 1", "value 2", "value 3"}}
	var stringMapStringSliceSingle = map[string][]string{"key 1": {"value 1"}, "key 2": {"value 2"}, "key 3": {"value 3"}}

	var stringMapInterface1 = map[string]any{"key 1": []string{"value 1"}, "key 2": []string{"value 2"}}
	var stringMapInterfaceResult1 = map[string][]string{"key 1": {"value 1"}, "key 2": {"value 2"}}

	var jsonStringMapString = `{"key 1": "value 1", "key 2": "value 2"}`
	var jsonStringMapStringArray = `{"key 1": ["value 1"], "key 2": ["value 2", "value 3"]}`
	var jsonStringMapStringArrayResult = map[string][]string{"key 1": {"value 1"}, "key 2": {"value 2", "value 3"}}

	type Key struct {
		k string
	}

	testCases := []testCase{
		{stringMapStringSlice, stringMapStringSlice, false},
		{stringMapInterfaceSlice, stringMapStringSlice, false},
		{stringMapInterfaceInterfaceSlice, stringMapStringSlice, false},
		{stringMapStringSliceMultiple, stringMapStringSlice, false},
		{stringMapStringSliceMultiple, stringMapStringSlice, false},
		{stringMapString, stringMapStringSliceSingle, false},
		{stringMapInterface, stringMapStringSliceSingle, false},
		{stringMapInterface1, stringMapInterfaceResult1, false},
		{interfaceMapStringSlice, stringMapStringSlice, false},
		{interfaceMapInterfaceSlice, stringMapStringSlice, false},
		{interfaceMapString, stringMapStringSingleSliceFieldsResult, false},
		{interfaceMapInterface, stringMapStringSingleSliceFieldsResult, false},
		{jsonStringMapStringArray, jsonStringMapStringArrayResult, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{map[any]any{"foo": testing.T{}}, nil, true},
		{map[any]any{Key{"foo"}: "bar"}, nil, true}, // ToStringE(Key{"foo"}) should fail
		{jsonStringMapString, nil, true},
		{"", nil, true},
	}

	runMapTests(t, testCases, cast.ToStringMapStringSlice, cast.ToStringMapStringSliceE)
}

func TestStringMap(t *testing.T) {
	testCases := []testCase{
		{map[any]any{"tag": "tags", "group": "groups"}, map[string]any{"tag": "tags", "group": "groups"}, false},
		{map[string]any{"tag": "tags", "group": "groups"}, map[string]any{"tag": "tags", "group": "groups"}, false},
		{`{"tag": "tags", "group": "groups"}`, map[string]any{"tag": "tags", "group": "groups"}, false},
		{`{"tag": "tags", "group": true}`, map[string]any{"tag": "tags", "group": true}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{"", nil, true},
	}

	runMapTests(t, testCases, cast.ToStringMap, cast.ToStringMapE)
}

func TestStringMapBool(t *testing.T) {
	testCases := []testCase{
		{map[any]any{"v1": true, "v2": false}, map[string]bool{"v1": true, "v2": false}, false},
		{map[string]any{"v1": true, "v2": false}, map[string]bool{"v1": true, "v2": false}, false},
		{map[string]bool{"v1": true, "v2": false}, map[string]bool{"v1": true, "v2": false}, false},
		{`{"v1": true, "v2": false}`, map[string]bool{"v1": true, "v2": false}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{"", nil, true},
	}

	runMapTests(t, testCases, cast.ToStringMapBool, cast.ToStringMapBoolE)
}

func TestStringMapInt(t *testing.T) {
	testCases := []testCase{
		{map[any]any{"v1": 1, "v2": 222}, map[string]int{"v1": 1, "v2": 222}, false},
		{map[string]any{"v1": 342, "v2": 5141}, map[string]int{"v1": 342, "v2": 5141}, false},
		{map[string]int{"v1": 33, "v2": 88}, map[string]int{"v1": 33, "v2": 88}, false},
		{map[string]int32{"v1": int32(33), "v2": int32(88)}, map[string]int{"v1": 33, "v2": 88}, false},
		{map[string]uint16{"v1": uint16(33), "v2": uint16(88)}, map[string]int{"v1": 33, "v2": 88}, false},
		{map[string]float64{"v1": float64(8.22), "v2": float64(43.32)}, map[string]int{"v1": 8, "v2": 43}, false},
		{`{"v1": 67, "v2": 56}`, map[string]int{"v1": 67, "v2": 56}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{"", nil, true},
	}

	runMapTests(t, testCases, cast.ToStringMapInt, cast.ToStringMapIntE)
}

func TestStringMapInt64(t *testing.T) {
	testCases := []testCase{
		{map[any]any{"v1": int32(8), "v2": int32(888)}, map[string]int64{"v1": int64(8), "v2": int64(888)}, false},
		{map[string]any{"v1": int64(45), "v2": int64(67)}, map[string]int64{"v1": 45, "v2": 67}, false},
		{map[string]int64{"v1": 33, "v2": 88}, map[string]int64{"v1": 33, "v2": 88}, false},
		{map[string]int{"v1": 33, "v2": 88}, map[string]int64{"v1": 33, "v2": 88}, false},
		{map[string]int32{"v1": int32(33), "v2": int32(88)}, map[string]int64{"v1": 33, "v2": 88}, false},
		{map[string]uint16{"v1": uint16(33), "v2": uint16(88)}, map[string]int64{"v1": 33, "v2": 88}, false},
		{map[string]float64{"v1": float64(8.22), "v2": float64(43.32)}, map[string]int64{"v1": 8, "v2": 43}, false},
		{`{"v1": 67, "v2": 56}`, map[string]int64{"v1": 67, "v2": 56}, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{"", nil, true},
	}

	runMapTests(t, testCases, cast.ToStringMapInt64, cast.ToStringMapInt64E)
}

func TestStringMapString(t *testing.T) {
	var stringMapString = map[string]string{"key 1": "value 1", "key 2": "value 2", "key 3": "value 3"}
	var stringMapInterface = map[string]any{"key 1": "value 1", "key 2": "value 2", "key 3": "value 3"}
	var interfaceMapString = map[any]string{"key 1": "value 1", "key 2": "value 2", "key 3": "value 3"}
	var interfaceMapInterface = map[any]any{"key 1": "value 1", "key 2": "value 2", "key 3": "value 3"}
	var jsonString = `{"key 1": "value 1", "key 2": "value 2", "key 3": "value 3"}`
	var invalidJsonString = `{"key 1": "value 1", "key 2": "value 2", "key 3": "value 3"`
	var emptyString = ""

	testCases := []testCase{
		{stringMapString, stringMapString, false},
		{stringMapInterface, stringMapString, false},
		{interfaceMapString, stringMapString, false},
		{interfaceMapInterface, stringMapString, false},
		{jsonString, stringMapString, false},

		// Failure cases
		{nil, nil, true},
		{testing.T{}, nil, true},
		{invalidJsonString, nil, true},
		{emptyString, nil, true},
	}

	runMapTests(t, testCases, cast.ToStringMapString, cast.ToStringMapStringE)
}
