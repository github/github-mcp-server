// Copyright (C) 2016 Kohei YOSHIDA. All rights reserved.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of The BSD 3-Clause License
// that can be found in the LICENSE file.

package uritemplate

// threadList implements https://research.swtch.com/sparse.
type threadList struct {
	dense  []threadEntry
	sparse []uint32
}

type threadEntry struct {
	pc uint32
	t  *thread
}

type thread struct {
	op  *progOp
	cap map[string][]int
}
