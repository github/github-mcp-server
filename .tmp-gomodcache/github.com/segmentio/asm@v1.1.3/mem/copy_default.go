//go:build purego || !amd64
// +build purego !amd64

package mem

func Copy(dst, src []byte) int {
	return copyGeneric(dst, src)
}
