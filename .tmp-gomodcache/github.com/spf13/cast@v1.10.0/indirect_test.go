// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cast

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestIndirect(t *testing.T) {
	c := qt.New(t)

	x := 13
	y := &x
	z := &y

	var ptr *string
	var ptrptr **string

	var i any

	var n = int(13)
	var i2 any = n

	var i3 any = &n

	var b *bool
	var i4 any = b

	testCases := []struct {
		input      any
		expected   any
		expectedOk bool
	}{
		{x, 13, false},
		{y, 13, true},
		{z, 13, true},
		{ptr, nil, true},
		{ptrptr, nil, true},
		{i, nil, false},
		{&i, nil, true},
		{i2, 13, false},
		{&i2, 13, true},
		{i3, 13, true},
		{&i3, 13, true},
		{i4, nil, true},
		{&i4, nil, true},
		{nil, nil, false},
	}

	for _, testCase := range testCases {
		// TODO: remove after minimum Go version is >=1.22
		testCase := testCase

		t.Run("", func(t *testing.T) {
			v, ok := indirect(testCase.input)

			c.Assert(v, qt.Equals, testCase.expected)
			c.Assert(ok, qt.Equals, testCase.expectedOk)
		})
	}
}
