package proto

import (
	"fmt"
	"io"
	"reflect"
	"unsafe"
)

const (
	embedded = 1 << 0
	repeated = 1 << 1
	zigzag   = 1 << 2
)

type structField struct {
	number  uint16
	tagsize uint8
	flags   uint8
	offset  uint32
	codec   *codec
}

func (f *structField) String() string {
	return fmt.Sprintf("[%d,%s]", f.fieldNumber(), f.wireType())
}

func (f *structField) fieldNumber() fieldNumber {
	return fieldNumber(f.number)
}

func (f *structField) wireType() wireType {
	return f.codec.wire
}

func (f *structField) embedded() bool {
	return (f.flags & embedded) != 0
}

func (f *structField) repeated() bool {
	return (f.flags & repeated) != 0
}

func (f *structField) pointer(p unsafe.Pointer) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + uintptr(f.offset))
}

func (f *structField) makeFlags(base flags) flags {
	return base | flags(f.flags&zigzag)
}

func structCodecOf(t reflect.Type, seen map[reflect.Type]*codec) *codec {
	c := &codec{wire: varlen}
	seen[t] = c

	numField := t.NumField()
	number := fieldNumber(1)
	fields := make([]structField, 0, numField)

	for i := range numField {
		f := t.Field(i)

		if f.PkgPath != "" {
			continue // unexported
		}

		field := structField{
			number: uint16(number),
			offset: uint32(f.Offset),
		}

		if tag, ok := f.Tag.Lookup("protobuf"); ok {
			t, err := parseStructTag(tag)
			if err == nil {
				field.number = uint16(t.fieldNumber)
				if t.repeated {
					field.flags |= repeated
				}
				if t.zigzag {
					field.flags |= zigzag
				}
				switch t.wireType {
				case Fixed32:
					switch baseKindOf(f.Type) {
					case reflect.Uint32:
						field.codec = &fixed32Codec
					case reflect.Float32:
						field.codec = &float32Codec
					}
				case Fixed64:
					switch baseKindOf(f.Type) {
					case reflect.Uint64:
						field.codec = &fixed64Codec
					case reflect.Float64:
						field.codec = &float64Codec
					}
				}
			}
		}

		if field.codec == nil {
			switch baseKindOf(f.Type) {
			case reflect.Struct:
				field.flags |= embedded
				field.codec = codecOf(f.Type, seen)

			case reflect.Slice:
				elem := f.Type.Elem()

				if elem.Kind() == reflect.Uint8 { // []byte
					field.codec = codecOf(f.Type, seen)
				} else {
					if baseKindOf(elem) == reflect.Struct {
						field.flags |= embedded
					}
					field.flags |= repeated
					field.codec = codecOf(elem, seen)
					field.codec = sliceCodecOf(f.Type, field, seen)
				}

			case reflect.Map:
				key, val := f.Type.Key(), f.Type.Elem()
				k := codecOf(key, seen)
				v := codecOf(val, seen)
				m := &mapField{
					number:   field.number,
					keyCodec: k,
					valCodec: v,
				}
				if baseKindOf(key) == reflect.Struct {
					m.keyFlags |= embedded
				}
				if baseKindOf(val) == reflect.Struct {
					m.valFlags |= embedded
				}
				field.flags |= embedded | repeated
				field.codec = mapCodecOf(f.Type, m, seen)

			default:
				field.codec = codecOf(f.Type, seen)
			}
		}

		field.tagsize = uint8(sizeOfTag(fieldNumber(field.number), wireType(field.codec.wire)))
		fields = append(fields, field)
		number++
	}

	c.size = structSizeFuncOf(t, fields)
	c.encode = structEncodeFuncOf(t, fields)
	c.decode = structDecodeFuncOf(t, fields)
	return c
}

func baseKindOf(t reflect.Type) reflect.Kind {
	return baseTypeOf(t).Kind()
}

func baseTypeOf(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func structSizeFuncOf(t reflect.Type, fields []structField) sizeFunc {
	inlined := inlined(t)
	var unique, repeated []*structField

	for i := range fields {
		f := &fields[i]
		if f.repeated() {
			repeated = append(repeated, f)
		} else {
			unique = append(unique, f)
		}
	}

	return func(p unsafe.Pointer, flags flags) int {
		if p == nil {
			return 0
		}

		if !inlined {
			flags = flags.without(inline | toplevel)
		} else {
			flags = flags.without(toplevel)
		}
		n := 0

		for _, f := range unique {
			size := f.codec.size(f.pointer(p), f.makeFlags(flags))
			if size > 0 {
				n += int(f.tagsize) + size
				if f.embedded() {
					n += sizeOfVarint(uint64(size))
				}
				flags = flags.without(wantzero)
			}
		}

		for _, f := range repeated {
			size := f.codec.size(f.pointer(p), f.makeFlags(flags))
			if size > 0 {
				n += size
				flags = flags.without(wantzero)
			}
		}

		return n
	}
}

func structEncodeFuncOf(t reflect.Type, fields []structField) encodeFunc {
	inlined := inlined(t)
	var unique, repeated []*structField

	for i := range fields {
		f := &fields[i]
		if f.repeated() {
			repeated = append(repeated, f)
		} else {
			unique = append(unique, f)
		}
	}

	return func(b []byte, p unsafe.Pointer, flags flags) (int, error) {
		if p == nil {
			return 0, nil
		}

		if !inlined {
			flags = flags.without(inline | toplevel)
		} else {
			flags = flags.without(toplevel)
		}
		offset := 0

		for _, f := range unique {
			fieldFlags := f.makeFlags(flags)
			elem := f.pointer(p)
			size := f.codec.size(elem, fieldFlags)

			if size > 0 {
				n, err := encodeTag(b[offset:], f.fieldNumber(), f.wireType())
				offset += n
				if err != nil {
					return offset, err
				}

				if f.embedded() {
					n, err := encodeVarint(b[offset:], uint64(size))
					offset += n
					if err != nil {
						return offset, err
					}
				}

				if (len(b) - offset) < size {
					return len(b), io.ErrShortBuffer
				}

				n, err = f.codec.encode(b[offset:offset+size], elem, fieldFlags)
				offset += n
				if err != nil {
					return offset, err
				}

				flags = flags.without(wantzero)
			}
		}

		for _, f := range repeated {
			n, err := f.codec.encode(b[offset:], f.pointer(p), f.makeFlags(flags))
			offset += n
			if err != nil {
				return offset, err
			}
			if n > 0 {
				flags = flags.without(wantzero)
			}
		}

		return offset, nil
	}
}

func structDecodeFuncOf(t reflect.Type, fields []structField) decodeFunc {
	maxFieldNumber := fieldNumber(0)

	for _, f := range fields {
		if n := f.fieldNumber(); n > maxFieldNumber {
			maxFieldNumber = n
		}
	}

	fieldIndex := make([]*structField, maxFieldNumber+1)

	for i := range fields {
		f := &fields[i]
		fieldIndex[f.fieldNumber()] = f
	}

	return func(b []byte, p unsafe.Pointer, flags flags) (int, error) {
		flags = flags.without(toplevel)
		offset := 0

		for offset < len(b) {
			fieldNumber, wireType, n, err := decodeTag(b[offset:])
			offset += n
			if err != nil {
				return offset, err
			}

			i := int(fieldNumber)
			f := (*structField)(nil)

			if i >= 0 && i < len(fieldIndex) {
				f = fieldIndex[i]
			}

			if f == nil {
				skip := 0
				size := uint64(0)
				switch wireType {
				case varint:
					_, skip, err = decodeVarint(b[offset:])
				case varlen:
					size, skip, err = decodeVarint(b[offset:])
					if err == nil {
						if size > uint64(len(b)-skip) {
							err = io.ErrUnexpectedEOF
						} else {
							skip += int(size)
						}
					}
				case fixed32:
					_, skip, err = decodeLE32(b[offset:])
				case fixed64:
					_, skip, err = decodeLE64(b[offset:])
				default:
					err = ErrWireTypeUnknown
				}
				if (offset + skip) <= len(b) {
					offset += skip
				} else {
					offset, err = len(b), io.ErrUnexpectedEOF
				}
				if err != nil {
					return offset, fieldError(fieldNumber, wireType, err)
				}
				continue
			}

			if wireType != f.wireType() {
				return offset, fieldError(fieldNumber, wireType, fmt.Errorf("expected wire type %d", f.wireType()))
			}

			// `data` will only contain the section of the input buffer where
			// the data for the next field is available. This is necessary to
			// limit how many bytes will be consumed by embedded messages.
			var data []byte
			switch wireType {
			case varint:
				_, n, err := decodeVarint(b[offset:])
				if err != nil {
					return offset, fieldError(fieldNumber, wireType, err)
				}
				data = b[offset : offset+n]

			case varlen:
				l, n, err := decodeVarint(b[offset:])
				if err != nil {
					return offset + n, fieldError(fieldNumber, wireType, err)
				}
				if l > uint64(len(b)-(offset+n)) {
					return len(b), fieldError(fieldNumber, wireType, io.ErrUnexpectedEOF)
				}
				if f.embedded() {
					offset += n
					data = b[offset : offset+int(l)]
				} else {
					data = b[offset : offset+n+int(l)]
				}

			case fixed32:
				if (offset + 4) > len(b) {
					return len(b), fieldError(fieldNumber, wireType, io.ErrUnexpectedEOF)
				}
				data = b[offset : offset+4]

			case fixed64:
				if (offset + 8) > len(b) {
					return len(b), fieldError(fieldNumber, wireType, io.ErrUnexpectedEOF)
				}
				data = b[offset : offset+8]

			default:
				return offset, fieldError(fieldNumber, wireType, ErrWireTypeUnknown)
			}

			n, err = f.codec.decode(data, f.pointer(p), f.makeFlags(flags))
			offset += n
			if err != nil {
				return offset, fieldError(fieldNumber, wireType, err)
			}
		}

		return offset, nil
	}
}
