package bswap

import (
	"encoding/binary"
	"io"
	"math/rand"
	"testing"
)

func TestSwap64(t *testing.T) {
	input := make([]byte, 4096)
	prng := rand.New(rand.NewSource(0))
	io.ReadFull(prng, input)

	output := make([]byte, 4096)

	for i := 0; i < 4096; i += 8 {
		copy(output, input)
		Swap64(output[:i])
		for j := 0; j < i; j += 8 {
			u1 := binary.BigEndian.Uint64(input[j:])
			u2 := binary.LittleEndian.Uint64(output[j:])
			if u1 != u2 {
				t.Fatalf("bytes weren't swapped at offset %d: %v / %v", i, u1, u2)
			}
		}
	}
}

func BenchmarkSwap64(b *testing.B) {
	input := make([]byte, 64*1024)
	prng := rand.New(rand.NewSource(0))
	io.ReadFull(prng, input)

	b.SetBytes(int64(len(input)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Swap64(input)
	}
}
