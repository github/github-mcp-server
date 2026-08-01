//go:build amd64 && !purego
// +build amd64,!purego

package base64

import (
	"encoding/base64"

	"github.com/segmentio/asm/cpu"
	"github.com/segmentio/asm/cpu/x86"
	"github.com/segmentio/asm/internal/unsafebytes"
)

// An Encoding is a radix 64 encoding/decoding scheme, defined by a
// 64-character alphabet.
type Encoding struct {
	enc    func(dst []byte, src []byte, lut *int8) (int, int)
	enclut [32]int8

	dec    func(dst []byte, src []byte, lut *int8) (int, int)
	declut [48]int8

	base *base64.Encoding
}

const (
	minEncodeLen = 28
	minDecodeLen = 45
)

func newEncoding(encoder string) *Encoding {
	e := &Encoding{base: base64.NewEncoding(encoder)}
	if cpu.X86.Has(x86.AVX2) {
		e.enableEncodeAVX2(encoder)
		e.enableDecodeAVX2(encoder)
	}
	return e
}

func (e *Encoding) enableEncodeAVX2(encoder string) {
	// Translate values 0..63 to the Base64 alphabet. There are five sets:
	//
	// From      To         Add    Index  Example
	// [0..25]   [65..90]   +65        0  ABCDEFGHIJKLMNOPQRSTUVWXYZ
	// [26..51]  [97..122]  +71        1  abcdefghijklmnopqrstuvwxyz
	// [52..61]  [48..57]    -4  [2..11]  0123456789
	// [62]      [43]       -19       12  +
	// [63]      [47]       -16       13  /
	tab := [32]int8{int8(encoder[0]), int8(encoder[letterRange]) - letterRange}
	for i, ch := range encoder[2*letterRange:] {
		tab[2+i] = int8(ch) - 2*letterRange - int8(i)
	}

	e.enc = encodeAVX2
	e.enclut = tab
}

func (e *Encoding) enableDecodeAVX2(encoder string) {
	c62, c63 := int8(encoder[62]), int8(encoder[63])
	url := c63 == '_'
	if url {
		c63 = '/'
	}

	// Translate values from the Base64 alphabet using five sets. Values outside
	// of these ranges are considered invalid:
	//
	// From       To        Add    Index  Example
	// [47]       [63]      +16        1  /
	// [43]       [62]      +19        2  +
	// [48..57]   [52..61]   +4        3  0123456789
	// [65..90]   [0..25]   -65      4,5  ABCDEFGHIJKLMNOPQRSTUVWXYZ
	// [97..122]  [26..51]  -71      6,7  abcdefghijklmnopqrstuvwxyz
	tab := [48]int8{
		0, 63 - c63, 62 - c62, 4, -65, -65, -71, -71,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x15, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x11,
		0x11, 0x11, 0x13, 0x1B, 0x1B, 0x1B, 0x1B, 0x1B,
	}
	tab[(c62&15)+16] = 0x1A
	tab[(c63&15)+16] = 0x1A

	if url {
		e.dec = decodeAVX2URI
	} else {
		e.dec = decodeAVX2
	}
	e.declut = tab
}

// WithPadding creates a duplicate Encoding updated with a specified padding
// character, or NoPadding to disable padding. The padding character must not
// be contained in the encoding alphabet, must not be '\r' or '\n', and must
// be no greater than '\xFF'.
func (enc Encoding) WithPadding(padding rune) *Encoding {
	enc.base = enc.base.WithPadding(padding)
	return &enc
}

// Strict creates a duplicate encoding updated with strict decoding enabled.
// This requires that trailing padding bits are zero.
func (enc Encoding) Strict() *Encoding {
	enc.base = enc.base.Strict()
	return &enc
}

// Encode encodes src using the defined encoding alphabet.
// This will write EncodedLen(len(src)) bytes to dst.
func (enc *Encoding) Encode(dst, src []byte) {
	if len(src) >= minEncodeLen && enc.enc != nil {
		d, s := enc.enc(dst, src, &enc.enclut[0])
		dst = dst[d:]
		src = src[s:]
	}
	enc.base.Encode(dst, src)
}

// Encode encodes src using the encoding enc, writing
// EncodedLen(len(src)) bytes to dst.
func (enc *Encoding) EncodeToString(src []byte) string {
	buf := make([]byte, enc.base.EncodedLen(len(src)))
	enc.Encode(buf, src)
	return string(buf)
}

// EncodedLen calculates the base64-encoded byte length for a message
// of length n.
func (enc *Encoding) EncodedLen(n int) int {
	return enc.base.EncodedLen(n)
}

// Decode decodes src using the defined encoding alphabet.
// This will write DecodedLen(len(src)) bytes to dst and return the number of
// bytes written.
func (enc *Encoding) Decode(dst, src []byte) (n int, err error) {
	var d, s int
	if len(src) >= minDecodeLen && enc.dec != nil {
		d, s = enc.dec(dst, src, &enc.declut[0])
		dst = dst[d:]
		src = src[s:]
	}
	n, err = enc.base.Decode(dst, src)
	n += d
	return
}

// DecodeString decodes the base64 encoded string s, returns the decoded
// value as bytes.
func (enc *Encoding) DecodeString(s string) ([]byte, error) {
	src := unsafebytes.BytesOf(s)
	dst := make([]byte, enc.base.DecodedLen(len(s)))
	n, err := enc.Decode(dst, src)
	return dst[:n], err
}

// DecodedLen calculates the decoded byte length for a base64-encoded message
// of length n.
func (enc *Encoding) DecodedLen(n int) int {
	return enc.base.DecodedLen(n)
}
