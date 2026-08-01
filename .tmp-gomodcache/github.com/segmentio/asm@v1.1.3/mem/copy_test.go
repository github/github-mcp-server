package mem

import "testing"

func TestCopy(t *testing.T) {
	testCopy(t, Copy, copyGeneric)
}

func BenchmarkCopy(b *testing.B) {
	benchmarkCopy(b, Copy)
}
