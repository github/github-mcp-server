package proto

import (
	"io"
	"reflect"
	"unsafe"
)

func customCodecOf(t reflect.Type) *codec {
	return &codec{
		wire:   varlen,
		size:   customSizeFuncOf(t),
		encode: customEncodeFuncOf(t),
		decode: customDecodeFuncOf(t),
	}
}

func customSizeFuncOf(t reflect.Type) sizeFunc {
	return func(p unsafe.Pointer, flags flags) int {
		if p != nil {
			if m := reflect.NewAt(t, p).Interface().(customMessage); m != nil {
				size := m.Size()
				if flags.has(toplevel) {
					return size
				}
				return sizeOfVarlen(size)
			}
		}
		return 0
	}
}

func customEncodeFuncOf(t reflect.Type) encodeFunc {
	return func(b []byte, p unsafe.Pointer, flags flags) (int, error) {
		if p != nil {
			if m := reflect.NewAt(t, p).Interface().(customMessage); m != nil {
				size := m.Size()

				if flags.has(toplevel) {
					if len(b) < size {
						return 0, io.ErrShortBuffer
					}
					return m.MarshalTo(b)
				}

				vlen := sizeOfVarlen(size)
				if len(b) < vlen {
					return 0, io.ErrShortBuffer
				}

				n1, err := encodeVarint(b, uint64(size))
				if err != nil {
					return n1, err
				}

				n2, err := m.MarshalTo(b[n1:])
				return n1 + n2, err
			}
		}
		return 0, nil
	}
}

func customDecodeFuncOf(t reflect.Type) decodeFunc {
	return func(b []byte, p unsafe.Pointer, flags flags) (int, error) {
		m := reflect.NewAt(t, p).Interface().(customMessage)

		if flags.has(toplevel) {
			return len(b), m.Unmarshal(b)
		}

		v, n, err := decodeVarlen(b)
		if err != nil {
			return n, err
		}

		return n + len(v), m.Unmarshal(v)
	}
}
