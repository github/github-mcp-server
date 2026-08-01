package proto

import (
	"errors"
	"io"
	"math"
	"testing"
)

func TestMarshalToShortBuffer(t *testing.T) {
	m := message{
		A: 1,
		B: 2,
		C: 3,
		S: submessage{
			X: "hello",
			Y: "world",
		},
	}

	b, _ := Marshal(m)
	short := make([]byte, len(b))

	for i := range b {
		t.Run("", func(t *testing.T) {
			n, err := MarshalTo(short[:i], m)
			if n != i {
				t.Errorf("byte count mismatch, want %d but got %d", i, n)
			}
			if !errors.Is(err, io.ErrShortBuffer) {
				t.Errorf("error mismatch, want io.ErrShortBuffer but got %q", err)
			}
		})
	}
}

func BenchmarkEncodeVarintShort(b *testing.B) {
	c := [10]byte{}

	for range b.N {
		encodeVarint(c[:], 0)
	}
}

func BenchmarkEncodeVarintLong(b *testing.B) {
	c := [10]byte{}

	for range b.N {
		encodeVarint(c[:], math.MaxUint64)
	}
}

func BenchmarkEncodeTag(b *testing.B) {
	c := [8]byte{}

	for range b.N {
		encodeTag(c[:], 1, varint)
	}
}

func BenchmarkEncodeMessage(b *testing.B) {
	buf := [128]byte{}
	msg := &message{
		A: 1,
		B: 100,
		C: 10000,
		S: submessage{
			X: "",
			Y: "Hello World!",
		},
	}

	size := Size(msg)
	data := buf[:size]
	b.SetBytes(int64(size))

	for range b.N {
		if _, err := MarshalTo(data, msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeMap(b *testing.B) {
	buf := [128]byte{}
	msg := struct {
		M map[string]string
	}{
		M: map[string]string{
			"hello": "world",
		},
	}

	size := Size(msg)
	data := buf[:size]
	b.SetBytes(int64(size))

	for range b.N {
		if _, err := MarshalTo(data, msg); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeSlice(b *testing.B) {
	buf := [128]byte{}
	msg := struct {
		S []int
	}{
		S: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}

	size := Size(msg)
	data := buf[:size]
	b.SetBytes(int64(size))

	for range b.N {
		if _, err := MarshalTo(data, &msg); err != nil {
			b.Fatal(err)
		}
	}
}
