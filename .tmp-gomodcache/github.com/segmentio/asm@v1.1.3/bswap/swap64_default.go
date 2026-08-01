//go:build purego || !amd64
// +build purego !amd64

package bswap

import "encoding/binary"

func swap64(b []byte) {
	for i := 0; i < len(b); i += 8 {
		u := binary.BigEndian.Uint64(b[i:])
		binary.LittleEndian.PutUint64(b[i:], u)
	}
}
