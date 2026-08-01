package proto

import (
	"math"
	"unsafe"
)

var float32Codec = codec{
	wire:   fixed32,
	size:   sizeOfFloat32,
	encode: encodeFloat32,
	decode: decodeFloat32,
}

func sizeOfFloat32(p unsafe.Pointer, flags flags) int {
	if p != nil {
		if v := *(*float32)(p); v != 0 || flags.has(wantzero) || math.Signbit(float64(v)) {
			return 4
		}
	}
	return 0
}

func encodeFloat32(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	if p != nil {
		if v := *(*float32)(p); v != 0 || flags.has(wantzero) || math.Signbit(float64(v)) {
			return encodeLE32(b, math.Float32bits(v))
		}
	}
	return 0, nil
}

func decodeFloat32(b []byte, p unsafe.Pointer, _ flags) (int, error) {
	v, n, err := decodeLE32(b)
	*(*float32)(p) = math.Float32frombits(v)
	return n, err
}
