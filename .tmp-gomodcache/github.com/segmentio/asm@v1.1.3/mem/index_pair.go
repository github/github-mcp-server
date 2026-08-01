package mem

import (
	"bytes"
)

// IndexPair returns the byte index of the first pair of two equal elements of
// size n.
//
// If no pairs of equal elements were found, -1 is returned.
func IndexPair(b []byte, n int) int {
	if len(b)%n != 0 {
		panic("input length is not a multiple of the item size")
	}
	// Delegate to indexPair to keep the function cost low and allow the size
	// check to be inlined and the modulo optimized away for power of two sizes
	// known at compile time.
	return indexPair(b, n)
}

func indexPair(b []byte, n int) int {
	switch n {
	case 1:
		return indexPair1(b)
	case 2:
		return indexPair2(b)
	case 4:
		return indexPair4(b)
	case 8:
		return indexPair8(b)
	case 16:
		return indexPair16(b)
	case 32:
		return indexPair32(b)
	default:
		return indexPairGeneric(b, n)
	}
}

func indexPairGeneric(b []byte, n int) int {
	for i := n; i < len(b); i += n {
		if bytes.Equal(b[i-n:i], b[i:i+n]) {
			return i - n
		}
	}
	return -1
}
