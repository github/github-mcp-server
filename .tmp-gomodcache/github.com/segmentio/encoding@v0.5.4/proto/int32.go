package proto

import (
	"fmt"
	"math"
	"unsafe"
)

var int32Codec = codec{
	wire:   varint,
	size:   sizeOfInt32,
	encode: encodeInt32,
	decode: decodeInt32,
}

func sizeOfInt32(p unsafe.Pointer, flags flags) int {
	if p != nil {
		if v := *(*int32)(p); v != 0 || flags.has(wantzero) {
			return sizeOfVarint(flags.uint64(int64(v)))
		}
	}
	return 0
}

func encodeInt32(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	if p != nil {
		if v := *(*int32)(p); v != 0 || flags.has(wantzero) {
			return encodeVarint(b, flags.uint64(int64(v)))
		}
	}
	return 0, nil
}

func decodeInt32(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	u, n, err := decodeVarint(b)
	v := flags.int64(u)
	if v < math.MinInt32 || v > math.MaxInt32 {
		return n, fmt.Errorf("integer overflow decoding %v into int32", v)
	}
	*(*int32)(p) = int32(v)
	return n, err
}
