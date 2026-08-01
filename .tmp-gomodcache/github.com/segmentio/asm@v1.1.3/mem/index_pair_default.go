//go:build purego || !amd64
// +build purego !amd64

package mem

func indexPair1(b []byte) int {
	return indexPairGeneric(b, 1)
}

func indexPair2(b []byte) int {
	return indexPairGeneric(b, 2)
}

func indexPair4(b []byte) int {
	return indexPairGeneric(b, 4)
}

func indexPair8(b []byte) int {
	return indexPairGeneric(b, 8)
}

func indexPair16(b []byte) int {
	return indexPairGeneric(b, 16)
}

func indexPair32(b []byte) int {
	return indexPairGeneric(b, 32)
}
