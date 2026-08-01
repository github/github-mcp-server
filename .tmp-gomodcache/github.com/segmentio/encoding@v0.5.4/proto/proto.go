package proto

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"unsafe"
)

func Size(v any) int {
	t, p := inspect(v)
	c := cachedCodecOf(t)
	return c.size(p, inline|toplevel)
}

func Marshal(v any) ([]byte, error) {
	t, p := inspect(v)
	c := cachedCodecOf(t)
	b := make([]byte, c.size(p, inline|toplevel))
	_, err := c.encode(b, p, inline|toplevel)
	if err != nil {
		return nil, fmt.Errorf("proto.Marshal(%T): %w", v, err)
	}
	return b, nil
}

func MarshalTo(b []byte, v any) (int, error) {
	t, p := inspect(v)
	c := cachedCodecOf(t)
	n, err := c.encode(b, p, inline|toplevel)
	if err != nil {
		err = fmt.Errorf("proto.MarshalTo: %w", err)
	}
	return n, err
}

func Unmarshal(b []byte, v any) error {
	if len(b) == 0 {
		// An empty input is a valid protobuf message with all fields set to the
		// zero-value.
		reflect.ValueOf(v).Elem().Set(reflect.Zero(reflect.TypeOf(v).Elem()))
		return nil
	}

	t, p := inspect(v)
	t = t.Elem() // Unmarshal must be passed a pointer
	c := cachedCodecOf(t)

	n, err := c.decode(b, p, toplevel)
	if err != nil {
		return err
	}
	if n < len(b) {
		return fmt.Errorf("proto.Unmarshal(%T): read=%d < buffer=%d", v, n, len(b))
	}
	return nil
}

type flags uintptr

const (
	noflags  flags = 0
	inline   flags = 1 << 0
	wantzero flags = 1 << 1
	// Shared with structField.flags in struct.go:
	// zigzag flags = 1 << 2
	toplevel flags = 1 << 3
)

func (f flags) has(x flags) bool {
	return (f & x) != 0
}

func (f flags) with(x flags) flags {
	return f | x
}

func (f flags) without(x flags) flags {
	return f & ^x
}

func (f flags) uint64(i int64) uint64 {
	if f.has(zigzag) {
		return encodeZigZag64(i)
	} else {
		return uint64(i)
	}
}

func (f flags) int64(u uint64) int64 {
	if f.has(zigzag) {
		return decodeZigZag64(u)
	} else {
		return int64(u)
	}
}

type iface struct {
	typ unsafe.Pointer
	ptr unsafe.Pointer
}

func inspect(v any) (reflect.Type, unsafe.Pointer) {
	return reflect.TypeOf(v), pointer(v)
}

func pointer(v any) unsafe.Pointer {
	return (*iface)(unsafe.Pointer(&v)).ptr
}

func inlined(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Ptr:
		return true
	case reflect.Map:
		return true
	case reflect.Struct:
		return t.NumField() == 1 && inlined(t.Field(0).Type)
	default:
		return false
	}
}

type fieldNumber uint

type wireType uint

const (
	varint  wireType = 0
	fixed64 wireType = 1
	varlen  wireType = 2
	fixed32 wireType = 5
)

func (wt wireType) String() string {
	switch wt {
	case varint:
		return "varint"
	case varlen:
		return "varlen"
	case fixed32:
		return "fixed32"
	case fixed64:
		return "fixed64"
	default:
		return "unknown"
	}
}

type codec struct {
	wire   wireType
	size   sizeFunc
	encode encodeFunc
	decode decodeFunc
}

var codecCache atomic.Value // map[unsafe.Pointer]*codec

func loadCachedCodec(t reflect.Type) (*codec, map[unsafe.Pointer]*codec) {
	cache, _ := codecCache.Load().(map[unsafe.Pointer]*codec)
	return cache[pointer(t)], cache
}

func storeCachedCodec(newCache map[unsafe.Pointer]*codec) {
	codecCache.Store(newCache)
}

func cachedCodecOf(t reflect.Type) *codec {
	c, oldCache := loadCachedCodec(t)
	if c != nil {
		return c
	}

	var p reflect.Type
	isPtr := t.Kind() == reflect.Ptr
	if isPtr {
		p = t
		t = t.Elem()
	} else {
		p = reflect.PtrTo(t)
	}

	seen := make(map[reflect.Type]*codec)
	c1 := codecOf(t, seen)
	c2 := codecOf(p, seen)

	newCache := make(map[unsafe.Pointer]*codec, len(oldCache)+2)
	for p, c := range oldCache {
		newCache[p] = c
	}

	newCache[pointer(t)] = c1
	newCache[pointer(p)] = c2
	storeCachedCodec(newCache)

	if isPtr {
		return c2
	} else {
		return c1
	}
}

func codecOf(t reflect.Type, seen map[reflect.Type]*codec) *codec {
	if c := seen[t]; c != nil {
		return c
	}

	switch {
	case implements(t, messageType):
		return messageCodecOf(t)
	case implements(t, customMessageType) && !implements(t, protoMessageType):
		return customCodecOf(t)
	}

	switch t.Kind() {
	case reflect.Bool:
		return &boolCodec
	case reflect.Int:
		return &intCodec
	case reflect.Int32:
		return &int32Codec
	case reflect.Int64:
		return &int64Codec
	case reflect.Uint:
		return &uintCodec
	case reflect.Uint32:
		return &uint32Codec
	case reflect.Uint64:
		return &uint64Codec
	case reflect.Float32:
		return &float32Codec
	case reflect.Float64:
		return &float64Codec
	case reflect.String:
		return &stringCodec
	case reflect.Array:
		elem := t.Elem()
		switch elem.Kind() {
		case reflect.Uint8:
			return byteArrayCodecOf(t, seen)
		}
	case reflect.Slice:
		elem := t.Elem()
		switch elem.Kind() {
		case reflect.Uint8:
			return &bytesCodec
		}
	case reflect.Struct:
		return structCodecOf(t, seen)
	case reflect.Ptr:
		return pointerCodecOf(t, seen)
	}

	panic("unsupported type: " + t.String())
}

// backward compatibility with gogoproto custom types.
type customMessage interface {
	Size() int
	MarshalTo([]byte) (int, error)
	Unmarshal([]byte) error
}

type protoMessage interface {
	ProtoMessage()
}

var (
	messageType       = reflect.TypeOf((*Message)(nil)).Elem()
	customMessageType = reflect.TypeOf((*customMessage)(nil)).Elem()
	protoMessageType  = reflect.TypeOf((*protoMessage)(nil)).Elem()
)

func implements(t, iface reflect.Type) bool {
	return t.Implements(iface) || reflect.PtrTo(t).Implements(iface)
}
