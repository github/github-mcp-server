package proto

import "unsafe"

var uintCodec = codec{
	wire:   varint,
	size:   sizeOfUint,
	encode: encodeUint,
	decode: decodeUint,
}

func sizeOfUint(p unsafe.Pointer, flags flags) int {
	if p != nil {
		if v := *(*uint)(p); v != 0 || flags.has(wantzero) {
			return sizeOfVarint(uint64(v))
		}
	}
	return 0
}

func encodeUint(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	if p != nil {
		if v := *(*uint)(p); v != 0 || flags.has(wantzero) {
			return encodeVarint(b, uint64(v))
		}
	}
	return 0, nil
}

func decodeUint(b []byte, p unsafe.Pointer, _ flags) (int, error) {
	v, n, err := decodeVarint(b)
	*(*uint)(p) = uint(v)
	return n, err
}
