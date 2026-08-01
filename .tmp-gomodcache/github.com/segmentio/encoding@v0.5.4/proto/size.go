package proto

import (
	"math/bits"
	"unsafe"
)

type sizeFunc = func(unsafe.Pointer, flags) int

func sizeOfVarint(v uint64) int {
	return (bits.Len64(v|1) + 6) / 7
}

func sizeOfVarintZigZag(v int64) int {
	return sizeOfVarint((uint64(v) << 1) ^ uint64(v>>63))
}

func sizeOfVarlen(n int) int {
	return sizeOfVarint(uint64(n)) + n
}

func sizeOfTag(f fieldNumber, t wireType) int {
	return sizeOfVarint(uint64(f)<<3 | uint64(t))
}
