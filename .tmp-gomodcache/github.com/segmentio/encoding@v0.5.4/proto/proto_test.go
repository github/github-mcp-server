package proto

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestEncodeDecodeVarint(t *testing.T) {
	b := [8]byte{}

	n, err := encodeVarint(b[:], 42)
	if err != nil {
		t.Fatal(err)
	}

	v, n2, err := decodeVarint(b[:n])
	if err != nil {
		t.Fatal(err)
	}
	if v != 42 {
		t.Errorf("decoded value mismatch: want %d, got %d", 42, v)
	}
	if n2 != n {
		t.Errorf("decoded byte count mismatch: want %d, got %d", n, n2)
	}
}

func TestEncodeDecodeVarintZigZag(t *testing.T) {
	b := [8]byte{}

	n, err := encodeVarintZigZag(b[:], -42)
	if err != nil {
		t.Fatal(err)
	}

	v, n2, err := decodeVarintZigZag(b[:n])
	if err != nil {
		t.Fatal(err)
	}
	if v != -42 {
		t.Errorf("decoded value mismatch: want %d, got %d", -42, v)
	}
	if n2 != n {
		t.Errorf("decoded byte count mismatch: want %d, got %d", n, n2)
	}
}

func TestEncodeDecodeTag(t *testing.T) {
	b := [8]byte{}

	n, err := encodeTag(b[:], 1, varint)
	if err != nil {
		t.Fatal(err)
	}

	num, typ, n2, err := decodeTag(b[:n])
	if err != nil {
		t.Fatal(err)
	}
	if num != 1 {
		t.Errorf("decoded field number mismatch: want %d, got %d", 1, num)
	}
	if typ != varint {
		t.Errorf("decoded wire type mismatch: want %d, got %d", varint, typ)
	}
	if n2 != n {
		t.Errorf("decoded byte count mismatch: want %d, got %d", n, n2)
	}
}

type key struct {
	Hi uint64
	Lo uint64
}

type message struct {
	A int
	B int
	C int
	S submessage
}

type submessage struct {
	X string
	Y string
}

type structWithMap struct {
	M map[int]string
}

type custom [16]byte

func (c *custom) Size() int { return len(c) }

func (c *custom) MarshalTo(b []byte) (int, error) {
	return copy(b, c[:]), nil
}

func (c *custom) Unmarshal(b []byte) error {
	copy(c[:], b)
	return nil
}

type messageWithRawMessage struct {
	Raw RawMessage
}

type messageWithCustomField struct {
	Custom custom
}

func TestMarshalUnmarshal(t *testing.T) {
	intVal := 42
	values := []any{
		// bool
		true,
		false,

		// zig-zag varint
		0,
		1,
		1234567890,
		-1,
		-1234567890,

		// sfixed32
		int32(0),
		int32(math.MinInt32),
		int32(math.MaxInt32),

		// sfixed64
		int64(0),
		int64(math.MinInt64),
		int64(math.MaxInt64),

		// varint
		uint(0),
		uint(1),
		uint(1234567890),

		// fixed32
		uint32(0),
		uint32(1234567890),

		// fixed64
		uint64(0),
		uint64(1234567890),

		// float
		float32(0),
		float32(math.Copysign(0, -1)),
		float32(0.1234),

		// double
		float64(0),
		float64(math.Copysign(0, -1)),
		float64(0.1234),

		// string
		"",
		"A",
		"Hello World!",

		// bytes
		([]byte)(nil),
		[]byte(""),
		[]byte("A"),
		[]byte("Hello World!"),

		// messages
		struct{ B bool }{B: false},
		struct{ B bool }{B: true},

		struct{ I int }{I: 0},
		struct{ I int }{I: 1},

		struct{ I32 int32 }{I32: 0},
		struct{ I32 int32 }{I32: -1234567890},

		struct{ I64 int64 }{I64: 0},
		struct{ I64 int64 }{I64: -1234567890},

		struct{ U int }{U: 0},
		struct{ U int }{U: 1},

		struct{ U32 uint32 }{U32: 0},
		struct{ U32 uint32 }{U32: 1234567890},

		struct{ U64 uint64 }{U64: 0},
		struct{ U64 uint64 }{U64: 1234567890},

		struct{ F32 float32 }{F32: 0},
		struct{ F32 float32 }{F32: 0.1234},

		struct{ F64 float64 }{F64: 0},
		struct{ F64 float64 }{F64: 0.1234},

		struct{ S string }{S: ""},
		struct{ S string }{S: "E"},

		struct{ B []byte }{B: nil},
		struct{ B []byte }{B: []byte{}},
		struct{ B []byte }{B: []byte{1, 2, 3}},

		&message{
			A: 1,
			B: 2,
			C: 3,
			S: submessage{
				X: "hello",
				Y: "world",
			},
		},

		struct {
			Min int64 `protobuf:"zigzag64,1,opt,name=min,proto3"`
			Max int64 `protobuf:"zigzag64,2,opt,name=min,proto3"`
		}{Min: math.MinInt64, Max: math.MaxInt64},

		// pointers
		struct{ M *message }{M: nil},
		struct {
			M1 *message
			M2 *message
			M3 *message
		}{
			M1: &message{A: 10, B: 100, C: 1000},
			M2: &message{S: submessage{X: "42"}},
		},

		// byte arrays
		[0]byte{},
		[8]byte{},
		[16]byte{0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xA, 0xB, 0xC, 0xD, 0xE, 0xF},
		&[...]byte{},
		&[...]byte{3, 2, 1},

		// slices (repeated)
		struct{ S []int }{S: nil},
		struct{ S []int }{S: []int{0}},
		struct{ S []int }{S: []int{0, 0, 0}},
		struct{ S []int }{S: []int{1, 2, 3}},
		struct{ S []string }{S: nil},
		struct{ S []string }{S: []string{""}},
		struct{ S []string }{S: []string{"A", "B", "C"}},
		struct{ K []key }{
			K: []key{
				{Hi: 0, Lo: 0},
				{Hi: 0, Lo: 1},
				{Hi: 0, Lo: 2},
				{Hi: 0, Lo: 3},
				{Hi: 0, Lo: 4},
			},
		},

		// maps (repeated)
		struct{ M map[int]string }{},
		struct{ M map[int]string }{
			M: map[int]string{0: ""},
		},
		struct{ M map[int]string }{
			M: map[int]string{0: "A", 1: "B", 2: "C"},
		},
		&struct{ M map[int]string }{
			M: map[int]string{0: "A", 1: "B", 2: "C"},
		},
		struct {
			M1 map[int]int
			M2 map[string]string
			M3 map[string]message
			M4 map[string]*message
			M5 map[key]uint
		}{
			M1: map[int]int{0: 1},
			M2: map[string]string{"": "A"},
			M3: map[string]message{
				"m0": {},
				"m1": {A: 42},
				"m3": {S: submessage{X: "X", Y: "Y"}},
			},
			M4: map[string]*message{
				"m0": {},
				"m1": {A: 42},
				"m3": {S: submessage{X: "X", Y: "Y"}},
			},
			M5: map[key]uint{
				{Hi: 0, Lo: 0}:                           0,
				{Hi: 1, Lo: 0}:                           1,
				{Hi: 0, Lo: 1}:                           2,
				{Hi: math.MaxUint64, Lo: math.MaxUint64}: 3,
			},
		},

		// more complex inlined types use cases
		struct{ I *int }{},
		struct{ I *int }{I: new(int)},
		struct{ I *int }{I: &intVal},
		struct{ M *message }{},
		struct{ M *message }{M: new(message)},
		struct{ M map[int]int }{},
		struct{ M map[int]int }{M: map[int]int{}},
		struct{ S structWithMap }{
			S: structWithMap{
				M: map[int]string{0: "A", 1: "B", 2: "C"},
			},
		},
		&struct{ S structWithMap }{
			S: structWithMap{
				M: map[int]string{0: "A", 1: "B", 2: "C"},
			},
		},

		// raw messages
		RawMessage(nil),
		RawMessage{0x08, 0x96, 0x01},
		messageWithRawMessage{
			Raw: RawMessage{1, 2, 3, 4},
		},
		struct {
			A int
			B string
			C RawMessage
		}{A: 42, B: "Hello World!", C: RawMessage{1, 2, 3, 4}},

		// custom messages
		custom{},
		custom{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		messageWithCustomField{
			Custom: custom{1: 42},
		},
		struct {
			A int
			B string
			C custom
		}{A: 42, B: "Hello World!", C: custom{1: 42}},
	}

	for _, v := range values {
		t.Run(fmt.Sprintf("%T/%+v", v, v), func(t *testing.T) {
			n := Size(v)

			b, err := Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if n != len(b) {
				t.Fatalf("value size and buffer length mismatch (%d != %d)", n, len(b))
			}

			p := reflect.New(reflect.TypeOf(v))
			if err := Unmarshal(b, p.Interface()); err != nil {
				t.Fatal(err)
			}

			x := p.Elem().Interface()
			if !reflect.DeepEqual(v, x) {
				t.Errorf("values mismatch:\nexpected: %#v\nfound:    %#v", v, x)
			}
		})
	}
}

func loadProtobuf(t *testing.T, fileName string) RawMessage {
	b, err := os.ReadFile("fixtures/protobuf/" + fileName)
	if err != nil {
		t.Fatal(err)
	}
	return RawMessage(b)
}

func makeVarint(v uint64) []byte {
	b := [12]byte{}
	n := binary.PutUvarint(b[:], v)
	return b[:n]
}

func makeFixed32(v uint32) []byte {
	b := [4]byte{}
	binary.LittleEndian.PutUint32(b[:], v)
	return b[:]
}

func makeFixed64(v uint64) []byte {
	b := [8]byte{}
	binary.LittleEndian.PutUint64(b[:], v)
	return b[:]
}
