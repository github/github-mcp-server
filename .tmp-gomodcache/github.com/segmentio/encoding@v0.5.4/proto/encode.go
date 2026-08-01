package proto

import (
	"encoding/binary"
	"io"
	"unsafe"
)

// EncodeTag encodes a pair of field number and wire type into a protobuf tag.
func EncodeTag(f FieldNumber, t WireType) uint64 {
	return uint64(f)<<3 | uint64(t)
}

// EncodeZigZag returns v as a zig-zag encoded value.
func EncodeZigZag(v int64) uint64 {
	return encodeZigZag64(v)
}

func encodeZigZag64(v int64) uint64 {
	return (uint64(v) << 1) ^ uint64(v>>63)
}

func encodeZigZag32(v int32) uint32 {
	return (uint32(v) << 1) ^ uint32(v>>31)
}

type encodeFunc = func([]byte, unsafe.Pointer, flags) (int, error)

func encodeVarint(b []byte, v uint64) (int, error) {
	n := sizeOfVarint(v)

	if len(b) < n {
		return 0, io.ErrShortBuffer
	}

	switch n {
	case 1:
		b[0] = byte(v)

	case 2:
		b[0] = byte(v) | 0x80
		b[1] = byte(v >> 7)

	case 3:
		b[0] = byte(v) | 0x80
		b[1] = byte(v>>7) | 0x80
		b[2] = byte(v >> 14)

	case 4:
		b[0] = byte(v) | 0x80
		b[1] = byte(v>>7) | 0x80
		b[2] = byte(v>>14) | 0x80
		b[3] = byte(v >> 21)

	case 5:
		b[0] = byte(v) | 0x80
		b[1] = byte(v>>7) | 0x80
		b[2] = byte(v>>14) | 0x80
		b[3] = byte(v>>21) | 0x80
		b[4] = byte(v >> 28)

	case 6:
		b[0] = byte(v) | 0x80
		b[1] = byte(v>>7) | 0x80
		b[2] = byte(v>>14) | 0x80
		b[3] = byte(v>>21) | 0x80
		b[4] = byte(v>>28) | 0x80
		b[5] = byte(v >> 35)

	case 7:
		b[0] = byte(v) | 0x80
		b[1] = byte(v>>7) | 0x80
		b[2] = byte(v>>14) | 0x80
		b[3] = byte(v>>21) | 0x80
		b[4] = byte(v>>28) | 0x80
		b[5] = byte(v>>35) | 0x80
		b[6] = byte(v >> 42)

	case 8:
		b[0] = byte(v) | 0x80
		b[1] = byte(v>>7) | 0x80
		b[2] = byte(v>>14) | 0x80
		b[3] = byte(v>>21) | 0x80
		b[4] = byte(v>>28) | 0x80
		b[5] = byte(v>>35) | 0x80
		b[6] = byte(v>>42) | 0x80
		b[7] = byte(v >> 49)

	case 9:
		b[0] = byte(v) | 0x80
		b[1] = byte(v>>7) | 0x80
		b[2] = byte(v>>14) | 0x80
		b[3] = byte(v>>21) | 0x80
		b[4] = byte(v>>28) | 0x80
		b[5] = byte(v>>35) | 0x80
		b[6] = byte(v>>42) | 0x80
		b[7] = byte(v>>49) | 0x80
		b[8] = byte(v >> 56)

	case 10:
		b[0] = byte(v) | 0x80
		b[1] = byte(v>>7) | 0x80
		b[2] = byte(v>>14) | 0x80
		b[3] = byte(v>>21) | 0x80
		b[4] = byte(v>>28) | 0x80
		b[5] = byte(v>>35) | 0x80
		b[6] = byte(v>>42) | 0x80
		b[7] = byte(v>>49) | 0x80
		b[8] = byte(v>>56) | 0x80
		b[9] = byte(v >> 63)
	}

	return n, nil
}

func encodeVarintZigZag(b []byte, v int64) (int, error) {
	return encodeVarint(b, encodeZigZag64(v))
}

func encodeLE32(b []byte, v uint32) (int, error) {
	if len(b) < 4 {
		return 0, io.ErrShortBuffer
	}
	binary.LittleEndian.PutUint32(b, v)
	return 4, nil
}

func encodeLE64(b []byte, v uint64) (int, error) {
	if len(b) < 8 {
		return 0, io.ErrShortBuffer
	}
	binary.LittleEndian.PutUint64(b, v)
	return 8, nil
}

func encodeTag(b []byte, f fieldNumber, t wireType) (int, error) {
	return encodeVarint(b, uint64(f)<<3|uint64(t))
}
