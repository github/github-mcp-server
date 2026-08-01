package sortedset

import (
	"bytes"

	"github.com/segmentio/asm/internal"
)

// Dedupe writes to dst the deduplicated sequence of items of the given size
// read from src, returning the byte slice containing the result.
//
// If dst is too small, a new slice is allocated and returned instead.
//
// The source and destination slices may be the same to perform in-place
// deduplication of the elements. The behavior is undefined for any other
// conditions where the source and destination slices overlap.
//
// The function panics if len(src) is not a multiple of the element size.
func Dedupe(dst, src []byte, size int) []byte {
	if !internal.MultipleOf(size, len(src)) {
		panic("input length is not a multiple of the item size")
	}
	if len(dst) < len(src) {
		dst = make([]byte, len(src))
	}
	var n int
	switch size {
	case 1:
		n = dedupe1(dst, src)
	case 2:
		n = dedupe2(dst, src)
	case 4:
		n = dedupe4(dst, src)
	case 8:
		n = dedupe8(dst, src)
	case 16:
		n = dedupe16(dst, src)
	case 32:
		n = dedupe32(dst, src)
	default:
		n = dedupeGeneric(dst, src, size)
	}
	return dst[:n]
}

func dedupeGeneric(dst, src []byte, size int) int {
	if len(src) == 0 {
		return 0
	}

	i := size
	j := size
	copy(dst, src[:size])

	for i < len(src) {
		if !bytes.Equal(src[i-size:i], src[i:i+size]) {
			copy(dst[j:], src[i:i+size])
			j += size
		}
		i += size
	}

	return j
}
