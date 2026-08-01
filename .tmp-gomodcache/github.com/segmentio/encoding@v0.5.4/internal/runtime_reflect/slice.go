package runtime_reflect

import "unsafe"

type Slice struct {
	data unsafe.Pointer
	len  int
	cap  int
}

func (s *Slice) Cap() int {
	return s.cap
}

func (s *Slice) Len() int {
	return s.len
}

func (s *Slice) SetLen(n int) {
	s.len = n
}

func (s *Slice) Index(i int, elemSize uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(s.data) + (uintptr(i) * elemSize))
}

func MakeSlice(elemType unsafe.Pointer, len, cap int) Slice {
	return Slice{
		data: newarray(elemType, cap),
		len:  len,
		cap:  cap,
	}
}

func CopySlice(elemType unsafe.Pointer, dst, src Slice) int {
	return typedslicecopy(elemType, dst, src)
}

//go:linkname newarray runtime.newarray
func newarray(t unsafe.Pointer, n int) unsafe.Pointer

//go:linkname typedslicecopy runtime.typedslicecopy
//go:noescape
func typedslicecopy(t unsafe.Pointer, dst, src Slice) int
