package proto

import (
	"encoding/binary"
	"errors"
	"io"
	"unsafe"
)

// DecodeTag reverses the encoding applied by EncodeTag.
func DecodeTag(tag uint64) (FieldNumber, WireType) {
	return FieldNumber(tag >> 3), WireType(tag & 7)
}

// DecodeZigZag reverses the encoding applied by EncodeZigZag.
func DecodeZigZag(v uint64) int64 {
	return decodeZigZag64(v)
}

func decodeZigZag64(v uint64) int64 {
	return int64(v>>1) ^ -(int64(v) & 1)
}

func decodeZigZag32(v uint32) int32 {
	return int32(v>>1) ^ -(int32(v) & 1)
}

type decodeFunc = func([]byte, unsafe.Pointer, flags) (int, error)

var errVarintOverflow = errors.New("varint overflowed 64 bits integer")

func decodeVarint(b []byte) (uint64, int, error) {
	if len(b) != 0 && b[0] < 0x80 {
		// Fast-path for decoding the common case of varints that fit on a
		// single byte.
		//
		// This path is ~60% faster than calling binary.Uvarint.
		return uint64(b[0]), 1, nil
	}

	var x uint64
	var s uint

	for i, c := range b {
		if c < 0x80 {
			if i > 9 || i == 9 && c > 1 {
				return 0, i, errVarintOverflow
			}
			return x | uint64(c)<<s, i + 1, nil
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}

	return x, len(b), io.ErrUnexpectedEOF
}

func decodeVarintZigZag(b []byte) (int64, int, error) {
	v, n, err := decodeVarint(b)
	return decodeZigZag64(v), n, err
}

func decodeLE32(b []byte) (uint32, int, error) {
	if len(b) < 4 {
		return 0, 0, io.ErrUnexpectedEOF
	}
	return binary.LittleEndian.Uint32(b), 4, nil
}

func decodeLE64(b []byte) (uint64, int, error) {
	if len(b) < 8 {
		return 0, 0, io.ErrUnexpectedEOF
	}
	return binary.LittleEndian.Uint64(b), 8, nil
}

func decodeTag(b []byte) (f fieldNumber, t wireType, n int, err error) {
	v, n, err := decodeVarint(b)
	return fieldNumber(v >> 3), wireType(v & 7), n, err
}

func decodeVarlen(b []byte) ([]byte, int, error) {
	v, n, err := decodeVarint(b)
	if err != nil {
		return nil, n, err
	}
	if v > uint64(len(b)-n) {
		return nil, n, io.ErrUnexpectedEOF
	}
	return b[n : n+int(v)], n + int(v), nil
}
