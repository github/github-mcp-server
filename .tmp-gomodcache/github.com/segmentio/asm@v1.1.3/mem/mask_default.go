//go:build purego || !amd64
// +build purego !amd64

package mem

// Mask performs a AND of src and dst into dst, returning the number of bytes
// written to dst.
func Mask(dst, src []byte) int {
	return maskGeneric(dst, src)
}
