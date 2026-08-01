package mem

import "testing"

func TestMask(t *testing.T) {
	testCopy(t, Mask, maskGeneric)
}

func BenchmarkMask(b *testing.B) {
	benchmarkCopy(b, Mask)
}
