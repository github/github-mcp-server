package proto

import (
	"io"
	"unsafe"
)

var stringCodec = codec{
	wire:   varlen,
	size:   sizeOfString,
	encode: encodeString,
	decode: decodeString,
}

func sizeOfString(p unsafe.Pointer, flags flags) int {
	if p != nil {
		if v := *(*string)(p); v != "" || flags.has(wantzero) {
			return sizeOfVarlen(len(v))
		}
	}
	return 0
}

func encodeString(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	if p != nil {
		if v := *(*string)(p); v != "" || flags.has(wantzero) {
			n, err := encodeVarint(b, uint64(len(v)))
			if err != nil {
				return n, err
			}
			c := copy(b[n:], v)
			n += c
			if c < len(v) {
				err = io.ErrShortBuffer
			}
			return n, err
		}
	}
	return 0, nil
}

func decodeString(b []byte, p unsafe.Pointer, _ flags) (int, error) {
	v, n, err := decodeVarlen(b)
	*(*string)(p) = string(v)
	return n, err
}
