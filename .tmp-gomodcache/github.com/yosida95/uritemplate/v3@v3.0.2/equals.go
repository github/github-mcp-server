// Copyright (C) 2016 Kohei YOSHIDA. All rights reserved.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of The BSD 3-Clause License
// that can be found in the LICENSE file.

package uritemplate

type CompareFlags uint8

const (
	CompareVarname CompareFlags = 1 << iota
)

// Equals reports whether or not two URI Templates t1 and t2 are equivalent.
func Equals(t1 *Template, t2 *Template, flags CompareFlags) bool {
	if len(t1.exprs) != len(t2.exprs) {
		return false
	}
	for i := 0; i < len(t1.exprs); i++ {
		switch t1 := t1.exprs[i].(type) {
		case literals:
			t2, ok := t2.exprs[i].(literals)
			if !ok {
				return false
			}
			if t1 != t2 {
				return false
			}
		case *expression:
			t2, ok := t2.exprs[i].(*expression)
			if !ok {
				return false
			}
			if t1.op != t2.op || len(t1.vars) != len(t2.vars) {
				return false
			}
			for n := 0; n < len(t1.vars); n++ {
				v1 := t1.vars[n]
				v2 := t2.vars[n]
				if flags&CompareVarname == CompareVarname && v1.name != v2.name {
					return false
				}
				if v1.maxlen != v2.maxlen || v1.explode != v2.explode {
					return false
				}
			}
		default:
			panic("unhandled case")
		}
	}
	return true
}
