package assert

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// True asserts that an expression is true.
func True(t testing.TB, ok bool, msgAndArgs ...any) {
	if ok {
		return
	}
	t.Helper()
	t.Fatal(formatMsgAndArgs("Expected expression to be true", msgAndArgs...))
}

// False asserts that an expression is false.
func False(t testing.TB, ok bool, msgAndArgs ...any) {
	if !ok {
		return
	}
	t.Helper()
	t.Fatal(formatMsgAndArgs("Expected expression to be false", msgAndArgs...))
}

// Equal asserts that "expected" and "actual" are equal.
func Equal[T any](t testing.TB, expected, actual T, msgAndArgs ...any) {
	if objectsAreEqual(expected, actual) {
		return
	}
	t.Helper()
	msg := formatMsgAndArgs("Expected values to be equal:", msgAndArgs...)
	t.Fatalf("%s\n%s", msg, diff(expected, actual))
}

// Error asserts that an error is not nil.
func Error(t testing.TB, err error, msgAndArgs ...any) {
	if err != nil {
		return
	}
	t.Helper()
	t.Fatal(formatMsgAndArgs("Expected an error", msgAndArgs...))
}

// NoError asserts that an error is nil.
func NoError(t testing.TB, err error, msgAndArgs ...any) {
	if err == nil {
		return
	}
	t.Helper()
	msg := formatMsgAndArgs("Unexpected error:", msgAndArgs...)
	t.Fatalf("%s\n%+v", msg, err)
}

// Panics asserts that the given function panics.
func Panics(t testing.TB, fn func(), msgAndArgs ...any) {
	t.Helper()
	defer func() {
		if recover() == nil {
			msg := formatMsgAndArgs("Expected function to panic", msgAndArgs...)
			t.Fatal(msg)
		}
	}()
	fn()
}

// Zero asserts that a value is its zero value.
func Zero[T any](t testing.TB, value T, msgAndArgs ...any) {
	var zero T
	if objectsAreEqual(value, zero) {
		return
	}
	val := reflect.ValueOf(value)
	if (val.Kind() == reflect.Slice || val.Kind() == reflect.Map || val.Kind() == reflect.Array) && val.Len() == 0 {
		return
	}
	t.Helper()
	msg := formatMsgAndArgs("Expected zero value but got:", msgAndArgs...)
	t.Fatalf("%s\n%s", msg, fmt.Sprintf("%v", value))
}

func NotZero[T any](t testing.TB, value T, msgAndArgs ...any) {
	var zero T
	if !objectsAreEqual(value, zero) {
		val := reflect.ValueOf(value)
		if !((val.Kind() == reflect.Slice || val.Kind() == reflect.Map || val.Kind() == reflect.Array) && val.Len() == 0) {
			return
		}
	}
	t.Helper()
	msg := formatMsgAndArgs("Unexpected zero value:", msgAndArgs...)
	t.Fatalf("%s\n%s", msg, fmt.Sprintf("%v", value))
}

func formatMsgAndArgs(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}
	format, ok := args[0].(string)
	if !ok {
		panic("message argument must be a fmt string")
	}
	return fmt.Sprintf(format, args[1:]...)
}

func diff(expected, actual any) string {
	lines := []string{
		"expected:",
		fmt.Sprintf("%v", expected),
		"actual:",
		fmt.Sprintf("%v", actual),
	}
	return strings.Join(lines, "\n")
}

func objectsAreEqual(expected, actual any) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}
	if exp, eok := expected.([]byte); eok {
		if act, aok := actual.([]byte); aok {
			return bytes.Equal(exp, act)
		}
	}
	if exp, eok := expected.(string); eok {
		if act, aok := actual.(string); aok {
			return exp == act
		}
	}

	return reflect.DeepEqual(expected, actual)
}
