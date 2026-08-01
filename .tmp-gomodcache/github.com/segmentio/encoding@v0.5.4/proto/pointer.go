package proto

import (
	"reflect"
	"unsafe"
)

func pointerCodecOf(t reflect.Type, seen map[reflect.Type]*codec) *codec {
	p := new(codec)
	seen[t] = p
	c := codecOf(t.Elem(), seen)
	p.wire = c.wire
	p.size = pointerSizeFuncOf(t, c)
	p.encode = pointerEncodeFuncOf(t, c)
	p.decode = pointerDecodeFuncOf(t, c)
	return p
}

func pointerSizeFuncOf(t reflect.Type, c *codec) sizeFunc {
	return func(p unsafe.Pointer, flags flags) int {
		if p != nil {
			if !flags.has(inline) {
				p = *(*unsafe.Pointer)(p)
			}
			return c.size(p, flags.without(inline).with(wantzero))
		}
		return 0
	}
}

func pointerEncodeFuncOf(t reflect.Type, c *codec) encodeFunc {
	return func(b []byte, p unsafe.Pointer, flags flags) (int, error) {
		if p != nil {
			if !flags.has(inline) {
				p = *(*unsafe.Pointer)(p)
			}
			return c.encode(b, p, flags.without(inline).with(wantzero))
		}
		return 0, nil
	}
}

func pointerDecodeFuncOf(t reflect.Type, c *codec) decodeFunc {
	t = t.Elem()
	return func(b []byte, p unsafe.Pointer, flags flags) (int, error) {
		v := (*unsafe.Pointer)(p)
		if *v == nil {
			*v = unsafe.Pointer(reflect.New(t).Pointer())
		}
		return c.decode(b, *v, flags)
	}
}
