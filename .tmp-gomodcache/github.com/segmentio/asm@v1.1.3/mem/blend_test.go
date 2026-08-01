package mem

import "testing"

func TestBlend(t *testing.T) {
	testCopy(t, Blend, blendGeneric)
}

func BenchmarkBlend(b *testing.B) {
	benchmarkCopy(b, Blend)
}
