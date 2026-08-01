// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.
package util

import "testing"

// TestIsLoopback tests the IsLoopback helper function.
func TestIsLoopback(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"localhost", true},
		{"localhost:3000", true},
		{"127.0.0.1", true},
		{"127.0.0.1:3000", true},
		{"[::1]", true},
		{"[::1]:3000", true},
		{"::1", true},
		{"", false},
		{"evil.com", false},
		{"evil.com:80", false},
		{"localhost.evil.com", false},
		{"127.0.0.1.evil.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			if got := IsLoopback(tt.addr); got != tt.want {
				t.Errorf("IsLoopback(%q) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}
