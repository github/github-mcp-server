// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package authutil

import "testing"

func TestIssuersEqual(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"https://issuer.example.com", "https://issuer.example.com", true},
		{"https://issuer.example.com/", "https://issuer.example.com", true},
		{"https://issuer.example.com", "https://issuer.example.com/", true},
		{"https://issuer.example.com/", "https://issuer.example.com/", true},
		{"https://issuer.example.com/tenant", "https://issuer.example.com/tenant", true},
		{"https://issuer.example.com/tenant/", "https://issuer.example.com/tenant", true},
		{"https://issuer.example.com", "https://other.example.com", false},
		{"https://issuer.example.com/a", "https://issuer.example.com/b", false},
		{"", "", true},
	}
	for _, tt := range tests {
		if got := IssuersEqual(tt.a, tt.b); got != tt.want {
			t.Errorf("IssuersEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
