package qsort

import "unsafe"

func unsafeBytesTo64(b []byte) []uint64 {
	return *(*[]uint64)(unsafe.Pointer(cast(b, 8)))
}

func unsafeBytesTo128(b []byte) []uint128 {
	return *(*[]uint128)(unsafe.Pointer(cast(b, 16)))
}

func unsafeBytesTo192(b []byte) []uint192 {
	return *(*[]uint192)(unsafe.Pointer(cast(b, 24)))
}

func unsafeBytesTo256(b []byte) []uint256 {
	return *(*[]uint256)(unsafe.Pointer(cast(b, 32)))
}

func cast(b []byte, size int) *sliceHeader {
	return &sliceHeader{
		Data: *(*unsafe.Pointer)(unsafe.Pointer(&b)),
		Len:  len(b) / size,
		Cap:  len(b) / size,
	}
}

type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}
