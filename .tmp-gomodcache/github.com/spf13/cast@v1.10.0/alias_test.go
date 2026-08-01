// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package cast

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestAlias(t *testing.T) {
	type MyStruct struct{}

	type MyString string
	type MyOtherString MyString
	type MyAliasString = MyOtherString

	type MyBool bool
	type MyOtherBool MyBool
	type MyAliasBool = MyOtherBool

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

	var myStruct *MyStruct

	testCases := []struct {
		input         any
		expectedValue any
		expectedOk    bool
	}{
		{"string", "string", false},     // Already resolved
		{MyStruct{}, MyStruct{}, false}, // Non-resolvable
		{nil, nil, false},
		{&MyStruct{}, &MyStruct{}, false},
		{myStruct, myStruct, false},

		{MyString("string"), "string", true},
		{MyOtherString("string"), "string", true},
		{MyAliasString("string"), "string", true},

		{MyBool(true), true, true},
		{MyOtherBool(true), true, true},
		{MyAliasBool(true), true, true},

		{MyInt(1234), int(1234), true},
		{MyInt8(123), int8(123), true},
		{MyInt16(1234), int16(1234), true},
		{MyInt32(1234), int32(1234), true},
		{MyInt64(1234), int64(1234), true},

		{MyUint(1234), uint(1234), true},
		{MyUint8(123), uint8(123), true},
		{MyUint16(1234), uint16(1234), true},
		{MyUint32(1234), uint32(1234), true},
		{MyUint64(1234), uint64(1234), true},

		{MyFloat32(1.0), float32(1.0), true},
		{MyFloat64(1.0), float64(1.0), true},
	}

	for _, testCase := range testCases {
		// TODO: remove after minimum Go version is >=1.22
		testCase := testCase

		t.Run("", func(t *testing.T) {
			c := qt.New(t)

			actualValue, ok := resolveAlias(testCase.input)

			c.Assert(actualValue, qt.Equals, testCase.expectedValue)
			c.Assert(ok, qt.Equals, testCase.expectedOk)
		})
	}
}
