package proto

import (
	"fmt"
	"math"
	"unsafe"
)

var uint32Codec = codec{
	wire:   varint,
	size:   sizeOfUint32,
	encode: encodeUint32,
	decode: decodeUint32,
}

func sizeOfUint32(p unsafe.Pointer, flags flags) int {
	if p != nil {
		if v := *(*uint32)(p); v != 0 || flags.has(wantzero) {
			return sizeOfVarint(uint64(v))
		}
	}
	return 0
}

func encodeUint32(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	if p != nil {
		if v := *(*uint32)(p); v != 0 || flags.has(wantzero) {
			return encodeVarint(b, uint64(v))
		}
	}
	return 0, nil
}

func decodeUint32(b []byte, p unsafe.Pointer, _ flags) (int, error) {
	v, n, err := decodeVarint(b)
	if v > math.MaxUint32 {
		return n, fmt.Errorf("integer overflow decoding %v into uint32", v)
	}
	*(*uint32)(p) = uint32(v)
	return n, err
}

var fixed32Codec = codec{
	wire:   fixed32,
	size:   sizeOfFixed32,
	encode: encodeFixed32,
	decode: decodeFixed32,
}

func sizeOfFixed32(p unsafe.Pointer, flags flags) int {
	if p != nil {
		if v := *(*uint32)(p); v != 0 || flags.has(wantzero) {
			return 4
		}
	}
	return 0
}

func encodeFixed32(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	if p != nil {
		if v := *(*uint32)(p); v != 0 || flags.has(wantzero) {
			return encodeLE32(b, v)
		}
	}
	return 0, nil
}

func decodeFixed32(b []byte, p unsafe.Pointer, _ flags) (int, error) {
	v, n, err := decodeLE32(b)
	*(*uint32)(p) = v
	return n, err
}
