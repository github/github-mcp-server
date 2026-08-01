package mem

import (
	"bytes"
	"testing"
)

func TestContainsByte(t *testing.T) {
	for _, test := range []struct {
		name     string
		haystack []byte
		needle   byte
		expect   bool
	}{
		{
			name:     "nil",
			haystack: nil,
			needle:   'x',
			expect:   false,
		},
		{
			name:     "empty",
			haystack: []byte{},
			needle:   'x',
			expect:   false,
		},
		{
			name:     "one equal byte",
			haystack: []byte{'x'},
			needle:   'x',
			expect:   true,
		},
		{
			name:     "one unequal byte",
			haystack: []byte{'a'},
			needle:   'x',
			expect:   false,
		},
		{
			name:     "run without byte",
			haystack: []byte("aaaaaaaaaaaaaaaaaa"),
			needle:   'x',
			expect:   false,
		},
		{
			name:     "run with byte prefix",
			haystack: []byte("xaaaaaaaaaaaaaaaaaa"),
			needle:   'x',
			expect:   true,
		},
		{
			name:     "run with byte suffix",
			haystack: []byte("aaaaaaaaaaaaaaaaaax"),
			needle:   'x',
			expect:   true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if actual := ContainsByte(test.haystack, test.needle); actual != test.expect {
				t.Fatalf("ContainsByte(%v, %v) => %v", test.haystack, test.needle, actual)
			}
		})
	}

	// Test specific lengths up to 1KB.
	var src []byte
	for i := 0; i < 1000; i++ {
		if ContainsByte(src, 'x') {
			t.Fatalf("ContainsByte(%v, 'x') => true", src)
		}
		src = append(src, 'x')
		if !ContainsByte(src, 'x') {
			t.Fatalf("ContainsByte(%v, 'x') => false", src)
		}
		src[i] = '0'
	}
}

func BenchmarkContainsByte(b *testing.B) {
	benchmarkContainsByte(b, ContainsByte)
}

func benchmarkContainsByte(b *testing.B, contains func([]byte, byte) bool) {
	large := bytes.Repeat([]byte{'a'}, 8*1024)

	b.Run("empty", benchmarkContainsByteCase(contains, nil, 'x'))
	b.Run("small-not-found", benchmarkContainsByteCase(contains, []byte("abcdef"), 'x'))
	b.Run("small-found-at-end", benchmarkContainsByteCase(contains, []byte("abcdefx"), 'x'))
	b.Run("large-not-found", benchmarkContainsByteCase(contains, large, 'x'))
	b.Run("large-found-at-end", benchmarkContainsByteCase(contains, append(large, 'x'), 'x'))
}

func benchmarkContainsByteCase(contains func([]byte, byte) bool, haystack []byte, needle byte) func(*testing.B) {
	return func(b *testing.B) {
		b.SetBytes(int64(len(haystack)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			contains(haystack, needle)
		}
	}
}
