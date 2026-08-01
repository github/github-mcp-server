// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"testing"
)

type BenchmarkStruct struct {
	Name    string
	Age     int
	Active  bool
	Score   float32
	Rank    float64
	Tags    []string
	Pointer *int
}

func BenchmarkStringify(b *testing.B) {
	s := &BenchmarkStruct{
		Name:    "benchmark",
		Age:     30,
		Active:  true,
		Score:   1.1,
		Rank:    99.999999,
		Tags:    []string{"go", "github", "api"},
		Pointer: Ptr(42),
	}
	b.ResetTimer()
	for b.Loop() {
		Stringify(s)
	}
}
