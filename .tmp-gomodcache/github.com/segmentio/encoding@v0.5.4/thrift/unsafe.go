package thrift

import (
	"reflect"
	"unsafe"
)

// typeID is used as key in encoder and decoder caches to enable using
// the optimize runtime.mapaccess2_fast64 function instead of the more
// expensive lookup if we were to use reflect.Type as map key.
//
// typeID holds the pointer to the reflect.Type value, which is unique
// in the program.
type typeID struct{ ptr unsafe.Pointer }

func makeTypeID(t reflect.Type) typeID {
	return typeID{
		ptr: (*[2]unsafe.Pointer)(unsafe.Pointer(&t))[1],
	}
}

func unsafeBytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
