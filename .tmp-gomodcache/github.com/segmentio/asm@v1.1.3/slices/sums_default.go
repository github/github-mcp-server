//go:build purego || !amd64
// +build purego !amd64

package slices

func sumUint64(x, y []uint64) {
	sumUint64Generic(x, y)
}

func sumUint32(x, y []uint32) {
	sumUint32Generic(x, y)
}

func sumUint16(x, y []uint16) {
	sumUint16Generic(x, y)
}

func sumUint8(x, y []uint8) {
	sumUint8Generic(x, y)
}
