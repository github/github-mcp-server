//go:build purego || !amd64
// +build purego !amd64

package mem

// Blend performs a OR of src and dst into dst, returning the number of bytes
// written to dst.
func Blend(dst, src []byte) int {
	return blendGeneric(dst, src)
}
