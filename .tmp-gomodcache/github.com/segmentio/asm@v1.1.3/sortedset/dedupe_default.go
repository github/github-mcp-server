//go:build purego || !amd64
// +build purego !amd64

package sortedset

func dedupe1(dst, src []byte) int {
	return dedupeGeneric(dst, src, 1)
}

func dedupe2(dst, src []byte) int {
	return dedupeGeneric(dst, src, 2)
}

func dedupe4(dst, src []byte) int {
	return dedupeGeneric(dst, src, 4)
}

func dedupe8(dst, src []byte) int {
	return dedupeGeneric(dst, src, 8)
}

func dedupe16(dst, src []byte) int {
	return dedupeGeneric(dst, src, 16)
}

func dedupe32(dst, src []byte) int {
	return dedupeGeneric(dst, src, 32)
}
