package sortedset

import (
	"bytes"

	"github.com/segmentio/asm/cpu"
	"github.com/segmentio/asm/cpu/x86"
	"github.com/segmentio/asm/internal"
)

func Union(dst, a, b []byte, size int) []byte {
	if size <= 0 || !internal.PairMultipleOf(size, len(a), len(b)) {
		panic("input lengths must be a multiple of size")
	}
	if cap(dst) < len(a)+len(b) {
		panic("cap(dst) < len(a)+len(b)")
	}

	// Fast paths for non-overlapping sets.
	switch {
	case len(a) == 0:
		return dst[:copy(dst[:cap(dst)], b)]
	case len(b) == 0:
		return dst[:copy(dst[:cap(dst)], a)]
	case bytes.Compare(a[len(a)-size:], b[:size]) < 0:
		k := copy(dst[:len(a)], a)
		k += copy(dst[k:k+len(b)], b)
		return dst[:k]
	case bytes.Compare(b[len(b)-size:], a[:size]) < 0:
		k := copy(dst[:len(b)], b)
		k += copy(dst[k:k+len(a)], a)
		return dst[:k]
	}

	i, j, k := 0, 0, 0
	switch {
	case size == 16 && cpu.X86.Has(x86.AVX):
		i, j, k = union16(dst, a, b)
	default:
		i, j, k = unionGeneric(dst, a, b, size)
	}

	if i < len(a) {
		k += copy(dst[k:k+len(a)-i], a[i:])
	} else if j < len(b) {
		k += copy(dst[k:k+len(b)-j], b[j:])
	}

	return dst[:k]
}

func unionGeneric(dst, a, b []byte, size int) (i, j, k int) {
	i, j, k = 0, 0, 0
	for i < len(a) && j < len(b) {
		itemA := a[i : i+size]
		itemB := b[j : j+size]
		switch bytes.Compare(itemA, itemB) {
		case 0:
			copy(dst[k:k+size], itemA)
			i += size
			j += size
		case -1:
			copy(dst[k:k+size], itemA)
			i += size
		case 1:
			copy(dst[k:k+size], itemB)
			j += size
		}
		k += size
	}
	return
}
