package slices

import _ "github.com/segmentio/asm/cpu"

// SumUint64 sums pairs of by index from x and y, similar to python's zip routine.
// If available AVX instructions will be used to operate on many uint64s simultaneously.
//
// Results are returned in the x slice and y is left unaltered. If x and y differ in size
// only len(x) elements will be processed.
func SumUint64(x []uint64, y []uint64) {
	sumUint64(x, y)
}

func sumUint64Generic(x, y []uint64) {
	for i := 0; i < len(x) && i < len(y); i++ {
		x[i] = x[i] + y[i]
	}
}

// SumUint32 sums pairs of by index from x and y, similar to python's zip routine.
// If available AVX instructions will be used to operate on many uint32s simultaneously.
//
// Results are returned in the x slice and y is left unaltered. If x and y differ in size
// only len(x) elements will be processed.
func SumUint32(x []uint32, y []uint32) {
	sumUint32(x, y)
}

func sumUint32Generic(x, y []uint32) {
	for i := 0; i < len(x) && i < len(y); i++ {
		x[i] = x[i] + y[i]
	}
}

// SumUint16 sums pairs of by index from x and y, similar to python's zip routine.
// If available AVX instructions will be used to operate on many uint16s simultaneously.
//
// Results are returned in the x slice and y is left unaltered. If x and y differ in size
// only len(x) elements will be processed.
func SumUint16(x []uint16, y []uint16) {
	sumUint16(x, y)
}

func sumUint16Generic(x, y []uint16) {
	for i := 0; i < len(x) && i < len(y); i++ {
		x[i] = x[i] + y[i]
	}
}

// SumUint8 sums pairs of by index from x and y, similar to python's zip routine.
// If available AVX instructions will be used to operate on many uint8s simultaneously.
//
// Results are returned in the x slice and y is left unaltered. If x and y differ in size
// only len(x) elements will be processed.
func SumUint8(x, y []uint8) {
	sumUint8(x, y)
}

func sumUint8Generic(x, y []uint8) {
	for i := 0; i < len(x) && i < len(y); i++ {
		x[i] = x[i] + y[i]
	}
}
