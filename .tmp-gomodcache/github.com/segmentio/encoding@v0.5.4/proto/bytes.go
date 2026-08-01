package proto

import (
	"fmt"
	"io"
	"reflect"
	"unsafe"
)

var bytesCodec = codec{
	wire:   varlen,
	size:   sizeOfBytes,
	encode: encodeBytes,
	decode: decodeBytes,
}

func sizeOfBytes(p unsafe.Pointer, flags flags) int {
	if p != nil {
		if v := *(*[]byte)(p); v != nil || flags.has(wantzero) {
			return sizeOfVarlen(len(v))
		}
	}
	return 0
}

func encodeBytes(b []byte, p unsafe.Pointer, flags flags) (int, error) {
	if p != nil {
		if v := *(*[]byte)(p); v != nil || flags.has(wantzero) {
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

func decodeBytes(b []byte, p unsafe.Pointer, _ flags) (int, error) {
	v, n, err := decodeVarlen(b)
	pb := (*[]byte)(p)
	if *pb == nil {
		*pb = make([]byte, 0, len(v))
	}
	*pb = append((*pb)[:0], v...)
	return n, err
}

func makeBytes(p unsafe.Pointer, n int) []byte {
	return *(*[]byte)(unsafe.Pointer(&sliceHeader{
		Data: p,
		Len:  n,
		Cap:  n,
	}))
}

type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

// isZeroBytes is an optimized version of this loop:
//
//	for i := range b {
//		if b[i] != 0 {
//			return false
//		}
//	}
//	return true
//
// This implementation significantly reduces the CPU footprint of checking for
// slices to be zero, especially when the length increases (these cases should
// be rare tho).
//
// name            old time/op  new time/op  delta
// IsZeroBytes0    1.78ns ± 1%  2.29ns ± 4%  +28.65%  (p=0.000 n=8+10)
// IsZeroBytes4    3.17ns ± 3%  2.37ns ± 3%  -25.21%  (p=0.000 n=10+10)
// IsZeroBytes7    3.97ns ± 4%  3.26ns ± 3%  -18.02%  (p=0.000 n=10+10)
// IsZeroBytes64K  14.8µs ± 3%   1.9µs ± 3%  -87.34%  (p=0.000 n=10+10)
func isZeroBytes(b []byte) bool {
	if n := len(b) / 8; n != 0 {
		if !isZeroUint64(*(*[]uint64)(unsafe.Pointer(&sliceHeader{
			Data: unsafe.Pointer(&b[0]),
			Len:  n,
			Cap:  n,
		}))) {
			return false
		}
		b = b[n*8:]
	}
	switch len(b) {
	case 7:
		return bto32(b) == 0 && bto16(b[4:]) == 0 && b[6] == 0
	case 6:
		return bto32(b) == 0 && bto16(b[4:]) == 0
	case 5:
		return bto32(b) == 0 && b[4] == 0
	case 4:
		return bto32(b) == 0
	case 3:
		return bto16(b) == 0 && b[2] == 0
	case 2:
		return bto16(b) == 0
	case 1:
		return b[0] == 0
	default:
		return true
	}
}

func bto32(b []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&b[0]))
}

func bto16(b []byte) uint16 {
	return *(*uint16)(unsafe.Pointer(&b[0]))
}

func isZeroUint64(b []uint64) bool {
	for i := range b {
		if b[i] != 0 {
			return false
		}
	}
	return true
}

func byteArrayCodecOf(t reflect.Type, seen map[reflect.Type]*codec) *codec {
	n := t.Len()
	c := &codec{
		wire:   varlen,
		size:   byteArraySizeFuncOf(n),
		encode: byteArrayEncodeFuncOf(n),
		decode: byteArrayDecodeFuncOf(n),
	}
	seen[t] = c
	return c
}

func byteArraySizeFuncOf(n int) sizeFunc {
	size := sizeOfVarlen(n)
	return func(p unsafe.Pointer, flags flags) int {
		if p != nil && (flags.has(wantzero) || !isZeroBytes(makeBytes(p, n))) {
			return size
		}
		return 0
	}
}

func byteArrayEncodeFuncOf(n int) encodeFunc {
	return func(b []byte, p unsafe.Pointer, flags flags) (int, error) {
		if p != nil {
			if v := makeBytes(p, n); flags.has(wantzero) || !isZeroBytes(v) {
				return encodeBytes(b, unsafe.Pointer(&v), noflags)
			}
		}
		return 0, nil
	}
}

func byteArrayDecodeFuncOf(n int) decodeFunc {
	return func(b []byte, p unsafe.Pointer, _ flags) (int, error) {
		v, r, err := decodeVarlen(b)
		if err == nil {
			if copy(makeBytes(p, n), v) != n {
				err = fmt.Errorf("cannot decode byte sequence of size %d into byte array of size %d", len(v), n)
			}
		}
		return r, err
	}
}
