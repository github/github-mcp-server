package mem

import (
	"bytes"
)

// CountPair returns the byte index of the first pair of two equal elements of
// size n.
//
// If no pairs of equal elements were found, len(b) is returned.
func CountPair(b []byte, n int) int {
	if len(b)%n != 0 {
		panic("input length is not a multiple of the item size")
	}
	// Delegate to countPair to keep the function cost low and allow the size
	// check to be inlined and the modulo optimized away for power of two sizes
	// known at compile time.
	return countPair(b, n)
}

func countPair(b []byte, n int) int {
	switch n {
	case 1:
		return countPair1(b)
	case 2:
		return countPair2(b)
	case 4:
		return countPair4(b)
	case 8:
		return countPair8(b)
	case 16:
		return countPair16(b)
	case 32:
		return countPair32(b)
	default:
		return countPairGeneric(b, n)
	}
}

func countPairGeneric(b []byte, n int) int {
	c := 0
	for i := n; i < len(b); i += n {
		if bytes.Equal(b[i-n:i], b[i:i+n]) {
			c++
		}
	}
	return c
}
