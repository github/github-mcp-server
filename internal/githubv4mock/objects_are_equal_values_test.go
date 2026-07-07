// The contents of this file are taken from https://github.com/stretchr/testify/blob/016e2e9c269209287f33ec203f340a9a723fe22c/assert/assertions_test.go#L140-L174
//
// There is a modification to test objectsAreEqualValues to check that typed nils are equal, even if their types are different.

// The original license, copied from https://github.com/stretchr/testify/blob/016e2e9c269209287f33ec203f340a9a723fe22c/LICENSE
//
// MIT License
//
// Copyright (c) 2012-2020 Mat Ryer, Tyler Bunnell and contributors.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package githubv4mock

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestObjectsAreEqualValues(t *testing.T) {
	now := time.Now()

	cases := []struct {
		expected interface{}
		actual   interface{}
		result   bool
	}{
		{uint32(10), int32(10), true},
		{0, nil, false},
		{nil, 0, false},
		{now, now.In(time.Local), false}, // should not be time zone independent
		{int(270), int8(14), false},      // should handle overflow/underflow
		{int8(14), int(270), false},
		{[]int{270, 270}, []int8{14, 14}, false},
		{complex128(1e+100 + 1e+100i), complex64(complex(math.Inf(0), math.Inf(0))), false},
		{complex64(complex(math.Inf(0), math.Inf(0))), complex128(1e+100 + 1e+100i), false},
		{complex128(1e+100 + 1e+100i), 270, false},
		{270, complex128(1e+100 + 1e+100i), false},
		{complex128(1e+100 + 1e+100i), 3.14, false},
		{3.14, complex128(1e+100 + 1e+100i), false},
		{complex128(1e+10 + 1e+10i), complex64(1e+10 + 1e+10i), true},
		{complex64(1e+10 + 1e+10i), complex128(1e+10 + 1e+10i), true},
		{(*string)(nil), nil, true},         // typed nil vs untyped nil
		{(*string)(nil), (*int)(nil), true}, // different typed nils
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("ObjectsAreEqualValues(%#v, %#v)", c.expected, c.actual), func(t *testing.T) {
			res := objectsAreEqualValues(c.expected, c.actual)

			if res != c.result {
				t.Errorf("ObjectsAreEqualValues(%#v, %#v) should return %#v", c.expected, c.actual, c.result)
			}
		})
	}
}
