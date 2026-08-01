// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package mcpgodebug

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseCompatibility_Success(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
		want   map[string]string
	}{
		{
			name:   "Basic",
			envVal: "foo=bar,baz=qux",
			want: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
		},
		{
			name:   "Empty",
			envVal: "",
			want:   nil,
		},
		{
			name:   "WithWhitespace",
			envVal: "  foo = bar  \t,  baz  = qux  ",
			want: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
		},
		{
			name:   "WithEqualsSignInValue",
			envVal: "foo=bar=baz",
			want: map[string]string{
				"foo": "bar=baz",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCompatibility(tt.envVal)
			if err != nil {
				t.Fatalf("parseCompatibility() failed: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("parseCompatibility() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseCompatibility_Failure(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
	}{
		{
			name:   "NoEqualsSign",
			envVal: "invalidformat",
		},
		{
			name:   "MixedValidAndInvalid",
			envVal: "foo=bar,baz",
		},
		{
			name:   "EmptyPart",
			envVal: "foo=bar,,baz=qux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCompatibility(tt.envVal)
			if err == nil {
				t.Error("parseCompatibility() expected error, got nil")
			}
		})
	}
}
