package proto

import (
	"io"
	"reflect"
	"sync"
	"unsafe"

	. "github.com/segmentio/encoding/internal/runtime_reflect"
)

const (
	zeroSize = 1 // sizeOfVarint(0)
)

type mapField struct {
	number   uint16
	keyFlags uint8
	valFlags uint8
	keyCodec *codec
	valCodec *codec
}

func mapCodecOf(t reflect.Type, f *mapField, seen map[reflect.Type]*codec) *codec {
	m := new(codec)
	seen[t] = m

	m.wire = varlen
	m.size = mapSizeFuncOf(t, f)
	m.encode = mapEncodeFuncOf(t, f)
	m.decode = mapDecodeFuncOf(t, f, seen)
	return m
}

func mapSizeFuncOf(t reflect.Type, f *mapField) sizeFunc {
	mapTagSize := sizeOfTag(fieldNumber(f.number), varlen)
	keyTagSize := sizeOfTag(1, wireType(f.keyCodec.wire))
	valTagSize := sizeOfTag(2, wireType(f.valCodec.wire))
	return func(p unsafe.Pointer, flags flags) int {
		if p == nil {
			return 0
		}

		if !flags.has(inline) {
			p = *(*unsafe.Pointer)(p)
		}

		n := 0
		m := MapIter{}
		defer m.Done()

		for m.Init(pointer(t), p); m.HasNext(); m.Next() {
			keySize := f.keyCodec.size(m.Key(), wantzero)
			valSize := f.valCodec.size(m.Value(), wantzero)

			if keySize > 0 {
				n += keyTagSize + keySize
				if (f.keyFlags & embedded) != 0 {
					n += sizeOfVarint(uint64(keySize))
				}
			}

			if valSize > 0 {
				n += valTagSize + valSize
				if (f.valFlags & embedded) != 0 {
					n += sizeOfVarint(uint64(valSize))
				}
			}

			n += mapTagSize + sizeOfVarint(uint64(keySize+valSize))
		}

		if n == 0 {
			n = mapTagSize + zeroSize
		}

		return n
	}
}

func mapEncodeFuncOf(t reflect.Type, f *mapField) encodeFunc {
	keyTag := [1]byte{}
	valTag := [1]byte{}
	encodeTag(keyTag[:], 1, f.keyCodec.wire)
	encodeTag(valTag[:], 2, f.valCodec.wire)

	number := fieldNumber(f.number)
	mapTag := make([]byte, sizeOfTag(number, varlen)+zeroSize)
	encodeTag(mapTag, number, varlen)

	zero := mapTag
	mapTag = mapTag[:len(mapTag)-1]

	return func(b []byte, p unsafe.Pointer, flags flags) (int, error) {
		if p == nil {
			return 0, nil
		}

		if !flags.has(inline) {
			p = *(*unsafe.Pointer)(p)
		}

		offset := 0
		m := MapIter{}
		defer m.Done()

		for m.Init(pointer(t), p); m.HasNext(); m.Next() {
			key := m.Key()
			val := m.Value()

			keySize := f.keyCodec.size(key, wantzero)
			valSize := f.valCodec.size(val, wantzero)
			elemSize := keySize + valSize

			if keySize > 0 {
				elemSize += len(keyTag)
				if (f.keyFlags & embedded) != 0 {
					elemSize += sizeOfVarint(uint64(keySize))
				}
			}

			if valSize > 0 {
				elemSize += len(valTag)
				if (f.valFlags & embedded) != 0 {
					elemSize += sizeOfVarint(uint64(valSize))
				}
			}

			n := copy(b[offset:], mapTag)
			offset += n
			if n < len(mapTag) {
				return offset, io.ErrShortBuffer
			}
			n, err := encodeVarint(b[offset:], uint64(elemSize))
			offset += n
			if err != nil {
				return offset, err
			}

			if keySize > 0 {
				n := copy(b[offset:], keyTag[:])
				offset += n
				if n < len(keyTag) {
					return offset, io.ErrShortBuffer
				}

				if (f.keyFlags & embedded) != 0 {
					n, err := encodeVarint(b[offset:], uint64(keySize))
					offset += n
					if err != nil {
						return offset, err
					}
				}

				if (len(b) - offset) < keySize {
					return len(b), io.ErrShortBuffer
				}

				n, err := f.keyCodec.encode(b[offset:offset+keySize], key, wantzero)
				offset += n
				if err != nil {
					return offset, err
				}
			}

			if valSize > 0 {
				n := copy(b[offset:], valTag[:])
				offset += n
				if n < len(valTag) {
					return n, io.ErrShortBuffer
				}

				if (f.valFlags & embedded) != 0 {
					n, err := encodeVarint(b[offset:], uint64(valSize))
					offset += n
					if err != nil {
						return offset, err
					}
				}

				if (len(b) - offset) < valSize {
					return len(b), io.ErrShortBuffer
				}

				n, err := f.valCodec.encode(b[offset:offset+valSize], val, wantzero)
				offset += n
				if err != nil {
					return offset, err
				}
			}
		}

		if offset == 0 {
			if offset = copy(b, zero); offset < len(zero) {
				return offset, io.ErrShortBuffer
			}
		}

		return offset, nil
	}
}

func mapDecodeFuncOf(t reflect.Type, f *mapField, seen map[reflect.Type]*codec) decodeFunc {
	structType := reflect.StructOf([]reflect.StructField{
		{Name: "Key", Type: t.Key()},
		{Name: "Elem", Type: t.Elem()},
	})

	structCodec := codecOf(structType, seen)
	structPool := new(sync.Pool)
	structZero := pointer(reflect.Zero(structType).Interface())

	valueType := t.Elem()
	valueOffset := structType.Field(1).Offset

	mtype := pointer(t)
	stype := pointer(structType)
	vtype := pointer(valueType)

	return func(b []byte, p unsafe.Pointer, _ flags) (int, error) {
		m := (*unsafe.Pointer)(p)
		if *m == nil {
			*m = MakeMap(mtype, 10)
		}
		if len(b) == 0 {
			return 0, nil
		}

		s := pointer(structPool.Get())
		if s == nil {
			s = unsafe.Pointer(reflect.New(structType).Pointer())
		}

		n, err := structCodec.decode(b, s, noflags)
		if err == nil {
			v := MapAssign(mtype, *m, s)
			Assign(vtype, v, unsafe.Pointer(uintptr(s)+valueOffset))
		}

		Assign(stype, s, structZero)
		structPool.Put(s)
		return n, err
	}
}
