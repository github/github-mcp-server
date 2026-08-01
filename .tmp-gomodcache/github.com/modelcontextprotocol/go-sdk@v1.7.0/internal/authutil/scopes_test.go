// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.
package authutil

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestUnionScopes(t *testing.T) {
	tests := []struct {
		name       string
		existing   []string
		challenged []string
		want       []string
	}{
		{
			name:       "both empty",
			existing:   nil,
			challenged: nil,
			want:       nil,
		},
		{
			name:       "existing only",
			existing:   []string{"read"},
			challenged: nil,
			want:       []string{"read"},
		},
		{
			name:       "challenged only",
			existing:   nil,
			challenged: []string{"write"},
			want:       []string{"write"},
		},
		{
			name:       "disjoint scopes",
			existing:   []string{"read"},
			challenged: []string{"write"},
			want:       []string{"read", "write"},
		},
		{
			name:       "overlapping scopes",
			existing:   []string{"read", "write"},
			challenged: []string{"write", "admin"},
			want:       []string{"read", "write", "admin"},
		},
		{
			name:       "identical scopes",
			existing:   []string{"read", "write"},
			challenged: []string{"read", "write"},
			want:       []string{"read", "write"},
		},
		{
			name:       "mixed scopes",
			existing:   []string{"b", "a"},
			challenged: []string{"c", "a"},
			want:       []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UnionScopes(tt.existing, tt.challenged)
			if diff := cmp.Diff(tt.want, got, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
				t.Errorf("UnionScopes() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
