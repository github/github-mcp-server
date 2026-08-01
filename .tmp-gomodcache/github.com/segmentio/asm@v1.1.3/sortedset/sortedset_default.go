//go:build purego || !amd64
// +build purego !amd64

package sortedset

func intersect16(dst, a, b []byte) int {
	return intersectGeneric(dst, a, b, 16)
}

func union16(dst, a, b []byte) (i, j, k int) {
	return unionGeneric(dst, a, b, 16)
}
