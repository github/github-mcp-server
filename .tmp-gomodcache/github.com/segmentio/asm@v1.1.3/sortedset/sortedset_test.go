package sortedset

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"sort"
	"testing"
)

func assertArraysEqual(t *testing.T, expected, actual []byte, size int) {
	t.Helper()

	if !bytes.Equal(expected, actual) {
		t.Logf("\nexpected (%d):\n%s\nfound (%d):\n%s",
			len(expected), hex.Dump(expected),
			len(actual), hex.Dump(actual))
		t.Fatal("arrays are not equal")
	}
}

func randomSortedArray(prng *rand.Rand, size int, count int, repeatChance float64) (array []byte, uniques []byte) {
	if count == 0 {
		return nil, nil
	}

	// Generate `count` random chunks of `size` bytes and then sort them.
	pool := make([]byte, size*count)
	prng.Read(pool)
	sortArray(pool, size)

	// Sanity checks — the items must be unique and sorted.
	for i := size; i < len(pool); i += size {
		switch bytes.Compare(pool[i-size:i], pool[i:i+size]) {
		case 0:
			panic("duplicate item in pool")
		case 1:
			panic("not sorted correctly")
		}
	}

	array = make([]byte, 0, size*count)

	// Build an array from the pool of unique items, using the configurable
	// chance of repeat. A repeatChance of 0 will yield an array where every
	// item is unique, while a repeatChance of 1 will yield an array where
	// every item is a duplicate of the first item.
	uniq := size
	for i := 0; i < count; i++ {
		array = append(array, pool[uniq-size:uniq]...)
		if prng.Float64() >= repeatChance && i != count-1 {
			uniq += size
		}
	}

	// Return a second array with just the unique items.
	uniques = pool[:uniq]
	return
}

func randomSortedSet(prng *rand.Rand, size int, count int) []byte {
	_, set := randomSortedArray(prng, size, count, 0.0)
	return set
}

func randomSortedSetPair(prng *rand.Rand, size int, count int, overlapChance float64) ([]byte, []byte) {
	setA := randomSortedSet(prng, size, count)
	setB := randomSortedSet(prng, size, count)

	// Sanity check: there must be no duplicates.
	if len(combineArrays(setA, setB, size)) != count*size*2 {
		panic("sorted sets overlap")
	}

	// Build a new set by taking items from both setA and setB depending
	// on the value of overlapChance.
	split := int(float64(count)*overlapChance) * size
	overlap := combineArrays(setA[:split], setB[:len(setB)-split], size)

	return setA, overlap
}

func combineArrays(a, b []byte, size int) []byte {
	return sortArray(append(append([]byte{}, a...), b...), size)
}

func sortArray(b []byte, size int) []byte {
	sort.Sort(&chunks{b: b, size: size})
	return b
}

type chunks struct {
	b    []byte
	size int
	tmp  []byte
}

func (s *chunks) Len() int {
	return len(s.b) / s.size
}

func (s *chunks) Less(i, j int) bool {
	return bytes.Compare(s.slice(i), s.slice(j)) < 0
}

func (s *chunks) Swap(i, j int) {
	tmp := make([]byte, s.size)
	copy(tmp, s.slice(j))
	copy(s.slice(j), s.slice(i))
	copy(s.slice(i), tmp)
}

func (s *chunks) slice(i int) []byte {
	return s.b[i*s.size : (i+1)*s.size]
}
