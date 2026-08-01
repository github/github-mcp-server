package proto

import (
	"errors"
	"io"
	"testing"
)

func TestUnarshalFromShortBuffer(t *testing.T) {
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

	for i := range b {
		switch i {
		case 0, 2, 4, 6:
			continue // these land on field boundaries, making the input valid
		}
		t.Run("", func(t *testing.T) {
			msg := &message{}
			err := Unmarshal(b[:i], msg)
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Errorf("error mismatch, want io.ErrUnexpectedEOF but got %q", err)
			}
		})
	}
}

func TestUnmarshalFixture(t *testing.T) {
	type Message struct {
		A uint
		B uint32
		C uint64
		D string
	}

	b := loadProtobuf(t, "message.pb")
	m := Message{}

	if err := Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}

	if m.A != 10 {
		t.Error("m.A mismatch, want 10 but got", m.A)
	}

	if m.B != 20 {
		t.Error("m.B mismatch, want 20 but got", m.B)
	}

	if m.C != 30 {
		t.Error("m.C mismatch, want 30 but got", m.C)
	}

	if m.D != "Hello World!" {
		t.Errorf("m.D mismatch, want \"Hello World!\" but got %q", m.D)
	}
}

func BenchmarkDecodeTag(b *testing.B) {
	c := [8]byte{}
	n, _ := encodeTag(c[:], 1, varint)

	for range b.N {
		decodeTag(c[:n])
	}
}

func BenchmarkDecodeMessage(b *testing.B) {
	data, _ := Marshal(message{
		A: 1,
		B: 100,
		C: 10000,
		S: submessage{
			X: "",
			Y: "Hello World!",
		},
	})

	msg := message{}
	b.SetBytes(int64(len(data)))

	for range b.N {
		if err := Unmarshal(data, &msg); err != nil {
			b.Fatal(err)
		}
		msg = message{}
	}
}

func BenchmarkDecodeMap(b *testing.B) {
	type message struct {
		M map[int]int
	}

	data, _ := Marshal(message{
		M: map[int]int{
			0: 0,
			1: 1,
			2: 2,
			3: 3,
			4: 4,
		},
	})

	msg := message{}
	b.SetBytes(int64(len(data)))

	for range b.N {
		if err := Unmarshal(data, &msg); err != nil {
			b.Fatal(err)
		}
		msg = message{}
	}
}

func BenchmarkDecodeSlice(b *testing.B) {
	type message struct {
		S []int
	}

	data, _ := Marshal(message{
		S: []int{
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
		},
	})

	msg := message{}
	b.SetBytes(int64(len(data)))

	for range b.N {
		if err := Unmarshal(data, &msg); err != nil {
			b.Fatal(err)
		}
		msg = message{}
	}
}
