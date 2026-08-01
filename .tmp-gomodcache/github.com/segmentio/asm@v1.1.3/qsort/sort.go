package qsort

import (
	"sort"

	"github.com/segmentio/asm/bswap"
	"github.com/segmentio/asm/cpu"
	"github.com/segmentio/asm/cpu/x86"
	"github.com/segmentio/asm/internal"
)

// Sort sorts contiguous big-endian chunks of bytes of a fixed size.
// Sorting specializations are available for sizes of 8, 16, 24 and 32 bytes.
func Sort(data []byte, size int, swap func(int, int)) {
	if len(data) <= size {
		return
	}
	if size <= 0 || !internal.MultipleOf(size, len(data)) {
		panic("input length is not a multiple of element size")
	}

	// No specialization available. Use the slower generic sorting routine.
	if purego || size%8 != 0 || size > 32 {
		sort.Sort(newGeneric(data, size, swap))
		return
	}

	// Byte swap each qword prior to sorting. Doing a single pass here, and
	// again after the sort, is faster than byte swapping during each
	// comparison. The sorting routines have been written to assume that high
	// qwords come before low qwords, and so we're able to use the same
	// Swap64() routine rather than needing separate byte swapping routines
	// for 8, 16, 24, or 32 bytes.
	bswap.Swap64(data)
	defer bswap.Swap64(data)

	// If no indirect swapping is required, try to use the hybrid partitioning scheme from
	// https://blog.reverberate.org/2020/05/29/hoares-rebuttal-bubble-sorts-comeback.html
	switch {
	case swap == nil && size == 8 && cpu.X86.Has(x86.CMOV):
		hybridQuicksort64(unsafeBytesTo64(data))
	case swap == nil && size == 16 && cpu.X86.Has(x86.AVX):
		hybridQuicksort128(unsafeBytesTo128(data))
	case swap == nil && size == 32 && cpu.X86.Has(x86.AVX2):
		hybridQuicksort256(unsafeBytesTo256(data))
	case size == 8:
		quicksort64(unsafeBytesTo64(data), 0, smallCutoff, insertionsort64, hoarePartition64, swap)
	case size == 16:
		quicksort128(unsafeBytesTo128(data), 0, smallCutoff, insertionsort128, hoarePartition128, swap)
	case size == 24:
		quicksort192(unsafeBytesTo192(data), 0, smallCutoff, insertionsort192, hoarePartition192, swap)
	case size == 32:
		quicksort256(unsafeBytesTo256(data), 0, smallCutoff, insertionsort256, hoarePartition256, swap)
	}
}

func hybridQuicksort64(data []uint64) {
	// The hybrid Lomuto/Hoare partition scheme requires scratch space. We
	// allocate some stack space for the task here in this trampoline function,
	// so that we don't pay the stack cost unless necessary.
	var buf [scratchSize]byte
	scratch := unsafeBytesTo64(buf[:])
	partition := func(data []uint64, base int, swap func(int, int)) int {
		return hybridPartition64(data, scratch)
	}
	quicksort64(data, 0, smallCutoff/2, bubblesort64NoSwap2, partition, nil)
}

func hybridQuicksort128(data []uint128) {
	var buf [scratchSize]byte
	scratch := unsafeBytesTo128(buf[:])
	partition := func(data []uint128, base int, swap func(int, int)) int {
		return hybridPartition128(data, scratch)
	}
	quicksort128(data, 0, smallCutoff*2, insertionsort128NoSwap, partition, nil)
}

func hybridQuicksort256(data []uint256) {
	var buf [scratchSize]byte
	scratch := unsafeBytesTo256(buf[:])
	partition := func(data []uint256, base int, swap func(int, int)) int {
		return hybridPartition256(data, scratch)
	}
	quicksort256(data, 0, smallCutoff*2, insertionsort256NoSwap, partition, nil)
}

// The threshold at which log-linear sorting methods switch to
// a quadratic (but cache-friendly) method such as insertionsort.
const smallCutoff = 256

// The amount of stack space to allocate as scratch space when
// using the hybrid Lomuto/Hoare partition scheme.
const scratchSize = 1024

func callswap(base int, swap func(int, int), i, j int) {
	if swap != nil {
		swap(base+i, base+j)
	}
}
