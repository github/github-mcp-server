// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package cast

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

func BenchmarkToInt(b *testing.B) {
	convert := func(num52 interface{}) {
		if v := ToInt(num52); v != 52 {
			b.Fatalf("ToInt returned wrong value, got %d, want %d", v, 32)
		}
	}
	for i := 0; i < b.N; i++ {
		convert("52")
		convert(52.0)
		convert(uint64(52))
	}
}

func BenchmarkTrimZeroDecimal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		trimZeroDecimal("")
		trimZeroDecimal("123")
		trimZeroDecimal("120")
		trimZeroDecimal("120.00")
	}
}

func TestTrimZeroDecimal(t *testing.T) {
	c := qt.New(t)

	c.Assert(trimZeroDecimal("10.0"), qt.Equals, "10")
	c.Assert(trimZeroDecimal("10.00"), qt.Equals, "10")
	c.Assert(trimZeroDecimal("10.010"), qt.Equals, "10.010")
	c.Assert(trimZeroDecimal("0.0000000000"), qt.Equals, "0")
	c.Assert(trimZeroDecimal("0.00000000001"), qt.Equals, "0.00000000001")
}

func TestTrimDecimal(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"10.0", "10"},
		{"10.010", "10"},
		{"00000.00001", "00000"},
		{"-0001.0", "-0001"},
		{".5", "0"},
		{"+12.", "+12"},
		{"+.25", "+0"},
		{"-.25", "-0"},
		{"0.0000000000", "0"},
		{"0.0000000001", "0"},
		{"10.0000000000", "10"},
		{"10.0000000001", "10"},
		{"10000000000000.0000000000", "10000000000000"},

		{"10...17", "10...17"},
		{"10.foobar", "10.foobar"},
		{"10.0i", "10.0i"},
		{"10.0E9", "10.0E9"},
	}

	for _, testCase := range testCases {
		// TODO: remove after minimum Go version is >=1.22
		testCase := testCase

		t.Run(testCase.input, func(t *testing.T) {
			c := qt.New(t)

			c.Assert(trimDecimal(testCase.input), qt.Equals, testCase.expected)
		})
	}
}

// Analysis (in the order of performance):
//
// - Trimming decimals based on decimal point yields a lot of incorrectly parsed values.
// - Parsing to float might be better, but we still need to cast the number, it might overflow, problematic.
// - Regex parsing is an order of magnitude slower, but it yields correct results.
func BenchmarkDecimal(b *testing.B) {
	testCases := []struct {
		input       string
		expectError bool
	}{
		{"10.0", false},
		{"10.00", false},
		{"10.010", false},
		{"0.0000000000", false},
		{"0.0000000001", false},
		{"10.0000000000", false},
		{"10.0000000001", false},
		{"10000000000000.0000000000", false},

		// {"10...17", true},
		// {"10.foobar", true},
		// {"10.0i", true},
		// {"10.0E9", true},
	}

	trimDecimalString := func(s string) string {
		// trim the decimal part (if any)
		if i := strings.Index(s, "."); i >= 0 {
			s = s[:i]
		}

		return s
	}

	re := regexp.MustCompile(`^([-+]?\d*)(\.\d*)?$`)

	trimDecimalRegex := func(s string) string {
		matches := re.FindStringSubmatch(s)
		if matches != nil {
			// matches[1] is the captured integer part with sign
			return matches[1]
		}

		return s
	}

	for _, testCase := range testCases {
		// TODO: remove after minimum Go version is >=1.22
		testCase := testCase

		b.Run(testCase.input, func(b *testing.B) {
			b.Run("ParseFloat", func(b *testing.B) {
				// TODO: use b.Loop() once updated to Go 1.24
				for i := 0; i < b.N; i++ {
					v, err := strconv.ParseFloat(testCase.input, 64)
					if (err != nil) != testCase.expectError {
						if err != nil {
							b.Fatal(err)
						}

						b.Fatal("expected error, but got none")
					}

					n := int64(v)
					_ = n
				}
			})

			b.Run("TrimDecimalString", func(b *testing.B) {
				// TODO: use b.Loop() once updated to Go 1.24
				for i := 0; i < b.N; i++ {
					v, err := strconv.ParseInt(trimDecimalString(testCase.input), 0, 0)
					if (err != nil) != testCase.expectError {
						if err != nil {
							b.Fatal(err)
						}

						b.Fatal("expected error, but got none")
					}

					_ = v
				}
			})

			b.Run("TrimDecimalRegex", func(b *testing.B) {
				// TODO: use b.Loop() once updated to Go 1.24
				for i := 0; i < b.N; i++ {
					v, err := strconv.ParseInt(trimDecimalRegex(testCase.input), 0, 0)
					if (err != nil) != testCase.expectError {
						if err != nil {
							b.Fatal(err)
						}

						b.Fatal("expected error, but got none")
					}

					_ = v
				}
			})
		})
	}
}
