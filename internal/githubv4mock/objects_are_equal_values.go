// The contents of this file are taken from https://github.com/stretchr/testify/blob/016e2e9c269209287f33ec203f340a9a723fe22c/assert/assertions.go#L166
// because I do not want to take a dependency on the entire testify module just to use this equality check.
//
// There is a modification in objectsAreEqual to check that typed nils are equal, even if their types are different.
//
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
	"bytes"
	"reflect"
)

func objectsAreEqualValues(expected, actual any) bool {
	if objectsAreEqual(expected, actual) {
		return true
	}

	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)
	if !expectedValue.IsValid() || !actualValue.IsValid() {
		return false
	}

	expectedType := expectedValue.Type()
	actualType := actualValue.Type()
	if !expectedType.ConvertibleTo(actualType) {
		return false
	}

	if !isNumericType(expectedType) || !isNumericType(actualType) {
		// Attempt comparison after type conversion
		return reflect.DeepEqual(
			expectedValue.Convert(actualType).Interface(), actual,
		)
	}

	// If BOTH values are numeric, there are chances of false positives due
	// to overflow or underflow. So, we need to make sure to always convert
	// the smaller type to a larger type before comparing.
	if expectedType.Size() >= actualType.Size() {
		return actualValue.Convert(expectedType).Interface() == expected
	}

	return expectedValue.Convert(actualType).Interface() == actual
}

// objectsAreEqual determines if two objects are considered equal.
//
// This function does no assertion of any kind.
func objectsAreEqual(expected, actual any) bool {
	// There is a modification in objectsAreEqual to check that typed nils are equal, even if their types are different.
	// This is required because when a nil is provided as a variable, the type is not known.
	if isNil(expected) && isNil(actual) {
		return true
	}

	exp, ok := expected.([]byte)
	if !ok {
		return reflect.DeepEqual(expected, actual)
	}

	act, ok := actual.([]byte)
	if !ok {
		return false
	}
	if exp == nil || act == nil {
		return exp == nil && act == nil
	}
	return bytes.Equal(exp, act)
}

// isNumericType returns true if the type is one of:
// int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
// float32, float64, complex64, complex128
func isNumericType(t reflect.Type) bool {
	return t.Kind() >= reflect.Int && t.Kind() <= reflect.Complex128
}

func isNil(i any) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
