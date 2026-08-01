package ascii

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

var testStrings = [...]string{
	"",
	"a",
	"ab",
	"abc",
	"abcd",
	"hello",
	"Hello World!",
	"Hello\"World!",
	"Hello\\World!",
	"Hello\nWorld!",
	"Hello\rWorld!",
	"Hello\tWorld!",
	"Hello\bWorld!",
	"Hello\fWorld!",
	"H~llo World!",
	"H~llo",
	"你好",
	"~",
	"\x80",
	"\x7F",
	"\xFF",
	"\x1fxxx",
	"\x1fxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	"a string of 16B.",
	"an invalid string of 32B. \x00......",
	"some kind of long string with only ascii characters.",
	"some kind of long string with a non-ascii character at the end.\xff",
	strings.Repeat("1234567890", 1000),
}

var testStringsUTF8 []string

func init() {
	for _, test := range testStrings {
		if utf8.ValidString(test) {
			testStringsUTF8 = append(testStringsUTF8, test)
		}
	}
}

func testString(s string, f func(byte) bool) bool {
	for i := range s {
		if !f(s[i]) {
			return false
		}
	}
	return true
}

func testValid(s string) bool {
	return testString(s, ValidByte)
}

func testValidPrint(s string) bool {
	return testString(s, ValidPrintByte)
}

func TestValid(t *testing.T) {
	testValidationFunction(t, testValid, ValidString)
}

func TestValidPrint(t *testing.T) {
	testValidationFunction(t, testValidPrint, ValidPrintString)
}

func testValidationFunction(t *testing.T, reference, function func(string) bool) {
	for _, test := range testStrings {
		t.Run(limit(test), func(t *testing.T) {
			expect := reference(test)

			if valid := function(test); expect != valid {
				t.Errorf("expected %t but got %t", expect, valid)
			}
		})
	}
}

func BenchmarkValid(b *testing.B) {
	benchmarkValidationFunction(b, ValidString)
}

func BenchmarkValidPrint(b *testing.B) {
	benchmarkValidationFunction(b, ValidPrintString)
}

func benchmarkValidationFunction(b *testing.B, function func(string) bool) {
	for _, test := range testStrings {
		b.Run(limit(test), func(b *testing.B) {
			for range b.N {
				_ = function(test)
			}
			b.SetBytes(int64(len(test)))
		})
	}
}

func limit(s string) string {
	if len(s) > 17 {
		return s[:17] + "..."
	}
	return s
}

func TestHasPrefixFold(t *testing.T) {
	for _, test := range testStringsUTF8 {
		t.Run(limit(test), func(t *testing.T) {
			prefix := test
			if len(prefix) > 0 {
				prefix = prefix[:len(prefix)/2]
			}
			upper := strings.ToUpper(prefix)
			lower := strings.ToLower(prefix)

			if !HasPrefixFoldString(test, prefix) {
				t.Errorf("%q does not match %q", test, prefix)
			}

			if !HasPrefixFoldString(test, upper) {
				t.Errorf("%q does not match %q", test, upper)
			}

			if !HasPrefixFoldString(test, lower) {
				t.Errorf("%q does not match %q", test, lower)
			}
		})
	}
}

func TestHasSuffixFold(t *testing.T) {
	for _, test := range testStringsUTF8 {
		t.Run(limit(test), func(t *testing.T) {
			suffix := test
			if len(suffix) > 0 {
				suffix = suffix[len(suffix)/2:]
			}
			upper := strings.ToUpper(suffix)
			lower := strings.ToLower(suffix)

			if !HasSuffixFoldString(test, suffix) {
				t.Errorf("%q does not match %q", test, suffix)
			}

			if !HasSuffixFoldString(test, upper) {
				t.Errorf("%q does not match %q", test, upper)
			}

			if !HasSuffixFoldString(test, lower) {
				t.Errorf("%q does not match %q", test, lower)
			}
		})
	}
}

func TestEqualFoldASCII(t *testing.T) {
	pairs := [...][2]byte{
		{0, ' '},
		{'@', '`'},
		{'[', '{'},
		{'_', 127},
	}

	for _, pair := range pairs {
		t.Run(fmt.Sprintf("0x%02x=0x%02x", pair[0], pair[1]), func(t *testing.T) {
			for i := 1; i <= 256; i++ {
				a := bytes.Repeat([]byte{'x'}, i)
				b := bytes.Repeat([]byte{'X'}, i)

				if !EqualFold(a, b) {
					t.Errorf("%q does not match %q", a, b)
					break
				}

				a[0] = pair[0]
				b[0] = pair[1]

				if EqualFold(a, b) {
					t.Errorf("%q matches %q", a, b)
					break
				}
			}
		})
	}
}

func TestEqualFold(t *testing.T) {
	// Only test valid UTF-8 otherwise ToUpper/ToLower will convert invalid
	// characters to UTF-8 placeholders, which breaks the case-insensitive
	// equality.
	for _, test := range testStringsUTF8 {
		t.Run(limit(test), func(t *testing.T) {
			upper := strings.ToUpper(test)
			lower := strings.ToLower(test)

			if !EqualFoldString(test, test) {
				t.Errorf("%q does not match %q", test, test)
			}

			if !EqualFoldString(test, upper) {
				t.Errorf("%q does not match %q", test, upper)
			}

			if !EqualFoldString(test, lower) {
				t.Errorf("%q does not match %q", test, lower)
			}

			if len(test) > 1 {
				reverse := make([]byte, len(test))
				for i := range reverse {
					reverse[i] = test[len(test)-(i+1)]
				}

				if EqualFoldString(test, string(reverse)) {
					t.Errorf("%q matches %q", test, reverse)
				}
			}
		})
	}
}

func BenchmarkEqualFold(b *testing.B) {
	for _, test := range testStringsUTF8 {
		b.Run(limit(test), func(b *testing.B) {
			other := test + "_" // not the same pointer

			for range b.N {
				_ = EqualFoldString(test, other[:len(test)]) // same length
			}

			b.SetBytes(int64(len(test)))
		})
	}
}
