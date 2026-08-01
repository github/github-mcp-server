package proto

import (
	"bytes"
	"math"
	"testing"
)

func TestAppendVarint(t *testing.T) {
	m := AppendVarint(nil, 1, 42)

	f, w, v, r, err := Parse(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 0 {
		t.Fatal("unexpected trailing bytes:", r)
	}
	if w != Varint {
		t.Fatal("unexpected wire type:", t)
	}
	if f != 1 {
		t.Fatal("unexpected field number:", f)
	}
	if u := v.Varint(); u != 42 {
		t.Fatal("value mismatch, want 42 but got", u)
	}
}

func TestAppendVarlen(t *testing.T) {
	m := AppendVarlen(nil, 1, []byte("Hello World!"))

	f, w, v, r, err := Parse(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 0 {
		t.Fatal("unexpected trailing bytes:", r)
	}
	if w != Varlen {
		t.Fatal("unexpected wire type:", t)
	}
	if f != 1 {
		t.Fatal("unexpected field number:", f)
	}
	if string(v) != "Hello World!" {
		t.Fatalf("value mismatch, want \"Hello World!\" but got %q", v)
	}
}

func TestAppendFixed32(t *testing.T) {
	m := AppendFixed32(nil, 1, 42)

	f, w, v, r, err := Parse(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 0 {
		t.Fatal("unexpected trailing bytes:", r)
	}
	if w != Fixed32 {
		t.Fatal("unexpected wire type:", t)
	}
	if f != 1 {
		t.Fatal("unexpected field number:", f)
	}
	if u := v.Fixed32(); u != 42 {
		t.Fatal("value mismatch, want 42 but got", u)
	}
}

func TestAppendFixed64(t *testing.T) {
	m := AppendFixed64(nil, 1, 42)

	f, w, v, r, err := Parse(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 0 {
		t.Fatal("unexpected trailing bytes:", r)
	}
	if w != Fixed64 {
		t.Fatal("unexpected wire type:", t)
	}
	if f != 1 {
		t.Fatal("unexpected field number:", f)
	}
	if u := v.Fixed64(); u != 42 {
		t.Fatal("value mismatch, want 42 but got", u)
	}
}

func TestDecodeFromAppend(t *testing.T) {
	m := RawMessage(nil)
	m = AppendVarint(m, 1, math.MaxUint64)
	m = AppendVarlen(m, 2, []byte("Hello World!"))
	m = AppendFixed32(m, 3, math.Float32bits(42.0))
	m = AppendFixed64(m, 4, math.Float64bits(1234.0))

	type M struct {
		I   int
		S   string
		F32 float32
		F64 float64
	}

	x := M{}

	if err := Unmarshal(m, &x); err != nil {
		t.Fatal(err)
	}
	if x.I != -1 {
		t.Errorf("x.I=%d", x.I)
	}
	if x.S != "Hello World!" {
		t.Errorf("x.S=%q", x.S)
	}
	if x.F32 != 42 {
		t.Errorf("x.F32=%g", x.F32)
	}
	if x.F64 != 1234 {
		t.Errorf("x.F64=%g", x.F64)
	}
}

func TestDecodeFixture(t *testing.T) {
	m := loadProtobuf(t, "message.pb")
	m = assertParse(t, m, 1, Varint, makeVarint(10))
	m = assertParse(t, m, 2, Varint, makeVarint(20))
	m = assertParse(t, m, 3, Varint, makeVarint(30))
	m = assertParse(t, m, 4, Varlen, []byte("Hello World!"))
	assertEmpty(t, m)
}

func assertParse(t *testing.T, m RawMessage, f FieldNumber, w WireType, b []byte) RawMessage {
	t.Helper()

	f0, w0, b0, m, err := Parse(m)
	if err != nil {
		t.Fatal(err)
	}

	if f0 != f {
		t.Errorf("field number mismatch, want %d but got %d", f, f0)
	}

	if w0 != w {
		t.Errorf("wire type mismatch, want %d but got %d", w, w0)
	}

	if !bytes.Equal(b0, b) {
		t.Errorf("value mismatch, want %v but got %v", b, b0)
	}

	return m
}

func assertEmpty(t *testing.T, m RawMessage) {
	t.Helper()

	if len(m) != 0 {
		t.Errorf("unexpected content remained in the protobuf message: %v", m)
	}
}

func BenchmarkScan(b *testing.B) {
	m, _ := Marshal(&message{
		A: 1,
		B: 2,
		C: 3,
		S: submessage{
			X: "hello",
			Y: "world",
		},
	})

	for range b.N {
		Scan(m, func(f FieldNumber, t WireType, v RawValue) (bool, error) {
			switch f {
			case 1, 2, 3:
				return true, nil
			case 4:
				err := Scan(v, func(f FieldNumber, t WireType, v RawValue) (bool, error) {
					switch f {
					case 1, 2:
						return true, nil
					default:
						b.Error("invalid field number:", f)
						return false, nil
					}
				})
				return err != nil, err
			default:
				b.Error("invalid field number:", f)
				return false, nil
			}
		})
	}
}
