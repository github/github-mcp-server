//go:build purego || !amd64
// +build purego !amd64

package qsort

const purego = true

func distributeForward64(data []uint64, scratch []uint64, limit int, lo int, hi int) int {
	panic("not implemented")
}

func distributeBackward64(data []uint64, scratch []uint64, limit int, lo int, hi int) int {
	panic("not implemented")
}

func insertionsort128NoSwap(data []uint128, base int, swap func(int, int)) {
	panic("not implemented")
}

func distributeForward128(data, scratch []uint128, limit, lo, hi int) int {
	panic("not implemented")
}

func distributeBackward128(data, scratch []uint128, limit, lo, hi int) int {
	panic("not implemented")
}

func insertionsort256NoSwap(data []uint256, base int, swap func(int, int)) {
	panic("not implemented")
}

func distributeForward256(data, scratch []uint256, limit, lo, hi int) int {
	panic("not implemented")
}

func distributeBackward256(data, scratch []uint256, limit, lo, hi int) int {
	panic("not implemented")
}
