// Copyright (C) 2016 Kohei YOSHIDA. All rights reserved.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of The BSD 3-Clause License
// that can be found in the LICENSE file.

package uritemplate

import (
	"bytes"
	"strconv"
)

type progOpcode uint16

const (
	// match
	opRune progOpcode = iota
	opRuneClass
	opLineBegin
	opLineEnd
	// capture
	opCapStart
	opCapEnd
	// stack
	opSplit
	opJmp
	opJmpIfNotDefined
	opJmpIfNotEmpty
	opJmpIfNotFirst
	// result
	opEnd
	// fake
	opNoop
	opcodeMax
)

var opcodeNames = []string{
	// match
	"opRune",
	"opRuneClass",
	"opLineBegin",
	"opLineEnd",
	// capture
	"opCapStart",
	"opCapEnd",
	// stack
	"opSplit",
	"opJmp",
	"opJmpIfNotDefined",
	"opJmpIfNotEmpty",
	"opJmpIfNotFirst",
	// result
	"opEnd",
}

func (code progOpcode) String() string {
	if code >= opcodeMax {
		return ""
	}
	return opcodeNames[code]
}

type progOp struct {
	code progOpcode
	r    rune
	rc   runeClass
	i    uint32

	name string
}

func dumpProgOp(b *bytes.Buffer, op *progOp) {
	b.WriteString(op.code.String())
	switch op.code {
	case opRune:
		b.WriteString("(")
		b.WriteString(strconv.QuoteToASCII(string(op.r)))
		b.WriteString(")")
	case opRuneClass:
		b.WriteString("(")
		b.WriteString(op.rc.String())
		b.WriteString(")")
	case opCapStart, opCapEnd:
		b.WriteString("(")
		b.WriteString(strconv.QuoteToASCII(op.name))
		b.WriteString(")")
	case opSplit:
		b.WriteString(" -> ")
		b.WriteString(strconv.FormatInt(int64(op.i), 10))
	case opJmp, opJmpIfNotFirst:
		b.WriteString(" -> ")
		b.WriteString(strconv.FormatInt(int64(op.i), 10))
	case opJmpIfNotDefined, opJmpIfNotEmpty:
		b.WriteString("(")
		b.WriteString(strconv.QuoteToASCII(op.name))
		b.WriteString(")")
		b.WriteString(" -> ")
		b.WriteString(strconv.FormatInt(int64(op.i), 10))
	}
}

type prog struct {
	op     []progOp
	numCap int
}

func dumpProg(b *bytes.Buffer, prog *prog, pc uint32) {
	for i := range prog.op {
		op := prog.op[i]

		pos := strconv.Itoa(i)
		if uint32(i) == pc {
			pos = "*" + pos
		}
		b.WriteString("    "[len(pos):])
		b.WriteString(pos)

		b.WriteByte('\t')
		dumpProgOp(b, &op)

		b.WriteByte('\n')
	}
}

func (p *prog) String() string {
	b := bytes.Buffer{}
	dumpProg(&b, p, 0)
	return b.String()
}
