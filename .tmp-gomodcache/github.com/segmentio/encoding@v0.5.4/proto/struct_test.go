package proto

import (
	"testing"
	"unsafe"
)

func TestStructFieldSize(t *testing.T) {
	t.Log("sizeof(structField) =", unsafe.Sizeof(structField{}))
}
