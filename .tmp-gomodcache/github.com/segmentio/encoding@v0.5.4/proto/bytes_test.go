package proto

import "testing"

func BenchmarkIsZeroBytes0(b *testing.B) {
	benchmarkIsZeroBytes(b, nil)
}

func BenchmarkIsZeroBytes4(b *testing.B) {
	benchmarkIsZeroBytes(b, make([]byte, 4))
}

func BenchmarkIsZeroBytes7(b *testing.B) {
	benchmarkIsZeroBytes(b, make([]byte, 7))
}

func BenchmarkIsZeroBytes64K(b *testing.B) {
	benchmarkIsZeroBytes(b, make([]byte, 64*1024))
}

func benchmarkIsZeroBytes(b *testing.B, slice []byte) {
	for range b.N {
		isZeroBytes(slice)
	}
}
