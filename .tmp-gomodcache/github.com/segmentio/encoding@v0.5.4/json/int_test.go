package json

import (
	"math"
	"strconv"
	"testing"
)

func TestAppendInt(t *testing.T) {
	var ints []int64
	for i := range 64 {
		u := uint64(1) << i
		ints = append(ints, int64(u-1), int64(u), int64(u+1), -int64(u))
	}

	var std, our [20]byte

	for _, i := range ints {
		expected := strconv.AppendInt(std[:], i, 10)
		actual := appendInt(our[:], i)
		if string(expected) != string(actual) {
			t.Fatalf("appendInt(%d) = %v, expected = %v", i, string(actual), string(expected))
		}
	}
}

func benchStd(b *testing.B, n int64) {
	var buf [20]byte
	b.ResetTimer()
	for range b.N {
		strconv.AppendInt(buf[:0], n, 10)
	}
}

func benchNew(b *testing.B, n int64) {
	var buf [20]byte
	b.ResetTimer()
	for range b.N {
		appendInt(buf[:0], n)
	}
}

func BenchmarkAppendIntStd1(b *testing.B) {
	benchStd(b, 1)
}

func BenchmarkAppendInt1(b *testing.B) {
	benchNew(b, 1)
}

func BenchmarkAppendIntStdMinI64(b *testing.B) {
	benchStd(b, math.MinInt64)
}

func BenchmarkAppendIntMinI64(b *testing.B) {
	benchNew(b, math.MinInt64)
}
