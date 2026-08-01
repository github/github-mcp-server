package sortedset

import (
	"fmt"
	"math/rand"
	"testing"
)

var dedupeSpecializationSizes = []int{4, 8, 16, 32}

var repeatChances = []float64{0, 0.1, 0.5, 1.0}

func TestDedupe(t *testing.T) {
	for _, size := range []int{1, 2, 3, 4, 8, 10, 16, 32} {

		makeArray := func(items ...byte) []byte {
			array := make([]byte, len(items)*size)
			for i := range items {
				array[i*size] = items[i]
			}
			return array
		}

		for _, test := range []struct {
			name   string
			b      []byte
			expect []byte
		}{
			{
				name: "empty",
			},
			{
				name:   "all dupes",
				b:      makeArray(1, 1, 1, 1, 1, 1, 1, 1),
				expect: makeArray(1),
			},
			{
				name:   "no dupes",
				b:      makeArray(1, 2, 3, 4, 5, 6, 7, 8),
				expect: makeArray(1, 2, 3, 4, 5, 6, 7, 8),
			},
			{
				name:   "some dupes",
				b:      makeArray(0, 0, 0, 1, 1, 2, 3, 3, 4, 4, 4),
				expect: makeArray(0, 1, 2, 3, 4),
			},
		} {
			t.Run(fmt.Sprintf("size %d, %s", size, test.name), func(t *testing.T) {
				actual := Dedupe(nil, test.b, size)
				assertArraysEqual(t, test.expect, actual, size)
			})
		}
	}

	// Test the specializations.
	for _, size := range dedupeSpecializationSizes {
		t.Run(fmt.Sprintf("size %d, random", size), func(t *testing.T) {
			const maxCount = 100
			const iterations = 1000

			prng := rand.New(rand.NewSource(0))

			for i := 0; i < iterations; i++ {
				count := prng.Intn(maxCount)
				for _, p := range repeatChances {
					array, uniques := randomSortedArray(prng, size, count, p)
					result := Dedupe(nil, array, size)
					assertArraysEqual(t, uniques, result, size)
				}
			}
		})
	}
}

func BenchmarkDedupe(b *testing.B) {
	for _, size := range dedupeSpecializationSizes {
		for _, p := range repeatChances {
			b.Run(fmt.Sprintf("size %d, with %d%% chance of repeat", size, int(p*100)), func(b *testing.B) {
				const bytes = 64 * 1024

				prng := rand.New(rand.NewSource(0))

				src, _ := randomSortedArray(prng, size, bytes/size, p)
				buf := make([]byte, len(src))

				b.SetBytes(bytes)
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					//copy(buf, src)
					_ = Dedupe(buf, src, size)
				}
			})
		}
	}
}
