// Copyright (C) 2016 Kohei YOSHIDA. All rights reserved.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of The BSD 3-Clause License
// that can be found in the LICENSE file.

package uritemplate

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	hex = []byte("0123456789ABCDEF")
	// reserved   = gen-delims / sub-delims
	// gen-delims =  ":" / "/" / "?" / "#" / "[" / "]" / "@"
	// sub-delims =  "!" / "$" / "&" / "’" / "(" / ")"
	//            /  "*" / "+" / "," / ";" / "="
	rangeReserved = &unicode.RangeTable{
		R16: []unicode.Range16{
			{Lo: 0x21, Hi: 0x21, Stride: 1}, // '!'
			{Lo: 0x23, Hi: 0x24, Stride: 1}, // '#' - '$'
			{Lo: 0x26, Hi: 0x2C, Stride: 1}, // '&' - ','
			{Lo: 0x2F, Hi: 0x2F, Stride: 1}, // '/'
			{Lo: 0x3A, Hi: 0x3B, Stride: 1}, // ':' - ';'
			{Lo: 0x3D, Hi: 0x3D, Stride: 1}, // '='
			{Lo: 0x3F, Hi: 0x40, Stride: 1}, // '?' - '@'
			{Lo: 0x5B, Hi: 0x5B, Stride: 1}, // '['
			{Lo: 0x5D, Hi: 0x5D, Stride: 1}, // ']'
		},
		LatinOffset: 9,
	}
	reReserved = `\x21\x23\x24\x26-\x2c\x2f\x3a\x3b\x3d\x3f\x40\x5b\x5d`
	// ALPHA      = %x41-5A / %x61-7A
	// DIGIT      = %x30-39
	// unreserved = ALPHA / DIGIT / "-" / "." / "_" / "~"
	rangeUnreserved = &unicode.RangeTable{
		R16: []unicode.Range16{
			{Lo: 0x2D, Hi: 0x2E, Stride: 1}, // '-' - '.'
			{Lo: 0x30, Hi: 0x39, Stride: 1}, // '0' - '9'
			{Lo: 0x41, Hi: 0x5A, Stride: 1}, // 'A' - 'Z'
			{Lo: 0x5F, Hi: 0x5F, Stride: 1}, // '_'
			{Lo: 0x61, Hi: 0x7A, Stride: 1}, // 'a' - 'z'
			{Lo: 0x7E, Hi: 0x7E, Stride: 1}, // '~'
		},
	}
	reUnreserved = `\x2d\x2e\x30-\x39\x41-\x5a\x5f\x61-\x7a\x7e`
)

type runeClass uint8

const (
	runeClassU runeClass = 1 << iota
	runeClassR
	runeClassPctE
	runeClassLast

	runeClassUR = runeClassU | runeClassR
)

var runeClassNames = []string{
	"U",
	"R",
	"pct-encoded",
}

func (rc runeClass) String() string {
	ret := make([]string, 0, len(runeClassNames))
	for i, j := 0, runeClass(1); j < runeClassLast; j <<= 1 {
		if rc&j == j {
			ret = append(ret, runeClassNames[i])
		}
		i++
	}
	return strings.Join(ret, "+")
}

func pctEncode(w *strings.Builder, r rune) {
	if s := r >> 24 & 0xff; s > 0 {
		w.Write([]byte{'%', hex[s/16], hex[s%16]})
	}
	if s := r >> 16 & 0xff; s > 0 {
		w.Write([]byte{'%', hex[s/16], hex[s%16]})
	}
	if s := r >> 8 & 0xff; s > 0 {
		w.Write([]byte{'%', hex[s/16], hex[s%16]})
	}
	if s := r & 0xff; s > 0 {
		w.Write([]byte{'%', hex[s/16], hex[s%16]})
	}
}

func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

func ishex(c byte) bool {
	switch {
	case '0' <= c && c <= '9':
		return true
	case 'a' <= c && c <= 'f':
		return true
	case 'A' <= c && c <= 'F':
		return true
	default:
		return false
	}
}

func pctDecode(s string) string {
	size := len(s)
	for i := 0; i < len(s); {
		switch s[i] {
		case '%':
			size -= 2
			i += 3
		default:
			i++
		}
	}
	if size == len(s) {
		return s
	}

	buf := make([]byte, size)
	j := 0
	for i := 0; i < len(s); {
		switch c := s[i]; c {
		case '%':
			buf[j] = unhex(s[i+1])<<4 | unhex(s[i+2])
			i += 3
			j++
		default:
			buf[j] = c
			i++
			j++
		}
	}
	return string(buf)
}

type escapeFunc func(*strings.Builder, string) error

func escapeLiteral(w *strings.Builder, v string) error {
	w.WriteString(v)
	return nil
}

func escapeExceptU(w *strings.Builder, v string) error {
	for i := 0; i < len(v); {
		r, size := utf8.DecodeRuneInString(v[i:])
		if r == utf8.RuneError {
			return errorf(i, "invalid encoding")
		}
		if unicode.Is(rangeUnreserved, r) {
			w.WriteRune(r)
		} else {
			pctEncode(w, r)
		}
		i += size
	}
	return nil
}

func escapeExceptUR(w *strings.Builder, v string) error {
	for i := 0; i < len(v); {
		r, size := utf8.DecodeRuneInString(v[i:])
		if r == utf8.RuneError {
			return errorf(i, "invalid encoding")
		}
		// TODO(yosida95): is pct-encoded triplets allowed here?
		if unicode.In(r, rangeUnreserved, rangeReserved) {
			w.WriteRune(r)
		} else {
			pctEncode(w, r)
		}
		i += size
	}
	return nil
}
