package base64

import (
	"bytes"
	"encoding/base64"
	"math/rand"
	"testing"
)

var encodings = []struct {
	name      string
	control   *base64.Encoding
	candidate *Encoding
}{
	{
		name:      "std",
		control:   base64.StdEncoding,
		candidate: StdEncoding,
	},
	{
		name:      "url",
		control:   base64.URLEncoding,
		candidate: URLEncoding,
	},
	{
		name:      "raw-std",
		control:   base64.RawStdEncoding,
		candidate: RawStdEncoding,
	},
	{
		name:      "raw-url",
		control:   base64.RawURLEncoding,
		candidate: RawURLEncoding,
	},
	{
		name:      "imap",
		control:   base64.NewEncoding(encodeIMAP).WithPadding(NoPadding),
		candidate: NewEncoding(encodeIMAP).WithPadding(NoPadding),
	},
}

func TestEncoding(t *testing.T) {
	for _, enc := range encodings {
		t.Run(enc.name, func(t *testing.T) {
			for i := 1; i < 1024; i++ {
				src := make([]byte, i)
				rand.Read(src)

				n := enc.control.EncodedLen(i)
				encExpect := make([]byte, n)
				encActual := make([]byte, n)

				enc.control.Encode(encExpect, src)
				enc.candidate.Encode(encActual, src)

				if !bytes.Equal(encExpect, encActual) {
					t.Errorf("failed encode:\n\texpect = %v\n\tactual = %v", encExpect, encActual)
				}

				n = enc.control.DecodedLen(n)
				decExpect := make([]byte, n)
				decActual := make([]byte, n)

				nControl, errControl := enc.control.Decode(decExpect, encExpect)
				nCandidate, errCandidate := enc.candidate.Decode(decActual, encActual)

				if errControl != nil {
					t.Fatalf("control decode error: %v", errControl)
				}

				if errCandidate != nil {
					t.Fatalf("candidate decode error: %v", errCandidate)
				}

				if nControl != nCandidate {
					t.Fatalf("failed decode length: expect = %d, actual = %d", nControl, nCandidate)
				}

				if !bytes.Equal(decExpect, decActual) {
					t.Fatalf("failed decode:\n\texpect = %v\n\tactual = %v", decExpect, decActual)
				}

				encString := enc.control.EncodeToString(src)
				decExpect, errControl = enc.control.DecodeString(encString)
				decActual, errCandidate = enc.candidate.DecodeString(encString)

				if errControl != nil {
					t.Fatalf("control decode error: %v", errControl)
				}

				if errCandidate != nil {
					t.Fatalf("candidate decode error: %v", errCandidate)
				}

				if !bytes.Equal(decExpect, decActual) {
					t.Fatalf("failed decode:\n\texpect = %v\n\tactual = %v", decExpect, decActual)
				}
			}
		})
	}
}

func TestDecodeLines(t *testing.T) {
	src := []byte(`dGVzdCB0ZXN0IHRlc3QgdGVzdCB0ZXN0IHRlc3QgdGVzdCB0ZXN0IHRlc3QgdGVzdCB0ZXN0IHRl
c3QgdGVzdCB0ZXN0IHRlc3QgdGVzdCB0ZXN0IHRlc3QgdGVzdCB0ZXN0IHRlc3QgdGVzdCB0ZXN0
IHRlc3QgdGVzdCB0ZXN0IHRlc3QgdGVzdCB0ZXN0IHRlc3QgdGVzdCB0ZXN0IHRlc3QgdGVzdCB0
ZXN0IHRlc3QgdGVzdCB0ZXN0IHRlc3QgdGVzdCB0ZXN0IHRlc3QgdGVzdCB0ZXN0IHRlc3QgdGVz
dCB0ZXN0IHRlc3QgdGVzdA==`)

	expect := []byte(`test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test test`)
	actual := make([]byte, StdEncoding.DecodedLen(len(src)))
	n, err := StdEncoding.Decode(actual, src)

	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if !bytes.Equal(expect, actual[:n]) {
		t.Errorf("failed decode:\n\texpect = %v\n\tactual = %v", expect, actual)
	}
}

func BenchmarkEncode(b *testing.B) {
	src := make([]byte, 4096)
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(src)))
	rand.Read(src)

	b.Run("asm", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			StdEncoding.Encode(dst, src)
		}
		b.SetBytes(int64(len(src)))
	})

	b.Run("go", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			base64.StdEncoding.Encode(dst, src)
		}
		b.SetBytes(int64(len(src)))
	})
}

func BenchmarkDecode(b *testing.B) {
	raw := make([]byte, 4096)
	src := make([]byte, base64.StdEncoding.EncodedLen(len(raw)))
	dst := make([]byte, base64.StdEncoding.DecodedLen(len(src)))

	rand.Read(raw)
	base64.StdEncoding.Encode(src, raw)

	b.Run("asm", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			StdEncoding.Decode(dst, src)
		}
		b.SetBytes(int64(len(src)))
	})

	b.Run("go", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			base64.StdEncoding.Decode(dst, src)
		}
		b.SetBytes(int64(len(src)))
	})
}
