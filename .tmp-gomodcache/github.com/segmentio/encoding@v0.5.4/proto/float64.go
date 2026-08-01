package proto

import (
	"math"
	"unsafe"
)

var float64Codec = codec{
	wire:   fixed64,
	size:   sizeOfFloat64,
	encode: encodeFloat64,
	decode: decodeFloat64,
}

func sizeOfFloat64(p unsafe.Pointer, flags flags) int {
	if p != nil {
		if v := *(*float64)(p); v != 0 || flags.has(wantzero) || math.Signbit(v) {
			return 8
		}
	}
	return 0
}

func encodeFloat64(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	if p != nil {
		if v := *(*float64)(p); v != 0 || flags.has(wantzero) || math.Signbit(v) {
			return encodeLE64(b, math.Float64bits(v))
		}
	}
	return 0, nil
}

func decodeFloat64(b []byte, p unsafe.Pointer, _ flags) (int, error) {
	v, n, err := decodeLE64(b)
	*(*float64)(p) = math.Float64frombits(v)
	return n, err
}
