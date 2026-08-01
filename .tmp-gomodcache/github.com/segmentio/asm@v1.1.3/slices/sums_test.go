package slices

import (
	"math/rand"
	"testing"
)

var size = 1024 * 1024

func TestSumUint8(t *testing.T) {
	x, y := generateUint8Slices()
	genericXCopy := make([]uint8, len(x))
	copy(genericXCopy, x)
	sumUint8(x, y)
	sumUint8Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint8YLarger(t *testing.T) {
	x, y := generateUint8Slices()
	y = append(y, uint8(100))
	genericXCopy := make([]uint8, len(x))
	copy(genericXCopy, x)
	sumUint8(x, y)
	sumUint8Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint8XLarger(t *testing.T) {
	x, y := generateUint8Slices()
	x = append(x, uint8(100))
	genericXCopy := make([]uint8, len(x))
	copy(genericXCopy, x)
	sumUint8(x, y)
	sumUint8Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint16(t *testing.T) {
	x, y := generateUint16Slices()
	genericXCopy := make([]uint16, len(x))
	copy(genericXCopy, x)
	sumUint16(x, y)
	sumUint16Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint16YLarger(t *testing.T) {
	x, y := generateUint16Slices()
	y = append(y, uint16(100))
	genericXCopy := make([]uint16, len(x))
	copy(genericXCopy, x)
	sumUint16(x, y)
	sumUint16Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint16XLarger(t *testing.T) {
	x, y := generateUint16Slices()
	x = append(x, uint16(100))
	genericXCopy := make([]uint16, len(x))
	copy(genericXCopy, x)
	sumUint16(x, y)
	sumUint16Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint32(t *testing.T) {
	x, y := generateUint32Slices()
	genericXCopy := make([]uint32, len(x))
	copy(genericXCopy, x)
	sumUint32(x, y)
	sumUint32Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint32YLarger(t *testing.T) {
	x, y := generateUint32Slices()
	y = append(y, uint32(100))
	genericXCopy := make([]uint32, len(x))
	copy(genericXCopy, x)
	sumUint32(x, y)
	sumUint32Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint32XLarger(t *testing.T) {
	x, y := generateUint32Slices()
	x = append(x, uint32(100))
	genericXCopy := make([]uint32, len(x))
	copy(genericXCopy, x)
	sumUint32(x, y)
	sumUint32Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint64(t *testing.T) {
	x, y := generateUint64Slices()
	genericXCopy := make([]uint64, len(x))
	copy(genericXCopy, x)
	sumUint64(x, y)
	sumUint64Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint64YLarger(t *testing.T) {
	x, y := generateUint64Slices()
	y = append(y, uint64(100))
	genericXCopy := make([]uint64, len(x))
	copy(genericXCopy, x)
	sumUint64(x, y)
	sumUint64Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func TestSumUint64XLarger(t *testing.T) {
	x, y := generateUint64Slices()
	x = append(x, uint64(100))
	genericXCopy := make([]uint64, len(x))
	copy(genericXCopy, x)
	sumUint64(x, y)
	sumUint64Generic(genericXCopy, y)
	for i := 0; i < len(x); i++ {
		if x[i] != genericXCopy[i] {
			t.Fatalf("mismatch sums at index %d, expected %d : got %d", i, genericXCopy[i], x[i])
		}
	}
}

func generateUint8Slices() ([]uint8, []uint8) {
	var x []uint8
	var y []uint8
	for i := 0; i < size; i++ {
		x = append(x, uint8(i))
		y = append(y, uint8(i))
	}
	return x, y
}

func generateUint16Slices() ([]uint16, []uint16) {
	var x []uint16
	var y []uint16
	for i := 0; i < size; i++ {
		x = append(x, uint16(i))
		y = append(y, uint16(i))
	}
	return x, y
}

func generateUint32Slices() ([]uint32, []uint32) {
	var x []uint32
	var y []uint32
	prng := rand.New(rand.NewSource(0))
	for i := 0; i < size; i++ {
		x = append(x, prng.Uint32())
		y = append(y, prng.Uint32())
	}
	return x, y
}

func generateUint64Slices() ([]uint64, []uint64) {
	var x []uint64
	var y []uint64
	prng := rand.New(rand.NewSource(0))
	for i := 0; i < size; i++ {
		x = append(x, prng.Uint64())
		y = append(y, prng.Uint64())
	}
	return x, y
}

func BenchmarkSumUnit8(b *testing.B) {
	x, y := generateUint8Slices()
	for i := 0; i < b.N; i++ {
		SumUint8(x, y)
	}
}

func BenchmarkSumUnit8Generic(b *testing.B) {
	x, y := generateUint8Slices()
	for i := 0; i < b.N; i++ {
		sumUint8Generic(x, y)
	}
}

func BenchmarkSumUnit16(b *testing.B) {
	x, y := generateUint16Slices()
	for i := 0; i < b.N; i++ {
		SumUint16(x, y)
	}
}

func BenchmarkSumUnit16Generic(b *testing.B) {
	x, y := generateUint16Slices()
	for i := 0; i < b.N; i++ {
		sumUint16Generic(x, y)
	}
}

func BenchmarkSumUnit32(b *testing.B) {
	x, y := generateUint32Slices()
	for i := 0; i < b.N; i++ {
		SumUint32(x, y)
	}
}

func BenchmarkSumUnit32Generic(b *testing.B) {
	x, y := generateUint32Slices()
	for i := 0; i < b.N; i++ {
		sumUint32Generic(x, y)
	}
}

func BenchmarkSumUnit64(b *testing.B) {
	x, y := generateUint64Slices()
	for i := 0; i < b.N; i++ {
		SumUint64(x, y)
	}
}

func BenchmarkSumUnit64Generic(b *testing.B) {
	x, y := generateUint64Slices()
	for i := 0; i < b.N; i++ {
		sumUint64Generic(x, y)
	}
}
