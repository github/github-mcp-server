package proto

import (
	"unsafe"
)

var intCodec = codec{
	wire:   varint,
	size:   sizeOfInt,
	encode: encodeInt,
	decode: decodeInt,
}

func sizeOfInt(p unsafe.Pointer, flags flags) int {
	if p != nil {
		if v := *(*int)(p); v != 0 || flags.has(wantzero) {
			return sizeOfVarint(flags.uint64(int64(v)))
		}
	}
	return 0
}

func encodeInt(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	if p != nil {
		if v := *(*int)(p); v != 0 || flags.has(wantzero) {
			return encodeVarint(b, flags.uint64(int64(v)))
		}
	}
	return 0, nil
}

func decodeInt(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	v, n, err := decodeVarint(b)
	*(*int)(p) = int(flags.int64(v))
	return n, err
}
