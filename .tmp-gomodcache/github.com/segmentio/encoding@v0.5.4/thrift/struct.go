package thrift

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type flags int16

const (
	enum     flags = 1 << 0
	union    flags = 1 << 1
	required flags = 1 << 2
	optional flags = 1 << 3
	strict   flags = 1 << 4

	featuresBitOffset  = 8
	useDeltaEncoding   = flags(UseDeltaEncoding) << featuresBitOffset
	coalesceBoolFields = flags(CoalesceBoolFields) << featuresBitOffset

	structFlags   flags = enum | union | required | optional
	encodeFlags   flags = strict | protocolFlags
	decodeFlags   flags = strict | protocolFlags
	protocolFlags flags = useDeltaEncoding | coalesceBoolFields
)

func (f flags) have(x flags) bool {
	return (f & x) == x
}

func (f flags) only(x flags) flags {
	return f & x
}

func (f flags) with(x flags) flags {
	return f | x
}

func (f flags) without(x flags) flags {
	return f & ^x
}

type structField struct {
	typ   reflect.Type
	index []int
	id    int16
	flags flags
}

func forEachStructField(t reflect.Type, index []int, do func(structField)) {
	for i := range t.NumField() {
		f := t.Field(i)

		if f.PkgPath != "" && !f.Anonymous { // unexported
			continue
		}

		fieldIndex := append(index, i)
		fieldIndex = fieldIndex[:len(fieldIndex):len(fieldIndex)]

		if f.Anonymous {
			fieldType := f.Type

			for fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			if fieldType.Kind() == reflect.Struct {
				forEachStructField(fieldType, fieldIndex, do)
				continue
			}
		}

		tag := f.Tag.Get("thrift")
		if tag == "" {
			continue
		}
		tags := strings.Split(tag, ",")
		flags := flags(0)

		for _, opt := range tags[1:] {
			switch opt {
			case "enum":
				flags = flags.with(enum)
			case "union":
				flags = flags.with(union)
			case "required":
				flags = flags.with(required)
			case "optional":
				flags = flags.with(optional)
			default:
				panic(fmt.Errorf("thrift struct field contains an unknown tag option %q in `thrift:\"%s\"`", opt, tag))
			}
		}

		if flags.have(optional | required) {
			panic(fmt.Errorf("thrift struct field cannot be both optional and required in `thrift:\"%s\"`", tag))
		}

		if flags.have(union) {
			if f.Type.Kind() != reflect.Interface {
				panic(fmt.Errorf("thrift union tag found on a field which is not an interface type `thrift:\"%s\"`", tag))
			}

			if tags[0] != "" {
				panic(fmt.Errorf("invalid thrift field id on union field `thrift:\"%s\"`", tag))
			}

			do(structField{
				typ:   f.Type,
				index: fieldIndex,
				flags: flags,
			})
		} else {
			if flags.have(enum) {
				switch f.Type.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				default:
					panic(fmt.Errorf("thrift enum tag found on a field which is not an integer type `thrift:\"%s\"`", tag))
				}
			}

			if id, err := strconv.ParseInt(tags[0], 10, 16); err != nil {
				panic(fmt.Errorf("invalid thrift field id found in struct tag `thrift:\"%s\"`: %w", tag, err))
			} else if id <= 0 {
				panic(fmt.Errorf("invalid thrift field id found in struct tag `thrift:\"%s\"`: %d <= 0", tag, id))
			} else {
				do(structField{
					typ:   f.Type,
					index: fieldIndex,
					id:    int16(id),
					flags: flags,
				})
			}
		}
	}
}
