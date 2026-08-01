package ascii

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/segmentio/asm/internal/buffer"
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
	buf := newBuffer(t, 1024)

	for _, input := range [2][]byte{buf.ProtectHead(), buf.ProtectTail()} {
		for i := 0; i < len(input); i++ {
			in := input[:i+1]

			for b := 0; b <= 0xFF; b++ {
				in[i] = byte(b)
				if b < 0x80 {
					if !Valid(in) {
						t.Errorf("should be valid: %v", in)
					}
				} else {
					if Valid(in) {
						t.Errorf("should not be valid: %v", in)
					}
				}
				in[i] = 'x'
			}
		}
	}
}

func TestValidPrint(t *testing.T) {
	buf := newBuffer(t, 1024)

	for _, input := range [2][]byte{buf.ProtectHead(), buf.ProtectTail()} {
		for i := 0; i < len(input); i++ {
			in := input[:i+1]

			for b := 0; b <= 0xFF; b++ {
				in[i] = byte(b)
				if ' ' <= b && b <= '~' {
					if !ValidPrint(in) {
						t.Errorf("should be valid: %v", in)
					}
				} else {
					if ValidPrint(in) {
						t.Errorf("should not be valid: %v", in)
					}
				}
				in[i] = 'x'
			}
		}
	}
}

func TestValidString(t *testing.T) {
	testValidationFunction(t, testValid, ValidString)
}

func TestValidPrintString(t *testing.T) {
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
			for i := 0; i < b.N; i++ {
				_ = function(test)
			}
			b.SetBytes(int64(len(test)))
		})
	}
}

func TestHasPrefixFoldString(t *testing.T) {
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

func TestHasSuffixFoldString(t *testing.T) {
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

func TestEqualFoldString(t *testing.T) {
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

func newBuffer(t *testing.T, n int) buffer.Buffer {
	buf, err := buffer.New(n)
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

func TestEqualFold(t *testing.T) {
	ubuf := newBuffer(t, 1024)
	defer ubuf.Release()

	lbuf := newBuffer(t, 1024)
	defer lbuf.Release()

	mbuf := newBuffer(t, 1024)
	defer mbuf.Release()

	upper := ubuf.ProtectHead()
	lower := lbuf.ProtectTail()
	mixed := mbuf.ProtectHead()

	for i := 0; i < len(upper); i++ {
		u := upper[:i+1]
		l := lower[:i+1]
		m := mixed[:i+1]

		u[i] = 'Z'
		l[i] = 'a'
		if EqualFold(l, u) {
			t.Errorf("%q matches %q", l, u)
		}

		u[i] = byte(i % 128)
		l[i] = byte(i % 128)
		if 'A' <= l[i] && l[i] <= 'Z' {
			l[i] += 32
		}
		if 'a' <= u[i] && u[i] <= 'z' {
			u[i] -= 32
		}

		if rand.Int()%2 == 0 {
			m[i] = l[i]
		} else {
			m[i] = u[i]
		}

		if !EqualFold(l, u) {
			t.Errorf("%q does not match %q", l, u)
		}

		if !EqualFold(l, m) {
			t.Errorf("%q does not match %q", l, m)
		}

		if !EqualFold(u, m) {
			t.Errorf("%q does not match %q", u, m)
		}
	}
}

func genValidString(n int, ch byte) (s string) {
	for i := 0; i < n; i++ {
		s += string(byte(i%26) + ch)
	}
	return
}

func genEqualStrings(n int) (l string, u string) {
	return genValidString(n, 'A'), genValidString(n, 'a')
}

func BenchmarkEqualFoldString(b *testing.B) {
	sizes := [...]int{7, 8, 9, 15, 16, 17, 31, 32, 33, 512, 2000}

	for _, s := range sizes {
		lower, upper := genEqualStrings(s)
		b.Run(fmt.Sprintf("%04d", s), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				EqualFoldString(lower, upper)
			}
			b.SetBytes(int64(len(lower) + len(upper)))
		})
	}
}

func BenchmarkValidString(b *testing.B) {
	sizes := [...]int{7, 8, 9, 15, 16, 17, 31, 32, 33, 512, 2000}

	for _, s := range sizes {
		str := genValidString(s, 'a')
		b.Run(fmt.Sprintf("%04d", s), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ValidString(str)
			}
			b.SetBytes(int64(s))
		})
	}
}

func BenchmarkValidPrintString(b *testing.B) {
	sizes := [...]int{7, 8, 9, 15, 16, 17, 31, 32, 33, 512, 2000}

	for _, s := range sizes {
		str := genValidString(s, 'a')
		b.Run(fmt.Sprintf("%04d", s), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ValidPrintString(str)
			}
			b.SetBytes(int64(s))
		})
	}
}

func limit(s string) string {
	if len(s) > 17 {
		return s[:17] + "..."
	}
	return s
}
