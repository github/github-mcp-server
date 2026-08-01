package proto

import "unsafe"

var int64Codec = codec{
	wire:   varint,
	size:   sizeOfInt64,
	encode: encodeInt64,
	decode: decodeInt64,
}

func sizeOfInt64(p unsafe.Pointer, flags flags) int {
	if p != nil {
		if v := *(*int64)(p); v != 0 || flags.has(wantzero) {
			return sizeOfVarint(flags.uint64(v))
		}
	}
	return 0
}

func encodeInt64(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	if p != nil {
		if v := *(*int64)(p); v != 0 || flags.has(wantzero) {
			return encodeVarint(b, flags.uint64(v))
		}
	}
	return 0, nil
}

func decodeInt64(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	v, n, err := decodeVarint(b)
	*(*int64)(p) = flags.int64(v)
	return n, err
}
