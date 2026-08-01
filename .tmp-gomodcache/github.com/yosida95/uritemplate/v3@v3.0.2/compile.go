// Copyright (C) 2016 Kohei YOSHIDA. All rights reserved.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of The BSD 3-Clause License
// that can be found in the LICENSE file.

package uritemplate

import (
	"fmt"
	"unicode/utf8"
)

type compiler struct {
	prog *prog
}

func (c *compiler) init() {
	c.prog = &prog{}
}

func (c *compiler) op(opcode progOpcode) uint32 {
	i := len(c.prog.op)
	c.prog.op = append(c.prog.op, progOp{code: opcode})
	return uint32(i)
}

func (c *compiler) opWithRune(opcode progOpcode, r rune) uint32 {
	addr := c.op(opcode)
	(&c.prog.op[addr]).r = r
	return addr
}

func (c *compiler) opWithRuneClass(opcode progOpcode, rc runeClass) uint32 {
	addr := c.op(opcode)
	(&c.prog.op[addr]).rc = rc
	return addr
}

func (c *compiler) opWithAddr(opcode progOpcode, absaddr uint32) uint32 {
	addr := c.op(opcode)
	(&c.prog.op[addr]).i = absaddr
	return addr
}

func (c *compiler) opWithAddrDelta(opcode progOpcode, delta uint32) uint32 {
	return c.opWithAddr(opcode, uint32(len(c.prog.op))+delta)
}

func (c *compiler) opWithName(opcode progOpcode, name string) uint32 {
	addr := c.op(opcode)
	(&c.prog.op[addr]).name = name
	return addr
}

func (c *compiler) compileString(str string) {
	for i := 0; i < len(str); {
		// NOTE(yosida95): It is confirmed at parse time that literals
		// consist of only valid-UTF8 runes.
		r, size := utf8.DecodeRuneInString(str[i:])
		c.opWithRune(opRune, r)
		i += size
	}
}

func (c *compiler) compileRuneClass(rc runeClass, maxlen int) {
	for i := 0; i < maxlen; i++ {
		if i > 0 {
			c.opWithAddrDelta(opSplit, 7)
		}
		c.opWithAddrDelta(opSplit, 3)                 // raw rune or pct-encoded
		c.opWithRuneClass(opRuneClass, rc)            // raw rune
		c.opWithAddrDelta(opJmp, 4)                   //
		c.opWithRune(opRune, '%')                     // pct-encoded
		c.opWithRuneClass(opRuneClass, runeClassPctE) //
		c.opWithRuneClass(opRuneClass, runeClassPctE) //
	}
}

func (c *compiler) compileRuneClassInfinite(rc runeClass) {
	start := c.opWithAddrDelta(opSplit, 3)        // raw rune or pct-encoded
	c.opWithRuneClass(opRuneClass, rc)            // raw rune
	c.opWithAddrDelta(opJmp, 4)                   //
	c.opWithRune(opRune, '%')                     // pct-encoded
	c.opWithRuneClass(opRuneClass, runeClassPctE) //
	c.opWithRuneClass(opRuneClass, runeClassPctE) //
	c.opWithAddrDelta(opSplit, 2)                 // loop
	c.opWithAddr(opJmp, start)                    //
}

func (c *compiler) compileVarspecValue(spec varspec, expr *expression) {
	var specname string
	if spec.maxlen > 0 {
		specname = fmt.Sprintf("%s:%d", spec.name, spec.maxlen)
	} else {
		specname = spec.name
	}

	c.prog.numCap++

	c.opWithName(opCapStart, specname)

	split := c.op(opSplit)
	if spec.maxlen > 0 {
		c.compileRuneClass(expr.allow, spec.maxlen)
	} else {
		c.compileRuneClassInfinite(expr.allow)
	}

	capEnd := c.opWithName(opCapEnd, specname)
	c.prog.op[split].i = capEnd
}

func (c *compiler) compileVarspec(spec varspec, expr *expression) {
	switch {
	case expr.named && spec.explode:
		split1 := c.op(opSplit)
		noop := c.op(opNoop)
		c.compileString(spec.name)

		split2 := c.op(opSplit)
		c.opWithRune(opRune, '=')
		c.compileVarspecValue(spec, expr)

		split3 := c.op(opSplit)
		c.compileString(expr.sep)
		c.opWithAddr(opJmp, noop)

		c.prog.op[split2].i = uint32(len(c.prog.op))
		c.compileString(expr.ifemp)
		c.opWithAddr(opJmp, split3)

		c.prog.op[split1].i = uint32(len(c.prog.op))
		c.prog.op[split3].i = uint32(len(c.prog.op))

	case expr.named && !spec.explode:
		c.compileString(spec.name)

		split2 := c.op(opSplit)
		c.opWithRune(opRune, '=')

		split3 := c.op(opSplit)

		split4 := c.op(opSplit)
		c.compileVarspecValue(spec, expr)

		split5 := c.op(opSplit)
		c.prog.op[split4].i = split5
		c.compileString(",")
		c.opWithAddr(opJmp, split4)

		c.prog.op[split3].i = uint32(len(c.prog.op))
		c.compileString(",")
		jmp1 := c.op(opJmp)

		c.prog.op[split2].i = uint32(len(c.prog.op))
		c.compileString(expr.ifemp)

		c.prog.op[split5].i = uint32(len(c.prog.op))
		c.prog.op[jmp1].i = uint32(len(c.prog.op))

	case !expr.named:
		start := uint32(len(c.prog.op))
		c.compileVarspecValue(spec, expr)

		split1 := c.op(opSplit)
		jmp := c.op(opJmp)

		c.prog.op[split1].i = uint32(len(c.prog.op))
		if spec.explode {
			c.compileString(expr.sep)
		} else {
			c.opWithRune(opRune, ',')
		}
		c.opWithAddr(opJmp, start)

		c.prog.op[jmp].i = uint32(len(c.prog.op))
	}
}

func (c *compiler) compileExpression(expr *expression) {
	if len(expr.vars) < 1 {
		return
	}

	split1 := c.op(opSplit)
	c.compileString(expr.first)

	for i, size := 0, len(expr.vars); i < size; i++ {
		spec := expr.vars[i]

		split2 := c.op(opSplit)
		if i > 0 {
			split3 := c.op(opSplit)
			c.compileString(expr.sep)
			c.prog.op[split3].i = uint32(len(c.prog.op))
		}
		c.compileVarspec(spec, expr)
		c.prog.op[split2].i = uint32(len(c.prog.op))
	}

	c.prog.op[split1].i = uint32(len(c.prog.op))
}

func (c *compiler) compileLiterals(lt literals) {
	c.compileString(string(lt))
}

func (c *compiler) compile(tmpl *Template) {
	c.op(opLineBegin)
	for i := range tmpl.exprs {
		expr := tmpl.exprs[i]
		switch expr := expr.(type) {
		default:
			panic("unhandled expression")
		case *expression:
			c.compileExpression(expr)
		case literals:
			c.compileLiterals(expr)
		}
	}
	c.op(opLineEnd)
	c.op(opEnd)
}
