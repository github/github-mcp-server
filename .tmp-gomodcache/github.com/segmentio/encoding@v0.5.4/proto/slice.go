package proto

import (
	"io"
	"reflect"
	"unsafe"

	. "github.com/segmentio/encoding/internal/runtime_reflect"
)

type repeatedField struct {
	codec       *codec
	fieldNumber fieldNumber
	wireType    wireType
	embedded    bool
}

func sliceCodecOf(t reflect.Type, f structField, seen map[reflect.Type]*codec) *codec {
	s := new(codec)
	seen[t] = s

	r := &repeatedField{
		codec:       f.codec,
		fieldNumber: f.fieldNumber(),
		wireType:    f.wireType(),
		embedded:    f.embedded(),
	}

	s.wire = f.codec.wire
	s.size = sliceSizeFuncOf(t, r)
	s.encode = sliceEncodeFuncOf(t, r)
	s.decode = sliceDecodeFuncOf(t, r)
	return s
}

func sliceSizeFuncOf(t reflect.Type, r *repeatedField) sizeFunc {
	elemSize := alignedSize(t.Elem())
	tagSize := sizeOfTag(r.fieldNumber, r.wireType)
	return func(p unsafe.Pointer, _ flags) int {
		n := 0

		if v := (*Slice)(p); v != nil {
			for i := range v.Len() {
				elem := v.Index(i, elemSize)
				size := r.codec.size(elem, wantzero)
				n += tagSize + size
				if r.embedded {
					n += sizeOfVarint(uint64(size))
				}
			}
		}

		return n
	}
}

func sliceEncodeFuncOf(t reflect.Type, r *repeatedField) encodeFunc {
	elemSize := alignedSize(t.Elem())
	tagSize := sizeOfTag(r.fieldNumber, r.wireType)
	tagData := make([]byte, tagSize)
	encodeTag(tagData, r.fieldNumber, r.wireType)
	return func(b []byte, p unsafe.Pointer, _ flags) (int, error) {
		offset := 0

		if s := (*Slice)(p); s != nil {
			for i := range s.Len() {
				elem := s.Index(i, elemSize)
				size := r.codec.size(elem, wantzero)

				n := copy(b[offset:], tagData)
				offset += n
				if n < len(tagData) {
					return offset, io.ErrShortBuffer
				}

				if r.embedded {
					n, err := encodeVarint(b[offset:], uint64(size))
					offset += n
					if err != nil {
						return offset, err
					}
				}

				if (len(b) - offset) < size {
					return len(b), io.ErrShortBuffer
				}

				n, err := r.codec.encode(b[offset:offset+size], elem, wantzero)
				offset += n
				if err != nil {
					return offset, err
				}
			}
		}

		return offset, nil
	}
}

func sliceDecodeFuncOf(t reflect.Type, r *repeatedField) decodeFunc {
	elemType := t.Elem()
	elemSize := alignedSize(elemType)
	return func(b []byte, p unsafe.Pointer, _ flags) (int, error) {
		s := (*Slice)(p)
		i := s.Len()

		if i == s.Cap() {
			*s = growSlice(elemType, s)
		}

		n, err := r.codec.decode(b, s.Index(i, elemSize), noflags)
		if err == nil {
			s.SetLen(i + 1)
		}
		return n, err
	}
}

func alignedSize(t reflect.Type) uintptr {
	a := t.Align()
	s := t.Size()
	return align(uintptr(a), uintptr(s))
}

func align(align, size uintptr) uintptr {
	if align != 0 && (size%align) != 0 {
		size = ((size / align) + 1) * align
	}
	return size
}

func growSlice(t reflect.Type, s *Slice) Slice {
	cap := 2 * s.Cap()
	if cap == 0 {
		cap = 10
	}
	p := pointer(t)
	d := MakeSlice(p, s.Len(), cap)
	CopySlice(p, d, *s)
	return d
}
