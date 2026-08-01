package bswap

import _ "github.com/segmentio/asm/cpu"

// Swap64 performs an in-place byte swap on each 64 bits elements in b.
//
// This function is useful when dealing with big-endian input; by converting it
// to little-endian, the data can then be compared using native CPU instructions
// instead of having to employ often slower byte comparison algorithms.
func Swap64(b []byte) {
	if len(b)%8 != 0 {
		panic("swap64 expects the input to contain full 64 bits elements")
	}
	swap64(b)
}
