package json

import (
	"strings"
	"testing"
)

func BenchmarkEscapeIndex4KB(b *testing.B) {
	benchmarkEscapeIndex(b, strings.Repeat("!foobar!", 512), false)
}

func BenchmarkEscapeIndex4KBEscapeHTML(b *testing.B) {
	benchmarkEscapeIndex(b, strings.Repeat("!foobar!", 512), true)
}

func BenchmarkEscapeIndex1(b *testing.B) {
	benchmarkEscapeIndex(b, "1", false)
}

func BenchmarkEscapeIndex1EscapeHTML(b *testing.B) {
	benchmarkEscapeIndex(b, "1", true)
}

func BenchmarkEscapeIndex7(b *testing.B) {
	benchmarkEscapeIndex(b, "1234567", false)
}

func BenchmarkEscapeIndex7EscapeHTML(b *testing.B) {
	benchmarkEscapeIndex(b, "1234567", true)
}

func benchmarkEscapeIndex(b *testing.B, s string, escapeHTML bool) {
	b.ResetTimer()
	for range b.N {
		escapeIndex(s, escapeHTML)
	}
	b.SetBytes(int64(len(s)))
}
