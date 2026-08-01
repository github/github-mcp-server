//go:build purego || !amd64
// +build purego !amd64

package mem

func countPair1(b []byte) int {
	return countPairGeneric(b, 1)
}

func countPair2(b []byte) int {
	return countPairGeneric(b, 2)
}

func countPair4(b []byte) int {
	return countPairGeneric(b, 4)
}

func countPair8(b []byte) int {
	return countPairGeneric(b, 8)
}

func countPair16(b []byte) int {
	return countPairGeneric(b, 16)
}

func countPair32(b []byte) int {
	return countPairGeneric(b, 32)
}
