package sortedset

import (
	"fmt"
	"math/rand"
	"testing"
)

var unionSpecializationSizes = []int{16}

var overlapChances = []float64{0, 0.1, 0.5, 1.0}

func TestUnion(t *testing.T) {
	for _, test := range []struct {
		name   string
		a      []byte
		b      []byte
		size   int
		expect []byte
	}{
		{
			name: "empty",
			size: 1,
		},
		{
			name:   "size 1, empty a",
			a:      nil,
			b:      []byte{1, 2, 3, 4, 5},
			size:   1,
			expect: []byte{1, 2, 3, 4, 5},
		},
		{
			name:   "size 1, empty b",
			a:      []byte{1, 2, 3, 4, 5},
			b:      nil,
			size:   1,
			expect: []byte{1, 2, 3, 4, 5},
		},
		{
			name:   "size 1, a == b",
			a:      []byte{1, 2, 3, 4, 5},
			b:      []byte{1, 2, 3, 4, 5},
			size:   1,
			expect: []byte{1, 2, 3, 4, 5},
		},
		{
			name:   "size 1, a < b",
			a:      []byte{1, 2, 3},
			b:      []byte{4, 5, 6},
			size:   1,
			expect: []byte{1, 2, 3, 4, 5, 6},
		},
		{
			name:   "size 1, b < a",
			a:      []byte{4, 5, 6},
			b:      []byte{1, 2, 3},
			size:   1,
			expect: []byte{1, 2, 3, 4, 5, 6},
		},
		{
			name:   "size 1, a <= b",
			a:      []byte{1, 2, 3},
			b:      []byte{3, 4, 5},
			size:   1,
			expect: []byte{1, 2, 3, 4, 5},
		},
		{
			name:   "size 1, b <= a",
			a:      []byte{3, 4, 5},
			b:      []byte{1, 2, 3},
			size:   1,
			expect: []byte{1, 2, 3, 4, 5},
		},
		{
			name:   "size 1, interleaved 1",
			a:      []byte{1, 3, 5},
			b:      []byte{2, 4, 6},
			size:   1,
			expect: []byte{1, 2, 3, 4, 5, 6},
		},
		{
			name:   "size 1, interleaved 2",
			a:      []byte{2, 4, 6},
			b:      []byte{1, 3, 5},
			size:   1,
			expect: []byte{1, 2, 3, 4, 5, 6},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			buf := make([]byte, len(test.a)+len(test.b))
			actual := Union(buf, test.a, test.b, test.size)
			assertArraysEqual(t, test.expect, actual, test.size)
		})
	}

	// Test the specializations.
	for _, size := range unionSpecializationSizes {
		t.Run(fmt.Sprintf("size %d, random", size), func(t *testing.T) {
			const maxCount = 100
			const iterations = 1000

			prng := rand.New(rand.NewSource(0))

			buf := make([]byte, size*maxCount*2)

			for i := 0; i < iterations; i++ {
				count := prng.Intn(maxCount)
				for _, p := range overlapChances {
					setA, setB := randomSortedSetPair(prng, size, count, p)
					actual := Union(buf[:0], setA, setB, size)
					expected := Dedupe(nil, combineArrays(setA, setB, size), size)
					assertArraysEqual(t, expected, actual, size)
				}
			}
		})
	}
}

func BenchmarkUnion(b *testing.B) {
	for _, size := range unionSpecializationSizes {
		for _, p := range overlapChances {
			b.Run(fmt.Sprintf("size %d, with %d%% chance of overlap", size, int(p*100)), func(b *testing.B) {
				const bytes = 64 * 1024
				prng := rand.New(rand.NewSource(0))

				setA, setB := randomSortedSetPair(prng, size, bytes/size, p)

				buf := make([]byte, bytes*2)

				b.SetBytes(int64(bytes * 2))
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					Union(buf[:0], setA, setB, size)
				}
			})
		}
	}
}
